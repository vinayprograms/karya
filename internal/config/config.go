package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
	lipglossthemes "github.com/willyv3/gogh-themes/lipgloss"
)

var colorNameMap = map[string]string{
	"black":          "0",
	"red":            "1",
	"green":          "2",
	"yellow":         "3",
	"blue":           "4",
	"magenta":        "5",
	"cyan":           "6",
	"white":          "7",
	"bright-black":   "8",
	"gray":           "8",
	"bright-red":     "9",
	"bright-green":   "10",
	"bright-yellow":  "11",
	"bright-blue":    "12",
	"bright-magenta": "13",
	"bright-cyan":    "14",
	"bright-white":   "15",
}

var themeColorCache map[string]lipgloss.Color

func resolveColorValue(colorInput string) string {
	if colorInput == "" {
		return colorInput
	}

	if themeColorCache != nil {
		if themeColor, exists := themeColorCache[strings.ToLower(colorInput)]; exists {
			return string(themeColor)
		}
	}

	if ansiValue, exists := colorNameMap[strings.ToLower(colorInput)]; exists {
		return ansiValue
	}

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
	SpecialTagColor    string `toml:"special-tag"`
	SpecialTagBgColor  string `toml:"special-tag-bg"`
	DateColor          string `toml:"date"`
	DateBgColor        string `toml:"date-bg"`
	PastDateColor      string `toml:"past-date"`
	PastDateBgColor    string `toml:"past-bg"`
	TodayDateColor     string `toml:"today-date"`
	TodayDateBgColor   string `toml:"today-bg"`
	AssigneeColor      string `toml:"assignee"`
	AssigneeBgColor    string `toml:"assignee-bg"`
}

type Directories struct {
	Projects     string `toml:"projects"`     // Project root directory
	Zettelkasten string `toml:"zettelkasten"` // Zettelkasten directory
	Karya        string `toml:"karya"`        // Karya inbox directory
}

type Todo struct {
	ShowCompleted bool     `toml:"show_completed"`
	Structured    bool     `toml:"structured"`
	Active        []string `toml:"active"`
	InProgress    []string `toml:"inprogress"`
	Completed     []string `toml:"completed"`
	SpecialTags   []string `toml:"special-tags"`
}
type GeneralConfig struct {
	EDITOR  string `toml:"editor"`
	Verbose bool   `toml:"verbose"`
	Theme   string `toml:"theme"`
}

type Config struct {
	GeneralConfig GeneralConfig `toml:"general"`
	Directories   Directories   `toml:"directories"`
	Todo          Todo          `toml:"todo"`
	Colors        ColorScheme   `toml:"colors"`
}

