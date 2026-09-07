package animator

import (
	"math/rand/v2"

	"github.com/xxlv/gitlogue-go/internal/gitengine"
)

type compiler struct {
	opts    Options
	rng     *rand.Rand
	file    string
	lines   [][]rune
	row     int // 1-based caret row; 0 when the buffer is empty
	col     int // 0-based caret column
	bufRow  int // 1-based row of the next unprocessed old line
	lastOld int // last original old-file line consumed (0 at start)
	out     []Action
}

// Compile turns a commit patch into a playable Action script. Files are
// processed in gitengine order; each file starts from its OldContent.
func Compile(diff *gitengine.CommitDiff, opts Options) Script {
	if diff == nil {
		return Script{}
	}
	opts = opts.withDefaults()
	rng, seed := newRNG(opts.Seed)

	var actions []Action
	for i := range diff.Files {
		c := newCompiler(opts, rng, diff.Files[i].DisplayPath())
		c.compileFile(diff.Files[i])
		actions = append(actions, c.out...)
	}
	return Script{
		Commit:  diff.Commit.ShortHash,
		Seed:    seed,
		Actions: actions,
	}
}

// CompileFile is the single-file entry used by tests and the scheduler.
func CompileFile(fc gitengine.FileChange, opts Options) Script {
	opts = opts.withDefaults()
	rng, seed := newRNG(opts.Seed)
	c := newCompiler(opts, rng, fc.DisplayPath())
	c.compileFile(fc)
	return Script{Seed: seed, Actions: c.out}
}

func newCompiler(opts Options, rng *rand.Rand, file string) *compiler {
	return &compiler{
		opts:   opts,
		rng:    rng,
		file:   file,
		bufRow: 1,
	}
}

func (c *compiler) compileFile(fc gitengine.FileChange) {
	c.lines = splitFile(fc.OldContent)
	c.emit(KindOpenFile, 0, 0, 0)

	if fc.Binary {
		return
	}
	if len(fc.Hunks) == 0 {
		c.compileWithoutHunks(fc)
		return
	}
	for _, h := range fc.Hunks {
		c.compileHunk(h)
	}
}

func (c *compiler) compileWithoutHunks(fc gitengine.FileChange) {
	switch fc.Kind {
	case gitengine.ChangeAdded:
		c.typeAll(fc.NewContent)
	case gitengine.ChangeDeleted:
		c.eraseAll()
	}
}

func (c *compiler) compileHunk(h gitengine.Hunk) {
	row := c.seekHunk(h)
	i := 0
	for i < len(h.Lines) {
		if h.Lines[i].Kind == gitengine.LineContext {
			row++
			i++
			continue
		}

		dels, adds, n := collectBlock(h.Lines[i:])
		c.move(row, 0)
		c.applyBlock(row, dels, adds)
		row += len(adds)
		i += n
	}
	c.bufRow = row
	if h.OldLines == 0 {
		c.lastOld = h.OldStart
		return
	}
	c.lastOld = h.OldStart + h.OldLines - 1
}

func (c *compiler) seekHunk(h gitengine.Hunk) int {
	targetOld := h.OldStart
	if h.OldLines == 0 {
		if h.OldStart == 0 {
			targetOld = 1
		} else {
			targetOld = h.OldStart + 1
		}
	}
	if targetOld > c.lastOld+1 {
		c.bufRow += targetOld - c.lastOld - 1
	}
	if c.bufRow < 1 {
		c.bufRow = 1
	}
	return c.bufRow
}

func collectBlock(lines []gitengine.DiffLine) (dels, adds []string, n int) {
	for _, l := range lines {
		switch l.Kind {
		case gitengine.LineDeleted:
			dels = append(dels, l.Content)
		case gitengine.LineAdded:
			adds = append(adds, l.Content)
		default:
			return dels, adds, n
		}
		n++
	}
	return dels, adds, n
}

