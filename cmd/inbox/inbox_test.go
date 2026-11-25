package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func setup(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KARYA", filepath.Join(home, "data"))
	t.Setenv("EDITOR", "vim")
	return filepath.Join(home, "data", "inbox.md")
}

func TestRun(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		input   string
		want    string
		failure bool
	}{
		{"title", []string{"add", "first", "task"}, "", "TODO: first task\n", false},
		{"alias", []string{"a", "task"}, "", "TODO: task\n", false},
		{"batch", []string{"add"}, " first\n\n  DONE: second #tag\r\n", "TODO: first\nDONE: second #tag\n", false},
		{"explicit", []string{"add", "--stdin", "--keyword", "DONE"}, "first\nTODO: second", "DONE: first\nTODO: second\n", false},
		{"terminator", []string{"add", "--", "--title"}, "", "TODO: --title\n", false},
		{"empty", []string{"add"}, " \n", "", true},
		{"empty record", []string{"add"}, "valid\nTODO: ", "", true},
		{"conflict", []string{"add", "title"}, "other", "", true},
		{"whitespace conflict", []string{"add", "title"}, " ", "", true},
		{"explicit conflict", []string{"add", "--stdin", "title"}, "", "", true},
		{"invalid keyword", []string{"add", "--keyword", "UNKNOWN", "task"}, "", "", true},
		{"empty keyword", []string{"add", "--keyword=", "task"}, "", "", true},
		{"unknown flag", []string{"add", "--bogus"}, "", "", true},
		{"unknown command", []string{"bogus"}, "", "", true},
		{"edit headless", []string{"edit"}, "", "", true},
		{"bare headless", nil, "", "", true},
		{"multiline title", []string{"add", "first\nsecond"}, "", "", true},
		{"output conflict", []string{"add", "--quiet", "--json", "task"}, "", "", true},
		{"invalid text", []string{"add"}, "first\nsecond\x00", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := setup(t)
			var output, diagnostics bytes.Buffer
			err := run(test.args, strings.NewReader(test.input), &output, &diagnostics, false)
			if (err != nil) != test.failure {
				t.Fatalf("run error = %v, want failure %v", err, test.failure)
			}
			data, readErr := os.ReadFile(path)
			if test.failure {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("failed input created storage: %q, %v", data, readErr)
				}
				return
			}
			if readErr != nil || string(data) != "# INBOX\n\n"+test.want {
				t.Fatalf("content = %q, error %v", data, readErr)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	path := setup(t)
	directory := filepath.Join(os.Getenv("HOME"), ".config", "karya")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("[invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}, {"help", "add"}, {"add", "--help"}, {"edit", "--help"}} {
		var output bytes.Buffer
		if err := run(args, strings.NewReader(""), &output, io.Discard, false); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(output.String(), "Usage:") {
			t.Fatalf("missing help: %q", output.String())
		}
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help created storage: %v", err)
	}
}

func TestOutput(t *testing.T) {
	path := setup(t)
	var output bytes.Buffer
	if err := run([]string{"add", "--json", "task"}, strings.NewReader(""), &output, io.Discard, false); err != nil {
		t.Fatal(err)
	}
	var result receipt
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Path != path || len(result.Tasks) != 1 || result.Tasks[0] != "TODO: task" {
		t.Fatalf("receipt = %+v", result)
	}
	output.Reset()
	if err := run([]string{"add", "--quiet", "second"}, strings.NewReader(""), &output, io.Discard, false); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("quiet output = %q", output.String())
	}
}

func TestKeywords(t *testing.T) {
	path := setup(t)
	directory := filepath.Join(os.Getenv("HOME"), ".config", "karya")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte("[todo]\nactive = ['NEXT']\ncompleted = ['FINISHED']\nevents = ['DAILY']\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"add"}, strings.NewReader("plain\nFINISHED: existing\nDAILY: routine"), io.Discard, io.Discard, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# INBOX\n\nNEXT: plain\nFINISHED: existing\nDAILY: routine\n" {
		t.Fatalf("content = %q", data)
	}
}

type broken struct{}

func (broken) Read([]byte) (int, error) { return 0, errors.New("broken stream") }

func TestValidation(t *testing.T) {
	path := setup(t)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, input := range []io.Reader{io.MultiReader(strings.NewReader("valid\n"), broken{}), strings.NewReader("valid\nTODO:")} {
		if err := run([]string{"add"}, input, io.Discard, io.Discard, false); err == nil {
			t.Fatal("expected failure")
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "original" {
			t.Fatalf("validation mutated file: %q, %v", data, err)
		}
	}
	long := strings.Repeat("x", 100000)
	if err := run([]string{"add"}, strings.NewReader(long), io.Discard, io.Discard, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original\nTODO: "+long+"\n" {
		t.Fatalf("long line/newline preservation failed: %v", err)
	}
}

func TestModel(t *testing.T) {
	current := initial()
	updated, _ := current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	current = updated.(model)
	if current.quitting || current.input.Value() != "q" {
		t.Fatalf("q cancelled input: %+v", current)
	}
	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.(model).accepted {
		t.Fatal("enter did not submit")
	}
	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if updated.(model).accepted || !updated.(model).quitting {
		t.Fatal("cancel submitted input")
	}
}

func TestEditor(t *testing.T) {
	path := setup(t)
	if err := run([]string{"edit"}, strings.NewReader(""), io.Discard, io.Discard, true); err == nil || !strings.Contains(err.Error(), "capture") {
		t.Fatalf("missing editor path: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("edit created storage: %v", err)
	}
}

func TestProcess(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "inbox")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	path := setup(t)
	for _, test := range []struct {
		args    []string
		input   string
		failure bool
	}{
		{[]string{"--help"}, "", false},
		{[]string{"unknown"}, "", true},
		{[]string{"add", "--json"}, "first\nDONE: second", false},
	} {
		command := exec.CommandContext(t.Context(), binary, test.args...)
		command.Stdin = strings.NewReader(test.input)
		var output, diagnostics bytes.Buffer
		command.Stdout, command.Stderr = &output, &diagnostics
		err := command.Run()
		if (err != nil) != test.failure {
			t.Fatalf("%v: %v stderr %q", test.args, err, diagnostics.String())
		}
		if test.failure && (output.Len() != 0 || diagnostics.Len() == 0) {
			t.Fatalf("incorrect error streams: %q / %q", output.String(), diagnostics.String())
		}
		if test.args[0] != "add" {
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("non-capture created inbox: %v", err)
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "# INBOX\n\nTODO: first\nDONE: second\n" {
		t.Fatalf("process capture = %q, %v", data, err)
	}
}
