package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSchedule_DateOnly(t *testing.T) {
	s, err := ParseSchedule("2025-03-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.HasTime {
		t.Error("expected HasTime=false")
	}
	if s.Recurrence != nil {
		t.Error("expected no recurrence")
	}
	if s.Warning != nil {
		t.Error("expected no warning")
	}
	if s.Date.Year() != 2025 || s.Date.Month() != 3 || s.Date.Day() != 15 {
		t.Errorf("unexpected date: %v", s.Date)
	}
}

func TestParseSchedule_DateTime(t *testing.T) {
	s, err := ParseSchedule("2025-03-15T14:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.HasTime {
		t.Error("expected HasTime=true")
	}
	if s.Date.Hour() != 14 || s.Date.Minute() != 30 {
		t.Errorf("unexpected time: %02d:%02d", s.Date.Hour(), s.Date.Minute())
	}
}

func TestParseSchedule_Recurrence(t *testing.T) {
	tests := []struct {
		input    string
		mode     RecurrenceMode
		interval int
		unit     byte
	}{
		{"2025-03-15+1w", RecurrenceFixed, 1, 'w'},
		{"2025-03-15+2d", RecurrenceFixed, 2, 'd'},
		{"2025-03-15.+1d", RecurrenceFromDone, 1, 'd'},
		{"2025-03-15++3m", RecurrenceNextFuture, 3, 'm'},
		{"2025-03-15T09:00+1w", RecurrenceFixed, 1, 'w'},
		{"2025-03-15T09:00.+2w", RecurrenceFromDone, 2, 'w'},
		{"2025-03-15++1y", RecurrenceNextFuture, 1, 'y'},
	}

	for _, tc := range tests {
		s, err := ParseSchedule(tc.input)
		if err != nil {
			t.Errorf("ParseSchedule(%q) error: %v", tc.input, err)
			continue
		}
		if s.Recurrence == nil {
			t.Errorf("ParseSchedule(%q): expected recurrence", tc.input)
			continue
		}
		if s.Recurrence.Mode != tc.mode {
			t.Errorf("ParseSchedule(%q): mode=%d, want %d", tc.input, s.Recurrence.Mode, tc.mode)
		}
		if s.Recurrence.Interval != tc.interval {
			t.Errorf("ParseSchedule(%q): interval=%d, want %d", tc.input, s.Recurrence.Interval, tc.interval)
		}
		if s.Recurrence.Unit != tc.unit {
			t.Errorf("ParseSchedule(%q): unit=%c, want %c", tc.input, s.Recurrence.Unit, tc.unit)
		}
	}
}

func TestParseSchedule_Warning(t *testing.T) {
	s, err := ParseSchedule("2025-03-21!3d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Warning == nil {
		t.Fatal("expected warning")
	}
	if s.Warning.Days != 3 {
		t.Errorf("warning days=%d, want 3", s.Warning.Days)
	}
}

func TestParseSchedule_Full(t *testing.T) {
	// Date + time + recurrence + warning
	s, err := ParseSchedule("2025-03-15T14:00+1w!2d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.HasTime {
		t.Error("expected HasTime=true")
	}
	if s.Recurrence == nil || s.Recurrence.Mode != RecurrenceFixed || s.Recurrence.Interval != 1 || s.Recurrence.Unit != 'w' {
		t.Errorf("unexpected recurrence: %+v", s.Recurrence)
	}
	if s.Warning == nil || s.Warning.Days != 2 {
		t.Errorf("unexpected warning: %+v", s.Warning)
	}
}

func TestParseSchedule_Invalid(t *testing.T) {
	tests := []string{"", "not-a-date", "2025", "03-15"}
	for _, tc := range tests {
		_, err := ParseSchedule(tc)
		if err == nil {
			t.Errorf("ParseSchedule(%q) expected error, got nil", tc)
		}
	}
}

func TestParseSchedule_EndTime(t *testing.T) {
	s, err := ParseSchedule("2025-06-17T09:00-10:30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.HasTime {
		t.Error("expected HasTime=true")
	}
	if !s.HasEnd {
		t.Error("expected HasEnd=true")
	}
	if s.Date.Hour() != 9 || s.Date.Minute() != 0 {
		t.Errorf("unexpected start time: %02d:%02d", s.Date.Hour(), s.Date.Minute())
	}
	if s.EndTime.Hour() != 10 || s.EndTime.Minute() != 30 {
		t.Errorf("unexpected end time: %02d:%02d", s.EndTime.Hour(), s.EndTime.Minute())
	}
}

