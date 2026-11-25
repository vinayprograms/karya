package task

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var calendarPattern = regexp.MustCompile(`(?i)^([a-z]+)(?:T(\d{2}:\d{2})(?:-(\d{2}:\d{2}))?)?(?:\+(\d+)([dwmy]))?(?:!(\d+)d)?$`)

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
		unit = strings.ToLower(match[5])[0]
		if name == "daily" && unit != 'd' {
			return nil, fmt.Errorf("daily requires recurrence in d units")
		}
		if name == "weekdays" && unit != 'w' {
			return nil, fmt.Errorf("weekdays requires recurrence in w units")
		}
		if name != "daily" && name != "weekdays" && unit == 'd' {
			return nil, fmt.Errorf("%s requires recurrence in w, m, or y units", name)
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
		anchored := s.Recurrence.Interval > 1 || s.Recurrence.Unit == 'm' || s.Recurrence.Unit == 'y'
		if anchored {
			if s.Anchor.IsZero() {
				continue
			}
			anchor := truncateToDay(s.Anchor)
			if day.Before(anchor) {
				continue
			}
			switch s.Recurrence.Unit {
			case 'd':
				if calendardays(anchor, day)%s.Recurrence.Interval != 0 {
					continue
				}
			case 'w':
				if day.Weekday() != anchor.Weekday() || calendardays(anchor, day)%(s.Recurrence.Interval*7) != 0 {
					continue
				}
			case 'm':
				months := (day.Year()-anchor.Year())*12 + int(day.Month()-anchor.Month())
				if months%s.Recurrence.Interval != 0 || ordinalday(day, anchor) != day.Day() {
					continue
				}
			case 'y':
				years := day.Year() - anchor.Year()
				if years%s.Recurrence.Interval != 0 || day.Month() != anchor.Month() || ordinalday(day, anchor) != day.Day() {
					continue
				}
			}
		}
		result = append(result, time.Date(day.Year(), day.Month(), day.Day(), s.Date.Hour(), s.Date.Minute(), 0, 0, day.Location()))
	}
	return result
}

func calendardays(start, end time.Time) int {
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(end.Sub(start).Hours() / 24)
}

func ordinalday(day, anchor time.Time) int {
	first := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
	offset := (int(anchor.Weekday()) - int(first.Weekday()) + 7) % 7
	candidate := first.AddDate(0, 0, offset+((anchor.Day()-1)/7)*7)
	last := time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, day.Location())
	if candidate.After(last) {
		candidate = candidate.AddDate(0, 0, -7)
	}
	return candidate.Day()
}

func (s *Schedule) identity() string {
	return fmt.Sprintf("%sT%02d:%02d/%t-%02d:%02d/%t+%d%c", s.Rule, s.Date.Hour(), s.Date.Minute(), s.HasTime, s.EndTime.Hour(), s.EndTime.Minute(), s.HasEnd, s.Recurrence.Interval, s.Recurrence.Unit)
}
