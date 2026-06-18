package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type DatePickerField int

const (
	FieldScheduled DatePickerField = iota
	FieldDue
)

type DatePicker struct {
	Task             *Task
	Field            DatePickerField
	Cursor           time.Time
	Confirmed        bool
	Cancelled        bool
	ClearedScheduled bool
	ClearedDue       bool
	ErrorMsg         string
}

func NewDatePicker(t *Task, field DatePickerField) *DatePicker {
	cursor := time.Now()

	switch field {
	case FieldScheduled:
		if t.ScheduledAt != "" {
			if s, err := ParseSchedule(t.ScheduledAt); err == nil {
				cursor = s.Date
			}
		}
	case FieldDue:
		if t.DueAt != "" {
			if s, err := ParseSchedule(t.DueAt); err == nil {
				cursor = s.Date
			}
		}
	}

	return &DatePicker{
		Task:   t,
		Field:  field,
		Cursor: cursor,
	}
}

func (dp *DatePicker) Update(key string) {
	dp.ErrorMsg = ""

	switch key {
	case "esc":
		dp.Cancelled = true
	case "enter":
		dp.Confirmed = true
	case "tab":
		if dp.Field == FieldScheduled {
			dp.Field = FieldDue
			if !dp.ClearedDue && dp.Task.DueAt != "" {
				if s, err := ParseSchedule(dp.Task.DueAt); err == nil {
					dp.Cursor = s.Date
				}
			}
		} else {
			dp.Field = FieldScheduled
			if !dp.ClearedScheduled && dp.Task.ScheduledAt != "" {
				if s, err := ParseSchedule(dp.Task.ScheduledAt); err == nil {
					dp.Cursor = s.Date
				}
			}
		}
	case "backspace":
		if dp.Field == FieldScheduled {
			dp.ClearedScheduled = true
		} else {
			dp.ClearedDue = true
		}
	case "left", "h":
		dp.unclearActive()
		dp.Cursor = dp.Cursor.AddDate(0, 0, -1)
	case "right", "l":
		dp.unclearActive()
		dp.Cursor = dp.Cursor.AddDate(0, 0, 1)
	case "up", "k":
		dp.unclearActive()
		dp.Cursor = dp.Cursor.AddDate(0, 0, -7)
	case "down", "j":
		dp.unclearActive()
		dp.Cursor = dp.Cursor.AddDate(0, 0, 7)
	case "[":
		dp.unclearActive()
		dp.Cursor = dp.Cursor.AddDate(0, -1, 0)
	case "]":
		dp.unclearActive()
		dp.Cursor = dp.Cursor.AddDate(0, 1, 0)
	case ".":
		dp.unclearActive()
		dp.Cursor = time.Now()
	}
}

func (dp *DatePicker) unclearActive() {
	if dp.Field == FieldScheduled {
		dp.ClearedScheduled = false
	} else {
		dp.ClearedDue = false
	}
}

func (dp *DatePicker) Result() (scheduledAt, dueAt string, removeScheduled, removeDue bool) {
	dateStr := dp.Cursor.Format("2006-01-02")

	// Active field: set date or remove
	if dp.Field == FieldScheduled {
		if dp.ClearedScheduled {
			removeScheduled = dp.Task.ScheduledAt != ""
		} else {
			scheduledAt = dateStr
		}
	} else {
		if dp.ClearedDue {
			removeDue = dp.Task.DueAt != ""
		} else {
			dueAt = dateStr
		}
	}

	// Inactive field: apply removal if it was cleared
	if dp.Field != FieldScheduled && dp.ClearedScheduled {
		removeScheduled = dp.Task.ScheduledAt != ""
	}
	if dp.Field != FieldDue && dp.ClearedDue {
		removeDue = dp.Task.DueAt != ""
	}

	return
}

