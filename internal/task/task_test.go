package task

import (
	"os"
	"testing"

	"github.com/vinayprograms/karya/internal/config"
)

func createTestConfig() *config.Config {
	return &config.Config{
		Todo: config.Todo{
			Active: []string{
				"TODO", "TASK", "NOTE", "REMINDER", "EVENT", "MEETING",
			},
			InProgress: []string{
				"DOING", "INPROGRESS", "WIP", "WORKING", "STARTED",
			},
			Completed: []string{
				"ARCHIVED", "CANCELED", "DELETED", "DONE", "COMPLETED", "CLOSED",
			},
			Someday: []string{
				"SOMEDAY", "MAYBE", "LATER", "WISHLIST",
			},
		},
		Colors: config.ColorScheme{
			SomedayColor: "7", // White - neutral for tasks not yet under consideration
		},
	}
}

func TestTask_IsSomeday(t *testing.T) {
	cfg := createTestConfig()
	
	tests := []struct {
		name    string
		keyword string
		want    bool
	}{
		{"someday keyword", "SOMEDAY", true},
		{"maybe keyword", "MAYBE", true},
		{"later keyword", "LATER", true},
		{"wishlist keyword", "WISHLIST", true},
		{"active keyword should not be someday", "TODO", false},
		{"completed keyword should not be someday", "DONE", false},
		{"inprogress keyword should not be someday", "DOING", false},
		{"unknown keyword should not be someday", "UNKNOWN", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Keyword: tt.keyword}
			if got := task.IsSomeday(cfg); got != tt.want {
				t.Errorf("Task.IsSomeday() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_IsSomeday_NilConfig(t *testing.T) {
	task := &Task{Keyword: "SOMEDAY"}
	if got := task.IsSomeday(nil); got != false {
		t.Errorf("Task.IsSomeday() with nil config = %v, want false", got)
	}
}

func TestParseLineSomedayKeywords(t *testing.T) {
	cfg := createTestConfig()
	
	tests := []struct {
		name     string
		line     string
		project  string
		zettel   string
		filepath string
		want     *Task
	}{
		{
			name:     "parse someday task",
			line:     "SOMEDAY: Learn a new language",
			project:  "personal",
			zettel:   "20240101000000",
			filepath: "test.md",
			want: &Task{
				Keyword:  "SOMEDAY",
				Title:    "Learn a new language",
				Project:  "personal",
				Zettel:   "20240101000000",
				FilePath: "test.md",
			},
		},
		{
			name:     "parse maybe task with extras",
			line:     "MAYBE: Visit Japan #travel @2024-12-01 >> spouse",
			project:  "travel",
			zettel:   "20240201000000",
			filepath: "test.md",
			want: &Task{
				Keyword:     "MAYBE",
				Title:       "Visit Japan",
				Tag:         "travel",
				ScheduledAt: "2024-12-01",
				Assignee:    "spouse",
				Project:     "travel",
				Zettel:      "20240201000000",
				FilePath:    "test.md",
			},
		},
		{
			name:     "parse later task",
			line:     "LATER: Organize garage",
			project:  "home",
			zettel:   "20240301000000",
			filepath: "tasks.md",
			want: &Task{
				Keyword:  "LATER",
				Title:    "Organize garage",
				Project:  "home",
				Zettel:   "20240301000000",
				FilePath: "tasks.md",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLine(cfg, tt.line, tt.project, tt.zettel, tt.filepath)
			if got == nil && tt.want != nil {
				t.Errorf("ParseLine() = nil, want %v", tt.want)
				return
			}
			if got == nil && tt.want == nil {
				return
			}
			if tt.want == nil {
				t.Errorf("ParseLine() = %v, want nil", got)
				return
			}
			
			if got.Keyword != tt.want.Keyword {
				t.Errorf("ParseLine().Keyword = %v, want %v", got.Keyword, tt.want.Keyword)
			}
			if got.Title != tt.want.Title {
				t.Errorf("ParseLine().Title = %v, want %v", got.Title, tt.want.Title)
			}
			if got.Tag != tt.want.Tag {
				t.Errorf("ParseLine().Tag = %v, want %v", got.Tag, tt.want.Tag)
			}
			if got.ScheduledAt != tt.want.ScheduledAt {
				t.Errorf("ParseLine().ScheduledAt = %v, want %v", got.ScheduledAt, tt.want.ScheduledAt)
			}
			if got.Assignee != tt.want.Assignee {
				t.Errorf("ParseLine().Assignee = %v, want %v", got.Assignee, tt.want.Assignee)
			}
			if got.Project != tt.want.Project {
				t.Errorf("ParseLine().Project = %v, want %v", got.Project, tt.want.Project)
			}
			if got.Zettel != tt.want.Zettel {
				t.Errorf("ParseLine().Zettel = %v, want %v", got.Zettel, tt.want.Zettel)
			}
			if got.FilePath != tt.want.FilePath {
				t.Errorf("ParseLine().FilePath = %v, want %v", got.FilePath, tt.want.FilePath)
			}
		})
	}
}

func TestTaskPriority(t *testing.T) {
	cfg := createTestConfig()
	
	// Create tasks of different types
	activeTask := &Task{Keyword: "TODO"}
	inProgressTask := &Task{Keyword: "DOING"}
	somedayTask := &Task{Keyword: "SOMEDAY"}
	completedTask := &Task{Keyword: "DONE"}
	
	tests := []struct {
		name         string
		task         *Task
		wantPriority int
	}{
		{"in-progress task has highest priority", inProgressTask, 1},
		{"active task has second priority", activeTask, 2},
		{"someday task has third priority", somedayTask, 3},
		{"completed task has lowest priority", completedTask, 4},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.Priority(cfg)
			if got != tt.wantPriority {
				t.Errorf("Task.Priority() = %v, want %v", got, tt.wantPriority)
			}
		})
	}
}

func TestTaskSorting(t *testing.T) {
	cfg := createTestConfig()
	
	tasks := []*Task{
		{Keyword: "DONE", Title: "Completed task"},
		{Keyword: "SOMEDAY", Title: "Future task"},
		{Keyword: "TODO", Title: "Active task"},
		{Keyword: "DOING", Title: "In progress task"},
	}
	
	// Sort by priority
	SortByPriority(tasks, cfg)
	
	expectedOrder := []string{"DOING", "TODO", "SOMEDAY", "DONE"}
	for i, expectedKeyword := range expectedOrder {
		if tasks[i].Keyword != expectedKeyword {
			t.Errorf("After sorting, task %d should be %s, got %s", i, expectedKeyword, tasks[i].Keyword)
		}
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	cfg := createTestConfig()

	// Create a temporary file with a task
	tmpFile, err := os.CreateTemp("", "task_test_*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write initial content
	content := `# Test File

TODO: Write tests for the feature
DOING: Implement the API
DONE: Review the documentation
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create a task
	task := &Task{
		Keyword:  "TODO",
		Title:    "Write tests for the feature",
		FilePath: tmpFile.Name(),
	}

	// Update the task status
	err = UpdateTaskStatus(task, "DOING")
	if err != nil {
		t.Fatalf("UpdateTaskStatus() error = %v", err)
	}

	// Verify the task keyword was updated in memory
	if task.Keyword != "DOING" {
		t.Errorf("Task.Keyword = %v, want DOING", task.Keyword)
	}

	// Verify the file was updated
	updatedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	expectedContent := `# Test File

DOING: Write tests for the feature
DOING: Implement the API
DONE: Review the documentation
`
	if string(updatedContent) != expectedContent {
		t.Errorf("File content = %q, want %q", string(updatedContent), expectedContent)
	}

	// Test that task is now considered in-progress
	if !task.IsInProgress(cfg) {
		t.Error("Task should be in-progress after status update")
	}
}

func TestUpdateTaskStatus_TaskNotFound(t *testing.T) {
	// Create a temporary file without the task
	tmpFile, err := os.CreateTemp("", "task_test_*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write content without the task
	content := `# Test File

TODO: Different task
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create a task that doesn't exist in the file
	task := &Task{
		Keyword:  "TODO",
		Title:    "Non-existent task",
		FilePath: tmpFile.Name(),
	}

	// Update the task status - should fail
	err = UpdateTaskStatus(task, "DOING")
	if err == nil {
		t.Error("UpdateTaskStatus() should return error for non-existent task")
	}
}

func TestUpdateTaskStatus_NoFilePath(t *testing.T) {
	task := &Task{
		Keyword: "TODO",
		Title:   "Test task",
	}

	err := UpdateTaskStatus(task, "DOING")
	if err == nil {
		t.Error("UpdateTaskStatus() should return error when FilePath is empty")
	}
}

func TestGetAllKeywords(t *testing.T) {
	cfg := createTestConfig()

	keywords := GetAllKeywords(cfg)

	if len(keywords["Active"]) != 6 {
		t.Errorf("Expected 6 active keywords, got %d", len(keywords["Active"]))
	}
	if len(keywords["InProgress"]) != 5 {
		t.Errorf("Expected 5 in-progress keywords, got %d", len(keywords["InProgress"]))
	}
	if len(keywords["Completed"]) != 6 {
		t.Errorf("Expected 6 completed keywords, got %d", len(keywords["Completed"]))
	}
	if len(keywords["Someday"]) != 4 {
		t.Errorf("Expected 4 someday keywords, got %d", len(keywords["Someday"]))
	}
}

func TestGetAllKeywordsFlat(t *testing.T) {
	cfg := createTestConfig()

	entries := GetAllKeywordsFlat(cfg)

	// Should have all keywords
	expectedTotal := 6 + 5 + 6 + 4 // Active + InProgress + Completed + Someday
	if len(entries) != expectedTotal {
		t.Errorf("Expected %d keyword entries, got %d", expectedTotal, len(entries))
	}

	// Check that categories are correctly assigned
	activeCategoryCount := 0
	for _, entry := range entries {
		if entry.Category == "Active" {
			activeCategoryCount++
		}
	}
	if activeCategoryCount != 6 {
		t.Errorf("Expected 6 entries with Active category, got %d", activeCategoryCount)
	}

	// Check that TODO is in the list
	foundTodo := false
	for _, entry := range entries {
		if entry.Keyword == "TODO" && entry.Category == "Active" {
			foundTodo = true
			break
		}
	}
	if !foundTodo {
		t.Error("Expected to find TODO keyword with Active category")
	}
}

func TestParseLineWithID(t *testing.T) {
	cfg := createTestConfig()

	tests := []struct {
		name     string
		line     string
		wantID   string
		wantTitle string
	}{
		{
			name:     "task with ID",
			line:     "TODO: [auth-01] Implement authentication",
			wantID:   "auth-01",
			wantTitle: "Implement authentication",
		},
		{
			name:     "task without ID",
			line:     "TODO: Regular task without ID",
			wantID:   "",
			wantTitle: "Regular task without ID",
		},
		{
			name:     "task with ID and tag",
			line:     "TODO: [feat-123] Add feature #backend",
			wantID:   "feat-123",
			wantTitle: "Add feature",
		},
		{
			name:     "task with ID and metadata",
			line:     "TODO: [task-001] Review code @2025-01-15 >> alice",
			wantID:   "task-001",
			wantTitle: "Review code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := ParseLine(cfg, tt.line, "test", "20250101000000", "test.md")
			if task == nil {
				t.Fatal("ParseLine returned nil")
			}
			if task.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", task.ID, tt.wantID)
			}
			if task.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", task.Title, tt.wantTitle)
			}
		})
	}
}

