package animator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
)

func TestApplyNewFile(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileChange{
		Path:       "a.txt",
		Kind:       gitengine.ChangeAdded,
		NewContent: "ab\ncd\n",
		Hunks: []gitengine.Hunk{{
			OldStart: 0,
			OldLines: 0,
			NewStart: 1,
			NewLines: 2,
			Lines: []gitengine.DiffLine{
				{Kind: gitengine.LineAdded, Content: "ab"},
				{Kind: gitengine.LineAdded, Content: "cd"},
			},
		}},
	}
	got := Apply("", CompileFile(fc, Options{Seed: 1}).Actions)
	if got != fc.NewContent {
		t.Fatalf("got %q, want %q", got, fc.NewContent)
	}
}

func TestApplyReplaceLine(t *testing.T) {
	t.Parallel()
	old := "hello\nworld\n"
	fc := gitengine.FileChange{
		Path:       "a.txt",
		Kind:       gitengine.ChangeModified,
		OldContent: old,
		NewContent: "hello\nthere\n",
		Hunks: []gitengine.Hunk{{
			OldStart: 1,
			OldLines: 2,
			NewStart: 1,
			NewLines: 2,
			Lines: []gitengine.DiffLine{
				{Kind: gitengine.LineContext, Content: "hello"},
				{Kind: gitengine.LineDeleted, Content: "world"},
				{Kind: gitengine.LineAdded, Content: "there"},
			},
		}},
	}
	got := Apply(old, CompileFile(fc, Options{Seed: 1}).Actions)
	if got != fc.NewContent {
		t.Fatalf("got %q, want %q", got, fc.NewContent)
	}
}

func TestApplyInsertBetween(t *testing.T) {
	t.Parallel()
	old := "a\nc\n"
	fc := gitengine.FileChange{
		Path:       "a.txt",
		Kind:       gitengine.ChangeModified,
		OldContent: old,
		NewContent: "a\nb\nc\n",
		Hunks: []gitengine.Hunk{{
			OldStart: 1,
			OldLines: 2,
			NewStart: 1,
			NewLines: 3,
			Lines: []gitengine.DiffLine{
				{Kind: gitengine.LineContext, Content: "a"},
				{Kind: gitengine.LineAdded, Content: "b"},
				{Kind: gitengine.LineContext, Content: "c"},
			},
		}},
	}
	got := Apply(old, CompileFile(fc, Options{Seed: 7}).Actions)
	if got != fc.NewContent {
		t.Fatalf("got %q, want %q", got, fc.NewContent)
	}
}

func TestApplyDeleteLine(t *testing.T) {
	t.Parallel()
	old := "keep\ngone\n"
	fc := gitengine.FileChange{
		Path:       "a.txt",
		Kind:       gitengine.ChangeModified,
		OldContent: old,
		NewContent: "keep\n",
		Hunks: []gitengine.Hunk{{
			OldStart: 1,
			OldLines: 2,
			NewStart: 1,
			NewLines: 1,
			Lines: []gitengine.DiffLine{
				{Kind: gitengine.LineContext, Content: "keep"},
				{Kind: gitengine.LineDeleted, Content: "gone"},
			},
		}},
	}
	got := Apply(old, CompileFile(fc, Options{Seed: 3}).Actions)
	if got != fc.NewContent {
		t.Fatalf("got %q, want %q", got, fc.NewContent)
	}
}

func TestGaussianDelayRanges(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileChange{
		Path:       "a.txt",
		Kind:       gitengine.ChangeAdded,
		NewContent: "a.\n",
		Hunks: []gitengine.Hunk{{
			NewStart: 1,
			NewLines: 1,
			Lines:    []gitengine.DiffLine{{Kind: gitengine.LineAdded, Content: "a."}},
		}},
	}
	script := CompileFile(fc, Options{Seed: 42})
	var sawType, sawPause bool
	for _, a := range script.Actions {
		switch a.Kind {
		case KindTypeChar:
			if a.Rune == 'a' {
				sawType = true
				assertRange(t, "type 'a'", a.Delay, 30*time.Millisecond, 60*time.Millisecond)
			}
			if a.Rune == '.' {
				sawPause = true
				assertRange(t, "type '.'", a.Delay, 150*time.Millisecond, 300*time.Millisecond)
			}
		case KindInsertLine:
			assertRange(t, "insert", a.Delay, 150*time.Millisecond, 300*time.Millisecond)
		}
	}
	if !sawType || !sawPause {
		t.Fatalf("missing sampled delays: type=%v pause=%v\n%s", sawType, sawPause, script.Format())
	}
}

func TestSameSeedIsDeterministic(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileChange{
		Path: "a.txt",
		Kind: gitengine.ChangeAdded,
		Hunks: []gitengine.Hunk{{
			Lines: []gitengine.DiffLine{{Kind: gitengine.LineAdded, Content: "hello, world."}},
		}},
	}
	a := CompileFile(fc, Options{Seed: 99})
	b := CompileFile(fc, Options{Seed: 99})
	if a.Format() != b.Format() {
		t.Fatal("same seed produced different scripts")
	}
}

func TestCompileGitCommitRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := initRepo(t, dir)
	write(t, dir, "hello.go", "package hello\n\nfunc Hi() {}\n")
	commitAll(t, repo, "initial")
	write(t, dir, "hello.go", "package hello\n\nfunc Hi() {\n\tprintln(\"hi\")\n}\n")
	write(t, dir, "README.md", "# hello\n")
	hash := commitAll(t, repo, "edit")

	engine, err := gitengine.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	diff, err := engine.Inspect(hash)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	script := Compile(diff, Options{Seed: 1})
	if len(script.Actions) == 0 {
		t.Fatal("empty script")
	}

	byPath := map[string][]Action{}
	for _, a := range script.Actions {
		byPath[a.File] = append(byPath[a.File], a)
	}
	for _, f := range diff.Files {
		if f.Binary {
			continue
		}
		got := Apply(f.OldContent, byPath[f.DisplayPath()])
		if got != f.NewContent {
			t.Fatalf("%s round-trip:\n got %q\nwant %q\n%s",
				f.DisplayPath(), got, f.NewContent, Script{Actions: byPath[f.DisplayPath()]}.Format())
		}
	}
}

func assertRange(t *testing.T, name string, got, min, max time.Duration) {
	t.Helper()
	if got < min || got > max {
		t.Fatalf("%s delay %s outside [%s, %s]", name, got, min, max)
	}
}

func initRepo(t *testing.T, dir string) *git.Repository {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	return repo
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func commitAll(t *testing.T, repo *git.Repository, msg string) string {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "gitlogue",
			Email: "gitlogue@example.com",
			When:  time.Date(2026, 9, 7, 15, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return hash.String()
}

func TestScriptFormatContainsHeader(t *testing.T) {
	t.Parallel()
	fc := gitengine.FileChange{
		Path: "a.txt",
		Kind: gitengine.ChangeAdded,
		Hunks: []gitengine.Hunk{{
			Lines: []gitengine.DiffLine{{Kind: gitengine.LineAdded, Content: "x"}},
		}},
	}
	dump := CompileFile(fc, Options{Seed: 1}).Format()
	if !strings.Contains(dump, "actions") || !strings.Contains(dump, "type") {
		t.Fatalf("unexpected dump:\n%s", dump)
	}
}
