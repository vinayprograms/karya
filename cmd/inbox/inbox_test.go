package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vinayprograms/karya/internal/config"
)

func TestInboxConfig(t *testing.T) {
	// Test that config loads without error
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test that we can determine inbox file path
	var inboxFile string
	if cfg.Directories.Karya != "" {
		inboxFile = filepath.Join(cfg.Directories.Karya, "inbox.md")
	} else {
		home, _ := os.UserHomeDir()
		inboxFile = filepath.Join(home, "inbox.md")
	}

	// Verify the path format
	expectedPattern := "inbox.md"
	if filepath.Base(inboxFile) != expectedPattern {
		t.Errorf("Expected inbox file to end with '%s', got '%s'", expectedPattern, filepath.Base(inboxFile))
	}
}