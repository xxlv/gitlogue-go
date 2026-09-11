package ui

import (
	"strings"
	"testing"

	"github.com/xxlv/gitlogue-go/internal/gitengine"
)

func TestBuildReviewReplaceAndInsert(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileChange{
		Path:       "a.go",
		Kind:       gitengine.ChangeModified,
		OldContent: "keep\nold\n",
		NewContent: "keep\nnew\nextra\n",
		Hunks: []gitengine.Hunk{{
			OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 3,
			Lines: []gitengine.DiffLine{
				{Kind: gitengine.LineContext, OldNumber: 1, NewNumber: 1, Content: "keep"},
				{Kind: gitengine.LineDeleted, OldNumber: 2, Content: "old"},
				{Kind: gitengine.LineAdded, NewNumber: 2, Content: "new"},
				{Kind: gitengine.LineAdded, NewNumber: 3, Content: "extra"},
			},
		}},
	}
	rows, ok := buildReview(fc)
	if !ok {
		t.Fatal("expected review overlay")
	}
	got := dumpReview(rows)
	want := "  keep\n- old\n~ new\n+ extra"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	add, mod, del := reviewStats(rows)
	if add != 1 || mod != 1 || del != 1 {
		t.Fatalf("stats +%d ~%d -%d, want +1 ~1 -1", add, mod, del)
	}
}

func TestBuildReviewPureDelete(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileChange{
		Path:       "a.go",
		Kind:       gitengine.ChangeModified,
		OldContent: "a\ngone\nb\n",
		NewContent: "a\nb\n",
		Hunks: []gitengine.Hunk{{
			OldStart: 1, OldLines: 3, NewStart: 1, NewLines: 2,
			Lines: []gitengine.DiffLine{
				{Kind: gitengine.LineContext, OldNumber: 1, NewNumber: 1, Content: "a"},
				{Kind: gitengine.LineDeleted, OldNumber: 2, Content: "gone"},
				{Kind: gitengine.LineContext, OldNumber: 3, NewNumber: 2, Content: "b"},
			},
		}},
	}
	rows, ok := buildReview(fc)
	if !ok {
		t.Fatal("expected review overlay")
	}
	if got := dumpReview(rows); got != "  a\n- gone\n  b" {
		t.Fatalf("got %q", dumpReview(rows))
	}
}

func TestBuildReviewFillsUnchanged(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileChange{
		Path:       "a.go",
		Kind:       gitengine.ChangeModified,
		OldContent: "one\ntwo\nthree\nfour\n",
		NewContent: "one\ntwo\nTHREE\nfour\n",
		Hunks: []gitengine.Hunk{{
			OldStart: 3, OldLines: 1, NewStart: 3, NewLines: 1,
			Lines: []gitengine.DiffLine{
				{Kind: gitengine.LineDeleted, OldNumber: 3, Content: "three"},
				{Kind: gitengine.LineAdded, NewNumber: 3, Content: "THREE"},
			},
		}},
	}
	rows, ok := buildReview(fc)
	if !ok {
		t.Fatal("expected review overlay")
	}
	if got := dumpReview(rows); got != "  one\n  two\n- three\n~ THREE\n  four" {
		t.Fatalf("got:\n%s", dumpReview(rows))
	}
}

func TestBuildReviewWholeFile(t *testing.T) {
	t.Parallel()
	added, ok := buildReview(gitengine.FileChange{
		Path: "new.go", Kind: gitengine.ChangeAdded, NewContent: "a\nb\n",
	})
	if !ok || dumpReview(added) != "+ a\n+ b" {
		t.Fatalf("added = ok=%v %q", ok, dumpReview(added))
	}
	deleted, ok := buildReview(gitengine.FileChange{
		Path: "", OldPath: "gone.go", Kind: gitengine.ChangeDeleted, OldContent: "x\n",
	})
	if !ok || dumpReview(deleted) != "- x" {
		t.Fatalf("deleted = ok=%v %q", ok, dumpReview(deleted))
	}
	_, ok = buildReview(gitengine.FileChange{
		Path: "mod.go", Kind: gitengine.ChangeModified, OldContent: "same\n", NewContent: "same\n",
	})
	if ok {
		t.Fatal("modified without hunks should keep the live editor")
	}
}

func TestReviewIndexSkipsGhosts(t *testing.T) {
	t.Parallel()
	rows := []reviewRow{
		{Mark: reviewContext, Number: 1, Text: "a"},
		{Mark: reviewDel, Text: "old"},
		{Mark: reviewMod, Number: 2, Text: "new"},
	}
	if got := reviewIndex(rows, 2); got != 2 {
		t.Fatalf("caret on new line 2 -> display %d, want 2", got)
	}
}

func dumpReview(rows []reviewRow) string {
	parts := make([]string, len(rows))
	for i, r := range rows {
		parts[i] = r.Mark.glyph() + " " + r.Text
	}
	return strings.Join(parts, "\n")
}
