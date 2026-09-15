// Package watch polls a Git worktree so --wip --listen can auto-play new edits.
package watch

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xxlv/gitlogue-go/internal/gitengine"
)

// Interval is how often listen mode re-reads `git diff HEAD`.
const Interval = 200 * time.Millisecond

// Msg is a new worktree snapshot. An empty Sig means a clean tree.
type Msg struct {
	Diff *gitengine.CommitDiff
	Sig  string
	Err  error
}

// Snapshot reads the current uncommitted patch (optionally --file filtered).
func Snapshot(engine *gitengine.Engine, against, filter string) (*gitengine.CommitDiff, string, error) {
	if engine == nil {
		return nil, "", nil
	}
	d, err := engine.WorkingTree(against)
	if err != nil {
		return nil, "", err
	}
	d = gitengine.FilterDiff(d, filter)
	return d, gitengine.SnapshotSig(d), nil
}

// Listen returns a Cmd that blocks until the worktree signature changes.
func Listen(engine *gitengine.Engine, against, filter, sig string) tea.Cmd {
	return ListenEvery(engine, against, filter, sig, Interval)
}

// ListenEvery is Listen with a custom poll interval (tests use a short tick).
func ListenEvery(engine *gitengine.Engine, against, filter, sig string, every time.Duration) tea.Cmd {
	if every <= 0 {
		every = Interval
	}
	return func() tea.Msg {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			d, next, err := Snapshot(engine, against, filter)
			if err != nil {
				return Msg{Err: err, Sig: sig}
			}
			if next != sig {
				return Msg{Diff: d, Sig: next}
			}
			<-ticker.C
		}
	}
}
