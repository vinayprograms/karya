package task

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vinayprograms/karya/internal/config"
)

type RecurrenceMode int

const (
	RecurrenceFixed      RecurrenceMode = iota // +N[dwmy] — advance from original date
	RecurrenceFromDone                         // .+N[dwmy] — advance from completion date
	RecurrenceNextFuture                       // ++N[dwmy] — next future occurrence from series
)

type RecurrenceSpec struct {
	Mode     RecurrenceMode
	Interval int
	Unit     byte // 'd', 'w', 'm', 'y'
}

type WarningSpec struct {
	Days int
}

type Schedule struct {
	Date       time.Time
	HasTime    bool
	EndTime    time.Time
	HasEnd     bool
	Recurrence *RecurrenceSpec
	Warning    *WarningSpec
	Raw        string
}

// ParseSchedule parses a date token (the part after @s: or @d:) into a Schedule.
// Token grammar: DATE[TTIME][RECURRENCE][WARNING]
// DATE: YYYY-MM-DD
// TIME: THH:MM
// RECURRENCE: +N[dwmyb] | .+N[dwmyb] | ++N[dwmyb]
// WARNING: !Nd
func ParseSchedule(raw string) (*Schedule, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty date token")
	}

	s := &Schedule{Raw: raw}
	remaining := raw

	// Extract warning suffix (!Nd) from end
	warningRe := regexp.MustCompile(`!(\d+)d$`)
	if m := warningRe.FindStringSubmatch(remaining); len(m) > 0 {
		days, _ := strconv.Atoi(m[1])
		s.Warning = &WarningSpec{Days: days}
		remaining = remaining[:len(remaining)-len(m[0])]
	}

	// Extract recurrence suffix. Find the recurrence marker after the date portion.
	// Date is at least 10 chars (YYYY-MM-DD), time adds 6 (THH:MM) = 16.
	// Look for .+, ++, or + (in that order to avoid prefix conflicts).
	recurrenceRe := regexp.MustCompile(`(\.\+|\+\+|\+)(\d+)([dwmyb])$`)
	if m := recurrenceRe.FindStringSubmatch(remaining); len(m) > 0 {
		matchStart := strings.LastIndex(remaining, m[0])
		// Only treat as recurrence if it's after the date portion (pos >= 10)
		if matchStart >= 10 {
			interval, _ := strconv.Atoi(m[2])
			var mode RecurrenceMode
			switch m[1] {
			case ".+":
				mode = RecurrenceFromDone
			case "++":
				mode = RecurrenceNextFuture
			default:
				mode = RecurrenceFixed
			}
			s.Recurrence = &RecurrenceSpec{
				Mode:     mode,
				Interval: interval,
				Unit:     m[3][0],
			}
			remaining = remaining[:matchStart]
		}
	}

	// Parse date and optional time from remaining
	// Supports: YYYY-MM-DD, YYYY-MM-DDTHH:MM, YYYY-MM-DDTHH:MM-HH:MM (with end time)
	if len(remaining) >= 16 && remaining[10] == 'T' {
		// Date + Time: YYYY-MM-DDTHH:MM[-HH:MM]
		t, err := time.ParseInLocation("2006-01-02T15:04", remaining[:16], time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid datetime %q: %w", remaining, err)
		}
		s.Date = t
		s.HasTime = true

		// Check for end time: -HH:MM
		if len(remaining) >= 22 && remaining[16] == '-' {
			endStr := remaining[17:22]
			endT, err := time.ParseInLocation("15:04", endStr, time.Local)
			if err == nil {
				s.EndTime = time.Date(t.Year(), t.Month(), t.Day(), endT.Hour(), endT.Minute(), 0, 0, time.Local)
				s.HasEnd = true
			}
		}
	} else if len(remaining) >= 10 {
		// Date only: YYYY-MM-DD
		t, err := time.ParseInLocation("2006-01-02", remaining[:10], time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid date %q: %w", remaining, err)
		}
		s.Date = t
		s.HasTime = false
	} else {
		return nil, fmt.Errorf("date token too short: %q", remaining)
	}

	return s, nil
}

