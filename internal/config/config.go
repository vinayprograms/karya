package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// colorNameMap maps user-friendly color names to ANSI 16-color values
var colorNameMap = map[string]string{
	// Standard colors (0-7)
	"black":   "0",
	"red":     "1",
	"green":   "2",
	"yellow":  "3",
	"blue":    "4",
	"magenta": "5",
	"cyan":    "6",
	"white":   "7",
	// Bright colors (8-15)
	"bright-black":   "8",
	"gray":           "8", // alias for bright-black
	"bright-red":     "9",
	"bright-green":   "10",
	"bright-yellow":  "11",
	"bright-blue":    "12",
	"bright-magenta": "13",
	"bright-cyan":    "14",
	"bright-white":   "15",
}

// resolveColorValue converts color names to ANSI 16-color numbers, or passes through other formats
// Accepts:
//   - Color names (red, bright-blue, etc.) → converted to ANSI numbers (0-15)
//   - ANSI numbers (0-15) → returned as-is
//   - Hex colors (#RRGGBB) → passed through to lipgloss (supports full RGB range)
//   - 256-color codes → passed through to lipgloss
func resolveColorValue(colorInput string) string {
	if colorInput == "" {
		return colorInput
	}

	// Try as color name first - only names get converted to ANSI
	if ansiValue, exists := colorNameMap[strings.ToLower(colorInput)]; exists {
		return ansiValue
	}

	// Everything else (hex, ANSI numbers, 256-color codes) passed through unchanged
	// lipgloss handles validation and rendering
	return colorInput
}

type ColorScheme struct {
	// 16-color palette values (0-15)
	ProjectColor       string `toml:"project"`
	ActiveColor        string `toml:"active"`
	InProgressColor    string `toml:"inprogress"`
	CompletedColor     string `toml:"completed"`
	TaskColor          string `toml:"description"`
	CompletedTaskColor string `toml:"completed-description"`
	TagColor           string `toml:"tag"`
	TagBgColor         string `toml:"tag-bg"`
	DateColor          string `toml:"date"`
	DateBgColor        string `toml:"date-bg"`
	PastDateColor      string `toml:"past-date"`
	PastDateBgColor    string `toml:"past-bg"`
	TodayDateColor     string `toml:"today-date"`
	TodayDateBgColor   string `toml:"today-bg"`
	AssigneeColor      string `toml:"assignee"`
	AssigneeBgColor    string `toml:"assignee-bg"`
}

type Config struct {
	PRJDIR             string      `toml:"prjdir"`
	ZETDIR             string      `toml:"zetdir"`
	EDITOR             string      `toml:"editor"`
	KARYA_DIR          string      `toml:"karya_dir"`
	ShowCompleted      bool        `toml:"show_completed"`
	Structured         bool        `toml:"structured"`
	ColorMode          string      `toml:"color_mode"` // "light", "dark", or empty for auto-detect
	Colors             ColorScheme `toml:"colors"`
	ActiveKeywords     []string    `toml:"active_keywords"`
	InProgressKeywords []string    `toml:"inprogress_keywords"`
	CompletedKeywords  []string    `toml:"completed_keywords"`
}

