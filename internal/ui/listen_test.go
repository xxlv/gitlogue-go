package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/xxlv/gitlogue-go/internal/animator"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
	"github.com/xxlv/gitlogue-go/internal/scheduler"
	"github.com/xxlv/gitlogue-go/internal/watch"
)

func TestListenIdleShowsHint(t *testing.T) {
	t.Parallel()
	m := NewPlaylist(nil, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	m.width, m.height = 100, 24
	m.EnableListen(nil, "")
	view := m.View()
	for _, want := range []string{"LISTEN", "WIP", "listening", "save a tracked file"} {
		if !strings.Contains(view, want) {
			t.Fatalf("idle listen missing %q:\n%s", want, view)
		}
	}
}

func TestListenAutoAppliesIncremental(t *testing.T) {
	t.Parallel()
	first := gitengine.FileFromContents("a.go", "hi\n", "hi!\n")
	diff := &gitengine.CommitDiff{
		Commit: gitengine.CommitInfo{Hash: gitengine.WorktreeHash, ShortHash: gitengine.WorktreeHash, Message: "uncommitted\n"},
		Files:  []gitengine.FileChange{first},
	}
	m := NewPlayer(animator.Compile(diff, animator.Options{Seed: 1}), diff, scheduler.Options{FPS: 60, MaxStep: 10 * time.Second}, "")
	m.width, m.height = 100, 24
	m.Seed = 1
	m.EnableListen(nil, gitengine.SnapshotSig(diff))
	m = drainFrames(t, m)
	if got := m.stage.File("a.go").String(); got != "hi!\n" {
		t.Fatalf("after first play got %q", got)
	}

	next := gitengine.FileFromContents("a.go", "hi\n", "hi!!\n")
	updated, _ := m.Update(watch.Msg{
		Diff: &gitengine.CommitDiff{
			Commit: diff.Commit,
			Files:  []gitengine.FileChange{next},
		},
		Sig: gitengine.SnapshotSig(&gitengine.CommitDiff{Files: []gitengine.FileChange{next}}),
	})
	m = updated.(Model)
	if m.sched == nil || m.sched.Done() {
		t.Fatal("listen should start playing the incremental edit")
	}
	m = drainFrames(t, m)
	if got := m.stage.File("a.go").String(); got != "hi!!\n" {
		t.Fatalf("after incremental play got %q, want hi!!", got)
	}
	if !m.listen {
		t.Fatal("listen flag should stay on")
	}
}

func TestListenFirstSnapshotFromIdle(t *testing.T) {
	t.Parallel()
	m := NewPlaylist(nil, scheduler.Options{FPS: 60, MaxStep: 10 * time.Second}, "")
	m.width, m.height = 100, 24
	m.Seed = 1
	m.EnableListen(nil, "")
	fc := gitengine.FileFromContents("a.go", "", "hello\n")
	diff := &gitengine.CommitDiff{
		Commit: gitengine.CommitInfo{Hash: gitengine.WorktreeHash, ShortHash: gitengine.WorktreeHash, Message: "uncommitted\n"},
		Files:  []gitengine.FileChange{fc},
	}
	updated, _ := m.Update(watch.Msg{Diff: diff, Sig: gitengine.SnapshotSig(diff)})
	m = updated.(Model)
	if len(m.files) != 1 || m.hash != gitengine.WorktreeHash {
		t.Fatalf("files=%d hash=%q", len(m.files), m.hash)
	}
	m = drainFrames(t, m)
	if got := m.stage.File("a.go").String(); got != "hello\n" {
		t.Fatalf("typed %q, want hello\\n", got)
	}
}

func TestListenSameSigIsNoop(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileFromContents("a.go", "a\n", "b\n")
	diff := &gitengine.CommitDiff{Files: []gitengine.FileChange{fc}}
	sig := gitengine.SnapshotSig(diff)
	m := NewPlayer(animator.Compile(diff, animator.Options{Seed: 1}), diff, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	m.EnableListen(nil, sig)
	updated, _ := m.Update(watch.Msg{Diff: diff, Sig: sig})
	m = updated.(Model)
	if m.fileReplay {
		t.Fatal("identical snapshot must not restart playback")
	}
}

func drainFrames(t *testing.T, m Model) Model {
	t.Helper()
	now := time.Unix(0, 0).UTC()
	for i := 0; i < 80; i++ {
		updated, _ := m.Update(scheduler.FrameMsg{Time: now.Add(time.Duration(i) * time.Second)})
		m = updated.(Model)
		if m.sched != nil && m.sched.Done() {
			return m
		}
	}
	t.Fatal("playback did not finish")
	return m
}