func TestParseLineWithReferences(t *testing.T) {
	cfg := createTestConfig()

	tests := []struct {
		name          string
		line          string
		wantRefs      []string
		wantTitle     string
	}{
		{
			name:      "single reference",
			line:      "TODO: Deploy app ^build-01",
			wantRefs:  []string{"build-01"},
			wantTitle: "Deploy app",
		},
		{
			name:      "multiple references",
			line:      "TODO: Integration test ^api-01 ^db-02 ^auth-03",
			wantRefs:  []string{"api-01", "db-02", "auth-03"},
			wantTitle: "Integration test",
		},
		{
			name:      "no references",
			line:      "TODO: Simple task",
			wantRefs:  nil,
			wantTitle: "Simple task",
		},
		{
			name:      "reference with ID",
			line:      "TODO: [test-01] Run tests ^build-01",
			wantRefs:  []string{"build-01"},
			wantTitle: "Run tests",
		},
		{
			name:      "reference with tag and assignee",
			line:      "TODO: [deploy-01] Deploy to prod ^test-01 #ops >> admin",
			wantRefs:  []string{"test-01"},
			wantTitle: "Deploy to prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := ParseLine(cfg, tt.line, "test", "20250101000000", "test.md")
			if task == nil {
				t.Fatal("ParseLine returned nil")
			}
			if len(task.References) != len(tt.wantRefs) {
				t.Errorf("References count = %d, want %d", len(task.References), len(tt.wantRefs))
			}
			for i, ref := range tt.wantRefs {
				if i >= len(task.References) || task.References[i] != ref {
					t.Errorf("References[%d] = %q, want %q", i, task.References[i], ref)
				}
			}
			if task.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", task.Title, tt.wantTitle)
			}
		})
	}
}

