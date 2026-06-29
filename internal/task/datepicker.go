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

type pickerSection int

const (
	sectionDate       pickerSection = iota
	sectionTime
	sectionRecurrence
	sectionWarning
)

type timeFocus int

const (
	timeFocusHour timeFocus = iota
	timeFocusMinute
)

type recurrenceFocus int

const (
	recFocusMode recurrenceFocus = iota
	recFocusInterval
	recFocusUnit
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

	// Section navigation
	Section pickerSection

	// Time
	HasTime   bool
	Hour      int
	Minute    int
	TimeFocus timeFocus

	// Recurrence
	HasRecurrence      bool
	RecurrenceMode     RecurrenceMode
	RecurrenceInterval int
	RecurrenceUnit     byte
	RecFocus           recurrenceFocus

	// Warning
	HasWarning  bool
	WarningDays int
}

func NewDatePicker(t *Task, field DatePickerField) *DatePicker {
	dp := &DatePicker{
		Task:               t,
		Field:              field,
		Cursor:             time.Now(),
		Section:            sectionDate,
		RecurrenceInterval: 1,
		RecurrenceUnit:     'w',
		WarningDays:        7,
	}

	var raw string
	switch field {
	case FieldScheduled:
		raw = t.ScheduledAt
	case FieldDue:
		raw = t.DueAt
	}

	if raw != "" {
		if s, err := ParseSchedule(raw); err == nil {
			dp.Cursor = s.Date
			if s.HasTime {
				dp.HasTime = true
				dp.Hour = s.Date.Hour()
				dp.Minute = s.Date.Minute()
			}
			if s.Recurrence != nil {
				dp.HasRecurrence = true
				dp.RecurrenceMode = s.Recurrence.Mode
				dp.RecurrenceInterval = s.Recurrence.Interval
				dp.RecurrenceUnit = s.Recurrence.Unit
			}
			if s.Warning != nil {
				dp.HasWarning = true
				dp.WarningDays = s.Warning.Days
			}
		}
	}

	return dp
}

func (dp *DatePicker) Update(key string) {
	dp.ErrorMsg = ""

	switch key {
	case "esc":
		dp.Cancelled = true
		return
	case "enter":
		dp.Confirmed = true
		return
	case "tab":
		dp.Section = (dp.Section + 1) % 4
		return
	case "shift+tab":
		dp.Section = (dp.Section + 3) % 4
		return
	case "f":
		if dp.Section != sectionDate {
			return
		}
		dp.toggleField()
		return
	}

	switch dp.Section {
	case sectionDate:
		dp.updateDate(key)
	case sectionTime:
		dp.updateTime(key)
	case sectionRecurrence:
		dp.updateRecurrence(key)
	case sectionWarning:
		dp.updateWarning(key)
	}
}

func (dp *DatePicker) toggleField() {
	if dp.Field == FieldScheduled {
		dp.Field = FieldDue
		if !dp.ClearedDue && dp.Task.DueAt != "" {
			dp.loadFromToken(dp.Task.DueAt)
		}
	} else {
		dp.Field = FieldScheduled
		if !dp.ClearedScheduled && dp.Task.ScheduledAt != "" {
			dp.loadFromToken(dp.Task.ScheduledAt)
		}
	}
}

func (dp *DatePicker) loadFromToken(raw string) {
	if s, err := ParseSchedule(raw); err == nil {
		dp.Cursor = s.Date
		dp.HasTime = s.HasTime
		if s.HasTime {
			dp.Hour = s.Date.Hour()
			dp.Minute = s.Date.Minute()
		}
		dp.HasRecurrence = s.Recurrence != nil
		if s.Recurrence != nil {
			dp.RecurrenceMode = s.Recurrence.Mode
			dp.RecurrenceInterval = s.Recurrence.Interval
			dp.RecurrenceUnit = s.Recurrence.Unit
		}
		dp.HasWarning = s.Warning != nil
		if s.Warning != nil {
			dp.WarningDays = s.Warning.Days
		}
	}
}

