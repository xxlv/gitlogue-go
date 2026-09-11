package gitengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestOpenMissingRepo(t *testing.T) {
	t.Parallel()
	_, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("expected error opening a directory without .git")
	}
}

func TestInspectRootAndFollowUpCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := initRepo(t, dir)

	write(t, dir, "hello.go", "package hello\n\nfunc Hi() {}\n")
	root := commitAll(t, repo, "initial import")

	engine, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	rootDiff, err := engine.Inspect(root)
	if err != nil {
		t.Fatalf("Inspect root: %v", err)
	}
	if rootDiff.Parent != "" {
		t.Fatalf("root parent = %q, want empty", rootDiff.Parent)
	}
	if len(rootDiff.Files) != 1 {
		t.Fatalf("root files = %d, want 1", len(rootDiff.Files))
	}
	if rootDiff.Files[0].Kind != ChangeAdded {
		t.Fatalf("root kind = %s, want added", rootDiff.Files[0].Kind)
	}
	if got := rootDiff.Files[0].DisplayPath(); got != "hello.go" {
		t.Fatalf("root path = %q, want hello.go", got)
	}
	if rootDiff.Files[0].NewContent == "" {
		t.Fatal("root commit missing NewContent")
	}
	if len(rootDiff.Files[0].Hunks) == 0 {
		t.Fatal("root commit produced no hunks")
	}

	write(t, dir, "hello.go", "package hello\n\nfunc Hi() {\n\tprintln(\"hi\")\n}\n")
	write(t, dir, "README.md", "# hello\n")
	second := commitAll(t, repo, "add greeting and readme")

	diff, err := engine.Inspect(second)
	if err != nil {
		t.Fatalf("Inspect second: %v", err)
	}
	if diff.Parent != root {
		t.Fatalf("parent = %s, want %s", diff.Parent, root)
	}
	if len(diff.Files) != 2 {
		t.Fatalf("files = %d, want 2\n%s", len(diff.Files), diff.Format())
	}

	byPath := map[string]FileChange{}
	for _, f := range diff.Files {
		byPath[f.DisplayPath()] = f
	}
	readme, ok := byPath["README.md"]
	if !ok || readme.Kind != ChangeAdded {
		t.Fatalf("README.md missing or not added: %+v", readme)
	}
	hello, ok := byPath["hello.go"]
	if !ok || hello.Kind != ChangeModified {
		t.Fatalf("hello.go missing or not modified: %+v", hello)
	}
	if hello.OldContent == "" || hello.NewContent == "" {
		t.Fatalf("hello.go missing blob text: old=%q new=%q", hello.OldContent, hello.NewContent)
	}

	dump := diff.Format()
	if !strings.Contains(dump, "@@") {
		t.Fatalf("dump missing hunk header:\n%s", dump)
	}
	if !strings.Contains(dump, "+") || !strings.Contains(dump, "println") {
		t.Fatalf("dump missing added greeting:\n%s", dump)
	}
}

func TestHunkHeaderPureInsertion(t *testing.T) {
	t.Parallel()
	lines := []rawLine{
		{Kind: LineAdded, Content: "a", NewNo: 1, OldPos: 0},
		{Kind: LineAdded, Content: "b", NewNo: 2, OldPos: 0},
	}
	h := buildHunk(lines)
	want := "@@ -0,0 +1,2 @@"
	if h.Header != want {
		t.Fatalf("header = %q, want %q", h.Header, want)
	}
}

func TestHunksFromTextsModify(t *testing.T) {
	t.Parallel()
	hunks := hunksFromTexts("package hello\n\nfunc Hi() {}\n", "package hello\n\nfunc Hi() {\n\tprintln(\"hi\")\n}\n")
	if len(hunks) == 0 {
		t.Fatal("expected hunks")
	}
	dump := ""
	for _, h := range hunks {
		dump += h.Header + "\n"
		for _, l := range h.Lines {
			dump += string(l.Kind.Prefix()) + l.Content + "\n"
		}
	}
	if !strings.Contains(dump, "println") {
		t.Fatalf("missing added line:\n%s", dump)
	}
}

func TestWorkingTreeClean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo := initRepo(t, dir)
	write(t, dir, "hello.go", "package hello\n")
	head := commitAll(t, repo, "initial")

	engine, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	diff, err := engine.WorkingTree("HEAD")
	if err != nil {
		t.Fatalf("WorkingTree: %v", err)
	}
	if diff.Commit.Hash != WorktreeHash {
		t.Fatalf("hash = %q, want %s", diff.Commit.Hash, WorktreeHash)
	}
	if diff.Parent != head {
		t.Fatalf("parent = %s, want %s", diff.Parent, head)
	}
	if len(diff.Files) != 0 {
		t.Fatalf("clean worktree files = %d\n%s", len(diff.Files), diff.Format())
	}
}

