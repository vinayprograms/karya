package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	colorspkg "github.com/vinayprograms/karya/internal/colors"
	"github.com/vinayprograms/karya/internal/config"
	kgit "github.com/vinayprograms/karya/internal/git"
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
	assignee    lipgloss.Style
	header      lipgloss.Style
	schedInfo   lipgloss.Style
	dimText     lipgloss.Style
	clockActive lipgloss.Style
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
		overdue:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.OverdueColor)).Bold(true),
		deadline:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.DeadlineColor)).Bold(true),
		assignee:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.AssigneeColor)).Background(lipgloss.Color(cfg.Colors.AssigneeBgColor)).Bold(true),
		header:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.AgendaHeaderColor)).Bold(true),
		schedInfo:   lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		dimText:     lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		clockActive: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ClockActiveColor)).Bold(true),
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

	// Clock resolution state (multiple active clocks)
	activeClockedTasks []*task.Task
	showClockResolve   bool
	clockResolveCursor int

	// Status picker state
	showingStatusPicker        bool
	statusPicker               *task.StatusPicker
	statusPickerTask           *task.Task
	showingPendingChildWarning bool
	pendingWarningKeyword      string

	// Help overlay
	showingHelp bool
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

type minuteTickMsg struct{}

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
		tea.Tick(time.Minute, func(t time.Time) tea.Msg { return minuteTickMsg{} }),
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

