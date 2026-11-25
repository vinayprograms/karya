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
	Rule       string
	Anchor     time.Time
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

	if raw[0] < '0' || raw[0] > '9' {
		return calendar(raw)
	}
	s := &Schedule{Raw: raw}
	remaining := raw

	warningRe := regexp.MustCompile(`!(\d+)d$`)
	if match := warningRe.FindStringSubmatch(remaining); len(match) > 0 {
		days, _ := strconv.Atoi(match[1])
		s.Warning = &WarningSpec{Days: days}
		remaining = remaining[:len(remaining)-len(match[0])]
	}

	recurrenceRe := regexp.MustCompile(`(\.\+|\+\+|\+)(\d+)([dwmyb])$`)
	if match := recurrenceRe.FindStringSubmatch(remaining); len(match) > 0 {
		matchStart := strings.LastIndex(remaining, match[0])
		if matchStart >= 10 {
			interval, err := strconv.Atoi(match[2])
			if err != nil || interval <= 0 {
				return nil, fmt.Errorf("invalid recurrence interval %q", match[2])
			}
			var mode RecurrenceMode
			switch match[1] {
			case ".+":
				mode = RecurrenceFromDone
			case "++":
				mode = RecurrenceNextFuture
			default:
				mode = RecurrenceFixed
			}
			s.Recurrence = &RecurrenceSpec{Mode: mode, Interval: interval, Unit: match[3][0]}
			remaining = remaining[:matchStart]
		}
	}

	dateLayouts := []string{"2006-01-02", "02-01-2006", "01-02-2006", "2006/01/02", "02/01/2006", "01/02/2006", "02.01.2006", "01.02.2006"}
	var parsed time.Time
	var parseErr error
	for _, layout := range dateLayouts {
		dateLength := len(layout)
		if len(remaining) < dateLength {
			continue
		}
		parsed, parseErr = time.ParseInLocation(layout, remaining[:dateLength], time.Local)
		if parseErr == nil {
			remaining = remaining[dateLength:]
			break
		}
	}
	if parseErr != nil || parsed.IsZero() {
		return nil, fmt.Errorf("invalid date %q: %w", remaining, parseErr)
	}
	s.Date = parsed
	if len(remaining) > 0 && remaining[0] == 'T' {
		remaining = remaining[1:]
		timeParts := regexp.MustCompile(`^(\d{1,2}:\d{2})(?:-(\d{1,2}:\d{2}))?$`).FindStringSubmatch(remaining)
		if timeParts == nil {
			return nil, fmt.Errorf("invalid time %q", remaining)
		}
		start, err := time.ParseInLocation("15:04", timeParts[1], time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid time %q: %w", timeParts[1], err)
		}
		s.Date = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), start.Hour(), start.Minute(), 0, 0, time.Local)
		s.HasTime = true
		if timeParts[2] != "" {
			end, err := time.ParseInLocation("15:04", timeParts[2], time.Local)
			if err != nil {
				return nil, fmt.Errorf("invalid end time %q: %w", timeParts[2], err)
			}
			s.EndTime = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), end.Hour(), end.Minute(), 0, 0, time.Local)
			s.HasEnd = true
		}
	} else if remaining != "" {
		return nil, fmt.Errorf("invalid date suffix %q", remaining)
	}
	return s, nil
}

