package zet

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Config holds zet configuration
type Config struct {
	ZetDir string
	Editor string
	Repo   *git.Repository
}

// Zettel represents a zettel
type Zettel struct {
	ID    string
	Title string
	Path  string
}

// NewConfig creates a new config
func NewConfig() (*Config, error) {
	zetDir := os.Getenv("ZETDIR")
	if zetDir == "" {
		return nil, fmt.Errorf("ZETDIR not set")
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // default
	}

	repo, err := git.PlainOpen(zetDir)
	if err != nil {
		return nil, err // assume git repo
	}

	return &Config{
		ZetDir: zetDir,
		Editor: editor,
		Repo:   repo,
	}, nil
}

// CountZettels counts the number of zettels
func CountZettels(config *Config) (int, error) {
	entries, err := os.ReadDir(config.ZetDir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) == 14 {
			count++
		}
	}
	return count, nil
}

// NewZettel creates a new zettel
func NewZettel(config *Config, title string) error {
	timestamp := time.Now().Format("20060102150405")
	zetPath := filepath.Join(config.ZetDir, timestamp)
	if err := os.MkdirAll(zetPath, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(zetPath, "README.md")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if title == "" {
		fmt.Print("Title: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			title = scanner.Text()
		}
	}

	fmt.Fprintf(file, "# %s\n\n\n", title)

	// Edit
	cmd := exec.Command(config.Editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Update README and commit
	if err := updateReadme(config); err != nil {
		log.Println("Failed to update README:", err)
	}
	return commitChanges(config, fmt.Sprintf("New zettel: %s", title), []string{filePath, filepath.Join(config.ZetDir, "README.md")})
}

// EditZettel edits a zettel
func EditZettel(config *Config, id string) error {
	zetPath := filepath.Join(config.ZetDir, id, "README.md")
	if _, err := os.Stat(zetPath); os.IsNotExist(err) {
		return fmt.Errorf("Zettel not found: %s", id)
	}

	// Read title
	title, err := getTitle(zetPath)
	if err != nil {
		return err
	}

	cmd := exec.Command(config.Editor, zetPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return commitChanges(config, title, []string{zetPath})
}

// ShowZettel shows a zettel with glamour
func ShowZettel(config *Config, id string) error {
	zetPath := filepath.Join(config.ZetDir, id, "README.md")
	content, err := os.ReadFile(zetPath)
	if err != nil {
		return err
	}

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	out, err := r.Render(string(content))
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// ListZettels lists all zettels
func ListZettels(config *Config, showCount string) error {
	zettels, err := getZettels(config)
	if err != nil {
		return err
	}

	magenta := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	normal := lipgloss.NewStyle()

	for _, z := range zettels {
		fmt.Printf("%s%s%s %s\n", magenta.Render(z.ID), normal.Render(""), normal.Render(z.Title))
	}

	if showCount == "-c" {
		fmt.Printf("\n%d zettels\n", len(zettels))
	}
	return nil
}

// InteractiveMode runs the TUI for selection
func InteractiveMode(config *Config) error {
	zettels, err := getZettels(config)
	if err != nil {
		return err
	}

	items := make([]list.Item, len(zettels))
	for i, z := range zettels {
		items[i] = z
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Zettel"

	p := tea.NewProgram(model{list: l, config: config})
	_, err = p.Run()
	return err
}

type model struct {
	list   list.Model
	config *Config
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if i, ok := m.list.SelectedItem().(Zettel); ok {
				// Edit
				if err := EditZettel(m.config, i.ID); err != nil {
					log.Println(err)
				}
				return m, tea.Quit
			}
		}
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return m.list.View()
}

// getZettels gets all zettels
func getZettels(config *Config) ([]Zettel, error) {
	entries, err := os.ReadDir(config.ZetDir)
	if err != nil {
		return nil, err
	}
	var zettels []Zettel
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) == 14 {
			id := entry.Name()
			title, _ := getTitle(filepath.Join(config.ZetDir, id, "README.md"))
			zettels = append(zettels, Zettel{ID: id, Title: title, Path: filepath.Join(config.ZetDir, id)})
		}
	}
	sort.Slice(zettels, func(i, j int) bool {
		return zettels[i].ID > zettels[j].ID // newest first
	})
	return zettels, nil
}

// getTitle gets the title from a zettel file
func getTitle(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# "), nil
		}
	}
	return "", nil
}

