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
	tagColor           lipgloss.Style
	dateColor          lipgloss.Style
	pastDateColor      lipgloss.Style
	todayDateColor     lipgloss.Style
	assigneeColor      lipgloss.Style
}

// Global color scheme (will be initialized from config)
var colors ColorScheme

// InitializeColors initializes the color scheme from task config
func InitializeColors(cfg *task.Config) {
	colors = ColorScheme{
		prjColor:           lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		activeColor:        lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		inProgressColor:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.InProgressColor)),
		completedColor:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedColor)),
		taskColor:          lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TaskColor)),
		completedTaskColor: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedTaskColor)),
		tagColor:           lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TagColor)).Background(lipgloss.Color(cfg.Colors.TagBgColor)),
		dateColor:          lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.DateColor)).Background(lipgloss.Color(cfg.Colors.DateBgColor)),
		pastDateColor:      lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.PastDateColor)).Background(lipgloss.Color(cfg.Colors.PastDateBgColor)),
		todayDateColor:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TodayDateColor)).Background(lipgloss.Color(cfg.Colors.TodayDateBgColor)).Bold(true),
		assigneeColor:      lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.AssigneeColor)).Background(lipgloss.Color(cfg.Colors.AssigneeBgColor)).Bold(true),
	}
}

type taskItem struct {
	task            *task.Task
	projectColWidth int
}

