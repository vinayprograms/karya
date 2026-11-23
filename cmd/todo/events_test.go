package main

import (
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vinayprograms/karya/internal/task"
)

func TestContainers(t *testing.T) {
	cfg := createTestConfig()
	cfg.Todo.Containers = []string{"EPIC"}
	InitializeColors(cfg)
	items := []*task.Task{{Keyword: "TODO", Title: "work"}, {Keyword: "EPIC", Title: "container"}, {Keyword: "MEETING", Title: "event"}}
	for _, mode := range []viewMode{viewWork, viewContainers} {
		m := model{config: cfg, viewMode: mode, list: list.New(nil, list.NewDefaultDelegate(), 80, 24)}
		updated, _ := m.Update(loadingDoneMsg{tasks: items})
		got := updated.(model)
		want := "TODO"
		if mode == viewContainers {
			want = "EPIC"
		}
		if len(got.tasks) != 1 || got.tasks[0].Keyword != want {
			t.Fatalf("view %v: %+v", mode, got.tasks)
		}
		updated, command := got.Update(tea.KeyMsg{Type: tea.KeyTab})
		if command == nil || updated.(model).viewMode == mode {
			t.Fatal("Tab did not switch views")
		}
	}
	m := model{config: cfg, list: list.New(nil, list.NewDefaultDelegate(), 80, 24)}
	for _, key := range []rune{'2', '1'} {
		updated, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = updated.(model)
		if command == nil || (key == '2' && m.viewMode != viewContainers) || (key == '1' && m.viewMode != viewWork) {
			t.Fatalf("key %c failed", key)
		}
	}
}

func TestEventsReload(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		for _, source := range []string{"load", "watch", "editor"} {
			t.Run(source+map[bool]string{true: "/verbose", false: "/normal"}[verbose], func(t *testing.T) {
				cfg := createTestConfig()
				cfg.Directories.Projects = t.TempDir()
				cfg.Directories.Karya = t.TempDir()
				cfg.GeneralConfig.Verbose = verbose
				if err := os.WriteFile(cfg.GetInboxFilePath(), []byte("TODO: work\nMEETING: one-off @2026-11-23\nMEETING: recurring @2026-11-23+1d\n"), 0600); err != nil {
					t.Fatal(err)
				}
				tasks, err := task.ListTasks(cfg, "", true)
				if err != nil {
					t.Fatal(err)
				}
				m := model{config: cfg, list: list.New(nil, list.NewDefaultDelegate(), 80, 24)}
				var message tea.Msg = loadingDoneMsg{tasks: tasks}
				if source == "watch" {
					message = fileChangedMsg{}
				}
				if source == "editor" {
					message = editorFinishedMsg{}
				}
				updated, _ := m.Update(message)
				got := updated.(model)
				if len(got.tasks) != 1 || got.tasks[0].Keyword != "TODO" || len(got.list.Items()) != 1 {
					t.Fatalf("events leaked into todo: %+v", got.tasks)
				}
			})
		}
	}
}
