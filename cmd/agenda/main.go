package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/task"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
)

type viewMode int

const (
	viewDay viewMode = iota
	viewWeek
	viewFortnight
	viewMonth
	viewYear
)

func (v viewMode) String() string {
	switch v {
	case viewDay:
		return "Day"
	case viewWeek:
		return "Week"
	case viewFortnight:
		return "Fortnight"
	case viewMonth:
		return "Month"
	case viewYear:
		return "Year"
	}
	return ""
}

// Colors
type colorScheme struct {
	project    lipgloss.Style
	active     lipgloss.Style
	inProgress lipgloss.Style
	completed  lipgloss.Style
	someday    lipgloss.Style
	taskText   lipgloss.Style
	tag        lipgloss.Style
	specialTag lipgloss.Style
	date       lipgloss.Style
	overdue    lipgloss.Style
	deadline   lipgloss.Style
	assignee   lipgloss.Style
	header     lipgloss.Style
	schedInfo  lipgloss.Style
	dimText    lipgloss.Style
}

var colors colorScheme

func initColors(cfg *config.Config) {
	colors = colorScheme{
		project:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		active:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		inProgress: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.InProgressColor)),
		completed:  lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedColor)),
		someday:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.SomedayColor)),
		taskText:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TaskColor)),
		tag:        lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TagColor)).Background(lipgloss.Color(cfg.Colors.TagBgColor)),
		specialTag: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.SpecialTagColor)).Background(lipgloss.Color(cfg.Colors.SpecialTagBgColor)).Bold(true),
		date:       lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.DateColor)).Background(lipgloss.Color(cfg.Colors.DateBgColor)),
		overdue:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.OverdueColor)),
		deadline:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.OverdueColor)).Bold(true),
		assignee:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.AssigneeColor)).Background(lipgloss.Color(cfg.Colors.AssigneeBgColor)).Bold(true),
		header:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.AgendaHeaderColor)).Bold(true),
		schedInfo:  lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		dimText:    lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	}
}

type model struct {
	config            *config.Config
	mode              viewMode
	focusDate         time.Time
	days              []task.AgendaDay
	flatItems         []task.AgendaItem
	cursorToLine      []int // maps cursor index -> line index in rendered output
	totalLines        int
	cursor            int
	scrollOffset      int
	termWidth         int
	termHeight        int
	quitting          bool
	showingDetailView bool
	showingClockView  bool
	showingDatePicker bool
	datePicker        *task.DatePicker
	selectedTask      *task.Task
	watcher           *fsnotify.Watcher
	err               error
	statusMessage     string

	// Clock table view state
	clockTable        *task.ClockTable
	clockCursor       int
	clockScrollOffset int
	clockCursorToLine []int
	clockTotalLines   int
	clockTasks        []*task.Task // selectable task entries in clock table
}

type fileChangedMsg struct{}

type agendaLoadedMsg struct {
	days []task.AgendaDay
	err  error
}

type clockTableLoadedMsg struct {
	table *task.ClockTable
	err   error
}

type clockResultMsg struct {
	message string
	err     error
}

func initialModel(cfg *config.Config) model {
	m := model{
		config:    cfg,
		focusDate: time.Now(),
	}

	switch cfg.Schedule.DefaultView {
	case "day":
		m.mode = viewDay
	case "fortnight":
		m.mode = viewFortnight
	case "month":
		m.mode = viewMonth
	default:
		m.mode = viewWeek
	}

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		loadAgendaCmd(m.config, m.focusDate, m.mode),
		waitForFileChange(m.watcher),
	)
}

func waitForFileChange(watcher *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		if watcher == nil {
			return nil
		}
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return nil
				}
				if event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Remove == fsnotify.Remove {
					time.Sleep(100 * time.Millisecond)
					return fileChangedMsg{}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return nil
				}
			}
		}
	}
}

func loadAgendaCmd(cfg *config.Config, focus time.Time, mode viewMode) tea.Cmd {
	return func() tea.Msg {
		start, end := viewRange(focus, mode, cfg)
		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		includeOverdue := !today.Before(start) && !today.After(end)
		days, err := task.QueryAgenda(cfg, start, end, includeOverdue)
		return agendaLoadedMsg{days: days, err: err}
	}
}

func loadClockTableCmd(cfg *config.Config, focus time.Time, mode viewMode) tea.Cmd {
	return func() tea.Msg {
		start, end := viewRange(focus, mode, cfg)
		table, err := task.QueryClockTable(cfg, start, end)
		return clockTableLoadedMsg{table: table, err: err}
	}
}

func viewRange(focus time.Time, mode viewMode, cfg *config.Config) (time.Time, time.Time) {
	day := time.Date(focus.Year(), focus.Month(), focus.Day(), 0, 0, 0, 0, time.Local)
	switch mode {
	case viewDay:
		return day, day
	case viewWeek:
		start := weekStart(day, cfg.Schedule.WeekStart)
		return start, start.AddDate(0, 0, 6)
	case viewFortnight:
		start := weekStart(day, cfg.Schedule.WeekStart)
		return start, start.AddDate(0, 0, 13)
	case viewMonth:
		start := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 1, -1)
		return start, end
	case viewYear:
		start := time.Date(day.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		end := time.Date(day.Year(), 12, 31, 0, 0, 0, 0, time.Local)
		return start, end
	}
	return day, day.AddDate(0, 0, 6)
}