// FormatToken reconstructs the date token string from the Schedule.
func (s *Schedule) FormatToken() string {
	var b strings.Builder

	if s.HasTime {
		b.WriteString(s.Date.Format("2006-01-02T15:04"))
		if s.HasEnd {
			b.WriteByte('-')
			b.WriteString(s.EndTime.Format("15:04"))
		}
	} else {
		b.WriteString(s.Date.Format("2006-01-02"))
	}

	if s.Recurrence != nil {
		switch s.Recurrence.Mode {
		case RecurrenceFromDone:
			b.WriteString(".+")
		case RecurrenceNextFuture:
			b.WriteString("++")
		default:
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(s.Recurrence.Interval))
		b.WriteByte(s.Recurrence.Unit)
	}

	if s.Warning != nil {
		b.WriteByte('!')
		b.WriteString(strconv.Itoa(s.Warning.Days))
		b.WriteByte('d')
	}

	return b.String()
}

// AddInterval adds N units to the given time, handling month overflow.
func AddInterval(t time.Time, interval int, unit byte) time.Time {
	return addInterval(t, interval, unit)
}

func addInterval(t time.Time, interval int, unit byte) time.Time {
	switch unit {
	case 'd':
		return t.AddDate(0, 0, interval)
	case 'w':
		return t.AddDate(0, 0, interval*7)
	case 'm':
		return addMonths(t, interval)
	case 'y':
		return t.AddDate(interval, 0, 0)
	case 'b':
		return addBusinessDays(t, interval)
	}
	return t
}

// addBusinessDays advances by N business days, skipping weekends.
func addBusinessDays(t time.Time, days int) time.Time {
	result := t
	for days > 0 {
		result = result.AddDate(0, 0, 1)
		if result.Weekday() != time.Saturday && result.Weekday() != time.Sunday {
			days--
		}
	}
	return result
}

// addMonths adds N months, capping to last day of target month on overflow.
func addMonths(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	targetMonth := time.Month(int(m) + months)
	targetYear := y
	for targetMonth > 12 {
		targetMonth -= 12
		targetYear++
	}
	for targetMonth < 1 {
		targetMonth += 12
		targetYear--
	}
	// Find last day of target month
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(targetYear, targetMonth, d, t.Hour(), t.Minute(), t.Second(), 0, t.Location())
}

// NextOccurrence computes the next occurrence date based on the recurrence mode.
func (s *Schedule) NextOccurrence(completionDate time.Time) time.Time {
	if s.Recurrence == nil {
		return s.Date
	}

	r := s.Recurrence
	switch r.Mode {
	case RecurrenceFixed:
		// Advance from original date by one interval
		return addInterval(s.Date, r.Interval, r.Unit)
	case RecurrenceFromDone:
		// Advance from completion date
		return addInterval(completionDate, r.Interval, r.Unit)
	case RecurrenceNextFuture:
		// Keep advancing from original date until we're in the future
		now := time.Now()
		next := s.Date
		for !next.After(now) {
			next = addInterval(next, r.Interval, r.Unit)
		}
		return next
	}
	return s.Date
}

