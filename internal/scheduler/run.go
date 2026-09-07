package scheduler

import (
	"context"
	"time"

	"github.com/xxlv/gitlogue-go/internal/animator"
)

// Run drives the scheduler with a time.Ticker until the script finishes or
// ctx is cancelled. onFrame is called every tick, including idle frames.
func (s *Scheduler) Run(ctx context.Context, onFrame func(due []animator.Action, snap Snapshot)) error {
	if s == nil {
		return nil
	}
	if onFrame == nil {
		onFrame = func([]animator.Action, Snapshot) {}
	}
	if s.Done() {
		onFrame(nil, s.Snapshot())
		return nil
	}

	ticker := time.NewTicker(Interval(s.fps))
	defer ticker.Stop()
	last := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-ticker.C:
			due := s.Advance(now.Sub(last))
			last = now
			onFrame(due, s.Snapshot())
			if s.Done() {
				return nil
			}
		}
	}
}
