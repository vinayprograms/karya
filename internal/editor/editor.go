// Package editor builds commands that open files in the user's editor,
// with optional line-number or search-term navigation.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// At describes where the editor should land inside the file.
// Search wins over Line when both are set.
type At struct {
	Line   int    // 1-based line number; 0 means no navigation
	Search string // term to jump to (supported by vim/emacs families)
}

// Command builds an exec.Cmd that opens file in the given editor.
// The spec may contain arguments (e.g. "emacs -nw") and a leading "~/".
// An empty spec falls back to $EDITOR, then "vim".
func Command(spec, file string, at At) *exec.Cmd {
	if spec == "" {
		spec = os.Getenv("EDITOR")
	}
	if spec == "" {
		spec = "vim"
	}
	if strings.HasPrefix(spec, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			spec = filepath.Join(home, spec[2:])
		}
	}

	parts := strings.Fields(spec)
	name := parts[0]
	args := parts[1:]
	base := filepath.Base(name)

	if at.Search != "" {
		switch base {
		case "vim", "nvim", "vi", "gvim":
			args = append(args, "+/"+at.Search)
		case "emacs":
			args = append(args, "--eval",
				fmt.Sprintf("(progn (goto-char (point-min)) (search-forward %q nil t))", at.Search))
		}
		// nano/code/subl cannot open with a search; fall through to plain open.
	}

	if at.Line > 0 && at.Search == "" {
		switch base {
		case "vim", "nvim", "vi", "gvim", "emacs", "nano":
			args = append(args, fmt.Sprintf("+%d", at.Line), file)
			return exec.Command(name, args...)
		case "code", "code-insiders":
			args = append(args, "-g", fmt.Sprintf("%s:%d", file, at.Line))
			return exec.Command(name, args...)
		}
	}
	args = append(args, file)
	return exec.Command(name, args...)
}

// Open runs the editor synchronously, wired to the current terminal.
func Open(spec, file string, at At) error {
	c := Command(spec, file, at)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
