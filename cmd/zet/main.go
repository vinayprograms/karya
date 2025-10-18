package main

import (
	"bufio"
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

	"github.com/vinayprograms/karya/internal/config"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
)

type Zettel struct {
	ID    string
	Title string
	Path  string
}

type SearchResult struct {
	ZettelID string
	Title    string
	LineNum  int
	Line     string
	Path     string
}

var (
	magentaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	yellowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	normalStyle  = lipgloss.NewStyle()
	boldOrange   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
)

type zettelItem struct {
	zettel  Zettel
	verbose bool
}

func (i zettelItem) FilterValue() string {
	return i.zettel.ID + " " + i.zettel.Title
}

func (i zettelItem) renderWithSelection(isSelected bool) string {
	var parts []string

	if i.verbose {
		parts = append(parts, magentaStyle.Render(fmt.Sprintf("%-14s", i.zettel.ID)))
	}

	if isSelected {
		indicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")).
			Bold(true).
			Render("█ ")
		parts = append(parts, indicator+i.zettel.Title)
	} else {
		parts = append(parts, "  "+i.zettel.Title)
	}

	return strings.Join(parts, " ")
}

func (i zettelItem) Title() string {
	var parts []string

	if i.verbose {
		parts = append(parts, magentaStyle.Render(fmt.Sprintf("%-14s", i.zettel.ID)))
	}

	parts = append(parts, i.zettel.Title)
	return strings.Join(parts, " ")
}

func (i zettelItem) Description() string { return "" }

type zettelDelegate struct {
	list.DefaultDelegate
}

