package cli

import (
	"fmt"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
)

// terminalText deliberately uses printable ASCII: terminal width and escape
// handling vary, and card contents are not trusted terminal control input.
func terminalText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

func terminalRender(b *board.Board, width, height, column, row int) string {
	if width < 1 || height < 1 {
		return ""
	}
	if column < 0 {
		column = 0
	}
	if row < 0 {
		row = 0
	}
	const cell = 28
	visible := width / cell
	if visible < 1 {
		visible = 1
	}
	end := column + visible
	if end > len(b.Columns) {
		end = len(b.Columns)
	}
	if column > end {
		column = end
	}
	var out strings.Builder
	line := func(s string) {
		// Keep the final terminal cell empty to avoid implicit wrapping before newline.
		limit := width - 1
		if len(s) > limit {
			s = s[:limit]
		}
		out.WriteString(s)
		out.WriteByte('\n')
	}
	line("Kanbanzai board | arrows or hjkl: move | q: quit")
	var header strings.Builder
	for _, c := range b.Columns[column:end] {
		label := terminalText(c.Name)
		if c.Lane != "" {
			label = terminalText(c.Lane) + "/" + label
		}
		if len(label) > cell-2 {
			label = label[:cell-2]
		}
		fmt.Fprintf(&header, "%-*s", cell, label)
	}
	if height > 1 {
		line(header.String())
	}
	for y := 0; y < height-3; y++ {
		var l strings.Builder
		for _, c := range b.Columns[column:end] {
			text := ""
			if row+y < len(c.Cards) {
				card := c.Cards[row+y]
				text = terminalText(card.ID)
				if card.Question != nil || (card.Session != nil && card.Session.State == "waiting") {
					text += " [asking]"
				}
				if card.Held != nil {
					text += " [held]"
				}
				if card.Interrupted {
					text += " [interrupted]"
				}
				if card.WaitsAtGate() {
					text += " [gate]"
				}
				if card.ClaimedNotStarted {
					text += " [unstarted]"
				}
				if card.Owner != nil {
					text += " [claimed]"
				}
				text += " " + terminalText(card.Title)
			}
			if len(text) > cell-2 {
				text = text[:cell-2]
			}
			fmt.Fprintf(&l, "%-*s", cell, text)
		}
		line(l.String())
	}
	if height > 2 && len(b.Problems) > 0 {
		line("Problems: " + terminalText(strings.Join(b.Problems, "; ")))
	}
	return out.String()
}
