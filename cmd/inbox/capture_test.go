package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminal(t *testing.T) {
	for _, test := range []struct {
		name, input string
		saved       bool
	}{
		{"submit", "question\r", true},
		{"cancel", "\x03", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := setup(t)
			var output, display bytes.Buffer
			if err := run([]string{"add", "--json"}, strings.NewReader(test.input), &output, &display, true); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if test.saved {
				if err != nil || string(data) != "# INBOX\n\nTODO: question\n" {
					t.Fatalf("terminal capture = %q, %v", data, err)
				}
				if !strings.HasPrefix(output.String(), "{\"path\":") {
					t.Fatalf("polluted JSON: %q", output.String())
				}
			} else if !errors.Is(err, os.ErrNotExist) || output.Len() != 0 {
				t.Fatalf("cancellation wrote data/output: %q / %q", data, output.String())
			}
		})
	}
}

func TestConfiguration(t *testing.T) {
	for _, keyword := range []string{"''", "'BAD:KEY'", "'BAD KEY'", "\"BAD\\nKEY\""} {
		t.Run(keyword, func(t *testing.T) {
			path := setup(t)
			directory := filepath.Join(os.Getenv("HOME"), ".config", "karya")
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("[todo]\nactive = ["+keyword+"]\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := run([]string{"add", "task"}, strings.NewReader(""), io.Discard, io.Discard, false); err == nil {
				t.Fatal("accepted unsafe keyword")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("created inbox: %v", err)
			}
		})
	}
}

func TestLaunch(t *testing.T) {
	if os.Getenv("INBOX_TEST_EDITOR") == "1" {
		for _, arg := range os.Args {
			if arg == "--" {
				os.Exit(0)
			}
		}
		os.Exit(2)
	}
	path := setup(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("INBOX_TEST_EDITOR", "1")
	for _, original := range []string{"", "text", "text\n", "text\n\n"} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(original), 0644); err != nil {
			t.Fatal(err)
		}
		if err := edit(executable+" -test.run=^TestLaunch$ --", path, strings.NewReader(""), io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		want := original
		if original != "" {
			want = "text\n\n"
		}
		if err != nil || string(data) != want {
			t.Fatalf("editor padding = %q, want %q: %v", data, want, err)
		}
	}
}