func Load() (*Config, error) {
	// Try environment variables first
	cfg := &Config{
		PRJDIR:    os.Getenv("PRJDIR"),
		ZETDIR:    os.Getenv("ZETDIR"),
		EDITOR:    os.Getenv("EDITOR"),
		KARYA_DIR: os.Getenv("KARYA_DIR"),
	}

	// Check SHOW_COMPLETED environment variable
	if showCompleted := os.Getenv("SHOW_COMPLETED"); showCompleted != "" {
		cfg.ShowCompleted = showCompleted == "true" || showCompleted == "1"
	}

	// Check STRUCTURED environment variable (defaults to true)
	cfg.Structured = true // default value
	if structured := os.Getenv("STRUCTURED"); structured != "" {
		cfg.Structured = structured == "true" || structured == "1"
	}

	// If PRJDIR not set, try loading from config file
	if cfg.PRJDIR == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}

		configPath := filepath.Join(home, ".config", "karya", "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			if _, err := toml.DecodeFile(configPath, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
			// Expand environment variables in config values
			cfg.PRJDIR = expandEnv(cfg.PRJDIR)
			cfg.ZETDIR = expandEnv(cfg.ZETDIR)
			cfg.EDITOR = expandEnv(cfg.EDITOR)
			cfg.KARYA_DIR = expandEnv(cfg.KARYA_DIR)

			// Environment variable overrides config file for ShowCompleted
			if showCompleted := os.Getenv("SHOW_COMPLETED"); showCompleted != "" {
				cfg.ShowCompleted = showCompleted == "true" || showCompleted == "1"
			}
		}
	}

	// Set defaults
	if cfg.EDITOR == "" {
		cfg.EDITOR = "vim"
	}
	if cfg.KARYA_DIR == "" && cfg.PRJDIR != "" {
		cfg.KARYA_DIR = cfg.PRJDIR
	}
	if len(cfg.ActiveKeywords) == 0 {
		cfg.ActiveKeywords = []string{
			"TODO", "TASK", "NOTE", "REMINDER", "EVENT", "MEETING",
			"CALL", "EMAIL", "MESSAGE", "FOLLOWUP", "REVIEW",
			"CHECKIN", "CHECKOUT", "RESEARCH", "READING", "WRITING",
			"DRAFT", "FINALIZE", "SUBMIT", "PRESENTATION",
		}
	}
	if len(cfg.InProgressKeywords) == 0 {
		cfg.InProgressKeywords = []string{
			"DOING", "INPROGRESS", "WIP", "WORKING", "STARTED",
		}
	}
	if len(cfg.CompletedKeywords) == 0 {
		cfg.CompletedKeywords = []string{
			"ARCHIVED", "CANCELED", "DELETED", "DONE", "COMPLETED", "CLOSED",
		}
	}

	// Initialize colors with defaults based on mode
	cfg.initializeColors()

	return cfg, nil
}

func expandEnv(s string) string {
	if s == "" {
		return s
	}
	// Replace $HOME with actual home directory
	if strings.Contains(s, "$HOME") {
		home, _ := os.UserHomeDir()
		s = strings.ReplaceAll(s, "$HOME", home)
	}
	// Expand other environment variables
	return os.ExpandEnv(s)
}

func (c *Config) Validate() error {
	if c.PRJDIR == "" {
		return fmt.Errorf("PRJDIR not set. Please create ~/.config/karya/config.toml with:\nprjdir = \"/path/to/projects\"")
	}
	return nil
}

