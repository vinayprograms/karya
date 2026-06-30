package task

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/vinayprograms/karya/internal/config"
)

type ClockEntry struct {
	Start time.Time
	End   time.Time
	Open  bool
}

type ClockTableEntry struct {
	Task         *Task
	Duration     time.Duration
	WasCompleted bool
}

type ClockTableProject struct {
	Project string
	Total   time.Duration
	Entries []ClockTableEntry
}

type ClockTable struct {
	GrandTotal time.Duration
	Projects   []ClockTableProject
}

type CompletionEntry struct {
	Timestamp time.Time
}

var clockLineRe = regexp.MustCompile(`^\s*(?:[-*+]\s*)?CLOCK:\s*(.+)$`)
var clockTimestampRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})--(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})?$`)
var completedLineRe = regexp.MustCompile(`^\s*(?:[-*+]\s*)?COMPLETED:\s*(.+)$`)

// ParseClockEntries reads a task's sub-lines and extracts CLOCK entries.
func ParseClockEntries(t *Task) ([]ClockEntry, error) {
	raw, err := ReadRawBlock(t)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	lines := strings.Split(raw, "\n")
	var entries []ClockEntry
	expectedRawIndent := detectSubItemRawIndent(lines)
	if expectedRawIndent < 0 {
		return nil, nil
	}

	for _, line := range lines[1:] { // skip task line itself
		if line == "" {
			continue
		}
		if countLeadingSpaces(line) != expectedRawIndent {
			continue
		}
		m := clockLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ts := strings.TrimSpace(m[1])
		tm := clockTimestampRe.FindStringSubmatch(ts)
		if tm == nil {
			continue
		}

		start, err := time.ParseInLocation("2006-01-02T15:04", tm[1], time.Local)
		if err != nil {
			continue
		}

		entry := ClockEntry{Start: start}
		if tm[2] != "" {
			end, err := time.ParseInLocation("2006-01-02T15:04", tm[2], time.Local)
			if err == nil {
				entry.End = end
			}
		} else {
			entry.Open = true
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func countLeadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

// detectSubItemRawIndent returns the raw leading-whitespace count of the
// task's direct sub-items. It finds the first line whose StripLinePrefix
// level exceeds the task's, then returns that line's countLeadingSpaces.
// This correctly handles both bulleted ("  * CLOCK:") and non-bulleted
// ("  CLOCK:") entries at the same raw indent. Returns -1 if no sub-items.
func detectSubItemRawIndent(lines []string) int {
	if len(lines) == 0 {
		return -1
	}
	_, taskLevel := StripLinePrefix(lines[0])
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		_, level := StripLinePrefix(line)
		if level > taskLevel {
			return countLeadingSpaces(line)
		}
	}
	return -1
}

// subItemWriteIndent determines the number of leading spaces to use when
// writing a bulleted sub-item (CLOCK, COMPLETED) under the task.
// Uses detectSubItemRawIndent if sub-items exist, otherwise falls back to
// raw whitespace detection and then a computed default.
func subItemWriteIndent(lines []string) int {
	if len(lines) == 0 {
		return 2
	}
	if indent := detectSubItemRawIndent(lines); indent >= 0 {
		return indent
	}
	// Fallback: find first line with more raw whitespace than task
	taskRawSpaces := countLeadingSpaces(lines[0])
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		ws := countLeadingSpaces(line)
		if ws > taskRawSpaces {
			return ws
		}
	}
	return taskRawSpaces + 2
}

// subItemIndentForTask reads the task's raw block and returns the indent string
// to use when writing direct sub-items (CLOCK, COMPLETED entries).
func subItemIndentForTask(t *Task) (string, error) {
	raw, err := ReadRawBlock(t)
	if err != nil {
		return strings.Repeat(" ", countLeadingSpaces("")+2), err
	}
	if raw == "" {
		// No existing block — use task line's raw whitespace + 2
		content, err := os.ReadFile(t.FilePath)
		if err != nil {
			return "  ", err
		}
		filelines := strings.Split(string(content), "\n")
		if t.LineNum-1 < len(filelines) {
			ws := countLeadingSpaces(filelines[t.LineNum-1])
			return strings.Repeat(" ", ws+2), nil
		}
		return "  ", nil
	}
	lines := strings.Split(raw, "\n")
	indent := subItemWriteIndent(lines)
	return strings.Repeat(" ", indent), nil
}

// IsClockActive returns true if the task has an open (running) clock entry.
func IsClockActive(t *Task) bool {
	entries, err := ParseClockEntries(t)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Open {
			return true
		}
	}
	return false
}

// FindActiveClocks returns all tasks that have an open (running) clock entry.
func FindActiveClocks(tasks []*Task) []*Task {
	var active []*Task
	for _, t := range tasks {
		if IsClockActive(t) {
			active = append(active, t)
		}
	}
	return active
}

// ClipDuration returns the duration of a clock entry clipped to [rangeStart, rangeEnd].
// Open entries use time.Now() as end. Returns 0 if entry doesn't overlap the range.
func ClipDuration(entry ClockEntry, rangeStart, rangeEnd time.Time) time.Duration {
	start := entry.Start
	end := entry.End
	if entry.Open {
		end = time.Now()
	}
	if end.IsZero() {
		return 0
	}

	// No overlap
	if end.Before(rangeStart) || start.After(rangeEnd) {
		return 0
	}

	// Clip to range
	if start.Before(rangeStart) {
		start = rangeStart
	}
	if end.After(rangeEnd) {
		end = rangeEnd
	}

	d := end.Sub(start)
	if d < 0 {
		return 0
	}
	return d
}

// QueryClockTable aggregates clock data for all tasks within [start, end].
func QueryClockTable(c *config.Config, start, end time.Time) (*ClockTable, error) {
	tasks, err := ListTasks(c, "", true)
	if err != nil {
		return nil, err
	}

	// Extend end to cover full last day
	rangeEnd := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, time.Local)

	projectMap := make(map[string][]ClockTableEntry)

	for _, t := range tasks {
		entries, err := ParseClockEntries(t)
		if err != nil || len(entries) == 0 {
			continue
		}

		var total time.Duration
		for _, e := range entries {
			total += ClipDuration(e, start, rangeEnd)
		}
		if total == 0 {
			continue
		}

		entry := ClockTableEntry{
			Task:     t,
			Duration: total,
		}

		// Check if this recurring task was completed during the view range
		dateField := t.ScheduledAt
		if dateField == "" {
			dateField = t.DueAt
		}
		if dateField != "" {
			if sched, err := ParseSchedule(dateField); err == nil && sched.Recurrence != nil {
				if completions, err := ParseCompletionEntries(t); err == nil {
					for _, comp := range completions {
						if !comp.Timestamp.Before(start) && !comp.Timestamp.After(rangeEnd) {
							entry.WasCompleted = true
							break
						}
					}
				}
			}
		}

		projectMap[t.Project] = append(projectMap[t.Project], entry)
	}

	var table ClockTable
	for proj, entries := range projectMap {
		// Sort entries by duration descending
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Duration > entries[j].Duration
		})

		var projTotal time.Duration
		for _, e := range entries {
			projTotal += e.Duration
		}

		table.Projects = append(table.Projects, ClockTableProject{
			Project: proj,
			Total:   projTotal,
			Entries: entries,
		})
		table.GrandTotal += projTotal
	}

	// Sort projects by total descending
	sort.Slice(table.Projects, func(i, j int) bool {
		return table.Projects[i].Total > table.Projects[j].Total
	})

	return &table, nil
}

// ClockIn appends a new open CLOCK entry after the task line.
// Returns error if task already has an active clock.
func ClockIn(t *Task) error {
	if IsClockActive(t) {
		return fmt.Errorf("task already clocked in")
	}
	if t.FilePath == "" || t.LineNum == 0 {
		return fmt.Errorf("task has no file location")
	}

	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if t.LineNum-1 >= len(lines) {
		return fmt.Errorf("line number out of range")
	}

	indent, _ := subItemIndentForTask(t)
	clockLine := fmt.Sprintf("%s* CLOCK: %s--", indent, time.Now().Format("2006-01-02T15:04"))

	// Insert after task line
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:t.LineNum]...)
	newLines = append(newLines, clockLine)
	newLines = append(newLines, lines[t.LineNum:]...)

	return os.WriteFile(t.FilePath, []byte(strings.Join(newLines, "\n")), 0644)
}

// ClockOut completes the open CLOCK entry for the task.
// Returns error if no active clock found.
func ClockOut(t *Task) error {
	if t.FilePath == "" || t.LineNum == 0 {
		return fmt.Errorf("task has no file location")
	}

	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	taskIdx := t.LineNum - 1
	if taskIdx >= len(lines) {
		return fmt.Errorf("line number out of range")
	}

	now := time.Now().Format("2006-01-02T15:04")
	found := false
	for i := taskIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		_, level := StripLinePrefix(line)
		if level <= t.IndentLevel {
			break
		}
		trimmed := strings.TrimSpace(line)
		// Strip optional bullet prefix for matching
		stripped := trimmed
		if strings.HasPrefix(stripped, "* ") || strings.HasPrefix(stripped, "- ") || strings.HasPrefix(stripped, "+ ") {
			stripped = stripped[2:]
		}
		if strings.HasPrefix(stripped, "CLOCK:") && strings.HasSuffix(stripped, "--") {
			lines[i] = line + now
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("no active clock entry found")
	}

	return os.WriteFile(t.FilePath, []byte(strings.Join(lines, "\n")), 0644)
}

// ParseCompletionEntries reads a task's sub-lines and extracts COMPLETED entries.
func ParseCompletionEntries(t *Task) ([]CompletionEntry, error) {
	raw, err := ReadRawBlock(t)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	lines := strings.Split(raw, "\n")
	var entries []CompletionEntry
	expectedRawIndent := detectSubItemRawIndent(lines)
	if expectedRawIndent < 0 {
		return nil, nil
	}

	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		if countLeadingSpaces(line) != expectedRawIndent {
			continue
		}
		m := completedLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ts := strings.TrimSpace(m[1])
		parsed, err := time.ParseInLocation("2006-01-02T15:04", ts, time.Local)
		if err != nil {
			continue
		}
		entries = append(entries, CompletionEntry{Timestamp: parsed})
	}

	return entries, nil
}

// recordOrUpdateCompletion records a completion for the given scheduled day.
// If a COMPLETED entry already exists for that day, its timestamp is updated.
// Otherwise a new entry is appended.
func recordOrUpdateCompletion(t *Task, schedDay time.Time) error {
	if t.FilePath == "" || t.LineNum == 0 {
		return fmt.Errorf("task has no file location")
	}

	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	taskIdx := t.LineNum - 1
	if taskIdx >= len(lines) {
		return fmt.Errorf("line number out of range")
	}

	indent, _ := subItemIndentForTask(t)
	now := time.Now().Format("2006-01-02T15:04")
	dayStr := schedDay.Format("2006-01-02")

	// Search for an existing COMPLETED entry on the same day
	for i := taskIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		_, level := StripLinePrefix(line)
		if level <= t.IndentLevel {
			break
		}
		m := completedLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ts := strings.TrimSpace(m[1])
		if strings.HasPrefix(ts, dayStr) {
			lines[i] = fmt.Sprintf("%s* COMPLETED: %s", indent, now)
			return os.WriteFile(t.FilePath, []byte(strings.Join(lines, "\n")), 0644)
		}
	}

	// No existing entry for this day — append new one
	completedLine := fmt.Sprintf("%s* COMPLETED: %s", indent, now)

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:t.LineNum]...)
	newLines = append(newLines, completedLine)
	newLines = append(newLines, lines[t.LineNum:]...)

	return os.WriteFile(t.FilePath, []byte(strings.Join(newLines, "\n")), 0644)
}

// RecordCompletion appends a COMPLETED entry after the task line.
func RecordCompletion(t *Task) error {
	if t.FilePath == "" || t.LineNum == 0 {
		return fmt.Errorf("task has no file location")
	}

	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	if t.LineNum-1 >= len(lines) {
		return fmt.Errorf("line number out of range")
	}

	indent, _ := subItemIndentForTask(t)
	completedLine := fmt.Sprintf("%s* COMPLETED: %s", indent, time.Now().Format("2006-01-02T15:04"))

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:t.LineNum]...)
	newLines = append(newLines, completedLine)
	newLines = append(newLines, lines[t.LineNum:]...)

	return os.WriteFile(t.FilePath, []byte(strings.Join(newLines, "\n")), 0644)
}

// FormatDuration formats a duration as H:MM.
func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%d:%02d", h, m)
}
