package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xxlv/gitlogue-go/internal/animator"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
	"github.com/xxlv/gitlogue-go/internal/scheduler"
)

func TestPlayerViewContainsStatus(t *testing.T) {
	t.Parallel()
	script := animator.Script{
		Commit: "abc1234",
		Actions: []animator.Action{
			{Kind: animator.KindOpenFile, File: "hello.go", Delay: time.Millisecond},
			{Kind: animator.KindInsertLine, File: "hello.go", Row: 1, Delay: time.Millisecond},
			{Kind: animator.KindTypeChar, File: "hello.go", Row: 1, Col: 0, Rune: 'x', Delay: time.Millisecond},
		},
	}
	diff := &gitengine.CommitDiff{
		Commit: gitengine.CommitInfo{
			ShortHash: "abc1234",
			Author:    gitengine.Person{Name: "Alice", Email: "alice@example.com"},
			Committer: gitengine.Person{Name: "Bob", Email: "bob@example.com"},
			Message:   "fix logs",
			Trailers: []gitengine.Trailer{
				{Key: "Co-authored-by", Value: "Copilot <copilot@github.com>"},
				{Key: "Reviewed-by", Value: "Chen <chen@example.com>"},
			},
		},
		Files: []gitengine.FileChange{
			{Path: "hello.go", Kind: gitengine.ChangeModified, OldContent: ""},
			{Path: "README.md", Kind: gitengine.ChangeAdded},
		},
	}
	m := NewPlayer(script, diff, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	m.width, m.height = 100, 24

	now := time.Unix(0, 0).UTC()
	updated, cmd := m.Update(scheduler.FrameMsg{Time: now})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected a Tick cmd")
	}
	updated, _ = m.Update(scheduler.FrameMsg{Time: now.Add(20 * time.Millisecond)})
	m = updated.(Model)

	view := m.View()
	for _, want := range []string{"gitlogue-go", "hello.go", "README.md", "FILES", "Space", "Alice", "Bob", "Copilot", "Chen", "author", "agent"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "ms") || strings.Contains(view, "fps") {
		t.Fatalf("status still has a jittery clock or fps:\n%s", view)
	}
}

func TestClockPairFixedWidth(t *testing.T) {
	t.Parallel()
	a := clockPair(12*time.Second+400*time.Millisecond, 68*time.Second)
	b := clockPair(12*time.Second+900*time.Millisecond, 68*time.Second)
	c := clockPair(13*time.Second, 68*time.Second)
	if a != "00:12 / 01:08" {
		t.Fatalf("sub-second should floor: %q", a)
	}
	if a != b {
		t.Fatalf("same second must not change the field: %q vs %q", a, b)
	}
	if len(a) != len(c) {
		t.Fatalf("width jumped %q (%d) -> %q (%d)", a, len(a), c, len(c))
	}
	if c != "00:13 / 01:08" {
		t.Fatalf("next second = %q", c)
	}
}

func TestLayoutTwentyEighty(t *testing.T) {
	t.Parallel()
	m := Model{width: 100, height: 24}
	treeW, editW, bodyH := m.layout()
	if treeW != 20 {
		t.Fatalf("treeW=%d, want 20", treeW)
	}
	if treeW+1+editW != 100 {
		t.Fatalf("tree+sep+edit=%d, want 100", treeW+1+editW)
	}
	if bodyH != 20 {
		t.Fatalf("bodyH=%d, want 20", bodyH)
	}
}