// FormatToken reconstructs the date token string from the Schedule.
func (s *Schedule) FormatToken() string {
	if s.Rule != "" {
		return s.Raw
	}
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
	if s.Rule != "" {
		start := truncateToDay(completionDate).AddDate(0, 0, 1)
		end := start.AddDate(0, 0, 365*10)
		if s.Recurrence.Unit == 'y' {
			end = start.AddDate(10000, 0, 0)
		}
		occurrences := s.occurrences(start, end)
		if len(occurrences) > 0 {
			return occurrences[0]
		}
		return time.Time{}
	}
	if s.Recurrence == nil {
		return s.Date
	}
	r := s.Recurrence
	switch r.Mode {
	case RecurrenceFixed:
		return addInterval(s.Date, r.Interval, r.Unit)
	case RecurrenceFromDone:
		return addInterval(completionDate, r.Interval, r.Unit)
	case RecurrenceNextFuture:
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
	if s.Rule != "" {
		return s.occurrences(rangeStart, rangeEnd)
	}
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
// Auto-clocks-out if the task has an active clock, then records a LOG transition entry.
func CompleteRecurringTask(t *Task, c *config.Config, targetKeyword string) (advanced bool, err error) {
	if t.Event(c) {
		return false, fmt.Errorf("events do not have completion states")
	}
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

	if sched.Rule != "" {
		data, err := os.ReadFile(t.FilePath)
		if err != nil {
			return false, err
		}
		lines := strings.Split(string(data), "\n")
		if t.LineNum < 1 || t.LineNum > len(lines) {
			return false, fmt.Errorf("task location changed; reload before completing")
		}
		text, _ := StripLinePrefix(lines[t.LineNum-1])
		if t.RawTitle != "" && (!strings.HasPrefix(text, t.Keyword+":") || strings.TrimSpace(strings.TrimPrefix(text, t.Keyword+":")) != strings.TrimSpace(t.RawTitle)) {
			return false, fmt.Errorf("task changed; reload before completing")
		}
		if PendingChildren(t, c) > 0 {
			return false, ErrPendingChildren
		}
		field := "scheduled"
		if !isScheduled {
			field = "due"
		}
		if err := sched.anchor(t, field, time.Now(), true); err != nil {
			return false, err
		}
		if IsClockActive(t) {
			if err := ClockOut(t); err != nil {
				return false, err
			}
		}
		return true, recordOrUpdateTransition(t, truncateToDay(time.Now()), t.Keyword, targetKeyword)
	}

	// Auto clock-out if active
	if IsClockActive(t) {
		if err := ClockOut(t); err != nil {
			return false, fmt.Errorf("failed to auto clock-out: %w", err)
		}
	}

	// Record state transition — one entry per scheduled day.
	schedDay := time.Date(sched.Date.Year(), sched.Date.Month(), sched.Date.Day(), 0, 0, 0, 0, time.Local)
	if err := recordOrUpdateTransition(t, schedDay, t.Keyword, targetKeyword); err != nil {
		return false, fmt.Errorf("failed to record transition: %w", err)
	}

	// Compute next occurrence
	now := time.Now()
	nextDate := sched.NextOccurrence(now)

	// Build new token, preserving end time on the new date
	newSched := &Schedule{
		Date:       nextDate,
		HasTime:    sched.HasTime,
		HasEnd:     sched.HasEnd,
		Recurrence: sched.Recurrence,
		Warning:    sched.Warning,
	}
	if sched.HasEnd {
		newSched.EndTime = time.Date(nextDate.Year(), nextDate.Month(), nextDate.Day(),
			sched.EndTime.Hour(), sched.EndTime.Minute(), 0, 0, time.Local)
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

	lineNum := t.LineNum
	if lineNum == 0 {
		var err error
		lineNum, err = Locate(t.FilePath, t)
		if err != nil {
			return err
		}
	}

	content, err := os.ReadFile(t.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	found := false
	if lineNum > 0 && lineNum <= len(lines) {
		line := lines[lineNum-1]
		stripped, _ := StripLinePrefix(line)
		if t.RawTitle != "" && (!strings.HasPrefix(stripped, t.Keyword+":") || strings.TrimSpace(strings.TrimPrefix(stripped, t.Keyword+":")) != strings.TrimSpace(t.RawTitle)) {
			return fmt.Errorf("task changed since loading; reload before advancing")
		}
		// Replace the date token on the located line.
		if isScheduled {
			if strings.Contains(line, "@s:"+oldToken) {
				lines[lineNum-1] = strings.Replace(line, "@s:"+oldToken, "@s:"+newToken, 1)
				found = true
			} else if strings.Contains(line, "@"+oldToken) {
				lines[lineNum-1] = strings.Replace(line, "@"+oldToken, "@"+newToken, 1)
				found = true
			}
		} else if strings.Contains(line, "@d:"+oldToken) {
			lines[lineNum-1] = strings.Replace(line, "@d:"+oldToken, "@d:"+newToken, 1)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("task line not found or date token not matched")
	}

	newContent := strings.Join(lines, "\n")
	return os.WriteFile(t.FilePath, []byte(newContent), 0644)
}
