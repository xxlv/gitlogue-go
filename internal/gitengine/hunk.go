package gitengine

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/diff"
	godiff "github.com/go-git/go-git/v5/utils/diff"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

// Number of unchanged lines kept on each side of a change island.
const hunkContext = 3

type rawLine struct {
	Kind    LineKind
	Content string
	OldNo   int // 1-based; 0 on added lines
	NewNo   int // 1-based; 0 on deleted lines
	OldPos  int // old-side cursor, used for pure-insertion headers (e.g. -15,0)
}

func hunksFromFilePatch(fp diff.FilePatch) []Hunk {
	lines := flattenChunks(fp.Chunks())
	return groupHunks(lines, hunkContext)
}

// hunksFromTexts builds unified-diff hunks from two blob texts, the same
// way go-git turns a tree change into a FilePatch (Myers line diff).
func hunksFromTexts(oldContent, newContent string) []Hunk {
	if oldContent == newContent {
		return nil
	}
	var chunks []diff.Chunk
	for _, d := range godiff.Do(oldContent, newContent) {
		var op diff.Operation
		switch d.Type {
		case dmp.DiffEqual:
			op = diff.Equal
		case dmp.DiffDelete:
			op = diff.Delete
		case dmp.DiffInsert:
			op = diff.Add
		default:
			continue
		}
		chunks = append(chunks, textChunk{content: d.Text, op: op})
	}
	return groupHunks(flattenChunks(chunks), hunkContext)
}

type textChunk struct {
	content string
	op      diff.Operation
}

func (c textChunk) Content() string      { return c.content }
func (c textChunk) Type() diff.Operation { return c.op }

func flattenChunks(chunks []diff.Chunk) []rawLine {
	oldNo, newNo := 1, 1
	var out []rawLine
	for _, chunk := range chunks {
		for _, text := range splitLines(chunk.Content()) {
			switch chunk.Type() {
			case diff.Equal:
				out = append(out, rawLine{
					Kind: LineContext, Content: text,
					OldNo: oldNo, NewNo: newNo, OldPos: oldNo,
				})
				oldNo++
				newNo++
			case diff.Delete:
				out = append(out, rawLine{
					Kind: LineDeleted, Content: text,
					OldNo: oldNo, NewNo: 0, OldPos: oldNo,
				})
				oldNo++
			case diff.Add:
				out = append(out, rawLine{
					Kind: LineAdded, Content: text,
					OldNo: 0, NewNo: newNo, OldPos: oldNo - 1,
				})
				newNo++
			}
		}
	}
	return out
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	// A trailing newline is a line terminator, not an extra empty line.
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

func groupHunks(lines []rawLine, context int) []Hunk {
	n := len(lines)
	if n == 0 {
		return nil
	}

	isChange := make([]bool, n)
	any := false
	for i, l := range lines {
		if l.Kind != LineContext {
			isChange[i] = true
			any = true
		}
	}
	if !any {
		return nil
	}

	var hunks []Hunk
	i := 0
	for i < n {
		if !isChange[i] {
			i++
			continue
		}

		start := i - context
		if start < 0 {
			start = 0
		}

		lastChange := i
		j := i + 1
		for j < n {
			if isChange[j] {
				lastChange = j
				j++
				continue
			}
			// Absorb the gap if the next change is close enough that the
			// two context windows would overlap or touch.
			k := j
			for k < n && !isChange[k] {
				k++
			}
			if k < n && k-lastChange-1 <= context*2 {
				j = k
				continue
			}
			break
		}

		end := lastChange + 1 + context
		if end > n {
			end = n
		}
		hunks = append(hunks, buildHunk(lines[start:end]))
		i = end
	}
	return hunks
}

func buildHunk(lines []rawLine) Hunk {
	h := Hunk{Lines: make([]DiffLine, 0, len(lines))}
	for _, l := range lines {
		h.Lines = append(h.Lines, DiffLine{
			Kind:      l.Kind,
			OldNumber: l.OldNo,
			NewNumber: l.NewNo,
			Content:   l.Content,
		})
		switch l.Kind {
		case LineContext:
			if h.OldStart == 0 {
				h.OldStart = l.OldNo
			}
			if h.NewStart == 0 {
				h.NewStart = l.NewNo
			}
			h.OldLines++
			h.NewLines++
		case LineDeleted:
			if h.OldStart == 0 {
				h.OldStart = l.OldNo
			}
			h.OldLines++
		case LineAdded:
			if h.NewStart == 0 {
				h.NewStart = l.NewNo
			}
			h.NewLines++
		}
	}

	// Pure insertion: Git reports the old range as "<line>,0", where <line>
	// is the last old line *before* the inserted block (0 at the file start).
	if h.OldLines == 0 && len(lines) > 0 {
		h.OldStart = lines[0].OldPos
		if h.OldStart < 0 {
			h.OldStart = 0
		}
	}
	// Pure deletion: analogous new-side empty range.
	if h.NewLines == 0 && len(lines) > 0 && h.NewStart == 0 {
		// Use neighbouring context's new number if present; otherwise 0.
		for _, l := range lines {
			if l.NewNo > 0 {
				h.NewStart = l.NewNo
				break
			}
		}
	}

	h.Header = formatHunkHeader(h.OldStart, h.OldLines, h.NewStart, h.NewLines)
	return h
}

func formatHunkHeader(oldStart, oldLines, newStart, newLines int) string {
	return fmt.Sprintf("@@ -%s +%s @@",
		hunkRange(oldStart, oldLines),
		hunkRange(newStart, newLines),
	)
}

func hunkRange(start, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}
