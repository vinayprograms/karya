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

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/note"
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
	zettelIDStyle  lipgloss.Style
	titleStyle     lipgloss.Style
	normalStyle    lipgloss.Style
	highlightStyle lipgloss.Style
	projectStyle   lipgloss.Style
	grayStyle      lipgloss.Style
	selectorStyle  lipgloss.Style
	navStyle       lipgloss.Style
	commandStyle   lipgloss.Style
	errorStyle     lipgloss.Style
	successStyle   lipgloss.Style
	filterStyle    lipgloss.Style
}

var colors ColorScheme

func InitializeColors(cfg *config.Config) {
	colors = ColorScheme{
		zettelIDStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		titleStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.TaskColor)),
		normalStyle:    lipgloss.NewStyle(),
		highlightStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.Colors.ActiveColor)),
		projectStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		grayStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.CompletedColor)),
		selectorStyle:  colors.selectorStyle,
		navStyle:       colors.navStyle,
		commandStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		errorStyle:     colors.errorStyle,
		successStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ProjectColor)),
		filterStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Colors.ActiveColor)).Background(lipgloss.Color("0")),
	}
}

type Project struct {
	Name      string
	Path      string
	HasNotes  bool
	HasTodos  bool
	NoteCount int
	TodoCount int
}

type projectModel struct {
	list          list.Model
	projects      []Project
	prjDir        string
	quitting      bool
	editor        string
	verbose       bool
	launchProject string
	cfg           *config.Config
	filtering     bool
	filterMode    string
	customFilter  string
	allItems      []list.Item
}

type projectItem struct {
	project Project
}

func (i projectItem) FilterValue() string {
	return i.project.Name
}

func (i projectItem) renderWithSelection(isSelected bool) string {
	var parts []string

	// Name line
	if isSelected {
		parts = append(parts, colors.selectorStyle.Render("█ ")+i.project.Name)
	} else {
		parts = append(parts, "  "+i.project.Name)
	}

	// Stats line
	stats := fmt.Sprintf("  %d notes • %d todos", i.project.NoteCount, i.project.TodoCount)
	if isSelected {
		statsStyle := colors.highlightStyle
		parts = append(parts, statsStyle.Render(stats))
	} else {
		statsStyle := colors.grayStyle
		parts = append(parts, statsStyle.Render(stats))
	}

	// Add extra blank line for spacing
	parts = append(parts, "")

	return strings.Join(parts, "\n")
}

func (i projectItem) Title() string {
	return i.project.Name
}

func (i projectItem) Description() string {
	return fmt.Sprintf("%d notes • %d todos", i.project.NoteCount, i.project.TodoCount)
}

type projectDelegate struct {
	list.DefaultDelegate
}