func Load() (*Config, error) {
	// Initialize empty config with defaults
	cfg := &Config{}

	// Load from config file first
	home, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(home, ".config", "karya", "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			if _, err := toml.DecodeFile(configPath, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
			if cfg.GeneralConfig.Theme != "" {
				if err := loadTheme(cfg.GeneralConfig.Theme); err != nil {
					return nil, fmt.Errorf("failed to load theme '%s': %w", cfg.GeneralConfig.Theme, err)
				}
			}
			cfg.GeneralConfig.EDITOR = expandEnv(cfg.GeneralConfig.EDITOR)
			cfg.Directories.Projects = expandEnv(cfg.Directories.Projects)
			cfg.Directories.Zettelkasten = expandEnv(cfg.Directories.Zettelkasten)
			cfg.Directories.Karya = expandEnv(cfg.Directories.Karya)
		}
	}

	// Environment variables override config file (applied AFTER config file load)
	if projects := os.Getenv("PROJECTS"); projects != "" {
		cfg.Directories.Projects = projects
	}
	if zettelkasten := os.Getenv("ZETTELKASTEN"); zettelkasten != "" {
		cfg.Directories.Zettelkasten = zettelkasten
	}
	if karya := os.Getenv("KARYA"); karya != "" {
		cfg.Directories.Karya = karya
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		cfg.GeneralConfig.EDITOR = editor
	}
	if showCompleted := os.Getenv("SHOW_COMPLETED"); showCompleted != "" {
		cfg.Todo.ShowCompleted = showCompleted == "true" || showCompleted == "1"
	}
	if structured := os.Getenv("STRUCTURED"); structured != "" {
		cfg.Todo.Structured = structured == "true" || structured == "1"
	}
	if verbose := os.Getenv("VERBOSE"); verbose != "" {
		cfg.GeneralConfig.Verbose = verbose == "true" || verbose == "1"
	}

	// Set defaults
	if cfg.GeneralConfig.EDITOR == "" {
		cfg.GeneralConfig.EDITOR = "vim"
	}
	if cfg.Directories.Karya == "" && cfg.Directories.Projects != "" {
		cfg.Directories.Karya = cfg.Directories.Projects
	}

	// Set keyword defaults if not provided
	if len(cfg.Todo.Active) == 0 {
		cfg.Todo.Active = []string{
			"TODO", "TASK", "NOTE", "REMINDER", "EVENT", "MEETING",
			"CALL", "EMAIL", "MESSAGE", "FOLLOWUP", "REVIEW",
			"CHECKIN", "CHECKOUT", "RESEARCH", "READING", "WRITING",
			"DRAFT", "FINALIZE", "SUBMIT", "PRESENTATION",
		}
	}
	if len(cfg.Todo.InProgress) == 0 {
		cfg.Todo.InProgress = []string{
			"DOING", "INPROGRESS", "WIP", "WORKING", "STARTED",
		}
	}
	if len(cfg.Todo.Completed) == 0 {
		cfg.Todo.Completed = []string{
			"ARCHIVED", "CANCELED", "DELETED", "DONE", "COMPLETED", "CLOSED",
		}
	}

	// Initialize colors with defaults
	cfg.initializeColors()

	// Resolve color names in user-specified colors (from config file)
	cfg.Colors.ProjectColor = resolveColorValue(cfg.Colors.ProjectColor)
	cfg.Colors.ActiveColor = resolveColorValue(cfg.Colors.ActiveColor)
	cfg.Colors.InProgressColor = resolveColorValue(cfg.Colors.InProgressColor)
	cfg.Colors.CompletedColor = resolveColorValue(cfg.Colors.CompletedColor)
	cfg.Colors.TaskColor = resolveColorValue(cfg.Colors.TaskColor)
	cfg.Colors.CompletedTaskColor = resolveColorValue(cfg.Colors.CompletedTaskColor)
	cfg.Colors.TagColor = resolveColorValue(cfg.Colors.TagColor)
	cfg.Colors.TagBgColor = resolveColorValue(cfg.Colors.TagBgColor)
	cfg.Colors.SpecialTagColor = resolveColorValue(cfg.Colors.SpecialTagColor)
	cfg.Colors.SpecialTagBgColor = resolveColorValue(cfg.Colors.SpecialTagBgColor)
	cfg.Colors.DateColor = resolveColorValue(cfg.Colors.DateColor)
	cfg.Colors.DateBgColor = resolveColorValue(cfg.Colors.DateBgColor)
	cfg.Colors.PastDateColor = resolveColorValue(cfg.Colors.PastDateColor)
	cfg.Colors.PastDateBgColor = resolveColorValue(cfg.Colors.PastDateBgColor)
	cfg.Colors.TodayDateColor = resolveColorValue(cfg.Colors.TodayDateColor)
	cfg.Colors.TodayDateBgColor = resolveColorValue(cfg.Colors.TodayDateBgColor)
	cfg.Colors.AssigneeColor = resolveColorValue(cfg.Colors.AssigneeColor)
	cfg.Colors.AssigneeBgColor = resolveColorValue(cfg.Colors.AssigneeBgColor)

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
	if c.Directories.Projects == "" {
		return fmt.Errorf("projects directory not set. Please create ~/.config/karya/config.toml with:\n[directories]\nprojects = \"/path/to/projects\"")
	}
	return nil
}