func (i taskItem) renderWithSelection(isSelected bool) string {
	var parts []string

	parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-*s", i.projectColWidth, i.task.Project)))
	parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))

	var titleStyle lipgloss.Style
	if i.task.IsActive() {
		parts = append(parts, colors.activeColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsInProgress() {
		parts = append(parts, colors.inProgressColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = colors.taskColor
	} else {
		parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
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

	if i.task.Tag != "" {
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
	parts = append(parts, colors.prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))

	var titleStyle lipgloss.Style
	if i.task.IsActive() {
		parts = append(parts, colors.activeColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = colors.taskColor
	} else if i.task.IsInProgress() {
		parts = append(parts, colors.inProgressColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = colors.taskColor
	} else {
		parts = append(parts, colors.completedColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = colors.completedTaskColor
	}

	parts = append(parts, titleStyle.Render(fmt.Sprintf("%-40s", i.task.Title)))

	if i.task.Tag != "" {
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

func (i taskItem) Description() string { return "" }

func getDateStyle(dateStr string) lipgloss.Style {
	if dateStr == "" {
		return colors.dateColor
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
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
	config          *task.Config
	project         string
	quitting        bool
	watcher         *fsnotify.Watcher
	projectColWidth int
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
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				// Debounce: wait a bit for multiple writes to settle
				time.Sleep(100 * time.Millisecond)
				return fileChangedMsg{}
			}
		case err := <-watcher.Errors:
			log.Printf("Watcher error: %v", err)
		}
		return nil
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
					m.config.Structured = true
					m.list.Title = "Tasks (Zettelkasten)"
					return m, reloadTasksCmd()
				}
				return m, nil
			}

			// Switch to unstructured mode
			if msg.String() == "u" {
				if m.structuredMode {
					m.structuredMode = false
					m.config.Structured = false
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
		tasks, err := m.config.ListTasks(m.project, m.config.ShowCompleted)
		if err == nil {
			m.tasks = tasks
			// Sort tasks: by priority first, then by project name
			sort.Slice(m.tasks, func(i, j int) bool {
				getPriority := func(t *task.Task) int {
					if t.IsActive() {
						return 0
					} else if t.IsInProgress() {
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
			items := make([]list.Item, len(m.tasks))
			for i, t := range m.tasks {
				items[i] = taskItem{task: t, projectColWidth: m.projectColWidth}
			}
			m.allItems = items
			if m.customFilter != "" {
				m.applyCustomFilter()
			} else {
				m.list.SetItems(items)
			}
			m.list.ResetSelected()
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
					if t.IsActive() {
						return 0
					} else if t.IsInProgress() {
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
			items := make([]list.Item, len(m.tasks))
			for i, t := range m.tasks {
				items[i] = taskItem{task: t, projectColWidth: m.projectColWidth}
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
		tasks, err := m.config.ListTasks(m.project, m.config.ShowCompleted)
		if err != nil {
			return m, tea.Quit
		}
		m.tasks = tasks

		// Sort tasks: by priority first, then by project name
		sort.Slice(m.tasks, func(i, j int) bool {
			getPriority := func(t *task.Task) int {
				if t.IsActive() {
					return 0
				} else if t.IsInProgress() {
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
		items := make([]list.Item, len(m.tasks))
		for i, t := range m.tasks {
			items[i] = taskItem{task: t, projectColWidth: m.projectColWidth}
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

func loadTasksCmd(cfg *task.Config, project string) tea.Cmd {
	return func() tea.Msg {
		tasks, err := cfg.ListTasks(project, cfg.ShowCompleted)
		return loadingDoneMsg{tasks: tasks, err: err}
	}
}

func openEditorCmd(cfg *task.Config, t *task.Task) tea.Cmd {
	var filePath string
	if cfg.Structured {
		// Structured mode: construct path from project/zettel
		filePath = filepath.Join(cfg.PRJDIR, t.Project, "notes", t.Zettel, "README.md")
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

func main() {
	config := task.NewConfig()

	// Initialize colors from config
	InitializeColors(config)

	if len(os.Args) == 1 {
		// Interactive TUI mode
		showInteractiveTUI(config, "")
		return
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "-h", "--help", "help":
		printHelp()
	case "ls", "list":
		project := ""
		if len(os.Args) > 2 {
			project = os.Args[2]
		}
		tasks, err := config.ListTasks(project, config.ShowCompleted)
		if err != nil {
			log.Fatal(err)
		}
		sort.Slice(tasks, func(i, j int) bool {
			getPriority := func(t *task.Task) int {
				if t.IsActive() {
					return 0
				} else if t.IsInProgress() {
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
		printTasksPlain(tasks)
	case "projects":
		summary, err := config.SummarizeProjects()
		if err != nil {
			log.Fatal(err)
		}
		showProjectsTable(summary)
	case "pl":
		summary, err := config.SummarizeProjects()
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
    todo [COMMAND] [OPTIONS]

COMMANDS:
    (no command)        Show interactive TUI with all tasks
    ls [PROJECT]        List tasks in plain text format (for scripting)
    projects            Show project summary table with task counts
    pl                  Show project list in plain text format
    <project-name>      Show interactive TUI filtered to specific project
    -h, --help, help    Show this help message

INTERACTIVE MODE:
    Type '/' to filter  Filter tasks list (See FILTERING below)
    j/k or ↑/↓          Navigate tasks
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
    todo ls                        # List all tasks (plain text)
    todo ls myproject              # List tasks for specific project
    todo projects                  # Show project summary table
    todo pl                        # Show project list (plain text)
    todo myproject                 # Show tasks for myproject in TUI
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

CONFIGURATION:
    See config file: ~/.config/karya/config.toml.example for full configuration options.
`
	fmt.Print(help)
}

func showInteractiveTUI(config *task.Config, project string) {
	tasks, err := config.ListTasks(project, config.ShowCompleted)
	if err != nil {
		log.Fatal(err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return
	}

	// Set up file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Warning: Could not create file watcher: %v", err)
	}

	// Watch all README.md files in the project
	files, err := config.FindFiles(project)
	if err == nil && watcher != nil {
		for _, file := range files {
			watcher.Add(file)
		}
	}

	// Sort tasks: pending first, in-progress second, completed last
	// Within each group, sort by project name
	sort.Slice(tasks, func(i, j int) bool {
		// Assign priority: pending=0, in-progress=1, completed=2
		getPriority := func(t *task.Task) int {
			if t.IsActive() {
				return 0
			} else if t.IsInProgress() {
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
	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = taskItem{task: t, projectColWidth: projectColWidth}
	}

	delegate := taskDelegate{DefaultDelegate: list.NewDefaultDelegate()}
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 0, 0)
	if config.Structured {
		l.Title = "Tasks (Zettelkasten)"
	} else {
		l.Title = "Tasks (All)"
	}
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false) // Disable built-in filtering
	l.KeyMap.Quit.SetKeys("ctrl+c")
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

	m := model{
		list:            l,
		tasks:           tasks,
		config:          config,
		project:         project,
		watcher:         watcher,
		projectColWidth: projectColWidth,
		allItems:        items,
		structuredMode:  config.Structured,
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

func printTasksPlain(tasks []*task.Task) {
	projectColWidth := calculateProjectColWidth(tasks)
	for _, t := range tasks {
		fmt.Printf("%-*s %-16s %-12s %-40s",
			projectColWidth, t.Project, t.Zettel, t.Keyword, t.Title)
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