func (d projectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	projectItem, ok := item.(projectItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	content := projectItem.renderWithSelection(isSelected)
	fmt.Fprint(w, content)
}

func (m projectModel) Init() tea.Cmd {
	return nil
}

func (m projectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

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
					m.applyProjectFilter()
				} else {
					m.filtering = false
					m.list.SetItems(m.allItems)
				}
				return m, nil
			default:
				if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 {
					m.customFilter += string(msg.Runes[0])
					m.applyProjectFilter()
					return m, nil
				}
				return m, nil
			}
		}

		if msg.String() == "/" {
			m.filtering = true
			m.filterMode = "project"
			m.customFilter = ""
			return m, nil
		}

		if msg.String() == "*" {
			m.filtering = true
			m.filterMode = "fulltext"
			m.customFilter = ""
			return m, nil
		}

		if msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}

		if msg.String() == "enter" {
			if i, ok := m.list.SelectedItem().(projectItem); ok {
				m.launchProject = i.project.Name
				m.quitting = true
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 3) // Reduce by 3 for title and help lines
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Remove unused functions

func (m projectModel) View() string {
	if m.quitting {
		return ""
	}
	
	view := m.list.View()
	
	// Add filter line if filtering
	if m.filtering || m.customFilter != "" {
		lines := strings.Split(view, "\n")
		if len(lines) > 0 {
			header := []string{lines[0]}
			
			var filterText string
			modeLabel := "Filter projects"
			if m.filterMode == "fulltext" {
				modeLabel = "Fulltext search"
			}
			if m.filtering {
				filterText = fmt.Sprintf("%s: %s▓", modeLabel, m.customFilter)
			} else {
				filterText = fmt.Sprintf("%s: %s", modeLabel, m.customFilter)
			}
			filterLine := colors.filterStyle.
				Padding(0, 1).
				Render(filterText)
			header = append(header, filterLine)
			
			result := header
			result = append(result, lines[1:]...)
			view = strings.Join(result, "\n")
		}
	}
	
	// Customize the status bar to show "projects" instead of "items"
	lines := strings.Split(view, "\n")
	if len(lines) > 0 {
		// Look for the status line (usually contains "item" or "items")
		for i, line := range lines {
			if strings.Contains(line, "item") || strings.Contains(line, "Item") {
				// Replace "item(s)" with "project(s)"
				lines[i] = strings.Replace(line, "item", "project", -1)
				lines[i] = strings.Replace(lines[i], "Item", "Project", -1)
				break
			}
		}
		view = strings.Join(lines, "\n")
	}
	
	return view
}

func (m *projectModel) applyProjectFilter() {
	if m.customFilter == "" {
		m.list.SetItems(m.allItems)
		return
	}

	filterLower := strings.ToLower(m.customFilter)
	var filteredItems []list.Item

	if m.filterMode == "fulltext" {
		// Search across all notes in all projects
		for _, item := range m.allItems {
			if prjItem, ok := item.(projectItem); ok {
				notesPath := filepath.Join(prjItem.project.Path, "notes")
				if _, err := os.Stat(notesPath); err == nil {
					// Check if this project has any matching notes
					if hasMatchingNotes(notesPath, filterLower) {
						filteredItems = append(filteredItems, item)
					}
				}
			}
		}
	} else {
		// Filter by project name
		for _, item := range m.allItems {
			if prjItem, ok := item.(projectItem); ok {
				if strings.Contains(strings.ToLower(prjItem.project.Name), filterLower) {
					filteredItems = append(filteredItems, item)
				}
			}
		}
	}

	m.list.SetItems(filteredItems)
}

func hasMatchingNotes(notesPath, searchTerm string) bool {
	zettels, err := zet.ListZettels(notesPath)
	if err != nil {
		return false
	}

	for _, z := range zettels {
		results := zet.SearchInFile(z.Path, searchTerm)
		if len(results) > 0 {
			return true
		}
	}
	return false
}

type zettelItem struct {
	zettel        Zettel
	verbose       bool
	searchResults []SearchResult
}

func (i zettelItem) FilterValue() string {
	return i.zettel.ID + " " + i.zettel.Title
}

func (i zettelItem) renderWithSelection(isSelected bool) string {
	var parts []string

	if len(i.searchResults) > 0 {
		matchCount := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true).
			Render(fmt.Sprintf("[%d] ", len(i.searchResults)))
		parts = append(parts, matchCount)
	}

	if i.verbose {
		parts = append(parts, colors.zettelIDStyle.Render(fmt.Sprintf("%-14s", i.zettel.ID)))
	}

	if isSelected {
		indicator := colors.selectorStyle.
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
		parts = append(parts, colors.zettelIDStyle.Render(fmt.Sprintf("%-14s", i.zettel.ID)))
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

type zettelModel struct {
	list                   list.Model
	zettels                []Zettel
	zetDir                 string
	projectName            string
	quitting               bool
	backToProjects         bool
	editor                 string
	verbose                bool
	watcher                *fsnotify.Watcher
	sortNewestFirst        bool
	showDeleteConfirm      bool
	deleteZettel           *Zettel
	deleteConfirmSelection int
	filtering              bool
	filterMode             string
	customFilter           string
	allItems               []list.Item
	searchResults          map[string][]SearchResult
}

func (m zettelModel) Init() tea.Cmd {
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

func (m zettelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

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
					title, _ := zet.GetZettelTitle(m.zetDir, m.deleteZettel.ID)
					err := zet.DeleteZettel(m.zetDir, m.deleteZettel.ID)
					if err != nil {
						log.Printf("Error deleting zettel: %v", err)
					}
					if title != "" {
						zet.GitDeleteZettel(m.zetDir, m.deleteZettel.ID, title)
					}
					m.showDeleteConfirm = false
					m.deleteZettel = nil
					m.deleteConfirmSelection = 0

					zettels, err := zet.ListZettels(m.zetDir)
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
					m.showDeleteConfirm = false
					m.deleteZettel = nil
					m.deleteConfirmSelection = 0
					return m, nil
				}
			}
			return m, nil
		}

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
					m.applyFilter()
				} else {
					m.filtering = false
					m.list.SetItems(m.allItems)
				}
				return m, nil
			default:
				if len(msg.Runes) > 0 && msg.Runes[0] >= 32 && msg.Runes[0] <= 126 {
					m.customFilter += string(msg.Runes[0])
					m.applyFilter()
					return m, nil
				}
				return m, nil
			}
		}

		if msg.String() == "esc" {
			if m.customFilter != "" {
				m.customFilter = ""
				m.list.SetItems(m.allItems)
				return m, nil
			}
			// If no filter active, go back to projects
			m.backToProjects = true
			m.quitting = true
			return m, tea.Quit
		}

		if msg.String() == "q" {
			m.backToProjects = true
			m.quitting = true
			return m, tea.Quit
		}

		if msg.String() == "/" {
			if m.projectName != "" {
				// If in a project, treat as title search
				m.filtering = true
				m.customFilter = ""
				m.filterMode = "title"
			} else {
				// If in project list, filter projects
				m.filtering = true
				m.customFilter = ""
				m.filterMode = "project"
			}
			return m, nil
		}

		if msg.String() == "*" {
			m.filtering = true
			m.customFilter = ""
			m.filterMode = "fulltext"
			return m, nil
		}

		if msg.String() == "s" {
			m.sortNewestFirst = !m.sortNewestFirst
			m.sortZettels()
			return m, nil
		}

		if msg.String() == "n" {
			return m, newZettelCmd(m.zetDir, m.editor)
		}

		if msg.String() == "l" {
			return m, editLastZettelCmd(m.zetDir, m.editor)
		}

		if msg.String() == "c" {
			return m, nil
		}

		if msg.String() == "shift+t" || msg.String() == "T" {
			tocPath := filepath.Join(m.zetDir, "README.md")
			return m, openEditorCmd(m.editor, tocPath, "")
		}

		if msg.String() == "d" {
			if i, ok := m.list.SelectedItem().(zettelItem); ok {
				m.showDeleteConfirm = true
				m.deleteZettel = &i.zettel
				m.deleteConfirmSelection = 0
				return m, nil
			}
		}

		if msg.String() == "enter" {
			if i, ok := m.list.SelectedItem().(zettelItem); ok {
				searchTerm := ""
				if m.filterMode == "fulltext" && m.customFilter != "" {
					searchTerm = m.customFilter
				}
				return m, openEditorCmd(m.editor, i.zettel.Path, searchTerm)
			}
		}
	case fileChangedMsg:
		zettels, err := zet.ListZettels(m.zetDir)
		if err == nil {
			m.zettels = zettels
			items := make([]list.Item, len(m.zettels))
			for i, z := range m.zettels {
				items[i] = zettelItem{zettel: z, verbose: m.verbose}
			}
			m.allItems = items
			if m.customFilter != "" {
				m.applyFilter()
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
		zettels, err := zet.ListZettels(m.zetDir)
		if err == nil {
			m.zettels = zettels
			items := make([]list.Item, len(m.zettels))
			for i, z := range m.zettels {
				items[i] = zettelItem{zettel: z, verbose: m.verbose}
			}
			m.allItems = items
			if m.customFilter != "" {
				m.applyFilter()
			} else {
				m.list.SetItems(items)
			}
			m.list.ResetSelected()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 5)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m zettelModel) View() string {
	if m.quitting {
		return ""
	}
	view := m.list.View()

	lines := strings.Split(view, "\n")
	if len(lines) > 0 {
		header := []string{lines[0]}

		var filterLine string
		if m.filtering || m.customFilter != "" {
			var filterText string
			modeLabel := "Search by title"
			if m.filterMode == "fulltext" {
				modeLabel = "Fulltext search"
			} else if m.filterMode == "project" {
				modeLabel = "Search projects"
			}
			if m.filtering {
				filterText = fmt.Sprintf("%s: %s▓", modeLabel, m.customFilter)
			} else {
				filterText = fmt.Sprintf("%s: %s", modeLabel, m.customFilter)
			}
			filterLine = colors.filterStyle.
				Padding(0, 1).
				Render(filterText)
		} else {
			filterLine = ""
		}
		header = append(header, filterLine)

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
				paginationInfo := colors.navStyle.
					Render(fmt.Sprintf("Showing %d-%d of %d • Page %d/%d",
						startIdx+1, endIdx, totalItems, currentPage+1, totalPages))
				header = append(header, paginationInfo)
			}
		}

		result := header
		result = append(result, lines[1:]...)
		view = strings.Join(result, "\n")
	}

	lines = strings.Split(view, "\n")
	if len(lines) > 0 {
		lines = lines[:len(lines)-1]

		commandStyle := colors.projectStyle
		navStyle := colors.navStyle

		line1 := commandStyle.Render(" Commands: n:new • l:last • d:delete • T:toc • c:count")
		line2 := navStyle.Render(" ↑↓/jk • g/G:top/bottom • Ctrl+d/u:page • /:project filter • *:fulltext search • s:sort order • Enter:edit • q/esc:back • ?:more help ")

		lines = append(lines, line1, line2, "")
		view = strings.Join(lines, "\n")
	}

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

		zettelInfo := fmt.Sprintf(" %s\n", m.deleteZettel.Title)

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
				colors.navStyle.Render("← → or h l to select • Enter to confirm • Esc to cancel"),
		)

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

