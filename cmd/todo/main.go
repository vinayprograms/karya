package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	colorspkg "github.com/vinayprograms/karya/internal/colors"
	configpkg "github.com/vinayprograms/karya/internal/config"
	kgit "github.com/vinayprograms/karya/internal/git"
	"github.com/vinayprograms/karya/internal/jira"
	"github.com/vinayprograms/karya/internal/task"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/fsnotify/fsnotify"
)

// ColorScheme holds the lipgloss color styles for rendering
type ColorScheme struct {
	prjColor             lipgloss.Style
	activeColor          lipgloss.Style
	inProgressColor      lipgloss.Style
	completedColor       lipgloss.Style
	somedayColor         lipgloss.Style
	taskColor            lipgloss.Style
	completedTaskColor   lipgloss.Style
	specialTagColor      lipgloss.Style
	tagColor             lipgloss.Style
	dateColor            lipgloss.Style
	pastDateColor        lipgloss.Style
	todayDateColor       lipgloss.Style
	assigneeColor        lipgloss.Style
	cycleColor           lipgloss.Style
	childConnectorColor  lipgloss.Style // ⌊ connector for child tasks
	pendingChildColor    lipgloss.Style // ◑ indicator for parents with pending children
}

// Global color scheme (will be initialized from config)
var colors ColorScheme

// InitializeColors initializes the color scheme from task config
func InitializeColors(cfg *configpkg.Config) {
	colors = ColorScheme{
		prjColor:           lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		activeColor:        lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		inProgressColor:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.InProgressColor)),
		completedColor:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedColor)),
		somedayColor:       lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.SomedayColor)),
		taskColor:          lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TaskColor)),
		completedTaskColor: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedTaskColor)),
		tagColor:           lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TagColor)).Background(lipgloss.Color(cfg.Colors.TagBgColor)),
		specialTagColor:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.SpecialTagColor)).Background(lipgloss.Color(cfg.Colors.SpecialTagBgColor)).Bold(true),
		dateColor:          lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.DateColor)).Background(lipgloss.Color(cfg.Colors.DateBgColor)),
		pastDateColor:      lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.PastDateColor)).Background(lipgloss.Color(cfg.Colors.PastDateBgColor)),
		todayDateColor:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TodayDateColor)).Background(lipgloss.Color(cfg.Colors.TodayDateBgColor)).Bold(true),
		assigneeColor:       lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.AssigneeColor)).Background(lipgloss.Color(cfg.Colors.AssigneeBgColor)).Bold(true),
		cycleColor:          lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CycleColor)).Background(lipgloss.Color(cfg.Colors.CycleBgColor)).Bold(true),
		childConnectorColor: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		pendingChildColor:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.InProgressColor)),
	}
}

type taskItem struct {
	config           *configpkg.Config
	task             *task.Task
	projectColWidth  int
	keywordColWidth  int
	fractionColWidth int
	maxTitleWidth    int
	verbose          bool
}

func NewTaskItem(c *configpkg.Config, t *task.Task, projectColWidth, keywordColWidth, fractionColWidth, maxTitleWidth int, verbose bool) taskItem {
	return taskItem{
		config:           c,
		task:             t,
		projectColWidth:  projectColWidth,
		keywordColWidth:  keywordColWidth,
		fractionColWidth: fractionColWidth,
		maxTitleWidth:    maxTitleWidth,
		verbose:          verbose,
	}
}

func (i taskItem) renderWithSelection(isSelected bool) string {
	var parts []string

	// Show cycle indicator if task is in a cycle
	if i.task.InCycle {
		parts = append(parts, colors.cycleColor.Render(" ⟲ "))
	}

	// Project column: blank for child tasks, project name for root tasks
	if i.task.Parent != nil {
		parts = append(parts, strings.Repeat(" ", i.projectColWidth))
	} else {
		parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-*s", i.projectColWidth, i.task.Project)))
	}

	// Only show Zettel column in verbose mode
	if i.verbose {
		parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))
	}

	// Indicator slot (always 2 display columns):
	//   ⌊  for child tasks (connector to parent above)
	//   ◑  for root tasks with pending children
	//      blank otherwise
	done, total := i.task.PendingChildCount(i.config)
	hasPending := total > 0 && done < total
	if i.task.Parent != nil {
		parts = append(parts, colors.childConnectorColor.Render("╰─"))
	} else if hasPending {
		parts = append(parts, colors.pendingChildColor.Render("◑ "))
	} else {
		parts = append(parts, "  ")
	}

	var titleStyle lipgloss.Style
	if i.task.IsActive(i.config) {
		parts = append(parts, colors.activeColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsInProgress(i.config) {
		parts = append(parts, colors.inProgressColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsSomeday(i.config) {
		parts = append(parts, colors.somedayColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else {
		parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.completedTaskColor
	}

	// Progress fraction column immediately after keyword (fixed width, blank when not applicable)
	if i.fractionColWidth > 0 {
		if hasPending {
			parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-*s", i.fractionColWidth, fmt.Sprintf("[%d/%d]", done, total))))
		} else {
			parts = append(parts, strings.Repeat(" ", i.fractionColWidth))
		}
	}

	// Build title with optional ID prefix
	displayTitle := i.task.Title
	if i.task.ID != "" {
		displayTitle = fmt.Sprintf("[%s] %s", i.task.ID, i.task.Title)
	}

	// Render task title with markdown formatting, then truncate (no padding)
	formattedTitle := task.RenderMarkdownDescription(displayTitle, titleStyle)
	titleWidth := i.maxTitleWidth
	if titleWidth <= 0 {
		titleWidth = 40
	}
	formattedTitle = task.TruncateString(formattedTitle, titleWidth)
	if isSelected {
		indicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")).
			Bold(true).
			Render("█ ")
		parts = append(parts, indicator+formattedTitle)
	} else {
		parts = append(parts, "  "+formattedTitle)
	}

	// Render tags with special color if they match special tags
	for _, tag := range i.task.Tags {
		isSpecial := false
		for _, specialTag := range i.config.Todo.SpecialTags {
			if tag == specialTag || strings.HasPrefix(tag, specialTag+":") {
				isSpecial = true
				break
			}
		}
		if isSpecial {
			parts = append(parts, colors.specialTagColor.Render(fmt.Sprintf(" %s ", tag)))
		} else {
			parts = append(parts, colors.tagColor.Render(fmt.Sprintf(" %s ", tag)))
		}
	}
	// Display date types with prefixes
	if i.task.ScheduledAt != "" {
		dateStyle := getDateStyle(i.task.ScheduledAt, false)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" S:%s ", i.task.ScheduledAt)))
	}
	if i.task.DueAt != "" {
		dateStyle := getDateStyle(i.task.DueAt, true)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" D:%s ", i.task.DueAt)))
	}
	if i.task.Assignee != "" {
		parts = append(parts, colors.assigneeColor.Render(fmt.Sprintf(" %s ", i.task.Assignee)))
	}

	// Show references if task has any
	if len(i.task.References) > 0 {
		refStr := "^" + strings.Join(i.task.References, " ^")
		parts = append(parts, colors.prjColor.Render(refStr))
	}

	return strings.Join(parts, " ")
}

func (i taskItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s %s %s %s %s %s %s %s",
		i.task.Project, i.task.Zettel, i.task.Keyword, i.task.ID, i.task.Title,
		strings.Join(i.task.Tags, " "), i.task.ScheduledAt, i.task.DueAt, i.task.Assignee,
		strings.Join(i.task.References, " "))
}

func (i taskItem) Title() string {
	var parts []string

	// Show cycle indicator if task is in a cycle
	if i.task.InCycle {
		parts = append(parts, colors.cycleColor.Render(" ⟲ "))
	}

	// Project column: blank for child tasks, project name for root tasks
	if i.task.Parent != nil {
		parts = append(parts, strings.Repeat(" ", i.projectColWidth))
	} else {
		parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-*s", i.projectColWidth, i.task.Project)))
	}

	// Only show Zettel column in verbose mode
	if i.verbose {
		parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))
	}

	// Indicator slot (always 2 display columns):
	//   ⌊  for child tasks (connector to parent above)
	//   ◑  for root tasks with pending children
	//      blank otherwise
	done, total := i.task.PendingChildCount(i.config)
	hasPending := total > 0 && done < total
	if i.task.Parent != nil {
		parts = append(parts, colors.childConnectorColor.Render("╰─"))
	} else if hasPending {
		parts = append(parts, colors.pendingChildColor.Render("◑ "))
	} else {
		parts = append(parts, "  ")
	}

	var titleStyle lipgloss.Style
	if i.task.IsActive(i.config) {
		parts = append(parts, colors.activeColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsInProgress(i.config) {
		parts = append(parts, colors.inProgressColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsSomeday(i.config) {
		parts = append(parts, colors.somedayColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else {
		parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.completedTaskColor
	}

	// Progress fraction column immediately after keyword (fixed width, blank when not applicable)
	if i.fractionColWidth > 0 {
		if hasPending {
			parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-*s", i.fractionColWidth, fmt.Sprintf("[%d/%d]", done, total))))
		} else {
			parts = append(parts, strings.Repeat(" ", i.fractionColWidth))
		}
	}

	// Build title with optional ID prefix
	displayTitle := i.task.Title
	if i.task.ID != "" {
		displayTitle = fmt.Sprintf("[%s] %s", i.task.ID, i.task.Title)
	}

	// Render task title with markdown formatting, then truncate (no padding)
	formattedTitle := task.RenderMarkdownDescription(displayTitle, titleStyle)
	titleWidth := i.maxTitleWidth
	if titleWidth <= 0 {
		titleWidth = 40
	}
	formattedTitle = task.TruncateString(formattedTitle, titleWidth)
	parts = append(parts, formattedTitle)

	// Render tags with special color if they match special tags
	for _, tag := range i.task.Tags {
		isSpecial := false
		for _, specialTag := range i.config.Todo.SpecialTags {
			if tag == specialTag || strings.HasPrefix(tag, specialTag+":") {
				isSpecial = true
				break
			}
		}
		if isSpecial {
			parts = append(parts, colors.specialTagColor.Render(fmt.Sprintf(" %s ", tag)))
		} else {
			parts = append(parts, colors.tagColor.Render(fmt.Sprintf(" %s ", tag)))
		}
	}
	// Display date types with prefixes
	if i.task.ScheduledAt != "" {
		dateStyle := getDateStyle(i.task.ScheduledAt, false)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" S:%s ", i.task.ScheduledAt)))
	}
	if i.task.DueAt != "" {
		dateStyle := getDateStyle(i.task.DueAt, true)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" D:%s ", i.task.DueAt)))
	}
	if i.task.Assignee != "" {
		parts = append(parts, colors.assigneeColor.Render(fmt.Sprintf(" %s ", i.task.Assignee)))
	}

	// Show references if task has any
	if len(i.task.References) > 0 {
		refStr := "^" + strings.Join(i.task.References, " ^")
		parts = append(parts, colors.prjColor.Render(refStr))
	}

	return strings.Join(parts, " ")
}

