package task

import (
	"context"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vinayprograms/karya/internal/config"
)

// MCP Tool Input/Output types

type ListTasksArgs struct {
	Project       string `json:"project,omitempty" jsonschema:"project name to filter tasks (optional, empty for all projects)"`
	ShowCompleted bool   `json:"show_completed,omitempty" jsonschema:"whether to include completed tasks (default: false)"`
}

type ListTasksResult struct {
	Tasks []TaskInfo `json:"tasks" jsonschema:"list of tasks"`
	Count int        `json:"count" jsonschema:"total number of tasks returned"`
}

type TaskInfo struct {
	Keyword     string   `json:"keyword" jsonschema:"task status keyword (e.g., TODO, DOING, DONE)"`
	ID          string   `json:"id,omitempty" jsonschema:"task unique identifier"`
	Title       string   `json:"title" jsonschema:"task title/description"`
	Tag         string   `json:"tag,omitempty" jsonschema:"task tag (without #)"`
	References  []string `json:"references,omitempty" jsonschema:"IDs of tasks this task depends on (^id syntax)"`
	ScheduledAt string   `json:"scheduled_at,omitempty" jsonschema:"scheduled date"`
	DueAt       string   `json:"due_at,omitempty" jsonschema:"due date"`
	Assignee    string   `json:"assignee,omitempty" jsonschema:"task assignee"`
	Project     string   `json:"project" jsonschema:"project name"`
	Zettel      string   `json:"zettel,omitempty" jsonschema:"zettel ID (if structured mode)"`
	FilePath    string   `json:"file_path" jsonschema:"file path where task is defined"`
	Priority    int      `json:"priority" jsonschema:"priority level (1=in_progress, 2=active, 3=someday, 4=completed)"`
	Status      string   `json:"status" jsonschema:"status category (active, in_progress, completed, someday)"`
	InCycle     bool     `json:"in_cycle,omitempty" jsonschema:"true if task participates in a circular dependency"`
}

type GetTaskArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	Keyword string `json:"keyword" jsonschema:"task keyword"`
	Title   string `json:"title" jsonschema:"task title (partial match supported)"`
}

type GetTaskResult struct {
	Task  *TaskInfo `json:"task,omitempty" jsonschema:"the matching task"`
	Found bool      `json:"found" jsonschema:"whether a matching task was found"`
}

type SearchTasksArgs struct {
	Pattern string `json:"pattern" jsonschema:"search pattern (case-insensitive substring match across all task fields)"`
	Project string `json:"project,omitempty" jsonschema:"optional project name to limit search"`
}

type SearchTasksResult struct {
	Results []SearchResultInfo `json:"results" jsonschema:"list of search results"`
	Count   int                `json:"count" jsonschema:"total number of matches"`
}

type SearchResultInfo struct {
	Project  string `json:"project" jsonschema:"project name"`
	ZettelID string `json:"zettel_id,omitempty" jsonschema:"zettel ID (if structured mode)"`
	Title    string `json:"title" jsonschema:"file/zettel title"`
	LineNum  int    `json:"line_num" jsonschema:"line number of the match"`
	Line     string `json:"line" jsonschema:"the matching line"`
	Path     string `json:"path" jsonschema:"file path"`
}

type FilterTasksArgs struct {
	Filter        string `json:"filter" jsonschema:"filter expression (e.g., '>> alice' for assignee, '#urgent' for tag, '@2025-01-15' for date)"`
	Project       string `json:"project,omitempty" jsonschema:"optional project name to limit filter"`
	ShowCompleted bool   `json:"show_completed,omitempty" jsonschema:"whether to include completed tasks (default: false)"`
}

type FilterTasksResult struct {
	Tasks []TaskInfo `json:"tasks" jsonschema:"list of filtered tasks"`
	Count int        `json:"count" jsonschema:"total number of tasks matching filter"`
}