func (d zettelDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	zettelItem, ok := item.(zettelItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	content := zettelItem.renderWithSelection(isSelected)
	fmt.Fprint(w, content)
}

type model struct {
	list                   list.Model
	zettels                []Zettel
	zetDir                 string
	quitting               bool
	editor                 string
	customFilter           string
	filtering              bool
	allItems               []list.Item
	verbose                bool
	watcher                *fsnotify.Watcher
	savedFilter            string
	sortNewestFirst        bool
	showDeleteConfirm      bool
	deleteZettel           *Zettel
	deleteConfirmSelection int // 0 = Cancel, 1 = Delete
}

func (m model) Init() tea.Cmd {
	return waitForFileChange(m.watcher)
}

type fileChangedMsg struct{}

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
			case err, ok := <-watcher.Errors:
				if !ok {
					return nil
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// Handle delete confirmation dialog
		if m.showDeleteConfirm {
			switch msg.String() {
			case "esc":
				m.showDeleteConfirm = false
				m.deleteZettel = nil
				m.deleteConfirmSelection = 0
				return m, nil
			case "left", "h":
				m.deleteConfirmSelection = 0
				return m, nil
			case "right", "l":
				m.deleteConfirmSelection = 1
				return m, nil
			case "enter":
				if m.deleteConfirmSelection == 1 && m.deleteZettel != nil {
					// Delete confirmed
					err := deleteZettel(m.zetDir, m.deleteZettel.ID)
					if err != nil {
						log.Printf("Error deleting zettel: %v", err)
					}
					m.showDeleteConfirm = false
					m.deleteZettel = nil
					m.deleteConfirmSelection = 0

					// Reload zettels
					zettels, err := listZettels(m.zetDir)
					if err == nil {
						m.zettels = zettels
						items := make([]list.Item, len(m.zettels))
						for i, z := range m.zettels {
							items[i] = zettelItem{zettel: z, verbose: m.verbose}
						}
						m.allItems = items
						if m.customFilter != "" {
							m.applyCustomFilter()
						} else {
							m.list.SetItems(items)
						}
						m.list.ResetSelected()
					}
					return m, nil
				} else {
					// Cancel
					m.showDeleteConfirm = false
					m.deleteZettel = nil
					m.deleteConfirmSelection = 0
					return m, nil
				}
			}
			return m, nil
		}

		// Handle custom filtering
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.customFilter = ""
				m.list.SetItems(m.allItems)
				return m, nil
			case "enter":
				m.filtering = false
				return m, nil
			case "backspace":
				if len(m.customFilter) > 0 {
					m.customFilter = m.customFilter[:len(m.customFilter)-1]
					m.applyCustomFilter()
				} else {
					m.filtering = false
					m.list.SetItems(m.allItems)
				}
				return m, nil
			default:
				if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 {
					m.customFilter += string(msg.Runes[0])
					m.applyCustomFilter()
					return m, nil
				}
			}
		} else {
			if msg.String() == "esc" {
				if m.customFilter != "" {
					m.customFilter = ""
					m.list.SetItems(m.allItems)
					return m, nil
				}
			}

			if msg.String() == "q" {
				m.quitting = true
				return m, tea.Quit
			}

			if msg.String() == "/" {
				m.filtering = true
				return m, nil
			}

			if msg.String() == "s" {
				m.sortNewestFirst = !m.sortNewestFirst
				m.sortZettels()
				return m, nil
			}

			// Subcommand keybindings
			if msg.String() == "n" {
				// New zettel
				return m, newZettelCmd(m.zetDir, m.editor)
			}

			if msg.String() == "l" {
				// Edit last zettel
				return m, editLastZettelCmd(m.zetDir, m.editor)
			}

			if msg.String() == "c" {
				// Show count (just refresh, count is visible in pagination)
				return m, nil
			}

			if msg.String() == "shift+t" || msg.String() == "T" {
				// Edit TOC
				tocPath := filepath.Join(m.zetDir, "README.md")
				return m, openEditorCmd(m.editor, tocPath)
			}

			if msg.String() == "d" {
				// Delete zettel - show confirmation
				if i, ok := m.list.SelectedItem().(zettelItem); ok {
					m.showDeleteConfirm = true
					m.deleteZettel = &i.zettel
					m.deleteConfirmSelection = 0 // Default to Cancel
					return m, nil
				}
			}

			if msg.String() == "enter" {
				if !m.filtering {
					if i, ok := m.list.SelectedItem().(zettelItem); ok {
						m.savedFilter = m.customFilter
						return m, openEditorCmd(m.editor, i.zettel.Path)
					}
				}
			}
		}
	case fileChangedMsg:
		zettels, err := listZettels(m.zetDir)
		if err == nil {
			m.zettels = zettels
			items := make([]list.Item, len(m.zettels))
			for i, z := range m.zettels {
				items[i] = zettelItem{zettel: z, verbose: m.verbose}
			}
			m.allItems = items
			if m.customFilter != "" {
				m.applyCustomFilter()
			} else {
				m.list.SetItems(items)
			}
			m.list.ResetSelected()
			updateWatcher(m.watcher, m.zetDir)
		}
		return m, waitForFileChange(m.watcher)
	case editorFinishedMsg:
		if msg.err != nil {
			log.Printf("Editor error: %v", msg.err)
		}
		zettels, err := listZettels(m.zetDir)
		if err == nil {
			m.zettels = zettels
			items := make([]list.Item, len(m.zettels))
			for i, z := range m.zettels {
				items[i] = zettelItem{zettel: z, verbose: m.verbose}
			}
			m.allItems = items
			if m.savedFilter != "" {
				m.customFilter = m.savedFilter
				m.applyCustomFilter()
			} else {
				m.list.SetItems(items)
			}
			m.list.ResetSelected()
		}
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

	if m.filtering || m.customFilter != "" {
		var filterText string
		if m.filtering {
			filterText = fmt.Sprintf("Filter: %s▓", m.customFilter)
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

	// Replace the default help with a custom compact status bar
	lines := strings.Split(view, "\n")
	if len(lines) > 0 {
		// Remove the last line (default help)
		lines = lines[:len(lines)-1]

		// Create custom compact help (2 lines)
		commandStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")) //.
			//Background(lipgloss.Color("236"))
		navStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

		line1 := commandStyle.Render(" Commands: n:new • l:last • d:delete • T:toc • c:count ")
		line2 := navStyle.Render(" ↑↓/jk • g/G:top/bottom • Ctrl+d/u:page • /:filter • s:sort • Enter:edit • q:quit")

		lines = append(lines, line1, line2, "") // Add empty line for spacing
		view = strings.Join(lines, "\n")
	}

	// Show delete confirmation dialog
	if m.showDeleteConfirm && m.deleteZettel != nil {
		dialogBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("1")).
			Padding(1, 2).
			Width(60)

		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("1")).
			Render("Delete Zettel?")

		zettelInfo := fmt.Sprintf(" %s\n",
			m.deleteZettel.Title)

		cancelStyle := lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("0"))

		deleteStyle := lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("0"))

		if m.deleteConfirmSelection == 0 {
			cancelStyle = cancelStyle.
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("2")).
				Bold(true)
		} else {
			deleteStyle = deleteStyle.
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("1")).
				Bold(true)
		}

		buttons := lipgloss.JoinHorizontal(lipgloss.Top,
			cancelStyle.Render("Cancel"),
			"  ",
			deleteStyle.Render("Delete"),
		)

		dialog := dialogBox.Render(
			title + zettelInfo + "\n" + buttons + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("← → or h l to select • Enter to confirm • Esc to cancel"),
		)

		// Center the dialog
		view = lipgloss.Place(
			lipgloss.Width(view),
			lipgloss.Height(view),
			lipgloss.Center,
			lipgloss.Center,
			dialog,
		)
	}

	return view
}

