package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseClockEntries(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := `DOING: Write design doc
  CLOCK: 2026-06-17T09:30--2026-06-17T11:15
  CLOCK: 2026-06-17T14:00--2026-06-17T15:30
  CLOCK: 2026-06-18T10:00--
  Some notes here
`
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "DOING",
		Title:       "Write design doc",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	entries, err := ParseClockEntries(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry: closed
	if entries[0].Open {
		t.Error("first entry should be closed")
	}
	if entries[0].Start.Hour() != 9 || entries[0].Start.Minute() != 30 {
		t.Errorf("first entry start: got %v", entries[0].Start)
	}
	if entries[0].End.Hour() != 11 || entries[0].End.Minute() != 15 {
		t.Errorf("first entry end: got %v", entries[0].End)
	}

	// Third entry: open
	if !entries[2].Open {
		t.Error("third entry should be open")
	}
	if entries[2].Start.Hour() != 10 {
		t.Errorf("third entry start hour: got %d", entries[2].Start.Hour())
	}
}

func TestParseClockEntriesNoClocks(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := `TODO: No clock task
  Some notes
`
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "No clock task",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	entries, err := ParseClockEntries(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestClipDuration(t *testing.T) {
	rangeStart := time.Date(2026, 6, 17, 0, 0, 0, 0, time.Local)
	rangeEnd := time.Date(2026, 6, 17, 23, 59, 59, 0, time.Local)

	tests := []struct {
		name     string
		entry    ClockEntry
		expected time.Duration
	}{
		{
			name: "fully within range",
			entry: ClockEntry{
				Start: time.Date(2026, 6, 17, 9, 0, 0, 0, time.Local),
				End:   time.Date(2026, 6, 17, 11, 0, 0, 0, time.Local),
			},
			expected: 2 * time.Hour,
		},
		{
			name: "spans start boundary",
			entry: ClockEntry{
				Start: time.Date(2026, 6, 16, 22, 0, 0, 0, time.Local),
				End:   time.Date(2026, 6, 17, 2, 0, 0, 0, time.Local),
			},
			expected: 2 * time.Hour, // only 00:00-02:00 counts
		},
		{
			name: "spans end boundary",
			entry: ClockEntry{
				Start: time.Date(2026, 6, 17, 22, 0, 0, 0, time.Local),
				End:   time.Date(2026, 6, 18, 2, 0, 0, 0, time.Local),
			},
			expected: 1*time.Hour + 59*time.Minute + 59*time.Second, // 22:00-23:59:59
		},
		{
			name: "completely before range",
			entry: ClockEntry{
				Start: time.Date(2026, 6, 16, 9, 0, 0, 0, time.Local),
				End:   time.Date(2026, 6, 16, 11, 0, 0, 0, time.Local),
			},
			expected: 0,
		},
		{
			name: "completely after range",
			entry: ClockEntry{
				Start: time.Date(2026, 6, 18, 9, 0, 0, 0, time.Local),
				End:   time.Date(2026, 6, 18, 11, 0, 0, 0, time.Local),
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClipDuration(tt.entry, rangeStart, rangeEnd)
			if got != tt.expected {
				t.Errorf("got %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestClockIn(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "TODO: My task\n  Some notes\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "My task",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	err := ClockIn(task)
	if err != nil {
		t.Fatalf("clock in failed: %v", err)
	}

	result, _ := os.ReadFile(f)
	lines := strings.Split(string(result), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Second line should be the bulleted clock entry
	clockLine := strings.TrimSpace(lines[1])
	if !strings.HasPrefix(clockLine, "* CLOCK:") {
		t.Errorf("expected * CLOCK line, got %q", clockLine)
	}
	if !strings.HasSuffix(clockLine, "--") {
		t.Errorf("expected open clock entry (ending with --), got %q", clockLine)
	}
}

func TestClockInAlreadyActive(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "TODO: My task\n  CLOCK: 2026-06-17T09:00--\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "My task",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	err := ClockIn(task)
	if err == nil {
		t.Fatal("expected error for already clocked in task")
	}
	if !strings.Contains(err.Error(), "already clocked in") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClockOut(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "TODO: My task\n  CLOCK: 2026-06-17T09:00--\n  Some notes\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "My task",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	err := ClockOut(task)
	if err != nil {
		t.Fatalf("clock out failed: %v", err)
	}

	result, _ := os.ReadFile(f)
	lines := strings.Split(string(result), "\n")
	clockLine := strings.TrimSpace(lines[1])
	if strings.HasSuffix(clockLine, "--") {
		t.Error("clock entry should be closed (not ending with --)")
	}
	// Should match pattern: CLOCK: <start>--<end>
	if !strings.Contains(clockLine, "--2") { // --2026...
		t.Errorf("expected end timestamp, got %q", clockLine)
	}
}

func TestClockOutNoActive(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "TODO: My task\n  CLOCK: 2026-06-17T09:00--2026-06-17T10:00\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "My task",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	err := ClockOut(task)
	if err == nil {
		t.Fatal("expected error for no active clock")
	}
	if !strings.Contains(err.Error(), "no active clock") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIsClockActive(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")

	// Active
	content := "DOING: Active task\n  CLOCK: 2026-06-17T09:00--\n"
	os.WriteFile(f, []byte(content), 0644)
	task := &Task{Keyword: "DOING", Title: "Active task", FilePath: f, LineNum: 1, IndentLevel: 0}
	if !IsClockActive(task) {
		t.Error("expected active clock")
	}

	// Not active
	content = "DONE: Done task\n  CLOCK: 2026-06-17T09:00--2026-06-17T10:00\n"
	os.WriteFile(f, []byte(content), 0644)
	task = &Task{Keyword: "DONE", Title: "Done task", FilePath: f, LineNum: 1, IndentLevel: 0}
	if IsClockActive(task) {
		t.Error("expected no active clock")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0, "0:00"},
		{30 * time.Minute, "0:30"},
		{1*time.Hour + 45*time.Minute, "1:45"},
		{12*time.Hour + 5*time.Minute, "12:05"},
	}
	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.expected {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.expected)
		}
	}
}

func TestParseClockEntriesBulletFormat(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := `DOING: Write design doc
  * CLOCK: 2026-06-17T09:30--2026-06-17T11:15
  - CLOCK: 2026-06-17T14:00--2026-06-17T15:30
  + CLOCK: 2026-06-17T16:00--2026-06-17T17:00
  CLOCK: 2026-06-18T10:00--2026-06-18T12:00
`
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "DOING",
		Title:       "Write design doc",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	entries, err := ParseClockEntries(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries (*, -, +, bare), got %d", len(entries))
	}
}

func TestClockOutBulletFormat(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "TODO: My task\n  * CLOCK: 2026-06-17T09:00--\n  Some notes\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "My task",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	err := ClockOut(task)
	if err != nil {
		t.Fatalf("clock out failed: %v", err)
	}

	result, _ := os.ReadFile(f)
	lines := strings.Split(string(result), "\n")
	clockLine := strings.TrimSpace(lines[1])
	if strings.HasSuffix(clockLine, "--") {
		t.Error("clock entry should be closed")
	}
	if !strings.HasPrefix(clockLine, "* CLOCK:") {
		t.Errorf("expected bullet prefix preserved, got %q", clockLine)
	}
}

func TestClockInIndented(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "- TODO: Parent task\n  - DOING: Child task\n    Some notes\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "DOING",
		Title:       "Child task",
		FilePath:    f,
		LineNum:     2,
		IndentLevel: 4, // "  - " = 4 bytes
	}

	err := ClockIn(task)
	if err != nil {
		t.Fatalf("clock in failed: %v", err)
	}

	result, _ := os.ReadFile(f)
	lines := strings.Split(string(result), "\n")
	// Line 2 (index 2) should be the clock entry, indented 6 spaces (4 + 2)
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d: %v", len(lines), lines)
	}
	clockLine := lines[2]
	if !strings.HasPrefix(clockLine, "      * CLOCK:") {
		t.Errorf("expected 6-space indent with bullet, got %q", clockLine)
	}
}

func TestParseCompletionEntries(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := `TODO: Weekly meeting @s:2026-07-13+1w
  * CLOCK: 2026-07-06T09:00--2026-07-06T10:15
  * COMPLETED: 2026-07-06T10:15
  * CLOCK: 2026-06-29T09:00--2026-06-29T10:00
  * COMPLETED: 2026-06-29T10:00
`
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "Weekly meeting",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	entries, err := ParseCompletionEntries(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Timestamp.Day() != 6 || entries[0].Timestamp.Month() != 7 {
		t.Errorf("first entry: got %v", entries[0].Timestamp)
	}
	if entries[1].Timestamp.Day() != 29 || entries[1].Timestamp.Month() != 6 {
		t.Errorf("second entry: got %v", entries[1].Timestamp)
	}
}

func TestParseCompletionEntriesNone(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "TODO: No completions\n  * CLOCK: 2026-06-29T09:00--2026-06-29T10:00\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:  "TODO",
		Title:    "No completions",
		FilePath: f,
		LineNum:  1,
	}

	entries, err := ParseCompletionEntries(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestRecordCompletion(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "TODO: Weekly meeting @s:2026-07-06+1w\n  Some notes\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "Weekly meeting",
		FilePath:    f,
		LineNum:     1,
		IndentLevel: 0,
	}

	err := RecordCompletion(task)
	if err != nil {
		t.Fatalf("record completion failed: %v", err)
	}

	result, _ := os.ReadFile(f)
	lines := strings.Split(string(result), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	completedLine := strings.TrimSpace(lines[1])
	if !strings.HasPrefix(completedLine, "* COMPLETED:") {
		t.Errorf("expected * COMPLETED line, got %q", completedLine)
	}

	now := time.Now().Format("2006-01-02T15:04")
	if !strings.Contains(completedLine, now) {
		t.Errorf("expected current timestamp %s in line %q", now, completedLine)
	}
}

func TestRecordCompletionIndented(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.md")
	content := "- TODO: Parent\n  - TODO: Child @s:2026-07-06+1d\n    Notes\n"
	os.WriteFile(f, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "Child",
		FilePath:    f,
		LineNum:     2,
		IndentLevel: 4,
	}

	err := RecordCompletion(task)
	if err != nil {
		t.Fatalf("record completion failed: %v", err)
	}

	result, _ := os.ReadFile(f)
	lines := strings.Split(string(result), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}
	// Should be indented 6 spaces (4 + 2)
	if !strings.HasPrefix(lines[2], "      * COMPLETED:") {
		t.Errorf("expected 6-space indent, got %q", lines[2])
	}
}
