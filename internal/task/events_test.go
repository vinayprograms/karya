package task

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vinayprograms/karya/internal/config"
)

func fixture(t *testing.T, content string) *config.Config {
	t.Helper()
	cfg := &config.Config{Directories: config.Directories{Projects: t.TempDir(), Karya: t.TempDir()}, Todo: config.Todo{Active: []string{"TODO", "MEETING"}, Completed: []string{"DONE"}, Events: []string{"MEETING", "EVENT", "CALL"}}}
	if err := os.WriteFile(cfg.GetInboxFilePath(), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestRefresh(t *testing.T) {
	for _, mode := range []string{"+", "++", ".+"} {
		t.Run(mode, func(t *testing.T) {
			content := "MEETING: sync @s:2026-11-20T09:00-10:00" + mode + "1d @d:2026-11-20" + mode + "1d\n  Notes stay here.\nMEETING: sync @2026-11-21" + mode + "1d\nTODO: work @2026-11-20+1d\nEVENT: once @2026-11-20\n"
			cfg := fixture(t, content)
			now := time.Date(2026, 11, 23, 16, 0, 0, 0, time.Local)
			if err := Refresh(cfg, now); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(cfg.GetInboxFilePath())
			if err != nil {
				t.Fatal(err)
			}
			want := strings.ReplaceAll(content, "2026-11-20T09:00-10:00"+mode, "2026-11-23T09:00-10:00"+mode)
			want = strings.ReplaceAll(want, "@d:2026-11-20"+mode, "@d:2026-11-23"+mode)
			want = strings.ReplaceAll(want, "@2026-11-21"+mode, "@2026-11-23"+mode)
			if string(data) != want {
				t.Fatalf("refresh = %q, want %q", data, want)
			}
			if err := Refresh(cfg, now); err != nil {
				t.Fatal(err)
			}
			again, _ := os.ReadFile(cfg.GetInboxFilePath())
			if string(again) != want {
				t.Fatal("refresh was not idempotent")
			}
		})
	}
}

func TestEvents(t *testing.T) {
	today := truncateToDay(time.Now())
	past := today.AddDate(0, 0, -2).Format("2006-01-02")
	cfg := fixture(t, "MEETING: past @"+past+"\nEVENT: recurring @"+past+"+1d\nCALL: today @"+today.Format("2006-01-02")+"\nTODO: overdue @"+past+"\n")
	days, err := QueryAgenda(cfg, today.AddDate(0, 0, -2), today, true)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, day := range days {
		for _, item := range day.Items {
			counts[item.Task.Title]++
			if item.Task.Event(cfg) && (item.IsOverdue || !truncateToDay(item.Date).Equal(day.Date) || item.IsCompleted || item.ClockActive) {
				t.Fatalf("invalid event agenda entry: %+v", item)
			}
			if item.Task.Title == "overdue" && !item.IsOverdue {
				t.Fatal("work overdue behavior regressed")
			}
		}
	}
	if counts["past"] != 1 || counts["recurring"] != 3 || counts["today"] != 1 || counts["overdue"] != 1 {
		t.Fatalf("counts: %v", counts)
	}
	event := ParseLine(cfg, "MEETING: past @"+past, "inbox", "", cfg.GetInboxFilePath())
	if event == nil || event.Status(cfg) != Events || event.IsActive(cfg) {
		t.Fatal("event precedence failed")
	}
	before, _ := os.ReadFile(cfg.GetInboxFilePath())
	if err := UpdateTaskStatus(event, "DONE", cfg); err == nil {
		t.Fatal("event completed")
	}
	if _, err := CompleteRecurringTask(event, cfg, "DONE"); err == nil {
		t.Fatal("event logged completion")
	}
	after, _ := os.ReadFile(cfg.GetInboxFilePath())
	if string(before) != string(after) {
		t.Fatal("event status guard modified file")
	}
}

func TestEventsMCP(t *testing.T) {
	cfg := fixture(t, "EVENT: launch @2026-11-20+1d\nTODO: work\n")
	server := &MCPServer{config: cfg}
	_, listed, err := server.events(t.Context(), nil, EventQuery{})
	if err != nil || listed.Count != 1 || listed.Events[0].Status != "events" {
		t.Fatalf("events: %+v, %v", listed, err)
	}
	_, work, err := server.listTasks(t.Context(), nil, ListTasksArgs{})
	if err != nil || work.Count != 1 || work.Tasks[0].Keyword != "TODO" {
		t.Fatalf("tasks: %+v, %v", work, err)
	}
	_, count, err := server.countTasks(t.Context(), nil, CountTasksArgs{})
	if err != nil || count.Count != 1 {
		t.Fatalf("count: %+v, %v", count, err)
	}
	_, status, err := server.updateTaskStatus(t.Context(), nil, UpdateTaskStatusArgs{Project: "", Keyword: "EVENT", Title: "launch", NewKeyword: "DONE"})
	if err != nil || status.Success {
		t.Fatalf("status: %+v, %v", status, err)
	}
	_, clock, err := server.clockIn(t.Context(), nil, ClockInArgs{Project: "", Keyword: "EVENT", Title: "launch"})
	if err != nil || clock.Success {
		t.Fatalf("clock: %+v, %v", clock, err)
	}
	_, updated, err := server.reschedule(t.Context(), nil, ScheduleTaskArgs{Project: "", Keyword: "EVENT", Title: "launch", ScheduledAt: "2026-12-01+1d"})
	if err != nil || !updated.Success {
		t.Fatalf("schedule: %+v, %v", updated, err)
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if string(data) != "EVENT: launch @s:2026-12-01+1d\nTODO: work\n" {
		t.Fatalf("content: %q", data)
	}
}

func TestEventIdentity(t *testing.T) {
	cfg := fixture(t, "EVENT:sync @2026-11-20+1d\nEVENT:  sync @2026-11-21+1d\n")
	server := &MCPServer{config: cfg}
	_, result, err := server.reschedule(t.Context(), nil, ScheduleTaskArgs{Keyword: "EVENT", RawTitle: "sync @2026-11-21+1d", ScheduledAt: "2026-12-01+1d"})
	if err != nil || !result.Success {
		t.Fatalf("reschedule: %+v, %v", result, err)
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if string(data) != "EVENT:sync @2026-11-20+1d\nEVENT:  sync @s:2026-12-01+1d\n" {
		t.Fatalf("wrong event changed: %q", data)
	}
	if err := Refresh(cfg, time.Date(2026, 11, 23, 12, 0, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(cfg.GetInboxFilePath())
	if string(data) != "EVENT:sync @2026-11-23+1d\nEVENT:  sync @s:2026-12-01+1d\n" {
		t.Fatalf("whitespace refresh: %q", data)
	}
}

func TestRefreshIsolation(t *testing.T) {
	cfg := fixture(t, "EVENT: invalid @2026-99-99+1d\nEVENT: valid @2026-11-20+1d\n")
	if err := Refresh(cfg, time.Date(2026, 11, 23, 12, 0, 0, 0, time.Local)); err == nil {
		t.Fatal("missing invalid-event error")
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if !strings.Contains(string(data), "EVENT: valid @2026-11-23+1d") {
		t.Fatalf("healthy event not refreshed: %q", data)
	}
}

func TestRecurrenceInterval(t *testing.T) {
	for _, token := range []string{"2026-11-20+0d", "2026-11-20++0w", "2026-11-20.+0m", "2026-11-20+999999999999999999999999d"} {
		if _, err := ParseSchedule(token); err == nil {
			t.Errorf("accepted invalid interval %q", token)
		}
	}
}