func weekStart(d time.Time, startDay string) time.Time {
	target := time.Monday
	if strings.ToLower(startDay) == "sunday" {
		target = time.Sunday
	}
	offset := int(d.Weekday()) - int(target)
	if offset < 0 {
		offset += 7
	}
	return d.AddDate(0, 0, -offset)
}

type datePickerResultMsg struct {
	message string
	err     error
}

type clearStatusMsg struct{}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Date picker mode
	if m.showingDatePicker {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			m.datePicker.Update(msg.String())
			if m.datePicker.Cancelled {
				m.showingDatePicker = false
				m.datePicker = nil
				m.selectedTask = nil
				return m, nil
			}
			if m.datePicker.Confirmed {
				return m, applyDatePickerCmd(m.datePicker)
			}
			return m, nil
		case datePickerResultMsg:
			m.showingDatePicker = false
			m.datePicker = nil
			m.selectedTask = nil
			if msg.err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
			} else {
				m.statusMessage = msg.message
			}
			return m, tea.Batch(
				loadAgendaCmd(m.config, m.focusDate, m.mode),
				tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
			)
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil
		}
		return m, nil
	}

	// Detail view mode
	if m.showingDetailView {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc", "q", "v":
				m.showingDetailView = false
				m.selectedTask = nil
				return m, nil
			}
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil
		case fileChangedMsg:
			// Re-render picks up fresh content via ReadRawBlock
			return m, waitForFileChange(m.watcher)
		}
		return m, nil
	}

	// Clock view mode
	if m.showingClockView {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c", "q":
				m.quitting = true
				return m, tea.Quit
			case "c", "esc", "a":
				m.showingClockView = false
				return m, nil

			// View switching (reload clock table for new view)
			case "d":
				m.mode = viewDay
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "w":
				m.mode = viewWeek
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "t":
				m.mode = viewFortnight
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "m":
				m.mode = viewMonth
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "y":
				m.mode = viewYear
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)

			// Navigation
			case "f", "l", "right":
				m.focusDate = advanceFocus(m.focusDate, m.mode, 1)
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "b", "h", "left":
				m.focusDate = advanceFocus(m.focusDate, m.mode, -1)
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case ".", " ":
				m.focusDate = time.Now()
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)

			// Cursor
			case "j", "down":
				if m.clockCursor < len(m.clockTasks)-1 {
					m.clockCursor++
					m.ensureClockVisible()
				} else {
					contentHeight := max(5, m.termHeight-4)
					if m.clockScrollOffset+contentHeight < m.clockTotalLines {
						m.clockScrollOffset++
					}
				}
			case "k", "up":
				if m.clockCursor > 0 {
					m.clockCursor--
					m.ensureClockVisible()
				} else if m.clockScrollOffset > 0 {
					m.clockScrollOffset--
				}

			// Half-page scroll
			case "ctrl+d":
				half := max(1, (m.termHeight-4)/2)
				m.clockScrollOffset = min(m.clockScrollOffset+half, max(0, m.clockTotalLines-(m.termHeight-4)))
				if m.clockCursor < len(m.clockCursorToLine) {
					for m.clockCursor < len(m.clockTasks)-1 && m.clockCursorToLine[m.clockCursor] < m.clockScrollOffset {
						m.clockCursor++
					}
				}
			case "ctrl+u":
				half := max(1, (m.termHeight-4)/2)
				m.clockScrollOffset = max(0, m.clockScrollOffset-half)
				if m.clockCursor < len(m.clockCursorToLine) {
					contentHeight := max(5, m.termHeight-4)
					for m.clockCursor > 0 && m.clockCursorToLine[m.clockCursor] >= m.clockScrollOffset+contentHeight {
						m.clockCursor--
					}
				}

			// Detail view
			case "v":
				if m.clockCursor < len(m.clockTasks) {
					m.selectedTask = m.clockTasks[m.clockCursor]
					m.showingDetailView = true
				}
				return m, nil

			// Open editor
			case "enter":
				if m.clockCursor < len(m.clockTasks) {
					return m, openEditorCmd(m.config, m.clockTasks[m.clockCursor])
				}

			// Clock in/out
			case "i":
				if m.clockCursor < len(m.clockTasks) {
					return m, clockInAgendaCmd(m.clockTasks[m.clockCursor])
				}
			case "o":
				if m.clockCursor < len(m.clockTasks) {
					return m, clockOutAgendaCmd(m.clockTasks[m.clockCursor])
				}
			}

		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height

		case clockTableLoadedMsg:
			if msg.err != nil {
				m.err = msg.err
			} else {
				m.clockTable = msg.table
				m.buildClockLineMapping()
				if m.clockCursor >= len(m.clockTasks) {
					m.clockCursor = max(0, len(m.clockTasks)-1)
				}
			}

		case fileChangedMsg:
			return m, tea.Batch(
				loadClockTableCmd(m.config, m.focusDate, m.mode),
				waitForFileChange(m.watcher),
			)

		case editorFinishedMsg:
			return m, tea.Batch(
				loadClockTableCmd(m.config, m.focusDate, m.mode),
				waitForFileChange(m.watcher),
			)

		case clockResultMsg:
			// Reload after clock operation
			return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
		}

		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		// View switching
		case "d":
			m.mode = viewDay
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "w":
			m.mode = viewWeek
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "t":
			m.mode = viewFortnight
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "m":
			m.mode = viewMonth
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "y":
			m.mode = viewYear
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)

		// Navigation
		case "f", "l", "right":
			m.focusDate = advanceFocus(m.focusDate, m.mode, 1)
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "b", "h", "left":
			m.focusDate = advanceFocus(m.focusDate, m.mode, -1)
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case ".", " ":
			m.focusDate = time.Now()
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)

		// Clock view toggle
		case "c":
			m.showingClockView = true
			m.clockCursor = 0
			m.clockScrollOffset = 0
			return m, loadClockTableCmd(m.config, m.focusDate, m.mode)

		// Cursor
		case "j", "down":
			if m.cursor < len(m.flatItems)-1 {
				m.cursor++
				m.ensureVisible()
			} else {
				contentHeight := max(5, m.termHeight-4)
				if m.scrollOffset+contentHeight < m.totalLines {
					m.scrollOffset++
				}
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			} else if m.scrollOffset > 0 {
				m.scrollOffset--
			}

		// Half-page scroll
		case "ctrl+d":
			half := max(1, (m.termHeight-4)/2)
			m.scrollOffset = min(m.scrollOffset+half, max(0, m.totalLines-(m.termHeight-4)))
			if m.cursor < len(m.cursorToLine) {
				for m.cursor < len(m.flatItems)-1 && m.cursorToLine[m.cursor] < m.scrollOffset {
					m.cursor++
				}
			}
		case "ctrl+u":
			half := max(1, (m.termHeight-4)/2)
			m.scrollOffset = max(0, m.scrollOffset-half)
			if m.cursor < len(m.cursorToLine) {
				contentHeight := max(5, m.termHeight-4)
				for m.cursor > 0 && m.cursorToLine[m.cursor] >= m.scrollOffset+contentHeight {
					m.cursor--
				}
			}

		// Detail view
		case "v":
			if m.cursor < len(m.flatItems) {
				m.selectedTask = m.flatItems[m.cursor].Task
				m.showingDetailView = true
			}
			return m, nil

		// Schedule/Due date picker
		case "S":
			if m.cursor < len(m.flatItems) {
				t := m.flatItems[m.cursor].Task
				m.selectedTask = t
				m.datePicker = task.NewDatePicker(t, task.FieldScheduled)
				m.showingDatePicker = true
				return m, nil
			}
		case "D":
			if m.cursor < len(m.flatItems) {
				t := m.flatItems[m.cursor].Task
				m.selectedTask = t
				m.datePicker = task.NewDatePicker(t, task.FieldDue)
				m.showingDatePicker = true
				return m, nil
			}

		// Clock in/out
		case "i":
			if m.cursor < len(m.flatItems) {
				return m, clockInAgendaCmd(m.flatItems[m.cursor].Task)
			}
		case "o":
			if m.cursor < len(m.flatItems) {
				return m, clockOutAgendaCmd(m.flatItems[m.cursor].Task)
			}

		// Open editor
		case "enter":
			if m.cursor < len(m.flatItems) {
				item := m.flatItems[m.cursor]
				return m, openEditorCmd(m.config, item.Task)
			}
		}

	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height

	case agendaLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.days = msg.days
			if m.mode == viewDay {
				m.flatItems = flattenDayItems(msg.days)
			} else {
				m.flatItems = flattenItems(msg.days)
			}
			m.buildLineMapping()
			if m.cursor >= len(m.flatItems) {
				m.cursor = max(0, len(m.flatItems)-1)
			}
		}

	case fileChangedMsg:
		return m, tea.Batch(
			loadAgendaCmd(m.config, m.focusDate, m.mode),
			waitForFileChange(m.watcher),
		)

	case editorFinishedMsg:
		return m, tea.Batch(
			loadAgendaCmd(m.config, m.focusDate, m.mode),
			waitForFileChange(m.watcher),
		)

	case clockResultMsg:
		// Reload after clock operation
		return m, loadAgendaCmd(m.config, m.focusDate, m.mode)

	case clearStatusMsg:
		m.statusMessage = ""
		return m, nil
	}

	return m, nil
}