func TestWorkingTreeModifiedIgnoresUntracked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo := initRepo(t, dir)
	write(t, dir, "hello.go", "package hello\n\nfunc Hi() {}\n")
	head := commitAll(t, repo, "initial")

	write(t, dir, "hello.go", "package hello\n\nfunc Hi() {\n\tprintln(\"hi\")\n}\n")
	write(t, dir, "scratch.md", "# untracked\n")

	engine, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	diff, err := engine.WorkingTree("")
	if err != nil {
		t.Fatalf("WorkingTree: %v", err)
	}
	if diff.Parent != head {
		t.Fatalf("parent = %s, want %s", diff.Parent, head)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("files = %d, want 1 (untracked omitted)\n%s", len(diff.Files), diff.Format())
	}
	f := diff.Files[0]
	if f.DisplayPath() != "hello.go" || f.Kind != ChangeModified {
		t.Fatalf("got %s [%s]", f.DisplayPath(), f.Kind)
	}
	if !strings.Contains(f.NewContent, "println") {
		t.Fatalf("new content missing edit: %q", f.NewContent)
	}
	if len(f.Hunks) == 0 {
		t.Fatal("modified file produced no hunks")
	}
}

func TestWorkingTreeStagedAddAndDelete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo := initRepo(t, dir)
	write(t, dir, "hello.go", "package hello\n")
	write(t, dir, "gone.go", "package gone\n")
	commitAll(t, repo, "initial")

	write(t, dir, "new.go", "package new\n")
	if err := os.Remove(filepath.Join(dir, "gone.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := wt.Add("new.go"); err != nil {
		t.Fatalf("Add new.go: %v", err)
	}

	engine, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	diff, err := engine.WorkingTree("HEAD")
	if err != nil {
		t.Fatalf("WorkingTree: %v", err)
	}

	byPath := map[string]FileChange{}
	for _, f := range diff.Files {
		byPath[f.DisplayPath()] = f
	}
	added, ok := byPath["new.go"]
	if !ok || added.Kind != ChangeAdded {
		t.Fatalf("new.go missing or not added: %+v\n%s", added, diff.Format())
	}
	deleted, ok := byPath["gone.go"]
	if !ok || deleted.Kind != ChangeDeleted {
		t.Fatalf("gone.go missing or not deleted: %+v\n%s", deleted, diff.Format())
	}
	if _, ok := byPath["hello.go"]; ok {
		t.Fatalf("hello.go should be unchanged:\n%s", diff.Format())
	}
}

func TestWorkingTreeFoldsStagedAndUnstaged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo := initRepo(t, dir)
	write(t, dir, "hello.go", "one\n")
	commitAll(t, repo, "initial")

	write(t, dir, "hello.go", "two\n")
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := wt.Add("hello.go"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	write(t, dir, "hello.go", "three\n")

	engine, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	diff, err := engine.WorkingTree("HEAD")
	if err != nil {
		t.Fatalf("WorkingTree: %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("files = %d, want 1\n%s", len(diff.Files), diff.Format())
	}
	f := diff.Files[0]
	if f.OldContent != "one\n" || f.NewContent != "three\n" {
		t.Fatalf("folded contents old=%q new=%q, want one -> three", f.OldContent, f.NewContent)
	}
}

func TestHistoryOldestFirst(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo := initRepo(t, dir)

	write(t, dir, "hello.go", "package hello\n")
	a := commitAll(t, repo, "one")
	write(t, dir, "hello.go", "package hello\n\nfunc A() {}\n")
	b := commitAll(t, repo, "two")
	write(t, dir, "README.md", "# hi\n")
	c := commitAll(t, repo, "three")

	engine, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	all, err := engine.History("HEAD", 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("History(HEAD, 10) = %d commits, want 3", len(all))
	}
	got := []string{all[0].Commit.Hash, all[1].Commit.Hash, all[2].Commit.Hash}
	want := []string{a, b, c}
	if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("oldest-first hashes = %v, want %v", got, want)
	}

	pair, err := engine.History(c, 2)
	if err != nil {
		t.Fatalf("History(c, 2): %v", err)
	}
	if len(pair) != 2 {
		t.Fatalf("n=2 yielded %d", len(pair))
	}
	if pair[0].Commit.Hash != b || pair[1].Commit.Hash != c {
		t.Fatalf("n=2 = %s %s, want %s %s", pair[0].Commit.Hash, pair[1].Commit.Hash, b, c)
	}

	one, err := engine.History(c, 0)
	if err != nil {
		t.Fatalf("History n=0: %v", err)
	}
	if len(one) != 1 || one[0].Commit.Hash != c {
		t.Fatalf("n<1 should clamp to the tip commit")
	}

	ranged, err := engine.RevSpec(a+".."+c, 0)
	if err != nil {
		t.Fatalf("RevSpec A..C: %v", err)
	}
	if len(ranged) != 2 {
		t.Fatalf("A..C = %d commits, want 2 (B and C)", len(ranged))
	}
	if ranged[0].Commit.Hash != b || ranged[1].Commit.Hash != c {
		t.Fatalf("A..C hashes = %s %s, want %s %s", ranged[0].Commit.Hash, ranged[1].Commit.Hash, b, c)
	}

	tip, err := engine.RevSpec("", -1)
	if err != nil {
		t.Fatalf("RevSpec empty: %v", err)
	}
	if len(tip) != 1 || tip[0].Commit.Hash != c {
		t.Fatalf("empty spec should be HEAD (1 commit)")
	}
}