func (m *model) applyCustomFilter() {
	if m.customFilter == "" {
		m.list.SetItems(m.allItems)
		return
	}

	filterLower := strings.ToLower(m.customFilter)
	var filteredItems []list.Item

	for _, item := range m.allItems {
		if zetItem, ok := item.(zettelItem); ok {
			// Search in ID and title
			if strings.Contains(strings.ToLower(zetItem.zettel.ID), filterLower) ||
				strings.Contains(strings.ToLower(zetItem.zettel.Title), filterLower) {
				filteredItems = append(filteredItems, item)
			}
		}
	}

	m.list.SetItems(filteredItems)
}

func (m *model) sortZettels() {
	// Sort the zettels slice
	sort.Slice(m.zettels, func(i, j int) bool {
		if m.sortNewestFirst {
			return m.zettels[i].ID > m.zettels[j].ID
		}
		return m.zettels[i].ID < m.zettels[j].ID
	})

	// Update title to reflect sort order
	if m.sortNewestFirst {
		m.list.Title = "Zettels (Newest First)"
	} else {
		m.list.Title = "Zettels (Oldest First)"
	}

	// Recreate items with sorted zettels
	items := make([]list.Item, len(m.zettels))
	for i, z := range m.zettels {
		items[i] = zettelItem{zettel: z, verbose: m.verbose}
	}
	m.allItems = items

	// Reapply filter if active
	if m.customFilter != "" {
		m.applyCustomFilter()
	} else {
		m.list.SetItems(items)
	}
	m.list.ResetSelected()
}

type editorFinishedMsg struct{ err error }

func newZettelCmd(zetDir, editor string) tea.Cmd {
	zetID := time.Now().UTC().Format("20060102150405")

	// Create empty zettel
	if err := createZettel(zetDir, zetID, ""); err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	// Open in editor using tea.ExecProcess
	zetPath := filepath.Join(zetDir, zetID, "README.md")

	if strings.HasPrefix(editor, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			editor = filepath.Join(home, editor[2:])
		}
	}

	editorParts := strings.Fields(editor)
	editorCmd := editorParts[0]
	editorArgs := editorParts[1:]
	editorArgs = append(editorArgs, zetPath)

	c := exec.Command(editorCmd, editorArgs...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		// Update README after editing
		updateReadme(zetDir)

		// Get actual title and commit
		actualTitle, _ := getZettelTitle(zetDir, zetID)
		if actualTitle != "" {
			gitCommit(zetDir, zetID, actualTitle)
		}

		return editorFinishedMsg{err: err}
	})
}

