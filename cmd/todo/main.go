package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vinayprograms/karya/internal/config"
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
	prjColor           lipgloss.Style
	activeColor        lipgloss.Style
	inProgressColor    lipgloss.Style
	completedColor     lipgloss.Style
	taskColor          lipgloss.Style
	completedTaskColor lipgloss.Style
	specialTagColor    lipgloss.Style
	tagColor           lipgloss.Style
	dateColor          lipgloss.Style
	pastDateColor      lipgloss.Style
	todayDateColor     lipgloss.Style
	assigneeColor      lipgloss.Style
}

// Global color scheme (will be initialized from config)
var colors ColorScheme

// InitializeColors initializes the color scheme from task config
func InitializeColors(cfg *config.Config) {
	colors = ColorScheme{
		prjColor:           lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		activeColor:        lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		inProgressColor:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.InProgressColor)),
		completedColor:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedColor)),
		taskColor:          lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TaskColor)),
		completedTaskColor: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedTaskColor)),
		tagColor:           lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TagColor)).Background(lipgloss.Color(cfg.Colors.TagBgColor)),
		specialTagColor:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.SpecialTagColor)).Background(lipgloss.Color(cfg.Colors.SpecialTagBgColor)).Bold(true),
		dateColor:          lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.DateColor)).Background(lipgloss.Color(cfg.Colors.DateBgColor)),
		pastDateColor:      lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.PastDateColor)).Background(lipgloss.Color(cfg.Colors.PastDateBgColor)),
		todayDateColor:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TodayDateColor)).Background(lipgloss.Color(cfg.Colors.TodayDateBgColor)).Bold(true),
		assigneeColor:      lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.AssigneeColor)).Background(lipgloss.Color(cfg.Colors.AssigneeBgColor)).Bold(true),
	}
}

type taskItem struct {
	config          *config.Config
	task            *task.Task
	projectColWidth int
	keywordColWidth int
	verbose         bool
}

func NewTaskItem(c *config.Config, t *task.Task, projectColWidth, keywordColWidth int, verbose bool) taskItem {
	return taskItem{
		config:          c,
		task:            t,
		projectColWidth: projectColWidth,
		keywordColWidth: keywordColWidth,
		verbose:         verbose,
	}
}

func (i taskItem) renderWithSelection(isSelected bool) string {
	var parts []string

	parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-*s", i.projectColWidth, i.task.Project)))

	// Only show Zettel column in verbose mode
	if i.verbose {
		parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))
	}

	var titleStyle lipgloss.Style
	if i.task.IsActive(i.config) {
		parts = append(parts, colors.activeColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsInProgress(i.config) {
		parts = append(parts, colors.inProgressColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else {
		parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.completedTaskColor
	}

	if isSelected {
		indicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")).
			Bold(true).
			Render("█ ")
		parts = append(parts, indicator+titleStyle.Render(fmt.Sprintf("%-40s", i.task.Title)))
	} else {
		parts = append(parts, "  "+titleStyle.Render(fmt.Sprintf("%-40s", i.task.Title)))
	}

	// Render tag with special color if it's in special tags. Special tags either contain
	// that exact text or start with that text followed by a colon.
	isSpecialTag := false
	for _, specialTag := range i.config.Todo.SpecialTags {
		if i.task.Tag == specialTag {
			isSpecialTag = true
			break
		} else if strings.HasPrefix(i.task.Tag, specialTag+":") {
			isSpecialTag = true
			break
		}
	}
	if isSpecialTag {
		parts = append(parts, colors.specialTagColor.Render(fmt.Sprintf(" %s ", i.task.Tag)))
	} else if i.task.Tag != "" {
		parts = append(parts, colors.tagColor.Render(fmt.Sprintf(" %s ", i.task.Tag)))
	}
	// Display date types with prefixes
	if i.task.ScheduledAt != "" {
		dateStyle := getDateStyle(i.task.ScheduledAt)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" S:%s ", i.task.ScheduledAt)))
	}
	if i.task.DueAt != "" {
		dateStyle := getDateStyle(i.task.DueAt)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" D:%s ", i.task.DueAt)))
	}
	if i.task.Assignee != "" {
		parts = append(parts, colors.assigneeColor.Render(fmt.Sprintf(" %s ", i.task.Assignee)))
	}

	return strings.Join(parts, " ")
}

func (i taskItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s %s %s %s %s %s",
		i.task.Project, i.task.Zettel, i.task.Keyword, i.task.Title,
		i.task.Tag, i.task.ScheduledAt, i.task.DueAt, i.task.Assignee)
}

