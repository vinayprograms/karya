package task

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vinayprograms/karya/internal/config"
)

func Refresh(cfg *config.Config, now time.Time) error {
	tasks, err := ListTasks(cfg, "", true)
	if err != nil {
		return err
	}
	sort.SliceStable(tasks, func(left, right int) bool {
		if tasks[left].FilePath != tasks[right].FilePath {
			return tasks[left].FilePath < tasks[right].FilePath
		}
		return tasks[left].LineNum > tasks[right].LineNum
	})
	var failures []error
	today := truncateToDay(now)
	for _, event := range tasks {
		for _, scheduled := range []bool{true, false} {
			token := event.DueAt
			if scheduled {
				token = event.ScheduledAt
			}
			if token == "" {
				continue
			}
			field := "due"
			if scheduled {
				field = "scheduled"
			}
			schedule, err := resolve(event, token, field, now, true)
			if err != nil {
				failures = append(failures, fmt.Errorf("event %q: %w", event.Title, err))
				continue
			}
			if schedule.Rule != "" || !event.Event(cfg) {
				continue
			}
			if schedule.Recurrence == nil || !truncateToDay(schedule.Date).Before(today) {
				continue
			}
			for truncateToDay(schedule.Date).Before(today) {
				next := addInterval(schedule.Date, schedule.Recurrence.Interval, schedule.Recurrence.Unit)
				if !next.After(schedule.Date) {
					return fmt.Errorf("event %q: recurrence does not advance", event.Title)
				}
				schedule.Date = next
			}
			if schedule.HasEnd {
				schedule.EndTime = time.Date(schedule.Date.Year(), schedule.Date.Month(), schedule.Date.Day(), schedule.EndTime.Hour(), schedule.EndTime.Minute(), 0, 0, schedule.Date.Location())
			}
			replacement := schedule.FormatToken()
			if err := advanceDateInFile(event, token, replacement, scheduled); err != nil {
				failures = append(failures, fmt.Errorf("refresh event %q: %w", event.Title, err))
				continue
			}
			prefix := "@d:"
			if scheduled {
				prefix = "@s:"
				if !strings.Contains(event.RawTitle, prefix+token) {
					prefix = "@"
				}
				event.ScheduledAt = replacement
			} else {
				event.DueAt = replacement
			}
			event.RawTitle = strings.Replace(event.RawTitle, prefix+token, prefix+replacement, 1)
		}
	}
	return errors.Join(failures...)
}