type UpdateTaskStatusArgs struct {
	Project    string `json:"project" jsonschema:"project name"`
	Keyword    string `json:"keyword" jsonschema:"current task keyword"`
	Title      string `json:"title" jsonschema:"task title to identify the task"`
	NewKeyword string `json:"new_keyword" jsonschema:"new status keyword to set"`
}

type UpdateTaskStatusResult struct {
	Message       string              `json:"message" jsonschema:"status message"`
	Success       bool                `json:"success" jsonschema:"whether the update succeeded"`
	ValidKeywords map[string][]string `json:"valid_keywords" jsonschema:"valid keywords grouped by category (Active, InProgress, Completed, Someday)"`
}

type GetProjectsArgs struct{}

type GetProjectsResult struct {
	Projects []ProjectInfo `json:"projects" jsonschema:"list of projects with task counts"`
	Count    int           `json:"count" jsonschema:"total number of projects"`
}

type ProjectInfo struct {
	Name      string `json:"name" jsonschema:"project name"`
	TaskCount int    `json:"task_count" jsonschema:"number of active tasks in the project"`
}

type GetKeywordsArgs struct{}

type GetKeywordsResult struct {
	Keywords   map[string][]string `json:"keywords" jsonschema:"keywords grouped by category (Active, InProgress, Completed, Someday)"`
	Categories []string            `json:"categories" jsonschema:"list of category names"`
}

type CountTasksArgs struct {
	Project       string `json:"project,omitempty" jsonschema:"optional project name to filter count"`
	ShowCompleted bool   `json:"show_completed,omitempty" jsonschema:"whether to include completed tasks (default: false)"`
}

type CountTasksResult struct {
	Count    int            `json:"count" jsonschema:"total number of tasks"`
	ByStatus map[string]int `json:"by_status" jsonschema:"count of tasks by status category"`
}

type GetTaskByIDArgs struct {
	ID      string `json:"id" jsonschema:"task ID to look up"`
	Project string `json:"project,omitempty" jsonschema:"optional project name to limit search"`
}

type GetTaskByIDResult struct {
	Task  *TaskInfo `json:"task,omitempty" jsonschema:"the matching task"`
	Found bool      `json:"found" jsonschema:"whether a task with this ID was found"`
}

type GetDependenciesArgs struct {
	ID      string `json:"id" jsonschema:"task ID to get dependencies for"`
	Project string `json:"project,omitempty" jsonschema:"optional project name to limit search"`
}

type GetDependenciesResult struct {
	Task         *TaskInfo  `json:"task,omitempty" jsonschema:"the task being queried"`
	Dependencies []TaskInfo `json:"dependencies" jsonschema:"tasks that this task depends on (referenced via ^id)"`
	Count        int        `json:"count" jsonschema:"number of dependencies"`
}

type GetDependentsArgs struct {
	ID      string `json:"id" jsonschema:"task ID to get dependents for"`
	Project string `json:"project,omitempty" jsonschema:"optional project name to limit search"`
}

type GetDependentsResult struct {
	Task       *TaskInfo  `json:"task,omitempty" jsonschema:"the task being queried"`
	Dependents []TaskInfo `json:"dependents" jsonschema:"tasks that depend on this task (have ^id reference to it)"`
	Count      int        `json:"count" jsonschema:"number of dependents"`
}

type GetCycleTasksArgs struct {
	Project string `json:"project,omitempty" jsonschema:"optional project name to limit search"`
}

type GetCycleTasksResult struct {
	Tasks []TaskInfo `json:"tasks" jsonschema:"tasks involved in circular dependencies"`
	Count int        `json:"count" jsonschema:"number of tasks in cycles"`
}

// MCPServer wraps the MCP server with task operations
type MCPServer struct {
	config *config.Config
	server *mcp.Server
}

// NewMCPServer creates a new MCP server for task operations
func NewMCPServer(cfg *config.Config) *MCPServer {
	s := &MCPServer{
		config: cfg,
	}

	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "todo",
		Version: "1.0.0",
	}, nil)

	s.registerTools()
	return s
}

