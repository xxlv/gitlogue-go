package gitengine

import (
	"path/filepath"
	"strings"
)

// MatchFile reports whether a file change matches a --file pattern.
// Empty pattern matches everything. A pattern may be a full path, a
// basename, or a glob (filepath.Match).
func MatchFile(f FileChange, pattern string) bool {
	if pattern == "" {
		return true
	}
	p := filepath.ToSlash(f.DisplayPath())
	p = strings.TrimPrefix(p, "./")
	if p == pattern || filepath.Base(p) == pattern {
		return true
	}
	if ok, _ := filepath.Match(pattern, p); ok {
		return true
	}
	if ok, _ := filepath.Match(pattern, filepath.Base(p)); ok {
		return true
	}
	return false
}

// FilterDiff copies d keeping only files that match pattern. The original
// is not modified. A nil or empty-pattern input is returned as-is.
func FilterDiff(d *CommitDiff, pattern string) *CommitDiff {
	if d == nil || pattern == "" {
		return d
	}
	files := make([]FileChange, 0, len(d.Files))
	for _, f := range d.Files {
		if MatchFile(f, pattern) {
			files = append(files, f)
		}
	}
	out := *d
	out.Files = files
	return &out
}

// FilterDiffs drops commits that have no matching files after FilterDiff.
func FilterDiffs(diffs []*CommitDiff, pattern string) []*CommitDiff {
	if pattern == "" {
		return diffs
	}
	out := make([]*CommitDiff, 0, len(diffs))
	for _, d := range diffs {
		f := FilterDiff(d, pattern)
		if f != nil && len(f.Files) > 0 {
			out = append(out, f)
		}
	}
	return out
}