// ExpandOccurrences generates all dates where this recurring schedule appears
// within [rangeStart, rangeEnd]. For non-recurring schedules, returns the single date
// if it falls within range. For .+ mode, only returns the stored date (cannot predict future).
// Range comparisons use calendar-day granularity so timed tasks are included when
// their day falls within the range, regardless of the time-of-day component.
func (s *Schedule) ExpandOccurrences(rangeStart, rangeEnd time.Time) []time.Time {
	startDay := truncateToDay(rangeStart)
	endDay := truncateToDay(rangeEnd)

	if s.Recurrence == nil {
		day := truncateToDay(s.Date)
		if !day.Before(startDay) && !day.After(endDay) {
			return []time.Time{s.Date}
		}
		return nil
	}

	// .+ mode: can only show the current stored date (next depends on completion)
	if s.Recurrence.Mode == RecurrenceFromDone {
		day := truncateToDay(s.Date)
		if !day.Before(startDay) && !day.After(endDay) {
			return []time.Time{s.Date}
		}
		return nil
	}

	// + and ++ modes: expand from original date forward
	var occurrences []time.Time
	current := s.Date
	r := s.Recurrence

	// Advance past dates before rangeStart
	for truncateToDay(current).Before(startDay) {
		current = addInterval(current, r.Interval, r.Unit)
		if truncateToDay(current).After(endDay) {
			return nil
		}
	}

	// Collect occurrences within range
	for !truncateToDay(current).After(endDay) {
		occurrences = append(occurrences, current)
		current = addInterval(current, r.Interval, r.Unit)
		if len(occurrences) > 366 {
			break // safety cap
		}
	}

	return occurrences
}


// CompleteRecurringTask handles advancing a recurring task's date on completion.
// Returns advanced=true if the task was recurring and the date was advanced.
// If not recurring, returns false and the caller should proceed with normal completion.
func CompleteRecurringTask(t *Task, c *config.Config) (advanced bool, err error) {
	// Try scheduled date first, then due date
	dateField := t.ScheduledAt
	isScheduled := true
	if dateField == "" {
		dateField = t.DueAt
		isScheduled = false
	}
	if dateField == "" {
		return false, nil
	}

	sched, err := ParseSchedule(dateField)
	if err != nil || sched.Recurrence == nil {
		return false, nil
	}

	// Compute next occurrence
	now := time.Now()
	nextDate := sched.NextOccurrence(now)

	// Build new token
	newSched := &Schedule{
		Date:       nextDate,
		HasTime:    sched.HasTime,
		Recurrence: sched.Recurrence,
		Warning:    sched.Warning,
	}
	newToken := newSched.FormatToken()
	oldToken := dateField

	// Replace in file
	if err := advanceDateInFile(t, oldToken, newToken, isScheduled); err != nil {
		return false, fmt.Errorf("failed to advance date: %w", err)
	}

	// Update in memory
	if isScheduled {
		t.ScheduledAt = newToken
	} else {
		t.DueAt = newToken
	}

	return true, nil
}

// advanceDateInFile replaces the date token in the task's source file line.
func advanceDateInFile(t *Task, oldToken, newToken string, isScheduled bool) error {
	if t.FilePath == "" {
		return fmt.Errorf("task has no file path")
	}

	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Build search prefix (same logic as UpdateTaskStatus)
	var searchPrefix string
	if t.ID != "" {
		searchPrefix = fmt.Sprintf("%s: [%s] %s", t.Keyword, t.ID, t.Title)
	} else {
		searchPrefix = fmt.Sprintf("%s: %s", t.Keyword, t.Title)
	}

	found := false
	for i, line := range lines {
		stripped, _ := StripLinePrefix(line)
		if strings.HasPrefix(stripped, searchPrefix) {
			// Found the line — replace the date token
			if isScheduled {
				if strings.Contains(line, "@s:"+oldToken) {
					lines[i] = strings.Replace(line, "@s:"+oldToken, "@s:"+newToken, 1)
					found = true
				} else if strings.Contains(line, "@"+oldToken) {
					lines[i] = strings.Replace(line, "@"+oldToken, "@"+newToken, 1)
					found = true
				}
			} else {
				if strings.Contains(line, "@d:"+oldToken) {
					lines[i] = strings.Replace(line, "@d:"+oldToken, "@d:"+newToken, 1)
					found = true
				}
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("task line not found or date token not matched")
	}

	newContent := strings.Join(lines, "\n")
	return os.WriteFile(t.FilePath, []byte(newContent), 0644)
}
