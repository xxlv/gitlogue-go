// Package scheduler is the 60 FPS playhead that drives the replay.
//
// It consumes an animator.Script and never re-parses hunks. Advance(dt) is
// the pure core (unit-tested with synthetic deltas); Tick() wraps tea.Tick
// and Run() wraps time.Ticker for headless playback.
package scheduler

import (
	"math"
	"time"

	"github.com/xxlv/gitlogue-go/internal/animator"
)

const (
	// DefaultFPS is the cinematic target frame rate.
	DefaultFPS = 60
	// DefaultMaxStep caps a single Advance so a blocked UI cannot dump
	// seconds of keystrokes in one frame.
	DefaultMaxStep = 100 * time.Millisecond
)

// Options configure a Scheduler. Zero values mean the defaults above.
type Options struct {
	FPS     int
	Speed   float64
	MaxStep time.Duration
}

func (o Options) withDefaults() Options {
	if o.FPS <= 0 {
		o.FPS = DefaultFPS
	}
	if o.Speed <= 0 {
		o.Speed = 1
	}
	if o.MaxStep <= 0 {
		o.MaxStep = DefaultMaxStep
	}
	return o
}

// Interval is the nominal duration of one frame at fps.
func Interval(fps int) time.Duration {
	if fps <= 0 {
		fps = DefaultFPS
	}
	return time.Second / time.Duration(fps)
}

// Snapshot is a read-only view of the playhead for the status bar.
type Snapshot struct {
	Fired    int
	Total    int
	Elapsed  time.Duration
	Duration time.Duration
	Playing  bool
	Done     bool
	Speed    float64
	FPS      int
}

// Progress is Elapsed/Duration clamped to [0, 1].
func (s Snapshot) Progress() float64 {
	if s.Duration <= 0 {
		if s.Done {
			return 1
		}
		return 0
	}
	p := float64(s.Elapsed) / float64(s.Duration)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// Scheduler advances a script against wall-clock (or synthetic) time.
// It is not safe for concurrent use; Bubble Tea's single-threaded Update
// is the intended caller.
type Scheduler struct {
	script  animator.Script
	fps     int
	speed   float64
	maxStep time.Duration

	index   int
	elapsed time.Duration
	nextDue time.Duration
	playing bool
	done    bool
}

// New binds a compiled script. An empty script is already Done.
func New(script animator.Script, opts Options) *Scheduler {
	opts = opts.withDefaults()
	s := &Scheduler{
		script:  script,
		fps:     opts.FPS,
		speed:   opts.Speed,
		maxStep: opts.MaxStep,
		playing: true,
	}
	if len(script.Actions) == 0 {
		s.done = true
		s.playing = false
	}
	return s
}

// Advance moves the playhead forward by dt (scaled by Speed) and returns
// every Action that became due. The first action fires at t=0; each action's
// Delay is the wait *after* it before the next one. Catch-up is allowed
// within a single call so a slow frame does not drift the script.
func (s *Scheduler) Advance(dt time.Duration) []animator.Action {
	if s == nil || !s.playing || s.done {
		return nil
	}
	if dt < 0 {
		dt = 0
	}
	if dt > s.maxStep {
		dt = s.maxStep
	}
	s.elapsed += scale(dt, s.speed)

	var fired []animator.Action
	for s.index < len(s.script.Actions) && s.elapsed >= s.nextDue {
		a := s.script.Actions[s.index]
		fired = append(fired, a)
		s.nextDue += a.Delay
		s.index++
	}
	if s.index >= len(s.script.Actions) && s.elapsed >= s.nextDue {
		s.done = true
		s.playing = false
	}
	return fired
}

// Play resumes playback. A finished script is left finished (call Reset).
func (s *Scheduler) Play() {
	if s == nil || s.done {
		return
	}
	s.playing = true
}

// Pause freezes the playhead. Subsequent Advance calls return nil.
func (s *Scheduler) Pause() {
	if s == nil {
		return
	}
	s.playing = false
}

// Toggle pauses a running script or resumes a paused one.
func (s *Scheduler) Toggle() {
	if s == nil || s.done {
		return
	}
	s.playing = !s.playing
}

// SetSpeed sets the playback multiplier. Non-positive, NaN, and Inf are
// ignored so the playhead cannot freeze or explode; there is otherwise
// no cap — the user may pass any --speed and keep tapping j/k.
func (s *Scheduler) SetSpeed(speed float64) {
	if s == nil || speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		return
	}
	s.speed = speed
}

// Speed is the current playback multiplier.
func (s *Scheduler) Speed() float64 {
	if s == nil {
		return 1
	}
	return s.speed
}

// Reset rewinds to t=0 without firing actions. The UI must also Reset the Stage.
func (s *Scheduler) Reset() {
	if s == nil {
		return
	}
	s.index = 0
	s.elapsed = 0
	s.nextDue = 0
	s.done = len(s.script.Actions) == 0
	s.playing = !s.done
}

// Done reports that every action has fired and the trailing delay has elapsed.
func (s *Scheduler) Done() bool {
	return s != nil && s.done
}

// Playing reports whether Advance will emit actions.
func (s *Scheduler) Playing() bool {
	return s != nil && s.playing
}

// Snapshot copies playhead state for the status bar.
func (s *Scheduler) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	return Snapshot{
		Fired:    s.index,
		Total:    len(s.script.Actions),
		Elapsed:  s.elapsed,
		Duration: s.script.Duration(),
		Playing:  s.playing,
		Done:     s.done,
		Speed:    s.speed,
		FPS:      s.fps,
	}
}

func scale(dt time.Duration, speed float64) time.Duration {
	if speed == 1 {
		return dt
	}
	return time.Duration(float64(dt) * speed)
}