func (i taskItem) Description() string { return "" }

func getDateStyle(dateStr string, isDeadline bool) lipgloss.Style {
	if dateStr == "" {
		return colors.dateColor
	}

	// Try multiple date formats
	dateFormats := []string{
		"2006-01-02", // YYYY-MM-DD (ISO 8601)
		"02-01-2006", // DD-MM-YYYY (British/Asian)
		"01-02-2006", // MM-DD-YYYY (American)
		"2006/01/02", // YYYY/MM/DD
		"02/01/2006", // DD/MM/YYYY
		"01/02/2006", // MM/DD/YYYY
	}

	var parsedDate time.Time
	var err error
	for _, format := range dateFormats {
		parsedDate, err = time.Parse(format, dateStr)
		if err == nil {
			break
		}
	}

	// If all formats fail, return default color
	if err != nil {
		return colors.dateColor
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	taskDate := time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, time.UTC)

	if taskDate.Before(today) {
		return colors.pastDateColor
	} else if taskDate.Equal(today) {
		return colors.todayDateColor
	} else if taskDate.After(today) {
		// For deadlines, apply today's color if within 7 days
		if isDeadline {
			sevenDaysFromNow := today.AddDate(0, 0, 7)
			if taskDate.Before(sevenDaysFromNow) || taskDate.Equal(sevenDaysFromNow) {
				return colors.todayDateColor
			}
		}
	}
	return colors.dateColor
}

// Custom delegate for proper selection highlighting
type taskDelegate struct {
	list.DefaultDelegate
}

func (d taskDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	taskItem, ok := item.(taskItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	content := taskItem.renderWithSelection(isSelected)
	fmt.Fprint(w, content)
}

type noResultsItem struct{}

func (i noResultsItem) FilterValue() string { return "" }

func (i noResultsItem) Title() string { return "No results found" }

func (i noResultsItem) Description() string { return "" }

type model struct {
	list            list.Model
	tasks           []*task.Task
	config          *configpkg.Config
	project         string
	quitting        bool
	watcher         *fsnotify.Watcher
	projectColWidth  int
	keywordColWidth  int
	fractionColWidth int
	savedFilter      string
	customFilter    string
	filtering       bool
	allItems        []list.Item
	structuredMode  bool
	loading         bool
	searchTerm      string // Track search term for editor highlighting

	// Status selector state
	showingStatusSelector     bool
	statusPicker              *task.StatusPicker
	selectedTask              *task.Task
	statusMessage             string
	statusMessageTimer        int

	// Pending-child warning state
	showingPendingChildWarning bool
	pendingWarningKeyword      string

	// Detail view state
	showingDetailView bool

	// Date picker state
	showingDatePicker bool
	datePicker        *task.DatePicker

	// Terminal dimensions
	termWidth  int
	termHeight int

	// Background sync errors (printed to stderr on exit)
	jiraSyncLog []string
}

func (m model) calcMaxTitleWidth() int {
	// Left columns: project + indicator(2) + keyword + fraction + selector(2)
	left := m.projectColWidth + 2 + m.keywordColWidth + m.fractionColWidth + 2
	if m.config.GeneralConfig.Verbose {
		left += 16 // zettel column
	}
	// Reserve 30 for right-side metadata (tag, dates, assignee)
	available := m.termWidth - left - 30
	if available < 20 {
		available = 20
	}
	return available
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForFileChange(m.watcher)}
	if m.config.HasJira() {
		cmds = append(cmds, jiraSyncCmd(m.config))
	}
	return tea.Batch(cmds...)
}

type fileChangedMsg struct{}

type jiraSyncDoneMsg struct {
	results []jiraSyncResult
}

type jiraSyncResult struct {
	conn  string
	count int
	err   error
}

func jiraSyncCmd(cfg *configpkg.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var results []jiraSyncResult
		for _, conn := range cfg.Jira.Connections {
			r := jiraSyncResult{conn: conn.Name}
			client, err := jira.NewClient(conn.Name)
			if err != nil {
				r.err = fmt.Errorf("client: %w", err)
				results = append(results, r)
				continue
			}
			if err := client.Init(ctx); err != nil {
				r.err = fmt.Errorf("init: %w", err)
				results = append(results, r)
				continue
			}
			count, err := task.SyncFromJira(ctx, cfg, client)
			if err != nil {
				r.err = fmt.Errorf("sync: %w", err)
				results = append(results, r)
				continue
			}
			r.count = count
			results = append(results, r)
		}
		return jiraSyncDoneMsg{results: results}
	}
}

func jiraSyncTick() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return jiraSyncTickMsg{}
	})
}

type jiraSyncTickMsg struct{}

type loadingStartMsg struct{}
type loadingDoneMsg struct {
	tasks []*task.Task
	err   error
}

type statusUpdateMsg struct {
	err     error
	message string
}

type clearStatusMsg struct{}

