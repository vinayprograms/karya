// Package zetui provides the shared Bubble Tea list components for
// browsing zettels, used by both the zet and note commands.
package zetui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"github.com/vinayprograms/karya/internal/zet"
)

// Styles configures how zettel rows are drawn.
type Styles struct {
	ID         lipgloss.Style // zettel ID column (verbose mode)
	Title      lipgloss.Style // zettel title
	Selector   lipgloss.Style // cursor indicator
	MatchCount lipgloss.Style // "[n]" fulltext match count
}

// Item is a zettel row in a list, optionally carrying search matches.
type Item struct {
	Zettel        zet.Zettel
	Verbose       bool
	SearchResults []zet.SearchResult
	Styles        Styles
}

// FilterValue implements list.Item.
func (i Item) FilterValue() string {
	return i.Zettel.ID + " " + i.Zettel.Title
}

// Title implements list.DefaultItem (no selector column).
func (i Item) Title() string {
	return i.render(false, "█")
}

// Description implements list.DefaultItem.
func (i Item) Description() string { return "" }

func (i Item) render(isSelected bool, glyph string) string {
	var parts []string

	if len(i.SearchResults) > 0 {
		parts = append(parts, i.Styles.MatchCount.Render(fmt.Sprintf("[%d] ", len(i.SearchResults))))
	}

	if i.Verbose {
		parts = append(parts, i.Styles.ID.Render(fmt.Sprintf("%-14s", i.Zettel.ID)))
	}

	title := i.Styles.Title.Render(i.Zettel.Title)
	if isSelected {
		parts = append(parts, i.Styles.Selector.Render(glyph+" ")+title)
	} else {
		parts = append(parts, "  "+title)
	}

	return strings.Join(parts, " ")
}

// Delegate renders Items in a bubbles list. CursorGlyph, when set, allows
// the owner to change the cursor character after list construction (the
// zet pinboard view uses a different glyph); nil means "█".
type Delegate struct {
	list.DefaultDelegate
	CursorGlyph *string
}

// Render implements list.ItemDelegate.
func (d Delegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(Item)
	if !ok {
		return
	}
	glyph := "█"
	if d.CursorGlyph != nil && *d.CursorGlyph != "" {
		glyph = *d.CursorGlyph
	}
	fmt.Fprint(w, it.render(index == m.Index(), glyph))
}
