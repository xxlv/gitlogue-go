package animator

import (
	"strings"
	"testing"
)

func TestBufferCaretAfterType(t *testing.T) {
	t.Parallel()
	buf := NewBuffer("a.txt", "")
	buf.Apply(Action{Kind: KindInsertLine, Row: 1})
	buf.Apply(Action{Kind: KindTypeChar, Row: 1, Col: 0, Rune: 'a'})
	buf.Apply(Action{Kind: KindTypeChar, Row: 1, Col: 1, Rune: 'b'})
	if buf.String() != "ab\n" {
		t.Fatalf("got %q", buf.String())
	}
	row, col := buf.Caret()
	if row != 1 || col != 2 {
		t.Fatalf("caret = %d:%d, want 1:2", row, col)
	}
}

func TestStageOpenFileSwitch(t *testing.T) {
	t.Parallel()
	st := NewStage(map[string]string{
		"a.txt": "A\n",
		"b.txt": "B\n",
	})
	st.Apply(Action{Kind: KindOpenFile, File: "a.txt"})
	st.Apply(Action{Kind: KindOpenFile, File: "b.txt"})
	if st.Current().Path() != "b.txt" {
		t.Fatalf("current = %q", st.Current().Path())
	}
	st.Reset()
	if st.Current() != nil {
		t.Fatal("reset should clear current")
	}
	if st.File("a.txt").String() != "A\n" {
		t.Fatalf("reset lost a.txt: %q", st.File("a.txt").String())
	}
}

func TestStageResetFileLeavesSiblings(t *testing.T) {
	t.Parallel()
	st := NewStage(map[string]string{
		"a.txt": "A\n",
		"b.txt": "B\n",
	})
	st.Apply(Action{Kind: KindOpenFile, File: "a.txt"})
	st.Apply(Action{Kind: KindTypeChar, File: "a.txt", Row: 1, Col: 1, Rune: 'x'})
	st.ResetFile("b.txt")
	if st.Current() == nil || st.Current().Path() != "b.txt" {
		t.Fatal("ResetFile should select the rewound file")
	}
	if st.File("b.txt").String() != "B\n" {
		t.Fatalf("b.txt = %q", st.File("b.txt").String())
	}
	if !strings.Contains(st.File("a.txt").String(), "x") {
		t.Fatalf("sibling a.txt should keep typed text, got %q", st.File("a.txt").String())
	}
}
