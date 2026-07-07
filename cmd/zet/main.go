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
	"sort"
	"strings"
	"time"

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/task"
	"github.com/vinayprograms/karya/internal/zet"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
)

type Zettel = zet.Zettel
type SearchResult = zet.SearchResult

type ColorScheme struct {
	zettelIDStyle   lipgloss.Style
	titleStyle      lipgloss.Style
	normalStyle     lipgloss.Style
	highlightStyle  lipgloss.Style
	matchCountStyle lipgloss.Style
	selectorStyle   lipgloss.Style
}

var colors ColorScheme

func InitializeColors(cfg *config.Config) {
	colors = ColorScheme{
		zettelIDStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		titleStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TaskColor)),
		normalStyle:     lipgloss.NewStyle(),
		highlightStyle:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		matchCountStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.InProgressColor)).Bold(true),
		selectorStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true),
	}
}

// listView defines how a view loads and presents its items.
type listView interface {
	refresh(zetDir string, verbose bool) []list.Item
	title(sortNewestFirst bool) string
	cursorGlyph() string
	helpLines() (commands string, nav string)
	renderHeader(width int) string
}

type allZettelsView struct{}

func (v allZettelsView) refresh(zetDir string, verbose bool) []list.Item {
	zettels, err := zet.ListZettels(zetDir)
	if err != nil {
		return nil
	}
	items := make([]list.Item, len(zettels))
	for i, z := range zettels {
		items[i] = zettelItem{zettel: z, verbose: verbose}
	}
	return items
}

func (v allZettelsView) title(sortNewestFirst bool) string {
	if sortNewestFirst {
		return "Zettels (Newest First)"
	}
	return "Zettels (Oldest First)"
}

func (v allZettelsView) cursorGlyph() string { return "█" }

func (v allZettelsView) helpLines() (string, string) {
	return " Commands: n:new • l:last • d:delete • T:toc • P:pinboard • c:count",
		" ↑↓/jk • g/G:top/bottom • Ctrl+d/u:page • /:title search • *:fulltext search • s:sort order • Enter:edit • q:quit • ?:more help "
}

func (v allZettelsView) renderHeader(width int) string { return "" }

type pinboardView struct{}

func (v pinboardView) refresh(zetDir string, verbose bool) []list.Item {
	pinned, err := zet.ListPinnedZettels(zetDir)
	if err != nil {
		return nil
	}
	items := make([]list.Item, len(pinned))
	for i, z := range pinned {
		items[i] = zettelItem{zettel: z, verbose: verbose}
	}
	return items
}

func (v pinboardView) title(_ bool) string { return "Pinboard" }

func (v pinboardView) cursorGlyph() string { return "░" }

func (v pinboardView) helpLines() (string, string) {
	return " Pinboard: d:delete • Enter:edit • P/Esc:back to all",
		" ↑↓/jk • g/G:top/bottom • Ctrl+d/u:page • /:title search • *:fulltext search • q:quit"
}

func (v pinboardView) renderHeader(width int) string {
	title := " ◆ Pinboard "
	padding := width - lipgloss.Width(title)
	if padding < 0 {
		padding = 0
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("5")).
		Render(title + strings.Repeat(" ", padding))
}

type zettelItem struct {
	zettel        Zettel
	verbose       bool
	searchResults []SearchResult // For fulltext search results
}

func (i zettelItem) FilterValue() string {
	return i.zettel.ID + " " + i.zettel.Title
}

func (i zettelItem) renderWithSelection(isSelected bool, glyph string) string {
	var parts []string

	// Show match count if there are search results
	if len(i.searchResults) > 0 {
		matchCount := colors.matchCountStyle.Render(fmt.Sprintf("[%d] ", len(i.searchResults)))
		parts = append(parts, matchCount)
	}

	if i.verbose {
		parts = append(parts, colors.zettelIDStyle.Render(fmt.Sprintf("%-14s", i.zettel.ID)))
	}

	if isSelected {
		indicator := colors.selectorStyle.Render(glyph + " ")
		parts = append(parts, indicator+colors.titleStyle.Render(i.zettel.Title))
	} else {
		parts = append(parts, "  "+colors.titleStyle.Render(i.zettel.Title))
	}

	return strings.Join(parts, " ")
}