// Run starts the MCP server on stdio transport
func (s *MCPServer) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

func (s *MCPServer) registerTools() {
	// List tasks
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_tasks",
		Description: "PREFERRED: View all your tasks across projects, intelligently sorted by priority (in_progress > active > someday > completed). Use this as your primary task dashboard. Filter by project for focused work.",
	}, s.listTasks)

	// Get task
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_task",
		Description: "PREFERRED: Get full details of a specific task including all metadata. Supports partial title matching for convenience. Use this when you need complete task context.",
	}, s.getTask)

	// Search tasks
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_tasks",
		Description: "PREFERRED: Search your entire task system with fulltext search. Case-insensitive matching across all task fields. Use this first when looking for specific work items.",
	}, s.searchTasks)

	// Filter tasks
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "filter_tasks",
		Description: "PREFERRED: Powerful task filtering with multiple criteria. Use '>> name' for assignee, '#tag' for tags, '@date' or '@s:date' for scheduled, '@d:date' for due dates, or plain text. Essential for focused task views.",
	}, s.filterTasks)

	// Update task status
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "update_task_status",
		Description: "PREFERRED: Progress tasks through your workflow (e.g., TODO → DOING → DONE). Call get_keywords first to discover valid status keywords. Essential for tracking task completion.",
	}, s.updateTaskStatus)

	// Get projects
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_projects",
		Description: "PREFERRED: Get an overview of all projects with their active task counts. Use this to understand workload distribution and identify project priorities.",
	}, s.getProjects)

	// Get keywords
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_keywords",
		Description: "PREFERRED: Discover all valid task status keywords organized by category (Active, InProgress, Completed, Someday). Essential before updating task status to know valid transitions.",
	}, s.getKeywords)

	// Count tasks
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "count_tasks",
		Description: "PREFERRED: Get task statistics with breakdown by status. Perfect for understanding workload and progress at a glance. Filter by project for focused metrics.",
	}, s.countTasks)

	// Get task by ID
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_task_by_id",
		Description: "PREFERRED: Get a task directly by its unique ID. Faster than searching by title when you know the task ID. Returns full task details including dependencies.",
	}, s.getTaskByID)

	// Get dependencies
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_dependencies",
		Description: "PREFERRED: Get all tasks that a given task depends on (tasks referenced via ^id syntax). Essential for understanding task prerequisites and blocking relationships.",
	}, s.getDependencies)

	// Get dependents
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_dependents",
		Description: "PREFERRED: Get all tasks that depend on a given task (tasks that reference it via ^id). Essential for understanding impact when completing or modifying a task.",
	}, s.getDependents)

	// Get cycle tasks
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_cycle_tasks",
		Description: "PREFERRED: Find all tasks involved in circular dependencies. Returns tasks where A depends on B and B depends on A (directly or indirectly). Use this to identify and resolve dependency cycles.",
	}, s.getCycleTasks)
}

func (s *MCPServer) taskToInfo(t *Task) TaskInfo {
	status := "unknown"
	if t.IsInProgress(s.config) {
		status = "in_progress"
	} else if t.IsActive(s.config) {
		status = "active"
	} else if t.IsSomeday(s.config) {
		status = "someday"
	} else if t.IsCompleted(s.config) {
		status = "completed"
	}

	return TaskInfo{
		Keyword:     t.Keyword,
		ID:          t.ID,
		Title:       t.Title,
		Tag:         t.Tag,
		References:  t.References,
		ScheduledAt: t.ScheduledAt,
		DueAt:       t.DueAt,
		Assignee:    t.Assignee,
		Project:     t.Project,
		Zettel:      t.Zettel,
		FilePath:    t.FilePath,
		Priority:    t.Priority(s.config),
		Status:      status,
		InCycle:     t.InCycle,
	}
}