func waitForFileChange(watcher *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		if watcher == nil {
			return nil
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					// Channel closed
					return nil
				}
				// Watch for Write (file modification), Create (new file), and Remove (file deletion) events
				if event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Remove == fsnotify.Remove {
					// Debounce: wait a bit for multiple writes to settle
					time.Sleep(100 * time.Millisecond)
					return fileChangedMsg{}
				}
				// If it's not a matching event, continue the loop to wait for the next one
			case err, ok := <-watcher.Errors:
				if !ok {
					// Channel closed
					return nil
				}
				log.Printf("Watcher error: %v", err)
				// Continue the loop to keep watching even after an error
			}
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle pending-child warning confirmation
	if m.showingPendingChildWarning {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				if m.watcher != nil {
					m.watcher.Close()
				}
				return m, tea.Quit
			case "y", "Y":
				t, kw := m.selectedTask, m.pendingWarningKeyword
				m.showingPendingChildWarning = false
				m.pendingWarningKeyword = ""
				return m, updateTaskStatusCmd(m.config, t, kw)
			case "n", "N", "esc", "q":
				m.showingPendingChildWarning = false
				m.selectedTask = nil
				m.pendingWarningKeyword = ""
				return m, nil
			}
		case statusUpdateMsg:
			m.showingPendingChildWarning = false
			m.selectedTask = nil
			m.pendingWarningKeyword = ""
			if msg.err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
			} else {
				m.statusMessage = msg.message
			}
			return m, tea.Batch(
				tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
				waitForFileChange(m.watcher),
			)
		case fileChangedMsg:
			return m, waitForFileChange(m.watcher)
		}
		return m, nil
	}

	// Handle detail view mode
	if m.showingDetailView {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				if m.watcher != nil {
					m.watcher.Close()
				}
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

	// Handle date picker mode
	if m.showingDatePicker {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				m.quitting = true
				if m.watcher != nil {
					m.watcher.Close()
				}
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
				return m, applyDatePickerCmd(m.config, m.datePicker)
			}
			return m, nil
		case datePickerResultMsg:
			m.showingDatePicker = false
			dp := m.datePicker
			m.datePicker = nil
			m.selectedTask = nil
			if msg.err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
			} else {
				m.statusMessage = msg.message
				_ = dp // keep reference until msg handled
			}
			return m, tea.Batch(
				tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
				waitForFileChange(m.watcher),
			)
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil
		case fileChangedMsg:
			return m, waitForFileChange(m.watcher)
		}
		return m, nil
	}

	// Handle status selector mode
	if m.showingStatusSelector {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				m.quitting = true
				if m.watcher != nil {
					m.watcher.Close()
				}
				return m, tea.Quit
			}
			m.statusPicker.Update(msg.String())
			if m.statusPicker.Cancelled {
				m.showingStatusSelector = false
				m.selectedTask = nil
				return m, nil
			}
			if m.statusPicker.Confirmed {
				kw := m.statusPicker.Selected
				if isCompletedKeyword(m.config, kw) && hasActiveChildren(m.selectedTask, m.config) {
					m.showingStatusSelector = false
					m.showingPendingChildWarning = true
					m.pendingWarningKeyword = kw
					return m, nil
				}
				return m, updateTaskStatusCmd(m.config, m.selectedTask, kw)
			}
			return m, nil
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil
		case statusUpdateMsg:
			m.showingStatusSelector = false
			m.selectedTask = nil
			if msg.err != nil {
				m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
			} else {
				m.statusMessage = msg.message
			}
			return m, tea.Batch(
				tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} }),
				waitForFileChange(m.watcher),
			)
		case fileChangedMsg:
			return m, waitForFileChange(m.watcher)
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit keys before list processes them
		if msg.String() == "ctrl+c" {
			m.quitting = true
			if m.watcher != nil {
				m.watcher.Close()
			}
			return m, tea.Quit
		}

		// Handle custom filtering
		if m.filtering {
			switch msg.String() {
			case "esc":
				// Cancel filtering
				m.filtering = false
				m.customFilter = ""
				m.list.SetItems(m.allItems)
				return m, nil
			case "enter":
				// Exit filtering mode but keep filter applied
				m.filtering = false
				return m, nil
			case "backspace":
				if len(m.customFilter) > 0 {
					m.customFilter = m.customFilter[:len(m.customFilter)-1]
					if m.customFilter == "" {
						m.filtering = false
						m.list.SetItems(m.allItems)
					} else {
						m.applyCustomFilter()
					}
				} else {
					m.filtering = false
					m.list.SetItems(m.allItems)
				}
				return m, nil
			default:
				// Add character to filter
				if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 { // Printable ASCII
					m.customFilter += string(msg.Runes[0])
					m.applyCustomFilter()
					return m, nil
				}
			}
		} else {
			// Handle esc: quit if not filtering
			if msg.String() == "esc" {
				if m.customFilter != "" {
					// Clear filter
					m.customFilter = ""
					m.list.SetItems(m.allItems)
					return m, nil
				}
			}

			// Handle q: quit only when not filtering
			if msg.String() == "q" {
				m.quitting = true
				if m.watcher != nil {
					m.watcher.Close()
				}
				return m, tea.Quit
			}

			// Start filtering (keep existing filter for editing)
			if msg.String() == "/" {
				m.filtering = true
				// Don't reset m.customFilter - keep existing filter for editing
				return m, nil
			}

			// Start fulltext search or edit existing fulltext search
			if msg.String() == "*" {
				m.filtering = true
				// If we're already in fulltext search mode, keep the existing search term for editing
				if !strings.HasPrefix(m.customFilter, "*") {
					// Not in fulltext search mode, start fresh
					m.customFilter = "*"
				}
				// If we're already in fulltext search mode, keep m.customFilter as is for editing
				return m, nil
			}

			// Switch to structured mode
			if msg.String() == "s" {
				if !m.structuredMode {
					m.structuredMode = true
					m.config.Todo.Structured = true
					m.list.Title = "Tasks (Zettelkasten)"
					return m, reloadTasksCmd()
				}
				return m, nil
			}

			// Switch to unstructured mode
			if msg.String() == "u" {
				if m.structuredMode {
					m.structuredMode = false
					m.config.Todo.Structured = false
					m.list.Title = "Tasks (All)"
					return m, reloadTasksCmd()
				}
				return m, nil
			}

			switch msg.String() {
			case "enter", "tab":
				// Only open editor if not actively filtering
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(taskItem); ok {
						m.savedFilter = m.customFilter
						return m, openEditorCmd(m.config, i.task, m.searchTerm)
					}
				}
			case "t":
				// Open status selector for the current task
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(taskItem); ok {
						m.selectedTask = i.task
						m.showingStatusSelector = true
						m.statusPicker = task.NewStatusPicker(i.task, m.config)
						return m, nil
					}
				}
			case "v":
				// Show detail view for the current task
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(taskItem); ok {
						m.selectedTask = i.task
						m.showingDetailView = true
						return m, nil
					}
				}
			case "S":
				// Open date picker for scheduled date
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(taskItem); ok {
						m.selectedTask = i.task
						m.datePicker = task.NewDatePicker(i.task, task.FieldScheduled)
						m.showingDatePicker = true
						return m, nil
					}
				}
			case "D":
				// Open date picker for due date
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(taskItem); ok {
						m.selectedTask = i.task
						m.datePicker = task.NewDatePicker(i.task, task.FieldDue)
						m.showingDatePicker = true
						return m, nil
					}
				}
			case "i":
				// Clock in to the current task
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(taskItem); ok {
						return m, clockInCmd(i.task)
					}
				}
			case "o":
				// Clock out of the current task
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(taskItem); ok {
						return m, clockOutCmd(i.task)
					}
				}
			}
		}
	case fileChangedMsg:
		// Reload tasks when files change
		tasks, err := task.ListTasks(m.config, m.project, m.config.Todo.ShowCompleted)
		if err == nil {
			task.DetectCycles(tasks)
			// Save current selection to restore after update
			var selectedKey string
			currentIdx := m.list.Index()
			if currentIdx >= 0 && currentIdx < len(m.tasks) {
				selectedKey = taskKey(m.tasks[currentIdx])
			}

			if m.config.GeneralConfig.Verbose {
				// Verbose mode: full re-sort and UI update (zettel column shown)
				m.tasks = tasks
				task.SortByPriority(m.tasks, m.config)
				sort.SliceStable(m.tasks, func(i, j int) bool {
					if m.tasks[i].Priority(m.config) == m.tasks[j].Priority(m.config) {
						if m.tasks[i].Project != m.tasks[j].Project {
							return m.tasks[i].Project < m.tasks[j].Project
						}
						if m.tasks[i].Title != m.tasks[j].Title {
							return m.tasks[i].Title < m.tasks[j].Title
						}
						return m.tasks[i].FilePath < m.tasks[j].FilePath
					}
					return false
				})
				m.tasks = task.GroupWithChildren(m.tasks)
				m.projectColWidth = calculateProjectColWidth(m.tasks)
				m.keywordColWidth = calculateKeywordColWidth(m.tasks)
				m.fractionColWidth = calculateFractionColWidth(m.tasks, m.config)
				items := make([]list.Item, len(m.tasks))
				for i, t := range m.tasks {
					items[i] = NewTaskItem(m.config, t, m.projectColWidth, m.keywordColWidth, m.fractionColWidth, m.calcMaxTitleWidth(), m.config.GeneralConfig.Verbose)
				}
				m.allItems = items
				if m.customFilter != "" {
					m.applyCustomFilter()
				} else {
					m.list.SetItems(items)
				}
			} else {
				// Non-verbose mode: preserve order, update tasks in place, append new tasks at end
				newTasks := appendNewTasksOnly(m.tasks, tasks, m.config)
				m.tasks = task.GroupWithChildren(newTasks)
				m.projectColWidth = calculateProjectColWidth(m.tasks)
				m.keywordColWidth = calculateKeywordColWidth(m.tasks)
				m.fractionColWidth = calculateFractionColWidth(m.tasks, m.config)
				items := make([]list.Item, len(m.tasks))
				for i, t := range m.tasks {
					items[i] = NewTaskItem(m.config, t, m.projectColWidth, m.keywordColWidth, m.fractionColWidth, m.calcMaxTitleWidth(), m.config.GeneralConfig.Verbose)
				}
				m.allItems = items
				if m.customFilter != "" {
					m.applyCustomFilter()
				} else {
					m.list.SetItems(items)
				}
			}

			// Restore cursor position
			restoreCursorPosition(&m.list, m.tasks, selectedKey, currentIdx)

			// Update watcher to monitor new files/directories
			updateWatcher(m.watcher, m.config, m.project)
		}
		// Continue watching for changes
		return m, waitForFileChange(m.watcher)
	case jiraSyncDoneMsg:
		for _, r := range msg.results {
			if r.err != nil {
				m.list.Title = fmt.Sprintf("⚠️ JIRA sync failed: [%s] %v", r.conn, r.err)
				m.jiraSyncLog = append(m.jiraSyncLog, fmt.Sprintf("%s  [%s] %v", time.Now().Format("15:04:05"), r.conn, r.err))
			} else {
				m.jiraSyncLog = append(m.jiraSyncLog, fmt.Sprintf("%s  [%s] ok (%d issues)", time.Now().Format("15:04:05"), r.conn, r.count))
			}
		}
		return m, jiraSyncTick()
	case jiraSyncTickMsg:
		return m, jiraSyncCmd(m.config)
	case loadingStartMsg:
		// Start loading
		m.loading = true
		return m, loadTasksCmd(m.config, m.project)
	case loadingDoneMsg:
		// Finish loading
		m.loading = false
		if msg.err == nil {
			m.tasks = msg.tasks
			// Sort tasks by priority: InProgress -> Active -> Someday -> Completed
			task.SortByPriority(m.tasks, m.config)
			// Secondary sort by project, then title, then file path for deterministic order
			sort.SliceStable(m.tasks, func(i, j int) bool {
				if m.tasks[i].Priority(m.config) == m.tasks[j].Priority(m.config) {
					if m.tasks[i].Project != m.tasks[j].Project {
						return m.tasks[i].Project < m.tasks[j].Project
					}
					if m.tasks[i].Title != m.tasks[j].Title {
						return m.tasks[i].Title < m.tasks[j].Title
					}
					return m.tasks[i].FilePath < m.tasks[j].FilePath
				}
				return false
			})
			m.tasks = task.GroupWithChildren(m.tasks)

			m.projectColWidth = calculateProjectColWidth(m.tasks)
			m.keywordColWidth = calculateKeywordColWidth(m.tasks)
			m.fractionColWidth = calculateFractionColWidth(m.tasks, m.config)
			items := make([]list.Item, len(m.tasks))
			for i, t := range m.tasks {
				items[i] = taskItem{config: m.config, task: t, projectColWidth: m.projectColWidth, keywordColWidth: m.keywordColWidth, fractionColWidth: m.fractionColWidth, maxTitleWidth: m.calcMaxTitleWidth(), verbose: m.config.GeneralConfig.Verbose}
			}
			m.allItems = items
			m.list.SetItems(items)
			m.applyCustomFilter() // Reapply any active filter
			m.list.ResetSelected()
		}
		return m, nil
	case editorFinishedMsg:
		// Log any errors from the editor
		if msg.err != nil {
			log.Printf("Editor error: %v", msg.err)
		}

		// Save current selection to restore after update
		var selectedKey string
		currentIdx := m.list.Index()
		if currentIdx >= 0 && currentIdx < len(m.tasks) {
			selectedKey = taskKey(m.tasks[currentIdx])
		}

		// Reload tasks after editing (including inbox)
		tasks, err := task.ListTasks(m.config, m.project, m.config.Todo.ShowCompleted)
		if err != nil {
			return m, tea.Quit
		}
		task.DetectCycles(tasks)

		if m.config.GeneralConfig.Verbose {
			// Verbose mode: full re-sort
			m.tasks = tasks
			task.SortByPriority(m.tasks, m.config)
			sort.SliceStable(m.tasks, func(i, j int) bool {
				if m.tasks[i].Priority(m.config) == m.tasks[j].Priority(m.config) {
					if m.tasks[i].Project != m.tasks[j].Project {
						return m.tasks[i].Project < m.tasks[j].Project
					}
					if m.tasks[i].Title != m.tasks[j].Title {
						return m.tasks[i].Title < m.tasks[j].Title
					}
					return m.tasks[i].FilePath < m.tasks[j].FilePath
				}
				return false
			})
			m.tasks = task.GroupWithChildren(m.tasks)
		} else {
			// Non-verbose mode: preserve existing order, append new tasks
			m.tasks = task.GroupWithChildren(mergeTasksPreservingOrder(m.tasks, tasks, m.config))
		}

		m.projectColWidth = calculateProjectColWidth(m.tasks)
		m.keywordColWidth = calculateKeywordColWidth(m.tasks)
		m.fractionColWidth = calculateFractionColWidth(m.tasks, m.config)
		items := make([]list.Item, len(m.tasks))
		for i, t := range m.tasks {
			items[i] = taskItem{config: m.config, task: t, projectColWidth: m.projectColWidth, keywordColWidth: m.keywordColWidth, fractionColWidth: m.fractionColWidth, maxTitleWidth: m.calcMaxTitleWidth(), verbose: m.config.GeneralConfig.Verbose}
		}
		m.allItems = items

		// Restore previous filter if there was one
		if m.savedFilter != "" {
			m.customFilter = m.savedFilter
			m.applyCustomFilter()
		} else {
			m.list.SetItems(items)
		}

		// Restore cursor position
		restoreCursorPosition(&m.list, m.tasks, selectedKey, currentIdx)

		return m, nil
	case statusUpdateMsg:
		m.showingStatusSelector = false
		m.selectedTask = nil
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.statusMessage = msg.message
		}
		// Clear status message after 3 seconds
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		})
	case clearStatusMsg:
		m.statusMessage = ""
		return m, nil
	case clockResultMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Clock: %v", msg.err)
		} else {
			m.statusMessage = msg.message
		}
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		})
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
		// Rebuild items with new title width
		newTitleWidth := m.calcMaxTitleWidth()
		for idx, item := range m.allItems {
			if ti, ok := item.(taskItem); ok {
				ti.maxTitleWidth = newTitleWidth
				m.allItems[idx] = ti
			}
		}
		m.list.SetItems(m.allItems)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) applyCustomFilter() {
	if m.customFilter == "" {
		m.list.SetItems(m.allItems)
		m.searchTerm = ""  // Clear search term
		return
	}

	// Check if this is a fulltext search (starts with "*")
	if strings.HasPrefix(m.customFilter, "*") {
		// Extract search term (everything after "*")
		searchTerm := strings.TrimSpace(m.customFilter[1:])
		m.searchTerm = searchTerm  // Store search term for editor
		if searchTerm == "" {
			m.list.SetItems(m.allItems)
			return
		}

		// Perform fulltext search across all task files
		results, err := task.SearchTasks(m.config, m.project, searchTerm)
		if err != nil {
			// On error, show all items
			m.list.SetItems(m.allItems)
			return
		}

		// Convert search results to task items for display
		var searchResultItems []list.Item
		for _, result := range results {
			// Create a pseudo-task to display the search result
			pseudoTask := &task.Task{
				Keyword:  "",  // Leave keyword empty
				Title:    result.Line,  // Don't include project name in brackets
				Project:  result.Project,
				Zettel:   result.ZettelID,
				FilePath: result.Path,
			}
			item := NewTaskItem(m.config, pseudoTask, m.projectColWidth, m.keywordColWidth, m.fractionColWidth, m.calcMaxTitleWidth(), m.config.GeneralConfig.Verbose)
			searchResultItems = append(searchResultItems, item)
		}

		m.list.SetItems(searchResultItems)
		return
	} else {
		m.searchTerm = ""  // Clear search term for non-fulltext search
	}

	// Extract tasks from all items
	var allTasks []*task.Task
	var itemToTask = make(map[*task.Task]list.Item)

	for _, item := range m.allItems {
		if taskItem, ok := item.(taskItem); ok {
			allTasks = append(allTasks, taskItem.task)
			itemToTask[taskItem.task] = item
		}
	}

	// Apply field-specific filtering
	filteredTasks := task.FilterTasks(allTasks, m.customFilter)

	// Convert back to list items
	var filteredItems []list.Item
	for _, t := range filteredTasks {
		if item, exists := itemToTask[t]; exists {
			filteredItems = append(filteredItems, item)
		}
	}

	// Skip SetItems if filter returned everything (avoids flicker on incomplete prefixes like "@d:")
	if len(filteredItems) == len(m.allItems) {
		return
	}

	m.list.SetItems(filteredItems)

	// If no items match the filter, show a message
	if len(filteredItems) == 0 {
		m.list.SetItems([]list.Item{list.Item(&noResultsItem{})})
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	// Show detail view overlay if active
	if m.showingDetailView {
		return m.renderDetailView()
	}

	// Show pending-child warning overlay if active
	if m.showingPendingChildWarning {
		return m.renderPendingChildWarning()
	}

	// Show date picker overlay if active
	if m.showingDatePicker && m.datePicker != nil {
		return m.datePicker.View(m.termWidth / 2)
	}

	// Show status selector overlay if active
	if m.showingStatusSelector {
		return m.renderStatusSelector()
	}

	view := m.list.View()

	// Add custom pagination/count info at the top
	totalItems := len(m.list.Items())
	if totalItems > 0 {
		p := m.list.Paginator
		totalPages := p.TotalPages

		var paginationText string
		if totalPages > 1 {
			currentPage := p.Page
			itemsPerPage := p.PerPage
			startIdx := currentPage * itemsPerPage
			endIdx := startIdx + itemsPerPage
			if endIdx > totalItems {
				endIdx = totalItems
			}
			paginationText = fmt.Sprintf("Showing %d-%d of %d • Page %d/%d",
				startIdx+1, endIdx, totalItems, currentPage+1, totalPages)
		} else if m.customFilter != "" {
			paginationText = fmt.Sprintf("%d matches", totalItems)
		}

		if paginationText != "" {
			paginationInfo := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(paginationText)

			lines := strings.Split(view, "\n")
			if len(lines) >= 1 {
				result := []string{lines[0], paginationInfo}
				result = append(result, lines[1:]...)
				view = strings.Join(result, "\n")
			}
		}
	}

	// Show filter status if active
	if m.filtering || m.customFilter != "" {
		var filterText string
		if m.filtering {
			if strings.HasPrefix(m.customFilter, "*") {
				searchTerm := strings.TrimSpace(m.customFilter[1:])
				filterText = fmt.Sprintf("Fulltext search: %s▓", searchTerm)
			} else {
				filterText = fmt.Sprintf("Filter: %s▓", m.customFilter) // Show cursor when actively typing
			}
		} else if m.customFilter != "" {
			if strings.HasPrefix(m.customFilter, "*") {
				searchTerm := strings.TrimSpace(m.customFilter[1:])
				filterText = fmt.Sprintf("Fulltext search: %s", searchTerm)
			} else {
				filterText = fmt.Sprintf("Filter: %s", m.customFilter)
			}
		}

		filterInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Background(lipgloss.Color("0")).
			Padding(0, 1).
			Render(filterText)
		view = filterInfo + "\n" + view
	}

	// Show status message by replacing the help bar line
	if m.statusMessage != "" {
		lines := strings.Split(view, "\n")
		statusLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render(m.statusMessage)
		if len(lines) > 1 {
			lines[len(lines)-1] = statusLine
		}
		view = strings.Join(lines, "\n")
	}

	return view
}

// renderStatusSelector renders the status selector popup
func (m model) renderStatusSelector() string {
	return m.statusPicker.View(m.termWidth)
}

// renderPendingChildWarning renders the warning overlay for completing a parent with active children
func (m model) renderPendingChildWarning() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("9")).
		Padding(1, 2)

	activeCount := 0
	for _, child := range m.selectedTask.Children {
		if child.IsActive(m.config) || child.IsInProgress(m.config) {
			activeCount++
		}
	}

	var content strings.Builder

	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	content.WriteString(warningStyle.Render("⚠  Incomplete children"))
	content.WriteString("\n\n")

	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	content.WriteString(infoStyle.Render(fmt.Sprintf(
		"%d active/in-progress child task(s) still pending.\nMark parent as %s anyway?",
		activeCount, m.pendingWarningKeyword,
	)))
	content.WriteString("\n\n")

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	content.WriteString(helpStyle.Render("y: confirm • n/esc: cancel"))

	return boxStyle.Render(content.String())
}

