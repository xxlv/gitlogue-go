package gitengine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
)

// SnapshotSig fingerprints a worktree patch so listen mode can skip no-ops.
// Nil and empty diffs share the empty signature.
func SnapshotSig(d *CommitDiff) string {
	if d == nil || len(d.Files) == 0 {
		return ""
	}
	h := sha256.New()
	for _, f := range d.Files {
		_, _ = io.WriteString(h, f.Path)
		h.Write([]byte{0})
		_, _ = io.WriteString(h, f.OldPath)
		h.Write([]byte{0})
		fmt.Fprintf(h, "%d", f.Kind)
		h.Write([]byte{0})
		if f.Binary {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
		_, _ = io.WriteString(h, f.OldContent)
		h.Write([]byte{0})
		_, _ = io.WriteString(h, f.NewContent)
		h.Write([]byte{1})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// FileFromContents builds a FileChange for path from old → new blob text.
// An empty old side is an add; an empty new side is a delete.
func FileFromContents(path, oldContent, newContent string) FileChange {
	oldExists := oldContent != ""
	newExists := newContent != ""
	if fc := fileChangeBetween(path, oldContent, oldExists, newContent, newExists); fc != nil {
		return *fc
	}
	return FileChange{
		Path:       path,
		OldPath:    path,
		Kind:       ChangeModified,
		OldContent: oldContent,
		NewContent: newContent,
	}
}

// LiveDelta is the keystroke patch from the on-screen buffers to want.
// live is path → current buffer (only files that exist on the stage).
// orig is path → HEAD content for files that exist in HEAD.
// Files that left want are reverted to orig (or deleted if they were adds).
func LiveDelta(live, orig map[string]string, want *CommitDiff) []FileChange {
	if live == nil {
		live = map[string]string{}
	}
	if orig == nil {
		orig = map[string]string{}
	}

	seen := make(map[string]struct{})
	var out []FileChange
	if want != nil {
		for _, f := range want.Files {
			path := f.DisplayPath()
			if path == "" || f.Binary {
				continue
			}
			seen[path] = struct{}{}

			cur, opened := live[path]
			oldExists := opened
			if !opened {
				if head, ok := orig[path]; ok {
					cur = head
					oldExists = true
				} else {
					cur = f.OldContent
					oldExists = f.Kind != ChangeAdded
				}
			}

			newContent := f.NewContent
			newExists := f.Kind != ChangeDeleted
			if !newExists {
				newContent = ""
			}
			if fc := fileChangeBetween(path, cur, oldExists, newContent, newExists); fc != nil {
				out = append(out, *fc)
			}
		}
	}

	for path, cur := range live {
		if _, ok := seen[path]; ok {
			continue
		}
		head, inHEAD := orig[path]
		if fc := fileChangeBetween(path, cur, true, head, inHEAD); fc != nil {
			out = append(out, *fc)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].DisplayPath() < out[j].DisplayPath()
	})
	return out
}

func fileChangeBetween(path, oldContent string, oldExists bool, newContent string, newExists bool) *FileChange {
	if !oldExists && !newExists {
		return nil
	}
	if oldExists && newExists && oldContent == newContent {
		return nil
	}
	fc := FileChange{
		OldContent: oldContent,
		NewContent: newContent,
		Hunks:      hunksFromTexts(oldContent, newContent),
	}
	switch {
	case !oldExists:
		fc.Kind = ChangeAdded
		fc.Path = path
	case !newExists:
		fc.Kind = ChangeDeleted
		fc.OldPath = path
	default:
		fc.Kind = ChangeModified
		fc.Path = path
		fc.OldPath = path
	}
	return &fc
}