func (m *zettelModel) applyFilter() {
	if m.customFilter == "" {
		m.list.SetItems(m.allItems)
		m.searchResults = nil
		return
	}

	filterLower := strings.ToLower(m.customFilter)
	var filteredItems []list.Item

	if m.filterMode == "fulltext" {
		m.searchResults = make(map[string][]SearchResult)
	}

	if m.filterMode == "project" {
		for _, item := range m.allItems {
			if prjItem, ok := item.(projectItem); ok {
				if strings.Contains(strings.ToLower(prjItem.project.Name), filterLower) {
					filteredItems = append(filteredItems, item)
				}
			}
		}
	} else {
		for _, item := range m.allItems {
			if zetItem, ok := item.(zettelItem); ok {
				if m.filterMode == "title" {
					if strings.Contains(strings.ToLower(zetItem.zettel.ID), filterLower) ||
						strings.Contains(strings.ToLower(zetItem.zettel.Title), filterLower) {
						filteredItems = append(filteredItems, item)
					}
				} else if m.filterMode == "fulltext" {
					results := m.searchInFile(zetItem.zettel.Path, filterLower)
					if len(results) > 0 {
						m.searchResults[zetItem.zettel.ID] = results
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
	}

	m.list.SetItems(filteredItems)
}

func (m *zettelModel) searchInFile(filePath, searchTerm string) []SearchResult {
	return zet.SearchInFile(filePath, searchTerm)
}

func (m *zettelModel) sortZettels() {
	sort.Slice(m.zettels, func(i, j int) bool {
		if m.sortNewestFirst {
			return m.zettels[i].ID > m.zettels[j].ID
		}
		return m.zettels[i].ID < m.zettels[j].ID
	})

	if m.sortNewestFirst {
		m.list.Title = fmt.Sprintf("Notes: %s (Newest First)", m.projectName)
	} else {
		m.list.Title = fmt.Sprintf("Notes: %s (Oldest First)", m.projectName)
	}

	items := make([]list.Item, len(m.zettels))
	for i, z := range m.zettels {
		items[i] = zettelItem{zettel: z, verbose: m.verbose}
	}
	m.allItems = items

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

	if err := zet.CreateZettel(zetDir, zetID, ""); err != nil {
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
		zet.UpdateReadme(zetDir)

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

	if searchTerm != "" {
		editorName := filepath.Base(editorCmd)
		switch editorName {
		case "vim", "nvim", "vi":
			editorArgs = append(editorArgs, fmt.Sprintf("+/%s", searchTerm))
		case "emacs":
			editorArgs = append(editorArgs, "--eval", fmt.Sprintf("(progn (goto-char (point-min)) (search-forward \"%s\" nil t))", searchTerm))
		}
	}

	editorArgs = append(editorArgs, filePath)

	c := exec.Command(editorCmd, editorArgs...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		dir := filepath.Dir(filePath)
		zetID := filepath.Base(dir)
		zetDir := filepath.Dir(dir)

		if zet.IsValidZettelID(zetID) {
			zet.UpdateReadme(zetDir)
			title, _ := zet.GetZettelTitle(zetDir, zetID)
			if title != "" {
				zet.GitCommit(zetDir, zetID, title)
			}
		}

		return editorFinishedMsg{err: err}
	})
}

func listProjects(prjDir string, cfg *config.Config) ([]Project, error) {
	entries, err := os.ReadDir(prjDir)
	if err != nil {
		return nil, err
	}

	var projects []Project
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			prjPath := filepath.Join(prjDir, entry.Name())
			notesPath := filepath.Join(prjPath, "notes")
			hasNotes := false
			if _, err := os.Stat(notesPath); err == nil {
				hasNotes = true
			}

			hasTodos := false
			if cfg != nil {
				tasks, err := task.ListTasks(cfg, entry.Name(), false)
				if err == nil && len(tasks) > 0 {
					hasTodos = true
				}
			}

			// Count notes
			noteCount := 0
			if hasNotes {
				notes, err := zet.ListZettels(notesPath)
				if err == nil {
					noteCount = len(notes)
				}
			}

			// Count todos
			todoCount := 0
			if hasTodos {
				tasks, err := task.ListTasks(cfg, entry.Name(), false)
				if err == nil {
					todoCount = len(tasks)
				}
			}

			projects = append(projects, Project{
				Name:      entry.Name(),
				Path:      prjPath,
				HasNotes:  hasNotes,
				HasTodos:  hasTodos,
				NoteCount: noteCount,
				TodoCount: todoCount,
			})
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

func checkProjectDir(prjDir, prjName string) (bool, error) {
	prjPath := filepath.Join(prjDir, prjName)
	if _, err := os.Stat(prjPath); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func checkNotesDir(prjDir, prjName string) (bool, error) {
	notesPath := filepath.Join(prjDir, prjName, "notes")
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

func createProjectDir(prjDir, prjName string) error {
	prjPath := filepath.Join(prjDir, prjName)
	return os.MkdirAll(prjPath, 0755)
}

func createNotesDir(prjDir, prjName string) error {
	notesPath := filepath.Join(prjDir, prjName, "notes")
	if err := os.MkdirAll(notesPath, 0755); err != nil {
		return err
	}

	return zet.GitInit(notesPath)
}

func getNotesDir(prjDir, prjName string) string {
	return filepath.Join(prjDir, prjName, "notes")
}

func showProjectList(prjDir, editor string, verbose bool, cfg *config.Config) {
	var savedFilterMode string
	var savedFilter string
	
	for {
		projects, err := listProjects(prjDir, cfg)
		if err != nil {
			log.Fatal(err)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found")
			return
		}

		// Convert projects to items
		items := make([]list.Item, len(projects))
		for i, prj := range projects {
			items[i] = projectItem{project: prj}
		}

		// Create delegate
		delegate := projectDelegate{DefaultDelegate: list.NewDefaultDelegate()}
		delegate.ShowDescription = true // Show description for stats
		delegate.SetHeight(3) // Three lines per item (name, stats, blank)
		delegate.SetSpacing(0) // No extra spacing since we're adding blank line manually

		// Create list
		l := list.New(items, delegate, 0, 0)
		l.Title = "Projects"
		l.SetShowTitle(true)
		l.SetShowStatusBar(true) // Show status bar for pagination info
		l.SetFilteringEnabled(false)
		l.KeyMap.Quit.SetKeys("q")
		l.KeyMap.ForceQuit.SetKeys("ctrl+c")
		
		// Set initial height
		l.SetHeight(15) // Will be updated on window resize

		// Match vim-style keybindings
		l.KeyMap.NextPage.SetKeys("pgdown", "ctrl+f", "ctrl+d")
		l.KeyMap.PrevPage.SetKeys("pgup", "ctrl+b", "ctrl+u")
		l.KeyMap.GoToStart.SetKeys("g")  // Jump to top
		l.KeyMap.GoToEnd.SetKeys("G")    // Jump to bottom

		// Add help keys
		l.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "open"),
				),
				key.NewBinding(
					key.WithKeys("/"),
					key.WithHelp("/", "filter"),
				),
				key.NewBinding(
					key.WithKeys("*"),
					key.WithHelp("*", "search"),
				),
				key.NewBinding(
					key.WithKeys("j"),
					key.WithHelp("j", "down"),
				),
				key.NewBinding(
					key.WithKeys("k"),
					key.WithHelp("k", "up"),
				),
			}
		}

		l.AdditionalFullHelpKeys = func() []key.Binding {
			return []key.Binding{
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "open selected project"),
				),
				key.NewBinding(
					key.WithKeys("/"),
					key.WithHelp("/", "filter projects"),
				),
				key.NewBinding(
					key.WithKeys("*"),
					key.WithHelp("*", "fulltext search across all notes"),
				),
				key.NewBinding(
					key.WithKeys("j", "down"),
					key.WithHelp("j/↓", "down"),
				),
				key.NewBinding(
					key.WithKeys("k", "up"),
					key.WithHelp("k/↑", "up"),
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
					key.WithHelp("ctrl+d", "page down"),
				),
				key.NewBinding(
					key.WithKeys("ctrl+u"),
					key.WithHelp("ctrl+u", "page up"),
				),
				key.NewBinding(
					key.WithKeys("pgdown"),
					key.WithHelp("pgdn", "page down"),
				),
				key.NewBinding(
					key.WithKeys("pgup"),
					key.WithHelp("pgup", "page up"),
				),
				key.NewBinding(
					key.WithKeys("q"),
					key.WithHelp("q", "quit"),
				),
			}
		}

		m := projectModel{
			list:          l,
			projects:      projects,
			prjDir:        prjDir,
			editor:        editor,
			verbose:       verbose,
			launchProject: "",
			cfg:           cfg,
			allItems:      items,
			filterMode:    savedFilterMode,
			customFilter:  savedFilter,
		}
		
		// Restore filter if it was active
		if savedFilter != "" {
			m.applyProjectFilter()
		}

		p := tea.NewProgram(m, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			log.Fatal(err)
		}

		if finalModel, ok := finalModel.(projectModel); ok && finalModel.launchProject != "" {
			// Save filter state before launching project
			savedFilterMode = finalModel.filterMode
			savedFilter = finalModel.customFilter
			
			projectName := finalModel.launchProject

			exists, err := checkProjectDir(prjDir, projectName)
			if err != nil {
				log.Fatal(err)
			}
			if !exists {
				fmt.Printf("Project directory - '%s', doesn't exist. Create [Y/n]? ", filepath.Join(prjDir, projectName))
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input == "Y" || input == "" {
					if err := createProjectDir(prjDir, projectName); err != nil {
						log.Fatal(err)
					}
				} else {
					continue // Go back to project list instead of exiting
				}
			}

			exists, err = checkNotesDir(prjDir, projectName)
			if err != nil {
				log.Fatal(err)
			}
			if !exists {
				fmt.Printf("Notes directory '%s' doesn't exist. Create (Y/n)? ", filepath.Join(prjDir, projectName, "notes"))
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				if input == "Y" || input == "" {
					if err := createNotesDir(prjDir, projectName); err != nil {
						log.Fatal(err)
					}
				} else {
					continue // Go back to project list instead of exiting
				}
			}

			zetDir := getNotesDir(prjDir, projectName)
			backToProjects := showZettelTUI(zetDir, projectName, editor, verbose)
			if !backToProjects {
				return
			}
		} else {
			return
		}
	}
}

func showZettelTUI(zetDir, projectName, editor string, verbose bool) bool {
	zettels, err := zet.ListZettels(zetDir)
	if err != nil {
		log.Fatal(err)
	}

	if len(zettels) == 0 {
		fmt.Println("No notes found")
		return false
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
	l.Title = fmt.Sprintf("Notes: %s (Newest First)", projectName)
	l.SetShowTitle(true)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.Quit.SetKeys("q")
	l.KeyMap.ForceQuit.SetKeys("ctrl+c")
	l.KeyMap.AcceptWhileFiltering.SetEnabled(false)
	l.KeyMap.CancelWhileFiltering.SetEnabled(false)
	l.KeyMap.ClearFilter.SetEnabled(false)

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
				key.WithKeys("*"),
				key.WithHelp("*", "search"),
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
				key.WithHelp("enter", "edit selected note"),
			),
			key.NewBinding(
				key.WithKeys("/"),
				key.WithHelp("/", "filter by title"),
			),
			key.NewBinding(
				key.WithKeys("*"),
				key.WithHelp("*", "fulltext search"),
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
				key.WithHelp("n", "new note"),
			),
			key.NewBinding(
				key.WithKeys("l"),
				key.WithHelp("l", "edit last note"),
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
				key.WithHelp("d", "delete note"),
			),
		}
	}

	m := zettelModel{
		list:            l,
		zettels:         zettels,
		zetDir:          zetDir,
		projectName:     projectName,
		editor:          editor,
		verbose:         verbose,
		watcher:         watcher,
		sortNewestFirst: true,
		allItems:        items,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	if watcher != nil {
		watcher.Close()
	}

	if finalModel, ok := finalModel.(zettelModel); ok {
		return finalModel.backToProjects
	}
	return false
}

