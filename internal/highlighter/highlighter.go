// Package highlighter incrementally syntax-highlights replay buffers with
// Chroma and maps tokens onto lipgloss styles.
//
// "Incremental" here means: re-lex only when the buffer text changes, then
// reuse the per-line span cache for every 60 FPS blink frame. The in-flight
// line is highlighted with the same lexer state as the rest of the file, so
// multiline comments and strings stay correct while characters are typed.
package highlighter

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// DefaultTheme is a dark, terminal-friendly Chroma style.
const DefaultTheme = "dracula"

// Span is one styled token fragment that does not contain a newline.
type Span struct {
	Text  string
	Type  chroma.TokenType
	Style lipgloss.Style
}

// Highlighter binds a filename's lexer to a colour theme and caches the last
// tokenised buffer.
type Highlighter struct {
	lexer  chroma.Lexer
	style  *chroma.Style
	styles map[chroma.TokenType]lipgloss.Style
	src    string
	lines  [][]Span
}

// New infers the lexer from filename and loads theme. Unknown languages fall
// back to Chroma's plaintext lexer; unknown themes fall back to swapoff.
func New(filename, theme string) *Highlighter {
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	if theme == "" {
		theme = DefaultTheme
	}
	style := styles.Get(theme)
	return &Highlighter{
		lexer:  lexer,
		style:  style,
		styles: make(map[chroma.TokenType]lipgloss.Style),
	}
}

// LexerName is the Chroma lexer title (e.g. "Go").
func (h *Highlighter) LexerName() string {
	if h == nil || h.lexer == nil {
		return ""
	}
	cfg := h.lexer.Config()
	if cfg == nil {
		return ""
	}
	return cfg.Name
}

// ThemeName is the resolved Chroma style name.
func (h *Highlighter) ThemeName() string {
	if h == nil || h.style == nil {
		return ""
	}
	return h.style.Name
}

// Lines tokenises src into per-line spans. A repeated call with the same
// source is a cache hit — this is what keeps 60 FPS blinks cheap.
func (h *Highlighter) Lines(src string) [][]Span {
	if h == nil {
		return unstyledLines(src)
	}
	if h.lines != nil && src == h.src {
		return h.lines
	}
	h.src = src
	h.lines = h.tokenise(src)
	return h.lines
}

// Line returns the 1-based line of src. Out of range yields nil.
func (h *Highlighter) Line(src string, row int) []Span {
	lines := h.Lines(src)
	if row < 1 || row > len(lines) {
		return nil
	}
	return lines[row-1]
}

func (h *Highlighter) tokenise(src string) [][]Span {
	if src == "" {
		return nil
	}
	it, err := h.lexer.Tokenise(nil, src)
	if err != nil {
		return unstyledLines(src)
	}
	lines := make([][]Span, 0, 32)
	var cur []Span
	for _, tok := range it.Tokens() {
		parts := splitNL(tok.Value)
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, cur)
				cur = nil
			}
			if part == "" {
				continue
			}
			cur = append(cur, Span{
				Text:  part,
				Type:  tok.Type,
				Style: h.lipgloss(tok.Type),
			})
		}
	}
	lines = append(lines, cur)
	if len(src) > 0 && src[len(src)-1] == '\n' && len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (h *Highlighter) lipgloss(tt chroma.TokenType) lipgloss.Style {
	if s, ok := h.styles[tt]; ok {
		return s
	}
	s := styleOf(h.style.Get(tt))
	h.styles[tt] = s
	return s
}

func styleOf(e chroma.StyleEntry) lipgloss.Style {
	s := lipgloss.NewStyle()
	if e.Colour.IsSet() {
		s = s.Foreground(lipgloss.Color(e.Colour.String()))
	}
	if e.Bold == chroma.Yes {
		s = s.Bold(true)
	}
	if e.Italic == chroma.Yes {
		s = s.Italic(true)
	}
	if e.Underline == chroma.Yes {
		s = s.Underline(true)
	}
	return s
}

func splitNL(s string) []string {
	if s == "" {
		return []string{""}
	}
	return splitKeep(s, '\n')
}

func splitKeep(s string, sep byte) []string {
	n := 1
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			n++
		}
	}
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func unstyledLines(src string) [][]Span {
	if src == "" {
		return nil
	}
	raw := splitKeep(src, '\n')
	if len(src) > 0 && src[len(src)-1] == '\n' && len(raw) > 0 {
		raw = raw[:len(raw)-1]
	}
	lines := make([][]Span, len(raw))
	for i, line := range raw {
		if line == "" {
			continue
		}
		lines[i] = []Span{{Text: line, Type: chroma.Text}}
	}
	return lines
}
