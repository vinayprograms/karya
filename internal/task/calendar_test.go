package task

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestCalendar(t *testing.T) {
	start := time.Date(2026, 11, 23, 0, 0, 0, 0, time.Local)
	for _, test := range []struct {
		token string
		count int
	}{{"mon", 1}, {"MONDAY+1w", 1}, {"weekdaysT09:00", 5}, {"dailyT09:00+1d", 7}, {"friT14:30-15:00", 1}, {"mondayT14:30+2w", 1}} {
		t.Run(test.token, func(t *testing.T) {
			schedule, err := ParseSchedule(test.token)
			if err != nil {
				t.Fatal(err)
			}
			schedule.Anchor = start
			dates := schedule.ExpandOccurrences(start, start.AddDate(0, 0, 6))
			if len(dates) != test.count {
				t.Fatalf("dates=%v", dates)
			}
			if schedule.FormatToken() != test.token {
				t.Fatal("spelling changed")
			}
		})
	}
	for _, token := range []string{"mon+2d", "weekdays+2d", "daily+2w", "mon+0w", "mondayT25:00", "monT09:00-08:00", "mon.+2w", "noday", "mon+1wgarbage"} {
		if _, err := ParseSchedule(token); err == nil {
			t.Errorf("accepted %s", token)
		}
	}
}

func TestAnchors(t *testing.T) {
	cfg := fixture(t, "MEETING: sync @s:monT14:30+2w @d:fri+2w\n  Notes stay.\nTODO: work\n")
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.Local)
	if err := Refresh(cfg, now); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if !strings.Contains(string(data), "LOG(scheduled): 2026-11-30T14:30") || !strings.Contains(string(data), "LOG(due): 2026-11-27T00:00") {
		t.Fatalf("anchors: %s", data)
	}
	if err := Refresh(cfg, now.AddDate(0, 0, 10)); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(cfg.GetInboxFilePath())
	if string(again) != string(data) {
		t.Fatal("anchor moved")
	}
	tasks, err := ListTasks(cfg, "", true)
	if err != nil {
		t.Fatal(err)
	}
	event := tasks[0]
	transitions, err := ParseStateTransitions(event)
	if err != nil || len(transitions) != 0 {
		t.Fatalf("anchor became completion: %v %v", transitions, err)
	}
	schedule, err := resolve(event, event.ScheduledAt, "scheduled", now.AddDate(0, 0, 10), false)
	if err != nil {
		t.Fatal(err)
	}
	dates := schedule.ExpandOccurrences(now, now.AddDate(0, 0, 30))
	if len(dates) != 2 || dates[0].Day() != 30 || dates[1].Day() != 14 {
		t.Fatalf("alternating dates: %v", dates)
	}
	if err := SetTaskDate(event, "mondayT14:30+2w", "", false, false); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(cfg.GetInboxFilePath())
	if strings.Count(string(data), "LOG(scheduled)") != 1 {
		t.Fatal("alias created new anchor")
	}
}

func TestAnchorChanges(t *testing.T) {
	cfg := fixture(t, "MEETING: sync @s:monT14:30+2w\n  * LOG(scheduled): 2026-11-30T14:30\n")
	tasks, err := ListTasks(cfg, "", true)
	if err != nil {
		t.Fatal(err)
	}
	item := tasks[0]
	if err := SetTaskDate(item, "mondayT14:30+3w", "", false, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if strings.Count(string(data), "LOG(scheduled):") != 2 || !strings.Contains(string(data), "LOG(scheduled): 2026-11-30T14:30") {
		t.Fatalf("history changed: %s", data)
	}
	schedule, err := resolve(item, item.ScheduledAt, "scheduled", time.Now(), false)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	want := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[1]), "* LOG(scheduled):"))
	if schedule.Anchor.Format("2006-01-02T15:04") != want {
		t.Fatal("latest entry not used")
	}
	transitions, err := ParseStateTransitions(item)
	if err != nil || len(transitions) != 0 {
		t.Fatal("anchor polluted transitions")
	}
}

func TestCalendarAgenda(t *testing.T) {
	today := truncateToDay(time.Now())
	cfg := fixture(t, "EVENT: daily @s:dailyT09:00-10:00\nTODO: habit @s:daily\n")
	days, err := QueryAgenda(cfg, today, today.AddDate(0, 0, 2), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 3 {
		t.Fatalf("days=%v", days)
	}
	for _, day := range days {
		if len(day.Items) != 2 {
			t.Fatalf("items=%v", day.Items)
		}
		for _, item := range day.Items {
			if item.IsOverdue {
				t.Fatal("calendar occurrence overdue")
			}
			if item.Task.Event(cfg) && item.EndTime.Day() != item.Date.Day() {
				t.Fatal("end date not shifted")
			}
		}
	}
	tasks, _ := ListTasks(cfg, "", true)
	for _, item := range tasks {
		if item.Keyword == "TODO" {
			if handled, err := CompleteRecurringTask(item, cfg, "DONE"); err != nil || !handled {
				t.Fatalf("completion %v %v", handled, err)
			}
		}
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if !strings.Contains(string(data), "TODO: habit @s:daily") {
		t.Fatal("completion rewrote rule")
	}
}

func TestCalendarMultiple(t *testing.T) {
	cfg := fixture(t, "MEETING: sync @s:mon+2w\nMEETING: sync @s:mon+2w\nMEETING: dated @s:2026-11-20+1d\n")
	now := time.Date(2026, 11, 24, 0, 0, 0, 0, time.Local)
	if err := Refresh(cfg, now); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if strings.Count(string(data), "LOG(scheduled)") != 2 || !strings.Contains(string(data), "dated @s:2026-11-24+1d") {
		t.Fatalf("refresh: %s", data)
	}
	if err := Refresh(cfg, now.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(cfg.GetInboxFilePath())
	if strings.Count(string(data), "LOG(scheduled)") != 2 {
		t.Fatalf("duplicate anchors: %s", data)
	}
}

func TestCalendarPicker(t *testing.T) {
	item := &Task{ScheduledAt: "MondayT09:00+1w"}
	picker := NewDatePicker(item, FieldScheduled)
	scheduled, _, _, _ := picker.Result()
	if scheduled != item.ScheduledAt {
		t.Fatalf("picker changed rule: %s", scheduled)
	}
	picker = NewDatePicker(&Task{}, FieldScheduled)
	picker.Update("r")
	for _, char := range "weekdaysT09:00" {
		picker.Update(string(char))
	}
	picker.Update("enter")
	scheduled, _, _, _ = picker.Result()
	if !picker.Confirmed || scheduled != "weekdaysT09:00" {
		t.Fatalf("picker entry: %s, %+v", scheduled, picker)
	}
}