type statusUpdateMsg struct {
	message string
	err     error
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Clear status message regardless of UI state
	if _, ok := msg.(clearStatusMsg); ok {
		m.statusMessage = ""
		return m, nil
	}

	// Help overlay
	if m.showingHelp {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "?", "esc", "q":
				m.showingHelp = false
				return m, nil
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil
		case fileChangedMsg:
			return m, tea.Batch(
				loadAgendaCmd(m.config, m.focusDate, m.mode),
				loadClockTableCmd(m.config, m.focusDate, m.mode),
				waitForFileChange(m.watcher),
			)
		}
		return m, nil
	}

	// Pending-child warning — informational only, completion is blocked
	// outright (see task.ErrPendingChildren), so there's no override.
	if m.showingPendingChildWarning {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			m.showingPendingChildWarning = false
			m.statusPickerTask = nil
			m.pendingWarningKeyword = ""
			return m, nil
		}
		return m, nil
	}

	// Status selector mode
	if m.showingStatusPicker {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			m.statusPicker.Update(msg.String())
			if m.statusPicker.Cancelled {
				m.showingStatusPicker = false
				m.statusPickerTask = nil
				return m, nil
			}
			if m.statusPicker.Confirmed {
				kw := m.statusPicker.Selected
				if task.IsCompletedKeyword(m.config, kw) && task.HasActiveChildren(m.statusPickerTask, m.config) {
					m.showingStatusPicker = false
					m.showingPendingChildWarning = true
					m.pendingWarningKeyword = kw
					return m, nil
				}
				return m, updateTaskStatusCmd(m.config, m.statusPickerTask, kw)
			}
			return m, nil
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil
		case statusUpdateMsg:
			m.showingStatusPicker = false
			m.statusPickerTask = nil
			if msg.err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
			} else {
				m.statusMessage = msg.message
			}
			return m, tea.Batch(
				loadAgendaCmd(m.config, m.focusDate, m.mode),
				loadClockTableCmd(m.config, m.focusDate, m.mode),
				waitForFileChange(m.watcher),
				tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
			)
		case fileChangedMsg:
			return m, tea.Batch(
				loadAgendaCmd(m.config, m.focusDate, m.mode),
				loadClockTableCmd(m.config, m.focusDate, m.mode),
				waitForFileChange(m.watcher),
			)
		}
		return m, nil
	}

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

	// Clock resolution mode (multiple active clocks)
	if m.showClockResolve {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.showClockResolve = false
				return m, nil
			case "j", "down":
				if m.clockResolveCursor < len(m.activeClockedTasks)-1 {
					m.clockResolveCursor++
				}
				return m, nil
			case "k", "up":
				if m.clockResolveCursor > 0 {
					m.clockResolveCursor--
				}
				return m, nil
			case "o":
				if m.clockResolveCursor < len(m.activeClockedTasks) {
					t := m.activeClockedTasks[m.clockResolveCursor]
					if err := task.ClockOut(t); err == nil {
						m.activeClockedTasks = append(m.activeClockedTasks[:m.clockResolveCursor], m.activeClockedTasks[m.clockResolveCursor+1:]...)
						if m.clockResolveCursor >= len(m.activeClockedTasks) {
							m.clockResolveCursor = max(0, len(m.activeClockedTasks)-1)
						}
						if len(m.activeClockedTasks) <= 1 {
							m.showClockResolve = false
							return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
						}
					}
				}
				return m, nil
			}
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
			case "f":
				m.mode = viewFortnight
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "m":
				m.mode = viewMonth
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "y":
				m.mode = viewYear
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)

			// Navigation
			case "l", "right":
				m.focusDate = advanceFocus(m.focusDate, m.mode, 1)
				return m, loadClockTableCmd(m.config, m.focusDate, m.mode)
			case "h", "left":
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

			// Status change
			case "t":
				if m.clockCursor < len(m.clockTasks) {
					t := m.clockTasks[m.clockCursor]
					m.statusPickerTask = t
					m.showingStatusPicker = true
					m.statusPicker = task.NewStatusPicker(t, m.config)
					return m, nil
				}

			// Help
			case "?":
				m.showingHelp = !m.showingHelp
				return m, nil
			}

		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height

		case statusUpdateMsg:
			if msg.err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
			} else {
				m.statusMessage = msg.message
			}
			return m, tea.Batch(
				loadClockTableCmd(m.config, m.focusDate, m.mode),
				tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
			)

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
		case "f":
			m.mode = viewFortnight
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "m":
			m.mode = viewMonth
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "y":
			m.mode = viewYear
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)

		// Navigation
		case "l", "right":
			m.focusDate = advanceFocus(m.focusDate, m.mode, 1)
			return m, loadAgendaCmd(m.config, m.focusDate, m.mode)
		case "h", "left":
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

		// Jump to top/bottom
		case "g":
			m.cursor = 0
			m.scrollOffset = 0
		case "G":
			if len(m.flatItems) > 0 {
				m.cursor = len(m.flatItems) - 1
				m.ensureVisible()
			}

		// Jump between day groups
		case "{":
			m.cursor = prevDayStart(m.days, m.cursor)
			m.ensureVisible()
		case "}":
			m.cursor = nextDayStart(m.days, m.cursor)
			m.ensureVisible()

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

		// Status change
		case "t":
			if m.cursor < len(m.flatItems) {
				t := m.flatItems[m.cursor].Task
				m.statusPickerTask = t
				m.showingStatusPicker = true
				m.statusPicker = task.NewStatusPicker(t, m.config)
				return m, nil
			}

		// Help
		case "?":
			m.showingHelp = !m.showingHelp
			return m, nil

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

			// Detect multiple active clocks
			var activeTasks []*task.Task
			seen := make(map[*task.Task]bool)
			for _, item := range m.flatItems {
				if item.ClockActive && !seen[item.Task] {
					activeTasks = append(activeTasks, item.Task)
					seen[item.Task] = true
				}
			}
			m.activeClockedTasks = activeTasks
			if len(activeTasks) > 1 {
				m.showClockResolve = true
				m.clockResolveCursor = 0
			}
		}

	case fileChangedMsg:
		return m, tea.Batch(
			loadAgendaCmd(m.config, m.focusDate, m.mode),
			loadClockTableCmd(m.config, m.focusDate, m.mode),
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

	case statusUpdateMsg:
		m.statusPickerTask = nil
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.statusMessage = msg.message
		}
		return m, tea.Batch(
			loadAgendaCmd(m.config, m.focusDate, m.mode),
			tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
		)

	case minuteTickMsg:
		return m, tea.Tick(time.Minute, func(t time.Time) tea.Msg { return minuteTickMsg{} })
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

