package note

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/zet"
)

func setupTestMCP(t *testing.T) (*MCPServer, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "note-mcp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		Directories: config.Directories{
			Projects: tmpDir,
		},
		Todo: config.Todo{
			Active:     []string{"TODO", "TASK"},
			InProgress: []string{"DOING", "WIP"},
			Completed:  []string{"DONE", "COMPLETED"},
			Someday:    []string{"SOMEDAY", "MAYBE"},
		},
	}

	server := NewMCPServer(cfg)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return server, tmpDir, cleanup
}

func TestGetLines_ByPattern(t *testing.T) {
	server, tmpDir, cleanup := setupTestMCP(t)
	defer cleanup()

	// Create project and note
	projectName := "test-project"
	projectDir := filepath.Join(tmpDir, projectName, "notes")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	noteID := zet.GenerateZettelID()
	if err := zet.CreateZettel(projectDir, noteID, "Test Note"); err != nil {
		t.Fatalf("failed to create zettel: %v", err)
	}

	// Write test content
	content := `# Test Note

## Introduction
This is an introduction paragraph.
With multiple lines.

TODO: First task
  - subtask 1
  - subtask 2
  - notes about this task

## Section Two
More content here.

TODO: Second task
More notes under second task.
`
	if err := zet.WriteZettelContent(projectDir, noteID, content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}

	ctx := context.Background()

	t.Run("find TODO and get lines after", func(t *testing.T) {
		args := GetLinesArgs{
			Project:    projectName,
			NoteID:     noteID,
			Pattern:    "TODO: First task",
			LinesAfter: 3,
		}

		_, result, err := server.getLines(ctx, nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Count != 1 {
			t.Errorf("expected 1 match, got %d", result.Count)
		}

		if len(result.Matches) == 0 {
			t.Fatal("expected at least one match")
		}

		match := result.Matches[0]
		if match.AnchorText != "TODO: First task" {
			t.Errorf("expected anchor text 'TODO: First task', got %q", match.AnchorText)
		}

		if len(match.LinesAfter) != 3 {
			t.Errorf("expected 3 lines after, got %d", len(match.LinesAfter))
		}

		if match.LinesAfter[0] != "  - subtask 1" {
			t.Errorf("expected first line after to be '  - subtask 1', got %q", match.LinesAfter[0])
		}
	})

	t.Run("find heading and get lines after", func(t *testing.T) {
		args := GetLinesArgs{
			Project:    projectName,
			NoteID:     noteID,
			Pattern:    "## Introduction",
			LinesAfter: 2,
		}

		_, result, err := server.getLines(ctx, nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Count != 1 {
			t.Errorf("expected 1 match, got %d", result.Count)
		}

		match := result.Matches[0]
		if len(match.LinesAfter) != 2 {
			t.Errorf("expected 2 lines after, got %d", len(match.LinesAfter))
		}
	})

	t.Run("find multiple matches", func(t *testing.T) {
		args := GetLinesArgs{
			Project:    projectName,
			NoteID:     noteID,
			Pattern:    "TODO:",
			LinesAfter: 1,
		}

		_, result, err := server.getLines(ctx, nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Count != 2 {
			t.Errorf("expected 2 matches, got %d", result.Count)
		}
	})

	t.Run("get lines before and after", func(t *testing.T) {
		args := GetLinesArgs{
			Project:     projectName,
			NoteID:      noteID,
			Pattern:     "TODO: First task",
			LinesBefore: 2,
			LinesAfter:  2,
		}

		_, result, err := server.getLines(ctx, nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		match := result.Matches[0]
		if len(match.LinesBefore) != 2 {
			t.Errorf("expected 2 lines before, got %d", len(match.LinesBefore))
		}
		if len(match.LinesAfter) != 2 {
			t.Errorf("expected 2 lines after, got %d", len(match.LinesAfter))
		}
	})
}

func TestGetLines_ByLineNumber(t *testing.T) {
	server, tmpDir, cleanup := setupTestMCP(t)
	defer cleanup()

	// Create project and note
	projectName := "test-project"
	projectDir := filepath.Join(tmpDir, projectName, "notes")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	noteID := zet.GenerateZettelID()
	if err := zet.CreateZettel(projectDir, noteID, "Test Note"); err != nil {
		t.Fatalf("failed to create zettel: %v", err)
	}

	content := `Line 1
Line 2
Line 3
Line 4
Line 5
Line 6
Line 7`
	if err := zet.WriteZettelContent(projectDir, noteID, content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}

	ctx := context.Background()

	t.Run("get specific line with context", func(t *testing.T) {
		args := GetLinesArgs{
			Project:     projectName,
			NoteID:      noteID,
			LineNumber:  4,
			LinesBefore: 2,
			LinesAfter:  2,
		}

		_, result, err := server.getLines(ctx, nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Count != 1 {
			t.Errorf("expected 1 match, got %d", result.Count)
		}

		match := result.Matches[0]
		if match.AnchorLine != 4 {
			t.Errorf("expected anchor line 4, got %d", match.AnchorLine)
		}
		if match.AnchorText != "Line 4" {
			t.Errorf("expected anchor text 'Line 4', got %q", match.AnchorText)
		}
		if len(match.LinesBefore) != 2 {
			t.Errorf("expected 2 lines before, got %d", len(match.LinesBefore))
		}
		if len(match.LinesAfter) != 2 {
			t.Errorf("expected 2 lines after, got %d", len(match.LinesAfter))
		}
		if match.StartLine != 2 {
			t.Errorf("expected start line 2, got %d", match.StartLine)
		}
		if match.EndLine != 6 {
			t.Errorf("expected end line 6, got %d", match.EndLine)
		}
	})

	t.Run("boundary handling at start", func(t *testing.T) {
		args := GetLinesArgs{
			Project:     projectName,
			NoteID:      noteID,
			LineNumber:  1,
			LinesBefore: 5, // More than available
			LinesAfter:  2,
		}

		_, result, err := server.getLines(ctx, nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		match := result.Matches[0]
		if len(match.LinesBefore) != 0 {
			t.Errorf("expected 0 lines before (at start), got %d", len(match.LinesBefore))
		}
		if match.StartLine != 1 {
			t.Errorf("expected start line 1, got %d", match.StartLine)
		}
	})

	t.Run("boundary handling at end", func(t *testing.T) {
		args := GetLinesArgs{
			Project:     projectName,
			NoteID:      noteID,
			LineNumber:  7,
			LinesBefore: 2,
			LinesAfter:  5, // More than available
		}

		_, result, err := server.getLines(ctx, nil, args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		match := result.Matches[0]
		if len(match.LinesAfter) != 0 {
			t.Errorf("expected 0 lines after (at end), got %d", len(match.LinesAfter))
		}
		if match.EndLine != 7 {
			t.Errorf("expected end line 7, got %d", match.EndLine)
		}
	})

	t.Run("line number out of bounds", func(t *testing.T) {
		args := GetLinesArgs{
			Project:    projectName,
			NoteID:     noteID,
			LineNumber: 100,
		}

		_, _, err := server.getLines(ctx, nil, args)
		if err == nil {
			t.Error("expected error for out of bounds line number")
		}
	})
}

func TestGetLines_Validation(t *testing.T) {
	server, _, cleanup := setupTestMCP(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("missing project", func(t *testing.T) {
		args := GetLinesArgs{
			NoteID:  "123",
			Pattern: "test",
		}

		_, _, err := server.getLines(ctx, nil, args)
		if err == nil {
			t.Error("expected error for missing project")
		}
	})

	t.Run("missing note_id", func(t *testing.T) {
		args := GetLinesArgs{
			Project: "test",
			Pattern: "test",
		}

		_, _, err := server.getLines(ctx, nil, args)
		if err == nil {
			t.Error("expected error for missing note_id")
		}
	})

	t.Run("missing pattern and line_number", func(t *testing.T) {
		args := GetLinesArgs{
			Project: "test",
			NoteID:  "123",
		}

		_, _, err := server.getLines(ctx, nil, args)
		if err == nil {
			t.Error("expected error when both pattern and line_number are missing")
		}
	})
}

func TestGetLines_CaseInsensitiveSearch(t *testing.T) {
	server, tmpDir, cleanup := setupTestMCP(t)
	defer cleanup()

	projectName := "test-project"
	projectDir := filepath.Join(tmpDir, projectName, "notes")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	noteID := zet.GenerateZettelID()
	if err := zet.CreateZettel(projectDir, noteID, "Test Note"); err != nil {
		t.Fatalf("failed to create zettel: %v", err)
	}

	content := `TODO: Important task
todo: lowercase task
Todo: Mixed case task`
	if err := zet.WriteZettelContent(projectDir, noteID, content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}

	ctx := context.Background()

	args := GetLinesArgs{
		Project: projectName,
		NoteID:  noteID,
		Pattern: "todo:",
	}

	_, result, err := server.getLines(ctx, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Count != 3 {
		t.Errorf("expected 3 case-insensitive matches, got %d", result.Count)
	}
}
