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
	for _, key := range []string{"i", "o", "t", "enter"} {
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
