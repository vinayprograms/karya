package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/vinayprograms/karya/internal/config"
)

func newProjectListForTest(projects []Project) projectModel {
	items := make([]list.Item, 0, len(projects))
	for _, project := range projects {
		items = append(items, projectItem{project: project})
	}
	width, nameWidth := 80, 18
	delegate := projectDelegate{DefaultDelegate: list.NewDefaultDelegate(), termWidth: &width, maxNameLen: &nameWidth}
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	l := list.New(nil, delegate, width, 19)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return projectModel{list: l, fullItems: items, sortField: "name", sortAsc: true, termWidth: &width, maxNameLen: &nameWidth}
}

func pressProjectKey(m projectModel, key string) projectModel {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if key == "enter" {
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	if key == "esc" {
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	}
	updated, _ := m.Update(msg)
	return updated.(projectModel)
}

func checkProjectOrder(t *testing.T, m projectModel, want ...string) {
	t.Helper()
	var got []string
	for _, item := range m.list.Items() {
		got = append(got, item.FilterValue())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("project list = %v, want %v", got, want)
	}
}

func TestProjectListSortKeys(t *testing.T) {
	m := newProjectListForTest([]Project{
		{Name: "a", NoteCount: 1, LastNoteID: "03"},
		{Name: "b", NoteCount: 3, TodoCount: 1, LastNoteID: "02"},
		{Name: "c", NoteCount: 1, TodoCount: 2, LastNoteID: "01"},
		{Name: "d", NoteCount: 3, LastNoteID: "01"},
	})
	m.applyViewFilter(true)
	checkProjectOrder(t, m, "b", "c", "a", "d")
	for _, step := range []struct {
		key  string
		want []string
	}{
		{"S", []string{"c", "b", "d", "a"}},
		{"s", []string{"b", "c", "d", "a"}},
		{"S", []string{"c", "b", "a", "d"}},
		{"s", []string{"c", "b", "a", "d"}},
		{"S", []string{"b", "c", "a", "d"}},
		{"s", []string{"b", "c", "a", "d"}},
		{"S", []string{"c", "b", "d", "a"}},
		{"s", []string{"b", "c", "a", "d"}},
	} {
		m = pressProjectKey(m, step.key)
		checkProjectOrder(t, m, step.want...)
	}
}

func TestProjectListVisibilityFallback(t *testing.T) {
	m := newProjectListForTest([]Project{{Name: "a"}, {Name: "b", TodoCount: 1}})
	m.applyViewFilter(true)
	checkProjectOrder(t, m, "b", "a")
	m = pressProjectKey(m, "a")
	checkProjectOrder(t, m, "b", "a")
	m = pressProjectKey(m, "a")
	checkProjectOrder(t, m)
}

func TestProjectListReload(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Todo: config.Todo{Active: []string{"TODO"}, Completed: []string{"DONE"}}}
	cfg.Directories.Projects = dir
	writeProjectFile := func(project, file, content string) {
		t.Helper()
		path := filepath.Join(dir, project, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	const notePath = "notes/20240101000000/README.md"
	writeProjectFile("aaa", notePath, "# First note\n\nDONE: Completed\n\nShared text\n")
	writeProjectFile("bbb", "README.md", "# No notes\n\nDONE: Completed\n")
	writeProjectFile("zzz", notePath, "# Last note\n\nTODO: Outstanding\n\nShared text\n")
	projects, err := listProjects(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := newProjectListForTest(projects)
	m.prjDir, m.cfg = dir, cfg
	m.applyViewFilter(true)
	checkProjectOrder(t, m, "zzz", "aaa")
	if got := m.list.SelectedItem().FilterValue(); got != "zzz" {
		t.Fatalf("initial selection = %q, want zzz", got)
	}
	m = pressProjectKey(m, "*")
	for _, r := range "shared" {
		m = pressProjectKey(m, string(r))
	}
	m = pressProjectKey(m, "enter")
	checkProjectOrder(t, m, "aaa First note", "zzz Last note")
	if view := m.View(); strings.Contains(view, "  "+colors.grayStyle.Render(strings.Repeat("─", 12))) {
		t.Fatalf("divider in fulltext results: %q", view)
	}
	m = pressProjectKey(m, "esc")
	checkProjectOrder(t, m, "zzz", "aaa")
	m = pressProjectKey(m, "S")
	m = pressProjectKey(m, "a")
	checkProjectOrder(t, m, "zzz", "bbb", "aaa")
	m = pressProjectKey(m, "a")
	checkProjectOrder(t, m, "zzz", "aaa")
	m = pressProjectKey(m, "a")
	writeProjectFile("bbb", "README.md", "# No notes\n\nTODO: Outstanding\n")
	writeProjectFile("aaa", notePath, "# First note\n\nTODO: Outstanding\n")
	writeProjectFile("zzz", notePath, "# Last note\n\nDONE: Completed\n")
	updated, _ := m.Update(fileChangedMsg{})
	m = updated.(projectModel)
	checkProjectOrder(t, m, "bbb", "aaa", "zzz")
	m = pressProjectKey(m, "a")
	checkProjectOrder(t, m, "aaa", "zzz")
}

func TestProjectListRefresh(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Todo: config.Todo{Active: []string{"TODO"}, Completed: []string{"DONE"}}}
	cfg.Directories.Projects = dir
	const notePath = "notes/20240101000000/README.md"
	for _, name := range []string{"report-a", "report-z", "report-docs", "other"} {
		path := filepath.Join(dir, name, notePath)
		if name == "report-docs" {
			path = filepath.Join(dir, name, "README.md")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# Example\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	projects, err := listProjects(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := newProjectListForTest(projects)
	m.prjDir, m.cfg = dir, cfg
	m.applyViewFilter(true)
	m = pressProjectKey(m, "a")
	m = pressProjectKey(m, "S")
	if err := os.WriteFile(filepath.Join(dir, "report-a", notePath), []byte("# Example\n\nTODO: Outstanding\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = pressProjectKey(m, "/")
	for _, r := range "report" {
		m = pressProjectKey(m, string(r))
	}
	m = pressProjectKey(m, "enter")
	checkProjectOrder(t, m, "report-z", "report-docs", "report-a")
	m.list.Select(2)
	if selected := m.list.SelectedItem().(projectItem).project; selected.TodoCount != 0 {
		t.Fatalf("todo count before refresh = %d, want 0", selected.TodoCount)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		t.Fatal("manual refresh returned a command")
	}
	m = updated.(projectModel)
	checkProjectOrder(t, m, "report-a", "report-z", "report-docs")
	if selected := m.list.SelectedItem().(projectItem).project; selected.Name != "report-a" || selected.TodoCount != 1 {
		t.Fatalf("selected project after refresh = %q (%d todos), want report-a (1 todo)", selected.Name, selected.TodoCount)
	}
	if !m.showAll || m.sortField != "name" || m.sortAsc || m.customFilter != "report" || m.filterMode != "project" || m.filtering {
		t.Fatal("manual refresh changed the view settings")
	}
}

func TestProjectListRefreshFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "without-notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	projects, err := listProjects(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	m := newProjectListForTest(projects)
	m.prjDir = dir
	m.applyViewFilter(true)
	m = pressProjectKey(m, "r")
	checkProjectOrder(t, m, "without-notes")
	m = pressProjectKey(m, "a")
	m = pressProjectKey(m, "a")
	m = pressProjectKey(m, "r")
	checkProjectOrder(t, m)
}

func TestInactiveProjectRow(t *testing.T) {
	previous := colors
	t.Cleanup(func() { colors = previous })
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.ANSI)
	colors.projectStyle = renderer.NewStyle().Foreground(lipgloss.Color("2"))
	colors.grayStyle = renderer.NewStyle().Foreground(lipgloss.Color("8"))
	colors.highlightStyle = renderer.NewStyle().Foreground(lipgloss.Color("3"))
	colors.selectorStyle = renderer.NewStyle().Foreground(lipgloss.Color("4"))

	active := Project{Name: "namespace.active", TodoCount: 1}
	inactive := Project{Name: "namespace.inactive"}
	m := newProjectListForTest([]Project{inactive, active})
	m.applyViewFilter(true)
	m = pressProjectKey(m, "j")
	selected, ok := m.list.SelectedItem().(projectItem)
	if !ok || selected.project != inactive {
		t.Fatalf("selected project = %q, want %q", selected.project.Name, inactive.Name)
	}
	row := selected.renderWithSelection(false, 18, 80)
	if !strings.Contains(row, colors.grayStyle.Render("inactive")) || strings.Contains(row, "█") {
		t.Fatalf("unselected row not muted: %q", row)
	}
	row = selected.renderWithSelection(true, 18, 80)
	if !strings.HasPrefix(row, colors.selectorStyle.Render("█ ")) || !strings.Contains(row, colors.highlightStyle.Render("inactive")) {
		t.Fatalf("selection not visible: %q", row)
	}
	match := matchedNoteItem{projectName: inactive.Name, noteTitle: "Matched note"}
	if row := match.renderWithSelection(false, 18, 80); !strings.Contains(row, colors.projectStyle.Render("inactive")) || !strings.HasSuffix(row, "Matched note") {
		t.Fatalf("matched note row was muted or lost its title: %q", row)
	}
	m = pressProjectKey(m, "enter")
	if !m.quitting || m.launchProject != inactive.Name {
		t.Fatalf("inactive project did not open: %q", m.launchProject)
	}
}

func TestProjectListDivider(t *testing.T) {
	previous := colors
	t.Cleanup(func() { colors = previous })
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.ANSI)
	colors.grayStyle = renderer.NewStyle().Foreground(lipgloss.Color("8"))
	divider := "  " + colors.grayStyle.Render(strings.Repeat("─", 12))

	m := newProjectListForTest([]Project{
		{Name: "aaa-empty", NoteCount: 1},
		{Name: "zzz-active", NoteCount: 1, TodoCount: 1},
	})
	m.applyViewFilter(true)
	rows := strings.Split(m.View(), "\n")
	found := 0
	for i, row := range rows {
		if strings.TrimRight(row, " ") == divider {
			found++
			if i == 0 || i == len(rows)-1 || !strings.Contains(rows[i-1], "zzz-active") || !strings.Contains(rows[i+1], "aaa-empty") {
				t.Fatalf("divider not between project rows: %q", rows)
			}
		}
	}
	if found != 1 {
		t.Fatalf("got %d muted dividers, want 1: %q", found, rows)
	}
	m = pressProjectKey(m, "j")
	if got := m.list.SelectedItem().FilterValue(); got != "aaa-empty" {
		t.Fatalf("selection after crossing divider = %q, want aaa-empty", got)
	}
	for _, name := range []string{"active", "empty"} {
		m.customFilter = name
		m.applyProjectFilter()
		if view := m.View(); strings.Contains(view, divider) {
			t.Fatalf("divider with only %s projects: %q", name, view)
		}
	}
}

func TestProjectListDividerPaginationAndResize(t *testing.T) {
	var projects []Project
	for i := range 40 {
		project := Project{Name: fmt.Sprintf("project-%02d", i), NoteCount: 1}
		if i < 6 {
			project.TodoCount = 1
		}
		projects = append(projects, project)
	}
	m := newProjectListForTest(projects)
	m.applyViewFilter(true)
	divider := "  " + colors.grayStyle.Render(strings.Repeat("─", 12))
	if view := m.View(); !strings.Contains(view, divider) || lipgloss.Height(view) > 24 {
		t.Fatalf("startup view missing divider or too tall: %q", view)
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 22}, {Width: 56, Height: 18}, {Width: 9, Height: 10}, {Width: 1, Height: 1}} {
		updated, _ := m.Update(size)
		m = updated.(projectModel)
		view := m.View()
		if h, w := lipgloss.Height(view), lipgloss.Width(view); h > size.Height || w > size.Width {
			t.Fatalf("view is %dx%d in %dx%d terminal: %q", w, h, size.Width, size.Height, view)
		}
		if m.list.Paginator.PerPage > 6 && !strings.Contains(view, divider) {
			t.Fatalf("missing divider on full mixed page: %q", view)
		}
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 22})
	m = updated.(projectModel)
	size := tea.WindowSizeMsg{Width: 80, Height: 22 - (m.list.Paginator.PerPage - 6)}
	updated, _ = m.Update(size)
	m = updated.(projectModel)
	if got := m.list.Paginator.PerPage; got != 6 {
		t.Fatalf("got %d projects per page, want 6", got)
	}
	if view := m.View(); strings.Contains(view, divider) || !strings.Contains(view, "project-05") {
		t.Fatalf("unexpected divider at page end or missing active item: %q", view)
	}
	m = pressProjectKey(m, "l")
	if view := m.View(); strings.Contains(view, divider) || !strings.Contains(view, "project-06") {
		t.Fatalf("unexpected divider at page start or missing inactive item: %q", view)
	}
}