func TestParseSchedule_EndTimeWithRecurrence(t *testing.T) {
	s, err := ParseSchedule("2025-06-17T09:00-10:30+1w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.HasEnd {
		t.Error("expected HasEnd=true")
	}
	if s.EndTime.Hour() != 10 || s.EndTime.Minute() != 30 {
		t.Errorf("unexpected end time: %02d:%02d", s.EndTime.Hour(), s.EndTime.Minute())
	}
	if s.Recurrence == nil || s.Recurrence.Interval != 1 || s.Recurrence.Unit != 'w' {
		t.Errorf("unexpected recurrence: %+v", s.Recurrence)
	}
}

func TestParseSchedule_EndTimeWithWarning(t *testing.T) {
	s, err := ParseSchedule("2025-06-17T09:00-11:00+1w!2d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.HasEnd {
		t.Error("expected HasEnd=true")
	}
	if s.EndTime.Hour() != 11 {
		t.Errorf("unexpected end hour: %d", s.EndTime.Hour())
	}
	if s.Recurrence == nil {
		t.Error("expected recurrence")
	}
	if s.Warning == nil || s.Warning.Days != 2 {
		t.Errorf("unexpected warning: %+v", s.Warning)
	}
}

func TestFormatToken_RoundTrip(t *testing.T) {
	tokens := []string{
		"2025-03-15",
		"2025-03-15T14:00",
		"2025-03-15T09:00-10:30",
		"2025-03-15+1w",
		"2025-03-15.+1d",
		"2025-03-15++2m",
		"2025-03-15T09:00+1w",
		"2025-03-15T09:00-10:30+1w",
		"2025-03-21!3d",
		"2025-03-15T14:00+1w!2d",
		"2025-03-15T14:00-15:30+1w!2d",
	}
	for _, tok := range tokens {
		s, err := ParseSchedule(tok)
		if err != nil {
			t.Errorf("ParseSchedule(%q) error: %v", tok, err)
			continue
		}
		out := s.FormatToken()
		if out != tok {
			t.Errorf("round-trip failed: %q -> %q", tok, out)
		}
	}
}

func TestNextOccurrence_Fixed(t *testing.T) {
	s, _ := ParseSchedule("2025-03-15+1w")
	next := s.NextOccurrence(time.Now())
	expected := time.Date(2025, 3, 22, 0, 0, 0, 0, time.Local)
	if !next.Equal(expected) {
		t.Errorf("next=%v, want %v", next, expected)
	}
}

func TestNextOccurrence_FromDone(t *testing.T) {
	s, _ := ParseSchedule("2025-03-15.+3d")
	completion := time.Date(2025, 6, 10, 0, 0, 0, 0, time.Local)
	next := s.NextOccurrence(completion)
	expected := time.Date(2025, 6, 13, 0, 0, 0, 0, time.Local)
	if !next.Equal(expected) {
		t.Errorf("next=%v, want %v", next, expected)
	}
}

func TestNextOccurrence_NextFuture(t *testing.T) {
	// A task scheduled far in the past with ++1w should jump to a future date
	s, _ := ParseSchedule("2020-01-06++1w")
	next := s.NextOccurrence(time.Now())
	if !next.After(time.Now()) {
		t.Errorf("expected future date, got %v", next)
	}
	// Should be a Monday (2020-01-06 was Monday, +1w stays on Mondays)
	if next.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %v", next.Weekday())
	}
}

func TestExpandOccurrences_NonRecurring(t *testing.T) {
	s, _ := ParseSchedule("2025-06-15")
	start := time.Date(2025, 6, 10, 0, 0, 0, 0, time.Local)
	end := time.Date(2025, 6, 20, 0, 0, 0, 0, time.Local)
	occ := s.ExpandOccurrences(start, end)
	if len(occ) != 1 {
		t.Fatalf("expected 1 occurrence, got %d", len(occ))
	}
}

func TestExpandOccurrences_OutOfRange(t *testing.T) {
	s, _ := ParseSchedule("2025-06-15")
	start := time.Date(2025, 7, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2025, 7, 7, 0, 0, 0, 0, time.Local)
	occ := s.ExpandOccurrences(start, end)
	if len(occ) != 0 {
		t.Fatalf("expected 0 occurrences, got %d", len(occ))
	}
}

