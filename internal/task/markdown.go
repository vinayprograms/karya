package task

import (
	"regexp"

	"github.com/charmbracelet/lipgloss"
)

// RenderMarkdownDescription renders a task description with markdown formatting.
// Supports: bold, italic, strikethrough, inline code, URLs, and bullet markers.
func RenderMarkdownDescription(description string, taskColor lipgloss.Style) string {
	// Process inline code first (protect from other transformations)
	codeRe := regexp.MustCompile("`([^`]+)`")
	description = codeRe.ReplaceAllStringFunc(description, func(match string) string {
		content := codeRe.FindStringSubmatch(match)[1]
		return lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Background(lipgloss.Color("#000000")).Render(content)
	})

	// Process markdown links [text](url) — show text underlined in blue
	mdLinkRe := regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`)
	description = mdLinkRe.ReplaceAllStringFunc(description, func(match string) string {
		parts := mdLinkRe.FindStringSubmatch(match)
		linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Underline(true)
		return linkStyle.Render(parts[1])
	})

	// Process bare URLs
	urlRe := regexp.MustCompile(`https?://[^\s\)>\]]+`)
	description = urlRe.ReplaceAllStringFunc(description, func(match string) string {
		linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Underline(true)
		return linkStyle.Render(match)
	})

	// Process bold text (**text**)
	boldRe := regexp.MustCompile(`\*\*(.*?)\*\*`)
	description = boldRe.ReplaceAllStringFunc(description, func(match string) string {
		content := boldRe.FindStringSubmatch(match)[1]
		return lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true).Render(content)
	})

	// Process italic text (*text*)
	italicRe := regexp.MustCompile(`\*(.*?)\*`)
	description = italicRe.ReplaceAllStringFunc(description, func(match string) string {
		content := italicRe.FindStringSubmatch(match)[1]
		return taskColor.Italic(true).Render(content)
	})

	// Process strikethrough text (~~text~~)
	strikeRe := regexp.MustCompile(`~~(.*?)~~`)
	description = strikeRe.ReplaceAllStringFunc(description, func(match string) string {
		content := strikeRe.FindStringSubmatch(match)[1]
		return taskColor.Strikethrough(true).Render(content)
	})

	// Colorize leading bullet markers (-, *, +)
	bulletRe := regexp.MustCompile(`^(\s*)([-*+])(\s)`)
	description = bulletRe.ReplaceAllStringFunc(description, func(match string) string {
		parts := bulletRe.FindStringSubmatch(match)
		bulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
		return parts[1] + bulletStyle.Render(parts[2]) + parts[3]
	})

	return description
}
