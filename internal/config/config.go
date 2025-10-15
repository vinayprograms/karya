package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	PRJDIR            string   `toml:"prjdir"`
	ZETDIR            string   `toml:"zetdir"`
	EDITOR            string   `toml:"editor"`
	KARYA_DIR         string   `toml:"karya_dir"`
	ActiveKeywords    []string `toml:"active_keywords"`
	CompletedKeywords []string `toml:"completed_keywords"`
}

func Load() (*Config, error) {
	// Try environment variables first
	cfg := &Config{
		PRJDIR:    os.Getenv("PRJDIR"),
		ZETDIR:    os.Getenv("ZETDIR"),
		EDITOR:    os.Getenv("EDITOR"),
		KARYA_DIR: os.Getenv("KARYA_DIR"),
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
			"DRAFT", "EDITING", "FINALIZE", "SUBMIT", "PRESENTATION",
			"WAITING", "DEFERRED", "DELEGATED",
		}
	}
	if len(cfg.CompletedKeywords) == 0 {
		cfg.CompletedKeywords = []string{
			"ARCHIVED", "CANCELED", "DELETED", "DONE", "COMPLETED", "CLOSED",
		}
	}

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