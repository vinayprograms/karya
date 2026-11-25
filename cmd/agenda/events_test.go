package main

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/task"
)

func TestEventColor(t *testing.T) {
	cfg := &config.Config{
		Todo:   config.Todo{Events: []string{"MEETING", "CUSTOM"}},
		Colors: config.ColorScheme{EventsColor: "#56B6C2", ActiveColor: "#00ff00", TaskColor: "#123456", CompletedColor: "#abcdef"},
	}
	initColors(cfg)
	for _, keyword := range cfg.Todo.Events {
		got := keywordStyle(task.KeywordStatus(cfg, keyword)).GetForeground()
		if got != colors.events.GetForeground() || got == colors.active.GetForeground() || got == colors.taskText.GetForeground() || got == colors.completed.GetForeground() {
			t.Errorf("%s color = %v, want dedicated event color, distinct from description", keyword, got)
		}
	}
}

func TestHints(t *testing.T) {
	cfg := &config.Config{Todo: config.Todo{Events: []string{"MEETING"}}}
	initColors(cfg)
	m := initialModel(cfg)
	m.termWidth, m.termHeight = 160, 40
	event, work := &task.Task{Keyword: "MEETING"}, &task.Task{Keyword: "TODO"}
	m.flatItems = []task.AgendaItem{{Task: event}, {Task: work}}
	m.clockTable = &task.ClockTable{GrandTotal: 2 * time.Hour, Projects: []task.ClockTableProject{{Total: 2 * time.Hour, Entries: []task.ClockTableEntry{{Task: event, Duration: time.Hour}, {Task: work, Duration: time.Hour}}}}}
	m.buildClockLineMapping()
	for _, clockView := range []bool{false, true} {
		m.showingClockView = clockView
		for _, cursor := range []int{0, 1, 0} {
			m.cursor, m.clockCursor = cursor, cursor
			view, help := m.View(), m.renderHelp()
			if strings.Contains(view, "t: status") != (cursor == 1) || strings.Contains(help, "change status") != (cursor == 1) {
				t.Errorf("clock view %t cursor %d: incorrect status hints", clockView, cursor)
			}
			if !strings.Contains(view, "i/o: in/out") || !strings.Contains(help, "clock in") || !strings.Contains(help, "clock out") {
				t.Errorf("clock view %t cursor %d: missing clock hints", clockView, cursor)
			}
			if !strings.Contains(view, "enter: edit") || (!clockView && !strings.Contains(view, "S/D: schedule/due")) {
				t.Fatal("event-compatible actions missing")
			}
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
			if updated.(model).showingStatusPicker != (cursor == 1) {
				t.Errorf("clock view %t cursor %d: incorrect status action", clockView, cursor)
			}
		}
	}
	m.showingClockView = false
	m.flatItems = nil
	if m.event() {
		t.Fatal("empty selection treated as event")
	}
}

