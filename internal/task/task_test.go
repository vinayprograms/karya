package task

import (
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