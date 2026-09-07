package ui

import (
	"time"

	"github.com/xxlv/gitlogue-go/internal/animator"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
	"github.com/xxlv/gitlogue-go/internal/highlighter"
	"github.com/xxlv/gitlogue-go/internal/scheduler"
)

// DefaultInterGap is the pause between playlist tracks during --play.
// Tests leave InterGap at 0 so the next commit starts in the same frame.
const DefaultInterGap = 500 * time.Millisecond

// Track is one commit in a playlist: the patch plus its compiled keystrokes.
type Track struct {
	Diff   *gitengine.CommitDiff
	Script animator.Script
}

// NewPlaylist plays tracks oldest-first. Speed is preserved across commits.
// InterGap defaults to 0 (same-frame advance); the CLI sets DefaultInterGap.
func NewPlaylist(tracks []Track, opts scheduler.Options, theme string) Model {
	if opts.FPS <= 0 {
		opts.FPS = scheduler.DefaultFPS
	}
	m := Model{
		tracks: tracks,
		opts:   opts,
		hl:     highlighter.NewPool(theme),
		fps:    opts.FPS,
	}
	if len(tracks) == 0 {
		m.sched = scheduler.New(animator.Script{}, opts)
		m.stage = animator.NewStage(nil)
		return m
	}
	m.install(0)
	return m
}

func (m *Model) install(i int) {
	if i < 0 || i >= len(m.tracks) {
		return
	}
	speed := m.opts.Speed
	if m.sched != nil {
		speed = m.sched.Speed()
	}
	t := m.tracks[i]
	m.idx = i
	opts := m.opts
	if speed > 0 {
		opts.Speed = speed
	}
	m.sched = scheduler.New(t.Script, opts)
	m.stage = stageFromDiff(t.Diff)
	m.files = nil
	m.hash = t.Script.Commit
	m.commit = gitengine.CommitInfo{}
	if t.Diff != nil {
		m.files = t.Diff.Files
		m.commit = t.Diff.Commit
		if m.hash == "" {
			m.hash = t.Diff.Commit.ShortHash
		}
	}
	m.browsing = false
	m.selected = ""
	m.between = false
	m.gapLeft = 0
	m.fileReplay = false
	m.editOff = 0
	m.played = make(map[string]struct{})
}

func (m *Model) applyDue(due []animator.Action) {
	for _, a := range due {
		prev := m.playheadPath()
		m.stage.Apply(a)
		if next := m.playheadPath(); prev != "" && next != "" && prev != next {
			m.markPlayed(prev)
		}
	}
	if !m.browsing {
		if buf := m.stage.Current(); buf != nil {
			m.selected = buf.Path()
		}
	}
	if m.sched != nil && m.sched.Done() {
		m.markPlayed(m.playheadPath())
	}
}

func (m *Model) markPlayed(path string) {
	if path == "" {
		return
	}
	if m.played == nil {
		m.played = make(map[string]struct{})
	}
	m.played[path] = struct{}{}
}

func (m Model) filePlayed(path string) bool {
	_, ok := m.played[path]
	return ok
}

func (m *Model) tickPlayback(dt time.Duration) {
	if m.sched == nil {
		return
	}
	if m.between {
		m.gapLeft -= dt
		if m.gapLeft > 0 {
			return
		}
		m.between = false
		if m.idx+1 < len(m.tracks) {
			m.install(m.idx + 1)
			m.applyDue(m.sched.Advance(0))
		}
		return
	}
	m.applyDue(m.sched.Advance(dt))
	if m.fileReplay {
		return
	}
	if m.sched.Done() && m.idx+1 < len(m.tracks) {
		if m.InterGap <= 0 {
			m.install(m.idx + 1)
			m.applyDue(m.sched.Advance(0))
			return
		}
		m.between = true
		m.gapLeft = m.InterGap
	}
}

func (m *Model) nextTrack() {
	if m.idx+1 >= len(m.tracks) {
		return
	}
	m.install(m.idx + 1)
	m.applyDue(m.sched.Advance(0))
}

func (m *Model) prevTrack() {
	if m.idx <= 0 {
		return
	}
	m.install(m.idx - 1)
	m.applyDue(m.sched.Advance(0))
}

