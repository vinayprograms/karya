package task

import (
	"sort"
	"time"

	"github.com/vinayprograms/karya/internal/config"
)

// AgendaItem represents a single entry in the agenda view.
type AgendaItem struct {
	Task        *Task
	Date        time.Time
	HasTime     bool
	HasEnd      bool
	EndTime     time.Time
	IsOverdue   bool
	IsDeadline  bool
	Warning     bool
	ClockActive bool
	Schedule    *Schedule
}

// AgendaDay groups agenda items appearing on a single date.
type AgendaDay struct {
	Date  time.Time
	Items []AgendaItem
}

// QueryAgenda loads all tasks and returns agenda items within [start, end],
// grouped by day. Only tasks with scheduled or due dates are included.
// When includeOverdue is true, past-due items appear on today's date.
func QueryAgenda(c *config.Config, start, end time.Time, includeOverdue bool) ([]AgendaDay, error) {
	tasks, err := ListTasks(c, "", false)
	if err != nil {
		return nil, err
	}

	today := truncateToDay(time.Now())
	startDay := truncateToDay(start)
	endDay := truncateToDay(end)

	// Collect agenda items grouped by day
	dayMap := make(map[time.Time][]AgendaItem)

	for _, t := range tasks {
		// Process scheduled date
		if t.ScheduledAt != "" {
			addAgendaEntries(c, t, t.ScheduledAt, false, startDay, endDay, today, includeOverdue, dayMap)
		}
		// Process deadline date
		if t.DueAt != "" {
			addAgendaEntries(c, t, t.DueAt, true, startDay, endDay, today, includeOverdue, dayMap)
		}
	}

	// Mark items with active clocks only on today's occurrence
	clockCache := make(map[*Task]bool)
	for date, items := range dayMap {
		for i := range items {
			t := items[i].Task
			if _, ok := clockCache[t]; !ok {
				clockCache[t] = IsClockActive(t)
			}
			items[i].ClockActive = clockCache[t] && date.Equal(today)
		}
		dayMap[date] = items
	}

	// Convert map to sorted slice of AgendaDay
	var days []AgendaDay
	for date, items := range dayMap {
		sortAgendaItems(items, c)
		days = append(days, AgendaDay{Date: date, Items: items})
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.Before(days[j].Date)
	})

	return days, nil
}

func addAgendaEntries(c *config.Config, t *Task, dateToken string, isDeadline bool, start, end, today time.Time, includeOverdue bool, dayMap map[time.Time][]AgendaItem) {
	sched, err := ParseSchedule(dateToken)
	if err != nil {
		return
	}

	warningDays := 0
	if sched.Warning != nil {
		warningDays = sched.Warning.Days
	} else if c.Schedule.DefaultWarningDays > 0 && isDeadline {
		warningDays = c.Schedule.DefaultWarningDays
	}

	if sched.Recurrence != nil {
		// Recurring: expand occurrences in range
		occurrences := sched.ExpandOccurrences(start, end)
		for _, occ := range occurrences {
			day := truncateToDay(occ)
			overdue := day.Before(today)
			if overdue && !includeOverdue {
				continue
			}
			item := AgendaItem{
				Task:       t,
				Date:       occ,
				HasTime:    sched.HasTime,
				HasEnd:     sched.HasEnd,
				EndTime:    sched.EndTime,
				IsOverdue:  overdue && includeOverdue,
				IsDeadline: isDeadline,
				Schedule:   sched,
			}
			if warningDays > 0 {
				warningStart := day.AddDate(0, 0, -warningDays)
				item.Warning = !today.Before(warningStart) && today.Before(day)
			}
			targetDay := day
			if item.IsOverdue {
				targetDay = today
			}
			dayMap[targetDay] = append(dayMap[targetDay], item)
		}

		// For recurring tasks, also check if overdue (most recent missed)
		if includeOverdue {
			schedDay := truncateToDay(sched.Date)
			if schedDay.Before(today) && schedDay.Before(start) {
				if sched.Recurrence.Mode == RecurrenceFromDone {
					// .+ mode: only the stored date matters (no predictable series)
					item := AgendaItem{
						Task:       t,
						Date:       sched.Date,
						HasTime:    sched.HasTime,
						HasEnd:     sched.HasEnd,
						EndTime:    sched.EndTime,
						IsOverdue:  true,
						IsDeadline: isDeadline,
						Schedule:   sched,
					}
					if warningDays > 0 {
						item.Warning = true
					}
					dayMap[today] = append(dayMap[today], item)
				} else {
					// + and ++ modes: walk forward to find most recent missed occurrence
					current := sched.Date
					var lastMissed time.Time
					for !truncateToDay(current).After(today) {
						if truncateToDay(current).Before(today) {
							lastMissed = current
						}
						current = addInterval(current, sched.Recurrence.Interval, sched.Recurrence.Unit)
					}
					if !lastMissed.IsZero() && truncateToDay(lastMissed).Before(start) {
						item := AgendaItem{
							Task:       t,
							Date:       lastMissed,
							HasTime:    sched.HasTime,
							HasEnd:     sched.HasEnd,
							EndTime:    sched.EndTime,
							IsOverdue:  true,
							IsDeadline: isDeadline,
							Schedule:   sched,
						}
						dayMap[today] = append(dayMap[today], item)
					}
				}
			}
		}
	} else {
		// Non-recurring: single date
		schedDay := truncateToDay(sched.Date)
		isOverdue := schedDay.Before(today) && !t.IsCompleted(c)

		inRange := (schedDay.Before(end) || schedDay.Equal(end)) && (schedDay.After(start) || schedDay.Equal(start))

		if inRange {
			displaced := isOverdue && includeOverdue
			item := AgendaItem{
				Task:       t,
				Date:       sched.Date,
				HasTime:    sched.HasTime,
				HasEnd:     sched.HasEnd,
				EndTime:    sched.EndTime,
				IsOverdue:  displaced,
				IsDeadline: isDeadline,
				Schedule:   sched,
			}
			if warningDays > 0 {
				warningStart := schedDay.AddDate(0, 0, -warningDays)
				item.Warning = !today.Before(warningStart) && today.Before(schedDay)
			}
			targetDay := schedDay
			if displaced {
				targetDay = today
			}
			dayMap[targetDay] = append(dayMap[targetDay], item)
		} else if isOverdue && includeOverdue {
			item := AgendaItem{
				Task:       t,
				Date:       sched.Date,
				HasTime:    sched.HasTime,
				HasEnd:     sched.HasEnd,
				EndTime:    sched.EndTime,
				IsOverdue:  true,
				IsDeadline: isDeadline,
				Schedule:   sched,
			}
			if warningDays > 0 {
				item.Warning = true
			}
			dayMap[today] = append(dayMap[today], item)
		}
	}
}

// sortAgendaItems sorts items within a day: overdue first, then timed (by time), then untimed (by priority).
func sortAgendaItems(items []AgendaItem, c *config.Config) {
	sort.SliceStable(items, func(i, j int) bool {
		// Overdue items first
		if items[i].IsOverdue != items[j].IsOverdue {
			return items[i].IsOverdue
		}
		// Then timed items before untimed
		if items[i].HasTime != items[j].HasTime {
			return items[i].HasTime
		}
		// Among timed, sort by time
		if items[i].HasTime && items[j].HasTime {
			return items[i].Date.Before(items[j].Date)
		}
		// Among untimed, sort by priority
		return items[i].Task.Priority(c) < items[j].Task.Priority(c)
	})
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
