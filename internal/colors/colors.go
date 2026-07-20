package colors

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vinayprograms/karya/internal/config"
)

var ansi16Hex = [16]string{
	"#000000", "#800000", "#008000", "#808000",
	"#000080", "#800080", "#008080", "#C0C0C0",
	"#808080", "#FF0000", "#00FF00", "#FFFF00",
	"#0000FF", "#FF00FF", "#00FFFF", "#FFFFFF",
}

type ColorValue struct {
	Fg string `json:"fg,omitempty"`
	Bg string `json:"bg,omitempty"`
}

type KeywordColor struct {
	Fg       string `json:"fg,omitempty"`
	Category string `json:"category"`
}

type Output struct {
	Keywords map[string]KeywordColor `json:"keywords"`
	Elements map[string]ColorValue   `json:"elements"`
	Theme    string                  `json:"theme"`
}

func normalizeHex(v string) string {
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "#") {
		return v
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 && n < 16 {
		return ansi16Hex[n]
	}
	return v
}

func Print(cfg *config.Config) error {
	c := cfg.Colors

	out := Output{
		Keywords: make(map[string]KeywordColor),
		Elements: make(map[string]ColorValue),
		Theme:    c.Theme,
	}

	categoryMap := map[string]string{}
	for _, kw := range cfg.Todo.Active {
		categoryMap[kw] = "active"
	}
	for _, kw := range cfg.Todo.InProgress {
		categoryMap[kw] = "inprogress"
	}
	for _, kw := range cfg.Todo.Completed {
		categoryMap[kw] = "completed"
	}
	for _, kw := range cfg.Todo.Someday {
		categoryMap[kw] = "someday"
	}

	colorForCategory := map[string]string{
		"active":     normalizeHex(c.ActiveColor),
		"inprogress": normalizeHex(c.InProgressColor),
		"completed":  normalizeHex(c.CompletedColor),
		"someday":    normalizeHex(c.SomedayColor),
	}

	for kw, cat := range categoryMap {
		out.Keywords[kw] = KeywordColor{
			Fg:       colorForCategory[cat],
			Category: cat,
		}
	}

	elements := map[string]ColorValue{
		"project":               {Fg: normalizeHex(c.ProjectColor)},
		"description":           {Fg: normalizeHex(c.TaskColor)},
		"completed-description": {Fg: normalizeHex(c.CompletedTaskColor)},
		"tag":                   {Fg: normalizeHex(c.TagColor), Bg: normalizeHex(c.TagBgColor)},
		"special-tag":           {Fg: normalizeHex(c.SpecialTagColor), Bg: normalizeHex(c.SpecialTagBgColor)},
		"date":                  {Fg: normalizeHex(c.DateColor), Bg: normalizeHex(c.DateBgColor)},
		"past-date":             {Fg: normalizeHex(c.PastDateColor), Bg: normalizeHex(c.PastDateBgColor)},
		"today-date":            {Fg: normalizeHex(c.TodayDateColor), Bg: normalizeHex(c.TodayDateBgColor)},
		"assignee":              {Fg: normalizeHex(c.AssigneeColor), Bg: normalizeHex(c.AssigneeBgColor)},
		"cycle":                 {Fg: normalizeHex(c.CycleColor), Bg: normalizeHex(c.CycleBgColor)},
		"overdue":               {Fg: normalizeHex(c.OverdueColor), Bg: normalizeHex(c.OverdueBgColor)},
		"deadline":              {Fg: normalizeHex(c.DeadlineColor)},
		"clock-active":          {Fg: normalizeHex(c.ClockActiveColor)},
		"agenda-header":         {Fg: normalizeHex(c.AgendaHeaderColor)},
	}

	for name, val := range elements {
		if val.Fg != "" || val.Bg != "" {
			out.Elements[name] = val
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