func (i zettelItem) Title() string {
	var parts []string

	if i.verbose {
		parts = append(parts, colors.zettelIDStyle.Render(fmt.Sprintf("%-14s", i.zettel.ID)))
	}

	parts = append(parts, colors.titleStyle.Render(i.zettel.Title))
	return strings.Join(parts, " ")
}

func (i zettelItem) Description() string { return "" }

type zettelDelegate struct {
	list.DefaultDelegate
	cursorGlyph *string
}

func (d zettelDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	zi, ok := item.(zettelItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	glyph := "█"
	if d.cursorGlyph != nil {
		glyph = *d.cursorGlyph
	}
	content := zi.renderWithSelection(isSelected, glyph)
	fmt.Fprint(w, content)
}

type model struct {
	list                   list.Model
	zettels                []Zettel
	zetDir                 string
	quitting               bool
	editor                 string
	verbose                bool
	watcher                *fsnotify.Watcher
	sortNewestFirst        bool
	showDeleteConfirm      bool
	deleteZettel           *Zettel
	deleteConfirmSelection int // 0 = Cancel, 1 = Delete
	filtering              bool
	filterMode             string // "title" or "fulltext"
	customFilter           string
	allItems               []list.Item
	searchResults          map[string][]SearchResult // Map zettel ID to search results
	activeView             listView
	views                  map[string]listView
	cursorGlyph            *string
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
					title, _ := zet.GetZettelTitle(m.zetDir, m.deleteZettel.ID)
					err := zet.DeleteZettel(m.zetDir, m.deleteZettel.ID)
					if err != nil {
						log.Printf("Error deleting zettel: %v", err)
					}
					// Git commit the deletion
					if title != "" {
						zet.GitDeleteZettel(m.zetDir, m.deleteZettel.ID, title)
					}
					m.showDeleteConfirm = false
					m.deleteZettel = nil
					m.deleteConfirmSelection = 0

					// Reload zettels
					zettels, err := zet.ListZettelsFromIndex(m.zetDir)
					if err == nil {
						m.zettels = zettels
						items := make([]list.Item, len(m.zettels))
						for i, z := range m.zettels {
							items[i] = zettelItem{zettel: z, verbose: m.verbose}
						}
						m.allItems = items
						m.list.SetItems(items)
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

		// Handle filtering
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
					m.applyFilter()
				} else {
					// Exit filtering if filter is empty
					m.filtering = false
					m.list.SetItems(m.allItems)
				}
				return m, nil
			default:
				// Add character to filter
				if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 {
					m.customFilter += string(msg.Runes[0])
					m.applyFilter()
					return m, nil
				}
				// Ignore other keys while filtering
				return m, nil
			}
		}

		// Handle esc when not filtering
		if msg.String() == "esc" {
			if m.activeView != m.views["all"] {
				m.switchView("all")
				return m, nil
			}
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

		// Start title filter
		if msg.String() == "/" {
			m.filtering = true
			m.filterMode = "title"
			return m, nil
		}

		// Start fulltext filter
		if msg.String() == "*" {
			m.filtering = true
			m.filterMode = "fulltext"
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
			return m, openEditorCmd(m.editor, tocPath, "")
		}

		if msg.String() == "shift+p" || msg.String() == "P" {
			if m.activeView == m.views["pinboard"] {
				m.switchView("all")
			} else {
				m.switchView("pinboard")
			}
			return m, nil
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
			if i, ok := m.list.SelectedItem().(zettelItem); ok {
				// Pass search term if in fulltext mode
				searchTerm := ""
				if m.filterMode == "fulltext" && m.customFilter != "" {
					searchTerm = m.customFilter
				}
				return m, openEditorCmd(m.editor, i.zettel.Path, searchTerm)
			}
		}
	case fileChangedMsg:
		m.refreshList()
		updateWatcher(m.watcher, m.zetDir)
		return m, waitForFileChange(m.watcher)
	case editorFinishedMsg:
		if msg.err != nil {
			log.Printf("Editor error: %v", msg.err)
		}
		m.refreshList()
		return m, nil
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 5) // Reduce by 5: 2 for custom help lines + 3 for spacing/margins
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

	lines := strings.Split(view, "\n")
	if len(lines) > 0 {
		// Build header in order: Title → Filter → Pagination
		titleLine := lines[0]
		if h := m.activeView.renderHeader(m.list.Width()); h != "" {
			titleLine = h
		}
		header := []string{titleLine}

		// Add filter line (always reserve space)
		var filterLine string
		if m.filtering || m.customFilter != "" {
			var filterText string
			modeLabel := "Search by title"
			if m.filterMode == "fulltext" {
				modeLabel = "Fulltext search"
			}
			if m.filtering {
				filterText = fmt.Sprintf("%s: %s▓", modeLabel, m.customFilter)
			} else {
				filterText = fmt.Sprintf("%s: %s", modeLabel, m.customFilter)
			}
			filterLine = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")).
				Background(lipgloss.Color("0")).
				Padding(0, 1).
				Render(filterText)
		} else {
			filterLine = "" // Empty line when not filtering
		}
		header = append(header, filterLine)

		// Add pagination info if multiple pages
		totalItems := len(m.list.Items())
		if totalItems > 0 {
			p := m.list.Paginator
			totalPages := p.TotalPages
			if totalPages > 1 {
				currentPage := p.Page
				itemsPerPage := p.PerPage
				startIdx := currentPage * itemsPerPage
				endIdx := startIdx + itemsPerPage
				if endIdx > totalItems {
					endIdx = totalItems
				}
				paginationInfo := lipgloss.NewStyle().
					Foreground(lipgloss.Color("240")).
					Render(fmt.Sprintf("Showing %d-%d of %d • Page %d/%d",
						startIdx+1, endIdx, totalItems, currentPage+1, totalPages))
				header = append(header, paginationInfo)
			}
		}

		// Combine header with rest of list (skip first line which is title)
		result := header
		result = append(result, lines[1:]...)
		view = strings.Join(result, "\n")
	}

	// Replace the default help with a custom compact status bar
	lines = strings.Split(view, "\n")
	if len(lines) > 0 {
		// Remove the last line (default help)
		lines = lines[:len(lines)-1]

		// Create custom compact help (2 lines)
		commandStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")) //.
			//Background(lipgloss.Color("236"))
		navStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

		cmd, nav := m.activeView.helpLines()
		line1 := commandStyle.Render(cmd)
		line2 := navStyle.Render(nav)

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

func (m *model) applyFilter() {
	if m.customFilter == "" {
		m.list.SetItems(m.allItems)
		m.searchResults = nil
		return
	}

	filterLower := strings.ToLower(m.customFilter)
	var filteredItems []list.Item

	if m.filterMode == "fulltext" {
		// Clear previous search results
		m.searchResults = make(map[string][]SearchResult)
	}

	for _, item := range m.allItems {
		if zetItem, ok := item.(zettelItem); ok {
			if m.filterMode == "title" {
				// Title search: search in ID and title
				if strings.Contains(strings.ToLower(zetItem.zettel.ID), filterLower) ||
					strings.Contains(strings.ToLower(zetItem.zettel.Title), filterLower) {
					filteredItems = append(filteredItems, item)
				}
			} else if m.filterMode == "fulltext" {
				// Fulltext search: search in file content and collect results
				results := m.searchInFile(zetItem.zettel.Path, filterLower)
				if len(results) > 0 {
					m.searchResults[zetItem.zettel.ID] = results
					// Create new zettelItem with search results
					newItem := zettelItem{
						zettel:        zetItem.zettel,
						verbose:       zetItem.verbose,
						searchResults: results,
					}
					filteredItems = append(filteredItems, newItem)
				}
			}
		}
	}

	m.list.SetItems(filteredItems)
}

func (m *model) searchInFile(filePath, searchTerm string) []SearchResult {
	return zet.SearchInFile(filePath, searchTerm)
}

func (m *model) switchView(name string) {
	m.activeView = m.views[name]
	*m.cursorGlyph = m.activeView.cursorGlyph()
	m.customFilter = ""
	m.list.Title = m.activeView.title(m.sortNewestFirst)
	items := m.activeView.refresh(m.zetDir, m.verbose)
	if name == "all" {
		m.allItems = items
	}
	m.list.SetItems(items)
	m.list.ResetSelected()
}

func (m *model) refreshList() {
	zettels, err := zet.ListZettels(m.zetDir)
	if err != nil {
		return
	}
	m.zettels = zettels
	items := make([]list.Item, len(m.zettels))
	for i, z := range m.zettels {
		items[i] = zettelItem{zettel: z, verbose: m.verbose}
	}
	m.allItems = items

	viewItems := m.activeView.refresh(m.zetDir, m.verbose)
	if m.customFilter != "" {
		m.applyFilter()
	} else {
		m.list.SetItems(viewItems)
	}
	m.list.ResetSelected()
}

func (m *model) sortZettels() {
	// Sort the zettels slice
	sort.Slice(m.zettels, func(i, j int) bool {
		if m.sortNewestFirst {
			return m.zettels[i].ID > m.zettels[j].ID
		}
		return m.zettels[i].ID < m.zettels[j].ID
	})

	m.list.Title = m.activeView.title(m.sortNewestFirst)

	// Recreate items with sorted zettels
	items := make([]list.Item, len(m.zettels))
	for i, z := range m.zettels {
		items[i] = zettelItem{zettel: z, verbose: m.verbose}
	}
	m.allItems = items

	// Reapply filter if active
	if m.customFilter != "" {
		m.applyFilter()
	} else {
		m.list.SetItems(items)
	}
	m.list.ResetSelected()
}

type editorFinishedMsg struct{ err error }

func newZettelCmd(zetDir, editor string) tea.Cmd {
	zetID := zet.GenerateZettelID()

	// Create empty zettel
	if err := zet.CreateZettel(zetDir, zetID, ""); err != nil {
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
		// Update README and pinboard after editing
		zet.UpdateReadme(zetDir)
		zet.UpdatePinboard(zetDir)

		// Get actual title and commit
		actualTitle, _ := zet.GetZettelTitle(zetDir, zetID)
		if actualTitle != "" {
			zet.GitCommit(zetDir, zetID, actualTitle)
		}

		return editorFinishedMsg{err: err}
	})
}

