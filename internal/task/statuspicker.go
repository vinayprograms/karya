package task

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vinayprograms/karya/internal/config"
)

type statusColumn struct {
	Title    string
	Color    string
	Keywords []string
	Cursor   int
}

type StatusPicker struct {
	Task      *Task
	Config    *config.Config
	Columns   []statusColumn
	ColCursor int
	Confirmed bool
	Cancelled bool
	Selected  string
}

func NewStatusPicker(t *Task, c *config.Config) *StatusPicker {
	sp := &StatusPicker{
		Task:   t,
		Config: c,
	}

	type catDef struct {
		title    string
		color    string
		keywords []string
	}

	cats := []catDef{
		{"Active", c.Colors.ActiveColor, c.Todo.Active},
		{"InProgress", c.Colors.InProgressColor, c.Todo.InProgress},
		{"Completed", c.Colors.CompletedColor, c.Todo.Completed},
		{"Someday", c.Colors.SomedayColor, c.Todo.Someday},
	}

	for _, cat := range cats {
		if len(cat.keywords) == 0 {
			continue
		}
		sp.Columns = append(sp.Columns, statusColumn{
			Title:    cat.title,
			Color:    cat.color,
			Keywords: cat.keywords,
		})
	}

	// Pre-select column and row matching current keyword
	for ci, col := range sp.Columns {
		for ki, kw := range col.Keywords {
			if kw == t.Keyword {
				sp.ColCursor = ci
				sp.Columns[ci].Cursor = ki
				return sp
			}
		}
	}

	return sp
}

func (sp *StatusPicker) Update(key string) {
	if sp.Confirmed || sp.Cancelled {
		return
	}

	switch key {
	case "j", "down":
		col := &sp.Columns[sp.ColCursor]
		col.Cursor++
		if col.Cursor >= len(col.Keywords) {
			col.Cursor = 0
		}
	case "k", "up":
		col := &sp.Columns[sp.ColCursor]
		col.Cursor--
		if col.Cursor < 0 {
			col.Cursor = len(col.Keywords) - 1
		}
	case "h", "left":
		if sp.ColCursor > 0 {
			sp.ColCursor--
		}
	case "l", "right":
		if sp.ColCursor < len(sp.Columns)-1 {
			sp.ColCursor++
		}
	case "enter":
		col := sp.Columns[sp.ColCursor]
		sp.Selected = col.Keywords[col.Cursor]
		sp.Confirmed = true
	case "esc":
		sp.Cancelled = true
	}
}

func (sp *StatusPicker) View(width int) string {
	if len(sp.Columns) == 0 {
		return ""
	}

	colWidth := width / len(sp.Columns)
	if colWidth < 16 {
		colWidth = 16
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)

	var rendered []string
	for ci, col := range sp.Columns {
		isActiveCol := ci == sp.ColCursor

		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(col.Color)).
			Bold(true)

		var b strings.Builder

		// Header
		header := col.Title
		if isActiveCol {
			header = "┃ " + header
		} else {
			header = "  " + header
		}
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		if isActiveCol {
			b.WriteString(headerStyle.Render("┃ " + strings.Repeat("─", len(col.Title))))
		} else {
			b.WriteString(dimStyle.Render("  " + strings.Repeat("─", len(col.Title))))
		}
		b.WriteString("\n")

		// Keywords
		for ki, kw := range col.Keywords {
			var line string
			if isActiveCol && ki == col.Cursor {
				line = fmt.Sprintf("┃ %s %s", cursorStyle.Render("█"), headerStyle.Render(kw))
			} else if isActiveCol {
				line = fmt.Sprintf("┃   %s", dimStyle.Render(kw))
			} else if ki == col.Cursor {
				line = fmt.Sprintf("    %s", dimStyle.Render(kw))
			} else {
				line = fmt.Sprintf("    %s", dimStyle.Render(kw))
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		colStyle := lipgloss.NewStyle().Width(colWidth)
		rendered = append(rendered, colStyle.Render(b.String()))
	}

	columns := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)

	// Task info + columns + help
	var view strings.Builder
	if sp.Task != nil {
		taskInfo := dimStyle.Render(fmt.Sprintf("Task: %s", sp.Task.Title))
		view.WriteString(taskInfo)
		view.WriteString("\n\n")
	}
	view.WriteString(columns)
	view.WriteString("\n")
	view.WriteString(dimStyle.Render("h/l: column • j/k: select • enter: confirm • esc: cancel"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	return boxStyle.Render(view.String())
}