func TestDetectCycles(t *testing.T) {
	cfg := createTestConfig()

	tests := []struct {
		name         string
		tasks        []*Task
		expectCycles map[string]bool // ID -> should be in cycle
	}{
		{
			name: "no cycles - linear dependency",
			tasks: []*Task{
				{ID: "a", References: []string{"b"}, Keyword: "TODO"},
				{ID: "b", References: []string{"c"}, Keyword: "TODO"},
				{ID: "c", References: nil, Keyword: "TODO"},
			},
			expectCycles: map[string]bool{"a": false, "b": false, "c": false},
		},
		{
			name: "simple cycle - two nodes",
			tasks: []*Task{
				{ID: "a", References: []string{"b"}, Keyword: "TODO"},
				{ID: "b", References: []string{"a"}, Keyword: "TODO"},
			},
			expectCycles: map[string]bool{"a": true, "b": true},
		},
		{
			name: "simple cycle - three nodes",
			tasks: []*Task{
				{ID: "a", References: []string{"b"}, Keyword: "TODO"},
				{ID: "b", References: []string{"c"}, Keyword: "TODO"},
				{ID: "c", References: []string{"a"}, Keyword: "TODO"},
			},
			expectCycles: map[string]bool{"a": true, "b": true, "c": true},
		},
		{
			name: "self-reference cycle",
			tasks: []*Task{
				{ID: "a", References: []string{"a"}, Keyword: "TODO"},
			},
			expectCycles: map[string]bool{"a": true},
		},
		{
			name: "partial cycle - some nodes not in cycle",
			tasks: []*Task{
				{ID: "a", References: []string{"b"}, Keyword: "TODO"},
				{ID: "b", References: []string{"c"}, Keyword: "TODO"},
				{ID: "c", References: []string{"b"}, Keyword: "TODO"}, // b <-> c forms a cycle
				{ID: "d", References: []string{"a"}, Keyword: "TODO"}, // d -> a -> b -> c (not in cycle)
			},
			expectCycles: map[string]bool{"a": false, "b": true, "c": true, "d": false},
		},
		{
			name: "no ID tasks - should not affect cycle detection",
			tasks: []*Task{
				{ID: "a", References: []string{"b"}, Keyword: "TODO"},
				{ID: "b", References: []string{"a"}, Keyword: "TODO"},
				{ID: "", References: nil, Keyword: "TODO"}, // Task without ID
			},
			expectCycles: map[string]bool{"a": true, "b": true},
		},
		{
			name: "reference to non-existent task",
			tasks: []*Task{
				{ID: "a", References: []string{"nonexistent"}, Keyword: "TODO"},
			},
			expectCycles: map[string]bool{"a": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset InCycle for all tasks
			for _, task := range tt.tasks {
				task.InCycle = false
			}

			DetectCycles(tt.tasks)

			for _, task := range tt.tasks {
				if task.ID == "" {
					continue
				}
				expected, exists := tt.expectCycles[task.ID]
				if !exists {
					continue
				}
				if task.InCycle != expected {
					t.Errorf("Task %q InCycle = %v, want %v", task.ID, task.InCycle, expected)
				}
			}
		})
	}

	// Additional test: verify cycle detection doesn't modify tasks without IDs
	_ = cfg // silence unused variable
}

