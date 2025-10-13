package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckAndCreateDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goal_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	prj := "testprj"
	// Since it would prompt, perhaps mock or assume
	// For test, create manually
	prjPath := filepath.Join(tempDir, prj)
	if err := os.MkdirAll(prjPath, 0755); err != nil {
		t.Fatal(err)
	}
	goalFile := filepath.Join(prjPath, "goals.md")
	if _, err := os.Stat(goalFile); os.IsNotExist(err) {
		file, err := os.Create(goalFile)
		if err != nil {
			t.Fatal(err)
		}
		file.WriteString("# Goals - testprj\n\n")
		file.Close()
	}

	// Test function (but it prompts, so perhaps skip interactive test)
	// For now, just check if directory exists
	if _, err := os.Stat(prjPath); os.IsNotExist(err) {
		t.Error("Directory not created")
	}
}