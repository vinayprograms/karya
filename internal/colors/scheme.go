package colors

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vinayprograms/karya/internal/config"
)

// Scheme holds the lipgloss styles for every configurable UI element.
// Build one with Styles; commands pick the fields they need.
type Scheme struct {
	Project        lipgloss.Style
	Active         lipgloss.Style
	InProgress     lipgloss.Style
	Completed      lipgloss.Style
	Someday        lipgloss.Style
	Task           lipgloss.Style
	CompletedTask  lipgloss.Style
	Tag            lipgloss.Style
	SpecialTag     lipgloss.Style
	Date           lipgloss.Style
	PastDate       lipgloss.Style
	TodayDate      lipgloss.Style
	Assignee       lipgloss.Style
	Cycle          lipgloss.Style
	Overdue        lipgloss.Style
	Deadline       lipgloss.Style
	ClockActive    lipgloss.Style
	AgendaHeader   lipgloss.Style
	ChildConnector lipgloss.Style
	PendingChild   lipgloss.Style
	Highlight      lipgloss.Style
	Selector       lipgloss.Style
	Error          lipgloss.Style
	DimText        lipgloss.Style
	Normal         lipgloss.Style
}

func fg(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

func fgbg(fgColor, bgColor string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fgColor)).Background(lipgloss.Color(bgColor))
}

// Styles builds the full style scheme from the configured colors.
func Styles(cfg *config.Config) Scheme {
	c := cfg.Colors
	return Scheme{
		Project:        fg(c.ProjectColor),
		Active:         fg(c.ActiveColor),
		InProgress:     fg(c.InProgressColor),
		Completed:      fg(c.CompletedColor),
		Someday:        fg(c.SomedayColor),
		Task:           fg(c.TaskColor),
		CompletedTask:  fg(c.CompletedTaskColor),
		Tag:            fgbg(c.TagColor, c.TagBgColor),
		SpecialTag:     fgbg(c.SpecialTagColor, c.SpecialTagBgColor).Bold(true),
		Date:           fgbg(c.DateColor, c.DateBgColor),
		PastDate:       fgbg(c.PastDateColor, c.PastDateBgColor),
		TodayDate:      fgbg(c.TodayDateColor, c.TodayDateBgColor).Bold(true),
		Assignee:       fgbg(c.AssigneeColor, c.AssigneeBgColor).Bold(true),
		Cycle:          fgbg(c.CycleColor, c.CycleBgColor).Bold(true),
		Overdue:        fg(c.OverdueColor).Bold(true),
		Deadline:       fg(c.DeadlineColor).Bold(true),
		ClockActive:    fg(c.ClockActiveColor).Bold(true),
		AgendaHeader:   fg(c.AgendaHeaderColor).Bold(true),
		ChildConnector: fg("8"),
		PendingChild:   fg(c.InProgressColor),
		Highlight:      fg(c.ActiveColor).Bold(true),
		Selector:       fg("13").Bold(true),
		Error:          fg("1"),
		DimText:        fg("241"),
		Normal:         lipgloss.NewStyle(),
	}
}