func (dp *DatePicker) updateDate(key string) {
	switch key {
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

func (dp *DatePicker) updateTime(key string) {
	switch key {
	case "backspace":
		dp.HasTime = false
		dp.Hour = 0
		dp.Minute = 0
		return
	case "left", "h":
		dp.TimeFocus = timeFocusHour
		return
	case "right", "l":
		dp.TimeFocus = timeFocusMinute
		return
	case "up", "k":
		dp.HasTime = true
		if dp.TimeFocus == timeFocusHour {
			dp.Hour = (dp.Hour + 1) % 24
		} else {
			dp.Minute = (dp.Minute + 5) % 60
		}
		return
	case "down", "j":
		dp.HasTime = true
		if dp.TimeFocus == timeFocusHour {
			dp.Hour = (dp.Hour + 23) % 24
		} else {
			dp.Minute = (dp.Minute + 55) % 60
		}
		return
	}

	// Number key direct entry
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		dp.HasTime = true
		digit := int(key[0] - '0')
		if dp.TimeFocus == timeFocusHour {
			newHour := (dp.Hour%10)*10 + digit
			if newHour > 23 {
				newHour = digit
			}
			dp.Hour = newHour
		} else {
			newMin := (dp.Minute%10)*10 + digit
			if newMin > 59 {
				newMin = digit
			}
			dp.Minute = newMin
		}
	}
}

func (dp *DatePicker) updateRecurrence(key string) {
	switch key {
	case "backspace":
		dp.HasRecurrence = false
		return
	case "left", "h":
		dp.HasRecurrence = true
		if dp.RecFocus > recFocusMode {
			dp.RecFocus--
		}
		return
	case "right", "l":
		dp.HasRecurrence = true
		if dp.RecFocus < recFocusUnit {
			dp.RecFocus++
		}
		return
	case "up", "k":
		dp.HasRecurrence = true
		switch dp.RecFocus {
		case recFocusMode:
			dp.RecurrenceMode = (dp.RecurrenceMode + 1) % 3
		case recFocusInterval:
			dp.RecurrenceInterval++
		case recFocusUnit:
			dp.RecurrenceUnit = nextUnit(dp.RecurrenceUnit)
		}
		return
	case "down", "j":
		dp.HasRecurrence = true
		switch dp.RecFocus {
		case recFocusMode:
			dp.RecurrenceMode = (dp.RecurrenceMode + 2) % 3
		case recFocusInterval:
			if dp.RecurrenceInterval > 1 {
				dp.RecurrenceInterval--
			}
		case recFocusUnit:
			dp.RecurrenceUnit = prevUnit(dp.RecurrenceUnit)
		}
		return
	case "b":
		dp.HasRecurrence = true
		dp.RecurrenceUnit = 'b'
		return
	case "d":
		dp.HasRecurrence = true
		dp.RecurrenceUnit = 'd'
		return
	case "w":
		dp.HasRecurrence = true
		dp.RecurrenceUnit = 'w'
		return
	case "m":
		dp.HasRecurrence = true
		dp.RecurrenceUnit = 'm'
		return
	case "y":
		dp.HasRecurrence = true
		dp.RecurrenceUnit = 'y'
		return
	}

	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		dp.HasRecurrence = true
		dp.RecurrenceInterval = int(key[0] - '0')
	}
}

func (dp *DatePicker) updateWarning(key string) {
	switch key {
	case "backspace":
		dp.HasWarning = false
		dp.WarningDays = 7
		return
	case "up", "k":
		dp.HasWarning = true
		dp.WarningDays++
		return
	case "down", "j":
		dp.HasWarning = true
		if dp.WarningDays > 1 {
			dp.WarningDays--
		}
		return
	}

	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		dp.HasWarning = true
		digit := int(key[0] - '0')
		newDays := (dp.WarningDays%10)*10 + digit
		if newDays == 0 {
			newDays = 1
		}
		if newDays > 99 {
			newDays = digit
			if newDays == 0 {
				newDays = 1
			}
		}
		dp.WarningDays = newDays
	}
}

func nextUnit(u byte) byte {
	switch u {
	case 'd':
		return 'b'
	case 'b':
		return 'w'
	case 'w':
		return 'm'
	case 'm':
		return 'y'
	case 'y':
		return 'd'
	}
	return 'w'
}

func prevUnit(u byte) byte {
	switch u {
	case 'd':
		return 'y'
	case 'b':
		return 'd'
	case 'w':
		return 'b'
	case 'm':
		return 'w'
	case 'y':
		return 'm'
	}
	return 'w'
}

func (dp *DatePicker) unclearActive() {
	if dp.Field == FieldScheduled {
		dp.ClearedScheduled = false
	} else {
		dp.ClearedDue = false
	}
}