func editLastZettelCmd(zetDir, editor string) tea.Cmd {
	zetID, err := zet.GetLastZettelID(zetDir)
	if err != nil {
		return func() tea.Msg {
			return editorFinishedMsg{err: err}
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
		// Update pinboard and commit changes
		zet.UpdatePinboard(zetDir)
		title, _ := zet.GetZettelTitle(zetDir, zetID)
		if title != "" {
			zet.GitCommit(zetDir, zetID, title)
		}

		return editorFinishedMsg{err: err}
	})
}

func openEditorCmd(editor, filePath, searchTerm string) tea.Cmd {
	if strings.HasPrefix(editor, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			editor = filepath.Join(home, editor[2:])
		}
	}

	editorParts := strings.Fields(editor)
	editorCmd := editorParts[0]
	editorArgs := editorParts[1:]

	// Add search term support for common editors
	if searchTerm != "" {
		editorName := filepath.Base(editorCmd)
		switch editorName {
		case "vim", "nvim", "vi":
			// Vim: +/pattern to search and highlight
			editorArgs = append(editorArgs, fmt.Sprintf("+/%s", searchTerm))
		case "nano":
			// Nano: -w (disable line wrapping) and then we can't directly search, but we can go to first match
			// Nano doesn't support opening with search, user will need to Ctrl+W to search
		case "emacs":
			// Emacs: --eval to search
			editorArgs = append(editorArgs, "--eval", fmt.Sprintf("(progn (goto-char (point-min)) (search-forward \"%s\" nil t))", searchTerm))
		case "code", "code-insiders":
			// VS Code: -g flag with :line:column, but we can't highlight search
			// VS Code doesn't support opening with search highlighting
		case "subl", "sublime_text":
			// Sublime: doesn't support opening with search
		}
	}

	editorArgs = append(editorArgs, filePath)

	c := exec.Command(editorCmd, editorArgs...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		// Extract zetDir and zetID from filePath
		// filePath format: /path/to/zetdir/YYYYMMDDHHMMSS/README.md
		dir := filepath.Dir(filePath)
		zetID := filepath.Base(dir)
		zetDir := filepath.Dir(dir)

		// Update README, pinboard, and commit
		if zet.IsValidZettelID(zetID) {
			zet.UpdateReadme(zetDir)
			zet.UpdatePinboard(zetDir)
			title, _ := zet.GetZettelTitle(zetDir, zetID)
			if title != "" {
				zet.GitCommit(zetDir, zetID, title)
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

	InitializeColors(cfg)

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

	editor := cfg.GeneralConfig.EDITOR
	if editor == "" {
		editor = "vim"
	}

	verbose := cfg.GeneralConfig.Verbose
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
		count, err := zet.CountZettels(zetDir)
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
		results, err := zet.SearchZettels(zetDir, pattern)
		if err != nil {
			log.Fatal(err)
		}
		printSearchResults(results)
	case "d", "todo":
		cfg.Todo.Structured = true // Always use structured mode for zettels

		// Get the parent directory (zettelkasten root)
		files, err := filepath.Glob(filepath.Join(zetDir, "??????????????", "README.md"))
		if err != nil {
			log.Fatal(err)
		}

		// Process each file and collect tasks
		var allTasks []*task.Task
		for _, file := range files {
			tasks, err := task.ProcessFile(cfg, file)
			if err != nil {
				continue
			}
			allTasks = append(allTasks, tasks...)
		}

		// Group tasks by zettel
		tasksByZettel := make(map[string][]*task.Task)
		for _, t := range allTasks {
			tasksByZettel[t.Zettel] = append(tasksByZettel[t.Zettel], t)
		}

		// Print tasks grouped by zettel
		for zettelID, tasks := range tasksByZettel {
			if len(tasks) == 0 {
				continue
			}

			// Get zettel title
			title, _ := zet.GetZettelTitle(zetDir, zettelID)
			if title == "" {
				title = "Unknown"
			}

			// Print zettel header once
			fmt.Printf("\n%s: %s\n",
				colors.zettelIDStyle.Render(zettelID),
				colors.highlightStyle.Render(title))

			// Print all tasks for this zettel
			for _, t := range tasks {
				taskLine := fmt.Sprintf("%s: %s", t.Keyword, t.Title)
				for _, tag := range t.Tags {
					taskLine += fmt.Sprintf(" #%s", tag)
				}
				if t.ScheduledAt != "" {
					taskLine += fmt.Sprintf(" @s:%s", t.ScheduledAt)
				}
				if t.DueAt != "" {
					taskLine += fmt.Sprintf(" @d:%s", t.DueAt)
				}
				if t.Assignee != "" {
					taskLine += fmt.Sprintf(" >> %s", t.Assignee)
				}
				fmt.Println(taskLine)
			}
		}
	case "t?", "title?":
		if len(args) < 2 {
			fmt.Println("ERROR: Search pattern required")
			os.Exit(1)
		}
		pattern := strings.Join(args[1:], " ")
		results, err := zet.SearchZettelTitles(zetDir, pattern)
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
		zettels, err := zet.ListZettelsFromIndex(zetDir)
		if err != nil {
			log.Fatal(err)
		}
		for _, z := range zettels {
			fmt.Printf("%s %s\n", colors.zettelIDStyle.Render(z.ID), z.Title)
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
	case "mcp":
		// Start MCP server on stdio
		mcpServer := zet.NewMCPServer(zetDir)
		ctx := context.Background()
		if err := mcpServer.Run(ctx); err != nil {
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
    d, todo             Find all tasks across zettels
    last                Edit the most recently modified zettel
    toc                 Edit the table of contents (README.md)
    mcp                 Start MCP server (stdio) for AI agent integration
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
    zet mcp                       # Start MCP server for AI agents

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
	zettels, err := zet.ListZettelsFromIndex(zetDir)
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

	cursorGlyph := "█"
	delegate := zettelDelegate{
		DefaultDelegate: list.NewDefaultDelegate(),
		cursorGlyph:     &cursorGlyph,
	}
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)

	l := list.New(items, delegate, 0, 0)
	l.Title = "Zettels (Newest First)"
	l.SetShowTitle(true)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.Quit.SetKeys("q")
	l.KeyMap.ForceQuit.SetKeys("ctrl+c")

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

	views := map[string]listView{
		"all":      allZettelsView{},
		"pinboard": pinboardView{},
	}
	allView := views["all"]

	m := model{
		list:            l,
		zettels:         zettels,
		zetDir:          zetDir,
		editor:          editor,
		verbose:         verbose,
		watcher:         watcher,
		sortNewestFirst: true,
		allItems:        items,
		activeView:      allView,
		views:           views,
		cursorGlyph:     &cursorGlyph,
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

	zetID := zet.GenerateZettelID()
	fmt.Printf("Creating zettel: %s\n", zetID)

	if err := zet.CreateZettel(zetDir, zetID, title); err != nil {
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

	if err := zet.UpdateReadme(zetDir); err != nil {
		return err
	}

	actualTitle, err := zet.GetZettelTitle(zetDir, zetID)
	if err != nil {
		actualTitle = title
	}

	if err := zet.GitCommit(zetDir, zetID, actualTitle); err != nil {
		log.Printf("Warning: git commit failed: %v", err)
	}

	return nil
}

func editZettel(zetDir, zetID, editor string) error {
	if !zet.IsValidZettelID(zetID) {
		matches, err := zet.FindMatchingZettels(zetDir, zetID)
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

	title, err := zet.GetZettelTitle(zetDir, zetID)
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

	if err := zet.GitCommit(zetDir, zetID, title); err != nil {
		log.Printf("Warning: git commit failed: %v", err)
	}

	return nil
}

func editLastZettel(zetDir, editor string) error {
	zetID, err := zet.GetLastZettelID(zetDir)
	if err != nil {
		return err
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

func printSearchResults(results []SearchResult) {
	if len(results) == 0 {
		return
	}

	currentTitle := ""
	for _, r := range results {
		if r.Title != currentTitle {
			fmt.Println()
			fmt.Printf("%s: %s\n",
				colors.zettelIDStyle.Render(r.ZettelID),
				colors.highlightStyle.Render(r.Title))
			currentTitle = r.Title
		}
		fmt.Printf("[%s]: %s\n",
			colors.highlightStyle.Render(fmt.Sprintf("%d", r.LineNum)),
			r.Line)
	}
}

func printTitleSearchResults(results []Zettel) {
	for _, z := range results {
		fmt.Printf("%s: %s\n",
			colors.zettelIDStyle.Render(z.ID),
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
