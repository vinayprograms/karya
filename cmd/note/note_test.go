package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListProjects(t *testing.T) {
	tmpDir := t.TempDir()
	
	projects := []string{"project1", "project2", "project3"}
	for _, prj := range projects {
		prjPath := filepath.Join(tmpDir, prj)
		if err := os.MkdirAll(prjPath, 0755); err != nil {
			t.Fatalf("Failed to create test project: %v", err)
		}
	}
	
	result, err := listProjects(tmpDir)
	if err != nil {
		t.Fatalf("listProjects failed: %v", err)
	}
	
	if len(result) != len(projects) {
		t.Errorf("Expected %d projects, got %d", len(projects), len(result))
	}
	
	for i, prj := range projects {
		if result[i].Name != prj {
			t.Errorf("Expected project %s, got %s", prj, result[i].Name)
		}
	}
}

func TestListProjectsWithNotes(t *testing.T) {
	tmpDir := t.TempDir()
	
	prjWithNotes := filepath.Join(tmpDir, "project1")
	prjWithoutNotes := filepath.Join(tmpDir, "project2")
	
	if err := os.MkdirAll(prjWithNotes, 0755); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(prjWithNotes, "notes"), 0755); err != nil {
		t.Fatalf("Failed to create notes dir: %v", err)
	}
	if err := os.MkdirAll(prjWithoutNotes, 0755); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	
	result, err := listProjects(tmpDir)
	if err != nil {
		t.Fatalf("listProjects failed: %v", err)
	}
	
	if len(result) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(result))
	}
	
	for _, prj := range result {
		if prj.Name == "project1" && !prj.HasNotes {
			t.Errorf("Expected project1 to have notes")
		}
		if prj.Name == "project2" && prj.HasNotes {
			t.Errorf("Expected project2 to not have notes")
		}
	}
}

func TestCheckProjectDir(t *testing.T) {
	tmpDir := t.TempDir()
	prjName := "testproject"
	
	exists, err := checkProjectDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("checkProjectDir failed: %v", err)
	}
	if exists {
		t.Errorf("Expected project to not exist")
	}
	
	prjPath := filepath.Join(tmpDir, prjName)
	if err := os.MkdirAll(prjPath, 0755); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	
	exists, err = checkProjectDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("checkProjectDir failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected project to exist")
	}
}

func TestCheckNotesDir(t *testing.T) {
	tmpDir := t.TempDir()
	prjName := "testproject"
	prjPath := filepath.Join(tmpDir, prjName)
	
	if err := os.MkdirAll(prjPath, 0755); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	
	exists, err := checkNotesDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("checkNotesDir failed: %v", err)
	}
	if exists {
		t.Errorf("Expected notes dir to not exist")
	}
	
	notesPath := filepath.Join(prjPath, "notes")
	if err := os.MkdirAll(notesPath, 0755); err != nil {
		t.Fatalf("Failed to create notes dir: %v", err)
	}
	
	exists, err = checkNotesDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("checkNotesDir failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected notes dir to exist")
	}
}

func TestCreateProjectDir(t *testing.T) {
	tmpDir := t.TempDir()
	prjName := "newproject"
	
	err := createProjectDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("createProjectDir failed: %v", err)
	}
	
	prjPath := filepath.Join(tmpDir, prjName)
	if _, err := os.Stat(prjPath); os.IsNotExist(err) {
		t.Errorf("Expected project directory to be created")
	}
}

func TestCreateNotesDir(t *testing.T) {
	tmpDir := t.TempDir()
	prjName := "testproject"
	prjPath := filepath.Join(tmpDir, prjName)
	
	if err := os.MkdirAll(prjPath, 0755); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	
	err := createNotesDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("createNotesDir failed: %v", err)
	}
	
	notesPath := filepath.Join(prjPath, "notes")
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		t.Errorf("Expected notes directory to be created")
	}
	
	gitPath := filepath.Join(notesPath, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		t.Errorf("Expected git repository to be initialized")
	}
}

func TestGetNotesDir(t *testing.T) {
	prjDir := "/path/to/projects"
	prjName := "myproject"
	
	expected := filepath.Join(prjDir, prjName, "notes")
	result := getNotesDir(prjDir, prjName)
	
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestListProjectsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	
	result, err := listProjects(tmpDir)
	if err != nil {
		t.Fatalf("listProjects failed: %v", err)
	}
	
	if len(result) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(result))
	}
}

func TestListProjectsIgnoresHidden(t *testing.T) {
	tmpDir := t.TempDir()
	
	visibleProject := filepath.Join(tmpDir, "visible")
	hiddenProject := filepath.Join(tmpDir, ".hidden")
	
	if err := os.MkdirAll(visibleProject, 0755); err != nil {
		t.Fatalf("Failed to create visible project: %v", err)
	}
	if err := os.MkdirAll(hiddenProject, 0755); err != nil {
		t.Fatalf("Failed to create hidden project: %v", err)
	}
	
	result, err := listProjects(tmpDir)
	if err != nil {
		t.Fatalf("listProjects failed: %v", err)
	}
	
	if len(result) != 1 {
		t.Errorf("Expected 1 project, got %d", len(result))
	}
	
	if result[0].Name != "visible" {
		t.Errorf("Expected project 'visible', got '%s'", result[0].Name)
	}
}

func TestListProjectsSorted(t *testing.T) {
	tmpDir := t.TempDir()
	
	projects := []string{"zebra", "alpha", "beta"}
	for _, prj := range projects {
		prjPath := filepath.Join(tmpDir, prj)
		if err := os.MkdirAll(prjPath, 0755); err != nil {
			t.Fatalf("Failed to create test project: %v", err)
		}
	}
	
	result, err := listProjects(tmpDir)
	if err != nil {
		t.Fatalf("listProjects failed: %v", err)
	}
	
	expected := []string{"alpha", "beta", "zebra"}
	for i, prj := range expected {
		if result[i].Name != prj {
			t.Errorf("Expected project %s at index %d, got %s", prj, i, result[i].Name)
		}
	}
}

func TestCreateProjectDirAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	prjName := "existingproject"
	prjPath := filepath.Join(tmpDir, prjName)
	
	if err := os.MkdirAll(prjPath, 0755); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	
	err := createProjectDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("createProjectDir should not fail for existing directory: %v", err)
	}
}

func TestCreateNotesDirAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	prjName := "testproject"
	prjPath := filepath.Join(tmpDir, prjName)
	notesPath := filepath.Join(prjPath, "notes")
	
	if err := os.MkdirAll(notesPath, 0755); err != nil {
		t.Fatalf("Failed to create notes dir: %v", err)
	}
	
	err := createNotesDir(tmpDir, prjName)
	if err != nil {
		t.Fatalf("createNotesDir should not fail for existing directory: %v", err)
	}
}