func updateTaskStatusCmd(cfg *config.Config, t *task.Task, newKeyword string) tea.Cmd {
	return func() tea.Msg {
		if t == nil {
			return statusUpdateMsg{err: fmt.Errorf("no task selected")}
		}

		oldKeyword := t.Keyword

		if task.IsCompletedKeyword(cfg, newKeyword) {
			advanced, err := task.CompleteRecurringTask(t, cfg, newKeyword)
			if err != nil {
				return statusUpdateMsg{err: fmt.Errorf("recurring advance failed: %w", err)}
			}
			if advanced {
				commitMsg := fmt.Sprintf("Advance recurring task: %s", t.Title)
				kgit.CommitFile(t.FilePath, commitMsg, true)
				return statusUpdateMsg{
					message: fmt.Sprintf("Recurring task advanced → %s", t.ScheduledAt),
				}
			}
		}

		if err := task.UpdateTaskStatus(t, newKeyword, cfg); err != nil {
			return statusUpdateMsg{err: err}
		}

		// Record state transition for all status changes
		if err := task.RecordStateTransition(t, oldKeyword, newKeyword); err != nil {
			return statusUpdateMsg{err: fmt.Errorf("status updated but failed to record transition: %w", err)}
		}

		commitMsg := fmt.Sprintf("Update task status: %s -> %s", oldKeyword, newKeyword)
		if err := kgit.CommitFile(t.FilePath, commitMsg, true); err != nil {
			return statusUpdateMsg{
				message: fmt.Sprintf("Status updated to %s (git commit failed: %v)", newKeyword, err),
			}
		}

		return statusUpdateMsg{
			message: fmt.Sprintf("Status updated: %s → %s", oldKeyword, newKeyword),
		}
	}
}

func (m model) renderStatusSelector() string {
	return m.statusPicker.View(m.termWidth)
}

func (m model) renderPendingChildWarning() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("9")).
		Padding(1, 2)

	activeCount := 0
	if m.statusPickerTask != nil {
		for _, child := range m.statusPickerTask.Children {
			if child.IsActive(m.config) || child.IsInProgress(m.config) {
				activeCount++
			}
		}
	}

	var content strings.Builder

	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	content.WriteString(warningStyle.Render("⚠  Cannot complete: incomplete children"))
	content.WriteString("\n\n")

	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	content.WriteString(infoStyle.Render(fmt.Sprintf(
		"%d active/in-progress child task(s) still pending.\nCannot mark parent as %s until they're resolved or reassigned.",
		activeCount, m.pendingWarningKeyword,
	)))
	content.WriteString("\n\n")

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	content.WriteString(helpStyle.Render("any key: dismiss"))

	return boxStyle.Render(content.String())
}

func (m model) renderHelp() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	var b strings.Builder

	if m.showingClockView {
		b.WriteString(titleStyle.Render("Clock View Keybindings"))
	} else {
		b.WriteString(titleStyle.Render("Agenda Keybindings"))
	}
	b.WriteString("\n\n")

	type binding struct{ key, desc string }

	common := []binding{
		{"d/w/f/m/y", "switch view (day/week/fortnight/month/year)"},
		{"l, →", "navigate forward"},
		{"h, ←", "navigate backward"},
		{"., space", "jump to today"},
		{"j, ↓", "cursor down"},
		{"k, ↑", "cursor up"},
		{"g", "go to top"},
		{"G", "go to bottom"},
		{"{", "previous day group"},
		{"}", "next day group"},
		{"C-d", "half-page down"},
		{"C-u", "half-page up"},
		{"v", "detail view"},
		{"enter", "open in editor"},
		{"t", "change status"},
		{"S", "set scheduled date"},
		{"D", "set due date"},
		{"i", "clock in"},
		{"o", "clock out"},
		{"?", "toggle this help"},
		{"q", "quit"},
	}

	agenda := []binding{
		{"c", "switch to clock view"},
	}

	clock := []binding{
		{"esc, a", "back to agenda"},
	}

	bindings := common
	if m.showingClockView {
		bindings = append(bindings, clock...)
	} else {
		bindings = append(bindings, agenda...)
	}

	for _, kb := range bindings {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			keyStyle.Render(fmt.Sprintf("%-12s", kb.key)),
			descStyle.Render(kb.desc)))
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press ? or esc to close"))

	return boxStyle.Render(b.String())
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

