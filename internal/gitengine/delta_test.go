package gitengine

import (
	"strings"
	"testing"
)

func TestSnapshotSigEmpty(t *testing.T) {
	t.Parallel()
	if SnapshotSig(nil) != "" {
		t.Fatal("nil should be empty")
	}
	if SnapshotSig(&CommitDiff{}) != "" {
		t.Fatal("no files should be empty")
	}
}

func TestSnapshotSigChangesWithContent(t *testing.T) {
	t.Parallel()
	a := &CommitDiff{Files: []FileChange{FileFromContents("a.go", "old\n", "new\n")}}
	b := &CommitDiff{Files: []FileChange{FileFromContents("a.go", "old\n", "new!\n")}}
	if SnapshotSig(a) == "" || SnapshotSig(a) != SnapshotSig(a) {
		t.Fatal("same diff must be stable")
	}
	if SnapshotSig(a) == SnapshotSig(b) {
		t.Fatal("content change must change signature")
	}
}

func TestFileFromContentsKinds(t *testing.T) {
	t.Parallel()
	add := FileFromContents("n.go", "", "package n\n")
	if add.Kind != ChangeAdded || add.Path != "n.go" || len(add.Hunks) == 0 {
		t.Fatalf("add = %+v", add)
	}
	del := FileFromContents("g.go", "gone\n", "")
	if del.Kind != ChangeDeleted || del.OldPath != "g.go" {
		t.Fatalf("del = %+v", del)
	}
	mod := FileFromContents("m.go", "a\n", "b\n")
	if mod.Kind != ChangeModified || mod.Path != "m.go" || len(mod.Hunks) == 0 {
		t.Fatalf("mod = %+v", mod)
	}
}

func TestLiveDeltaFromHEAD(t *testing.T) {
	t.Parallel()
	want := &CommitDiff{Files: []FileChange{FileFromContents("a.go", "hi\n", "hi!\n")}}
	delta := LiveDelta(nil, map[string]string{"a.go": "hi\n"}, want)
	if len(delta) != 1 {
		t.Fatalf("delta = %d, want 1", len(delta))
	}
	if delta[0].OldContent != "hi\n" || delta[0].NewContent != "hi!\n" {
		t.Fatalf("delta contents %+v", delta[0])
	}
}

func TestLiveDeltaIncremental(t *testing.T) {
	t.Parallel()
	want := &CommitDiff{Files: []FileChange{FileFromContents("a.go", "hi\n", "hi!!\n")}}
	delta := LiveDelta(map[string]string{"a.go": "hi!\n"}, map[string]string{"a.go": "hi\n"}, want)
	if len(delta) != 1 {
		t.Fatalf("delta = %d, want 1", len(delta))
	}
	if delta[0].OldContent != "hi!\n" || delta[0].NewContent != "hi!!\n" {
		t.Fatalf("incremental old=%q new=%q", delta[0].OldContent, delta[0].NewContent)
	}
	if delta[0].Kind != ChangeModified {
		t.Fatalf("kind = %s", delta[0].Kind)
	}
}

func TestLiveDeltaSkipAlreadyApplied(t *testing.T) {
	t.Parallel()
	want := &CommitDiff{Files: []FileChange{FileFromContents("a.go", "hi\n", "hi!\n")}}
	delta := LiveDelta(map[string]string{"a.go": "hi!\n"}, map[string]string{"a.go": "hi\n"}, want)
	if len(delta) != 0 {
		t.Fatalf("already applied: %+v", delta)
	}
}

func TestLiveDeltaRevertRemovedFile(t *testing.T) {
	t.Parallel()
	delta := LiveDelta(
		map[string]string{"a.go": "wip\n"},
		map[string]string{"a.go": "head\n"},
		&CommitDiff{},
	)
	if len(delta) != 1 {
		t.Fatalf("revert delta = %d, want 1", len(delta))
	}
	if delta[0].OldContent != "wip\n" || delta[0].NewContent != "head\n" {
		t.Fatalf("revert %+v", delta[0])
	}
	if !strings.Contains(delta[0].DisplayPath(), "a.go") {
		t.Fatalf("path = %q", delta[0].DisplayPath())
	}
}

func TestLiveDeltaDeleteAddedFile(t *testing.T) {
	t.Parallel()
	delta := LiveDelta(map[string]string{"new.go": "pkg\n"}, nil, nil)
	if len(delta) != 1 || delta[0].Kind != ChangeDeleted {
		t.Fatalf("added-then-gone = %+v", delta)
	}
}
