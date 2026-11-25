package task

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func (s *Schedule) anchor(item *Task, field string, now time.Time, persist bool, reset ...bool) error {
	if s.Rule == "" || s.Recurrence.Interval == 1 {
		return nil
	}
	if item.FilePath == "" {
		return fmt.Errorf("calendar interval requires a task file for its anchor")
	}
	raw, err := ReadRawBlock(item)
	if err != nil {
		return err
	}
	lines := strings.Split(raw, "\n")
	indent := detectSubItemRawIndent(lines)
	prefix := "LOG(" + field + "): "
	for _, line := range lines[1:] {
		if countLeadingSpaces(line) != indent {
			continue
		}
		text, _ := StripLinePrefix(line)
		if !strings.HasPrefix(text, prefix) {
			continue
		}
		if len(reset) > 0 && reset[0] {
			break
		}
		s.Anchor, err = time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(strings.TrimPrefix(text, prefix)), time.Local)
		if err != nil {
			return fmt.Errorf("invalid %s anchor timestamp: %w", field, err)
		}
		if !s.matches(s.Anchor) {
			break
		}
		return nil
	}
	s.Anchor = time.Date(now.Year(), now.Month(), now.Day(), s.Date.Hour(), s.Date.Minute(), 0, 0, now.Location())
	for !s.matches(s.Anchor) {
		s.Anchor = s.Anchor.AddDate(0, 0, 1)
	}
	if !persist {
		return fmt.Errorf("calendar rule %q needs an anchor; refresh schedules first", s.Raw)
	}
	location, err := Locate(item.FilePath, item)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(item.FilePath)
	if err != nil {
		return err
	}
	all := strings.Split(string(data), "\n")
	if item.LineNum > 0 && item.LineNum <= len(all) {
		text, _ := StripLinePrefix(all[item.LineNum-1])
		if strings.HasPrefix(text, item.Keyword+":") && strings.TrimSpace(strings.TrimPrefix(text, item.Keyword+":")) == strings.TrimSpace(item.RawTitle) {
			location = item.LineNum
		}
	}
	item.LineNum = location
	padding, err := subItemIndentForTask(item)
	if err != nil {
		return err
	}
	entry := fmt.Sprintf("%s* LOG(%s): %s", padding, field, s.Anchor.Format("2006-01-02T15:04"))
	all = append(all[:location], append([]string{entry}, all[location:]...)...)
	return os.WriteFile(item.FilePath, []byte(strings.Join(all, "\n")), 0644)
}

func resolve(item *Task, token, field string, now time.Time, persist bool) (*Schedule, error) {
	schedule, err := ParseSchedule(token)
	if err != nil {
		return nil, err
	}
	if err := schedule.anchor(item, field, now, persist); err != nil {
		return nil, err
	}
	return schedule, nil
}