func printHelp() {
	help := `note - Project-based note management using zettelkasten

USAGE:
    note [OPTIONS] [PROJECT] [COMMAND] [ARGS]

OPTIONS:
    -v, --verbose       Show note IDs in interactive mode

COMMANDS:
    (no project)        Show interactive list of projects
    mcp                 Start MCP server (stdio) for AI agent integration
    <project>           Manage notes for the specified project
    <project> count     Count total number of notes in project
    <project> n, new    Create a new note (optionally with title)
    <project> e, edit   Edit a note (interactive if no ID provided)
    <project> ls, list  List all notes with IDs and titles
    <project> show <ID> Display note content
    <project> ? <pattern> Search for pattern across all notes
    <project> t?, title? <pattern> Search for pattern in note titles
    <project> d, todo   Find all tasks across notes
    <project> last      Edit the most recently modified note
    <project> toc       Edit the table of contents (README.md)
    -h, --help, help    Show this help message

INTERACTIVE MODE (Project List):
    hjkl or ↑↓←→        Navigate projects in grid
    g / G               Jump to first / last project
    Enter               Open project notes
    q, Ctrl+C           Quit

INTERACTIVE MODE (Notes):
    j/k or ↑/↓          Navigate notes
    g / G               Jump to top / bottom
    Ctrl+d / Ctrl+u     Page down / up (vim-style)
    Ctrl+f / Ctrl+b     Page down / up (emacs-style)
    PgDn / PgUp         Page down / up
    s                   Toggle sort (newest first / oldest first)
    /                   Filter project names or note titles
    *                   Start fulltext search across all notes
    Enter               Edit selected note / Exit filter mode
    Esc                 Exit filter mode or clear filter
    q, Ctrl+C           Quit

SUBCOMMANDS (available in TUI):
    n                   Create new note
    l                   Edit last modified note
    d                   Delete selected note (with confirmation)
    T                   Edit table of contents (README.md)
    c                   Show count (visible in pagination)

EXAMPLES:
    note                          # Browse projects interactively
    note myproject                # Browse notes for myproject
    note -v myproject             # Browse with note IDs visible
    note myproject new "My Note"  # Create new note with title
    note myproject n              # Create new note (prompt for title)
    note myproject ? "golang"     # Search for "golang" in all notes
    note myproject count          # Show total note count
    note mcp                      # Start MCP server for AI agents

CONFIGURATION:
    Set projects directory in ~/.config/karya/config.toml:
    [directories]
    projects = "/path/to/projects"

ENVIRONMENT VARIABLES:
    EDITOR              Editor to use (default: vim)
`
	fmt.Print(help)
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
	fmt.Printf("Creating note: %s\n", zetID)

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
			return fmt.Errorf("no note found matching: %s", zetID)
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
		return fmt.Errorf("note not found: %s", zetID)
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

	fmt.Printf("Editing last note: %s\n", zetID)
	return editZettel(zetDir, zetID, editor)
}

