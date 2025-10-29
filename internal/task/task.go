package task

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/vinayprograms/karya/internal/config"
)

// Task represents a parsed task from a line
type Task struct {
	Keyword     string
	Title       string
	Tag         string
	ScheduledAt string // @date or @s:date (scheduled date)
	DueAt       string // @d:date (due date)
	Assignee    string
	Project     string
	Zettel      string
	FilePath    string // Original file path where this task was found
}

// IsActive returns true if the task is active (not completed)
func (t *Task) IsActive(c *config.Config) bool {
	if c == nil {
		return false
	}
	for _, kw := range c.Todo.Active {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// IsInProgress returns true if the task is in progress
func (t *Task) IsInProgress(c *config.Config) bool {
	if c == nil {
		return false
	}
	for _, kw := range c.Todo.InProgress {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// IsCompleted returns true if the task is completed
func (t *Task) IsCompleted(c *config.Config) bool {
	if c == nil {
		return false
	}
	for _, kw := range c.Todo.Completed {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// IsSomeday returns true if the task is a someday/maybe task
func (t *Task) IsSomeday(c *config.Config) bool {
	if c == nil {
		return false
	}
	for _, kw := range c.Todo.Someday {
		if t.Keyword == kw {
			return true
		}
	}
	return false
}

// Priority returns the sorting priority of the task
// Lower numbers indicate higher priority
// 1 = In Progress, 2 = Active, 3 = Someday, 4 = Completed
func (t *Task) Priority(c *config.Config) int {
	if t.IsInProgress(c) {
		return 1
	}
	if t.IsActive(c) {
		return 2
	}
	if t.IsSomeday(c) {
		return 3
	}
	if t.IsCompleted(c) {
		return 4
	}
	// Unknown status gets lowest priority
	return 5
}

// FindFiles finds README.md files in project directories (structured mode)
// or all .md files in the project tree (unstructured mode)
func FindFiles(c *config.Config, project string) ([]string, error) {
	if c.Todo.Structured {
		// Structured mode: look for specific zettelkasten directory structure
		pattern := filepath.Join(c.Directories.Projects, project, "notes", "??????????????", "README.md")
		if project == "" || project == "*" {
			pattern = filepath.Join(c.Directories.Projects, "*", "notes", "??????????????", "README.md")
		}
		matches, err := filepath.Glob(pattern)
		return matches, err
	} else {
		// Unstructured mode: find all .md files in project directory tree
		return findUnstructuredFiles(c, project)
	}
}

// findUnstructuredFiles walks the project directory tree to find all .md files
func findUnstructuredFiles(c *config.Config, project string) ([]string, error) {
	var files []string

	// Determine the root directory to scan
	rootDir := c.Directories.Projects
	if project != "" && project != "*" {
		rootDir = filepath.Join(c.Directories.Projects, project)
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
func ProcessFile(c *config.Config, filePath string) ([]*Task, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Extract project and zettel from path
	var project, zettel string

	if c.Todo.Structured {
		// Structured mode: Path: PRJDIR/project/notes/zet/README.md
		parts := strings.Split(filePath, string(filepath.Separator))
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid structured path: %s", filePath)
		}
		zettel = parts[len(parts)-2]
		project = parts[len(parts)-4]
	} else {
		// Unstructured mode: derive project and zettel from file path
		relPath, err := filepath.Rel(c.Directories.Projects, filePath)
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
		task := ParseLine(c, line, project, zettel, filePath)
		if task != nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, scanner.Err()
}

// ParseLine parses a task line and returns a Task
func ParseLine(c *config.Config, line, project, zettel, filePath string) *Task {
	// First check for basic structure: KEYWORD: title
	basicRe := regexp.MustCompile(`^([A-Z]+):\s*(.+)$`)
	basicMatches := basicRe.FindStringSubmatch(line)
	if len(basicMatches) == 0 {
		return nil
	}

	keyword := basicMatches[1]
	// Check if keyword is valid
	if !isValidKeyword(c, keyword) {
		return nil
	}

	// Parse the rest of the line for metadata
	title := basicMatches[2]

	// Extract tag (#tag)
	var tag string
	tagRe := regexp.MustCompile(`\s*#([^ ]+)`)
	if tagMatch := tagRe.FindStringSubmatch(title); len(tagMatch) > 1 {
		tag = tagMatch[1]
		title = tagRe.ReplaceAllString(title, "") // Remove from title
	}

	// Extract assignee (>> assignee)
	var assignee string
	assigneeRe := regexp.MustCompile(`\s*>>\s*(.+?)(?:\s*#|$)`)
	if assigneeMatch := assigneeRe.FindStringSubmatch(title); len(assigneeMatch) > 1 {
		assignee = strings.TrimSpace(assigneeMatch[1])
		title = assigneeRe.ReplaceAllString(title, "") // Remove from title
	}

	// Extract dates (@date, @s:date, @d:date)
	var scheduledAt, dueAt string

	// Match @s:date pattern
	scheduledRe := regexp.MustCompile(`\s*@s:([^ ]+)`)
	if scheduledMatch := scheduledRe.FindStringSubmatch(title); len(scheduledMatch) > 1 {
		scheduledAt = scheduledMatch[1]
		title = scheduledRe.ReplaceAllString(title, "") // Remove from title
	}

	// Match @d:date pattern
	dueRe := regexp.MustCompile(`\s*@d:([^ ]+)`)
	if dueMatch := dueRe.FindStringSubmatch(title); len(dueMatch) > 1 {
		dueAt = dueMatch[1]
		title = dueRe.ReplaceAllString(title, "") // Remove from title
	}

	// Match simple @date pattern (treat as scheduled date if no explicit @s:date)
	dateRe := regexp.MustCompile(`\s*@([^ :]+)`)
	if dateMatch := dateRe.FindStringSubmatch(title); len(dateMatch) > 1 {
		if scheduledAt == "" { // Only use it if no explicit @s:date was found
			scheduledAt = dateMatch[1]
		}
		title = dateRe.ReplaceAllString(title, "") // Always remove from title
	}

	return &Task{
		Keyword:     keyword,
		Title:       strings.TrimSpace(title),
		Tag:         tag,
		ScheduledAt: scheduledAt,
		DueAt:       dueAt,
		Assignee:    assignee,
		Project:     project,
		Zettel:      zettel,
		FilePath:    filePath,
	}
}

func isValidKeyword(c *config.Config, keyword string) bool {
	for _, kw := range c.Todo.Active {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.Todo.InProgress {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.Todo.Completed {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.Todo.Someday {
		if keyword == kw {
			return true
		}
	}
	return false
}

// ListTasks lists tasks for a project, filtering by showCompleted
func ListTasks(c *config.Config, project string, showCompleted bool) ([]*Task, error) {
	files, err := FindFiles(c, project)
	if err != nil {
		return nil, err
	}

	// Use concurrent processing for better performance
	// Choose between basic or adaptive worker calculation
	numWorkers := calculateAdaptiveWorkers(len(files), "file-processing")
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
				tasks, err := ProcessFile(c, file)
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
			if !t.IsCompleted(c) {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}
	return allTasks, nil
}

// SortByPriority sorts tasks by their priority order
// Order: In Progress (1) -> Active (2) -> Someday (3) -> Completed (4)
func SortByPriority(tasks []*Task, c *config.Config) {
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Priority(c) < tasks[j].Priority(c)
	})
}

// SummarizeProjects summarizes task counts per project
func SummarizeProjects(c *config.Config) (map[string]int, error) {
	files, err := FindFiles(c, "")
	if err != nil {
		return nil, err
	}

	// Use concurrent processing
	numWorkers := calculateOptimalWorkers(len(files), "file-processing")
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
				tasks, err := ProcessFile(c, file)
				if err != nil {
					continue
				}
				parts := strings.Split(file, string(filepath.Separator))
				project := parts[len(parts)-4]
				activeCount := 0
				for _, t := range tasks {
					if t.IsActive(c) {
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

// FilterTasks applies field-specific filtering to a slice of tasks
func FilterTasks(tasks []*Task, filterString string) []*Task {
	if filterString == "" {
		return tasks
	}

	// Detect filter type and apply appropriate logic
	switch {
	case strings.HasPrefix(filterString, ">>"):
		// Assignee filter: >> assignee
		assignee := strings.TrimSpace(filterString[2:])
		return filterByAssignee(tasks, assignee)

	case strings.HasPrefix(filterString, "@s:"):
		// Scheduled date filter: @s:date
		date := strings.TrimSpace(filterString[3:])
		return filterByScheduledDate(tasks, date)

	case strings.HasPrefix(filterString, "@d:"):
		// Due date filter: @d:date
		date := strings.TrimSpace(filterString[3:])
		return filterByDueDate(tasks, date)

	case strings.HasPrefix(filterString, "@"):
		// Simple date filter: @date
		date := strings.TrimSpace(filterString[1:])
		return filterByDate(tasks, date)

	case strings.HasPrefix(filterString, "#"):
		// Tag filter: #tag
		tag := strings.TrimSpace(filterString[1:])
		return filterByTag(tasks, tag)

	default:
		// Default: search in all fields (original behavior)
		return filterByAnyField(tasks, filterString)
	}
}

// filterByAssignee filters tasks by assignee
func filterByAssignee(tasks []*Task, assignee string) []*Task {
	if assignee == "" {
		return tasks
	}

	var filtered []*Task
	assigneeLower := strings.ToLower(assignee)

	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.Assignee), assigneeLower) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// filterByDate filters tasks by the simple date field (now treated as scheduled date)
func filterByDate(tasks []*Task, date string) []*Task {
	if date == "" {
		return tasks
	}

	var filtered []*Task
	dateLower := strings.ToLower(date)

	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.ScheduledAt), dateLower) {
			filtered = append(filtered, task)
		}
	}
	return filtered
} // filterByScheduledDate filters tasks by scheduled date (@s:)
func filterByScheduledDate(tasks []*Task, date string) []*Task {
	if date == "" {
		return tasks
	}

	var filtered []*Task
	dateLower := strings.ToLower(date)

	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.ScheduledAt), dateLower) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// filterByDueDate filters tasks by due date (@d:)
func filterByDueDate(tasks []*Task, date string) []*Task {
	if date == "" {
		return tasks
	}

	var filtered []*Task
	dateLower := strings.ToLower(date)

	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.DueAt), dateLower) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// filterByTag filters tasks by tag
func filterByTag(tasks []*Task, tag string) []*Task {
	if tag == "" {
		return tasks
	}

	var filtered []*Task
	tagLower := strings.ToLower(tag)

	for _, task := range tasks {
		if strings.Contains(strings.ToLower(task.Tag), tagLower) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// filterByAnyField filters tasks by searching in all fields (original behavior)
func filterByAnyField(tasks []*Task, searchTerm string) []*Task {
	if searchTerm == "" {
		return tasks
	}

	var filtered []*Task
	searchLower := strings.ToLower(searchTerm)

	for _, task := range tasks {
		searchString := fmt.Sprintf("%s %s %s %s %s %s %s %s",
			task.Project, task.Zettel, task.Keyword, task.Title,
			task.Tag, task.ScheduledAt, task.DueAt, task.Assignee)

		if strings.Contains(strings.ToLower(searchString), searchLower) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// SearchResult represents a search result from file content
type SearchResult struct {
	ZettelID string
	Title    string
	LineNum  int
	Line     string
	Path     string
	Project  string
}

// SearchInFile searches for a term within a file and returns matching lines
func SearchInFile(filePath, searchTerm string) []SearchResult {
	var results []SearchResult

	file, err := os.Open(filePath)
	if err != nil {
		return results
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), strings.ToLower(searchTerm)) {
			// Extract zettel ID from path if possible
			dir := filepath.Dir(filePath)
			zetID := filepath.Base(dir)

			results = append(results, SearchResult{
				ZettelID: zetID,
				LineNum:  lineNum,
				Line:     line,
				Path:     filePath,
			})
		}
	}
	return results
}

// SearchTasks searches for a term across all task files
func SearchTasks(c *config.Config, project string, searchTerm string) ([]SearchResult, error) {
	files, err := FindFiles(c, project)
	if err != nil {
		return nil, err
	}

	var allResults []SearchResult
	for _, file := range files {
		results := SearchInFile(file, searchTerm)
		if len(results) > 0 {
			// Add project and title information to results
			for i := range results {
				// Extract project from path
				relPath, _ := filepath.Rel(c.Directories.Projects, file)
				if relPath != "" {
					parts := strings.Split(relPath, string(filepath.Separator))
					if len(parts) > 0 {
						results[i].Project = parts[0]
					}
				}
				
				// Get title if possible
				if c.Todo.Structured {
					// For structured mode, try to get the zettel title
					if results[i].ZettelID != "" && IsValidZettelID(results[i].ZettelID) {
						notesDir := filepath.Join(c.Directories.Projects, results[i].Project, "notes")
						title, err := GetZettelTitle(notesDir, results[i].ZettelID)
						if err == nil {
							results[i].Title = title
						}
					}
				} else {
					// For unstructured mode, use filename as title
					filename := filepath.Base(file)
					results[i].Title = strings.TrimSuffix(filename, filepath.Ext(filename))
				}
			}
			allResults = append(allResults, results...)
		}
	}

	return allResults, nil
}

// IsValidZettelID checks if a string is a valid zettel ID
func IsValidZettelID(id string) bool {
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

// GetZettelTitle gets the title of a zettel from its README.md file
func GetZettelTitle(zetDir, zetID string) (string, error) {
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
