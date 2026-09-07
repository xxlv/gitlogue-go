package gitengine

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// SplitRev parses a Git revision the way users type it:
//
//	tip            -> ("", tip, 0)
//	A..B           -> (A, B, 2)   commits reachable from B, not from A
//	A...B          -> (A, B, 3)   first-parent of B until merge-base(A,B)
//
// An empty left side means HEAD (`..feature` → HEAD..feature).
func SplitRev(spec string) (left, right string, dots int) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "HEAD", 0
	}
	if i := strings.Index(spec, "..."); i >= 0 {
		return spec[:i], spec[i+3:], 3
	}
	if i := strings.Index(spec, ".."); i >= 0 {
		return spec[:i], spec[i+2:], 2
	}
	return "", spec, 0
}

// RevSpec is the CLI entry: a single revision (History) or A..B / A...B.
// n is a cap on how many newest commits to keep. For a single revision,
// n < 1 means 1. For a range, n < 1 means the whole range.
func (e *Engine) RevSpec(spec string, n int) ([]*CommitDiff, error) {
	left, right, dots := SplitRev(spec)
	if right == "" {
		return nil, fmt.Errorf("invalid revision %q", spec)
	}
	if dots == 0 {
		return e.History(right, n)
	}
	if left == "" {
		left = "HEAD"
	}
	return e.rangeFirstParent(left, right, dots == 3, n)
}

func (e *Engine) rangeFirstParent(from, to string, threeDot bool, n int) ([]*CommitDiff, error) {
	if e == nil || e.repo == nil {
		return nil, fmt.Errorf("gitengine: engine is not open")
	}
	toC, err := e.resolveCommit(to)
	if err != nil {
		return nil, err
	}
	fromC, err := e.resolveCommit(from)
	if err != nil {
		return nil, err
	}
	stopAt := fromC
	if threeDot {
		bases, err := fromC.MergeBase(toC)
		if err != nil {
			return nil, fmt.Errorf("merge-base %s %s: %w", from, to, err)
		}
		if len(bases) > 0 {
			stopAt = bases[0]
		}
	}

	stop, err := firstParentSet(stopAt)
	if err != nil {
		return nil, err
	}

	newestFirst := make([]string, 0)
	c := toC
	for c != nil {
		h := c.Hash.String()
		if _, ok := stop[h]; ok {
			break
		}
		newestFirst = append(newestFirst, h)
		if c.NumParents() == 0 {
			break
		}
		c, err = c.Parent(0)
		if err != nil {
			return nil, fmt.Errorf("walk first parent of %s: %w", h, err)
		}
	}
	if n > 0 && len(newestFirst) > n {
		newestFirst = newestFirst[:n]
	}
	if len(newestFirst) == 0 {
		return nil, fmt.Errorf("no commits in %s..%s", from, to)
	}

	out := make([]*CommitDiff, 0, len(newestFirst))
	for i := len(newestFirst) - 1; i >= 0; i-- {
		d, err := e.Inspect(newestFirst[i])
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func firstParentSet(tip *object.Commit) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	c := tip
	for c != nil {
		out[c.Hash.String()] = struct{}{}
		if c.NumParents() == 0 {
			return out, nil
		}
		parent, err := c.Parent(0)
		if err != nil {
			return nil, fmt.Errorf("walk first parent of %s: %w", c.Hash.String(), err)
		}
		c = parent
	}
	return out, nil
}
