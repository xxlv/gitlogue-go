package animator

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Format renders a human-readable action script for --script dumps.
func (s Script) Format() string {
	var b strings.Builder
	s.Fprint(&b)
	return b.String()
}

// Fprint writes the script dump to w.
func (s Script) Fprint(w io.Writer) {
	commit := s.Commit
	if commit == "" {
		commit = "(local)"
	}
	fmt.Fprintf(w, "# script %s — %d actions, %s, seed=%d\n",
		commit, len(s.Actions), s.Duration().Round(time.Millisecond), s.Seed)
	for i, a := range s.Actions {
		fmt.Fprintf(w, "%5d  %-6s  %s\n", i+1, a.Kind, formatAction(a))
	}
}

func formatAction(a Action) string {
	loc := a.File
	if a.Kind != KindOpenFile {
		loc = fmt.Sprintf("%s:%d:%d", a.File, a.Row, a.Col)
	}
	extra := ""
	if a.Kind == KindTypeChar || a.Kind == KindDeleteChar {
		extra = "  " + quoteRune(a.Rune)
	}
	return fmt.Sprintf("%-32s%s  %s", loc, extra, a.Delay.Round(time.Millisecond))
}

func quoteRune(r rune) string {
	if unicode.IsPrint(r) || r == '\t' {
		return strconv.QuoteRune(r)
	}
	return strconv.QuoteRune(r)
}
