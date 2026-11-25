package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/vinayprograms/karya/internal/config"
)

func TestRender(t *testing.T) {
	setup(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Todo.Active = []string{"NEXT"}
	cfg.Colors.ActiveColor = "196"
	cfg.Colors.TagColor = "33"
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.ANSI256)
	input := "# INBOX\r\n  NEXT: **keep** #tag @s:2000-01-01 >> person\nDONE: finished"
	output := render(input, cfg, renderer)
	if ansi.Strip(output) != input {
		t.Fatalf("render changed source: %q", ansi.Strip(output))
	}
	if !strings.Contains(output, "38;5;196") || !strings.Contains(output, "38;5;33") {
		t.Fatalf("missing configured colors: %q", output)
	}
}

func TestRaw(t *testing.T) {
	path := setup(t)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	content := "TODO: **untouched** #tag\r\nlast"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"show"}, {"show", "--raw"}} {
		var output bytes.Buffer
		if err := run(args, strings.NewReader(""), &output, io.Discard, true); err != nil {
			t.Fatal(err)
		}
		if output.String() != content {
			t.Fatalf("redirected output changed: %q", output.String())
		}
	}
	selected, err := parse([]string{"show", "--raw"})
	if err != nil || !selected.raw {
		t.Fatalf("raw flag: %+v, %v", selected, err)
	}
}