func (i taskItem) Title() string {
	var parts []string

	parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-*s", i.projectColWidth, i.task.Project)))

	// Only show Zettel column in verbose mode
	if i.verbose {
		parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))
	}

	var titleStyle lipgloss.Style
	if i.task.IsActive(i.config) {
		parts = append(parts, colors.activeColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsInProgress(i.config) {
		parts = append(parts, colors.inProgressColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.taskColor
	} else {
		parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-*s", i.keywordColWidth, i.task.Keyword)))
		titleStyle = colors.completedTaskColor
	}

	parts = append(parts, titleStyle.Render(fmt.Sprintf("%-40s", i.task.Title)))

	// Render tag with special color if it's in special tags. Special tags either contain
	// that exact text or start with that text followed by a colon.
	isSpecialTag := false
	for _, specialTag := range i.config.Todo.SpecialTags {
		if i.task.Tag == specialTag {
			isSpecialTag = true
			break
		} else if strings.HasPrefix(i.task.Tag, specialTag+":") {
			isSpecialTag = true
			break
		}
	}
	if i.task.Tag != "" {
		if isSpecialTag {
			parts = append(parts, colors.specialTagColor.Render(fmt.Sprintf(" %s ", i.task.Tag)))
		} else {
			parts = append(parts, colors.tagColor.Render(fmt.Sprintf(" %s ", i.task.Tag)))
		}
	}
	// Display date types with prefixes
	if i.task.ScheduledAt != "" {
		dateStyle := getDateStyle(i.task.ScheduledAt)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" S:%s ", i.task.ScheduledAt)))
	}
	if i.task.DueAt != "" {
		dateStyle := getDateStyle(i.task.DueAt)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" D:%s ", i.task.DueAt)))
	}
	if i.task.Assignee != "" {
		parts = append(parts, colors.assigneeColor.Render(fmt.Sprintf(" %s ", i.task.Assignee)))
	}

	return strings.Join(parts, " ")
}

func (i taskItem) Description() string { return "" }

