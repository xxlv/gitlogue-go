package gitengine

import "fmt"

// History walks n first-parent commits ending at tip (default HEAD) and
// returns their patches oldest-first, so a playlist can play history in
// chronological order. n < 1 is treated as 1. If the chain is shorter than
// n, the result is truncated to what exists.
func (e *Engine) History(tip string, n int) ([]*CommitDiff, error) {
	if e == nil || e.repo == nil {
		return nil, fmt.Errorf("gitengine: engine is not open")
	}
	if n < 1 {
		n = 1
	}
	if tip == "" {
		tip = "HEAD"
	}

	c, err := e.resolveCommit(tip)
	if err != nil {
		return nil, err
	}

	newestFirst := make([]string, 0, n)
	for i := 0; i < n && c != nil; i++ {
		newestFirst = append(newestFirst, c.Hash.String())
		if c.NumParents() == 0 {
			break
		}
		c, err = c.Parent(0)
		if err != nil {
			return nil, fmt.Errorf("walk first parent of %s: %w", newestFirst[len(newestFirst)-1], err)
		}
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