func editLastZettelCmd(zetDir, editor string) tea.Cmd {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return func() tea.Msg {
			return editorFinishedMsg{err: fmt.Errorf("not a git repository")}
		}
	}

	cmd := exec.Command("git", "-C", zetDir, "log", "--pretty=format:%h", "-n", "1")
	output, err := cmd.Output()
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}
	commit := strings.TrimSpace(string(output))

	cmd = exec.Command("git", "-C", zetDir, "show", "--name-only", "--pretty=", commit)
	output, err = cmd.Output()
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
		}
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 0 {
		return func() tea.Msg {
			return editorFinishedMsg{err: fmt.Errorf("no files in last commit")}
		}
	}

	zetID := ""
	for _, file := range files {
		if strings.Contains(file, "/") {
			parts := strings.Split(file, "/")
			if len(parts) > 0 && isValidZettelID(parts[0]) {
				zetID = parts[0]
				break
			}
		}
	}

	if zetID == "" {
		return func() tea.Msg {
			return editorFinishedMsg{err: fmt.Errorf("could not determine zettel ID from last commit")}
		}
	}

	zetPath := filepath.Join(zetDir, zetID, "README.md")

	if strings.HasPrefix(editor, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			editor = filepath.Join(home, editor[2:])
		}
	}

	editorParts := strings.Fields(editor)
	editorCmd := editorParts[0]
	editorArgs := editorParts[1:]
	editorArgs = append(editorArgs, zetPath)

	c := exec.Command(editorCmd, editorArgs...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		// Commit changes
		title, _ := getZettelTitle(zetDir, zetID)
		if title != "" {
			gitCommit(zetDir, zetID, title)
		}

		return editorFinishedMsg{err: err}
	})
}