// renderDetailView renders the task detail overlay showing full raw content from file
func (m model) renderDetailView() string {
	if m.selectedTask == nil {
		return ""
	}

	// Box width = terminal width minus outer margin (2 per side)
	boxWidth := m.termWidth - 4
	if boxWidth < 40 {
		boxWidth = 40
	}
	// Content width = box width minus border (1 per side) minus padding (2 per side)
	contentWidth := boxWidth - 6

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(boxWidth)

	t := m.selectedTask

	// Read raw block from source file
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

	// Render each line with task-syntax and markdown coloring, then hard-wrap
	var wrapped []string
	for _, line := range lines {
		rendered := m.renderDetailLine(line)
		if len(line) <= contentWidth {
			wrapped = append(wrapped, rendered)
		} else {
			// Wrap based on raw length, render each segment
			for len(line) > contentWidth {
				segment := line[:contentWidth]
				wrapped = append(wrapped, m.renderDetailLine(segment))
				line = line[contentWidth:]
			}
			if line != "" {
				wrapped = append(wrapped, m.renderDetailLine(line))
			}
		}
	}

	// Metadata footer
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	wrapped = append(wrapped, "")
	wrapped = append(wrapped, metaStyle.Render(fmt.Sprintf("── %s", t.FilePath)))

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	wrapped = append(wrapped, "")
	wrapped = append(wrapped, helpStyle.Render("esc/q/v: close"))

	return boxStyle.Render(strings.Join(wrapped, "\n"))
}

