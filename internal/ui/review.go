package ui

import (
	"strings"

	"github.com/xxlv/gitlogue-go/internal/gitengine"
)

// reviewMark is how a finished replay row should be read. Playback itself is
// a typing movie; once a file is done the editor becomes a review overlay
// so added / deleted / replaced lines stay distinguishable.
type reviewMark uint8

const (
	reviewContext reviewMark = iota
	reviewAdd
	reviewDel
	reviewMod
)

func (m reviewMark) glyph() string {
	switch m {
	case reviewAdd:
		return "+"
	case reviewDel:
		return "-"
	case reviewMod:
		return "~"
	default:
		return " "
	}
}

// reviewRow is one on-screen editor line in review mode. Deleted lines are
// ghost rows (Number == 0) inserted at their original site so a removal is
// still visible after the buffer has forgotten it.
type reviewRow struct {
	Mark   reviewMark
	Number int // new-side 1-based; 0 for pure deletions
	Text   string
}

func (m reviewMark) isGhost() bool { return m == reviewDel }

func lookupFile(files []gitengine.FileChange, path string) *gitengine.FileChange {
	for i := range files {
		if files[i].DisplayPath() == path {
			return &files[i]
		}
	}
	return nil
}

func (m Model) lookupSelected() *gitengine.FileChange {
	buf := m.editorBuffer()
	if buf == nil {
		return nil
	}
	return lookupFile(m.files, buf.Path())
}

func (m Model) canReview() bool {
	buf := m.editorBuffer()
	if buf == nil || buf.Path() == "" || !m.filePlayed(buf.Path()) {
		return false
	}
	if m.sched != nil && m.sched.Playing() && !m.browsing && buf.Path() == m.playheadPath() {
		return false
	}
	fc := lookupFile(m.files, buf.Path())
	return fc != nil && !fc.Binary
}

func (m Model) showingFinal() bool {
	return m.finalView && m.canReview()
}

func (m *Model) toggleFinalView() {
	m.finalView = !m.finalView
	if maxOff := m.editorMaxOff(); m.editOff > maxOff {
		m.editOff = maxOff
	}
}

func (m Model) reviewDoc() ([]reviewRow, bool) {
	if !m.canReview() {
		return nil, false
	}
	if m.finalView {
		return m.cleanRows()
	}
	buf := m.editorBuffer()
	fc := lookupFile(m.files, buf.Path())
	rows, ok := buildReview(*fc)
	if !ok {
		return nil, false
	}
	if len(rows) == 0 && buf.LineCount() > 0 && fc.Kind == gitengine.ChangeAdded {
		rows = markAll(bufferStrings(buf), reviewAdd)
	}
	return rows, true
}

func (m Model) cleanRows() ([]reviewRow, bool) {
	buf := m.editorBuffer()
	fc := lookupFile(m.files, buf.Path())
	if fc == nil {
		return nil, false
	}
	lines := splitBlob(fc.NewContent)
	if len(lines) == 0 && buf != nil && buf.LineCount() > 0 {
		lines = bufferStrings(buf)
	}
	return markAll(lines, reviewContext), true
}

func bufferStrings(buf interface {
	LineCount() int
	Line(int) string
}) []string {
	n := buf.LineCount()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = buf.Line(i + 1)
	}
	return out
}

func buildReview(fc gitengine.FileChange) (rows []reviewRow, ok bool) {
	if fc.Binary {
		return nil, false
	}
	newLines := splitBlob(fc.NewContent)
	oldLines := splitBlob(fc.OldContent)
	if len(fc.Hunks) == 0 {
		return reviewWithoutHunks(fc, newLines, oldLines)
	}

	out := make([]reviewRow, 0, len(newLines)+8)
	nextNew := 1
	for _, h := range fc.Hunks {
		fillTo := h.NewStart
		if fillTo < 1 {
			fillTo = nextNew
		}
		for nextNew < fillTo && nextNew <= len(newLines) {
			out = append(out, reviewRow{Mark: reviewContext, Number: nextNew, Text: newLines[nextNew-1]})
			nextNew++
		}
		out, nextNew = appendHunk(out, h, nextNew)
	}
	for nextNew <= len(newLines) {
		out = append(out, reviewRow{Mark: reviewContext, Number: nextNew, Text: newLines[nextNew-1]})
		nextNew++
	}
	return out, true
}

func reviewWithoutHunks(fc gitengine.FileChange, newLines, oldLines []string) ([]reviewRow, bool) {
	switch fc.Kind {
	case gitengine.ChangeAdded:
		return markAll(newLines, reviewAdd), true
	case gitengine.ChangeDeleted:
		return markAll(oldLines, reviewDel), true
	default:
		return nil, false
	}
}

func markAll(lines []string, mark reviewMark) []reviewRow {
	if len(lines) == 0 {
		return []reviewRow{}
	}
	out := make([]reviewRow, len(lines))
	for i, text := range lines {
		row := reviewRow{Mark: mark, Text: text}
		if mark != reviewDel {
			row.Number = i + 1
		}
		out[i] = row
	}
	return out
}

func appendHunk(out []reviewRow, h gitengine.Hunk, nextNew int) ([]reviewRow, int) {
	i := 0
	for i < len(h.Lines) {
		l := h.Lines[i]
		if l.Kind == gitengine.LineContext {
			out = append(out, reviewRow{Mark: reviewContext, Number: l.NewNumber, Text: l.Content})
			if l.NewNumber >= nextNew {
				nextNew = l.NewNumber + 1
			}
			i++
			continue
		}
		dels, adds, n := collectChange(h.Lines[i:])
		paired := min(len(dels), len(adds))
		for k := 0; k < paired; k++ {
			out = append(out, reviewRow{Mark: reviewDel, Text: dels[k].Content})
			out = append(out, reviewRow{Mark: reviewMod, Number: adds[k].NewNumber, Text: adds[k].Content})
			if adds[k].NewNumber >= nextNew {
				nextNew = adds[k].NewNumber + 1
			}
		}
		for k := paired; k < len(dels); k++ {
			out = append(out, reviewRow{Mark: reviewDel, Text: dels[k].Content})
		}
		for k := paired; k < len(adds); k++ {
			out = append(out, reviewRow{Mark: reviewAdd, Number: adds[k].NewNumber, Text: adds[k].Content})
			if adds[k].NewNumber >= nextNew {
				nextNew = adds[k].NewNumber + 1
			}
		}
		i += n
	}
	return out, nextNew
}

func collectChange(lines []gitengine.DiffLine) (dels, adds []gitengine.DiffLine, n int) {
	for _, l := range lines {
		switch l.Kind {
		case gitengine.LineDeleted:
			dels = append(dels, l)
		case gitengine.LineAdded:
			adds = append(adds, l)
		default:
			return dels, adds, n
		}
		n++
	}
	return dels, adds, n
}

func splitBlob(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

func reviewIndex(rows []reviewRow, newRow int) int {
	last := 0
	for i, r := range rows {
		if !r.Mark.isGhost() && r.Number == newRow {
			return i
		}
		if !r.Mark.isGhost() {
			last = i
		}
	}
	return last
}

func reviewStats(rows []reviewRow) (add, mod, del int) {
	for _, r := range rows {
		switch r.Mark {
		case reviewAdd:
			add++
		case reviewMod:
			mod++
		case reviewDel:
			del++
		}
	}
	return add, mod, del
}