// initializeColors sets up default colors based on color mode
// Colors can be overridden in the config file [colors] section
func (c *Config) initializeColors() {
	// Determine color mode: explicit config > environment > empty (auto-detect)
	colorMode := c.ColorMode
	if colorMode == "" {
		if envMode := os.Getenv("KARYA_COLOR_MODE"); envMode != "" {
			colorMode = envMode
		}
	}

	// Light mode colors (better for light terminal themes)
	lightMode := ColorScheme{
		ProjectColor:       "4",  // Blue
		ActiveColor:        "5",  // Magenta
		InProgressColor:    "4",  // Blue (progress)
		CompletedColor:     "8",  // Bright black (faded)
		TaskColor:          "0",  // Black
		CompletedTaskColor: "8",  // Bright black (faded)
		TagColor:           "15", // White text
		TagBgColor:         "4",  // Magenta background
		DateColor:          "4",  // Blue
		DateBgColor:        "7",  // Light gray background (visible with blue text)
		PastDateColor:      "15", // White text
		PastDateBgColor:    "1",  // Red (urgent) background
		TodayDateColor:     "0",  // Black text
		TodayDateBgColor:   "3",  // Yellow background
		AssigneeColor:      "0",  // Black text
		AssigneeBgColor:    "7",  // Light gray background
	}

	// Dark mode colors (better for dark terminal themes)
	darkMode := ColorScheme{
		ProjectColor:       "2",  // Green
		ActiveColor:        "3",  // Yellow
		InProgressColor:    "6",  // Cyan
		CompletedColor:     "8",  // Bright black (faded)
		TaskColor:          "15", // White
		CompletedTaskColor: "8",  // Bright black (faded)
		TagColor:           "0",  // Black text
		TagBgColor:         "14", // Light blue background
		DateColor:          "0",  // Black text
		DateBgColor:        "12", // Light blue background
		PastDateColor:      "0",  // Black text
		PastDateBgColor:    "1",  // Red background
		TodayDateColor:     "0",  // Black text
		TodayDateBgColor:   "11", // Yellow background
		AssigneeColor:      "15", // White text
		AssigneeBgColor:    "8",  // Light gray background
	}

	// Select default mode based on colorMode
	var defaults ColorScheme
	switch strings.ToLower(colorMode) {
	case "light":
		defaults = lightMode
	case "dark":
		defaults = darkMode
	default:
		// Auto-detect: use dark mode as default (most common)
		defaults = darkMode
	}

	// Apply defaults only if colors are not explicitly set
	if c.Colors.ProjectColor == "" {
		c.Colors.ProjectColor = defaults.ProjectColor
	}
	if c.Colors.ActiveColor == "" {
		c.Colors.ActiveColor = defaults.ActiveColor
	}
	if c.Colors.InProgressColor == "" {
		c.Colors.InProgressColor = defaults.InProgressColor
	}
	if c.Colors.CompletedColor == "" {
		c.Colors.CompletedColor = defaults.CompletedColor
	}
	if c.Colors.TaskColor == "" {
		c.Colors.TaskColor = defaults.TaskColor
	}
	if c.Colors.CompletedTaskColor == "" {
		c.Colors.CompletedTaskColor = defaults.CompletedTaskColor
	}
	if c.Colors.TagColor == "" {
		c.Colors.TagColor = defaults.TagColor
	}
	if c.Colors.TagBgColor == "" {
		c.Colors.TagBgColor = defaults.TagBgColor
	}
	if c.Colors.DateColor == "" {
		c.Colors.DateColor = defaults.DateColor
	}
	if c.Colors.DateBgColor == "" {
		c.Colors.DateBgColor = defaults.DateBgColor
	}
	if c.Colors.PastDateColor == "" {
		c.Colors.PastDateColor = defaults.PastDateColor
	}
	if c.Colors.PastDateBgColor == "" {
		c.Colors.PastDateBgColor = defaults.PastDateBgColor
	}
	if c.Colors.TodayDateColor == "" {
		c.Colors.TodayDateColor = defaults.TodayDateColor
	}
	if c.Colors.TodayDateBgColor == "" {
		c.Colors.TodayDateBgColor = defaults.TodayDateBgColor
	}
	if c.Colors.AssigneeColor == "" {
		c.Colors.AssigneeColor = defaults.AssigneeColor
	}
	if c.Colors.AssigneeBgColor == "" {
		c.Colors.AssigneeBgColor = defaults.AssigneeBgColor
	}

	// Resolve all color names to ANSI values
	c.Colors.ProjectColor = resolveColorValue(c.Colors.ProjectColor)
	c.Colors.ActiveColor = resolveColorValue(c.Colors.ActiveColor)
	c.Colors.InProgressColor = resolveColorValue(c.Colors.InProgressColor)
	c.Colors.CompletedColor = resolveColorValue(c.Colors.CompletedColor)
	c.Colors.TaskColor = resolveColorValue(c.Colors.TaskColor)
	c.Colors.CompletedTaskColor = resolveColorValue(c.Colors.CompletedTaskColor)
	c.Colors.TagColor = resolveColorValue(c.Colors.TagColor)
	c.Colors.TagBgColor = resolveColorValue(c.Colors.TagBgColor)
	c.Colors.DateColor = resolveColorValue(c.Colors.DateColor)
	c.Colors.DateBgColor = resolveColorValue(c.Colors.DateBgColor)
	c.Colors.PastDateColor = resolveColorValue(c.Colors.PastDateColor)
	c.Colors.PastDateBgColor = resolveColorValue(c.Colors.PastDateBgColor)
	c.Colors.TodayDateColor = resolveColorValue(c.Colors.TodayDateColor)
	c.Colors.TodayDateBgColor = resolveColorValue(c.Colors.TodayDateBgColor)
	c.Colors.AssigneeColor = resolveColorValue(c.Colors.AssigneeColor)
	c.Colors.AssigneeBgColor = resolveColorValue(c.Colors.AssigneeBgColor)
}