// renderDetailLine colorizes a raw file line with task-syntax highlighting and markdown.
func (m model) renderDetailLine(line string) string {
	textStyle := colors.taskColor

	// Check if this line is a task line (KEYWORD: ...)
	stripped, _ := task.StripLinePrefix(line)
	kwRe := regexp.MustCompile(`^([A-Z]+):\s`)
	if kwMatch := kwRe.FindStringSubmatch(stripped); len(kwMatch) > 0 {
		kw := kwMatch[1]
		if task.IsKeywordValid(m.config, kw) {
			// Determine keyword style
			var kwStyle lipgloss.Style
			t := &task.Task{Keyword: kw}
			if t.IsInProgress(m.config) {
				kwStyle = colors.inProgressColor
			} else if t.IsActive(m.config) {
				kwStyle = colors.activeColor
			} else if t.IsSomeday(m.config) {
				kwStyle = colors.somedayColor
			} else {
				kwStyle = colors.completedColor
				textStyle = colors.completedTaskColor
			}
			// Replace the keyword in the line with styled version
			line = strings.Replace(line, kw+":", kwStyle.Render(kw)+":", 1)
		}
	}

	// Colorize tags (#tag)
	tagRe := regexp.MustCompile(`(#[^\s]+)`)
	line = tagRe.ReplaceAllStringFunc(line, func(match string) string {
		tagName := match[1:]
		isSpecial := false
		for _, st := range m.config.Todo.SpecialTags {
			if tagName == st || strings.HasPrefix(tagName, st+":") {
				isSpecial = true
				break
			}
		}
		if isSpecial {
			return colors.specialTagColor.Render(match)
		}
		return colors.tagColor.Render(match)
	})

	// Colorize dates (@date, @s:date, @d:date)
	dateRe := regexp.MustCompile(`(@(?:s:|d:)?[^\s]+)`)
	line = dateRe.ReplaceAllStringFunc(line, func(match string) string {
		return colors.dateColor.Render(match)
	})

	// Colorize assignees (>> name)
	assigneeRe := regexp.MustCompile(`(>>\s*[^\s#@^]+)`)
	line = assigneeRe.ReplaceAllStringFunc(line, func(match string) string {
		return colors.assigneeColor.Render(match)
	})

	// Colorize references (^id)
	refRe := regexp.MustCompile(`(\^[^\s]+)`)
	line = refRe.ReplaceAllStringFunc(line, func(match string) string {
		return colors.prjColor.Render(match)
	})

	// Colorize task IDs ([id])
	idRe := regexp.MustCompile(`(\[[^\]]+\])`)
	line = idRe.ReplaceAllStringFunc(line, func(match string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Render(match)
	})

	// Apply markdown rendering (bold, italic, code, URLs, bullets)
	line = task.RenderMarkdownDescription(line, textStyle)

	return line
}

type editorFinishedMsg struct{ err error }


func isCompletedKeyword(cfg *configpkg.Config, keyword string) bool {
	return task.IsCompletedKeyword(cfg, keyword)
}

func hasActiveChildren(t *task.Task, cfg *configpkg.Config) bool {
	return task.HasActiveChildren(t, cfg)
}

type datePickerResultMsg struct {
	message string
	err     error
}