func openEditorCmd(editor, filePath string) tea.Cmd {
	if strings.HasPrefix(editor, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			editor = filepath.Join(home, editor[2:])
		}
	}

	editorParts := strings.Fields(editor)
	editorCmd := editorParts[0]
	editorArgs := editorParts[1:]
	editorArgs = append(editorArgs, filePath)

	c := exec.Command(editorCmd, editorArgs...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		// Extract zetDir and zetID from filePath
		// filePath format: /path/to/zetdir/YYYYMMDDHHMMSS/README.md
		dir := filepath.Dir(filePath)
		zetID := filepath.Base(dir)
		zetDir := filepath.Dir(dir)

		// Update README and commit
		if isValidZettelID(zetID) {
			updateReadme(zetDir)
			title, _ := getZettelTitle(zetDir, zetID)
			if title != "" {
				gitCommit(zetDir, zetID, title)
			}
		}

		return editorFinishedMsg{err: err}
	})
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	zetDir := cfg.Directories.Zettelkasten
	if zetDir == "" {
		fmt.Println("ERROR: No zettelkasten location set.")
		fmt.Println("Please set zettelkasten directory in ~/.config/karya/config.toml")
		os.Exit(1)
	}

	if _, err := os.Stat(zetDir); os.IsNotExist(err) {
		fmt.Printf("ERROR: Zettelkasten directory does not exist: %s\n", zetDir)
		os.Exit(1)
	}

	editor := cfg.EDITOR
	if editor == "" {
		editor = "vim"
	}

	verbose := cfg.Verbose
	args := os.Args[1:]

	// Parse flags
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-v" || arg == "--verbose" {
			verbose = true
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	if len(args) == 0 {
		showInteractiveTUI(zetDir, editor, verbose)
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "-h", "--help", "help":
		printHelp()
	case "count":
		count, err := countZettels(zetDir)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(count)
	case "n", "new", "a", "add":
		title := ""
		if len(args) > 1 {
			title = strings.Join(args[1:], " ")
		}
		if err := newZettel(zetDir, title, editor); err != nil {
			log.Fatal(err)
		}
	case "?":
		if len(args) < 2 {
			fmt.Println("ERROR: Search pattern required")
			os.Exit(1)
		}
		pattern := strings.Join(args[1:], " ")
		results, err := searchZettels(zetDir, pattern)
		if err != nil {
			log.Fatal(err)
		}
		printSearchResults(results)
	case "d", "todo":
		results, err := findTodos(zetDir)
		if err != nil {
			log.Fatal(err)
		}
		printSearchResults(results)
	case "t?", "title?":
		if len(args) < 2 {
			fmt.Println("ERROR: Search pattern required")
			os.Exit(1)
		}
		pattern := strings.Join(args[1:], " ")
		results, err := searchZettelTitles(zetDir, pattern)
		if err != nil {
			log.Fatal(err)
		}
		printTitleSearchResults(results)
	case "e", "edit":
		if len(args) < 2 {
			showInteractiveTUI(zetDir, editor, verbose)
		} else {
			zetID := args[1]
			if err := editZettel(zetDir, zetID, editor); err != nil {
				log.Fatal(err)
			}
		}
	case "ls", "list":
		zettels, err := listZettels(zetDir)
		if err != nil {
			log.Fatal(err)
		}
		for _, z := range zettels {
			fmt.Printf("%s %s\n", magentaStyle.Render(z.ID), z.Title)
		}
	case "show":
		if len(args) < 2 {
			fmt.Println("ERROR: Zettel ID required")
			os.Exit(1)
		}
		zetID := args[1]
		if err := showZettel(zetDir, zetID); err != nil {
			log.Fatal(err)
		}
	case "last":
		if err := editLastZettel(zetDir, editor); err != nil {
			log.Fatal(err)
		}
	case "toc":
		tocPath := filepath.Join(zetDir, "README.md")
		cmd := exec.Command(editor, tocPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	help := `zet - Zettelkasten note management

USAGE:
    zet [OPTIONS] [COMMAND] [ARGS]

OPTIONS:
    -v, --verbose       Show zettel IDs in interactive mode

COMMANDS:
    (no command)        Show interactive TUI to browse and edit zettels
    n, new, a, add      Create a new zettel (optionally with title)
    e, edit [ID]        Edit a zettel (interactive if no ID provided)
    ls, list            List all zettels with IDs and titles
    show <ID>           Display zettel content
    count               Count total number of zettels
    ? <pattern>         Search for pattern across all zettels (substring match)
    t?, title? <pattern> Search for pattern in zettel titles (substring match)
    d, todo             Find all checklist items (- [ ]) across zettels
    last                Edit the most recently modified zettel
    toc                 Edit the table of contents (README.md)
    -h, --help, help    Show this help message

INTERACTIVE MODE:
    j/k or ↑/↓          Navigate zettels
    g / G               Jump to top / bottom
    Ctrl+d / Ctrl+u     Page down / up (vim-style)
    Ctrl+f / Ctrl+b     Page down / up (emacs-style)
    PgDn / PgUp         Page down / up
    s                   Toggle sort (newest first / oldest first)
    /                   Start filtering (substring match on ID and title)
    Enter               Edit selected zettel / Exit filter mode
    Esc                 Exit filter mode or clear filter
    q, Ctrl+C           Quit

SUBCOMMANDS (available in TUI):
    n                   Create new zettel
    l                   Edit last modified zettel
    d                   Delete selected zettel (with confirmation)
    T                   Edit table of contents (README.md)
    c                   Show count (visible in pagination)

SEARCH:
    Search uses case-insensitive substring matching (like cmd/todo).
    Searching for "go" will match "golang", "Go", "going", etc.

EXAMPLES:
    zet                           # Browse zettels interactively
    zet -v                        # Browse with zettel IDs visible
    zet new "My New Note"         # Create new zettel with title
    zet n                         # Create new zettel (prompt for title)
    zet edit 20231001120000       # Edit specific zettel
    zet ? "golang"                # Search for "golang" in all zettels
    zet t? "Programming"          # Search for "Programming" in titles
    zet todo                      # List all unchecked todo items
    zet last                      # Edit most recent zettel
    zet count                     # Show total zettel count

CONFIGURATION:
    Set zettelkasten directory in ~/.config/karya/config.toml:
    [directories]
    zettelkasten = "/path/to/zettelkasten"

ENVIRONMENT VARIABLES:
    EDITOR              Editor to use (default: vim)
`
	fmt.Print(help)
}

func showInteractiveTUI(zetDir, editor string, verbose bool) {
	zettels, err := listZettels(zetDir)
	if err != nil {
		log.Fatal(err)
	}

	if len(zettels) == 0 {
		fmt.Println("No zettels found")
		return
	}

	watcher, err := setupWatcher(zetDir)
	if err != nil {
		log.Printf("Warning: Could not create file watcher: %v", err)
	}

	items := make([]list.Item, len(zettels))
	for i, z := range zettels {
		items[i] = zettelItem{zettel: z, verbose: verbose}
	}

	delegate := zettelDelegate{DefaultDelegate: list.NewDefaultDelegate()}
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 0, 0)
	l.Title = "Zettels (Newest First)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.Quit.SetKeys("ctrl+c")

	// Keep help compact (short mode)
	l.Help.ShowAll = false

	// Match vim-style keybindings
	l.KeyMap.NextPage.SetKeys("pgdown", "ctrl+f", "ctrl+d")
	l.KeyMap.PrevPage.SetKeys("pgup", "ctrl+b", "ctrl+u")

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "edit"),
			),
			key.NewBinding(
				key.WithKeys("/"),
				key.WithHelp("/", "filter"),
			),
			key.NewBinding(
				key.WithKeys("s"),
				key.WithHelp("s", "sort"),
			),
			key.NewBinding(
				key.WithKeys("n"),
				key.WithHelp("n", "new"),
			),
			key.NewBinding(
				key.WithKeys("l"),
				key.WithHelp("l", "last"),
			),
			key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "delete"),
			),
		}
	}

	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "edit selected zettel"),
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
				key.WithHelp("s", "toggle sort (newest/oldest)"),
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
			key.NewBinding(
				key.WithKeys("n"),
				key.WithHelp("n", "new zettel"),
			),
			key.NewBinding(
				key.WithKeys("l"),
				key.WithHelp("l", "edit last zettel"),
			),
			key.NewBinding(
				key.WithKeys("T"),
				key.WithHelp("T", "edit TOC"),
			),
			key.NewBinding(
				key.WithKeys("c"),
				key.WithHelp("c", "count (see pagination)"),
			),
			key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "delete zettel"),
			),
		}
	}

	m := model{
		list:            l,
		zettels:         zettels,
		zetDir:          zetDir,
		editor:          editor,
		allItems:        items,
		verbose:         verbose,
		watcher:         watcher,
		sortNewestFirst: true,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}

	if watcher != nil {
		watcher.Close()
	}
}

