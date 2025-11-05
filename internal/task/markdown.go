package task

import (
	"regexp"

	"github.com/charmbracelet/lipgloss"
)

// RenderMarkdownDescription renders a task description with markdown formatting
// It supports bold (**text**), italic (*text*), strikethrough (~~text~~), and inline code (`code`)
// The raw markdown syntax is hidden in the output
//
// Color choices:
// - Bold text: Uses bright white color to make it stand out
// - Italic text: Preserves the original text color but adds italic styling
// - Strikethrough text: Preserves the original text color but adds strikethrough styling
// - Inline code: Uses black text on white background for clear distinction
func RenderMarkdownDescription(description string, taskColor lipgloss.Style) string {
	// Process bold text (**text**) - use bright white color to make it stand out
	boldRe := regexp.MustCompile(`\*\*(.*?)\*\*`)
	description = boldRe.ReplaceAllStringFunc(description, func(match string) string {
		content := boldRe.FindStringSubmatch(match)[1]
		// Use bright white color for bold text to make it stand out
		boldStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
		return boldStyle.Render(content)
	})

	// Process italic text (*text*) - preserve original color but add italic styling
	italicRe := regexp.MustCompile(`\*(.*?)\*`)
	description = italicRe.ReplaceAllStringFunc(description, func(match string) string {
		content := italicRe.FindStringSubmatch(match)[1]
		// Apply italic to the existing style to preserve the original color
		return taskColor.Italic(true).Render(content)
	})

	// Process strikethrough text (~~text~~) - preserve original color but add strikethrough styling
	strikeRe := regexp.MustCompile(`~~(.*?)~~`)
	description = strikeRe.ReplaceAllStringFunc(description, func(match string) string {
		content := strikeRe.FindStringSubmatch(match)[1]
		// Apply strikethrough to the existing style to preserve the original color
		return taskColor.Strikethrough(true).Render(content)
	})

	// Process inline code (`code`) - use distinct styling for clear distinction
	codeRe := regexp.MustCompile("`([^`]+)`")
	description = codeRe.ReplaceAllStringFunc(description, func(match string) string {
		content := codeRe.FindStringSubmatch(match)[1]
		// Use black text on white background for clear distinction of inline code
		return lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Background(lipgloss.Color("#000000")).Render(content)
	})

	return description
}
