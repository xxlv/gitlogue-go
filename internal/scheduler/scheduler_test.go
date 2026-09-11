package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/xxlv/gitlogue-go/internal/animator"
)

func TestAdvanceFiresFirstActionAtZero(t *testing.T) {
	t.Parallel()
	s := New(script(
		act(10*time.Millisecond),
		act(20*time.Millisecond),
	), Options{FPS: 60})

	got := s.Advance(0)
	if len(got) != 1 {
		t.Fatalf("t=0 fired %d, want 1", len(got))
	}
	if s.Done() {
		t.Fatal("done too early")
	}
}

func TestAdvanceWaitsForDelay(t *testing.T) {
	t.Parallel()
	s := New(script(
		act(10*time.Millisecond),
		act(20*time.Millisecond),
	), Options{})

	s.Advance(0) // fire first
	got := s.Advance(9 * time.Millisecond)
	if len(got) != 0 {
		t.Fatalf("t=9ms fired %d, want 0", len(got))
	}
	got = s.Advance(1 * time.Millisecond)
	if len(got) != 1 {
		t.Fatalf("t=10ms fired %d, want 1", len(got))
	}
}

func TestAdvanceCatchUp(t *testing.T) {
	t.Parallel()
	s := New(script(
		act(10*time.Millisecond),
		act(10*time.Millisecond),
		act(10*time.Millisecond),
	), Options{MaxStep: time.Second})

	got := s.Advance(25 * time.Millisecond)
	if len(got) != 3 {
		t.Fatalf("catch-up fired %d, want 3", len(got))
	}
	if s.Done() {
		t.Fatal("trailing delay of last action should keep us unfinished")
	}
	got = s.Advance(10 * time.Millisecond)
	if len(got) != 0 {
		t.Fatalf("after last fire, extra dt should not emit, got %d", len(got))
	}
	if !s.Done() {
		t.Fatal("want done after trailing delay")
	}
}

func TestPauseAndResume(t *testing.T) {
	t.Parallel()
	s := New(script(act(10*time.Millisecond), act(10*time.Millisecond)), Options{})
	s.Advance(0)
	s.Pause()
	if got := s.Advance(50 * time.Millisecond); len(got) != 0 {
		t.Fatalf("paused scheduler fired %d", len(got))
	}
	s.Play()
	if got := s.Advance(10 * time.Millisecond); len(got) != 1 {
		t.Fatalf("resume fired %d, want 1", len(got))
	}
}

func TestSpeedMultiplier(t *testing.T) {
	t.Parallel()
	s := New(script(act(10*time.Millisecond), act(10*time.Millisecond)), Options{Speed: 2})
	s.Advance(0)
	got := s.Advance(5 * time.Millisecond) // 5ms * 2 = 10ms of script time
	if len(got) != 1 {
		t.Fatalf("2× speed fired %d, want 1", len(got))
	}
}

func TestSetSpeedUncapped(t *testing.T) {
	t.Parallel()
	s := New(script(act(time.Millisecond)), Options{})
	s.SetSpeed(32)
	if s.Speed() != 32 {
		t.Fatalf("speed=%v, want 32", s.Speed())
	}
	s.SetSpeed(0)
	if s.Speed() != 32 {
		t.Fatalf("non-positive should be ignored, speed=%v", s.Speed())
	}
}

func TestEmptyScriptIsDone(t *testing.T) {
	t.Parallel()
	s := New(animator.Script{}, Options{})
	if !s.Done() {
		t.Fatal("empty script should be done")
	}
	if got := s.Advance(time.Second); len(got) != 0 {
		t.Fatalf("empty script fired %d", len(got))
	}
}

func TestMaxStepCapsCatchUp(t *testing.T) {
	t.Parallel()
	s := New(script(act(10*time.Millisecond), act(10*time.Millisecond)), Options{MaxStep: 5 * time.Millisecond})
	s.Advance(0)
	got := s.Advance(time.Second) // clamped to 5ms, next due at 10ms
	if len(got) != 0 {
		t.Fatalf("capped step fired %d, want 0", len(got))
	}
}

func TestSnapshotProgress(t *testing.T) {
	t.Parallel()
	s := New(script(act(10*time.Millisecond), act(10*time.Millisecond)), Options{})
	s.Advance(0)
	s.Advance(10 * time.Millisecond)
	snap := s.Snapshot()
	if snap.Fired != 2 || snap.Total != 2 {
		t.Fatalf("snapshot fired=%d total=%d", snap.Fired, snap.Total)
	}
	if snap.Progress() < 0.4 || snap.Progress() > 0.6 {
		t.Fatalf("progress=%v, want ~0.5", snap.Progress())
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	s := New(script(act(1*time.Millisecond)), Options{MaxStep: time.Second})
	s.Advance(time.Second)
	if !s.Done() {
		t.Fatal("want done")
	}
	s.Reset()
	if s.Done() || !s.Playing() {
		t.Fatal("reset should rewind")
	}
	if got := s.Advance(0); len(got) != 1 {
		t.Fatalf("after reset fired %d, want 1", len(got))
	}
}

func TestTickCmdIsNonNil(t *testing.T) {
	t.Parallel()
	if Tick(60) == nil {
		t.Fatal("Tick returned nil cmd")
	}
}

func TestRunEmptyReturnsImmediately(t *testing.T) {
	t.Parallel()
	s := New(animator.Script{}, Options{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Run(ctx, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestDrainFiresRemaining(t *testing.T) {
	t.Parallel()
	s := New(script(
		act(10*time.Millisecond),
		act(10*time.Millisecond),
		act(10*time.Millisecond),
	), Options{MaxStep: time.Millisecond})
	if n := len(s.Advance(0)); n != 1 {
		t.Fatalf("first fire = %d, want 1", n)
	}
	got := s.Drain()
	if len(got) != 2 {
		t.Fatalf("Drain fired %d, want 2", len(got))
	}
	if !s.Done() || s.Playing() {
		t.Fatalf("done=%v playing=%v", s.Done(), s.Playing())
	}
	if n := len(s.Drain()); n != 0 {
		t.Fatalf("second Drain fired %d", n)
	}
}

func TestPauseAndFinishStopsPlayback(t *testing.T) {
	t.Parallel()
	s := New(script(act(10*time.Millisecond), act(10*time.Millisecond)), Options{})
	s.PauseAndFinish()
	if !s.Done() || s.Playing() {
		t.Fatalf("done=%v playing=%v", s.Done(), s.Playing())
	}
	if n := len(s.Advance(time.Second)); n != 0 {
		t.Fatalf("Advance after PauseAndFinish fired %d", n)
	}
}

func TestInterval(t *testing.T) {
	t.Parallel()
	if Interval(60) != time.Second/60 {
		t.Fatalf("Interval(60)=%s", Interval(60))
	}
	if Interval(0) != time.Second/60 {
		t.Fatalf("Interval(0) should default to 60fps")
	}
}

func script(actions ...animator.Action) animator.Script {
	return animator.Script{Actions: actions}
}

func act(d time.Duration) animator.Action {
	return animator.Action{Kind: animator.KindTypeChar, Rune: 'x', Delay: d, File: "a.txt"}
}
