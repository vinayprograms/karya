package main

import (
	"io/ioutil"
	"os"
	"testing"
	"todo-toolkit/internal/task"
)

func TestTask_IsActive(t *testing.T) {
	tests := []struct {
		keyword string
		want    bool
	}{
		{"TODO", true},
		{"DONE", false},
		{"INVALID", false},
	}
	for _, tt := range tests {
		tk := &task.Task{Keyword: tt.keyword}
		if got := tk.IsActive(); got != tt.want {
			t.Errorf("Task.IsActive() = %v, want %v", got, tt.want)
		}
	}
}

func TestTask_IsCompleted(t *testing.T) {
	tests := []struct {
		keyword string
		want    bool
	}{
		{"DONE", true},
		{"TODO", false},
		{"INVALID", false},
	}
	for _, tt := range tests {
		tk := &task.Task{Keyword: tt.keyword}
		if got := tk.IsCompleted(); got != tt.want {
			t.Errorf("Task.IsCompleted() = %v, want %v", got, tt.want)
		}
	}
}

func TestParseLine(t *testing.T) {
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
				Keyword:  "TODO",
				Title:    "Write documentation",
				Tag:      "urgent",
				Date:     "2023-10-01",
				Assignee: "john",
				Project:  "proj1",
				Zettel:   "20231001120000",
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
		got := task.ParseLine(tt.line, tt.project, tt.zettel)
		if (got == nil && tt.want != nil) || (got != nil && tt.want == nil) {
			t.Errorf("ParseLine(%q, %q, %q) = %v, want %v", tt.line, tt.project, tt.zettel, got, tt.want)
			continue
		}
		if got != nil && tt.want != nil {
			if *got != *tt.want {
				t.Errorf("ParseLine(%q, %q, %q) = %v, want %v", tt.line, tt.project, tt.zettel, got, tt.want)
			}
		}
	}
}

func TestProcessFile(t *testing.T) {
	// Create temp file
	content := `TODO: Write code #urgent @2023-10-01 >> john
DONE: Completed task
INVALID: Skip this`
	tempFile, err := ioutil.TempFile("", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.WriteString(content)
	tempFile.Close()

	tasks, err := task.ProcessFile(tempFile.Name())
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