package animator

import (
	"math"
	"math/rand/v2"
	"time"
)

// Options controls delay sampling. Zero values mean DefaultOptions.
type Options struct {
	// Seed is the PCG seed. Zero picks time.Now so each run feels alive;
	// tests and --seed pin it for reproducibility.
	Seed int64

	TypeMin  time.Duration // ordinary rune lower bound (default 30ms)
	TypeMax  time.Duration // ordinary rune upper bound (default 60ms)
	PauseMin time.Duration // punctuation / newline lower bound (default 150ms)
	PauseMax time.Duration // punctuation / newline upper bound (default 300ms)
	SeekMin  time.Duration // MoveCursor / OpenFile lower bound (default 40ms)
	SeekMax  time.Duration // MoveCursor / OpenFile upper bound (default 90ms)
}

// DefaultOptions is the cinematic typing model from the project brief.
var DefaultOptions = Options{
	TypeMin:  30 * time.Millisecond,
	TypeMax:  60 * time.Millisecond,
	PauseMin: 150 * time.Millisecond,
	PauseMax: 300 * time.Millisecond,
	SeekMin:  40 * time.Millisecond,
	SeekMax:  90 * time.Millisecond,
}

func (o Options) withDefaults() Options {
	d := DefaultOptions
	if o.Seed != 0 {
		d.Seed = o.Seed
	}
	if o.TypeMin != 0 {
		d.TypeMin = o.TypeMin
	}
	if o.TypeMax != 0 {
		d.TypeMax = o.TypeMax
	}
	if o.PauseMin != 0 {
		d.PauseMin = o.PauseMin
	}
	if o.PauseMax != 0 {
		d.PauseMax = o.PauseMax
	}
	if o.SeekMin != 0 {
		d.SeekMin = o.SeekMin
	}
	if o.SeekMax != 0 {
		d.SeekMax = o.SeekMax
	}
	return d
}

func newRNG(seed int64) (*rand.Rand, int64) {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9e3779b97f4a7c15)), seed
}

func (c *compiler) delayFor(kind Kind, r rune) time.Duration {
	switch kind {
	case KindTypeChar, KindDeleteChar:
		if isPauseRune(r) {
			return gaussian(c.rng, c.opts.PauseMin, c.opts.PauseMax)
		}
		return gaussian(c.rng, c.opts.TypeMin, c.opts.TypeMax)
	case KindInsertLine, KindDeleteLine:
		return gaussian(c.rng, c.opts.PauseMin, c.opts.PauseMax)
	default:
		return gaussian(c.rng, c.opts.SeekMin, c.opts.SeekMax)
	}
}

func isPauseRune(r rune) bool {
	switch r {
	case '.', ',', ';', ':', '!', '?', '\n',
		'。', '，', '；', '：', '！', '？':
		return true
	default:
		return false
	}
}

// gaussian samples N(mean, σ) with σ = (max-min)/6 so ~99.7% of mass sits
// inside [min, max], then clamps the rare tail.
func gaussian(rng *rand.Rand, min, max time.Duration) time.Duration {
	if min > max {
		min, max = max, min
	}
	if min == max {
		return min
	}
	mean := float64(min+max) / 2
	std := float64(max-min) / 6
	v := rng.NormFloat64()*std + mean
	v = math.Min(float64(max), math.Max(float64(min), v))
	return time.Duration(v)
}
