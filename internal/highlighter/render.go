package highlighter

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var caretStyle = lipgloss.NewStyle().Reverse(true)

// Slice returns the rune window [start, start+n) across spans, splitting a
// span when the window lands inside it. n is a rune count, matching Buffer
// caret columns.
func Slice(spans []Span, start, n int) []Span {
	if n <= 0 || len(spans) == 0 {
		return nil
	}
	if start < 0 {
		start = 0
	}
	end := start + n
	out := make([]Span, 0, len(spans))
	pos := 0
	for _, sp := range spans {
		rs := []rune(sp.Text)
		spanEnd := pos + len(rs)
		if spanEnd <= start {
			pos = spanEnd
			continue
		}
		if pos >= end {
			break
		}
		a := start - pos
		if a < 0 {
			a = 0
		}
		b := end - pos
		if b > len(rs) {
			b = len(rs)
		}
		if a < b {
			out = append(out, Span{Text: string(rs[a:b]), Type: sp.Type, Style: sp.Style})
		}
		pos = spanEnd
	}
	return out
}

// Emphasize copies spans with bold + background so a playhead row still
// carries syntax colours on top of the cursorline fill.
func Emphasize(spans []Span, bg lipgloss.Color) []Span {
	if len(spans) == 0 {
		return nil
	}
	out := make([]Span, len(spans))
	for i, sp := range spans {
		sp.Style = sp.Style.Bold(true).Background(bg)
		out[i] = sp
	}
	return out
}

// Tint paints a background onto spans without stealing syntax colours or
// adding bold. Used by the post-playback add / delete / replace overlay.
func Tint(spans []Span, bg lipgloss.Color) []Span {
	if len(spans) == 0 {
		return nil
	}
	out := make([]Span, len(spans))
	for i, sp := range spans {
		sp.Style = sp.Style.Background(bg)
		out[i] = sp
	}
	return out
}

// Render concatenates spans with their lipgloss styles applied.
func Render(spans []Span) string {
	if len(spans) == 0 {
		return ""
	}
	var b strings.Builder
	for _, sp := range spans {
		if sp.Text == "" {
			continue
		}
		b.WriteString(sp.Style.Render(sp.Text))
	}
	return b.String()
}

// RenderWithCaret is Render plus a reverse-video caret at the 0-based rune
// column. When on is false the caret is omitted (blink off).
func RenderWithCaret(spans []Span, col int, on bool) string {
	if !on {
		return Render(spans)
	}
	if col < 0 {
		col = 0
	}

	var b strings.Builder
	pos := 0
	placed := false
	for _, sp := range spans {
		rs := []rune(sp.Text)
		n := len(rs)
		if placed || col >= pos+n {
			if sp.Text != "" {
				b.WriteString(sp.Style.Render(sp.Text))
			}
			pos += n
			continue
		}
		i := col - pos
		if i > 0 {
			b.WriteString(sp.Style.Render(string(rs[:i])))
		}
		ch := string(rs[i])
		if ch == "\t" {
			ch = " "
		}
		b.WriteString(caretStyle.Render(ch))
		if i+1 < n {
			b.WriteString(sp.Style.Render(string(rs[i+1:])))
		}
		placed = true
		pos += n
	}
	if !placed {
		b.WriteString(caretStyle.Render(" "))
	}
	return b.String()
}

// Visible reconstructs the unstyled text of a line. Used by tests and by
// the UI when comparing caret columns against buffer runes.
func Visible(spans []Span) string {
	var b strings.Builder
	for _, sp := range spans {
		b.WriteString(sp.Text)
	}
	return b.String()
}
