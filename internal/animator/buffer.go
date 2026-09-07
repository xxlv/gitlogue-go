package animator

// Buffer is a live editor document. The scheduler emits Actions; the TUI
// (and Apply) fold them into a Buffer so the on-screen text is the same
// document the compiler simulated.
type Buffer struct {
	path  string
	lines [][]rune
	row   int // 1-based; 0 when empty
	col   int // 0-based rune index
}

// NewBuffer loads old file text. An empty string is a brand-new file.
func NewBuffer(path, old string) *Buffer {
	return &Buffer{path: path, lines: splitFile(old)}
}

// Path is the file this buffer is bound to.
func (b *Buffer) Path() string { return b.path }

// LineCount is the number of lines currently in the buffer.
func (b *Buffer) LineCount() int {
	if b == nil {
		return 0
	}
	return len(b.lines)
}

// Line returns the 1-based line as a string. Out of range yields "".
func (b *Buffer) Line(row int) string {
	if b == nil || row < 1 || row > len(b.lines) {
		return ""
	}
	return string(b.lines[row-1])
}

// Caret is the 1-based row and 0-based column after the last applied action.
func (b *Buffer) Caret() (row, col int) {
	if b == nil {
		return 0, 0
	}
	return b.row, b.col
}

// String is the buffer as a Git-style blob (trailing newline when non-empty).
func (b *Buffer) String() string {
	if b == nil {
		return ""
	}
	return joinFile(b.lines)
}

// Apply folds one Action into the buffer. OpenFile is a no-op here; Stage
// handles file switches.
func (b *Buffer) Apply(a Action) {
	if b == nil {
		return
	}
	switch a.Kind {
	case KindOpenFile:
		return
	case KindMoveCursor:
		b.row, b.col = a.Row, a.Col
	case KindInsertLine:
		b.lines = insertLineAt(b.lines, a.Row)
		b.row, b.col = a.Row, 0
	case KindDeleteLine:
		b.lines = deleteLineAt(b.lines, a.Row)
		b.row, b.col = a.Row, 0
		if len(b.lines) == 0 {
			b.row = 0
			return
		}
		if b.row > len(b.lines) {
			b.row = len(b.lines)
		}
	case KindTypeChar:
		b.lines = insertRuneAt(b.lines, a.Row, a.Col, a.Rune)
		b.row, b.col = a.Row, a.Col+1
	case KindDeleteChar:
		b.lines = deleteRuneBefore(b.lines, a.Row, a.Col)
		b.row, b.col = a.Row, a.Col-1
		if b.col < 0 {
			b.col = 0
		}
	}
}

// Stage is a multi-file workspace. OpenFile selects the current buffer;
// each file starts at its OldContent.
type Stage struct {
	orig    map[string]string
	files   map[string]*Buffer
	current *Buffer
}

// NewStage binds path → old content. Missing paths are created empty on first use.
func NewStage(oldByPath map[string]string) *Stage {
	s := &Stage{
		orig:  make(map[string]string, len(oldByPath)),
		files: make(map[string]*Buffer, len(oldByPath)),
	}
	for path, old := range oldByPath {
		s.orig[path] = old
		s.files[path] = NewBuffer(path, old)
	}
	return s
}

// Apply dispatches an action to the matching file buffer.
func (s *Stage) Apply(a Action) {
	if s == nil {
		return
	}
	buf := s.ensure(a.File)
	if a.Kind == KindOpenFile || s.current != buf {
		s.current = buf
	}
	buf.Apply(a)
}

// Current is the buffer selected by the last OpenFile (or first mutation).
func (s *Stage) Current() *Buffer {
	if s == nil {
		return nil
	}
	return s.current
}

// File returns the buffer for path, if any.
func (s *Stage) File(path string) *Buffer {
	if s == nil {
		return nil
	}
	return s.files[path]
}

// Reset restores every file to its original OldContent.
func (s *Stage) Reset() {
	if s == nil {
		return
	}
	s.files = make(map[string]*Buffer, len(s.orig))
	for path, old := range s.orig {
		s.files[path] = NewBuffer(path, old)
	}
	s.current = nil
}

// ResetFile rewinds one path to OldContent and makes it current. Other
// buffers are left as they are so a single-file replay can sit beside
// already-typed siblings.
func (s *Stage) ResetFile(path string) {
	if s == nil || path == "" {
		return
	}
	buf := NewBuffer(path, s.orig[path])
	if s.files == nil {
		s.files = make(map[string]*Buffer)
	}
	s.files[path] = buf
	s.current = buf
}

func (s *Stage) ensure(path string) *Buffer {
	if buf, ok := s.files[path]; ok {
		return buf
	}
	old := s.orig[path]
	buf := NewBuffer(path, old)
	s.files[path] = buf
	return buf
}
