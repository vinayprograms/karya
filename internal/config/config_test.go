package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigWithDirectories(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `editor = "nvim"
structured = true

[directories]
projects = "/test/projects"
zettelkasten = "/test/zet"
karya = "/test/karya"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set HOME to temp dir so it looks for config there
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create .config/karya directory structure
	configDir := filepath.Join(tmpDir, ".config", "karya")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy config to expected location
	expectedPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(expectedPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tmpDir)

	// Clear environment variables to test config file
	os.Unsetenv("PROJECTS")
	os.Unsetenv("ZETTELKASTEN")
	os.Unsetenv("KARYA")
	os.Unsetenv("EDITOR")

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify [directories] section is loaded
	if cfg.Directories.Projects != "/test/projects" {
		t.Errorf("Expected directories.projects = /test/projects, got %s", cfg.Directories.Projects)
	}

	if cfg.Directories.Zettelkasten != "/test/zet" {
		t.Errorf("Expected directories.zettelkasten = /test/zet, got %s", cfg.Directories.Zettelkasten)
	}

	if cfg.Directories.Karya != "/test/karya" {
		t.Errorf("Expected directories.karya = /test/karya, got %s", cfg.Directories.Karya)
	}

	// Verify other settings
	if cfg.GeneralConfig.EDITOR != "nvim" {
		t.Errorf("Expected editor = nvim, got %s", cfg.GeneralConfig.EDITOR)
	}

	if !cfg.Todo.Structured {
		t.Error("Expected structured = true")
	}
}

func TestEnvironmentVariablesPrecedence(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()

	configContent := `editor = "vim"

[directories]
projects = "/config/projects"
zettelkasten = "/config/zet"
`

	// Set HOME to temp dir
	origHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", origHome)
		os.Unsetenv("PROJECTS")
		os.Unsetenv("ZETTELKASTEN")
		os.Unsetenv("EDITOR")
	}()

	// Create .config/karya directory structure
	configDir := filepath.Join(tmpDir, ".config", "karya")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write config
	expectedPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(expectedPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tmpDir)

	// Set environment variables - these should override config file
	os.Setenv("PROJECTS", "/env/projects")
	os.Setenv("ZETTELKASTEN", "/env/zet")
	os.Setenv("EDITOR", "nvim")

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variables take precedence
	if cfg.Directories.Projects != "/env/projects" {
		t.Errorf("Expected directories.projects from env = /env/projects, got %s", cfg.Directories.Projects)
	}

	if cfg.Directories.Zettelkasten != "/env/zet" {
		t.Errorf("Expected directories.zettelkasten from env = /env/zet, got %s", cfg.Directories.Zettelkasten)
	}

	if cfg.GeneralConfig.EDITOR != "nvim" {
		t.Errorf("Expected EDITOR from env = nvim, got %s", cfg.GeneralConfig.EDITOR)
	}
}
