package zet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMCPServerTools(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "zet-mcp-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the MCP server
	server := NewMCPServer(tmpDir)
	if server == nil {
		t.Fatal("NewMCPServer returned nil")
	}

	// Verify zetDir is set correctly
	if server.zetDir != tmpDir {
		t.Errorf("zetDir = %q, want %q", server.zetDir, tmpDir)
	}
}

func TestReadWriteZettelContent(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "zet-content-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a zettel
	zetID := "20240101120000"
	title := "Test Zettel"
	if err := CreateZettel(tmpDir, zetID, title); err != nil {
		t.Fatal(err)
	}

	// Read initial content
	content, err := ReadZettelContent(tmpDir, zetID)
	if err != nil {
		t.Fatalf("ReadZettelContent failed: %v", err)
	}
	expectedContent := "# Test Zettel\n\n\n"
	if content != expectedContent {
		t.Errorf("ReadZettelContent = %q, want %q", content, expectedContent)
	}

	// Write new content
	newContent := "# Updated Title\n\nThis is new content.\n"
	if err := WriteZettelContent(tmpDir, zetID, newContent); err != nil {
		t.Fatalf("WriteZettelContent failed: %v", err)
	}

	// Read again and verify
	content, err = ReadZettelContent(tmpDir, zetID)
	if err != nil {
		t.Fatalf("ReadZettelContent after write failed: %v", err)
	}
	if content != newContent {
		t.Errorf("ReadZettelContent after write = %q, want %q", content, newContent)
	}
}

func TestReadZettelContentNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zet-read-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Try to read a non-existent zettel
	_, err = ReadZettelContent(tmpDir, "99999999999999")
	if err == nil {
		t.Error("ReadZettelContent should fail for non-existent zettel")
	}
}

func TestWriteZettelContentNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zet-write-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Try to write to a non-existent zettel (directory doesn't exist)
	err = WriteZettelContent(tmpDir, "99999999999999", "content")
	if err == nil {
		t.Error("WriteZettelContent should fail for non-existent zettel directory")
	}
}

func TestMCPServerZetDir(t *testing.T) {
	testCases := []struct {
		name   string
		zetDir string
	}{
		{"simple path", "/tmp/zet"},
		{"path with spaces", "/tmp/my zettels"},
		{"nested path", "/home/user/documents/notes/zet"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewMCPServer(tc.zetDir)
			if server.zetDir != tc.zetDir {
				t.Errorf("zetDir = %q, want %q", server.zetDir, tc.zetDir)
			}
		})
	}
}

func TestDeleteZettelWithContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "zet-delete-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a zettel
	zetID := "20240101120000"
	if err := CreateZettel(tmpDir, zetID, "Test"); err != nil {
		t.Fatal(err)
	}

	// Verify it exists
	zetPath := filepath.Join(tmpDir, zetID, "README.md")
	if _, err := os.Stat(zetPath); os.IsNotExist(err) {
		t.Fatal("zettel should exist after creation")
	}

	// Delete it
	if err := DeleteZettel(tmpDir, zetID); err != nil {
		t.Fatalf("DeleteZettel failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(zetPath); !os.IsNotExist(err) {
		t.Error("zettel should not exist after deletion")
	}
}
