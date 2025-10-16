package main

import (
	"github.com/vinayprograms/karya/internal/task"
	"os"
	"testing"
)

func TestTask_IsActive(t *testing.T) {
	config := task.NewConfig()
	tests := []struct {
		keyword string
		want    bool
	}{
		{"TODO", true},
		{"DONE", false},
		{"INVALID", false},
	}
	for _, tt := range tests {
		tk := config.ParseLine(tt.keyword+": test", "proj", "zettel", "test.md")
		if tk == nil {
			if tt.want {
				t.Errorf("Task.IsActive() = nil, want %v", tt.want)
			}
			continue
		}
		if got := tk.IsActive(); got != tt.want {
			t.Errorf("Task.IsActive() = %v, want %v", got, tt.want)
		}
	}
}

func TestTask_IsCompleted(t *testing.T) {
	config := task.NewConfig()
	tests := []struct {
		keyword string
		want    bool
	}{
		{"DONE", true},
		{"TODO", false},
		{"INVALID", false},
	}
	for _, tt := range tests {
		tk := config.ParseLine(tt.keyword+": test", "proj", "zettel", "test.md")
		if tk == nil {
			if tt.want {
				t.Errorf("Task.IsCompleted() = nil, want %v", tt.want)
			}
			continue
		}
		if got := tk.IsCompleted(); got != tt.want {
			t.Errorf("Task.IsCompleted() = %v, want %v", got, tt.want)
		}
	}
}

func TestParseLine(t *testing.T) {
	config := task.NewConfig()
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
	}
	for _, tt := range tests {
		got := config.ParseLine(tt.line, tt.project, tt.zettel, "test.md")
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
	config := task.NewConfig()
	content := `TODO: Write code #urgent @2023-10-01 >> john
DONE: Completed task
INVALID: Skip this`
	tempFile, err := os.CreateTemp("", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.WriteString(content)
	tempFile.Close()

	tasks, err := config.ProcessFile(tempFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Keyword != "TODO" || tasks[1].Keyword != "DONE" {
		t.Errorf("Unexpected tasks: %+v %+v", tasks[0], tasks[1])
	}
}

func TestListTasks(t *testing.T) {
	// Mock config with non-existent dir
	config := &task.Config{PRJDIR: "/nonexistent"}
	tasks, err := config.ListTasks("", true)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}
}