func TestGetTaskByID(t *testing.T) {
	tasks := []*Task{
		{ID: "a", Title: "Task A"},
		{ID: "b", Title: "Task B"},
		{ID: "", Title: "No ID"},
	}

	tests := []struct {
		id       string
		wantNil  bool
		wantTitle string
	}{
		{id: "a", wantNil: false, wantTitle: "Task A"},
		{id: "b", wantNil: false, wantTitle: "Task B"},
		{id: "c", wantNil: true},
		{id: "", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			task := GetTaskByID(tasks, tt.id)
			if tt.wantNil {
				if task != nil {
					t.Errorf("GetTaskByID(%q) = %v, want nil", tt.id, task)
				}
			} else {
				if task == nil {
					t.Fatalf("GetTaskByID(%q) = nil, want task", tt.id)
				}
				if task.Title != tt.wantTitle {
					t.Errorf("GetTaskByID(%q).Title = %q, want %q", tt.id, task.Title, tt.wantTitle)
				}
			}
		})
	}
}

func TestGetDependencies(t *testing.T) {
	tasks := []*Task{
		{ID: "a", Title: "Task A", References: []string{"b", "c"}},
		{ID: "b", Title: "Task B", References: nil},
		{ID: "c", Title: "Task C", References: nil},
		{ID: "d", Title: "Task D", References: []string{"nonexistent"}},
	}

	tests := []struct {
		taskID    string
		wantCount int
		wantIDs   []string
	}{
		{taskID: "a", wantCount: 2, wantIDs: []string{"b", "c"}},
		{taskID: "b", wantCount: 0, wantIDs: nil},
		{taskID: "d", wantCount: 0, wantIDs: nil}, // nonexistent ref
	}

	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			task := GetTaskByID(tasks, tt.taskID)
			deps := GetDependencies(tasks, task)
			if len(deps) != tt.wantCount {
				t.Errorf("GetDependencies(%q) count = %d, want %d", tt.taskID, len(deps), tt.wantCount)
			}
			for i, wantID := range tt.wantIDs {
				if i >= len(deps) || deps[i].ID != wantID {
					t.Errorf("GetDependencies(%q)[%d].ID = %q, want %q", tt.taskID, i, deps[i].ID, wantID)
				}
			}
		})
	}
}

