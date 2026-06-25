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
	Task     *Task
	Duration time.Duration
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

var clockLineRe = regexp.MustCompile(`^\s*(?:[-*+]\s*)?CLOCK:\s*(.+)$`)
var clockTimestampRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})--(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})?$`)

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

	for _, line := range lines[1:] { // skip task line itself
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

		projectMap[t.Project] = append(projectMap[t.Project], ClockTableEntry{
			Task:     t,
			Duration: total,
		})
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

	indent := strings.Repeat(" ", t.IndentLevel+2)
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

// FormatDuration formats a duration as H:MM.
func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%d:%02d", h, m)
}
