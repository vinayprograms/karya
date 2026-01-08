package task

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/parallel"
)

// Task represents a parsed task from a line
type Task struct {
	Keyword     string
	ID          string   // Optional unique identifier [id]
	Title       string
	Tag         string
	References  []string // IDs of tasks this task depends on (^id syntax)
	ScheduledAt string   // @date or @s:date (scheduled date)
	DueAt       string   // @d:date (due date)
	Assignee    string
	Project     string
	Zettel      string
	FilePath    string // Original file path where this task was found
	InCycle     bool   // True if this task participates in a circular dependency
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

	// Extract task ID [id] - must be at the start of title
	var id string
	idRe := regexp.MustCompile(`^\[([^\]]+)\]\s*`)
	if idMatch := idRe.FindStringSubmatch(title); len(idMatch) > 1 {
		id = idMatch[1]
		title = idRe.ReplaceAllString(title, "") // Remove from title
	}

	// Extract references (^id) - can appear multiple times
	var references []string
	refRe := regexp.MustCompile(`\s*\^([^ ]+)`)
	refMatches := refRe.FindAllStringSubmatch(title, -1)
	for _, match := range refMatches {
		if len(match) > 1 {
			references = append(references, match[1])
		}
	}
	title = refRe.ReplaceAllString(title, "") // Remove all references from title

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
		ID:          id,
		Title:       strings.TrimSpace(title),
		Tag:         tag,
		References:  references,
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

	if len(files) == 0 {
		return []*Task{}, nil
	}

	// Use parallel processing with the shared parallel package
	taskSlices := parallel.Process(files, func(file string) *[]*Task {
		tasks, err := ProcessFile(c, file)
		if err != nil {
			return nil
		}
		return &tasks
	})

	// Flatten results
	var allTasks []*Task
	for _, tasksPtr := range taskSlices {
		allTasks = append(allTasks, *tasksPtr...)
	}

	if !showCompleted {
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
// Uses stable sort to preserve relative order of equal-priority tasks
func SortByPriority(tasks []*Task, c *config.Config) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Priority(c) < tasks[j].Priority(c)
	})
}

