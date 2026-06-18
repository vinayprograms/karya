package task

import "github.com/charmbracelet/x/ansi"

// TruncateString truncates s to maxWidth visible characters (ANSI-aware),
// appending "…" if truncated. Returns s unchanged if it fits.
func TruncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= maxWidth {
		return s
	}
	return ansi.Truncate(s, maxWidth-1, "…")
}
