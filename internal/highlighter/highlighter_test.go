package highlighter

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

func TestGoLexerFromFilename(t *testing.T) {
	t.Parallel()
	h := New("hello.go", DefaultTheme)
	if got := h.LexerName(); got != "Go" {
		t.Fatalf("lexer = %q, want Go", got)
	}
}

func TestFallbackLexer(t *testing.T) {
	t.Parallel()
	h := New("notes.unknownext", DefaultTheme)
	if got := h.LexerName(); got == "Go" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestPackageIsKeyword(t *testing.T) {
	t.Parallel()
	h := New("main.go", DefaultTheme)
	spans := h.Line("package main\n", 1)
	if !hasCategory(spans, "package", chroma.Keyword) {
		t.Fatalf("package not a keyword: %+v", dump(spans))
	}
}

func TestIncompleteLineStillHighlights(t *testing.T) {
	t.Parallel()
	h := New("main.go", DefaultTheme)
	src := "package hello\n\nfunc Hi() {\n\tprintl\n"
	spans := h.Line(src, 3)
	if Visible(spans) != "func Hi() {" {
		t.Fatalf("line 3 = %q", Visible(spans))
	}
	if !hasCategory(spans, "func", chroma.Keyword) {
		t.Fatalf("func not a keyword while typing: %+v", dump(spans))
	}
}

func TestLinesMatchBufferLineCount(t *testing.T) {
	t.Parallel()
	src := "package hello\n\nfunc Hi() {}\n"
	h := New("hello.go", DefaultTheme)
	lines := h.Lines(src)
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3: %v", len(lines), visibles(lines))
	}
}

func TestCacheHit(t *testing.T) {
	t.Parallel()
	h := New("a.go", DefaultTheme)
	src := "package a\n"
	a := h.Lines(src)
	b := h.Lines(src)
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("empty tokenisation")
	}
	if &a[0] != &b[0] {
		t.Fatal("expected cached line slice")
	}
}

func TestRenderContainsText(t *testing.T) {
	t.Parallel()
	h := New("a.go", DefaultTheme)
	spans := h.Line("package main\n", 1)
	if Visible(spans) != "package main" {
		t.Fatalf("visible = %q", Visible(spans))
	}
	if !hasCategory(spans, "package", chroma.Keyword) {
		t.Fatalf("package not a keyword: %+v", dump(spans))
	}
	for _, sp := range spans {
		if sp.Text != "package" {
			continue
		}
		_, _, _, a := sp.Style.GetForeground().RGBA()
		if a == 0 {
			t.Fatal("package span has no foreground colour")
		}
		return
	}
	t.Fatal("package span missing")
}

func TestSliceWindow(t *testing.T) {
	t.Parallel()
	spans := []Span{
		{Text: "package "},
		{Text: "main"},
	}
	got := Visible(Slice(spans, 8, 4))
	if got != "main" {
		t.Fatalf("slice = %q, want main", got)
	}
	mid := Visible(Slice(spans, 2, 5))
	if mid != "ckage" {
		t.Fatalf("mid slice = %q, want ckage", mid)
	}
	if Visible(Slice(spans, 0, 20)) != "package main" {
		t.Fatalf("oversize slice lost text: %q", Visible(Slice(spans, 0, 20)))
	}
	if Slice(nil, 0, 10) != nil {
		t.Fatal("empty spans should stay empty")
	}
}

func TestEmphasizeKeepsText(t *testing.T) {
	t.Parallel()
	spans := []Span{{Text: "fn", Style: lipgloss.NewStyle().Foreground(lipgloss.Color("5"))}}
	got := Emphasize(spans, lipgloss.Color("237"))
	if Visible(got) != "fn" {
		t.Fatalf("emphasize lost text: %q", Visible(got))
	}
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	_, _, _, a := got[0].Style.GetBackground().RGBA()
	if a == 0 {
		t.Fatal("expected a background on the playhead span")
	}
}

func TestRenderWithCaretEOL(t *testing.T) {
	t.Parallel()
	spans := []Span{{Text: "ab"}}
	got := RenderWithCaret(spans, 2, true)
	if !strings.Contains(got, "ab") {
		t.Fatalf("caret render lost text: %q", got)
	}
	plain := RenderWithCaret(spans, 2, false)
	if plain != "ab" && !strings.Contains(plain, "ab") {
		t.Fatalf("blink-off = %q", plain)
	}
}

func TestDefaultThemeRegistered(t *testing.T) {
	t.Parallel()
	if _, ok := styles.Registry[DefaultTheme]; !ok {
		t.Fatalf("theme %q missing; have %v", DefaultTheme, styles.Names())
	}
}

func TestPoolReusesHighlighter(t *testing.T) {
	t.Parallel()
	p := NewPool(DefaultTheme)
	a := p.For("a.go")
	b := p.For("a.go")
	if a != b {
		t.Fatal("pool should reuse the same highlighter")
	}
	if p.For("b.md") == a {
		t.Fatal("different paths must not share a lexer")
	}
}

func hasCategory(spans []Span, text string, cat chroma.TokenType) bool {
	for _, sp := range spans {
		if sp.Text == text && sp.Type.Category() == cat {
			return true
		}
	}
	return false
}

func dump(spans []Span) []string {
	out := make([]string, len(spans))
	for i, sp := range spans {
		out[i] = sp.Type.String() + ":" + sp.Text
	}
	return out
}

func visibles(lines [][]Span) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = Visible(l)
	}
	return out
}