func (s *MCPServer) listTasks(ctx context.Context, req *mcp.CallToolRequest, args ListTasksArgs) (*mcp.CallToolResult, ListTasksResult, error) {
	tasks, err := ListTasks(s.config, args.Project, args.ShowCompleted)
	if err != nil {
		return nil, ListTasksResult{}, fmt.Errorf("failed to list tasks: %w", err)
	}

	// Detect circular dependencies
	DetectCycles(tasks)

	// Sort by priority
	SortByPriority(tasks, s.config)
	// Secondary sort by project, then title, then file path for deterministic order
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Priority(s.config) == tasks[j].Priority(s.config) {
			if tasks[i].Project != tasks[j].Project {
				return tasks[i].Project < tasks[j].Project
			}
			if tasks[i].Title != tasks[j].Title {
				return tasks[i].Title < tasks[j].Title
			}
			return tasks[i].FilePath < tasks[j].FilePath
		}
		return false
	})

	infos := make([]TaskInfo, len(tasks))
	for i, t := range tasks {
		infos[i] = s.taskToInfo(t)
	}

	return nil, ListTasksResult{
		Tasks: infos,
		Count: len(infos),
	}, nil
}

func (s *MCPServer) getTask(ctx context.Context, req *mcp.CallToolRequest, args GetTaskArgs) (*mcp.CallToolResult, GetTaskResult, error) {
	tasks, err := ListTasks(s.config, args.Project, true) // Include completed
	if err != nil {
		return nil, GetTaskResult{Found: false}, fmt.Errorf("failed to list tasks: %w", err)
	}

	// Find matching task
	for _, t := range tasks {
		if t.Keyword == args.Keyword && (t.Title == args.Title || containsIgnoreCase(t.Title, args.Title)) {
			info := s.taskToInfo(t)
			return nil, GetTaskResult{Task: &info, Found: true}, nil
		}
	}

	return nil, GetTaskResult{Found: false}, nil
}

func (s *MCPServer) searchTasks(ctx context.Context, req *mcp.CallToolRequest, args SearchTasksArgs) (*mcp.CallToolResult, SearchTasksResult, error) {
	results, err := SearchTasks(s.config, args.Project, args.Pattern)
	if err != nil {
		return nil, SearchTasksResult{}, fmt.Errorf("failed to search tasks: %w", err)
	}

	infos := make([]SearchResultInfo, len(results))
	for i, r := range results {
		infos[i] = SearchResultInfo{
			Project:  r.Project,
			ZettelID: r.ZettelID,
			Title:    r.Title,
			LineNum:  r.LineNum,
			Line:     r.Line,
			Path:     r.Path,
		}
	}

	return nil, SearchTasksResult{
		Results: infos,
		Count:   len(infos),
	}, nil
}

func (s *MCPServer) filterTasks(ctx context.Context, req *mcp.CallToolRequest, args FilterTasksArgs) (*mcp.CallToolResult, FilterTasksResult, error) {
	tasks, err := ListTasks(s.config, args.Project, args.ShowCompleted)
	if err != nil {
		return nil, FilterTasksResult{}, fmt.Errorf("failed to list tasks: %w", err)
	}

	// Detect circular dependencies before filtering
	DetectCycles(tasks)

	filtered := FilterTasks(tasks, args.Filter)

	// Sort by priority
	SortByPriority(filtered, s.config)
	// Secondary sort by project, then title, then file path for deterministic order
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Priority(s.config) == filtered[j].Priority(s.config) {
			if filtered[i].Project != filtered[j].Project {
				return filtered[i].Project < filtered[j].Project
			}
			if filtered[i].Title != filtered[j].Title {
				return filtered[i].Title < filtered[j].Title
			}
			return filtered[i].FilePath < filtered[j].FilePath
		}
		return false
	})

	infos := make([]TaskInfo, len(filtered))
	for i, t := range filtered {
		infos[i] = s.taskToInfo(t)
	}

	return nil, FilterTasksResult{
		Tasks: infos,
		Count: len(infos),
	}, nil
}