func TestSplitRev(t *testing.T) {
	t.Parallel()
	left, right, dots := SplitRev("main..feature")
	if left != "main" || right != "feature" || dots != 2 {
		t.Fatalf("got %q %q %d", left, right, dots)
	}
	left, right, dots = SplitRev("main...feature")
	if left != "main" || right != "feature" || dots != 3 {
		t.Fatalf("three-dot got %q %q %d", left, right, dots)
	}
	left, right, dots = SplitRev("7a8b9c0")
	if left != "" || right != "7a8b9c0" || dots != 0 {
		t.Fatalf("hash got %q %q %d", left, right, dots)
	}
}

func TestFilterDiffBasename(t *testing.T) {
	t.Parallel()
	d := &CommitDiff{Files: []FileChange{
		{Path: "cmd/main.go", Kind: ChangeModified},
		{Path: "README.md", Kind: ChangeAdded},
	}}
	got := FilterDiff(d, "main.go")
	if len(got.Files) != 1 || got.Files[0].Path != "cmd/main.go" {
		t.Fatalf("filter main.go = %+v", got.Files)
	}
	none := FilterDiffs([]*CommitDiff{d}, "nope.go")
	if len(none) != 0 {
		t.Fatalf("unmatched file should drop the commit")
	}
}

func TestInspectAttribution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo := initRepo(t, dir)
	write(t, dir, "hello.go", "package hello\n")

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	when := time.Date(2026, 9, 7, 16, 0, 0, 0, time.UTC)
	hash, err := wt.Commit("fix logs\n\nCo-authored-by: Copilot <copilot@github.com>\nReviewed-by: Chen <chen@example.com>\nGenerated-by: Cursor\n", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Alice",
			Email: "alice@example.com",
			When:  when,
		},
		Committer: &object.Signature{
			Name:  "Bob",
			Email: "bob@example.com",
			When:  when,
		},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	engine, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	diff, err := engine.Inspect(hash.String())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if diff.Commit.Author.Name != "Alice" || diff.Commit.Committer.Name != "Bob" {
		t.Fatalf("author/committer = %s / %s", diff.Commit.Author, diff.Commit.Committer)
	}
	if len(diff.Commit.Trailers) != 3 {
		t.Fatalf("trailers = %+v", diff.Commit.Trailers)
	}
	dump := diff.Format()
	for _, want := range []string{"Alice", "Bob", "Copilot", "Chen", "Cursor", "[agent]", "author", "co-author", "review", "generated"} {
		if !strings.Contains(dump, want) {
			t.Fatalf("dump missing %q:\n%s", want, dump)
		}
	}
}

func TestGroupHunksMergesNearbyChanges(t *testing.T) {
	t.Parallel()
	// Two edits separated by 2 context lines should become a single hunk
	// when context=3 (gap 2 <= 6).
	lines := []rawLine{
		{Kind: LineContext, Content: "1", OldNo: 1, NewNo: 1},
		{Kind: LineDeleted, Content: "old", OldNo: 2},
		{Kind: LineAdded, Content: "new", NewNo: 2},
		{Kind: LineContext, Content: "3", OldNo: 3, NewNo: 3},
		{Kind: LineContext, Content: "4", OldNo: 4, NewNo: 4},
		{Kind: LineAdded, Content: "extra", NewNo: 5, OldPos: 4},
		{Kind: LineContext, Content: "5", OldNo: 5, NewNo: 6},
	}
	hunks := groupHunks(lines, 3)
	if len(hunks) != 1 {
		t.Fatalf("hunks = %d, want 1", len(hunks))
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