func TestExpandOccurrences_Weekly(t *testing.T) {
	s, _ := ParseSchedule("2025-06-02+1w")
	start := time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(2025, 6, 30, 0, 0, 0, 0, time.Local)
	occ := s.ExpandOccurrences(start, end)
	// June 2, 9, 16, 23, 30
	if len(occ) != 5 {
		t.Errorf("expected 5 occurrences, got %d: %v", len(occ), occ)
	}
}

func TestExpandOccurrences_FromDoneOnlyShowsCurrent(t *testing.T) {
	s, _ := ParseSchedule("2025-06-15.+1d")
	start := time.Date(2025, 6, 10, 0, 0, 0, 0, time.Local)
	end := time.Date(2025, 6, 30, 0, 0, 0, 0, time.Local)
	occ := s.ExpandOccurrences(start, end)
	// .+ mode only shows the stored date, not expanded series
	if len(occ) != 1 {
		t.Errorf("expected 1 occurrence for .+ mode, got %d", len(occ))
	}
}

func TestAddMonths_Overflow(t *testing.T) {
	// Jan 31 + 1 month should be Feb 28 (non-leap year)
	jan31 := time.Date(2025, 1, 31, 0, 0, 0, 0, time.Local)
	result := addMonths(jan31, 1)
	if result.Month() != 2 || result.Day() != 28 {
		t.Errorf("expected Feb 28, got %v", result)
	}

	// Jan 31 + 1 month in leap year should be Feb 29
	jan31Leap := time.Date(2024, 1, 31, 0, 0, 0, 0, time.Local)
	result = addMonths(jan31Leap, 1)
	if result.Month() != 2 || result.Day() != 29 {
		t.Errorf("expected Feb 29, got %v", result)
	}
}

func TestCompleteRecurringTask(t *testing.T) {
	// Create a temp file with a recurring task
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.md")
	content := "TODO: Weekly meeting @s:2025-03-15+1w\n"
	os.WriteFile(fp, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "Weekly meeting",
		ScheduledAt: "2025-03-15+1w",
		FilePath:    fp,
		LineNum:     1,
		IndentLevel: 0,
	}

	cfg := createTestConfig()
	advanced, err := CompleteRecurringTask(task, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !advanced {
		t.Fatal("expected advanced=true")
	}

	// Verify file was updated
	data, _ := os.ReadFile(fp)
	fileContent := string(data)
	if !contains(fileContent, "2025-03-22+1w") {
		t.Errorf("expected date advanced to 2025-03-22, got: %s", fileContent)
	}
	// Keyword should NOT have changed
	if !contains(fileContent, "TODO:") {
		t.Errorf("keyword should remain TODO, got: %s", fileContent)
	}
	// Should have a COMPLETED entry
	if !contains(fileContent, "* COMPLETED:") {
		t.Errorf("expected COMPLETED entry, got: %s", fileContent)
	}
}

func TestCompleteRecurringTask_AutoClockOut(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.md")
	content := "TODO: Daily standup @s:2025-06-20+1d\n  * CLOCK: 2025-06-20T09:00--\n"
	os.WriteFile(fp, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "Daily standup",
		ScheduledAt: "2025-06-20+1d",
		FilePath:    fp,
		LineNum:     1,
		IndentLevel: 0,
	}

	cfg := createTestConfig()
	advanced, err := CompleteRecurringTask(task, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !advanced {
		t.Fatal("expected advanced=true")
	}

	data, _ := os.ReadFile(fp)
	fileContent := string(data)

	// Clock should be closed (no trailing --)
	lines := strings.Split(fileContent, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if contains(trimmed, "CLOCK:") && strings.HasSuffix(trimmed, "--") {
			t.Errorf("expected clock to be auto-closed, found open: %s", trimmed)
		}
	}

	// Should have COMPLETED entry
	if !contains(fileContent, "* COMPLETED:") {
		t.Errorf("expected COMPLETED entry, got: %s", fileContent)
	}
	// Date should be advanced
	if !contains(fileContent, "2025-06-21+1d") {
		t.Errorf("expected date advanced to 2025-06-21, got: %s", fileContent)
	}
}

func TestCompleteRecurringTask_NonRecurring(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.md")
	content := "TODO: One-off task @s:2025-03-15\n"
	os.WriteFile(fp, []byte(content), 0644)

	task := &Task{
		Keyword:     "TODO",
		Title:       "One-off task",
		ScheduledAt: "2025-03-15",
		FilePath:    fp,
	}

	cfg := createTestConfig()
	advanced, err := CompleteRecurringTask(task, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if advanced {
		t.Fatal("expected advanced=false for non-recurring task")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