func (c *compiler) applyBlock(row int, dels, adds []string) {
	n := min(len(dels), len(adds))
	for k := 0; k < n; k++ {
		c.replaceLine(row + k)
		c.typeRunes(adds[k])
	}
	for k := n; k < len(dels); k++ {
		c.eraseLine(row + n)
	}
	insertAt := row + n
	for k := n; k < len(adds); k++ {
		c.insertAndType(insertAt, adds[k])
		insertAt++
	}
}

func (c *compiler) replaceLine(row int) {
	if row < 1 || row > len(c.lines) {
		c.insertLine(row)
		return
	}
	c.move(row, len(c.lines[row-1]))
	for c.col > 0 {
		c.deleteChar()
	}
}

func (c *compiler) eraseLine(row int) {
	if row < 1 || row > len(c.lines) {
		return
	}
	c.replaceLine(row)
	c.deleteLine()
}

func (c *compiler) insertAndType(row int, text string) {
	c.insertLine(row)
	c.typeRunes(text)
}

func (c *compiler) typeRunes(text string) {
	for _, r := range text {
		c.typeChar(r)
	}
}

func (c *compiler) typeAll(content string) {
	lines := splitFile(content)
	for i, line := range lines {
		c.insertAndType(i+1, string(line))
	}
}

func (c *compiler) eraseAll() {
	for len(c.lines) > 0 {
		c.eraseLine(1)
	}
}

func (c *compiler) move(row, col int) {
	if len(c.lines) == 0 {
		return
	}
	if row < 1 {
		row = 1
	}
	if row > len(c.lines) {
		row = len(c.lines)
	}
	maxCol := len(c.lines[row-1])
	if col < 0 {
		col = 0
	}
	if col > maxCol {
		col = maxCol
	}
	if row == c.row && col == c.col {
		return
	}
	c.row, c.col = row, col
	c.emit(KindMoveCursor, row, col, 0)
}

func (c *compiler) insertLine(row int) {
	if row < 1 {
		row = 1
	}
	empty := []rune{}
	idx := row - 1
	if idx >= len(c.lines) {
		c.lines = append(c.lines, empty)
		row = len(c.lines)
	} else {
		c.lines = append(c.lines, nil)
		copy(c.lines[idx+1:], c.lines[idx:])
		c.lines[idx] = empty
	}
	c.row, c.col = row, 0
	c.emit(KindInsertLine, row, 0, 0)
}

func (c *compiler) deleteLine() {
	if c.row < 1 || c.row > len(c.lines) {
		return
	}
	row := c.row
	c.emit(KindDeleteLine, row, 0, 0)
	idx := row - 1
	c.lines = append(c.lines[:idx], c.lines[idx+1:]...)
	c.col = 0
	if len(c.lines) == 0 {
		c.row = 0
		return
	}
	if c.row > len(c.lines) {
		c.row = len(c.lines)
	}
}

func (c *compiler) typeChar(r rune) {
	if len(c.lines) == 0 {
		c.insertLine(1)
	}
	if c.row < 1 {
		c.row = 1
	}
	idx := c.row - 1
	line := c.lines[idx]
	col := c.col
	if col > len(line) {
		col = len(line)
	}
	line = append(line, 0)
	copy(line[col+1:], line[col:])
	line[col] = r
	c.lines[idx] = line
	c.emit(KindTypeChar, c.row, col, r)
	c.col = col + 1
}

func (c *compiler) deleteChar() {
	if c.row < 1 || c.col < 1 {
		return
	}
	idx := c.row - 1
	line := c.lines[idx]
	r := line[c.col-1]
	c.emit(KindDeleteChar, c.row, c.col, r)
	c.lines[idx] = append(line[:c.col-1], line[c.col:]...)
	c.col--
}

func (c *compiler) emit(kind Kind, row, col int, r rune) {
	c.out = append(c.out, Action{
		Kind:  kind,
		File:  c.file,
		Row:   row,
		Col:   col,
		Rune:  r,
		Delay: c.delayFor(kind, r),
	})
}