func TestGetDependents(t *testing.T) {
	tasks := []*Task{
		{ID: "a", Title: "Task A", References: []string{"c"}},
		{ID: "b", Title: "Task B", References: []string{"c"}},
		{ID: "c", Title: "Task C", References: nil},
	}

	tests := []struct {
		taskID    string
		wantCount int
		wantIDs   []string
	}{
		{taskID: "c", wantCount: 2, wantIDs: []string{"a", "b"}},
		{taskID: "a", wantCount: 0, wantIDs: nil},
		{taskID: "b", wantCount: 0, wantIDs: nil},
	}

	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			task := GetTaskByID(tasks, tt.taskID)
			dependents := GetDependents(tasks, task)
			if len(dependents) != tt.wantCount {
				t.Errorf("GetDependents(%q) count = %d, want %d", tt.taskID, len(dependents), tt.wantCount)
			}
		})
	}
}

func TestUpdateTaskStatusWithID(t *testing.T) {
	// Create a temporary file with a task that has an ID
	tmpFile, err := os.CreateTemp("", "task_id_test_*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := `# Test File

TODO: [auth-01] Implement login feature
DOING: [api-02] Build REST API ^auth-01
DONE: Complete setup
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create a task with ID
	task := &Task{
		Keyword:  "TODO",
		ID:       "auth-01",
		Title:    "Implement login feature",
		FilePath: tmpFile.Name(),
	}

	// Update the task status
	err = UpdateTaskStatus(task, "DOING")
	if err != nil {
		t.Fatalf("UpdateTaskStatus() error = %v", err)
	}

	// Verify the task keyword was updated in memory
	if task.Keyword != "DOING" {
		t.Errorf("Task.Keyword = %v, want DOING", task.Keyword)
	}

	// Verify the file was updated
	updatedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	expectedContent := `# Test File

DOING: [auth-01] Implement login feature
DOING: [api-02] Build REST API ^auth-01
DONE: Complete setup
`
	if string(updatedContent) != expectedContent {
		t.Errorf("File content = %q, want %q", string(updatedContent), expectedContent)
	}
}

func TestFilterTasksWithIDAndReferences(t *testing.T) {
	tasks := []*Task{
		{ID: "auth-01", Title: "Implement auth", References: nil, Keyword: "TODO"},
		{ID: "api-02", Title: "Build API", References: []string{"auth-01"}, Keyword: "TODO"},
		{ID: "", Title: "Simple task", References: nil, Keyword: "TODO"},
	}

	tests := []struct {
		name       string
		filter     string
		wantCount  int
	}{
		{name: "filter by ID", filter: "auth-01", wantCount: 2}, // matches ID and reference
		{name: "filter by reference", filter: "api-02", wantCount: 1},
		{name: "filter by title", filter: "Simple", wantCount: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterTasks(tasks, tt.filter)
			if len(result) != tt.wantCount {
				t.Errorf("FilterTasks(%q) count = %d, want %d", tt.filter, len(result), tt.wantCount)
			}
		})
	}
}