func newZettel(zetDir, title, editor string) error {
	if title == "" {
		fmt.Print("Title: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		title = strings.TrimSpace(input)
	}

	zetID := time.Now().UTC().Format("20060102150405")
	fmt.Printf("Creating zettel: %s\n", zetID)

	if err := createZettel(zetDir, zetID, title); err != nil {
		return err
	}

	zetPath := filepath.Join(zetDir, zetID, "README.md")
	cmd := exec.Command(editor, zetPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	if err := updateReadme(zetDir); err != nil {
		return err
	}

	actualTitle, err := getZettelTitle(zetDir, zetID)
	if err != nil {
		actualTitle = title
	}

	if err := gitCommit(zetDir, zetID, actualTitle); err != nil {
		log.Printf("Warning: git commit failed: %v", err)
	}

	return nil
}

func editZettel(zetDir, zetID, editor string) error {
	if !isValidZettelID(zetID) {
		matches, err := findMatchingZettels(zetDir, zetID)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("no zettel found matching: %s", zetID)
		}
		if len(matches) > 1 {
			fmt.Println("Multiple matches found:")
			for _, z := range matches {
				fmt.Printf("  %s %s\n", z.ID, z.Title)
			}
			return fmt.Errorf("please specify a more complete ID")
		}
		zetID = matches[0].ID
	}

	zetPath := filepath.Join(zetDir, zetID, "README.md")
	if _, err := os.Stat(zetPath); os.IsNotExist(err) {
		return fmt.Errorf("zettel not found: %s", zetID)
	}

	title, err := getZettelTitle(zetDir, zetID)
	if err != nil {
		title = "Unknown"
	}
	fmt.Printf("EDITING: %s\n", title)

	cmd := exec.Command(editor, zetPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	if err := gitCommit(zetDir, zetID, title); err != nil {
		log.Printf("Warning: git commit failed: %v", err)
	}

	return nil
}

func editLastZettel(zetDir, editor string) error {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "-C", zetDir, "log", "--pretty=format:%h", "-n", "1")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(string(output))

	cmd = exec.Command("git", "-C", zetDir, "show", "--name-only", "--pretty=", commit)
	output, err = cmd.Output()
	if err != nil {
		return err
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 0 {
		return fmt.Errorf("no files in last commit")
	}

	zetID := ""
	for _, file := range files {
		if strings.Contains(file, "/") {
			parts := strings.Split(file, "/")
			if len(parts) > 0 && isValidZettelID(parts[0]) {
				zetID = parts[0]
				break
			}
		}
	}

	if zetID == "" {
		return fmt.Errorf("could not determine zettel ID from last commit")
	}

	fmt.Printf("Editing last zettel: %s\n", zetID)
	return editZettel(zetDir, zetID, editor)
}

func showZettel(zetDir, zetID string) error {
	zetPath := filepath.Join(zetDir, zetID, "README.md")
	if _, err := os.Stat(zetPath); os.IsNotExist(err) {
		return fmt.Errorf("zettel not found: %s", zetID)
	}

	content, err := os.ReadFile(zetPath)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Print(string(content))
	fmt.Println()

	return nil
}

func createZettel(zetDir, zetID, title string) error {
	zetPath := filepath.Join(zetDir, zetID)
	if err := os.MkdirAll(zetPath, 0755); err != nil {
		return err
	}

	readmePath := filepath.Join(zetPath, "README.md")
	content := fmt.Sprintf("# %s\n\n\n", title)
	return os.WriteFile(readmePath, []byte(content), 0644)
}

func listZettels(zetDir string) ([]Zettel, error) {
	entries, err := os.ReadDir(zetDir)
	if err != nil {
		return nil, err
	}

	// Filter valid zettel directories
	var validDirs []string
	for _, entry := range entries {
		if entry.IsDir() && isValidZettelID(entry.Name()) {
			validDirs = append(validDirs, entry.Name())
		}
	}

	if len(validDirs) == 0 {
		return []Zettel{}, nil
	}

	// Calculate worker count (similar to todo)
	numWorkers := len(validDirs)
	maxWorkers := 8 // Reasonable limit for directory operations
	if numWorkers > maxWorkers {
		numWorkers = maxWorkers
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Create channels
	jobs := make(chan string, len(validDirs))
	results := make(chan Zettel, len(validDirs))

	// Start workers
	for w := 0; w < numWorkers; w++ {
		go func() {
			for zetID := range jobs {
				readmePath := filepath.Join(zetDir, zetID, "README.md")
				title, err := getZettelTitle(zetDir, zetID)
				if err != nil {
					continue
				}
				results <- Zettel{
					ID:    zetID,
					Title: title,
					Path:  readmePath,
				}
			}
		}()
	}

	// Send jobs
	for _, zetID := range validDirs {
		jobs <- zetID
	}
	close(jobs)

	// Collect results
	var zettels []Zettel
	for i := 0; i < len(validDirs); i++ {
		select {
		case z := <-results:
			zettels = append(zettels, z)
		case <-time.After(5 * time.Second):
			// Timeout after 5 seconds
			break
		}
	}

	sort.Slice(zettels, func(i, j int) bool {
		return zettels[i].ID > zettels[j].ID
	})

	return zettels, nil
}

func countZettels(zetDir string) (int, error) {
	entries, err := os.ReadDir(zetDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() && isValidZettelID(entry.Name()) {
			count++
		}
	}

	return count, nil
}

func getZettelTitle(zetDir, zetID string) (string, error) {
	readmePath := filepath.Join(zetDir, zetID, "README.md")
	file, err := os.Open(readmePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:]), nil
		}
	}

	return "", fmt.Errorf("no title found")
}