func applyDatePickerCmd(dp *task.DatePicker) tea.Cmd {
	return func() tea.Msg {
		scheduledAt, dueAt, removeScheduled, removeDue := dp.Result()
		if err := task.SetTaskDate(dp.Task, scheduledAt, dueAt, removeScheduled, removeDue); err != nil {
			return datePickerResultMsg{err: err}
		}
		var msg string
		if removeScheduled || removeDue {
			msg = "Date removed"
		} else if scheduledAt != "" {
			msg = fmt.Sprintf("Scheduled: %s", scheduledAt)
		} else {
			msg = fmt.Sprintf("Due: %s", dueAt)
		}
		return datePickerResultMsg{message: msg}
	}
}

func (m *model) ensureVisible() {
	contentHeight := m.termHeight - 4
	if contentHeight < 5 {
		contentHeight = 5
	}
	if m.cursor < 0 || m.cursor >= len(m.cursorToLine) {
		return
	}
	targetLine := m.cursorToLine[m.cursor]
	if targetLine < m.scrollOffset {
		m.scrollOffset = targetLine
	}
	if targetLine >= m.scrollOffset+contentHeight {
		m.scrollOffset = targetLine - contentHeight + 1
	}
}

func advanceFocus(focus time.Time, mode viewMode, direction int) time.Time {
	switch mode {
	case viewDay:
		return focus.AddDate(0, 0, direction)
	case viewWeek:
		return focus.AddDate(0, 0, 7*direction)
	case viewFortnight:
		return focus.AddDate(0, 0, 14*direction)
	case viewMonth:
		return focus.AddDate(0, direction, 0)
	case viewYear:
		return focus.AddDate(direction, 0, 0)
	}
	return focus
}

