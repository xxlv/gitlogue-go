package gitengine

import (
	"fmt"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

func diffTrees(from, to *object.Tree) ([]FileChange, error) {
	changes, err := object.DiffTree(from, to)
	if err != nil {
		return nil, err
	}

	// Best-effort rename detection; if the helper is unavailable or fails
	// we still return a usable insert/delete/modify list.
	if renamed, rerr := object.DetectRenames(changes, nil); rerr == nil {
		changes = renamed
	}

	out := make([]FileChange, 0, len(changes))
	for _, ch := range changes {
		fc, err := fileChangeFrom(ch)
		if err != nil {
			return nil, err
		}
		out = append(out, fc)
	}
	return out, nil
}

func fileChangeFrom(ch *object.Change) (FileChange, error) {
	action, err := ch.Action()
	if err != nil {
		return FileChange{}, fmt.Errorf("change action: %w", err)
	}

	fc := FileChange{
		OldPath: ch.From.Name,
		Path:    ch.To.Name,
		Kind:    kindFrom(action, ch.From.Name, ch.To.Name),
	}

	from, to, err := ch.Files()
	if err != nil {
		return FileChange{}, fmt.Errorf("files %s: %w", fc.DisplayPath(), err)
	}
	if fc.OldContent, fc.Binary, err = blobText(from); err != nil {
		return FileChange{}, fmt.Errorf("read old blob %s: %w", fc.OldPath, err)
	}
	if !fc.Binary {
		var toBin bool
		if fc.NewContent, toBin, err = blobText(to); err != nil {
			return FileChange{}, fmt.Errorf("read new blob %s: %w", fc.Path, err)
		}
		fc.Binary = toBin
	}
	if fc.Binary {
		fc.OldContent = ""
		fc.NewContent = ""
	}

	patch, err := ch.Patch()
	if err != nil {
		return FileChange{}, fmt.Errorf("patch %s: %w", fc.DisplayPath(), err)
	}

	for _, fp := range patch.FilePatches() {
		if fp.IsBinary() {
			fc.Binary = true
			continue
		}
		fc.Hunks = append(fc.Hunks, hunksFromFilePatch(fp)...)
	}
	if fc.Binary {
		fc.OldContent = ""
		fc.NewContent = ""
		fc.Hunks = nil
	}
	return fc, nil
}

func blobText(f *object.File) (string, bool, error) {
	if f == nil {
		return "", false, nil
	}
	bin, err := f.IsBinary()
	if err != nil {
		return "", false, err
	}
	if bin {
		return "", true, nil
	}
	text, err := f.Contents()
	if err != nil {
		return "", false, err
	}
	return text, false, nil
}

func kindFrom(action merkletrie.Action, oldPath, newPath string) ChangeKind {
	switch action {
	case merkletrie.Insert:
		return ChangeAdded
	case merkletrie.Delete:
		return ChangeDeleted
	default:
		if oldPath != "" && newPath != "" && oldPath != newPath {
			return ChangeRenamed
		}
		return ChangeModified
	}
}