func applyDatePickerCmd(_ *configpkg.Config, dp *task.DatePicker) tea.Cmd {
	return func() tea.Msg {
		scheduledAt, dueAt, removeScheduled, removeDue := dp.Result()
		if err := task.SetTaskDate(dp.Task, scheduledAt, dueAt, removeScheduled, removeDue); err != nil {
			return datePickerResultMsg{err: err}
		}

		commitMsg := fmt.Sprintf("Schedule task: %s", dp.Task.Title)
		kgit.CommitFile(dp.Task.FilePath, commitMsg, true)

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

type clockResultMsg struct {
	message string
	err     error
}

func clockInCmd(t *task.Task) tea.Cmd {
	return func() tea.Msg {
		err := task.ClockIn(t)
		if err != nil {
			return clockResultMsg{err: err}
		}
		return clockResultMsg{message: "Clocked in"}
	}
}

func clockOutCmd(t *task.Task) tea.Cmd {
	return func() tea.Msg {
		err := task.ClockOut(t)
		if err != nil {
			return clockResultMsg{err: err}
		}
		return clockResultMsg{message: "Clocked out"}
	}
}

// updateTaskStatusCmd creates a command that updates the task status.
// For recurring tasks being marked complete, it advances the date instead.
func updateTaskStatusCmd(cfg *configpkg.Config, t *task.Task, newKeyword string) tea.Cmd {
	return func() tea.Msg {
		if t == nil {
			return statusUpdateMsg{err: fmt.Errorf("no task selected")}
		}

		oldKeyword := t.Keyword

		// Check if this is a completion of a recurring task
		if isCompletedKeyword(cfg, newKeyword) {
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

		// Normal (non-recurring) status update
		if err := task.UpdateTaskStatus(t, newKeyword); err != nil {
			return statusUpdateMsg{err: err}
		}

		// Record state transition for all status changes
		if err := task.RecordStateTransition(t, oldKeyword, newKeyword); err != nil {
			return statusUpdateMsg{err: fmt.Errorf("status updated but failed to record transition: %w", err)}
		}

		// Commit the change if in a git repo
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

func reloadTasksCmd() tea.Cmd {
	return func() tea.Msg {
		return loadingStartMsg{}
	}
}

func loadTasksCmd(cfg *configpkg.Config, project string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := task.ListTasks(cfg, project, cfg.Todo.ShowCompleted)
		if err == nil {
			task.DetectCycles(tasks)
		}
		return loadingDoneMsg{tasks: tasks, err: err}
	}
}

func openEditorCmd(cfg *configpkg.Config, t *task.Task, searchTerm string) tea.Cmd {
	var filePath string
	if t.Project == "inbox" {
		// Inbox tasks are in the inbox.md file directly
		filePath = cfg.GetInboxFilePath()
	} else if cfg.Todo.Structured {
		// Structured mode: construct path from project/zettel
		filePath = filepath.Join(cfg.Directories.Projects, t.Project, "notes", t.Zettel, "README.md")
	} else {
		// Unstructured mode: use the original file path where task was found
		filePath = t.FilePath
	}

	// Use stored line number if available, otherwise search
	lineNum := t.LineNum
	if lineNum == 0 {
		var err error
		lineNum, err = findTaskLine(filePath, t)
		if err != nil {
			return func() tea.Msg {
				return editorFinishedMsg{err: err}
			}
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Expand ~ in editor path
	if strings.HasPrefix(editor, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			editor = filepath.Join(home, editor[2:])
		}
	}

	// Parse editor command (may contain arguments like "emacs -nw")
	editorParts := strings.Fields(editor)
	editorCmd := editorParts[0]
	editorArgs := editorParts[1:]

	// Get the base name of the editor to determine syntax
	editorBase := filepath.Base(editorCmd)

	// Handle search term if provided (similar to note command)
	if searchTerm != "" {
		switch editorBase {
		case "vim", "nvim", "vi":
			editorArgs = append(editorArgs, fmt.Sprintf("+/%s", searchTerm))
		case "emacs":
			editorArgs = append(editorArgs, "--eval", fmt.Sprintf("(progn (goto-char (point-min)) (search-forward \"%s\" nil t))", searchTerm))
		}
	}

	// Handle line number navigation
	if strings.Contains(editorBase, "vim") || strings.Contains(editorBase, "nvim") {
		if searchTerm == "" {
			// Only add line number if no search term
			editorArgs = append(editorArgs, fmt.Sprintf("+%d", lineNum))
		}
		editorArgs = append(editorArgs, filePath)
	} else if strings.Contains(editorBase, "emacs") {
		if searchTerm == "" {
			// Only add line number if no search term
			editorArgs = append(editorArgs, fmt.Sprintf("+%d", lineNum))
		}
		editorArgs = append(editorArgs, filePath)
	} else if strings.Contains(editorBase, "nano") {
		editorArgs = append(editorArgs, fmt.Sprintf("+%d", lineNum), filePath)
	} else if strings.Contains(editorBase, "code") {
		editorArgs = append(editorArgs, "-g", fmt.Sprintf("%s:%d", filePath, lineNum))
	} else {
		// Unknown editor, just pass the file
		editorArgs = append(editorArgs, filePath)
	}

	c := exec.Command(editorCmd, editorArgs...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func findTaskLine(filePath string, t *task.Task) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Try matching by ID first (most reliable for JIRA tasks)
	idPrefix := ""
	if t.ID != "" {
		idPrefix = fmt.Sprintf("[%s]", t.ID)
	}
	titleSearch := fmt.Sprintf("%s: %s", t.Keyword, t.Title)

	for scanner.Scan() {
		lineNum++
		stripped, _ := task.StripLinePrefix(scanner.Text())
		if idPrefix != "" && strings.Contains(stripped, idPrefix) {
			return lineNum, nil
		}
		if strings.HasPrefix(stripped, titleSearch) {
			return lineNum, nil
		}
	}

	return 1, scanner.Err()
}

func calculateProjectColWidth(tasks []*task.Task) int {
	maxLen := 15
	for _, t := range tasks {
		if len(t.Project) > maxLen {
			maxLen = len(t.Project)
		}
	}
	return maxLen
}

func calculateKeywordColWidth(tasks []*task.Task) int {
	maxLen := 4 // Minimum width for short keywords like "TODO"
	for _, t := range tasks {
		if len(t.Keyword) > maxLen {
			maxLen = len(t.Keyword)
		}
	}
	return maxLen
}

func calculateFractionColWidth(tasks []*task.Task, cfg *configpkg.Config) int {
	maxLen := 0
	for _, t := range tasks {
		done, total := t.PendingChildCount(cfg)
		if total > 0 && done < total {
			w := len(fmt.Sprintf("[%d/%d]", done, total))
			if w > maxLen {
				maxLen = w
			}
		}
	}
	return maxLen
}

// setupWatcher creates a new watcher and watches all relevant directories
func setupWatcher(config *configpkg.Config, project string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	updateWatcher(watcher, config, project)
	return watcher, nil
}

// updateWatcher updates the watcher to monitor all relevant directories and files
func updateWatcher(watcher *fsnotify.Watcher, config *configpkg.Config, project string) {
	if watcher == nil {
		return
	}

	// Get list of directories to watch
	dirsToWatch := getWatchDirectories(config, project)

	// Remove all current watches (fsnotify doesn't have a list method, so we track what we add)
	// Since we can't efficiently remove specific watches, we'll just add new ones
	// fsnotify handles duplicate adds gracefully

	for _, dir := range dirsToWatch {
		// Ignore errors - directory might not exist yet or might already be watched
		watcher.Add(dir)
	}

	// Watch the inbox file's parent directory so external edits trigger a refresh
	if inboxPath := config.GetInboxFilePath(); inboxPath != "" {
		watcher.Add(filepath.Dir(inboxPath))
	}
}

// maxWatchDirs limits the number of directories to watch in unstructured mode
// to avoid exhausting file descriptors. Structured mode has no cap since it
// only watches zettel directories (bounded by actual project/note count).
const maxWatchDirs = 1000

// getWatchDirectories returns a list of directories that should be watched.
// In structured mode, watches only the zettel directories (PRJDIR/*/notes/*/).
// In unstructured mode, watches up to maxWatchDirs directories, prioritizing shallower ones.
func getWatchDirectories(config *configpkg.Config, project string) []string {
	if config.Todo.Structured {
		return getStructuredWatchDirs(config, project)
	}
	return getUnstructuredWatchDirs(config, project)
}

// getStructuredWatchDirs returns directories for structured (zettelkasten) mode.
// Only watches: PRJDIR, PRJDIR/*, PRJDIR/*/notes, and PRJDIR/*/notes/*
func getStructuredWatchDirs(config *configpkg.Config, project string) []string {
	var dirs []string
	prjDir := config.Directories.Projects

	if project != "" && project != "*" {
		// Specific project: watch project dir and its notes subdirs
		projectDir := filepath.Join(prjDir, project)
		notesDir := filepath.Join(projectDir, "notes")
		dirs = append(dirs, projectDir, notesDir)

		// Add zettel directories under notes/
		entries, err := os.ReadDir(notesDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, filepath.Join(notesDir, e.Name()))
				}
			}
		}
	} else {
		// All projects: watch PRJDIR and each project's notes structure
		dirs = append(dirs, prjDir)

		entries, err := os.ReadDir(prjDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					projectDir := filepath.Join(prjDir, e.Name())
					notesDir := filepath.Join(projectDir, "notes")
					dirs = append(dirs, projectDir, notesDir)

					// Add zettel directories under notes/
					zettelEntries, err := os.ReadDir(notesDir)
					if err == nil {
						for _, z := range zettelEntries {
							if z.IsDir() {
								dirs = append(dirs, filepath.Join(notesDir, z.Name()))
							}
						}
					}
				}
			}
		}
	}

	return dirs
}

// getUnstructuredWatchDirs returns directories for unstructured mode.
// Watches up to maxWatchDirs directories, prioritizing shallower ones.
func getUnstructuredWatchDirs(config *configpkg.Config, project string) []string {
	var dirs []string

	rootDir := config.Directories.Projects
	if project != "" && project != "*" {
		rootDir = filepath.Join(config.Directories.Projects, project)
	}

	// Collect directories with depth info for prioritization
	rootDepth := strings.Count(rootDir, string(filepath.Separator))
	type dirInfo struct {
		path  string
		depth int
	}
	var allDirs []dirInfo

	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			depth := strings.Count(path, string(filepath.Separator)) - rootDepth
			allDirs = append(allDirs, dirInfo{path: path, depth: depth})
		}
		return nil
	})

	// Sort by depth (shallower first) to prioritize important directories
	sort.Slice(allDirs, func(i, j int) bool {
		return allDirs[i].depth < allDirs[j].depth
	})

	// Take only up to maxWatchDirs directories
	for i, d := range allDirs {
		if i >= maxWatchDirs {
			break
		}
		dirs = append(dirs, d.path)
	}

	return dirs
}

// taskKey creates a unique identifier for a task (includes keyword for cursor restoration)
func taskKey(t *task.Task) string {
	return t.FilePath + ":" + t.Keyword + ":" + t.Title
}

// taskIdentityKey creates a stable identifier for a task that doesn't change when status changes.
// Used for merge operations to recognize the same task after status updates.
func taskIdentityKey(t *task.Task) string {
	return t.FilePath + ":" + t.Title
}

// restoreCursorPosition finds the task by key and restores cursor, or clamps to bounds if deleted.
// First tries exact match (including keyword), then tries identity match (FilePath+Title) for
// when the task status has changed.
func restoreCursorPosition(l *list.Model, tasks []*task.Task, selectedKey string, fallbackIdx int) {
	if len(tasks) == 0 {
		return
	}

	// Try to find the previously selected task by exact key
	if selectedKey != "" {
		for i, t := range tasks {
			if taskKey(t) == selectedKey {
				l.Select(i)
				return
			}
		}

		// Exact key not found - try identity match (task status may have changed)
		// Extract identity from selectedKey: "FilePath:Keyword:Title" -> "FilePath:Title"
		parts := strings.SplitN(selectedKey, ":", 3)
		if len(parts) == 3 {
			selectedIdentity := parts[0] + ":" + parts[2] // FilePath:Title
			for i, t := range tasks {
				if taskIdentityKey(t) == selectedIdentity {
					l.Select(i)
					return
				}
			}
		}
	}

	// Task was deleted or not found - clamp to bounds
	if fallbackIdx >= len(tasks) {
		fallbackIdx = len(tasks) - 1
	}
	if fallbackIdx < 0 {
		fallbackIdx = 0
	}
	l.Select(fallbackIdx)
}

// appendNewTasksOnly keeps existing tasks in place when priority unchanged, repositions tasks
// whose priority changed, removes deleted tasks, and appends new tasks at the correct position.
// Uses taskIdentityKey (FilePath+Title) to match tasks even when their status changes.
func appendNewTasksOnly(existing, incoming []*task.Task, cfg *configpkg.Config) []*task.Task {
	// Build a map of incoming tasks by identity key for matching
	incomingByIdentity := make(map[string]*task.Task)
	for _, t := range incoming {
		incomingByIdentity[taskIdentityKey(t)] = t
	}

	// Build maps for existing tasks
	existingByIdentity := make(map[string]*task.Task)
	existingIdentityKeys := make(map[string]bool)
	for _, t := range existing {
		key := taskIdentityKey(t)
		existingByIdentity[key] = t
		existingIdentityKeys[key] = true
	}

	// Separate tasks into: unchanged (keep in place), priority-changed, and new
	var unchangedTasks []*task.Task
	var priorityChangedTasks []*task.Task

	for _, t := range existing {
		identityKey := taskIdentityKey(t)
		if updated, ok := incomingByIdentity[identityKey]; ok {
			// Task still exists - check if priority changed
			if t.Priority(cfg) == updated.Priority(cfg) {
				// Priority unchanged - keep in place with updated data
				unchangedTasks = append(unchangedTasks, updated)
			} else {
				// Priority changed - will be repositioned
				priorityChangedTasks = append(priorityChangedTasks, updated)
			}
		}
		// Task no longer exists - don't add it (it was deleted)
	}

	// Find truly new tasks (not in existing at all)
	var newTasks []*task.Task
	for _, t := range incoming {
		if !existingIdentityKeys[taskIdentityKey(t)] {
			newTasks = append(newTasks, t)
		}
	}

	// Start with unchanged tasks (they keep their positions)
	result := unchangedTasks

	// Insert priority-changed and new tasks at correct positions
	tasksToInsert := append(priorityChangedTasks, newTasks...)
	task.SortByPriority(tasksToInsert, cfg)

	for _, insertTask := range tasksToInsert {
		insertPriority := insertTask.Priority(cfg)
		inserted := false

		// Find the last task with the same or higher priority and insert after it
		for i := len(result) - 1; i >= 0; i-- {
			if result[i].Priority(cfg) <= insertPriority {
				// Insert after this position
				result = append(result[:i+1], append([]*task.Task{insertTask}, result[i+1:]...)...)
				inserted = true
				break
			}
		}

		if !inserted {
			// No task with same or higher priority found, prepend
			result = append([]*task.Task{insertTask}, result...)
		}
	}

	return result
}