func TestEvents(t *testing.T) {
	cfg := &config.Config{Directories: config.Directories{Projects: t.TempDir(), Karya: t.TempDir()}, Todo: config.Todo{Active: []string{"TODO"}, Events: []string{"MEETING"}}}
	content := "MEETING: sync @2026-11-20+1d\n  Notes\n"
	if err := os.WriteFile(cfg.GetInboxFilePath(), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	initColors(cfg)
	for _, overlay := range []string{"none", "help", "detail", "clock", "status", "date"} {
		t.Run(overlay, func(t *testing.T) {
			m := initialModel(cfg)
			m.showingHelp = overlay == "help"
			m.showingDetailView = overlay == "detail"
			m.showingClockView = overlay == "clock"
			m.showingStatusPicker = overlay == "status"
			m.showingDatePicker = overlay == "date"
			updated, command := m.Update(refresh(time.Date(2026, 11, 23, 12, 0, 0, 0, time.Local)))
			if updated.(model).err != nil || command == nil {
				t.Fatalf("refresh: %+v", updated)
			}
			if overlay == "date" {
				if len(updated.(model).flatItems) != 0 {
					t.Fatal("refresh should defer while editing dates")
				}
				return
			}
			if len(updated.(model).flatItems) == 0 {
				t.Fatal("overlay lost refreshed agenda")
			}
			batch, ok := command().(tea.BatchMsg)
			if !ok || len(batch) != 2 {
				t.Fatalf("refresh must reload and rearm timer: %T", command())
			}
		})
	}
	data, _ := os.ReadFile(cfg.GetInboxFilePath())
	if string(data) != strings.Replace(content, "2026-11-20", "2026-11-23", 1) {
		t.Fatalf("content: %q", data)
	}
	event := &task.Task{Keyword: "MEETING", Title: "sync", FilePath: cfg.GetInboxFilePath()}
	for _, key := range []string{"t", "enter"} {
		m := initialModel(cfg)
		m.flatItems = []task.AgendaItem{{Task: event}}
		updated, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		if key == "enter" {
			if command == nil {
				t.Fatal("event editor unavailable")
			}
		} else if command != nil || updated.(model).showingStatusPicker {
			t.Fatalf("event action %s was enabled", key)
		}
	}
}

func TestEventClockKeys(t *testing.T) {
	for _, test := range []struct {
		name, schedule       string
		clockView, overnight bool
	}{
		{name: "one-off"},
		{name: "recurring", schedule: "dailyT09:00"},
		{name: "after midnight in agenda", schedule: "dailyT09:00", overnight: true},
		{name: "after midnight in clock table", overnight: true, clockView: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now()
			schedule := test.schedule
			if test.clockView {
				schedule = now.AddDate(0, 0, 6).Format("2006-01-02") + "+1w"
			} else if schedule == "" {
				schedule = now.Format("2006-01-02")
			}
			cfg := &config.Config{Directories: config.Directories{Projects: t.TempDir(), Karya: t.TempDir()}, Todo: config.Todo{Events: []string{"MEETING"}}, Schedule: config.Schedule{DefaultView: "day"}}
			path := cfg.GetInboxFilePath()
			heading := "MEETING: Clock sample @s:" + schedule
			content := heading + "\n  Notes\n"
			if test.overnight {
				content = heading + "\n  * CLOCK: " + now.AddDate(0, 0, -1).Format("2006-01-02") + "T23:00--\n  Notes\n"
			}
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			initColors(cfg)
			m := initialModel(cfg)
			m.termWidth, m.termHeight = 160, 40
			m.showingClockView = test.clockView
			if test.overnight {
				m.focusDate = now.AddDate(0, 0, -1)
			}
			var loaded tea.Msg
			if test.clockView {
				loaded = loadClockTableCmd(cfg, m.focusDate, m.mode)()
			} else {
				loaded = loadAgendaCmd(cfg, m.focusDate, m.mode)()
			}
			updated, _ := m.Update(loaded)
			m = updated.(model)
			if m.err != nil || !m.event() {
				t.Fatalf("event not loaded: %v", m.err)
			}
			press := func(m model, key rune) clockResultMsg {
				t.Helper()
				_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
				if cmd == nil {
					t.Fatalf("event clock key %c disabled", key)
				}
				return cmd().(clockResultMsg)
			}
			var item *task.Task
			if test.clockView {
				item = m.clockTasks[m.clockCursor]
			} else {
				item = m.flatItems[m.cursor].Task
			}
			if !test.overnight && test.schedule != "" {
				futureAgenda := m
				futureAgenda.focusDate = now.AddDate(0, 0, 1)
				updated, _ := futureAgenda.Update(loadAgendaCmd(cfg, futureAgenda.focusDate, futureAgenda.mode)())
				if result := press(updated.(model), 'i'); result.err == nil {
					t.Fatal("clocked in to a future occurrence")
				}
			}
			started := time.Now().Truncate(time.Minute)
			result := press(m, 'i')
			if test.overnight {
				if result.err == nil || !strings.Contains(result.err.Error(), "today's occurrence") {
					t.Fatalf("unexpected clock-in result: %+v", result)
				}
				updated, _ := m.Update(result)
				if !strings.Contains(updated.View(), "today's occurrence") {
					t.Fatal("clock-in error not shown")
				}
			} else if result.err != nil {
				t.Fatal(result.err)
			}
			entries, err := task.ParseClockEntries(item)
			if err != nil || len(entries) != 1 || !entries[0].Open {
				t.Fatalf("open clocks: %v, %v", entries, err)
			}
			if !test.overnight && (entries[0].Start.Before(started) || entries[0].Start.After(time.Now())) {
				t.Fatalf("clock start %v is not the actual time", entries[0].Start)
			}
			if result := press(m, 'o'); result.err != nil {
				t.Fatal(result.err)
			}
			closed, err := task.ParseClockEntries(item)
			if err != nil || len(closed) != 1 || closed[0].Open || !closed[0].Start.Equal(entries[0].Start) || closed[0].End.Before(closed[0].Start) || closed[0].End.IsZero() {
				t.Fatalf("closed clocks: %v, %v", closed, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := heading + "\n  * CLOCK: " + closed[0].Start.Format("2006-01-02T15:04") + "--" + closed[0].End.Format("2006-01-02T15:04") + "\n  Notes\n"
			if string(data) != want {
				t.Fatalf("file = %q, want %q", data, want)
			}
		})
	}
}