func flattenItems(days []task.AgendaDay) []task.AgendaItem {
	var items []task.AgendaItem
	for _, day := range days {
		items = append(items, day.Items...)
	}
	return items
}

// flattenDayItems reorders items for day view: untimed first, then timed sorted by start time.
// This matches renderDayTimeGrid's output order so cursor indices align.
func flattenDayItems(days []task.AgendaDay) []task.AgendaItem {
	var all []task.AgendaItem
	for _, day := range days {
		all = append(all, day.Items...)
	}

	var timed, untimed []task.AgendaItem
	for _, item := range all {
		if item.HasTime {
			timed = append(timed, item)
		} else {
			untimed = append(untimed, item)
		}
	}

	// If no timed items, keep original order
	if len(timed) == 0 {
		return all
	}

	var result []task.AgendaItem
	result = append(result, untimed...)
	result = append(result, timed...)
	return result
}

func (m *model) buildLineMapping() {
	start, end := viewRange(m.focusDate, m.mode, m.config)

	dayItemMap := make(map[time.Time][]task.AgendaItem)
	for _, day := range m.days {
		dayItemMap[day.Date] = day.Items
	}

	var cursorToLine []int
	lineIdx := 0

	if m.mode == viewDay {
		items := dayItemMap[start]
		if len(items) == 0 {
			lineIdx++ // "No scheduled items" line
		} else {
			lineIdx = m.buildDayGridLineMapping(items, &cursorToLine, lineIdx)
		}
	} else {
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			items := dayItemMap[d]
			lineIdx++ // day header
			for range items {
				cursorToLine = append(cursorToLine, lineIdx)
				lineIdx++
			}
		}
	}

	m.cursorToLine = cursorToLine
	m.totalLines = lineIdx
}

// buildDayGridLineMapping mirrors renderDayTimeGrid's output structure for cursor mapping.
func (m *model) buildDayGridLineMapping(items []task.AgendaItem, cursorToLine *[]int, lineIdx int) int {
	var timed []task.AgendaItem
	var untimed []task.AgendaItem

	for _, item := range items {
		if item.HasTime {
			timed = append(timed, item)
		} else {
			untimed = append(untimed, item)
		}
	}

	if len(timed) == 0 {
		for range items {
			*cursorToLine = append(*cursorToLine, lineIdx)
			lineIdx++
		}
		return lineIdx
	}

	// Untimed items first
	for range untimed {
		*cursorToLine = append(*cursorToLine, lineIdx)
		lineIdx++
	}
	if len(untimed) > 0 {
		lineIdx++ // separator line
	}

	// Compute same grid boundaries as renderDayTimeGrid
	earliestHour := timed[0].Date.Hour()
	latestHour := timed[0].Date.Hour()
	for _, item := range timed {
		h := item.Date.Hour()
		if h < earliestHour {
			earliestHour = h
		}
		endHour := h
		if item.HasEnd {
			endHour = item.EndTime.Hour()
			if item.EndTime.Minute() > 0 {
				endHour++
			}
		} else {
			endHour = h + 1
		}
		if endHour > latestHour {
			latestHour = endHour
		}
	}

	gridStart := earliestHour - 2
	if gridStart < 0 {
		gridStart = 0
	}
	gridEnd := latestHour + 2
	if gridEnd > 24 {
		gridEnd = 24
	}

	hourItems := make(map[int]int) // hour -> count of items
	for _, item := range timed {
		hourItems[item.Date.Hour()]++
	}

	for hour := gridStart; hour < gridEnd; hour++ {
		if count, ok := hourItems[hour]; ok {
			for range count {
				*cursorToLine = append(*cursorToLine, lineIdx)
				lineIdx++
			}
		} else {
			lineIdx++ // empty slot line
		}
	}

	return lineIdx
}