// SummarizeProjects summarizes task counts per project
func SummarizeProjects(c *config.Config) (map[string]int, error) {
	files, err := FindFiles(c, "")
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return make(map[string]int), nil
	}

	type projectCount struct {
		project string
		count   int
	}

	// Use parallel processing with the shared parallel package
	results := parallel.Collect(files, func(file string) (projectCount, bool) {
		tasks, err := ProcessFile(c, file)
		if err != nil {
			return projectCount{}, false
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
			return projectCount{project: project, count: activeCount}, true
		}
		return projectCount{}, false
	})

	// Aggregate results
	summary := make(map[string]int)
	for _, res := range results {
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
		searchString := fmt.Sprintf("%s %s %s %s %s %s %s %s %s %s",
			task.Project, task.Zettel, task.Keyword, task.ID, task.Title,
			task.Tag, task.ScheduledAt, task.DueAt, task.Assignee,
			strings.Join(task.References, " "))

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

// UpdateTaskStatus updates the keyword of a task in its source file.
// It finds the line matching the task and replaces the keyword with the new one.
// Returns the updated line number, or an error if the task was not found.
func UpdateTaskStatus(t *Task, newKeyword string) error {
	if t.FilePath == "" {
		return fmt.Errorf("task has no file path")
	}

	// Read the file
	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	found := false

	// Build the search pattern: "KEYWORD: [id] title" or "KEYWORD: title" (with optional metadata after title)
	var searchPrefix string
	if t.ID != "" {
		searchPrefix = fmt.Sprintf("%s: [%s] %s", t.Keyword, t.ID, t.Title)
	} else {
		searchPrefix = fmt.Sprintf("%s: %s", t.Keyword, t.Title)
	}

	for i, line := range lines {
		// Check if this line starts with our task's keyword and title
		if strings.HasPrefix(line, searchPrefix) {
			// Replace the old keyword with the new one
			newLine := newKeyword + line[len(t.Keyword):]
			lines[i] = newLine
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task not found in file: %s: %s", t.Keyword, t.Title)
	}

	// Write the file back
	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(t.FilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Update the task's keyword in memory
	t.Keyword = newKeyword

	return nil
}

// GetAllKeywords returns all configured keywords grouped by category
func GetAllKeywords(c *config.Config) map[string][]string {
	return map[string][]string{
		"Active":     c.Todo.Active,
		"InProgress": c.Todo.InProgress,
		"Completed":  c.Todo.Completed,
		"Someday":    c.Todo.Someday,
	}
}

// GetAllKeywordsFlat returns all configured keywords as a flat slice with category labels
type KeywordEntry struct {
	Keyword  string
	Category string
}

func GetAllKeywordsFlat(c *config.Config) []KeywordEntry {
	var entries []KeywordEntry

	for _, kw := range c.Todo.Active {
		entries = append(entries, KeywordEntry{Keyword: kw, Category: "Active"})
	}
	for _, kw := range c.Todo.InProgress {
		entries = append(entries, KeywordEntry{Keyword: kw, Category: "InProgress"})
	}
	for _, kw := range c.Todo.Completed {
		entries = append(entries, KeywordEntry{Keyword: kw, Category: "Completed"})
	}
	for _, kw := range c.Todo.Someday {
		entries = append(entries, KeywordEntry{Keyword: kw, Category: "Someday"})
	}

	return entries
}

// DetectCycles finds all tasks that participate in circular dependencies.
// It builds a directed graph from task references and uses DFS to detect cycles.
// Tasks participating in cycles will have their InCycle field set to true.
func DetectCycles(tasks []*Task) {
	// Build ID -> Task map for tasks that have IDs
	taskByID := make(map[string]*Task)
	for _, t := range tasks {
		if t.ID != "" {
			taskByID[t.ID] = t
		}
	}

	// Build adjacency list (task ID -> referenced task IDs)
	graph := make(map[string][]string)
	for _, t := range tasks {
		if t.ID != "" {
			graph[t.ID] = t.References
		}
	}

	// Find all cycle participants using DFS
	cycleParticipants := findCycleParticipants(graph)

	// Mark tasks that are in cycles
	for _, t := range tasks {
		if t.ID != "" {
			if _, inCycle := cycleParticipants[t.ID]; inCycle {
				t.InCycle = true
			}
		}
	}
}

// findCycleParticipants returns a set of node IDs that participate in any cycle
func findCycleParticipants(graph map[string][]string) map[string]bool {
	// Track visit states: 0=unvisited, 1=visiting (in current path), 2=visited
	state := make(map[string]int)
	// Track the path during DFS
	path := make([]string, 0)
	// All nodes that are part of any cycle
	cycleNodes := make(map[string]bool)

	var dfs func(node string) bool
	dfs = func(node string) bool {
		if state[node] == 1 {
			// Found a cycle - mark all nodes in the path from this node
			foundStart := false
			for _, n := range path {
				if n == node {
					foundStart = true
				}
				if foundStart {
					cycleNodes[n] = true
				}
			}
			return true
		}
		if state[node] == 2 {
			return false
		}

		state[node] = 1 // Mark as visiting
		path = append(path, node)

		for _, neighbor := range graph[node] {
			// Only follow edges to nodes that exist in the graph
			if _, exists := graph[neighbor]; exists {
				dfs(neighbor)
			}
		}

		path = path[:len(path)-1] // Remove from path
		state[node] = 2           // Mark as visited
		return false
	}

	// Run DFS from all nodes
	for node := range graph {
		if state[node] == 0 {
			dfs(node)
		}
	}

	return cycleNodes
}

// GetTaskByID returns a task by its ID from a slice of tasks.
// Returns nil if id is empty or not found.
func GetTaskByID(tasks []*Task, id string) *Task {
	if id == "" {
		return nil
	}
	for _, t := range tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// GetDependencies returns the tasks that the given task depends on
func GetDependencies(tasks []*Task, t *Task) []*Task {
	var deps []*Task
	for _, refID := range t.References {
		if dep := GetTaskByID(tasks, refID); dep != nil {
			deps = append(deps, dep)
		}
	}
	return deps
}

// GetDependents returns the tasks that depend on the given task
func GetDependents(tasks []*Task, t *Task) []*Task {
	if t.ID == "" {
		return nil
	}
	var dependents []*Task
	for _, other := range tasks {
		for _, refID := range other.References {
			if refID == t.ID {
				dependents = append(dependents, other)
				break
			}
		}
	}
	return dependents
}