// updateReadme updates the README.md index
func updateReadme(config *Config) error {
	zettels, err := getZettels(config)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(config.ZetDir, "README.md"))
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintln(file, "# Index")
	fmt.Fprintln(file, "")
	for _, z := range zettels {
		fmt.Fprintf(file, "* [%s](./%s/README.md) - %s\n", z.ID, z.ID, z.Title)
	}
	return nil
}

// commitChanges commits changes
func commitChanges(config *Config, message string, files []string) error {
	w, err := config.Repo.Worktree()
	if err != nil {
		return err
	}

	for _, file := range files {
		relPath, _ := filepath.Rel(config.ZetDir, file)
		_, err := w.Add(relPath)
		if err != nil {
			return err
		}
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Zet",
			Email: "zet@localhost",
			When:  time.Now(),
		},
	})
	return err
}

// EditLastZettel edits the last committed zettel
func EditLastZettel(config *Config) error {
	ref, err := config.Repo.Head()
	if err != nil {
		return err
	}
	commit, err := config.Repo.CommitObject(ref.Hash())
	if err != nil {
		return err
	}
	files, err := commit.Files()
	if err != nil {
		return err
	}

	var lastZettel string
	err = files.ForEach(func(f *object.File) error {
		if strings.Contains(f.Name, "/README.md") {
			parts := strings.Split(f.Name, "/")
			if len(parts) > 1 {
				id := parts[0]
				if len(id) == 14 {
					if id > lastZettel {
						lastZettel = id
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if lastZettel == "" {
		return fmt.Errorf("No zettel found")
	}
	return EditZettel(config, lastZettel)
}

// EditTOC edits the TOC
func EditTOC(config *Config) error {
	tocPath := filepath.Join(config.ZetDir, "README.md")
	cmd := exec.Command(config.Editor, tocPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SearchZettels searches content
func SearchZettels(config *Config, query []string) error {
	pattern := strings.Join(query, " ")
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
	if err != nil {
		return err
	}

	zettels, err := getZettels(config)
	if err != nil {
		return err
	}

	magenta := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	normal := lipgloss.NewStyle()

	for _, z := range zettels {
		filePath := filepath.Join(z.Path, "README.md")
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				fmt.Printf("\n%s%s: %s%s\n", magenta.Render(z.ID), normal.Render(""), normal.Render(z.Title), normal.Render(""))
				fmt.Printf("[%d]: %s\n", i+1, line)
			}
		}
	}
	return nil
}

// SearchTitles searches titles
func SearchTitles(config *Config, query []string) error {
	pattern := strings.Join(query, " ")
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
	if err != nil {
		return err
	}

	zettels, err := getZettels(config)
	if err != nil {
		return err
	}

	magenta := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	normal := lipgloss.NewStyle()

	for _, z := range zettels {
		if re.MatchString(z.Title) {
			fmt.Printf("%s%s: %s\n", magenta.Render(z.ID), normal.Render(""), z.Title)
		}
	}
	return nil
}

// SearchTodos searches for TODOs
func SearchTodos(config *Config, query []string) error {
	re := regexp.MustCompile(`- \[ \]`)

	zettels, err := getZettels(config)
	if err != nil {
		return err
	}

	for _, z := range zettels {
		filePath := filepath.Join(z.Path, "README.md")
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				fmt.Printf("%s/%s/README.md:%d: %s\n", z.ID, z.ID, i+1, line)
			}
		}
	}
	return nil
}