// prevDayStart returns the cursor index of the first item in the previous day group.
func prevDayStart(days []task.AgendaDay, cursor int) int {
	offset := 0
	for _, day := range days {
		end := offset + len(day.Items)
		if cursor >= offset && cursor < end {
			if cursor > offset {
				return offset
			}
			// Already at start of this day — go to previous day
			if offset > 0 {
				// Find start of previous day
				prev := 0
				for _, d := range days {
					next := prev + len(d.Items)
					if next == offset {
						return prev
					}
					prev = next
				}
			}
			return 0
		}
		offset = end
	}
	return 0
}

// nextDayStart returns the cursor index of the first item in the next day group.
func nextDayStart(days []task.AgendaDay, cursor int) int {
	offset := 0
	for _, day := range days {
		end := offset + len(day.Items)
		if cursor >= offset && cursor < end {
			if end < totalItems(days) {
				return end
			}
			return cursor
		}
		offset = end
	}
	return cursor
}

func totalItems(days []task.AgendaDay) int {
	n := 0
	for _, d := range days {
		n += len(d.Items)
	}
	return n
}

func flattenItems(days []task.AgendaDay) []task.AgendaItem {
	var items []task.AgendaItem
	for _, day := range days {
		items = append(items, day.Items...)
	}
	return items
}

