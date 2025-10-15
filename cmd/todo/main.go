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

	"karya/internal/task"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/fsnotify/fsnotify"
)

var (
	prjColor           = lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // Cyan for project
	activeColor        = lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // Magenta for pending
	inProgressColor    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow for in-progress
	completedColor     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // Dark gray for completed keyword
	taskColor          = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // White for active task text
	completedTaskColor = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))  // Light gray for completed task text
	tagColor           = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("5"))
	dateColor          = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("15"))
	pastDateColor      = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("1")) // Inverted for past dates
	todayDateColor     = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11")).Bold(true) // Yellow background for today
	assigneeColor      = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")).Bold(true)
)

type taskItem struct {
	task           *task.Task
	projectColWidth int
}

func (i taskItem) renderWithSelection(isSelected bool) string {
	var parts []string

	parts = append(parts, prjColor.Render(fmt.Sprintf("%-*s", i.projectColWidth, i.task.Project)))
	parts = append(parts, prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))

	var titleStyle lipgloss.Style
	if i.task.IsActive() {
		parts = append(parts, activeColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = taskColor
	} else if i.task.IsInProgress() {
		parts = append(parts, inProgressColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = taskColor
	} else {
		parts = append(parts, completedColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = completedTaskColor
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
		parts = append(parts, tagColor.Render(fmt.Sprintf(" %s ", i.task.Tag)))
	}
	if i.task.Date != "" {
		dateStyle := getDateStyle(i.task.Date)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" %s ", i.task.Date)))
	}
	if i.task.Assignee != "" {
		parts = append(parts, assigneeColor.Render(fmt.Sprintf(" %s ", i.task.Assignee)))
	}

	return strings.Join(parts, " ")
}

func (i taskItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s %s %s %s %s",
		i.task.Project, i.task.Zettel, i.task.Keyword, i.task.Title,
		i.task.Tag, i.task.Date, i.task.Assignee)
}

func (i taskItem) Title() string {
	var parts []string

	parts = append(parts, prjColor.Render(fmt.Sprintf("%-*s", i.projectColWidth, i.task.Project)))
	parts = append(parts, prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))

	var titleStyle lipgloss.Style
	if i.task.IsActive() {
		parts = append(parts, activeColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = taskColor
	} else if i.task.IsInProgress() {
		parts = append(parts, inProgressColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = taskColor
	} else {
		parts = append(parts, completedColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
		titleStyle = completedTaskColor
	}

	parts = append(parts, titleStyle.Render(fmt.Sprintf("%-40s", i.task.Title)))

	if i.task.Tag != "" {
		parts = append(parts, tagColor.Render(fmt.Sprintf(" %s ", i.task.Tag)))
	}
	if i.task.Date != "" {
		dateStyle := getDateStyle(i.task.Date)
		parts = append(parts, dateStyle.Render(fmt.Sprintf(" %s ", i.task.Date)))
	}
	if i.task.Assignee != "" {
		parts = append(parts, assigneeColor.Render(fmt.Sprintf(" %s ", i.task.Assignee)))
	}

	return strings.Join(parts, " ")
}

func (i taskItem) Description() string { return "" }

func getDateStyle(dateStr string) lipgloss.Style {
	if dateStr == "" {
		return dateColor
	}

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateColor
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	taskDate := time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, time.UTC)

	if taskDate.Before(today) {
		return pastDateColor
	} else if taskDate.Equal(today) {
		return todayDateColor
	}
	return dateColor
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
}

func (m model) Init() tea.Cmd {
	return waitForFileChange(m.watcher)
}

type fileChangedMsg struct{}

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
		
		// Handle esc: cancel filter if filtering, otherwise quit
		if msg.String() == "esc" {
			if m.list.FilterState() == list.Filtering || m.list.FilterState() == list.FilterApplied {
				m.list.ResetFilter()
			} else {
				m.quitting = true
				if m.watcher != nil {
					m.watcher.Close()
				}
				return m, tea.Quit
			}
		}
		
		// Handle q: quit only when not filtering
		if msg.String() == "q" && m.list.FilterState() != list.Filtering {
			m.quitting = true
			if m.watcher != nil {
				m.watcher.Close()
			}
			return m, tea.Quit
		}

		switch msg.String() {
		case "enter":
			// Only open editor if not filtering and filter is applied
			if m.list.FilterState() != list.Filtering && m.list.FilterState() != list.FilterApplied {
				if i, ok := m.list.SelectedItem().(taskItem); ok {
					return m, openEditorCmd(m.config, i.task, m.project)
				}
			}
		}
		
		// Auto-dismiss filter when backspace/delete makes it empty
		if m.list.FilterState() == list.Filtering {
			if msg.String() == "backspace" || msg.String() == "delete" {
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(msg)
				if m.list.FilterValue() == "" {
					m.list.ResetFilter()
				}
				return m, cmd
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
			m.list.SetItems(items)
			m.list.ResetSelected()
			m.list.ResetFilter()
		}
		// Continue watching for changes
		return m, waitForFileChange(m.watcher)
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
		m.list.SetItems(items)
		m.list.ResetSelected()
		m.list.ResetFilter()
		return m, nil
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	return m.list.View()
}

type editorFinishedMsg struct{ err error }

func openEditorCmd(cfg *task.Config, t *task.Task, project string) tea.Cmd {
	filePath := filepath.Join(cfg.PRJDIR, t.Project, "notes", t.Zettel, "README.md")

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
    Type to filter      Filter tasks by typing
    j/k or ↑/↓          Navigate tasks
    Enter               Edit selected task at specific line
    q                   Quit (when not filtering)
    Esc                 Exit filter mode or quit
    Ctrl+C              Quit

EXAMPLES:
    todo                        # Show all tasks in interactive TUI
    todo ls                     # List all tasks (plain text)
    todo ls myproject           # List tasks for specific project
    todo projects               # Show project summary table
    todo pl                     # Show project list (plain text)
    todo myproject              # Show tasks for myproject in TUI
    SHOW_COMPLETED=true todo   # Show completed tasks in TUI
    SHOW_COMPLETED=false todo  # Hide completed tasks (default)

ENVIRONMENT VARIABLES:
    EDITOR              Editor to use (supports vim, nvim, emacs, nano, code)
                        Can include arguments, e.g., EDITOR="emacs -nw"
    
    SHOW_COMPLETED      Show completed tasks (true/false, default: false)
                        Can also be set in ~/.config/karya/config.toml

CONFIGURATION:
    Config file: ~/.config/karya/config.toml
    
    Options:
        show_completed = true/false     # Show completed tasks
        active_keywords = [...]          # Customize active task keywords
        inprogress_keywords = [...]      # Customize in-progress keywords
        completed_keywords = [...]       # Customize completed keywords
    
    See config.toml.example for full configuration options.
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
	l.Title = "Tasks"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.KeyMap.Quit.SetKeys("esc", "ctrl+c")
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "edit"),
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
		if t.Date != "" {
			fmt.Printf(" @%s", t.Date)
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

func printProjectsTable(summary map[string]int) {
	var projects []string
	maxLen := 0
	for p := range summary {
		projects = append(projects, p)
		if len(p) > maxLen {
			maxLen = len(p)
		}
	}
	sort.Strings(projects)

	// Print header
	fmt.Printf("\n%-*s Tasks\n", maxLen, "Project")
	fmt.Printf("%s %s\n", strings.Repeat("-", maxLen), "-----")

	// Print projects
	for _, p := range projects {
		fmt.Printf("%-*s %5d\n", maxLen, p, summary[p])
	}
	fmt.Println()
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
