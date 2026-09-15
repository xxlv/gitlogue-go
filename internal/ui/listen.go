package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/xxlv/gitlogue-go/internal/animator"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
	"github.com/xxlv/gitlogue-go/internal/scheduler"
	"github.com/xxlv/gitlogue-go/internal/watch"
)

// EnableListen keeps the TUI open and auto-plays new worktree edits.
func (m *Model) EnableListen(cmd func(string) tea.Cmd, sig string) {
	if m == nil {
		return
	}
	m.listen = true
	m.listenCmd = cmd
	m.listenSig = sig
	if m.hash == "" {
		m.hash = gitengine.WorktreeHash
	}
	if m.commit.ShortHash == "" {
		m.commit.Hash = gitengine.WorktreeHash
		m.commit.ShortHash = gitengine.WorktreeHash
		m.commit.Message = "listening · worktree clean\n"
	}
}

func (m *Model) rescheduleListen() tea.Cmd {
	if m == nil || m.listenCmd == nil {
		return nil
	}
	return m.listenCmd(m.listenSig)
}

func (m *Model) applyListen(msg watch.Msg) tea.Cmd {
	if msg.Err != nil {
		m.listenErr = msg.Err.Error()
		return m.rescheduleListen()
	}
	m.listenErr = ""
	if msg.Sig == m.listenSig {
		return m.rescheduleListen()
	}
	m.listenSig = msg.Sig
	m.replaceWIP(msg.Diff)
	return m.rescheduleListen()
}

func (m *Model) replaceWIP(diff *gitengine.CommitDiff) {
	if m.stage == nil {
		m.stage = animator.NewStage(nil)
	}
	if diff == nil {
		diff = &gitengine.CommitDiff{
			Commit: gitengine.CommitInfo{
				Hash:      gitengine.WorktreeHash,
				ShortHash: gitengine.WorktreeHash,
				Message:   "listening · worktree clean\n",
			},
		}
	}

	for _, f := range diff.Files {
		path := f.DisplayPath()
		if path == "" || f.Kind == gitengine.ChangeAdded {
			continue
		}
		m.stage.EnsureOrig(path, f.OldContent)
	}

	delta := gitengine.LiveDelta(m.stage.LiveContents(), m.stage.OrigContents(), diff)
	m.installWIPTrack(diff)

	if len(delta) == 0 {
		return
	}

	speed := m.opts.Speed
	if m.sched != nil {
		speed = m.sched.Speed()
	}
	opts := m.opts
	if speed > 0 {
		opts.Speed = speed
	}
	inc := &gitengine.CommitDiff{Commit: diff.Commit, Parent: diff.Parent, Files: delta}
	m.sched = scheduler.New(animator.Compile(inc, animator.Options{Seed: m.Seed}), opts)
	m.browsing = false
	m.inspecting = false
	m.fileReplay = true
	m.between = false
	m.gapLeft = 0
	m.finalView = false
	for _, f := range delta {
		if m.played != nil {
			delete(m.played, f.DisplayPath())
		}
	}
	m.applyDue(m.sched.Advance(0))
}

func (m *Model) installWIPTrack(diff *gitengine.CommitDiff) {
	if diff == nil || len(diff.Files) == 0 {
		m.tracks = nil
		m.files = nil
		m.idx = 0
		m.hash = gitengine.WorktreeHash
		m.commit = gitengine.CommitInfo{
			Hash:      gitengine.WorktreeHash,
			ShortHash: gitengine.WorktreeHash,
			Message:   "listening · worktree clean\n",
		}
		if diff != nil && diff.Commit.Message != "" {
			m.commit = diff.Commit
			if m.commit.ShortHash == "" {
				m.commit.ShortHash = gitengine.WorktreeHash
				m.commit.Hash = gitengine.WorktreeHash
			}
		}
		return
	}
	m.tracks = []Track{{
		Diff:   diff,
		Script: animator.Compile(diff, animator.Options{Seed: m.Seed}),
	}}
	m.idx = 0
	m.files = diff.Files
	m.commit = diff.Commit
	m.hash = gitengine.WorktreeHash
}