func (m *model) ensureClockVisible() {
	contentHeight := m.termHeight - 4
	if contentHeight < 5 {
		contentHeight = 5
	}
	if m.clockCursor < 0 || m.clockCursor >= len(m.clockCursorToLine) {
		return
	}
	targetLine := m.clockCursorToLine[m.clockCursor]
	if targetLine < m.clockScrollOffset {
		m.clockScrollOffset = targetLine
	}
	if targetLine >= m.clockScrollOffset+contentHeight {
		m.clockScrollOffset = targetLine - contentHeight + 1
	}
}

func (m *model) buildClockLineMapping() {
	if m.clockTable == nil {
		m.clockCursorToLine = nil
		m.clockTasks = nil
		m.clockTotalLines = 0
		return
	}

	var cursorToLine []int
	var tasks []*task.Task
	lineIdx := 0

	// Header line (column headers)
	lineIdx++
	// Separator
	lineIdx++
	// Grand total row
	lineIdx++
	// Separator
	lineIdx++

	for _, proj := range m.clockTable.Projects {
		// Project total row
		lineIdx++
		// Task entries
		for _, entry := range proj.Entries {
			cursorToLine = append(cursorToLine, lineIdx)
			tasks = append(tasks, entry.Task)
			lineIdx++
		}
		// Separator after project
		lineIdx++
	}

	m.clockCursorToLine = cursorToLine
	m.clockTasks = tasks
	m.clockTotalLines = lineIdx
}

func clockInAgendaCmd(t *task.Task) tea.Cmd {
	return func() tea.Msg {
		err := task.ClockIn(t)
		if err != nil {
			return clockResultMsg{err: err}
		}
		return clockResultMsg{message: "Clocked in"}
	}
}

func clockOutAgendaCmd(t *task.Task) tea.Cmd {
	return func() tea.Msg {
		err := task.ClockOut(t)
		if err != nil {
			return clockResultMsg{err: err}
		}
		return clockResultMsg{message: "Clocked out"}
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	if m.showingDatePicker && m.datePicker != nil {
		return m.datePicker.View(m.termWidth / 2)
	}

	if m.showingDetailView {
		return m.renderDetailView()
	}

	if m.showingClockView {
		return m.renderClockView()
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	var b strings.Builder

	// Title bar
	start, end := viewRange(m.focusDate, m.mode, m.config)
	_, week := m.focusDate.ISOWeek()
	title := fmt.Sprintf("%s-agenda (W%d):", m.mode.String(), week)
	b.WriteString(colors.header.Render(title))
	b.WriteString("\n")

	// Date range subtitle
	if m.mode == viewDay {
		b.WriteString(colors.dimText.Render(m.focusDate.Format("Monday  2 January 2006")))
	} else {
		b.WriteString(colors.dimText.Render(fmt.Sprintf("%s — %s", start.Format("2 Jan"), end.Format("2 Jan 2006"))))
	}
	b.WriteString("\n")

	// Build day lookup from loaded data
	dayItemMap := make(map[time.Time][]task.AgendaItem)
	for _, day := range m.days {
		dayItemMap[day.Date] = day.Items
	}

	// Render all days in the range
	itemIdx := 0
	var lines []string

	if m.mode == viewDay {
		// Day view: time grid with empty hour slots interspersed
		items := dayItemMap[start]
		if len(items) == 0 {
			lines = append(lines, colors.dimText.Render("  No scheduled items."))
		} else {
			lines, itemIdx = m.renderDayTimeGrid(items, itemIdx)
		}
	} else {
		// Multi-day views: show all days in range
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			items := dayItemMap[d]
			dayHeader := fmt.Sprintf("── %s ──", d.Format("Monday  2 January 2006"))
			if len(items) > 0 {
				lines = append(lines, colors.header.Render(dayHeader))
			} else {
				lines = append(lines, colors.dimText.Render(dayHeader))
			}

			for _, item := range items {
				selected := itemIdx == m.cursor
				lines = append(lines, m.renderItem(item, selected))
				itemIdx++
			}
		}
	}

	// Apply scroll window
	contentHeight := m.termHeight - 4 // 2 header lines + 2 footer lines
	if contentHeight < 5 {
		contentHeight = 5
	}
	visibleStart := m.scrollOffset
	visibleEnd := m.scrollOffset + contentHeight
	if visibleEnd > len(lines) {
		visibleEnd = len(lines)
	}
	if visibleStart > len(lines) {
		visibleStart = len(lines)
	}
	rendered := 0
	for i := visibleStart; i < visibleEnd; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
		rendered++
	}
	// Pad to push footer to bottom
	for i := rendered; i < contentHeight; i++ {
		b.WriteString("\n")
	}

	// Status message
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
		b.WriteString(statusStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	// Footer (anchored to bottom)
	footer := colors.dimText.Render("d/w/t/m/y: view • f/b: navigate • .: today • S/D: schedule/due • c: clock • i/o: clock in/out • v: detail • q: quit")
	b.WriteString(footer)

	return b.String()
}

func (m model) renderItem(item task.AgendaItem, selected bool) string {
	var parts []string
	t := item.Task

	// Selection indicator
	if selected {
		parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true).Render("█ "))
	} else {
		parts = append(parts, "  ")
	}

	// Project column (10 chars)
	proj := t.Project
	if len(proj) > 9 {
		proj = proj[:9]
	}
	parts = append(parts, colors.project.Render(fmt.Sprintf("%-10s", proj+":")))

	// Schedule info column (14 chars)
	schedStr := m.formatScheduleInfo(item)
	schedStyle := colors.schedInfo
	if item.IsOverdue {
		schedStyle = colors.deadline
	} else if item.IsDeadline {
		itemDay := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, time.Local)
		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		if itemDay.Equal(today) || item.Warning {
			schedStyle = colors.overdue
		}
	}
	parts = append(parts, schedStyle.Render(fmt.Sprintf("%-14s", schedStr)))

	// Keyword column
	kwWidth := 12
	var kwStyle lipgloss.Style
	if t.IsInProgress(m.config) {
		kwStyle = colors.inProgress
	} else if t.IsActive(m.config) {
		kwStyle = colors.active
	} else if t.IsSomeday(m.config) {
		kwStyle = colors.someday
	} else {
		kwStyle = colors.completed
	}
	parts = append(parts, kwStyle.Render(fmt.Sprintf("%-*s", kwWidth, t.Keyword)))

	// Title (truncated to fit terminal)
	titleStyle := colors.taskText
	if t.IsCompleted(m.config) {
		titleStyle = colors.completed
	} else if item.IsOverdue {
		titleStyle = colors.deadline
	} else if item.IsDeadline {
		itemDay := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, time.Local)
		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		if itemDay.Equal(today) || item.Warning {
			titleStyle = colors.overdue
		}
	}
	displayTitle := t.Title
	if t.ID != "" {
		displayTitle = fmt.Sprintf("[%s] %s", t.ID, t.Title)
	}
	formattedTitle := task.RenderMarkdownDescription(displayTitle, titleStyle)
	// Fixed columns: selector(2) + project(10) + schedule(14) + keyword(12) + right-side(30)
	maxTitle := m.termWidth - 2 - 10 - 14 - 12 - 30
	if maxTitle < 20 {
		maxTitle = 20
	}
	parts = append(parts, task.TruncateString(formattedTitle, maxTitle))

	// Tags (right side)
	for _, tag := range t.Tags {
		isSpecial := false
		for _, st := range m.config.Todo.SpecialTags {
			if tag == st || strings.HasPrefix(tag, st+":") {
				isSpecial = true
				break
			}
		}
		if isSpecial {
			parts = append(parts, colors.specialTag.Render(fmt.Sprintf(" %s ", tag)))
		} else {
			parts = append(parts, colors.tag.Render(fmt.Sprintf(" %s ", tag)))
		}
	}

	// Assignee
	if t.Assignee != "" {
		parts = append(parts, " ")
		parts = append(parts, colors.assignee.Render(fmt.Sprintf(" %s ", t.Assignee)))
	}

	return strings.Join(parts, "")
}