func searchZettels(zetDir, pattern string) ([]SearchResult, error) {
	zettels, err := listZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	patternLower := strings.ToLower(pattern)

	for _, z := range zettels {
		file, err := os.Open(z.Path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), patternLower) {
				results = append(results, SearchResult{
					ZettelID: z.ID,
					Title:    z.Title,
					LineNum:  lineNum,
					Line:     line,
					Path:     z.Path,
				})
			}
		}
		file.Close()
	}

	return results, nil
}

func searchZettelTitles(zetDir, pattern string) ([]Zettel, error) {
	zettels, err := listZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var results []Zettel
	patternLower := strings.ToLower(pattern)

	for _, z := range zettels {
		if strings.Contains(strings.ToLower(z.Title), patternLower) {
			results = append(results, z)
		}
	}

	return results, nil
}

func findTodos(zetDir string) ([]SearchResult, error) {
	zettels, err := listZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	todoPattern := regexp.MustCompile(`- \[ \]`)

	for _, z := range zettels {
		file, err := os.Open(z.Path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if todoPattern.MatchString(line) {
				results = append(results, SearchResult{
					ZettelID: z.ID,
					Title:    z.Title,
					LineNum:  lineNum,
					Line:     line,
					Path:     z.Path,
				})
			}
		}
		file.Close()
	}

	return results, nil
}

