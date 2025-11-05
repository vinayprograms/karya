package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/task"
)

func createTestConfig() *config.Config {
	return &config.Config{
		Todo: config.Todo{
			Active: []string{"TODO", "TASK"},
			InProgress: []string{"DOING", "WIP"},
			Completed: []string{"DONE", "COMPLETED"},
			Someday: []string{"SOMEDAY", "MAYBE"},
		},
	}
}

func TestTask_IsActive(t *testing.T) {
	cfg := createTestConfig()
	tests := []struct {
		keyword string
		want    bool
	}{
		{"TODO", true},
		{"DONE", false},
		{"SOMEDAY", false},
		{"INVALID", false},
	}
	for _, tt := range tests {
		tk := task.ParseLine(cfg, tt.keyword+": test", "proj", "zettel", "test.md")
		if tk == nil {
			if tt.want {
				t.Errorf("Task.IsActive() = nil, want %v", tt.want)
			}
			continue
		}
		if got := tk.IsActive(cfg); got != tt.want {
			t.Errorf("Task.IsActive() = %v, want %v", got, tt.want)
		}
	}
}

func TestTask_IsCompleted(t *testing.T) {
	cfg := createTestConfig()
	tests := []struct {
		keyword string
		want    bool
	}{
		{"DONE", true},
		{"TODO", false},
		{"SOMEDAY", false},
		{"INVALID", false},
	}
	for _, tt := range tests {
		tk := task.ParseLine(cfg, tt.keyword+": test", "proj", "zettel", "test.md")
		if tk == nil {
			if tt.want {
				t.Errorf("Task.IsCompleted() = nil, want %v", tt.want)
			}
			continue
		}
		if got := tk.IsCompleted(cfg); got != tt.want {
			t.Errorf("Task.IsCompleted() = %v, want %v", got, tt.want)
		}
	}
}

func TestTask_IsSomeday(t *testing.T) {
	cfg := createTestConfig()
	tests := []struct {
		keyword string
		want    bool
	}{
		{"SOMEDAY", true},
		{"MAYBE", true},
		{"TODO", false},
		{"DONE", false},
		{"INVALID", false},
	}
	for _, tt := range tests {
		tk := task.ParseLine(cfg, tt.keyword+": test", "proj", "zettel", "test.md")
		if tk == nil {
			if tt.want {
				t.Errorf("Task.IsSomeday() = nil, want %v", tt.want)
			}
			continue
		}
		if got := tk.IsSomeday(cfg); got != tt.want {
			t.Errorf("Task.IsSomeday() = %v, want %v", got, tt.want)
		}
	}
}

func TestParseLine(t *testing.T) {
	cfg := createTestConfig()
	tests := []struct {
		line    string
		project string
		zettel  string
		want    *task.Task
	}{
		{
			line:    "TODO: Write documentation #urgent @2023-10-01 >> john",
			project: "proj1",
			zettel:  "20231001120000",
			want: &task.Task{
				Keyword:     "TODO",
				Title:       "Write documentation",
				Tag:         "urgent",
				ScheduledAt: "2023-10-01",
				Assignee:    "john",
				Project:     "proj1",
				Zettel:      "20231001120000",
			},
		},
		{
			line:    "DONE: Task completed",
			project: "proj2",
			zettel:  "20231001130000",
			want: &task.Task{
				Keyword: "DONE",
				Title:   "Task completed",
				Project: "proj2",
				Zettel:  "20231001130000",
			},
		},
		{
			line:    "INVALID: This should not parse",
			project: "proj3",
			zettel:  "zettel",
			want:    nil,
		},
		{
			line:    "TODO: Task without extras",
			project: "proj4",
			zettel:  "z",
			want: &task.Task{
				Keyword: "TODO",
				Title:   "Task without extras",
				Project: "proj4",
				Zettel:  "z",
			},
		},
		{
			line:    "SOMEDAY: Learn Go deeply #learning @2024-06-01",
			project: "personal",
			zettel:  "20240201000000",
			want: &task.Task{
				Keyword:     "SOMEDAY",
				Title:       "Learn Go deeply",
				Tag:         "learning",
				ScheduledAt: "2024-06-01",
				Project:     "personal",
				Zettel:      "20240201000000",
			},
		},
	}
	for _, tt := range tests {
		got := task.ParseLine(cfg, tt.line, tt.project, tt.zettel, "test.md")
		if (got == nil && tt.want != nil) || (got != nil && tt.want == nil) {
			t.Errorf("ParseLine(%q, %q, %q, %q) = %v, want %v", tt.line, tt.project, tt.zettel, "test.md", got, tt.want)
			continue
		}
		if got != nil && tt.want != nil {
			if got.Keyword != tt.want.Keyword || got.Title != tt.want.Title || got.Tag != tt.want.Tag || got.ScheduledAt != tt.want.ScheduledAt || got.Assignee != tt.want.Assignee || got.Project != tt.want.Project || got.Zettel != tt.want.Zettel {
				t.Errorf("ParseLine(%q, %q, %q, %q) = %v, want %v", tt.line, tt.project, tt.zettel, "test.md", got, tt.want)
			}
		}
	}
}

func TestProcessFile(t *testing.T) {
	cfg := createTestConfig()
	
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "testproject", "notes", "20240101000000")
	err := os.MkdirAll(projectDir, 0755)
	if err != nil {
		t.Fatal(err)
	}
	
	// Set the projects directory in config
	cfg.Directories.Projects = tmpDir
	cfg.Todo.Structured = true
	
	content := `TODO: Write code #urgent @2023-10-01 >> john
DONE: Completed task
SOMEDAY: Learn new language
INVALID: Skip this`
	
	filePath := filepath.Join(projectDir, "README.md")
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := task.ProcessFile(cfg, filePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
	
	expectedKeywords := []string{"TODO", "DONE", "SOMEDAY"}
	for i, expectedKeyword := range expectedKeywords {
		if i >= len(tasks) || tasks[i].Keyword != expectedKeyword {
			t.Errorf("Expected task %d to have keyword %s, got %v", i, expectedKeyword, tasks[i])
		}
	}
}

func TestListTasks(t *testing.T) {
	cfg := createTestConfig()
	// Create empty temp directory
	tmpDir := t.TempDir()
	cfg.Directories.Projects = tmpDir
	cfg.Todo.Structured = true
	
	tasks, err := task.ListTasks(cfg, "", true)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}
}

func TestTaskItemMarkdownRendering(t *testing.T) {
	cfg := createTestConfig()
	
	// Initialize colors
	InitializeColors(cfg)
	
	task := &task.Task{
		Keyword: "TODO",
		Title:   "This is **bold** and *italic* text with `code` and ~~strikethrough~~",
		Project: "test",
		Zettel:  "12345678901234",
	}
	
	item := taskItem{
		config:          cfg,
		task:            task,
		projectColWidth: 10,
		keywordColWidth: 10,
		verbose:         false,
	}
	
	// Test Title rendering
	title := item.Title()
	
	// Check that markdown syntax is not present in the output
	if strings.Contains(title, "**") || strings.Contains(title, "*") || 
	   strings.Contains(title, "~~") || strings.Contains(title, "`") {
		t.Errorf("Title() should remove markdown syntax, got %v", title)
	}
	
	// Test renderWithSelection
	rendered := item.renderWithSelection(false)
	
	// Check that markdown syntax is not present in the output
	if strings.Contains(rendered, "**") || strings.Contains(rendered, "*") || 
	   strings.Contains(rendered, "~~") || strings.Contains(rendered, "`") {
		t.Errorf("renderWithSelection() should remove markdown syntax, got %v", rendered)
	}
}