// renderDayTimeGrid renders day view with hour-resolution time grid.
// Timed items are placed chronologically; empty hour slots fill the gaps.
// Two padding slots are added before the first and after the last timed task,
// except no trailing padding if the last task ends at/near midnight.
func (m model) renderDayTimeGrid(items []task.AgendaItem, startIdx int) ([]string, int) {
	var timed []task.AgendaItem
	var untimed []task.AgendaItem

	for _, item := range items {
		if item.HasTime {
			timed = append(timed, item)
		} else {
			untimed = append(untimed, item)
		}
	}

	var lines []string
	itemIdx := startIdx

	if len(timed) == 0 {
		// No timed items — just render everything flat
		for _, item := range items {
			selected := itemIdx == m.cursor
			lines = append(lines, m.renderItem(item, selected))
			itemIdx++
		}
		return lines, itemIdx
	}

	// Determine grid boundaries from timed items
	earliestHour := timed[0].Date.Hour()
	latestHour := timed[0].Date.Hour()
	for _, item := range timed {
		h := item.Date.Hour()
		if h < earliestHour {
			earliestHour = h
		}
		endHour := h
		if item.HasEnd {
			endHour = item.EndTime.Hour()
			if item.EndTime.Minute() > 0 {
				endHour++
			}
		} else {
			endHour = h + 1
		}
		if endHour > latestHour {
			latestHour = endHour
		}
	}

	// Add 2 padding slots before and after
	gridStart := earliestHour - 2
	if gridStart < 0 {
		gridStart = 0
	}
	gridEnd := latestHour + 2
	// No trailing padding past midnight
	if latestHour >= 24 {
		gridEnd = 24
	} else if gridEnd > 24 {
		gridEnd = 24
	}

	// Build a map: hour -> items starting in that hour
	hourItems := make(map[int][]int) // hour -> indices into timed slice
	for i, item := range timed {
		hourItems[item.Date.Hour()] = append(hourItems[item.Date.Hour()], i)
	}

	// Render untimed items first (overdue, deadlines, all-day)
	for _, item := range untimed {
		selected := itemIdx == m.cursor
		lines = append(lines, m.renderItem(item, selected))
		itemIdx++
	}

	if len(untimed) > 0 {
		lines = append(lines, colors.dimText.Render("  ─────────────────"))
	}

	// Render the time grid
	for hour := gridStart; hour < gridEnd; hour++ {
		if indices, ok := hourItems[hour]; ok {
			for _, idx := range indices {
				item := timed[idx]
				selected := itemIdx == m.cursor
				lines = append(lines, m.renderItem(item, selected))
				itemIdx++
			}
		} else {
			// Empty hour slot
			lines = append(lines, colors.dimText.Render(fmt.Sprintf("  %02d:00 ··················", hour)))
		}
	}

	return lines, itemIdx
}

