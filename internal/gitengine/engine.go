// Package gitengine reads a local Git repository and turns a commit into a
// structured patch (file list + unified-diff hunks) for later animation.
package gitengine

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Engine is a thin handle around an opened go-git repository.
type Engine struct {
	path string
	repo *git.Repository
}

// Open locates a Git repository at path (or in a parent directory) and
// returns an Engine bound to it.
func Open(path string) (*Engine, error) {
	if path == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve repository path %q: %w", path, err)
	}

	repo, err := git.PlainOpenWithOptions(abs, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open git repository %q: %w", abs, err)
	}

	return &Engine{path: abs, repo: repo}, nil
}

// Path returns the absolute path that was opened.
func (e *Engine) Path() string {
	return e.path
}

// Inspect resolves rev (default "HEAD") and returns every file change plus
// reconstructed hunks against the first parent. The root commit is treated
// as a full-tree addition against an empty parent.
func (e *Engine) Inspect(rev string) (*CommitDiff, error) {
	if e == nil || e.repo == nil {
		return nil, fmt.Errorf("gitengine: engine is not open")
	}
	if rev == "" {
		rev = "HEAD"
	}

	commit, err := e.resolveCommit(rev)
	if err != nil {
		return nil, err
	}

	parent, parentHash, err := firstParent(commit)
	if err != nil {
		return nil, err
	}

	toTree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("read tree of %s: %w", commit.Hash.String(), err)
	}

	var fromTree *object.Tree
	if parent != nil {
		fromTree, err = parent.Tree()
		if err != nil {
			return nil, fmt.Errorf("read parent tree of %s: %w", commit.Hash.String(), err)
		}
	}

	files, err := diffTrees(fromTree, toTree)
	if err != nil {
		return nil, fmt.Errorf("diff commit %s: %w", commit.Hash.String(), err)
	}

	return &CommitDiff{
		Commit: commitInfo(commit),
		Parent: parentHash,
		Files:  files,
	}, nil
}

func (e *Engine) resolveCommit(rev string) (*object.Commit, error) {
	hash, err := e.repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, fmt.Errorf("resolve revision %q: %w", rev, err)
	}
	commit, err := e.repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("read commit %s: %w", hash.String(), err)
	}
	return commit, nil
}

func firstParent(c *object.Commit) (*object.Commit, string, error) {
	if c.NumParents() == 0 {
		return nil, "", nil
	}
	parent, err := c.Parent(0)
	if err != nil {
		return nil, "", fmt.Errorf("read first parent of %s: %w", c.Hash.String(), err)
	}
	return parent, parent.Hash.String(), nil
}

func commitInfo(c *object.Commit) CommitInfo {
	hash := c.Hash.String()
	short := hash
	if len(short) > 7 {
		short = short[:7]
	}
	return CommitInfo{
		Hash:      hash,
		ShortHash: short,
		Author: Person{
			Name:  c.Author.Name,
			Email: c.Author.Email,
			When:  c.Author.When,
		},
		Committer: Person{
			Name:  c.Committer.Name,
			Email: c.Committer.Email,
			When:  c.Committer.When,
		},
		Message:  c.Message,
		Trailers: ParseTrailers(c.Message),
	}
}