func (m *Model) restart() {
	m.fileReplay = false
	if len(m.tracks) == 0 {
		if m.sched != nil {
			m.sched.Reset()
		}
		if m.stage != nil {
			m.stage.Reset()
		}
		m.browsing = false
		m.selected = ""
		m.editOff = 0
		m.played = make(map[string]struct{})
		return
	}
	m.install(m.idx)
	m.applyDue(m.sched.Advance(0))
}

func (m *Model) moveTree(delta int) {
	paths := filePaths(m.files)
	if len(paths) == 0 {
		return
	}
	if m.sched != nil && m.sched.Playing() {
		m.sched.Pause()
	}
	m.browsing = true
	idx := 0
	if m.selected != "" {
		for i, p := range paths {
			if p == m.selected {
				idx = i
				break
			}
		}
	} else if buf := m.stage.Current(); buf != nil {
		for i, p := range paths {
			if p == buf.Path() {
				idx = i
				break
			}
		}
	}
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(paths) {
		idx = len(paths) - 1
	}
	if paths[idx] != m.selected {
		m.editOff = 0
	}
	m.selected = paths[idx]
}

func (m *Model) onSpace() {
	if m.browsing && m.selected != "" && m.selected != m.playheadPath() {
		m.replayFile(m.selected)
		return
	}
	if m.browsing {
		m.stopBrowsing()
		return
	}
	if m.sched == nil {
		return
	}
	if m.sched.Done() {
		if m.fileReplay && m.selected != "" {
			m.replayFile(m.selected)
			return
		}
		m.restart()
		return
	}
	m.sched.Toggle()
}

func (m *Model) onEnter() {
	path := m.selected
	if path == "" {
		path = m.playheadPath()
	}
	if path != "" {
		m.replayFile(path)
	}
}

func (m *Model) nudgeSpeed(dir int) {
	if m.sched == nil {
		return
	}
	m.sched.SetSpeed(stepSpeed(m.sched.Speed(), dir))
}

func (m *Model) replayFile(path string) {
	if path == "" {
		return
	}
	src := animator.Script{}
	if m.idx >= 0 && m.idx < len(m.tracks) {
		src = m.tracks[m.idx].Script
	}
	speed := m.opts.Speed
	if m.sched != nil {
		speed = m.sched.Speed()
	}
	opts := m.opts
	if speed > 0 {
		opts.Speed = speed
	}
	if m.stage != nil {
		m.stage.ResetFile(path)
	}
	m.sched = scheduler.New(src.ForFile(path), opts)
	m.browsing = false
	m.selected = path
	m.fileReplay = true
	m.editOff = 0
	if m.played != nil {
		delete(m.played, path)
	}
	m.between = false
	m.gapLeft = 0
	m.applyDue(m.sched.Advance(0))
}

func (m *Model) stopBrowsing() {
	m.browsing = false
	if buf := m.stage.Current(); buf != nil {
		m.selected = buf.Path()
	}
	if m.sched != nil && !m.sched.Done() {
		m.sched.Play()
	}
}

func filePaths(files []gitengine.FileChange) []string {
	rows := treeRows(files)
	out := make([]string, 0, len(files))
	for _, r := range rows {
		if !r.IsDir {
			out = append(out, r.Path)
		}
	}
	return out
}

func (m Model) playlistProgress() float64 {
	var total, done time.Duration
	for i, t := range m.tracks {
		d := t.Script.Duration()
		total += d
		if i < m.idx {
			done += d
		}
	}
	if m.between && m.idx < len(m.tracks) {
		done += m.tracks[m.idx].Script.Duration()
	} else if m.sched != nil {
		done += m.sched.Snapshot().Elapsed
	}
	if total <= 0 {
		if m.sched != nil && m.sched.Done() && m.idx+1 >= len(m.tracks) {
			return 1
		}
		return 0
	}
	p := float64(done) / float64(total)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func (m Model) playheadPath() string {
	if m.stage == nil {
		return ""
	}
	if buf := m.stage.Current(); buf != nil {
		return buf.Path()
	}
	return ""
}

func (m Model) editorBuffer() *animator.Buffer {
	if m.stage == nil {
		return nil
	}
	if m.selected != "" {
		if buf := m.stage.File(m.selected); buf != nil {
			return buf
		}
	}
	return m.stage.Current()
}