func (m model) formatScheduleInfo(item task.AgendaItem) string {
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	itemDay := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, time.Local)

	if item.IsOverdue {
		daysAgo := int(math.Round(today.Sub(itemDay).Hours() / 24))
		if daysAgo == 1 {
			return "1 d. ago"
		}
		return fmt.Sprintf("%d d. ago", daysAgo)
	}

	if item.HasTime {
		if item.HasEnd {
			return fmt.Sprintf("%s-%s", item.Date.Format("15:04"), item.EndTime.Format("15:04"))
		}
		return item.Date.Format("15:04")
	}

	if itemDay.Equal(today) {
		if item.IsDeadline {
			return "Due today"
		}
		return "Scheduled"
	}

	// Show relative for items with recurrence that have been rescheduled many times
	if item.Schedule != nil && item.Schedule.Recurrence != nil {
		r := item.Schedule.Recurrence
		orig := item.Schedule.Date
		if !orig.Equal(item.Date) {
			// Calculate how many periods from original
			count := 0
			current := orig
			for current.Before(item.Date) {
				current = task.AddInterval(current, r.Interval, r.Unit)
				count++
			}
			if count > 1 {
				return fmt.Sprintf("Sched.%dx:", count)
			}
		}
	}

	// Future or past date
	if item.IsDeadline {
		return "DL: " + item.Date.Format("Jan 02")
	}
	return item.Date.Format("Jan 02")
}

func (m model) renderDetailView() string {
	if m.selectedTask == nil {
		return ""
	}

	boxWidth := m.termWidth - 4
	if boxWidth < 40 {
		boxWidth = 40
	}
	contentWidth := boxWidth - 6

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(boxWidth)

	t := m.selectedTask

	var rawBlock string
	var err error
	rawBlock, err = task.ReadRawBlock(t)

	var lines []string
	if err != nil || rawBlock == "" {
		lines = append(lines, fmt.Sprintf("%s: %s", t.Keyword, t.Title))
		if err != nil {
			lines = append(lines, fmt.Sprintf("(could not read file: %v)", err))
		}
	} else {
		lines = strings.Split(rawBlock, "\n")
	}

	// Hard-wrap lines
	var wrapped []string
	textStyle := lipgloss.NewStyle()
	for _, line := range lines {
		rendered := task.RenderMarkdownDescription(line, textStyle)
		if len(line) <= contentWidth {
			wrapped = append(wrapped, rendered)
		} else {
			for len(line) > contentWidth {
				segment := line[:contentWidth]
				wrapped = append(wrapped, task.RenderMarkdownDescription(segment, textStyle))
				line = line[contentWidth:]
			}
			if line != "" {
				wrapped = append(wrapped, task.RenderMarkdownDescription(line, textStyle))
			}
		}
	}

	// Footer
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	wrapped = append(wrapped, "")
	wrapped = append(wrapped, metaStyle.Render(fmt.Sprintf("── %s", t.FilePath)))
	wrapped = append(wrapped, "")
	wrapped = append(wrapped, metaStyle.Render("esc/q/v: close"))

	return boxStyle.Render(strings.Join(wrapped, "\n"))
}

type editorFinishedMsg struct{ err error }