func (dp *DatePicker) View(width int) string {
	if width < 30 {
		width = 30
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(width)

	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	b.WriteString(titleStyle.Render("Set Date"))
	b.WriteString("\n")

	// Task info
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("Task: %s", dp.Task.Title)))
	b.WriteString("\n\n")

	// Field selector — active field shows cursor date or (none) if cleared
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	clearedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	inactiveStyle := dimStyle
	cursorDateStr := dp.Cursor.Format("2006-01-02")

	schedDisplay := dp.Task.ScheduledAt
	if schedDisplay == "" {
		schedDisplay = "(none)"
	}
	dueDisplay := dp.Task.DueAt
	if dueDisplay == "" {
		dueDisplay = "(none)"
	}

	// Scheduled line
	if dp.Field == FieldScheduled {
		if dp.ClearedScheduled {
			b.WriteString(clearedStyle.Render("> Scheduled: (none)"))
		} else {
			b.WriteString(activeStyle.Render(fmt.Sprintf("> Scheduled: %s", cursorDateStr)))
		}
	} else {
		if dp.ClearedScheduled {
			b.WriteString(clearedStyle.Render("  Scheduled: (none)"))
		} else {
			b.WriteString(inactiveStyle.Render(fmt.Sprintf("  Scheduled: %s", schedDisplay)))
		}
	}
	b.WriteString("\n")

	// Due line
	if dp.Field == FieldDue {
		if dp.ClearedDue {
			b.WriteString(clearedStyle.Render("> Due:       (none)"))
		} else {
			b.WriteString(activeStyle.Render(fmt.Sprintf("> Due:       %s", cursorDateStr)))
		}
	} else {
		if dp.ClearedDue {
			b.WriteString(clearedStyle.Render("  Due:       (none)"))
		} else {
			b.WriteString(inactiveStyle.Render(fmt.Sprintf("  Due:       %s", dueDisplay)))
		}
	}
	b.WriteString("\n\n")

	// Calendar — show as many months as fit side-by-side
	calWidth := 22 // one month is ~22 chars wide (3*7 + 1)
	gap := 3
	monthCount := (width - 6) / (calWidth + gap) // subtract border+padding
	if monthCount < 1 {
		monthCount = 1
	}
	if monthCount > 3 {
		monthCount = 3
	}

	b.WriteString(dp.renderCalendars(monthCount))
	b.WriteString("\n")

	// Error
	if dp.ErrorMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString(errStyle.Render(dp.ErrorMsg))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("arrows: navigate • [/]: month • .: today"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("tab: switch field • backspace: clear • enter: confirm • esc: cancel"))

	return boxStyle.Render(b.String())
}

func (dp *DatePicker) renderCalendars(count int) string {
	today := time.Now()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	cursorDate := time.Date(dp.Cursor.Year(), dp.Cursor.Month(), dp.Cursor.Day(), 0, 0, 0, 0, time.Local)

	// Render each month into lines, then join side-by-side
	var allMonthLines [][]string
	baseYear, baseMonth, _ := dp.Cursor.Date()

	for i := 0; i < count; i++ {
		monthOffset := i - 0 // start from cursor's month
		y, m := shiftMonth(baseYear, baseMonth, monthOffset)
		lines := renderMonth(y, m, todayDate, cursorDate)
		allMonthLines = append(allMonthLines, lines)
	}

	// Determine max lines across months
	maxLines := 0
	for _, lines := range allMonthLines {
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}

	// Pad shorter months
	for i := range allMonthLines {
		for len(allMonthLines[i]) < maxLines {
			allMonthLines[i] = append(allMonthLines[i], strings.Repeat(" ", 21))
		}
	}

	// Join side-by-side
	gap := "   "
	var result strings.Builder
	for row := 0; row < maxLines; row++ {
		for col, lines := range allMonthLines {
			if col > 0 {
				result.WriteString(gap)
			}
			result.WriteString(lines[row])
		}
		if row < maxLines-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func shiftMonth(year int, month time.Month, offset int) (int, time.Month) {
	m := int(month) + offset
	for m > 12 {
		m -= 12
		year++
	}
	for m < 1 {
		m += 12
		year--
	}
	return year, time.Month(m)
}

func renderMonth(year int, month time.Month, todayDate, cursorDate time.Time) []string {
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	todayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("14")).Bold(true)

	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	lastDay := firstOfMonth.AddDate(0, 1, -1).Day()

	var lines []string

	// Month header (padded to 21 chars)
	header := fmt.Sprintf("%s %d", month.String()[:3], year)
	lines = append(lines, headerStyle.Render(fmt.Sprintf(" %-20s", header)))

	// Weekday headers
	lines = append(lines, dimStyle.Render(" Mo Tu We Th Fr Sa Su"))

	// Offset for first day (Monday=0)
	offset := int(firstOfMonth.Weekday()) - 1
	if offset < 0 {
		offset = 6
	}

	var row strings.Builder
	row.WriteString(" ")
	for i := 0; i < offset; i++ {
		row.WriteString("   ")
	}

	col := offset
	for day := 1; day <= lastDay; day++ {
		d := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		cell := fmt.Sprintf("%2d", day)

		if d.Equal(cursorDate) {
			row.WriteString(selectedStyle.Render(cell))
		} else if d.Equal(todayDate) {
			row.WriteString(todayStyle.Render(cell))
		} else {
			row.WriteString(cell)
		}
		row.WriteString(" ")
		col++

		if col == 7 {
			lines = append(lines, row.String())
			row.Reset()
			row.WriteString(" ")
			col = 0
		}
	}

	// Flush last partial row
	if col > 0 {
		for col < 7 {
			row.WriteString("   ")
			col++
		}
		lines = append(lines, row.String())
	}

	return lines
}