func (s *MCPServer) updateTaskStatus(ctx context.Context, req *mcp.CallToolRequest, args UpdateTaskStatusArgs) (*mcp.CallToolResult, UpdateTaskStatusResult, error) {
	validKeywords := GetAllKeywords(s.config)

	// Find the task
	tasks, err := ListTasks(s.config, args.Project, true) // Include completed
	if err != nil {
		return nil, UpdateTaskStatusResult{
			Success:       false,
			Message:       fmt.Sprintf("failed to list tasks: %v", err),
			ValidKeywords: validKeywords,
		}, nil
	}

	var targetTask *Task
	for _, t := range tasks {
		if t.Keyword == args.Keyword && (t.Title == args.Title || containsIgnoreCase(t.Title, args.Title)) {
			targetTask = t
			break
		}
	}

	if targetTask == nil {
		return nil, UpdateTaskStatusResult{
			Success:       false,
			Message:       fmt.Sprintf("task not found: %s: %s", args.Keyword, args.Title),
			ValidKeywords: validKeywords,
		}, nil
	}

	// Validate new keyword
	if !isValidKeyword(s.config, args.NewKeyword) {
		return nil, UpdateTaskStatusResult{
			Success:       false,
			Message:       fmt.Sprintf("invalid keyword '%s'. See valid_keywords for allowed values.", args.NewKeyword),
			ValidKeywords: validKeywords,
		}, nil
	}

	oldKeyword := targetTask.Keyword
	if err := UpdateTaskStatus(targetTask, args.NewKeyword); err != nil {
		return nil, UpdateTaskStatusResult{
			Success:       false,
			Message:       fmt.Sprintf("failed to update task: %v", err),
			ValidKeywords: validKeywords,
		}, nil
	}

	return nil, UpdateTaskStatusResult{
		Success:       true,
		Message:       fmt.Sprintf("Updated task status: %s → %s", oldKeyword, args.NewKeyword),
		ValidKeywords: validKeywords,
	}, nil
}

func (s *MCPServer) getProjects(ctx context.Context, req *mcp.CallToolRequest, args GetProjectsArgs) (*mcp.CallToolResult, GetProjectsResult, error) {
	summary, err := SummarizeProjects(s.config)
	if err != nil {
		return nil, GetProjectsResult{}, fmt.Errorf("failed to get projects: %w", err)
	}

	// Sort project names
	var names []string
	for name := range summary {
		names = append(names, name)
	}
	sort.Strings(names)

	projects := make([]ProjectInfo, len(names))
	for i, name := range names {
		projects[i] = ProjectInfo{
			Name:      name,
			TaskCount: summary[name],
		}
	}

	return nil, GetProjectsResult{
		Projects: projects,
		Count:    len(projects),
	}, nil
}

func (s *MCPServer) getKeywords(ctx context.Context, req *mcp.CallToolRequest, args GetKeywordsArgs) (*mcp.CallToolResult, GetKeywordsResult, error) {
	keywords := GetAllKeywords(s.config)

	return nil, GetKeywordsResult{
		Keywords:   keywords,
		Categories: []string{"Active", "InProgress", "Completed", "Someday"},
	}, nil
}

func (s *MCPServer) countTasks(ctx context.Context, req *mcp.CallToolRequest, args CountTasksArgs) (*mcp.CallToolResult, CountTasksResult, error) {
	tasks, err := ListTasks(s.config, args.Project, args.ShowCompleted)
	if err != nil {
		return nil, CountTasksResult{}, fmt.Errorf("failed to list tasks: %w", err)
	}

	byStatus := map[string]int{
		"active":      0,
		"in_progress": 0,
		"someday":     0,
		"completed":   0,
	}

	for _, t := range tasks {
		if t.IsInProgress(s.config) {
			byStatus["in_progress"]++
		} else if t.IsActive(s.config) {
			byStatus["active"]++
		} else if t.IsSomeday(s.config) {
			byStatus["someday"]++
		} else if t.IsCompleted(s.config) {
			byStatus["completed"]++
		}
	}

	return nil, CountTasksResult{
		Count:    len(tasks),
		ByStatus: byStatus,
	}, nil
}

