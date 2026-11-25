package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShow(t *testing.T) {
	path := setup(t)
	var output bytes.Buffer
	if err := run([]string{"show"}, strings.NewReader(""), &output, io.Discard, false); err == nil {
		t.Fatal("missing inbox succeeded")
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("show created storage: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"", "# INBOX\r\n\nTODO: unchanged", "bytes\x00\xff\n"} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		output.Reset()
		if err := run([]string{"show"}, strings.NewReader("ignored"), &output, io.Discard, false); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != content || output.String() != content {
			t.Fatalf("show changed bytes: %q / %q, %v", data, output.String(), err)
		}
	}
	for _, args := range [][]string{{"show", "extra"}, {"show", "--json"}} {
		if err := run(args, strings.NewReader(""), io.Discard, io.Discard, false); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{"show", "--help"}, {"help", "show"}} {
		if err := run(args, strings.NewReader(""), io.Discard, io.Discard, false); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSelectors(t *testing.T) {
	for _, test := range []struct {
		name, content string
		exists        bool
		first         int
	}{
		{"new", "", false, 3},
		{"empty", "", true, 1},
		{"unterminated", "existing", true, 2},
		{"terminated", "existing\n", true, 2},
		{"blank line", "existing\n\n", true, 3},
		{"crlf", "existing\r\n", true, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := setup(t)
			if test.exists {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(test.content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			if err := run([]string{"add", "--json"}, strings.NewReader("same\nsame"), &output, io.Discard, false); err != nil {
				t.Fatal(err)
			}
			var result receipt
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Selectors) != 2 || len(result.Tasks) != 2 {
				t.Fatalf("receipt: %+v", result)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(data), "\n")
			for index, selected := range result.Selectors {
				if selected.Path != path || selected.Line != test.first+index || selected.Expected != result.Tasks[index] || lines[selected.Line-1] != selected.Expected {
					t.Fatalf("selector mismatch: %+v, file %q", selected, data)
				}
			}
		})
	}
}