// mergeTasksPreservingOrder merges new tasks into existing list while preserving order.
// - Keeps existing tasks in their current positions (updating if status changed)
// - Removes tasks that no longer exist
// - Appends new tasks at the end of their priority group
// Uses taskIdentityKey (FilePath+Title) to match tasks even when status changes.
func mergeTasksPreservingOrder(existing, incoming []*task.Task, cfg *configpkg.Config) []*task.Task {
	// Build a map of incoming tasks by identity key for matching
	incomingByIdentity := make(map[string]*task.Task)
	for _, t := range incoming {
		incomingByIdentity[taskIdentityKey(t)] = t
	}

	// Build a set of existing identity keys
	existingIdentityKeys := make(map[string]bool)
	for _, t := range existing {
		existingIdentityKeys[taskIdentityKey(t)] = true
	}

	// Keep existing tasks that still exist (preserving order), updating their state
	var result []*task.Task
	for _, t := range existing {
		identityKey := taskIdentityKey(t)
		if updated, ok := incomingByIdentity[identityKey]; ok {
			result = append(result, updated)
		}
		// Task no longer exists - don't add it
	}

	// Find new tasks (in incoming but not in existing)
	var newTasks []*task.Task
	for _, t := range incoming {
		if !existingIdentityKeys[taskIdentityKey(t)] {
			newTasks = append(newTasks, t)
		}
	}

	// Sort new tasks by priority for proper insertion
	task.SortByPriority(newTasks, cfg)

	// Append new tasks at the end of their respective priority groups
	for _, newTask := range newTasks {
		newPriority := newTask.Priority(cfg)
		inserted := false

		// Find the last task with the same or higher priority and insert after it
		for i := len(result) - 1; i >= 0; i-- {
			if result[i].Priority(cfg) <= newPriority {
				// Insert after this position
				result = append(result[:i+1], append([]*task.Task{newTask}, result[i+1:]...)...)
				inserted = true
				break
			}
		}

		if !inserted {
			// No task with same or higher priority found, prepend
			result = append([]*task.Task{newTask}, result...)
		}
	}

	return result
}

func main() {
	config, err := configpkg.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize colors from config
	InitializeColors(config)

	// Parse flags
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-v" || arg == "--verbose" {
			config.GeneralConfig.Verbose = true
			// Remove this flag from args
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	// JIRA sync runs as background tick inside the TUI (see Init/jiraSyncCmd)

	if len(args) == 0 {
		// Interactive TUI mode
		showInteractiveTUI(config, "")
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "colors":
		if err := colorspkg.Print(config); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		printHelp()
	case "ls", "list":
		project := ""
		if len(args) > 1 {
			project = args[1]
		}
		tasks, err := task.ListTasks(config, project, config.Todo.ShowCompleted)
		if err != nil {
			log.Fatal(err)
		}
		task.DetectCycles(tasks)
		// Sort tasks by priority: InProgress -> Active -> Someday -> Completed
		task.SortByPriority(tasks, config)
		// Secondary sort by project name within same priority
		sort.SliceStable(tasks, func(i, j int) bool {
			if tasks[i].Priority(config) == tasks[j].Priority(config) {
				return tasks[i].Project < tasks[j].Project
			}
			return false
		})
		printTasksPlain(config, tasks)
	case "projects":
		summary, err := task.SummarizeProjects(config)
		if err != nil {
			log.Fatal(err)
		}
		showProjectsTable(summary)
	case "pl":
		summary, err := task.SummarizeProjects(config)
		if err != nil {
			log.Fatal(err)
		}
		printProjectsList(summary)
	case "clock-in":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: todo clock-in <project> <keyword> <title>")
			os.Exit(1)
		}
		project, keyword, title := args[1], args[2], strings.Join(args[3:], " ")
		tasks, err := task.ListTasks(config, project, true)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range tasks {
			if t.Keyword == keyword && strings.Contains(strings.ToLower(t.Title), strings.ToLower(title)) {
				if err := task.ClockIn(t); err != nil {
					log.Fatal(err)
				}
				fmt.Printf("Clocked in: %s\n", t.Title)
				return
			}
		}
		fmt.Fprintln(os.Stderr, "task not found")
		os.Exit(1)
	case "clock-out":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: todo clock-out <project> <keyword> <title>")
			os.Exit(1)
		}
		project, keyword, title := args[1], args[2], strings.Join(args[3:], " ")
		tasks, err := task.ListTasks(config, project, true)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range tasks {
			if t.Keyword == keyword && strings.Contains(strings.ToLower(t.Title), strings.ToLower(title)) {
				if err := task.ClockOut(t); err != nil {
					log.Fatal(err)
				}
				fmt.Printf("Clocked out: %s\n", t.Title)
				return
			}
		}
		fmt.Fprintln(os.Stderr, "task not found")
		os.Exit(1)
	case "mcp":
		// Start MCP server on stdio
		mcpServer := task.NewMCPServer(config)
		ctx := context.Background()
		if err := mcpServer.Run(ctx); err != nil {
			log.Fatal(err)
		}
	case "jira-auth":
		if !config.HasJira() {
			fmt.Fprintln(os.Stderr, "JIRA not configured. Add [[jira.connections]] to ~/.config/karya/config.toml.")
			os.Exit(1)
		}
		// Determine which connection to auth
		var conn *configpkg.JiraConnection
		if len(args) > 2 {
			name := args[2]
			for i := range config.Jira.Connections {
				if config.Jira.Connections[i].Name == name {
					conn = &config.Jira.Connections[i]
					break
				}
			}
			if conn == nil {
				fmt.Fprintf(os.Stderr, "Connection %q not found in config.\n", name)
				os.Exit(1)
			}
		} else if len(config.Jira.Connections) == 1 {
			conn = &config.Jira.Connections[0]
		} else {
			fmt.Fprintln(os.Stderr, "Multiple connections configured. Specify which: todo jira-auth <name>")
			for _, c := range config.Jira.Connections {
				fmt.Fprintf(os.Stderr, "  - %s\n", c.Name)
			}
			os.Exit(1)
		}
		store := jira.NewTokenStore(conn.Name)
		ctx := context.Background()
		if err := store.RunAuthFlow(ctx, conn.Endpoint); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("JIRA authentication successful for %q. Token stored.\n", conn.Name)
	default:
		// Project name - show interactive TUI for that project
		showInteractiveTUI(config, subcommand)
	}
}

func printHelp() {
	help := `todo - Interactive task manager using markdown files

USAGE:
    todo [OPTIONS] [COMMAND]

OPTIONS:
    -v, --verbose       Show additional details like Zettel ID column

COMMANDS:
    (no command)        Show interactive TUI with all tasks
    ls [PROJECT]        List tasks in plain text format (for scripting)
    projects            Show project summary table with task counts
    pl                  Show project list in plain text format
    clock-in <p> <k> <t> Clock in on a task (project, keyword, title)
    clock-out <p> <k> <t> Clock out of a task (project, keyword, title)
    mcp                 Start MCP server (stdio) for AI agent integration
    jira-auth           Authenticate with JIRA (OAuth browser flow, one-time setup)
    <project-name>      Show interactive TUI filtered to specific project
    -h, --help, help    Show this help message

INTERACTIVE MODE:
    The TUI features live file monitoring - the task list automatically updates
    when files are modified (by external editors or tools like 'zet', 'note'),
    when new projects are created, or when new files are added to existing projects.
    No manual refresh needed!

    NAVIGATION:
    j/k or ↑/↓          Navigate tasks (vim-style)
    g / G               Jump to top / bottom
    Ctrl+d / Ctrl+u     Page down / up (vim-style)
    Ctrl+f / Ctrl+b     Page down / up (emacs-style)
    PgDn / PgUp         Page down / up

    ACTIONS:
    Type '/' to filter  Filter tasks list (See FILTERING below)
    Enter               Edit selected task at specific line / Exit filter mode
    t                   Change task status (opens keyword selector)
    s                   Switch to structured mode (zettelkasten format)
    u                   Switch to unstructured mode (all .md files)
    q                   Quit
    Esc                 Exit filter mode or clear filter
    Ctrl+C              Quit

FILTERING:
    text                Search for 'text' across all task fields
    >> assignee         Filter by assignee (e.g., ">> alice")
    #tag                Filter by tag (e.g., "#urgent")
    @date               Filter by scheduled date (e.g., "@2025-01-15")
    @s:date             Filter by scheduled date (e.g., "@s:2025-01-15")
    @d:date             Filter by due date (e.g., "@d:2025-01-20")

EXAMPLES:
    todo                           # Show all tasks in interactive TUI
    todo -v                        # Show all tasks with Zettel ID column
    todo --verbose ls              # List all tasks in verbose mode
    todo ls myproject              # List tasks for specific project
    todo -v myproject              # Show tasks for myproject with details
    todo projects                  # Show project summary table
    todo pl                        # Show project list (plain text)
    todo mcp                       # Start MCP server for AI agents
    SHOW_COMPLETED=true todo       # Show completed tasks in TUI
    STRUCTURED=false todo          # Use unstructured mode (all .md files)
    
    # In interactive mode, press '/' and then type:
    #   >> alice                   # Show tasks assigned to alice
    #   #urgent                    # Show tasks with #urgent tag
    #   @2025-01-15               # Show tasks scheduled for Jan 15, 2025
    #   @d:2025-01-20             # Show tasks due on Jan 20, 2025

ENVIRONMENT VARIABLES:
    EDITOR              Editor to use (supports vim, nvim, emacs, nano, code)
                        Can include arguments, e.g., EDITOR="emacs -nw"

    SHOW_COMPLETED      Show completed tasks (true/false, default: false)
                        Can also be set in ~/.config/karya/config.toml

    STRUCTURED          Use structured zettelkasten format (true/false, default: true)
                        - true: Search for project/notes/zettelID/README.md files
                        - false: Search all .md files in project directory tree
                        Can also be set in ~/.config/karya/config.toml

    VERBOSE             Show additional details like Zettel ID column (true/false, default: false)
                        Can also be set in ~/.config/karya/config.toml
                        Note: -v/--verbose flag takes precedence over this variable

CONFIGURATION:
    See config file: ~/.config/karya/config.toml.example for full configuration options.
    Command-line flags take precedence over environment variables and config file settings.
`
	fmt.Print(help)
}

