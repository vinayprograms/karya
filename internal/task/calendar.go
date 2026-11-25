package task

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var calendarPattern = regexp.MustCompile(`(?i)^([a-z]+)(?:T(\d{2}:\d{2})(?:-(\d{2}:\d{2}))?)?(?:\+(\d+)([dw]))?(?:!(\d+)d)?$`)

func calendar(raw string) (*Schedule, error) {
	match := calendarPattern.FindStringSubmatch(raw)
	if match == nil {
		return nil, fmt.Errorf("invalid calendar rule %q", raw)
	}
	name := strings.ToLower(match[1])
	aliases := map[string]string{"mon": "monday", "tue": "tuesday", "wed": "wednesday", "thu": "thursday", "fri": "friday", "sat": "saturday", "sun": "sunday"}
	if full, ok := aliases[name]; ok {
		name = full
	}
	valid := name == "daily" || name == "weekdays"
	for day := time.Sunday; day <= time.Saturday; day++ {
		valid = valid || name == strings.ToLower(day.String())
	}
	if !valid {
		return nil, fmt.Errorf("unknown calendar day %q; use a weekday, daily, or weekdays", match[1])
	}
	interval, unit := 1, byte('w')
	if name == "daily" {
		unit = 'd'
	}
	if match[4] != "" {
		var err error
		interval, err = strconv.Atoi(match[4])
		if err != nil || interval < 1 || interval > 10000 {
			return nil, fmt.Errorf("invalid calendar interval %q", match[4])
		}
		supplied := strings.ToLower(match[5])[0]
		if supplied != unit {
			return nil, fmt.Errorf("%s requires recurrence in %c units", name, unit)
		}
	}
	if name == "weekdays" && interval != 1 {
		return nil, fmt.Errorf("weekdays supports +1w; use an individual weekday for alternating weeks")
	}
	schedule := &Schedule{Raw: raw, Rule: name, Recurrence: &RecurrenceSpec{Interval: interval, Unit: unit}}
	schedule.Date = truncateToDay(time.Now())
	if match[2] != "" {
		parsed, err := time.Parse("15:04", match[2])
		if err != nil {
			return nil, err
		}
		schedule.Date = time.Date(schedule.Date.Year(), schedule.Date.Month(), schedule.Date.Day(), parsed.Hour(), parsed.Minute(), 0, 0, time.Local)
		schedule.HasTime = true
	}
	if match[3] != "" {
		parsed, err := time.Parse("15:04", match[3])
		if err != nil {
			return nil, err
		}
		schedule.EndTime = time.Date(schedule.Date.Year(), schedule.Date.Month(), schedule.Date.Day(), parsed.Hour(), parsed.Minute(), 0, 0, time.Local)
		schedule.HasEnd = true
		if !schedule.EndTime.After(schedule.Date) {
			return nil, fmt.Errorf("end time must be after start time")
		}
	}
	if match[6] != "" {
		days, err := strconv.Atoi(match[6])
		if err != nil {
			return nil, err
		}
		schedule.Warning = &WarningSpec{Days: days}
	}
	return schedule, nil
}

func (s *Schedule) matches(day time.Time) bool {
	switch s.Rule {
	case "daily":
		return true
	case "weekdays":
		return day.Weekday() != time.Saturday && day.Weekday() != time.Sunday
	default:
		return strings.EqualFold(day.Weekday().String(), s.Rule)
	}
}

func (s *Schedule) occurrences(start, end time.Time) []time.Time {
	var result []time.Time
	for day := truncateToDay(start); !day.After(truncateToDay(end)); day = day.AddDate(0, 0, 1) {
		if !s.matches(day) {
			continue
		}
		if s.Recurrence.Interval > 1 {
			if s.Anchor.IsZero() || day.Before(truncateToDay(s.Anchor)) {
				continue
			}
			anchor := s.Anchor
			days := int(time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Sub(time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, time.UTC)).Hours() / 24)
			interval := s.Recurrence.Interval
			if s.Recurrence.Unit == 'w' {
				interval *= 7
			}
			if days%interval != 0 {
				continue
			}
		}
		result = append(result, time.Date(day.Year(), day.Month(), day.Day(), s.Date.Hour(), s.Date.Minute(), 0, 0, day.Location()))
	}
	return result
}

func (s *Schedule) identity() string {
	return fmt.Sprintf("%sT%02d:%02d/%t-%02d:%02d/%t+%d%c", s.Rule, s.Date.Hour(), s.Date.Minute(), s.HasTime, s.EndTime.Hour(), s.EndTime.Minute(), s.HasEnd, s.Recurrence.Interval, s.Recurrence.Unit)
}