// flattenDayItems reorders items for day view: untimed first, then timed sorted by hour.
// This matches renderDayTimeGrid's output order so cursor indices align with rendered positions.
func flattenDayItems(days []task.AgendaDay) []task.AgendaItem {
	var all []task.AgendaItem
	for _, day := range days {
		all = append(all, day.Items...)
	}

	var timed, overdue, todayUntimed []task.AgendaItem
	for _, item := range all {
		if item.HasTime && !item.IsOverdue {
			timed = append(timed, item)
		} else if item.IsOverdue {
			overdue = append(overdue, item)
		} else {
			todayUntimed = append(todayUntimed, item)
		}
	}

	// If no timed items, keep original order
	if len(timed) == 0 {
		return all
	}

	// Sort timed items chronologically to match renderDayTimeGrid
	sort.SliceStable(timed, func(i, j int) bool {
		return timed[i].Date.Before(timed[j].Date)
	})

	var result []task.AgendaItem
	result = append(result, overdue...)
	result = append(result, todayUntimed...)
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
	var overdue []task.AgendaItem
	var todayUntimed []task.AgendaItem

	for _, item := range items {
		if item.HasTime && !item.IsOverdue {
			timed = append(timed, item)
		} else if item.IsOverdue {
			overdue = append(overdue, item)
		} else {
			todayUntimed = append(todayUntimed, item)
		}
	}

	if len(timed) == 0 {
		for range items {
			*cursorToLine = append(*cursorToLine, lineIdx)
			lineIdx++
		}
		return lineIdx
	}

	// Overdue items
	for range overdue {
		*cursorToLine = append(*cursorToLine, lineIdx)
		lineIdx++
	}
	if len(overdue) > 0 {
		lineIdx++ // separator line
	}

	// Today's untimed items
	for range todayUntimed {
		*cursorToLine = append(*cursorToLine, lineIdx)
		lineIdx++
	}
	if len(todayUntimed) > 0 {
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

	sort.SliceStable(timed, func(i, j int) bool {
		return timed[i].Date.Before(timed[j].Date)
	})

	hourItemIndices := make(map[int][]int) // hour -> indices into timed slice
	for i, item := range timed {
		hourItemIndices[item.Date.Hour()] = append(hourItemIndices[item.Date.Hour()], i)
	}

	// Mirror gap computation from renderDayTimeGrid
	gapAfter := make(map[int]bool)
	hoursInGap := make(map[int]bool)
	for i := 0; i < len(timed)-1; i++ {
		if !timed[i].HasEnd || timed[i].IsCompleted {
			continue
		}
		if timed[i+1].IsCompleted {
			continue
		}
		endTime := timed[i].EndTime
		nextStart := timed[i+1].Date
		if nextStart.Sub(endTime) >= 15*time.Minute {
			gapAfter[i] = true
			for h := endTime.Hour(); h < nextStart.Hour(); h++ {
				if _, occupied := hourItemIndices[h]; !occupied {
					hoursInGap[h] = true
				}
			}
		}
	}

	for hour := gridStart; hour < gridEnd; hour++ {
		if indices, ok := hourItemIndices[hour]; ok {
			for _, idx := range indices {
				*cursorToLine = append(*cursorToLine, lineIdx)
				lineIdx++
				if gapAfter[idx] {
					lineIdx++ // gap indicator line
				}
			}
		} else if !hoursInGap[hour] {
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

	if m.showingHelp {
		return m.renderHelp()
	}

	if m.showingPendingChildWarning {
		return m.renderPendingChildWarning()
	}

	if m.showingStatusPicker {
		return m.renderStatusSelector()
	}

	if m.showingDatePicker && m.datePicker != nil {
		return m.datePicker.View(m.termWidth / 2)
	}

	if m.showClockResolve {
		return m.renderClockResolveView()
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
		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		isToday := start.Equal(today)
		if len(items) == 0 {
			lines = append(lines, colors.dimText.Render("  No scheduled items."))
		} else {
			lines, itemIdx = m.renderDayTimeGrid(items, itemIdx, isToday)
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
	footer := colors.dimText.Render("t: status • S/D: schedule/due • c: clock • i/o: in/out • v: detail • enter: edit • ?: help • q: quit")
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
	projStyle := colors.project
	if item.IsCompleted {
		projStyle = colors.completed
	} else if item.ClockActive {
		projStyle = colors.clockActive
	}
	parts = append(parts, projStyle.Render(fmt.Sprintf("%-10s", proj+":")))

	// Schedule info column (14 chars)
	schedStr := m.formatScheduleInfo(item)
	schedStyle := colors.schedInfo
	if item.IsCompleted {
		schedStyle = colors.completed
	} else if item.ClockActive {
		schedStyle = colors.clockActive
	} else if item.IsOverdue {
		schedStyle = colors.deadline
	} else if item.Warning {
		schedStyle = colors.overdue
	} else if item.IsDeadline {
		itemDay := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, time.Local)
		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		if itemDay.Equal(today) {
			schedStyle = colors.overdue
		}
	}
	parts = append(parts, schedStyle.Render(fmt.Sprintf("%-14s", schedStr)))

	// Keyword column
	kwWidth := 12
	displayKeyword := t.Keyword
	var kwStyle lipgloss.Style
	if item.IsCompleted {
		kwStyle = colors.completed
		if len(m.config.Todo.Completed) > 0 {
			displayKeyword = m.config.Todo.Completed[0]
		}
	} else if t.IsInProgress(m.config) {
		kwStyle = colors.inProgress
	} else if t.IsActive(m.config) {
		kwStyle = colors.active
	} else if t.IsSomeday(m.config) {
		kwStyle = colors.someday
	} else {
		kwStyle = colors.completed
	}
	parts = append(parts, kwStyle.Render(fmt.Sprintf("%-*s", kwWidth, displayKeyword)))

	// Title (truncated to fit terminal)
	titleStyle := colors.taskText
	if item.IsCompleted {
		titleStyle = colors.completed
	} else if t.IsCompleted(m.config) {
		titleStyle = colors.completed
	} else if item.IsOverdue {
		titleStyle = colors.deadline
	} else if item.Warning {
		titleStyle = colors.overdue
	} else if item.IsDeadline {
		itemDay := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, time.Local)
		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
		if itemDay.Equal(today) {
			titleStyle = colors.overdue
		}
	}
	displayTitle := t.Title
	if t.ID != "" {
		displayTitle = fmt.Sprintf("[%s] %s", t.ID, t.Title)
	}
	if item.IsCompleted {
		displayTitle = "✓ " + displayTitle
	} else if item.ClockActive {
		displayTitle = "⏱ " + displayTitle
		titleStyle = colors.clockActive
	}
	formattedTitle := titleStyle.Render(task.RenderMarkdownDescription(displayTitle, titleStyle))
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
func (m model) renderDayTimeGrid(items []task.AgendaItem, startIdx int, isToday bool) ([]string, int) {
	var timed []task.AgendaItem
	var overdue []task.AgendaItem
	var todayUntimed []task.AgendaItem

	for _, item := range items {
		if item.HasTime && !item.IsOverdue {
			timed = append(timed, item)
		} else if item.IsOverdue {
			overdue = append(overdue, item)
		} else {
			todayUntimed = append(todayUntimed, item)
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

	// Sort timed items strictly by time for chronological rendering
	sort.SliceStable(timed, func(i, j int) bool {
		return timed[i].Date.Before(timed[j].Date)
	})

	// Build a map: hour -> items starting in that hour
	hourItems := make(map[int][]int) // hour -> indices into timed slice
	for i, item := range timed {
		hourItems[item.Date.Hour()] = append(hourItems[item.Date.Hour()], i)
	}

	// Pre-compute gaps between consecutive timed items (≥15 min free).
	// Only consider items that occupy real calendar slots (have an end time,
	// not overdue, not completed).
	gapAfter := make(map[int]time.Duration) // index into timed -> gap duration
	hoursInGap := make(map[int]bool)        // hours covered by a gap (suppress empty-hour markers)
	for i := 0; i < len(timed)-1; i++ {
		if !timed[i].HasEnd || timed[i].IsCompleted {
			continue
		}
		if timed[i+1].IsCompleted {
			continue
		}
		endTime := timed[i].EndTime
		nextStart := timed[i+1].Date
		gap := nextStart.Sub(endTime)
		if gap >= 15*time.Minute {
			gapAfter[i] = gap
			for h := endTime.Hour(); h < nextStart.Hour(); h++ {
				if _, occupied := hourItems[h]; !occupied {
					hoursInGap[h] = true
				}
			}
		}
	}

	// Render overdue items first
	for _, item := range overdue {
		selected := itemIdx == m.cursor
		lines = append(lines, m.renderItem(item, selected))
		itemIdx++
	}
	if len(overdue) > 0 {
		lines = append(lines, colors.dimText.Render("  ─────────────────"))
	}

	// Render today's untimed items (scheduled for today, no specific time)
	for _, item := range todayUntimed {
		selected := itemIdx == m.cursor
		lines = append(lines, m.renderItem(item, selected))
		itemIdx++
	}
	if len(todayUntimed) > 0 {
		lines = append(lines, colors.dimText.Render("  ─────────────────"))
	}

	// Render the time grid
	now := time.Now()
	nowRendered := false
	nowMarker := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).
		Render(fmt.Sprintf("  %s now %s", now.Format("15:04"), strings.Repeat("─", 12)))

	for hour := gridStart; hour < gridEnd; hour++ {
		if isToday && !nowRendered && hour > now.Hour() {
			lines = append(lines, nowMarker)
			nowRendered = true
		}

		if indices, ok := hourItems[hour]; ok {
			for _, idx := range indices {
				item := timed[idx]
				if isToday && !nowRendered && item.Date.After(now) {
					lines = append(lines, nowMarker)
					nowRendered = true
				}
				selected := itemIdx == m.cursor
				lines = append(lines, m.renderItem(item, selected))
				itemIdx++

				if gap, hasGap := gapAfter[idx]; hasGap {
					lines = append(lines, colors.dimText.Render(
						fmt.Sprintf("  ··· %s free ···········", formatGapDuration(gap))))
				}
			}
		} else if !hoursInGap[hour] {
			if isToday && !nowRendered && hour == now.Hour() {
				lines = append(lines, nowMarker)
				nowRendered = true
			} else {
				lines = append(lines, colors.dimText.Render(fmt.Sprintf("  %02d:00 ··················", hour)))
			}
		}
	}

	if isToday && !nowRendered {
		lines = append(lines, nowMarker)
	}

	return lines, itemIdx
}

func formatGapDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	} else if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

func (m model) formatScheduleInfo(item task.AgendaItem) string {
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	itemDay := time.Date(item.Date.Year(), item.Date.Month(), item.Date.Day(), 0, 0, 0, 0, time.Local)

	if item.IsCompleted {
		if item.CompletedAt.IsZero() {
			return "Done"
		}
		return "Done " + item.CompletedAt.Format("15:04")
	}

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

func (m model) renderClockResolveView() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).
		Render("Multiple active clocks detected")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")
	lines = append(lines, "The following tasks have open clock entries:")
	lines = append(lines, "")

	for i, t := range m.activeClockedTasks {
		indicator := "  "
		style := lipgloss.NewStyle()
		if i == m.clockResolveCursor {
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true).Render("█ ")
			style = colors.clockActive
		}
		proj := t.Project
		if len(proj) > 12 {
			proj = proj[:12]
		}
		line := fmt.Sprintf("%s%-13s %s", indicator, proj+":", style.Render(t.Title))
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, colors.dimText.Render("  j/k: navigate • o: clock out • esc: dismiss"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("11")).
		Padding(1, 2).
		Width(min(70, m.termWidth-4))

	content := box.Render(strings.Join(lines, "\n"))

	return lipgloss.Place(m.termWidth, m.termHeight, lipgloss.Center, lipgloss.Center, content)
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
		footer := colors.dimText.Render("t: status • i/o: in/out • v: detail • enter: edit • esc/a: agenda • ?: help • q: quit")
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

			displayKeyword := entry.Task.Keyword
			var kwStyle lipgloss.Style
			if entry.WasCompleted {
				kwStyle = colors.completed
				if len(m.config.Todo.Completed) > 0 {
					displayKeyword = m.config.Todo.Completed[0]
				}
			} else if entry.Task.IsInProgress(m.config) {
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
			titleStyle := colors.taskText
			if task.IsClockActive(entry.Task) {
				displayTitle = "⏱ " + displayTitle
				titleStyle = colors.clockActive
			}
			formattedTitle := task.RenderMarkdownDescription(displayTitle, titleStyle)
			formattedTitle = task.TruncateString(formattedTitle, titleWidth)

			var indicator string
			if selected {
				indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true).Render("█")
			} else {
				indicator = " "
			}

			durationStr := task.FormatDuration(entry.Duration)
			if task.IsClockActive(entry.Task) {
				durationStr = colors.clockActive.Render(fmt.Sprintf("%*s", timeColWidth, durationStr))
			} else {
				durationStr = fmt.Sprintf("%*s", timeColWidth, durationStr)
			}

			taskLine := fmt.Sprintf("%s%-*s %s %s %-*s %s",
				indicator,
				projColWidth, "",
				colors.dimText.Render("╰─"),
				kwStyle.Render(fmt.Sprintf("%-*s", kwColWidth-1, displayKeyword)),
				titleWidth, formattedTitle,
				durationStr)
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
	footer := colors.dimText.Render("t: status • i/o: in/out • v: detail • enter: edit • esc/a: agenda • ?: help • q: quit")
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

	if len(os.Args) > 1 && os.Args[1] == "colors" {
		if err := colorspkg.Print(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
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