func updateReadme(zetDir string) error {
	zettels, err := listZettels(zetDir)
	if err != nil {
		return err
	}

	var content strings.Builder
	content.WriteString("# Index\n")

	for _, z := range zettels {
		content.WriteString(fmt.Sprintf("* [%s](./%s/README.md) - %s\n", z.ID, z.ID, z.Title))
	}

	readmePath := filepath.Join(zetDir, "README.md")
	return os.WriteFile(readmePath, []byte(content.String()), 0644)
}

func deleteZettel(zetDir, zetID string) error {
	zetPath := filepath.Join(zetDir, zetID)

	// Get the title before deleting
	title, err := getZettelTitle(zetDir, zetID)
	if err != nil {
		title = zetID // Fallback to ID if title can't be read
	}

	// Remove the directory
	if err := os.RemoveAll(zetPath); err != nil {
		return err
	}

	// Update README
	if err := updateReadme(zetDir); err != nil {
		return err
	}

	// Git operations
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil // No git repo, we're done
	}

	// Git rm the zettel directory
	cmd := exec.Command("git", "-C", zetDir, "rm", "-rf", zetID)
	if err := cmd.Run(); err != nil {
		// If git rm fails, just add the README
		cmd = exec.Command("git", "-C", zetDir, "add", "README.md")
		cmd.Run()
	} else {
		// Also add the updated README
		cmd = exec.Command("git", "-C", zetDir, "add", "README.md")
		cmd.Run()
	}

	// Commit with title
	commitMsg := fmt.Sprintf("Delete zettel '%s'", title)
	cmd = exec.Command("git", "-C", zetDir, "commit", "-m", commitMsg)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Push if remote exists
	cmd = exec.Command("git", "-C", zetDir, "remote")
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		return nil
	}

	cmd = exec.Command("git", "-C", zetDir, "push")
	return cmd.Run()
}

func gitCommit(zetDir, zetID, title string) error {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	zetPath := filepath.Join(zetID, "README.md")
	readmePath := "README.md"

	cmd := exec.Command("git", "-C", zetDir, "add", zetPath, readmePath)
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "-C", zetDir, "commit", "-m", title)
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "-C", zetDir, "remote")
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		fmt.Println("No remotes for this repository")
		return nil
	}

	cmd = exec.Command("git", "-C", zetDir, "push")
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func isValidZettelID(id string) bool {
	if len(id) != 14 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func findMatchingZettels(zetDir, prefix string) ([]Zettel, error) {
	zettels, err := listZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var matches []Zettel
	for _, z := range zettels {
		if strings.HasPrefix(z.ID, prefix) {
			matches = append(matches, z)
		}
	}

	return matches, nil
}

func printSearchResults(results []SearchResult) {
	if len(results) == 0 {
		return
	}

	currentTitle := ""
	for _, r := range results {
		if r.Title != currentTitle {
			fmt.Println()
			fmt.Printf("%s: %s\n",
				magentaStyle.Render(r.ZettelID),
				boldOrange.Render(r.Title))
			currentTitle = r.Title
		}
		fmt.Printf("[%s]: %s\n",
			yellowStyle.Render(fmt.Sprintf("%d", r.LineNum)),
			r.Line)
	}
}

func printTitleSearchResults(results []Zettel) {
	for _, z := range results {
		fmt.Printf("%s: %s\n",
			magentaStyle.Render(z.ID),
			z.Title)
	}
}

func setupWatcher(zetDir string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	updateWatcher(watcher, zetDir)
	return watcher, nil
}

func updateWatcher(watcher *fsnotify.Watcher, zetDir string) {
	if watcher == nil {
		return
	}

	dirsToWatch := getWatchDirectories(zetDir)
	for _, dir := range dirsToWatch {
		watcher.Add(dir)
	}
}

func getWatchDirectories(zetDir string) []string {
	var dirs []string

	filepath.Walk(zetDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})

	return dirs
}
