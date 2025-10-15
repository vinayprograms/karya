package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"karya/internal/task"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	prjColor      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	activeColor   = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	completedColor = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	taskColor     = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	tagColor      = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("5"))
	dateColor     = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Background(lipgloss.Color("15"))
	assigneeColor = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")).Bold(true)
)

type taskItem struct {
	task *task.Task
}

func (i taskItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s %s %s %s %s",
		i.task.Project, i.task.Zettel, i.task.Keyword, i.task.Title,
		i.task.Tag, i.task.Date, i.task.Assignee)
}

func (i taskItem) Title() string {
	var parts []string
	
	parts = append(parts, prjColor.Render(fmt.Sprintf("%-15s", i.task.Project)))
	parts = append(parts, prjColor.Render(fmt.Sprintf("%-16s", i.task.Zettel)))
	
	if i.task.IsActive() {
		parts = append(parts, activeColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
	} else {
		parts = append(parts, completedColor.Render(fmt.Sprintf("%-12s", i.task.Keyword)))
	}
	
	parts = append(parts, taskColor.Render(fmt.Sprintf("%-40s", i.task.Title)))
	
	if i.task.Tag != "" {
		parts = append(parts, tagColor.Render(fmt.Sprintf(" %s ", i.task.Tag)))
	}
	if i.task.Date != "" {
		parts = append(parts, dateColor.Render(fmt.Sprintf(" %s ", i.task.Date)))
	}
	if i.task.Assignee != "" {
		parts = append(parts, assigneeColor.Render(fmt.Sprintf(" %s ", i.task.Assignee)))
	}
	
	return strings.Join(parts, " ")
}

func (i taskItem) Description() string { return "" }

type model struct {
	list     list.Model
	tasks    []*task.Task
	config   *task.Config
	project  string
	quitting bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit keys before list processes them
		if msg.String() == "ctrl+c" || msg.String() == "esc" || (msg.String() == "q" && m.list.FilterState() != list.Filtering) {
			m.quitting = true
			return m, tea.Quit
		}
		
		switch msg.String() {
		case "enter":
			if i, ok := m.list.SelectedItem().(taskItem); ok {
				return m, openEditorCmd(m.config, i.task, m.project)
			}
		}
	case editorFinishedMsg:
		// Log any errors from the editor
		if msg.err != nil {
			log.Printf("Editor error: %v", msg.err)
		}
		// Reload tasks after editing
		tasks, err := m.config.ListTasks(m.project, true)
		if err != nil {
			return m, tea.Quit
		}
		m.tasks = tasks
		items := make([]list.Item, len(tasks))
		for i, t := range tasks {
			items[i] = taskItem{task: t}
		}
		m.list.SetItems(items)
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
		tasks, err := config.ListTasks(project, true)
		if err != nil {
			log.Fatal(err)
		}
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
	help := `todo - Interactive task manager

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
    todo                # Show all tasks in interactive TUI
    todo ls             # List all tasks (plain text)
    todo ls myproject   # List tasks for specific project
    todo projects       # Show project summary table
    todo pl             # Show project list (plain text)
    todo myproject      # Show tasks for myproject in TUI

ENVIRONMENT:
    EDITOR              Editor to use (supports vim, nvim, emacs, nano, code)
                        Can include arguments, e.g., EDITOR="emacs -nw"
`
	fmt.Print(help)
}

func showInteractiveTUI(config *task.Config, project string) {
	tasks, err := config.ListTasks(project, true)
	if err != nil {
		log.Fatal(err)
	}
	
	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return
	}
	
	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = taskItem{task: t}
	}
	
	delegate := list.NewDefaultDelegate()
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
		list:    l,
		tasks:   tasks,
		config:  config,
		project: project,
	}
	
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func printTasksPlain(tasks []*task.Task) {
	for _, t := range tasks {
		fmt.Printf("%-15s %-16s %-12s %-40s",
			t.Project, t.Zettel, t.Keyword, t.Title)
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
