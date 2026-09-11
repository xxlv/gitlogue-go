package gitengine

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// WorktreeHash is the synthetic commit id for an uncommitted patch.
const WorktreeHash = "WIP"

const binarySniffLen = 8000

// WorkingTree returns a synthetic CommitDiff for `git diff <against>`:
// staged and unstaged changes against that commit (default HEAD).
// Untracked files are omitted, matching git diff. A clean worktree
// yields a diff with no files (not an error).
func (e *Engine) WorkingTree(against string) (*CommitDiff, error) {
	if e == nil || e.repo == nil {
		return nil, fmt.Errorf("gitengine: engine is not open")
	}
	if against == "" {
		against = "HEAD"
	}

	commit, err := e.resolveCommit(against)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("read tree of %s: %w", commit.Hash.String(), err)
	}

	wt, err := e.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("open worktree: %w", err)
	}
	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("worktree status: %w", err)
	}

	paths := make([]string, 0, len(status))
	for path, st := range status {
		if isUntracked(st) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)

	files := make([]FileChange, 0, len(paths))
	for _, path := range paths {
		fc, err := fileChangeFromWorktree(tree, wt, path, status[path])
		if err != nil {
			return nil, err
		}
		if fc == nil {
			continue
		}
		files = append(files, *fc)
	}

	return &CommitDiff{
		Commit: CommitInfo{
			Hash:      WorktreeHash,
			ShortHash: WorktreeHash,
			Author:    e.userIdentity(),
			Message:   "uncommitted changes vs " + against + "\n",
		},
		Parent: commit.Hash.String(),
		Files:  files,
	}, nil
}

func isUntracked(st *git.FileStatus) bool {
	return st != nil && st.Staging == git.Untracked && st.Worktree == git.Untracked
}

func (e *Engine) userIdentity() Person {
	cfg, err := e.repo.Config()
	if err != nil {
		return Person{}
	}
	name := cfg.User.Name
	email := cfg.User.Email
	if name == "" && email == "" {
		return Person{}
	}
	return Person{Name: name, Email: email}
}

func fileChangeFromWorktree(tree *object.Tree, wt *git.Worktree, path string, st *git.FileStatus) (*FileChange, error) {
	headPath := path
	if st != nil && st.Staging == git.Renamed && st.Extra != "" {
		headPath = st.Extra
	}

	oldContent, oldBin, oldExists, err := blobFromTree(tree, headPath)
	if err != nil {
		return nil, fmt.Errorf("read HEAD %s: %w", headPath, err)
	}
	newContent, newBin, newExists, err := blobFromWorktree(wt, path)
	if err != nil {
		return nil, fmt.Errorf("read worktree %s: %w", path, err)
	}

	if !oldExists && !newExists {
		return nil, nil
	}
	if oldExists && newExists && oldBin == newBin && oldContent == newContent {
		return nil, nil
	}

	fc := FileChange{
		Binary: oldBin || newBin,
	}
	switch {
	case !oldExists:
		fc.Kind = ChangeAdded
		fc.Path = path
	case !newExists:
		fc.Kind = ChangeDeleted
		fc.OldPath = headPath
	case headPath != path:
		fc.Kind = ChangeRenamed
		fc.OldPath = headPath
		fc.Path = path
	default:
		fc.Kind = ChangeModified
		fc.OldPath = path
		fc.Path = path
	}

	if fc.Binary {
		return &fc, nil
	}
	if oldExists {
		fc.OldContent = oldContent
	}
	if newExists {
		fc.NewContent = newContent
	}
	fc.Hunks = hunksFromTexts(oldContent, newContent)
	return &fc, nil
}

func blobFromTree(tree *object.Tree, path string) (content string, binary, exists bool, err error) {
	if tree == nil || path == "" {
		return "", false, false, nil
	}
	f, err := tree.File(path)
	if err != nil {
		if err == object.ErrFileNotFound {
			return "", false, false, nil
		}
		return "", false, false, err
	}
	text, bin, err := blobText(f)
	if err != nil {
		return "", false, false, err
	}
	return text, bin, true, nil
}

func blobFromWorktree(wt *git.Worktree, path string) (content string, binary, exists bool, err error) {
	if wt == nil || path == "" {
		return "", false, false, nil
	}
	fi, err := wt.Filesystem.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, false, nil
		}
		return "", false, false, err
	}
	if fi.IsDir() {
		return "", false, false, nil
	}

	data, err := util.ReadFile(wt.Filesystem, path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, false, nil
		}
		return "", false, false, err
	}
	if sniffBinary(data) {
		return "", true, true, nil
	}
	return string(data), false, true, nil
}

func sniffBinary(data []byte) bool {
	n := binarySniffLen
	if len(data) < n {
		n = len(data)
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}
