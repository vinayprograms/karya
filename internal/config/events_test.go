package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEventsColor(t *testing.T) {
	for _, test := range []struct{ value, want string }{{"", "6"}, {"cyan", "6"}, {"14", "14"}, {"#56B6C2", "#56B6C2"}} {
		t.Run(test.want, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			previous := themeColorCache
			themeColorCache = nil
			t.Cleanup(func() { themeColorCache = previous })
			directory := filepath.Join(home, ".config", "karya")
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("[colors]\nactive = 'yellow'\nevents = '"+test.value+"'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Colors.EventsColor != test.want || cfg.Colors.ActiveColor != "3" {
				t.Fatalf("colors = %+v", cfg.Colors)
			}
		})
	}
}