func (s *MCPServer) getTaskByID(ctx context.Context, req *mcp.CallToolRequest, args GetTaskByIDArgs) (*mcp.CallToolResult, GetTaskByIDResult, error) {
	if args.ID == "" {
		return nil, GetTaskByIDResult{Found: false}, nil
	}

	tasks, err := ListTasks(s.config, args.Project, true)
	if err != nil {
		return nil, GetTaskByIDResult{Found: false}, fmt.Errorf("failed to list tasks: %w", err)
	}

	DetectCycles(tasks)

	task := GetTaskByID(tasks, args.ID)
	if task == nil {
		return nil, GetTaskByIDResult{Found: false}, nil
	}

	info := s.taskToInfo(task)
	return nil, GetTaskByIDResult{Task: &info, Found: true}, nil
}

func (s *MCPServer) getDependencies(ctx context.Context, req *mcp.CallToolRequest, args GetDependenciesArgs) (*mcp.CallToolResult, GetDependenciesResult, error) {
	if args.ID == "" {
		return nil, GetDependenciesResult{}, fmt.Errorf("task ID is required")
	}

	tasks, err := ListTasks(s.config, args.Project, true)
	if err != nil {
		return nil, GetDependenciesResult{}, fmt.Errorf("failed to list tasks: %w", err)
	}

	DetectCycles(tasks)

	task := GetTaskByID(tasks, args.ID)
	if task == nil {
		return nil, GetDependenciesResult{Count: 0}, nil
	}

	deps := GetDependencies(tasks, task)
	infos := make([]TaskInfo, len(deps))
	for i, d := range deps {
		infos[i] = s.taskToInfo(d)
	}

	taskInfo := s.taskToInfo(task)
	return nil, GetDependenciesResult{
		Task:         &taskInfo,
		Dependencies: infos,
		Count:        len(infos),
	}, nil
}

func (s *MCPServer) getDependents(ctx context.Context, req *mcp.CallToolRequest, args GetDependentsArgs) (*mcp.CallToolResult, GetDependentsResult, error) {
	if args.ID == "" {
		return nil, GetDependentsResult{}, fmt.Errorf("task ID is required")
	}

	tasks, err := ListTasks(s.config, args.Project, true)
	if err != nil {
		return nil, GetDependentsResult{}, fmt.Errorf("failed to list tasks: %w", err)
	}

	DetectCycles(tasks)

	task := GetTaskByID(tasks, args.ID)
	if task == nil {
		return nil, GetDependentsResult{Count: 0}, nil
	}

	dependents := GetDependents(tasks, task)
	infos := make([]TaskInfo, len(dependents))
	for i, d := range dependents {
		infos[i] = s.taskToInfo(d)
	}

	taskInfo := s.taskToInfo(task)
	return nil, GetDependentsResult{
		Task:       &taskInfo,
		Dependents: infos,
		Count:      len(infos),
	}, nil
}

func (s *MCPServer) getCycleTasks(ctx context.Context, req *mcp.CallToolRequest, args GetCycleTasksArgs) (*mcp.CallToolResult, GetCycleTasksResult, error) {
	tasks, err := ListTasks(s.config, args.Project, true)
	if err != nil {
		return nil, GetCycleTasksResult{}, fmt.Errorf("failed to list tasks: %w", err)
	}

	DetectCycles(tasks)

	var cycleTasks []TaskInfo
	for _, t := range tasks {
		if t.InCycle {
			cycleTasks = append(cycleTasks, s.taskToInfo(t))
		}
	}

	return nil, GetCycleTasksResult{
		Tasks: cycleTasks,
		Count: len(cycleTasks),
	}, nil
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