func getDateStyle(dateStr string) lipgloss.Style {
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

type model struct {
	list            list.Model
	tasks           []*task.Task
	config          *config.Config
	project         string
	quitting        bool
	watcher         *fsnotify.Watcher
	projectColWidth int
	keywordColWidth int
	savedFilter     string
	customFilter    string
	filtering       bool
	allItems        []list.Item
	structuredMode  bool
	loading         bool
}

func (m model) Init() tea.Cmd {
	return waitForFileChange(m.watcher)
}

type fileChangedMsg struct{}

type loadingStartMsg struct{}
type loadingDoneMsg struct {
	tasks []*task.Task
	err   error
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
					m.applyCustomFilter()
				} else {
					// Exit filtering if filter is empty
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
						return m, openEditorCmd(m.config, i.task)
					}
				}
			}
		}
	case fileChangedMsg:
		// Reload tasks when files change
		tasks, err := task.ListTasks(m.config, m.project, m.config.Todo.ShowCompleted)
		if err == nil {
			m.tasks = tasks
			// Sort tasks: by priority first, then by project name
			sort.Slice(m.tasks, func(i, j int) bool {
				getPriority := func(t *task.Task) int {
					if t.IsActive(m.config) {
						return 0
					} else if t.IsInProgress(m.config) {
						return 1
					}
					return 2
				}

				priorityI := getPriority(m.tasks[i])
				priorityJ := getPriority(m.tasks[j])

				// First sort by priority
				if priorityI != priorityJ {
					return priorityI < priorityJ
				}

				// Within same priority, sort by project name
				return m.tasks[i].Project < m.tasks[j].Project
			})
			m.projectColWidth = calculateProjectColWidth(m.tasks)
			m.keywordColWidth = calculateKeywordColWidth(m.tasks)
			items := make([]list.Item, len(m.tasks))
			for i, t := range m.tasks {
				items[i] = NewTaskItem(m.config, t, m.projectColWidth, m.keywordColWidth, m.config.GeneralConfig.Verbose)
			}
			m.allItems = items
			if m.customFilter != "" {
				m.applyCustomFilter()
			} else {
				m.list.SetItems(items)
			}
			m.list.ResetSelected()

			// Update watcher to monitor new files/directories
			updateWatcher(m.watcher, m.config, m.project)
		}
		// Continue watching for changes
		return m, waitForFileChange(m.watcher)
	case loadingStartMsg:
		// Start loading
		m.loading = true
		return m, loadTasksCmd(m.config, m.project)
	case loadingDoneMsg:
		// Finish loading
		m.loading = false
		if msg.err == nil {
			m.tasks = msg.tasks
			// Sort tasks: by priority first, then by project name
			sort.Slice(m.tasks, func(i, j int) bool {
				getPriority := func(t *task.Task) int {
					if t.IsActive(m.config) {
						return 0
					} else if t.IsInProgress(m.config) {
						return 1
					}
					return 2
				}

				priorityI := getPriority(m.tasks[i])
				priorityJ := getPriority(m.tasks[j])

				// First sort by priority
				if priorityI != priorityJ {
					return priorityI < priorityJ
				}

				// Within same priority, sort by project name
				return m.tasks[i].Project < m.tasks[j].Project
			})

			m.projectColWidth = calculateProjectColWidth(m.tasks)
			m.keywordColWidth = calculateKeywordColWidth(m.tasks)
			items := make([]list.Item, len(m.tasks))
			for i, t := range m.tasks {
				items[i] = taskItem{config: m.config, task: t, projectColWidth: m.projectColWidth, keywordColWidth: m.keywordColWidth, verbose: m.config.GeneralConfig.Verbose}
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
		// Reload tasks after editing
		tasks, err := task.ListTasks(m.config, m.project, m.config.Todo.ShowCompleted)
		if err != nil {
			return m, tea.Quit
		}
		m.tasks = tasks

		// Sort tasks: by priority first, then by project name
		sort.Slice(m.tasks, func(i, j int) bool {
			getPriority := func(t *task.Task) int {
				if t.IsActive(m.config) {
					return 0
				} else if t.IsInProgress(m.config) {
					return 1
				}
				return 2
			}

			priorityI := getPriority(m.tasks[i])
			priorityJ := getPriority(m.tasks[j])

			// First sort by priority
			if priorityI != priorityJ {
				return priorityI < priorityJ
			}

			// Within same priority, sort by project name
			return m.tasks[i].Project < m.tasks[j].Project
		})

		m.projectColWidth = calculateProjectColWidth(m.tasks)
		m.keywordColWidth = calculateKeywordColWidth(m.tasks)
		items := make([]list.Item, len(m.tasks))
		for i, t := range m.tasks {
			items[i] = taskItem{config: m.config, task: t, projectColWidth: m.projectColWidth, keywordColWidth: m.keywordColWidth, verbose: m.config.GeneralConfig.Verbose}
		}
		m.allItems = items

		// Restore previous filter if there was one
		if m.savedFilter != "" {
			m.customFilter = m.savedFilter
			m.applyCustomFilter()
		} else {
			m.list.SetItems(items)
		}
		m.list.ResetSelected()

		return m, nil
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) applyCustomFilter() {
	if m.customFilter == "" {
		m.list.SetItems(m.allItems)
		return
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

	m.list.SetItems(filteredItems)
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	view := m.list.View()

	// Add custom pagination info at the top, right after the title (only if multiple pages)
	totalItems := len(m.list.Items())
	if totalItems > 0 {
		// Use paginator information for accurate page display
		p := m.list.Paginator
		totalPages := p.TotalPages

		// Only show pagination info if there's more than one page
		if totalPages > 1 {
			currentPage := p.Page
			itemsPerPage := p.PerPage

			// Calculate the range of items on current page
			startIdx := currentPage * itemsPerPage
			endIdx := startIdx + itemsPerPage
			if endIdx > totalItems {
				endIdx = totalItems
			}

			paginationInfo := lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Render(fmt.Sprintf("Showing %d-%d of %d • Page %d/%d",
					startIdx+1, endIdx, totalItems, currentPage+1, totalPages))

			// Split the view and insert pagination info after the title line (first line)
			lines := strings.Split(view, "\n")
			if len(lines) >= 1 {
				// Insert pagination info after the title (first line)
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
			filterText = fmt.Sprintf("Filter: %s▓", m.customFilter) // Show cursor when actively typing
		} else if m.customFilter != "" {
			filterText = fmt.Sprintf("Filter: %s", m.customFilter)
		}

		filterInfo := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Background(lipgloss.Color("0")).
			Padding(0, 1).
			Render(filterText)
		view = filterInfo + "\n" + view
	}

	return view
}

type editorFinishedMsg struct{ err error }

func reloadTasksCmd() tea.Cmd {
	return func() tea.Msg {
		return loadingStartMsg{}
	}
}

func loadTasksCmd(cfg *config.Config, project string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := task.ListTasks(cfg, project, cfg.Todo.ShowCompleted)
		return loadingDoneMsg{tasks: tasks, err: err}
	}
}

func openEditorCmd(cfg *config.Config, t *task.Task) tea.Cmd {
	var filePath string
	if cfg.Todo.Structured {
		// Structured mode: construct path from project/zettel
		filePath = filepath.Join(cfg.Directories.Projects, t.Project, "notes", t.Zettel, "README.md")
	} else {
		// Unstructured mode: use the original file path where task was found
		filePath = t.FilePath
	}

	// Find line number by matching keyword and title
	lineNum, err := findTaskLine(filePath, t.Keyword, t.Title)
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
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

	// Get the base name of the editor to determine line number syntax
	editorBase := filepath.Base(editorCmd)

	var args []string
	args = append(args, editorArgs...) // Add any existing args from EDITOR

	if strings.Contains(editorBase, "vim") || strings.Contains(editorBase, "nvim") {
		args = append(args, fmt.Sprintf("+%d", lineNum), filePath)
	} else if strings.Contains(editorBase, "emacs") {
		args = append(args, fmt.Sprintf("+%d", lineNum), filePath)
	} else if strings.Contains(editorBase, "nano") {
		args = append(args, fmt.Sprintf("+%d", lineNum), filePath)
	} else if strings.Contains(editorBase, "code") {
		args = append(args, "-g", fmt.Sprintf("%s:%d", filePath, lineNum))
	} else {
		// Unknown editor, just pass the file
		args = append(args, filePath)
	}

	c := exec.Command(editorCmd, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func findTaskLine(filePath, keyword, title string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	searchStr := fmt.Sprintf("%s: %s", keyword, title)

	for scanner.Scan() {
		lineNum++
		if strings.HasPrefix(scanner.Text(), searchStr) {
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

// setupWatcher creates a new watcher and watches all relevant directories
func setupWatcher(config *config.Config, project string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	updateWatcher(watcher, config, project)
	return watcher, nil
}

// updateWatcher updates the watcher to monitor all relevant directories and files
func updateWatcher(watcher *fsnotify.Watcher, config *config.Config, project string) {
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
}

// getWatchDirectories returns a list of directories that should be watched
func getWatchDirectories(config *config.Config, project string) []string {
	var dirs []string

	// Determine the root directory to watch
	var rootDir string
	if project == "" || project == "*" {
		// Watch everything under PRJDIR
		rootDir = config.Directories.Projects
	} else {
		// Watch specific project directory tree
		rootDir = filepath.Join(config.Directories.Projects, project)
	}

	// Recursively walk and watch all directories under the root
	// This handles any directory structure: flat, nested, or hierarchical groupings
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})

	return dirs
}

func main() {
	config, err := config.Load()
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

	if len(args) == 0 {
		// Interactive TUI mode
		showInteractiveTUI(config, "")
		return
	}

	subcommand := args[0]
	switch subcommand {
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
		sort.Slice(tasks, func(i, j int) bool {
			getPriority := func(t *task.Task) int {
				if t.IsActive(config) {
					return 0
				} else if t.IsInProgress(config) {
					return 1
				}
				return 2
			}

			priorityI := getPriority(tasks[i])
			priorityJ := getPriority(tasks[j])

			if priorityI != priorityJ {
				return priorityI < priorityJ
			}

			return tasks[i].Project < tasks[j].Project
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

func showInteractiveTUI(config *config.Config, project string) {
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

	// Sort tasks: pending first, in-progress second, completed last
	// Within each group, sort by project name
	sort.Slice(tasks, func(i, j int) bool {
		// Assign priority: pending=0, in-progress=1, completed=2
		getPriority := func(t *task.Task) int {
			if t.IsActive(config) {
				return 0
			} else if t.IsInProgress(config) {
				return 1
			}
			return 2
		}

		priorityI := getPriority(tasks[i])
		priorityJ := getPriority(tasks[j])

		// First sort by priority
		if priorityI != priorityJ {
			return priorityI < priorityJ
		}

		// Within same priority, sort by project name
		return tasks[i].Project < tasks[j].Project
	})

	projectColWidth := calculateProjectColWidth(tasks)
	keywordColWidth := calculateKeywordColWidth(tasks)
	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = taskItem{config: config, task: t, projectColWidth: projectColWidth, keywordColWidth: keywordColWidth, verbose: config.GeneralConfig.Verbose}
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
				key.WithKeys("/"),
				key.WithHelp("/", "filter"),
			),
			key.NewBinding(
				key.WithKeys("s"),
				key.WithHelp("s", "structured"),
			),
			key.NewBinding(
				key.WithKeys("u"),
				key.WithHelp("u", "unstructured"),
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
				key.WithKeys("/"),
				key.WithHelp("/", "start filtering"),
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
		list:            l,
		tasks:           tasks,
		config:          config,
		project:         project,
		watcher:         watcher,
		projectColWidth: projectColWidth,
		keywordColWidth: keywordColWidth,
		allItems:        items,
		structuredMode:  config.Todo.Structured,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}

	// Clean up watcher
	if watcher != nil {
		watcher.Close()
	}
}

func printTasksPlain(config *config.Config, tasks []*task.Task) {
	projectColWidth := calculateProjectColWidth(tasks)
	for _, t := range tasks {
		if config.GeneralConfig.Verbose {
			fmt.Printf("%-*s %-16s %-12s %-40s",
				projectColWidth, t.Project, t.Zettel, t.Keyword, t.Title)
		} else {
			fmt.Printf("%-*s %-12s %-40s",
				projectColWidth, t.Project, t.Keyword, t.Title)
		}
		if t.Tag != "" {
			fmt.Printf(" #%s", t.Tag)
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
