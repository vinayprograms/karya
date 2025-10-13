package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrerequisitesCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rnr_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	file := filepath.Join(tempDir, "reading-list.md")
	prerequisitesCheck(file)

	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Error("File not created")
	}
}

func TestWebpageTitle(t *testing.T) {
	// Mock test, hard to test http
	title := webpageTitle("https://example.com")
	if title == "" {
		t.Error("Title should not be empty")
	}
}

func TestReadNReview(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rnr_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	file := filepath.Join(tempDir, "reading-list.md")
	// Create file
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	readNReview("https://example.com", file)

	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Example Domain") {
		t.Errorf("Unexpected content: %s", content)
	}
}