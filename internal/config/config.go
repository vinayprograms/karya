package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	PRJDIR    string `toml:"prjdir"`
	ZETDIR    string `toml:"zetdir"`
	EDITOR    string `toml:"editor"`
	KARYA_DIR string `toml:"karya_dir"`
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

		configPath := filepath.Join(home, ".config", "todo-toolkit", "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			if _, err := toml.DecodeFile(configPath, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
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

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.PRJDIR == "" {
		return fmt.Errorf("PRJDIR not set. Please create ~/.config/todo-toolkit/config.toml with:\nprjdir = \"/path/to/projects\"")
	}
	return nil
}