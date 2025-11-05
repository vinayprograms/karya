package task

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderMarkdownDescription(t *testing.T) {
	// Create a basic style for testing
	baseStyle := lipgloss.NewStyle()

	tests := []struct {
		name        string
		description string
		expected    string // We'll check if the rendered text contains the content
	}{
		{
			name:        "bold text",
			description: "This is **bold** text",
			expected:    "This is bold text",
		},
		{
			name:        "italic text",
			description: "This is *italic* text",
			expected:    "This is italic text",
		},
		{
			name:        "strikethrough text",
			description: "This is ~~strikethrough~~ text",
			expected:    "This is strikethrough text",
		},
		{
			name:        "inline code",
			description: "This is `inline code` text",
			expected:    "This is inline code text",
		},
		{
			name:        "multiple markdown elements",
			description: "**Bold** and *italic* and `code` and ~~strikethrough~~",
			expected:    "Bold and italic and code and strikethrough",
		},
		{
			name:        "no markdown",
			description: "Plain text without markdown",
			expected:    "Plain text without markdown",
		},
		{
			name:        "empty string",
			description: "",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderMarkdownDescription(tt.description, baseStyle)
			// Since lipgloss adds escape sequences, we check if the content is present
			// and that the markdown syntax is removed
			if !strings.Contains(result, tt.expected) {
				t.Errorf("RenderMarkdownDescription() = %v, should contain %v", result, tt.expected)
			}
			
			// Ensure markdown syntax is removed
			if strings.Contains(result, "**") || strings.Contains(result, "*") || 
			   strings.Contains(result, "~~") || strings.Contains(result, "`") {
				t.Errorf("RenderMarkdownDescription() should remove markdown syntax, got %v", result)
			}
		})
	}
}