func (dp *DatePicker) buildToken() string {
	sched := &Schedule{
		Date:    dp.Cursor,
		HasTime: dp.HasTime,
	}
	if dp.HasTime {
		sched.Date = time.Date(dp.Cursor.Year(), dp.Cursor.Month(), dp.Cursor.Day(),
			dp.Hour, dp.Minute, 0, 0, time.Local)
		sched.HasTime = true
	}
	if dp.HasRecurrence {
		sched.Recurrence = &RecurrenceSpec{
			Mode:     dp.RecurrenceMode,
			Interval: dp.RecurrenceInterval,
			Unit:     dp.RecurrenceUnit,
		}
	}
	if dp.HasWarning {
		sched.Warning = &WarningSpec{Days: dp.WarningDays}
	}
	return sched.FormatToken()
}

func (dp *DatePicker) Result() (scheduledAt, dueAt string, removeScheduled, removeDue bool) {
	token := dp.buildToken()

	if dp.Field == FieldScheduled {
		if dp.ClearedScheduled {
			removeScheduled = dp.Task.ScheduledAt != ""
		} else {
			scheduledAt = token
		}
	} else {
		if dp.ClearedDue {
			removeDue = dp.Task.DueAt != ""
		} else {
			dueAt = token
		}
	}

	if dp.Field != FieldScheduled && dp.ClearedScheduled {
		removeScheduled = dp.Task.ScheduledAt != ""
	}
	if dp.Field != FieldDue && dp.ClearedDue {
		removeDue = dp.Task.DueAt != ""
	}

	return
}