func showZettel(zetDir, zetID string) error {
	zetPath := filepath.Join(zetDir, zetID, "README.md")
	if _, err := os.Stat(zetPath); os.IsNotExist(err) {
		return fmt.Errorf("note not found: %s", zetID)
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

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	InitializeColors(cfg)

	prjDir := cfg.Directories.Projects
	if prjDir == "" {
		fmt.Println("ERROR: No projects directory set.")
		fmt.Println("Please set projects directory in ~/.config/karya/config.toml")
		os.Exit(1)
	}

	if _, err := os.Stat(prjDir); os.IsNotExist(err) {
		fmt.Printf("ERROR: Projects directory does not exist: %s\n", prjDir)
		os.Exit(1)
	}

	editor := cfg.GeneralConfig.EDITOR
	if editor == "" {
		editor = "vim"
	}

	verbose := cfg.GeneralConfig.Verbose
	args := os.Args[1:]

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-v" || arg == "--verbose" {
			verbose = true
			args = append(args[:i], args[i+1:]...)
			i--
		}
	}

	if len(args) == 0 {
		showProjectList(prjDir, editor, verbose, cfg)
		return
	}

	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printHelp()
		return
	}

	if args[0] == "mcp" {
		// Start MCP server on stdio
		mcpServer := note.NewMCPServer(cfg)
		ctx := context.Background()
		if err := mcpServer.Run(ctx); err != nil {
			log.Fatal(err)
		}
		return
	}

	projectName := args[0]
	subArgs := args[1:]

	if len(subArgs) == 0 {
		exists, err := checkProjectDir(prjDir, projectName)
		if err != nil {
			log.Fatal(err)
		}
		if !exists {
			fmt.Printf("Project directory - '%s', doesn't exist. Create [Y/n]? ", filepath.Join(prjDir, projectName))
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "Y" || input == "" {
				if err := createProjectDir(prjDir, projectName); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Fatal("Cannot capture notes! Exiting.")
			}
		}

		exists, err = checkNotesDir(prjDir, projectName)
		if err != nil {
			log.Fatal(err)
		}
		if !exists {
			fmt.Printf("Notes directory '%s' doesn't exist. Create (Y/n)? ", filepath.Join(prjDir, projectName, "notes"))
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "Y" || input == "" {
				if err := createNotesDir(prjDir, projectName); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Fatal("Cannot capture notes! Exiting.")
			}
		}

		zetDir := getNotesDir(prjDir, projectName)
		showZettelTUI(zetDir, projectName, editor, verbose)
		return
	}

	exists, err := checkProjectDir(prjDir, projectName)
	if err != nil {
		log.Fatal(err)
	}
	if !exists {
		fmt.Printf("Project directory - '%s', doesn't exist. Create [Y/n]? ", filepath.Join(prjDir, projectName))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "Y" || input == "" {
			if err := createProjectDir(prjDir, projectName); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal("Cannot capture notes! Exiting.")
		}
	}

	exists, err = checkNotesDir(prjDir, projectName)
	if err != nil {
		log.Fatal(err)
	}
	if !exists {
		fmt.Printf("Notes directory '%s' doesn't exist. Create (Y/n)? ", filepath.Join(prjDir, projectName, "notes"))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "Y" || input == "" {
			if err := createNotesDir(prjDir, projectName); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal("Cannot capture notes! Exiting.")
		}
	}

	zetDir := getNotesDir(prjDir, projectName)

	subcommand := subArgs[0]
	switch subcommand {
	case "count":
		count, err := zet.CountZettels(zetDir)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(count)
	case "n", "new", "a", "add":
		title := ""
		if len(subArgs) > 1 {
			title = strings.Join(subArgs[1:], " ")
		}
		if err := newZettel(zetDir, title, editor); err != nil {
			log.Fatal(err)
		}
	case "?":
		if len(subArgs) < 2 {
			fmt.Println("ERROR: Search pattern required")
			os.Exit(1)
		}
		pattern := strings.Join(subArgs[1:], " ")
		results, err := zet.SearchZettels(zetDir, pattern)
		if err != nil {
			log.Fatal(err)
		}
		printSearchResults(results)
	case "d", "todo":
		taskCfg := cfg                 // Use the loaded config
		taskCfg.Todo.Structured = true // Always use structured mode for notes

		files, err := task.FindFiles(taskCfg, projectName)
		if err != nil {
			log.Fatal(err)
		}

		// Process each file and collect tasks
		var allTasks []*task.Task
		for _, file := range files {
			tasks, err := task.ProcessFile(taskCfg, file)
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

			// Print zettel header once
			fmt.Printf("\n%s: %s\n",
				colors.zettelIDStyle.Render(zettelID),
				colors.highlightStyle.Render(tasks[0].Project))

			// Print all tasks for this zettel
			for _, t := range tasks {
				taskLine := fmt.Sprintf("%s: %s", t.Keyword, t.Title)
				if t.Tag != "" {
					taskLine += fmt.Sprintf(" #%s", t.Tag)
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
		if len(subArgs) < 2 {
			fmt.Println("ERROR: Search pattern required")
			os.Exit(1)
		}
		pattern := strings.Join(subArgs[1:], " ")
		results, err := zet.SearchZettelTitles(zetDir, pattern)
		if err != nil {
			log.Fatal(err)
		}
		printTitleSearchResults(results)
	case "e", "edit":
		if len(subArgs) < 2 {
			showZettelTUI(zetDir, projectName, editor, verbose)
		} else {
			zetID := subArgs[1]
			if err := editZettel(zetDir, zetID, editor); err != nil {
				log.Fatal(err)
			}
		}
	case "ls", "list":
		zettels, err := zet.ListZettels(zetDir)
		if err != nil {
			log.Fatal(err)
		}
		for _, z := range zettels {
			fmt.Printf("%s %s\n", colors.zettelIDStyle.Render(z.ID), z.Title)
		}
	case "show":
		if len(subArgs) < 2 {
			fmt.Println("ERROR: Note ID required")
			os.Exit(1)
		}
		zetID := subArgs[1]
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
