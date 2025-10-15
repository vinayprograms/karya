package task

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Task represents a parsed task from a line
type Task struct {
	Keyword  string
	Title    string
	Tag      string
	Date     string
	Assignee string
	Project  string
	Zettel   string
}

// IsActive returns true if the task is active (not completed)
func (t *Task) IsActive() bool {
	activeKeywords := []string{"TODO", "TASK", "NOTE", "REMINDER", "EVENT", "MEETING", "CALL", "EMAIL", "MESSAGE", "FOLLOWUP", "REVIEW", "CHECKIN", "CHECKOUT", "RESEARCH", "READING", "WRITING", "DRAFT", "EDITING", "FINALIZE", "SUBMIT", "PRESENTATION", "WAITING", "DEFERRED", "DELEGATED"}
	for _, kw := range activeKeywords {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// IsCompleted returns true if the task is completed
func (t *Task) IsCompleted() bool {
	completedKeywords := []string{"ARCHIVED", "CANCELED", "DELETED", "DONE", "COMPLETED", "CLOSED"}
	for _, kw := range completedKeywords {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// Config holds configuration
type Config struct {
	PRJDIR string
}

// NewConfig creates a config from env, loading ~/.gtdrc if needed
func NewConfig() *Config {
	prjdir := os.Getenv("PRJDIR")
	if prjdir == "" {
		loadGtdrc()
		prjdir = os.Getenv("PRJDIR")
		if prjdir == "" {
			fmt.Fprintln(os.Stderr, "PRJDIR not set. Please create ~/.gtdrc with 'export PRJDIR=/path/to/projects'")
			os.Exit(1)
		}
	}
	return &Config{PRJDIR: prjdir}
}

// loadGtdrc loads config from ~/.gtdrc
func loadGtdrc() {
	home, _ := os.UserHomeDir()
	gtdrc := filepath.Join(home, ".gtdrc")
	if _, err := os.Stat(gtdrc); os.IsNotExist(err) {
		return
	}
	file, err := os.Open(gtdrc)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimPrefix(line, "export ")
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				os.Setenv(key, value)
			}
		}
	}
}

// FindFiles finds README.md files in project directories
func (c *Config) FindFiles(project string) ([]string, error) {
	pattern := filepath.Join(c.PRJDIR, project, "notes", "??????????????", "README.md")
	if project == "" || project == "*" {
		pattern = filepath.Join(c.PRJDIR, "*", "notes", "??????????????", "README.md")
	}
	matches, err := filepath.Glob(pattern)
	return matches, err
}

// ProcessFile processes a README.md file and returns tasks
func ProcessFile(filePath string) ([]*Task, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Extract project and zettel from path
	// Path: PRJDIR/project/notes/zet/README.md
	parts := strings.Split(filePath, string(filepath.Separator))
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid path: %s", filePath)
	}
	zettel := parts[len(parts)-2]
	project := parts[len(parts)-4]

	var tasks []*Task
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		task := ParseLine(line, project, zettel)
		if task != nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, scanner.Err()
}

// ParseLine parses a task line and returns a Task
func ParseLine(line, project, zettel string) *Task {
	// Regex to match: ^[A-Z]+: .+( #[^ ]+)?( @[^ ]+)?( >> [^ ]+)?$
	re := regexp.MustCompile(`^([A-Z]+):\s*(.+?)(?:\s*#([^ ]+))?(?:\s*@([^ ]+))?(?:\s*>>\s*([^ ]+))?$`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 0 {
		return nil
	}

	keyword := matches[1]
	// Check if keyword is valid
	if !isValidKeyword(keyword) {
		return nil
	}

	title := matches[2]
	tag := ""
	if len(matches) > 3 && matches[3] != "" {
		tag = matches[3]
	}
	date := ""
	if len(matches) > 4 && matches[4] != "" {
		date = matches[4]
	}
	assignee := ""
	if len(matches) > 5 && matches[5] != "" {
		assignee = matches[5]
	}

	return &Task{
		Keyword:  keyword,
		Title:    strings.TrimSpace(title),
		Tag:      tag,
		Date:     date,
		Assignee: assignee,
		Project:  project,
		Zettel:   zettel,
	}
}

func isValidKeyword(keyword string) bool {
	active := []string{"TODO", "TASK", "NOTE", "REMINDER", "EVENT", "MEETING", "CALL", "EMAIL", "MESSAGE", "FOLLOWUP", "REVIEW", "CHECKIN", "CHECKOUT", "RESEARCH", "READING", "WRITING", "DRAFT", "EDITING", "FINALIZE", "SUBMIT", "PRESENTATION", "WAITING", "DEFERRED", "DELEGATED"}
	completed := []string{"ARCHIVED", "CANCELED", "DELETED", "DONE", "COMPLETED", "CLOSED"}
	for _, kw := range append(active, completed...) {
		if keyword == kw {
			return true
		}
	}
	return false
}

// ListTasks lists tasks for a project, filtering by showPending
func (c *Config) ListTasks(project string, showPending bool) ([]*Task, error) {
	files, err := c.FindFiles(project)
	if err != nil {
		return nil, err
	}
	var allTasks []*Task
	for _, file := range files {
		tasks, err := ProcessFile(file)
		if err != nil {
			return nil, err
		}
		allTasks = append(allTasks, tasks...)
	}
	if showPending {
		var filtered []*Task
		for _, t := range allTasks {
			if t.IsActive() {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}
	return allTasks, nil
}

// SummarizeProjects summarizes task counts per project
func (c *Config) SummarizeProjects() (map[string]int, error) {
	files, err := c.FindFiles("")
	if err != nil {
		return nil, err
	}
	summary := make(map[string]int)
	for _, file := range files {
		tasks, err := ProcessFile(file)
		if err != nil {
			return nil, err
		}
		parts := strings.Split(file, string(filepath.Separator))
		project := parts[len(parts)-4]
		activeCount := 0
		for _, t := range tasks {
			if t.IsActive() {
				activeCount++
			}
		}
		summary[project] += activeCount
	}
	return summary, nil
}
