package task

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"karya/internal/config"
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

// NewConfig creates a config from shared config
func NewConfig() *Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	return &Config{PRJDIR: cfg.PRJDIR}
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

	// Use concurrent processing for better performance
	numWorkers := 10 // Adjust based on system
	if len(files) < numWorkers {
		numWorkers = len(files)
	}
	if numWorkers == 0 {
		return []*Task{}, nil
	}

	// Channel for file paths
	fileChan := make(chan string, len(files))
	// Channel for results
	resultChan := make(chan []*Task, len(files))
	// WaitGroup for workers
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				tasks, err := ProcessFile(file)
				if err != nil {
					// Skip files with errors
					continue
				}
				resultChan <- tasks
			}
		}()
	}

	// Send files to workers
	go func() {
		for _, file := range files {
			fileChan <- file
		}
		close(fileChan)
	}()

	// Close result channel when all workers done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var allTasks []*Task
	for tasks := range resultChan {
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

	// Use concurrent processing
	numWorkers := 10
	if len(files) < numWorkers {
		numWorkers = len(files)
	}
	if numWorkers == 0 {
		return make(map[string]int), nil
	}

	type result struct {
		project string
		count   int
	}

	fileChan := make(chan string, len(files))
	resultChan := make(chan result, len(files))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				tasks, err := ProcessFile(file)
				if err != nil {
					continue
				}
				parts := strings.Split(file, string(filepath.Separator))
				project := parts[len(parts)-4]
				activeCount := 0
				for _, t := range tasks {
					if t.IsActive() {
						activeCount++
					}
				}
				if activeCount > 0 {
					resultChan <- result{project: project, count: activeCount}
				}
			}
		}()
	}

	// Send files to workers
	go func() {
		for _, file := range files {
			fileChan <- file
		}
		close(fileChan)
	}()

	// Close result channel when all workers done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	summary := make(map[string]int)
	for res := range resultChan {
		summary[res.project] += res.count
	}

	return summary, nil
}