func showInteractiveTUI(config *configpkg.Config, project string) {
	tasks, err := task.ListTasks(config, project, config.Todo.ShowCompleted)
	if err != nil {
		log.Fatal(err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return
	}

	// Set up file watcher to monitor directories
	watcher, err := setupWatcher(config, project)
	if err != nil {
		log.Printf("Warning: Could not create file watcher: %v", err)
	}

	// Load tasks including inbox tasks
	tasks, err = task.ListTasks(config, project, config.Todo.ShowCompleted)
	if err != nil {
		log.Fatal(err)
	}
	task.DetectCycles(tasks)

	// Sort tasks by priority: InProgress -> Active -> Someday -> Completed
	task.SortByPriority(tasks, config)
	// Secondary sort by project name within same priority
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Priority(config) == tasks[j].Priority(config) {
			return tasks[i].Project < tasks[j].Project
		}
		return false
	})
	tasks = task.GroupWithChildren(tasks)

	projectColWidth := calculateProjectColWidth(tasks)
	keywordColWidth := calculateKeywordColWidth(tasks)
	fractionColWidth := calculateFractionColWidth(tasks, config)
	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = taskItem{config: config, task: t, projectColWidth: projectColWidth, keywordColWidth: keywordColWidth, fractionColWidth: fractionColWidth, maxTitleWidth: 40, verbose: config.GeneralConfig.Verbose}
	}

	delegate := taskDelegate{DefaultDelegate: list.NewDefaultDelegate()}
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 0, 0)
	if config.Todo.Structured {
		l.Title = "Tasks (Zettelkasten)"
	} else {
		l.Title = "Tasks (All)"
	}
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false) // Disable built-in filtering
	l.KeyMap.Quit.SetKeys("ctrl+c")

	// Match vim-style keybindings
	l.KeyMap.NextPage.SetKeys("pgdown", "ctrl+f", "ctrl+d")
	l.KeyMap.PrevPage.SetKeys("pgup", "ctrl+b", "ctrl+u")

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter/tab"),
				key.WithHelp("enter/tab", "edit"),
			),
			key.NewBinding(
				key.WithKeys("t"),
				key.WithHelp("t", "change status"),
			),
			key.NewBinding(
				key.WithKeys("/"),
				key.WithHelp("/", "filter"),
			),
			key.NewBinding(
				key.WithKeys("*"),
				key.WithHelp("*", "fulltext search"),
			),
			key.NewBinding(
				key.WithKeys("s"),
				key.WithHelp("s", "structured"),
			),
			key.NewBinding(
				key.WithKeys("u"),
				key.WithHelp("u", "unstructured"),
			),
			key.NewBinding(
				key.WithKeys("v"),
				key.WithHelp("v", "detail"),
			),
			key.NewBinding(
				key.WithKeys("S"),
				key.WithHelp("S", "schedule"),
			),
			key.NewBinding(
				key.WithKeys("D"),
				key.WithHelp("D", "due date"),
			),
			key.NewBinding(
				key.WithKeys("i"),
				key.WithHelp("i", "clock in"),
			),
			key.NewBinding(
				key.WithKeys("o"),
				key.WithHelp("o", "clock out"),
			),
		}
	}

	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter", "tab"),
				key.WithHelp("enter/tab", "edit selected task"),
			),
			key.NewBinding(
				key.WithKeys("t"),
				key.WithHelp("t", "change task status"),
			),
			key.NewBinding(
				key.WithKeys("v"),
				key.WithHelp("v", "show task detail view"),
			),
			key.NewBinding(
				key.WithKeys("S"),
				key.WithHelp("S", "set scheduled date"),
			),
			key.NewBinding(
				key.WithKeys("D"),
				key.WithHelp("D", "set due date"),
			),
			key.NewBinding(
				key.WithKeys("i"),
				key.WithHelp("i", "clock in"),
			),
			key.NewBinding(
				key.WithKeys("o"),
				key.WithHelp("o", "clock out"),
			),
			key.NewBinding(
				key.WithKeys("/"),
				key.WithHelp("/", "start filtering"),
			),
			key.NewBinding(
				key.WithKeys("*"),
				key.WithHelp("*", "fulltext search"),
			),
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit filter/clear filter"),
			),
			key.NewBinding(
				key.WithKeys("s"),
				key.WithHelp("s", "switch to structured mode (zettelkasten)"),
			),
			key.NewBinding(
				key.WithKeys("u"),
				key.WithHelp("u", "switch to unstructured mode (all .md files)"),
			),
			key.NewBinding(
				key.WithKeys("g"),
				key.WithHelp("g", "jump to top"),
			),
			key.NewBinding(
				key.WithKeys("G"),
				key.WithHelp("G", "jump to bottom"),
			),
			key.NewBinding(
				key.WithKeys("ctrl+d"),
				key.WithHelp("ctrl+d", "page down (vim-style)"),
			),
			key.NewBinding(
				key.WithKeys("ctrl+u"),
				key.WithHelp("ctrl+u", "page up (vim-style)"),
			),
			key.NewBinding(
				key.WithKeys("ctrl+f"),
				key.WithHelp("ctrl+f", "page down (emacs-style)"),
			),
			key.NewBinding(
				key.WithKeys("ctrl+b"),
				key.WithHelp("ctrl+b", "page up (emacs-style)"),
			),
			key.NewBinding(
				key.WithKeys("q"),
				key.WithHelp("q", "quit"),
			),
		}
	}

	m := model{
		list:             l,
		tasks:            tasks,
		config:           config,
		project:          project,
		watcher:          watcher,
		projectColWidth:  projectColWidth,
		keywordColWidth:  keywordColWidth,
		fractionColWidth: fractionColWidth,
		allItems:         items,
		structuredMode:   config.Todo.Structured,
		searchTerm:       "",
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Print JIRA sync log from the session
	if fm, ok := finalModel.(model); ok && len(fm.jiraSyncLog) > 0 {
		fmt.Fprintf(os.Stderr, "\nJIRA sync log:\n")
		for _, e := range fm.jiraSyncLog {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
	}

	// Clean up watcher
	if watcher != nil {
		watcher.Close()
	}
}

func printTasksPlain(config *configpkg.Config, tasks []*task.Task) {
	projectColWidth := calculateProjectColWidth(tasks)
	taskColor := lipgloss.NewStyle()
	completedTaskColor := lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // Gray for completed tasks
	
	for _, t := range tasks {
		var titleStyle lipgloss.Style
		if t.IsActive(config) || t.IsInProgress(config) || t.IsSomeday(config) {
			titleStyle = taskColor
		} else {
			titleStyle = completedTaskColor
		}
		
		// Render task title with markdown formatting
		formattedTitle := task.RenderMarkdownDescription(t.Title, titleStyle)
		
		if config.GeneralConfig.Verbose {
			fmt.Printf("%-*s %-16s %-12s %-40s",
				projectColWidth, t.Project, t.Zettel, t.Keyword, formattedTitle)
		} else {
			fmt.Printf("%-*s %-12s %-40s",
				projectColWidth, t.Project, t.Keyword, formattedTitle)
		}
		for _, tag := range t.Tags {
			fmt.Printf(" #%s", tag)
		}
		if t.ScheduledAt != "" {
			fmt.Printf(" S:%s", t.ScheduledAt)
		}
		if t.DueAt != "" {
			fmt.Printf(" D:%s", t.DueAt)
		}
		if t.Assignee != "" {
			fmt.Printf(" >> %s", t.Assignee)
		}
		fmt.Println()
	}
}

func showProjectsTable(summary map[string]int) {
	var projects []string
	for p := range summary {
		projects = append(projects, p)
	}
	sort.Strings(projects)

	// Create table data
	var rows [][]string
	for _, p := range projects {
		rows = append(rows, []string{p, fmt.Sprintf("%d", summary[p])})
	}

	// Create styled table
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("212"))).
		Headers("Project", "Tasks").
		Rows(rows...)

	fmt.Println(t)
}

func printProjectsList(summary map[string]int) {
	var projects []string
	for p := range summary {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	for _, p := range projects {
		fmt.Printf("%-20s %5d\n", p, summary[p])
	}
}