func (c *Config) initializeColors() {
	// When no theme is set and no custom colors specified,
	// leave colors empty so terminal's native ANSI colors are used
	if themeColorCache != nil {
		if c.Colors.ProjectColor == "" {
			c.Colors.ProjectColor = string(themeColorCache["green"])
		}
		if c.Colors.ActiveColor == "" {
			c.Colors.ActiveColor = string(themeColorCache["yellow"])
		}
		if c.Colors.InProgressColor == "" {
			c.Colors.InProgressColor = string(themeColorCache["cyan"])
		}
		if c.Colors.CompletedColor == "" {
			c.Colors.CompletedColor = string(themeColorCache["gray"])
		}
		if c.Colors.TaskColor == "" {
			c.Colors.TaskColor = string(themeColorCache["white"])
		}
		if c.Colors.CompletedTaskColor == "" {
			c.Colors.CompletedTaskColor = string(themeColorCache["gray"])
		}
		if c.Colors.TagColor == "" {
			c.Colors.TagColor = string(themeColorCache["black"])
		}
		if c.Colors.TagBgColor == "" {
			c.Colors.TagBgColor = string(themeColorCache["bright-cyan"])
		}
		if c.Colors.SpecialTagColor == "" {
			c.Colors.SpecialTagColor = string(themeColorCache["bright-white"])
		}
		if c.Colors.SpecialTagBgColor == "" {
			c.Colors.SpecialTagBgColor = string(themeColorCache["magenta"])
		}
		if c.Colors.DateColor == "" {
			c.Colors.DateColor = string(themeColorCache["black"])
		}
		if c.Colors.DateBgColor == "" {
			c.Colors.DateBgColor = string(themeColorCache["bright-blue"])
		}
		if c.Colors.PastDateColor == "" {
			c.Colors.PastDateColor = string(themeColorCache["black"])
		}
		if c.Colors.PastDateBgColor == "" {
			c.Colors.PastDateBgColor = string(themeColorCache["red"])
		}
		if c.Colors.TodayDateColor == "" {
			c.Colors.TodayDateColor = string(themeColorCache["black"])
		}
		if c.Colors.TodayDateBgColor == "" {
			c.Colors.TodayDateBgColor = string(themeColorCache["bright-yellow"])
		}
		if c.Colors.AssigneeColor == "" {
			c.Colors.AssigneeColor = string(themeColorCache["bright-white"])
		}
		if c.Colors.AssigneeBgColor == "" {
			c.Colors.AssigneeBgColor = string(themeColorCache["gray"])
		}
	} else {
		// No theme set - use terminal's native ANSI colors by setting color names only
		// This allows the terminal emulator to use its own color scheme (light/dark)
		if c.Colors.ProjectColor == "" {
			c.Colors.ProjectColor = "2"  // ANSI green - terminal decides actual color
		}
		if c.Colors.ActiveColor == "" {
			c.Colors.ActiveColor = "3"  // ANSI yellow
		}
		if c.Colors.InProgressColor == "" {
			c.Colors.InProgressColor = "6"  // ANSI cyan
		}
		if c.Colors.CompletedColor == "" {
			c.Colors.CompletedColor = "8"  // ANSI bright black (gray)
		}
		if c.Colors.TaskColor == "" {
			c.Colors.TaskColor = ""  // Empty = terminal default foreground
		}
		if c.Colors.CompletedTaskColor == "" {
			c.Colors.CompletedTaskColor = "8"  // ANSI bright black (gray)
		}
		if c.Colors.TagColor == "" {
			c.Colors.TagColor = ""  // Empty = terminal default foreground
		}
		if c.Colors.TagBgColor == "" {
			c.Colors.TagBgColor = "6"  // ANSI cyan
		}
		if c.Colors.SpecialTagColor == "" {
			c.Colors.SpecialTagColor = ""  // Empty = terminal default foreground
		}
		if c.Colors.SpecialTagBgColor == "" {
			c.Colors.SpecialTagBgColor = "5"  // ANSI magenta
		}
		if c.Colors.DateColor == "" {
			c.Colors.DateColor = ""  // Empty = terminal default foreground
		}
		if c.Colors.DateBgColor == "" {
			c.Colors.DateBgColor = "4"  // ANSI blue
		}
		if c.Colors.PastDateColor == "" {
			c.Colors.PastDateColor = ""  // Empty = terminal default foreground
		}
		if c.Colors.PastDateBgColor == "" {
			c.Colors.PastDateBgColor = "1"  // ANSI red
		}
		if c.Colors.TodayDateColor == "" {
			c.Colors.TodayDateColor = "0"  // ANSI black
		}
		if c.Colors.TodayDateBgColor == "" {
			c.Colors.TodayDateBgColor = "3"  // ANSI yellow
		}
		if c.Colors.AssigneeColor == "" {
			c.Colors.AssigneeColor = ""  // Empty = terminal default foreground
		}
		if c.Colors.AssigneeBgColor == "" {
			c.Colors.AssigneeBgColor = "8"  // ANSI bright black (gray)
		}
	}

}

func loadTheme(themeName string) error {
	theme, ok := lipglossthemes.Get(themeName)
	if !ok {
		return fmt.Errorf("theme not found")
	}

	themeColorCache = map[string]lipgloss.Color{
		"black":          theme.Black,
		"red":            theme.Red,
		"green":          theme.Green,
		"yellow":         theme.Yellow,
		"blue":           theme.Blue,
		"magenta":        theme.Magenta,
		"cyan":           theme.Cyan,
		"white":          theme.White,
		"bright-black":   theme.BrightBlack,
		"gray":           theme.BrightBlack,
		"bright-red":     theme.BrightRed,
		"bright-green":   theme.BrightGreen,
		"bright-yellow":  theme.BrightYellow,
		"bright-blue":    theme.BrightBlue,
		"bright-magenta": theme.BrightMagenta,
		"bright-cyan":    theme.BrightCyan,
		"bright-white":   theme.BrightWhite,
	}

	return nil
}
