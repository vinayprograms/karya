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
	FilePath string // Original file path where this task was found
	config   *Config
}

// IsActive returns true if the task is active (not completed)
func (t *Task) IsActive() bool {
	if t.config == nil {
		return false
	}
	for _, kw := range t.config.ActiveKeywords {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// IsInProgress returns true if the task is in progress
func (t *Task) IsInProgress() bool {
	if t.config == nil {
		return false
	}
	for _, kw := range t.config.InProgressKeywords {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// IsCompleted returns true if the task is completed
func (t *Task) IsCompleted() bool {
	if t.config == nil {
		return false
	}
	for _, kw := range t.config.CompletedKeywords {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// Config holds configuration
type Config struct {
	PRJDIR             string
	ShowCompleted      bool
	Structured         bool
	ActiveKeywords     []string
	InProgressKeywords []string
	CompletedKeywords  []string
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
	return &Config{
		PRJDIR:             cfg.PRJDIR,
		ShowCompleted:      cfg.ShowCompleted,
		Structured:         cfg.Structured,
		ActiveKeywords:     cfg.ActiveKeywords,
		InProgressKeywords: cfg.InProgressKeywords,
		CompletedKeywords:  cfg.CompletedKeywords,
	}
}

// FindFiles finds README.md files in project directories (structured mode)
// or all .md files in the project tree (unstructured mode)
func (c *Config) FindFiles(project string) ([]string, error) {
	if c.Structured {
		// Structured mode: look for specific zettelkasten directory structure
		pattern := filepath.Join(c.PRJDIR, project, "notes", "??????????????", "README.md")
		if project == "" || project == "*" {
			pattern = filepath.Join(c.PRJDIR, "*", "notes", "??????????????", "README.md")
		}
		matches, err := filepath.Glob(pattern)
		return matches, err
	} else {
		// Unstructured mode: find all .md files in project directory tree
		return c.findUnstructuredFiles(project)
	}
}

// findUnstructuredFiles walks the project directory tree to find all .md files
func (c *Config) findUnstructuredFiles(project string) ([]string, error) {
	var files []string

	// Determine the root directory to scan
	rootDir := c.PRJDIR
	if project != "" && project != "*" {
		rootDir = filepath.Join(c.PRJDIR, project)
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-.md files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// ProcessFile processes a README.md file and returns tasks
func (c *Config) ProcessFile(filePath string) ([]*Task, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Extract project and zettel from path
	var project, zettel string

	if c.Structured {
		// Structured mode: Path: PRJDIR/project/notes/zet/README.md
		parts := strings.Split(filePath, string(filepath.Separator))
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid structured path: %s", filePath)
		}
		zettel = parts[len(parts)-2]
		project = parts[len(parts)-4]
	} else {
		// Unstructured mode: derive project and zettel from file path
		relPath, err := filepath.Rel(c.PRJDIR, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get relative path: %w", err)
		}

		parts := strings.Split(relPath, string(filepath.Separator))
		if len(parts) > 0 {
			project = parts[0] // First directory is the project
		} else {
			project = "unknown"
		}

		// Check if this file follows zettelkasten structure: project/notes/zettelID/README.md
		if len(parts) >= 4 && parts[1] == "notes" && filepath.Base(filePath) == "README.md" {
			// This is a zettelkasten file, use the directory name as zettel ID
			zettel = parts[2]
		} else {
			// Regular file, use filename without extension as zettel ID
			filename := filepath.Base(filePath)
			zettel = strings.TrimSuffix(filename, filepath.Ext(filename))
		}
	}

	var tasks []*Task
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		task := c.ParseLine(line, project, zettel, filePath)
		if task != nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, scanner.Err()
}

// ParseLine parses a task line and returns a Task
func (c *Config) ParseLine(line, project, zettel, filePath string) *Task {
	// Regex to match: ^[A-Z]+: .+( #[^ ]+)?( @[^ ]+)?( >> .+)?$
	re := regexp.MustCompile(`^([A-Z]+):\s*(.+?)(?:\s*#([^ ]+))?(?:\s*@([^ ]+))?(?:\s*>>\s*(.+))?$`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 0 {
		return nil
	}

	keyword := matches[1]
	// Check if keyword is valid
	if !c.isValidKeyword(keyword) {
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
		assignee = strings.TrimSpace(matches[5])
	}

	return &Task{
		Keyword:  keyword,
		Title:    strings.TrimSpace(title),
		Tag:      tag,
		Date:     date,
		Assignee: assignee,
		Project:  project,
		Zettel:   zettel,
		FilePath: filePath,
		config:   c,
	}
}

func (c *Config) isValidKeyword(keyword string) bool {
	for _, kw := range c.ActiveKeywords {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.InProgressKeywords {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.CompletedKeywords {
		if keyword == kw {
			return true
		}
	}
	return false
}

// ListTasks lists tasks for a project, filtering by showCompleted
func (c *Config) ListTasks(project string, showCompleted bool) ([]*Task, error) {
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
				tasks, err := c.ProcessFile(file)
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

	if !showCompleted {
		// Exclude completed tasks
		var filtered []*Task
		for _, t := range allTasks {
			if !t.IsCompleted() {
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
				tasks, err := c.ProcessFile(file)
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