func openEditorCmd(cfg *config.Config, t *task.Task) tea.Cmd {
	editor := cfg.GeneralConfig.EDITOR
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	args := []string{fmt.Sprintf("+%d", t.LineNum), t.FilePath}
	c := exec.Command(editor, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m model) renderClockView() string {
	var b strings.Builder

	// Title bar
	start, end := viewRange(m.focusDate, m.mode, m.config)
	_, week := m.focusDate.ISOWeek()
	title := fmt.Sprintf("%s-agenda (W%d): clock table", m.mode.String(), week)
	b.WriteString(colors.header.Render(title))
	b.WriteString("\n")

	// Date range subtitle
	if m.mode == viewDay {
		b.WriteString(colors.dimText.Render(m.focusDate.Format("Monday  2 January 2006")))
	} else {
		b.WriteString(colors.dimText.Render(fmt.Sprintf("%s — %s", start.Format("2 Jan"), end.Format("2 Jan 2006"))))
	}
	b.WriteString("\n")

	if m.clockTable == nil || m.clockTable.GrandTotal == 0 {
		contentHeight := max(5, m.termHeight-4)
		b.WriteString(colors.dimText.Render("  No clock data for this period."))
		b.WriteString("\n")
		for i := 1; i < contentHeight-1; i++ {
			b.WriteString("\n")
		}
		footer := colors.dimText.Render("d/w/t/m/y: view • f/b: navigate • .: today • esc/a: agenda • q: quit")
		b.WriteString(footer)
		return b.String()
	}

	// Column widths
	projColWidth := 14
	timeColWidth := 7
	kwColWidth := 9
	treeWidth := 3 // "╰─ "
	titleWidth := m.termWidth - projColWidth - treeWidth - kwColWidth - timeColWidth - 4
	if titleWidth < 20 {
		titleWidth = 20
	}

	var lines []string
	cursorIdx := 0

	// Empty line separating header from table
	lines = append(lines, "")

	// Top separator
	sep := strings.Repeat("─", m.termWidth-2)
	lines = append(lines, colors.dimText.Render(sep))

	// Grand total
	grandLine := fmt.Sprintf(" %-*s %-*s %*s",
		projColWidth, "",
		treeWidth+kwColWidth+titleWidth, "Total",
		timeColWidth, task.FormatDuration(m.clockTable.GrandTotal))
	lines = append(lines, colors.header.Render(grandLine))

	// Separator after grand total
	lines = append(lines, colors.dimText.Render(sep))

	for _, proj := range m.clockTable.Projects {
		// Project total row
		projLine := fmt.Sprintf(" %-*s %-*s %*s",
			projColWidth, proj.Project+":",
			treeWidth+kwColWidth+titleWidth, "Project time",
			timeColWidth, task.FormatDuration(proj.Total))
		lines = append(lines, colors.project.Render(projLine))

		// Task entries
		for _, entry := range proj.Entries {
			selected := cursorIdx == m.clockCursor

			var kwStyle lipgloss.Style
			if entry.Task.IsInProgress(m.config) {
				kwStyle = colors.inProgress
			} else if entry.Task.IsActive(m.config) {
				kwStyle = colors.active
			} else if entry.Task.IsSomeday(m.config) {
				kwStyle = colors.someday
			} else {
				kwStyle = colors.completed
			}

			displayTitle := entry.Task.Title
			if entry.Task.ID != "" {
				displayTitle = fmt.Sprintf("[%s] %s", entry.Task.ID, entry.Task.Title)
			}
			formattedTitle := task.RenderMarkdownDescription(displayTitle, colors.taskText)
			formattedTitle = task.TruncateString(formattedTitle, titleWidth)

			var indicator string
			if selected {
				indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true).Render("█")
			} else {
				indicator = " "
			}

			taskLine := fmt.Sprintf("%s%-*s %s %s %-*s %*s",
				indicator,
				projColWidth, "",
				colors.dimText.Render("╰─"),
				kwStyle.Render(fmt.Sprintf("%-*s", kwColWidth-1, entry.Task.Keyword)),
				titleWidth, formattedTitle,
				timeColWidth, task.FormatDuration(entry.Duration))
			lines = append(lines, taskLine)
			cursorIdx++
		}

		// Separator after project
		lines = append(lines, colors.dimText.Render(sep))
	}

	// Apply scroll window
	contentHeight := max(5, m.termHeight-4)
	visibleStart := m.clockScrollOffset
	visibleEnd := m.clockScrollOffset + contentHeight
	if visibleEnd > len(lines) {
		visibleEnd = len(lines)
	}
	if visibleStart > len(lines) {
		visibleStart = len(lines)
	}
	rendered := 0
	for i := visibleStart; i < visibleEnd; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
		rendered++
	}
	for i := rendered; i < contentHeight; i++ {
		b.WriteString("\n")
	}

	// Footer
	footer := colors.dimText.Render("j/k: navigate • C-d/C-u: scroll • v: detail • enter: edit • i: clock in • o: clock out • esc/a: agenda • q: quit")
	b.WriteString(footer)

	return b.String()
}

func setupWatcher(cfg *config.Config) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Warning: Could not create file watcher: %v", err)
		return nil
	}

	prjDir := cfg.Directories.Projects
	watcher.Add(prjDir)

	entries, err := os.ReadDir(prjDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				projectDir := filepath.Join(prjDir, e.Name())
				notesDir := filepath.Join(projectDir, "notes")
				watcher.Add(projectDir)
				watcher.Add(notesDir)

				zettelEntries, _ := os.ReadDir(notesDir)
				for _, z := range zettelEntries {
					if z.IsDir() {
						watcher.Add(filepath.Join(notesDir, z.Name()))
					}
				}
			}
		}
	}

	if inboxPath := cfg.GetInboxFilePath(); inboxPath != "" {
		watcher.Add(filepath.Dir(inboxPath))
	}

	return watcher
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	initColors(cfg)

	watcher := setupWatcher(cfg)
	if watcher != nil {
		defer watcher.Close()
	}

	m := initialModel(cfg)
	m.watcher = watcher
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