func (dp *DatePicker) View(width int) string {
	if width < 40 {
		width = 40
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(width)

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	clearedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	inactiveStyle := dimStyle
	sectionActiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	sectionInactiveStyle := dimStyle

	b.WriteString(titleStyle.Render("Set Date"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Task: %s", dp.Task.Title)))
	b.WriteString("\n\n")

	// Live token preview for active field
	tokenPreview := dp.buildToken()

	// Scheduled line
	schedDisplay := dp.Task.ScheduledAt
	if schedDisplay == "" {
		schedDisplay = "(none)"
	}
	if dp.Field == FieldScheduled {
		if dp.ClearedScheduled {
			b.WriteString(clearedStyle.Render("> Scheduled: (none)"))
		} else {
			b.WriteString(activeStyle.Render(fmt.Sprintf("> Scheduled: %s", tokenPreview)))
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
	dueDisplay := dp.Task.DueAt
	if dueDisplay == "" {
		dueDisplay = "(none)"
	}
	if dp.Field == FieldDue {
		if dp.ClearedDue {
			b.WriteString(clearedStyle.Render("> Due:       (none)"))
		} else {
			b.WriteString(activeStyle.Render(fmt.Sprintf("> Due:       %s", tokenPreview)))
		}
	} else {
		if dp.ClearedDue {
			b.WriteString(clearedStyle.Render("  Due:       (none)"))
		} else {
			b.WriteString(inactiveStyle.Render(fmt.Sprintf("  Due:       %s", dueDisplay)))
		}
	}
	b.WriteString("\n\n")

	// Date section
	dateLabelStyle := sectionInactiveStyle
	if dp.Section == sectionDate {
		dateLabelStyle = sectionActiveStyle
	}
	b.WriteString(dateLabelStyle.Render("  [Date]"))
	b.WriteString("\n")

	calWidth := 22
	gap := 3
	monthCount := min(3, max(1, (width-6)/(calWidth+gap)))
	b.WriteString(dp.renderCalendars(monthCount))
	b.WriteString("\n\n")

	// Time section
	timeLabelStyle := sectionInactiveStyle
	if dp.Section == sectionTime {
		timeLabelStyle = sectionActiveStyle
	}
	b.WriteString(timeLabelStyle.Render("  [Time]       "))
	if dp.HasTime {
		hourStr := fmt.Sprintf("%02d", dp.Hour)
		minStr := fmt.Sprintf("%02d", dp.Minute)
		if dp.Section == sectionTime && dp.TimeFocus == timeFocusHour {
			b.WriteString(sectionActiveStyle.Render(hourStr))
		} else {
			b.WriteString(dimStyle.Render(hourStr))
		}
		b.WriteString(dimStyle.Render(":"))
		if dp.Section == sectionTime && dp.TimeFocus == timeFocusMinute {
			b.WriteString(sectionActiveStyle.Render(minStr))
		} else {
			b.WriteString(dimStyle.Render(minStr))
		}
	} else {
		b.WriteString(dimStyle.Render("--:--"))
	}
	b.WriteString("\n")

	// Recurrence section
	recLabelStyle := sectionInactiveStyle
	if dp.Section == sectionRecurrence {
		recLabelStyle = sectionActiveStyle
	}
	b.WriteString(recLabelStyle.Render("  [Recurrence] "))
	if dp.HasRecurrence {
		modeStr := recurrenceModeStr(dp.RecurrenceMode)
		intervalStr := fmt.Sprintf("%d", dp.RecurrenceInterval)
		unitStr := string(dp.RecurrenceUnit)
		if dp.Section == sectionRecurrence {
			switch dp.RecFocus {
			case recFocusMode:
				b.WriteString(sectionActiveStyle.Render(modeStr))
				b.WriteString(dimStyle.Render(intervalStr + unitStr))
			case recFocusInterval:
				b.WriteString(dimStyle.Render(modeStr))
				b.WriteString(sectionActiveStyle.Render(intervalStr))
				b.WriteString(dimStyle.Render(unitStr))
			case recFocusUnit:
				b.WriteString(dimStyle.Render(modeStr+intervalStr))
				b.WriteString(sectionActiveStyle.Render(unitStr))
			}
		} else {
			b.WriteString(dimStyle.Render(modeStr + intervalStr + unitStr))
		}
		b.WriteString(dimStyle.Render(fmt.Sprintf("  (%s)", recurrenceModeName(dp.RecurrenceMode))))
	} else {
		b.WriteString(dimStyle.Render("(none)"))
	}
	b.WriteString("\n")

	// Warning section
	warnLabelStyle := sectionInactiveStyle
	if dp.Section == sectionWarning {
		warnLabelStyle = sectionActiveStyle
	}
	b.WriteString(warnLabelStyle.Render("  [Warning]    "))
	if dp.HasWarning {
		warnStr := fmt.Sprintf("!%dd", dp.WarningDays)
		if dp.Section == sectionWarning {
			b.WriteString(sectionActiveStyle.Render(warnStr))
		} else {
			b.WriteString(dimStyle.Render(warnStr))
		}
	} else {
		b.WriteString(dimStyle.Render("(none)"))
	}
	b.WriteString("\n")

	// Error
	if dp.ErrorMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString("\n")
		b.WriteString(errStyle.Render(dp.ErrorMsg))
	}

	// Help
	b.WriteString("\n\n")
	switch dp.Section {
	case sectionDate:
		b.WriteString(dimStyle.Render("arrows: navigate • [/]: month • .: today • f: field"))
	case sectionTime:
		b.WriteString(dimStyle.Render("h/l: hour/min • j/k: adjust • 0-9: type • backspace: clear"))
	case sectionRecurrence:
		b.WriteString(dimStyle.Render("h/l: mode/interval/unit • j/k: adjust • b/d/w/m/y: unit • backspace: clear"))
	case sectionWarning:
		b.WriteString(dimStyle.Render("j/k: adjust days • 0-9: type • backspace: clear"))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("tab/shift+tab: section • enter: confirm • esc: cancel"))

	return boxStyle.Render(b.String())
}

func recurrenceModeStr(mode RecurrenceMode) string {
	switch mode {
	case RecurrenceFromDone:
		return ".+"
	case RecurrenceNextFuture:
		return "++"
	default:
		return "+"
	}
}

func recurrenceModeName(mode RecurrenceMode) string {
	switch mode {
	case RecurrenceFromDone:
		return "from done"
	case RecurrenceNextFuture:
		return "next future"
	default:
		return "fixed"
	}
}

func (dp *DatePicker) renderCalendars(count int) string {
	today := time.Now()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	cursorDate := time.Date(dp.Cursor.Year(), dp.Cursor.Month(), dp.Cursor.Day(), 0, 0, 0, 0, time.Local)

	var allMonthLines [][]string
	baseYear, baseMonth, _ := dp.Cursor.Date()

	for i := range count {
		y, m := shiftMonth(baseYear, baseMonth, i)
		lines := renderMonth(y, m, todayDate, cursorDate)
		allMonthLines = append(allMonthLines, lines)
	}

	maxLines := 0
	for _, lines := range allMonthLines {
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}

	for i := range allMonthLines {
		for len(allMonthLines[i]) < maxLines {
			allMonthLines[i] = append(allMonthLines[i], strings.Repeat(" ", 21))
		}
	}

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

	header := fmt.Sprintf("%s %d", month.String()[:3], year)
	lines = append(lines, headerStyle.Render(fmt.Sprintf(" %-20s", header)))

	lines = append(lines, dimStyle.Render(" Mo Tu We Th Fr Sa Su"))

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

	if col > 0 {
		for col < 7 {
			row.WriteString("   ")
			col++
		}
		lines = append(lines, row.String())
	}

	return lines
}