func TestTreeRowsNested(t *testing.T) {
	t.Parallel()
	rows := treeRows([]gitengine.FileChange{
		{Path: "internal/ui/player.go", Kind: gitengine.ChangeModified},
		{Path: "README.md", Kind: gitengine.ChangeAdded},
	})
	var got []string
	for _, r := range rows {
		if r.IsDir {
			got = append(got, r.Display+"/")
			continue
		}
		got = append(got, r.Path)
	}
	want := []string{"README.md", "internal/", "ui/", "internal/ui/player.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTreeCursorPlayed(t *testing.T) {
	t.Parallel()
	if got := treeCursor(true, false); got != "▸" {
		t.Fatalf("playing = %q, want ▸", got)
	}
	if got := treeCursor(false, true); got != "✓" {
		t.Fatalf("played = %q, want ✓", got)
	}
	if got := treeCursor(true, true); got != "✓" {
		t.Fatalf("playhead after done = %q, want ✓", got)
	}
	if got := treeCursor(false, false); got != " " {
		t.Fatalf("upcoming = %q, want space", got)
	}
}

func TestPlayedFilesMarkedInTree(t *testing.T) {
	t.Parallel()
	script := animator.Script{
		Commit: "abc1234",
		Actions: []animator.Action{
			{Kind: animator.KindOpenFile, File: "a.go", Delay: time.Millisecond},
			{Kind: animator.KindTypeChar, File: "a.go", Row: 1, Col: 0, Rune: 'x', Delay: 80 * time.Millisecond},
			{Kind: animator.KindOpenFile, File: "b.go", Delay: time.Millisecond},
			{Kind: animator.KindTypeChar, File: "b.go", Row: 1, Col: 0, Rune: 'y', Delay: time.Millisecond},
		},
	}
	diff := &gitengine.CommitDiff{
		Files: []gitengine.FileChange{
			{Path: "a.go", Kind: gitengine.ChangeAdded},
			{Path: "b.go", Kind: gitengine.ChangeAdded},
			{Path: "c.go", Kind: gitengine.ChangeAdded},
		},
	}
	m := NewPlayer(script, diff, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	m.width, m.height = 100, 24

	now := time.Unix(0, 0).UTC()
	updated, _ := m.Update(scheduler.FrameMsg{Time: now})
	m = updated.(Model)
	if m.filePlayed("a.go") {
		t.Fatal("a.go should still be playing after the first frame")
	}
	if m.filePlayed("c.go") {
		t.Fatal("c.go is not in the script")
	}
	view := m.View()
	if !strings.Contains(view, "▸") {
		t.Fatalf("playhead should show ▸ while typing:\n%s", view)
	}

	updated, _ = m.Update(scheduler.FrameMsg{Time: now.Add(200 * time.Millisecond)})
	m = updated.(Model)
	if !m.sched.Done() {
		t.Fatal("expected both files to finish")
	}
	if !m.filePlayed("a.go") || !m.filePlayed("b.go") {
		t.Fatalf("played a=%v b=%v, want both true", m.filePlayed("a.go"), m.filePlayed("b.go"))
	}
	if m.filePlayed("c.go") {
		t.Fatal("untouched c.go should not be marked")
	}
	view = m.View()
	if !strings.Contains(view, "✓") {
		t.Fatalf("finished files should show ✓:\n%s", view)
	}
	if strings.Contains(view, "▸") {
		t.Fatalf("no playhead arrow after the commit is done:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(Model)
	if m.filePlayed("a.go") || m.filePlayed("b.go") {
		t.Fatal("restart should clear played marks")
	}
}

func TestWithCaret(t *testing.T) {
	t.Parallel()
	if got := withCaret("ab", 1, true); !strings.Contains(got, "a") {
		t.Fatalf("got %q", got)
	}
	if got := withCaret("ab", 1, false); got != "ab" {
		t.Fatalf("blink-off = %q", got)
	}
}

func TestStepSpeedDoubles(t *testing.T) {
	t.Parallel()
	if got := stepSpeed(1, 1); got != 2 {
		t.Fatalf("faster from 1 = %v, want 2", got)
	}
	if got := stepSpeed(1, -1); got != 0.5 {
		t.Fatalf("slower from 1 = %v, want 0.5", got)
	}
	if got := stepSpeed(5, 1); got != 10 {
		t.Fatalf("faster from 5 = %v, want 10 (no cap)", got)
	}
	if got := stepSpeed(0.5, -1); got != 0.25 {
		t.Fatalf("slower from 0.5 = %v, want 0.25 (no floor stop)", got)
	}
}

func TestPlaylistAdvancesOnDone(t *testing.T) {
	t.Parallel()
	opts := scheduler.Options{FPS: 60, MaxStep: time.Second}
	m := NewPlaylist([]Track{
		{
			Script: animator.Script{
				Commit: "aaa1111",
				Actions: []animator.Action{
					{Kind: animator.KindOpenFile, File: "a.go", Delay: time.Millisecond},
					{Kind: animator.KindTypeChar, File: "a.go", Row: 1, Col: 0, Rune: 'a', Delay: time.Millisecond},
				},
			},
			Diff: &gitengine.CommitDiff{
				Commit: gitengine.CommitInfo{ShortHash: "aaa1111"},
				Files:  []gitengine.FileChange{{Path: "a.go", Kind: gitengine.ChangeAdded}},
			},
		},
		{
			Script: animator.Script{
				Commit: "bbb2222",
				Actions: []animator.Action{
					{Kind: animator.KindOpenFile, File: "b.go", Delay: time.Millisecond},
					{Kind: animator.KindTypeChar, File: "b.go", Row: 1, Col: 0, Rune: 'b', Delay: time.Millisecond},
				},
			},
			Diff: &gitengine.CommitDiff{
				Commit: gitengine.CommitInfo{ShortHash: "bbb2222"},
				Files:  []gitengine.FileChange{{Path: "b.go", Kind: gitengine.ChangeAdded}},
			},
		},
	}, opts, "")
	m.width, m.height = 100, 24

	now := time.Unix(0, 0).UTC()
	updated, _ := m.Update(scheduler.FrameMsg{Time: now})
	m = updated.(Model)
	if m.idx != 1 {
		t.Fatalf("after first frame idx=%d, want 1 (InterGap=0)", m.idx)
	}
	if m.hash != "bbb2222" {
		t.Fatalf("hash=%q, want second track", m.hash)
	}
	m.sched.SetSpeed(2)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = updated.(Model)
	if m.idx != 0 {
		t.Fatalf("[ should go to previous commit, idx=%d", m.idx)
	}
	if m.sched.Speed() != 2 {
		t.Fatalf("speed=%v, want 2 preserved across tracks", m.sched.Speed())
	}
	view := m.View()
	if !strings.Contains(view, "1/2") {
		t.Fatalf("view missing playlist index:\n%s", view)
	}
}

func TestMoveTreeThenSpaceReplaysFile(t *testing.T) {
	t.Parallel()
	script := animator.Script{
		Commit: "abc1234",
		Actions: []animator.Action{
			{Kind: animator.KindOpenFile, File: "a.go", Delay: 100 * time.Millisecond},
			{Kind: animator.KindTypeChar, File: "a.go", Row: 1, Col: 0, Rune: 'x', Delay: 100 * time.Millisecond},
			{Kind: animator.KindOpenFile, File: "b.go", Delay: 100 * time.Millisecond},
			{Kind: animator.KindTypeChar, File: "b.go", Row: 1, Col: 0, Rune: 'y', Delay: 100 * time.Millisecond},
		},
	}
	diff := &gitengine.CommitDiff{
		Files: []gitengine.FileChange{
			{Path: "a.go", Kind: gitengine.ChangeAdded, OldContent: "old-a\n"},
			{Path: "b.go", Kind: gitengine.ChangeAdded, OldContent: "old-b\n"},
		},
	}
	m := NewPlayer(script, diff, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	m.width, m.height = 100, 24

	now := time.Unix(0, 0).UTC()
	updated, _ := m.Update(scheduler.FrameMsg{Time: now})
	m = updated.(Model)
	if m.playheadPath() != "a.go" {
		t.Fatalf("playhead=%q, want a.go", m.playheadPath())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)
	if m.sched.Playing() {
		t.Fatal("n should pause playback")
	}
	if m.selected != "b.go" {
		t.Fatalf("selected=%q, want b.go", m.selected)
	}
	view := m.View()
	if !strings.Contains(view, "old-b") {
		t.Fatalf("editor should show selected file OldContent:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if m.browsing {
		t.Fatal("space on another file should start a file replay")
	}
	if !m.fileReplay {
		t.Fatal("expected fileReplay")
	}
	if m.playheadPath() != "b.go" {
		t.Fatalf("playhead=%q, want b.go after file replay", m.playheadPath())
	}
	if !m.sched.Playing() {
		t.Fatal("file replay should be playing")
	}
}

func TestSpaceOnPlayheadResumes(t *testing.T) {
	t.Parallel()
	script := animator.Script{
		Commit: "abc1234",
		Actions: []animator.Action{
			{Kind: animator.KindOpenFile, File: "a.go", Delay: 100 * time.Millisecond},
			{Kind: animator.KindTypeChar, File: "a.go", Row: 1, Col: 0, Rune: 'x', Delay: 100 * time.Millisecond},
		},
	}
	diff := &gitengine.CommitDiff{
		Files: []gitengine.FileChange{
			{Path: "a.go", Kind: gitengine.ChangeAdded, OldContent: "old-a\n"},
			{Path: "b.go", Kind: gitengine.ChangeAdded, OldContent: "old-b\n"},
		},
	}
	m := NewPlayer(script, diff, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	now := time.Unix(0, 0).UTC()
	updated, _ := m.Update(scheduler.FrameMsg{Time: now})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = updated.(Model)
	if m.selected != "a.go" {
		t.Fatalf("selected=%q, want a.go", m.selected)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if m.fileReplay {
		t.Fatal("space on the playhead file should resume, not replay")
	}
	if !m.sched.Playing() {
		t.Fatal("expected resume")
	}
	if m.playheadPath() != "a.go" {
		t.Fatalf("playhead=%q", m.playheadPath())
	}
}

func TestJSlowsPlayback(t *testing.T) {
	t.Parallel()
	m := NewPlayer(animator.Script{
		Actions: []animator.Action{
			{Kind: animator.KindOpenFile, File: "a.go", Delay: time.Second},
		},
	}, nil, scheduler.Options{FPS: 60}, "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.sched.Speed() != 0.5 {
		t.Fatalf("j speed=%v, want 0.5", m.sched.Speed())
	}
}

func TestHOriginKeepsCaretInView(t *testing.T) {
	t.Parallel()
	if got := hOrigin(0, 40); got != 0 {
		t.Fatalf("start of line = %d, want 0", got)
	}
	if got := hOrigin(37, 40); got != 0 {
		t.Fatalf("still in first window = %d, want 0", got)
	}
	if got := hOrigin(38, 40); got != 1 {
		t.Fatalf("first scroll = %d, want 1", got)
	}
	if got := hOrigin(100, 40); got != 63 {
		t.Fatalf("far caret origin = %d, want 63", got)
	}
}

func TestScrollWindowClamps(t *testing.T) {
	t.Parallel()
	from, to := scrollWindow(0, 10, 20)
	if from != 0 || to != 10 {
		t.Fatalf("short file = [%d, %d), want [0, 10)", from, to)
	}
	from, to = scrollWindow(100, 40, 10)
	if from != 30 || to != 40 {
		t.Fatalf("past end = [%d, %d), want [30, 40)", from, to)
	}
	from, to = scrollWindow(-3, 40, 10)
	if from != 0 || to != 10 {
		t.Fatalf("negative off = [%d, %d), want [0, 10)", from, to)
	}
}

func TestEditorScrollsAfterPlayback(t *testing.T) {
	t.Parallel()
	const n = 40
	lines := make([]string, n)
	lines[0] = "LINE_AAA_HEAD"
	for i := 1; i < n-1; i++ {
		lines[i] = "mid"
	}
	lines[n-1] = "LINE_ZZZ_TAIL"
	old := strings.Join(lines, "\n") + "\n"

	m := NewPlayer(animator.Script{
		Commit: "abc1234",
		Actions: []animator.Action{
			{Kind: animator.KindOpenFile, File: "long.go", Delay: time.Millisecond},
			{Kind: animator.KindTypeChar, File: "long.go", Row: n, Col: len(lines[n-1]), Rune: '!', Delay: time.Millisecond},
		},
	}, &gitengine.CommitDiff{
		Files: []gitengine.FileChange{{Path: "long.go", Kind: gitengine.ChangeModified, OldContent: old}},
	}, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	m.width, m.height = 80, 24

	now := time.Unix(0, 0).UTC()
	updated, _ := m.Update(scheduler.FrameMsg{Time: now})
	m = updated.(Model)
	if !m.sched.Done() {
		t.Fatal("expected playback to finish in one catch-up frame")
	}

	view := m.View()
	if !strings.Contains(view, "LINE_ZZZ_TAIL") {
		t.Fatalf("finished playback should keep the caret line in view:\n%s", view)
	}
	if strings.Contains(view, "LINE_AAA_HEAD") {
		t.Fatalf("first line should be scrolled off until the user scrolls:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(Model)
	view = m.View()
	if !strings.Contains(view, "LINE_AAA_HEAD") {
		t.Fatalf("Home should reveal the first line:\n%s", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(Model)
	view = m.View()
	if !strings.Contains(view, "LINE_ZZZ_TAIL") {
		t.Fatalf("End should return to the last line:\n%s", view)
	}

	updated, _ = m.Update(tea.MouseMsg{
		X:      40,
		Y:      10,
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.editOff == m.editorMaxOff() {
		t.Fatal("wheel up should move the editor off the last line")
	}
}

func TestLongLineFollowsCaret(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 90) + "Z"
	actions := []animator.Action{
		{Kind: animator.KindOpenFile, File: "a.go", Delay: time.Millisecond},
		{Kind: animator.KindInsertLine, File: "a.go", Row: 1, Delay: time.Millisecond},
	}
	for i, r := range long {
		actions = append(actions, animator.Action{
			Kind: animator.KindTypeChar, File: "a.go", Row: 1, Col: i, Rune: r, Delay: time.Millisecond,
		})
	}
	m := NewPlayer(animator.Script{Commit: "abc1234", Actions: actions}, &gitengine.CommitDiff{
		Files: []gitengine.FileChange{{Path: "a.go", Kind: gitengine.ChangeAdded}},
	}, scheduler.Options{FPS: 60, MaxStep: time.Second}, "")
	m.width, m.height = 60, 24

	now := time.Unix(0, 0).UTC()
	updated, _ := m.Update(scheduler.FrameMsg{Time: now})
	m = updated.(Model)
	updated, _ = m.Update(scheduler.FrameMsg{Time: now.Add(200 * time.Millisecond)})
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "Z") {
		t.Fatalf("caret tail should stay in view:\n%s", view)
	}
	if !strings.Contains(view, "▌") {
		t.Fatalf("playhead row missing accent:\n%s", view)
	}
}
