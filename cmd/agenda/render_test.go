package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/task"
)

func TestRenderingPreservesStyles(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	cfg := &config.Config{Todo: config.Todo{Active: []string{"TODO"}, Events: []string{"MEETING"}}, Colors: config.ColorScheme{TaskColor: "#a89984", EventsColor: "#56b6c2", ActiveColor: "#d79921", DeadlineColor: "#ff00ff", OverdueColor: "#ff0000", CompletedColor: "#888888", ClockActiveColor: "#ffffff"}}
	initColors(cfg)
	m := initialModel(cfg)
	m.termWidth = 160
	for _, test := range []struct {
		state  string
		style  lipgloss.Style
		prefix string
	}{
		{"normal", colors.taskText, ""}, {"overdue", colors.deadline, ""}, {"warning", colors.overdue, ""}, {"deadline", colors.overdue, ""}, {"completed", colors.completed, "✓ "}, {"clock", colors.clockActive, "⏱ "},
	} {
		for _, keyword := range []string{"MEETING", "TODO"} {
			for _, title := range []string{"Plain description", "A **bold** and `code` description"} {
				item := task.AgendaItem{Task: &task.Task{Keyword: keyword, Title: title}, Date: time.Now(), IsOverdue: test.state == "overdue", Warning: test.state == "warning", IsDeadline: test.state == "deadline", IsCompleted: test.state == "completed", ClockActive: test.state == "clock"}
				row := m.renderItem(item, true)
				want := test.style.Render(task.RenderMarkdownDescription(test.prefix+title, test.style))
				if !strings.Contains(row, want) {
					t.Errorf("%s %s: original description style missing: %q", keyword, test.state, row)
				}
				if keyword == "MEETING" && test.state == "clock" && !strings.Contains(row, colors.events.Render("MEETING     ")) {
					t.Errorf("active event keyword lost its dedicated color: %q", row)
				}
			}
		}
	}
}
