// Package animator compiles structured Git hunks into a cinematic Action
// stream that a 60 FPS scheduler can play back as keystrokes.
//
// The instruction set is the one the TUI will execute:
//
//	OpenFile, MoveCursor, TypeChar, DeleteChar, InsertLine, DeleteLine
//
// Typing delay is sampled from a clamped Gaussian: ordinary runes land in
// 30–60 ms, punctuation and line breaks in 150–300 ms.
package animator

import "time"

// Kind is one editor primitive. Extra kinds (OpenFile, DeleteLine) are the
// minimum needed to switch files and drop empty lines after a backspace pass.
type Kind int

const (
	KindOpenFile Kind = iota
	KindMoveCursor
	KindTypeChar
	KindDeleteChar
	KindInsertLine
	KindDeleteLine
)

func (k Kind) String() string {
	switch k {
	case KindOpenFile:
		return "open"
	case KindMoveCursor:
		return "move"
	case KindTypeChar:
		return "type"
	case KindDeleteChar:
		return "delete"
	case KindInsertLine:
		return "insert"
	case KindDeleteLine:
		return "drop"
	default:
		return "unknown"
	}
}

// Action is one scheduled editor event. Row is 1-based; Col is a 0-based
// rune index (the caret sits *before* that rune, or at EOL when Col == len).
//
// DeleteChar is a backspace: it removes the rune immediately before Col.
type Action struct {
	Kind  Kind
	File  string
	Row   int
	Col   int
	Rune  rune
	Delay time.Duration
}

// Script is the compiled, time-stamped replay of one commit.
type Script struct {
	Commit  string // short hash, for dumps
	Seed    int64
	Actions []Action
}

// Duration is the sum of per-action delays — the theoretical wall time of
// a 1× playback.
func (s Script) Duration() time.Duration {
	var d time.Duration
	for _, a := range s.Actions {
		d += a.Delay
	}
	return d
}

// ForFile keeps only actions that target path, preserving delays.
func (s Script) ForFile(path string) Script {
	if path == "" {
		return Script{Commit: s.Commit, Seed: s.Seed}
	}
	out := make([]Action, 0, len(s.Actions))
	for _, a := range s.Actions {
		if a.File == path {
			out = append(out, a)
		}
	}
	return Script{Commit: s.Commit, Seed: s.Seed, Actions: out}
}
