package gitengine

import (
	"strings"
	"time"
)

// ChangeKind classifies how a path changed between the parent tree and the
// target commit. Rename detection is best-effort (go-git similarity).
type ChangeKind int

const (
	ChangeAdded ChangeKind = iota
	ChangeDeleted
	ChangeModified
	ChangeRenamed
)

func (k ChangeKind) String() string {
	switch k {
	case ChangeAdded:
		return "added"
	case ChangeDeleted:
		return "deleted"
	case ChangeModified:
		return "modified"
	case ChangeRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// LineKind is a single line's role inside a unified-diff hunk.
type LineKind int

const (
	LineContext LineKind = iota
	LineAdded
	LineDeleted
)

func (k LineKind) Prefix() byte {
	switch k {
	case LineAdded:
		return '+'
	case LineDeleted:
		return '-'
	default:
		return ' '
	}
}

// DiffLine is one row of a hunk. Line numbers are 1-based; zero means the
// side does not exist (added lines have OldNumber == 0, deleted have NewNumber == 0).
type DiffLine struct {
	Kind      LineKind
	OldNumber int
	NewNumber int
	Content   string // raw text without the +/-/space prefix
}

// Hunk is a unified-diff hunk reconstructed from go-git FilePatch chunks.
// Animator (Step 2) will consume these as the source of Action streams.
type Hunk struct {
	OldStart int // 1-based; 0 means the old side is empty
	OldLines int
	NewStart int
	NewLines int
	Header   string // e.g. "@@ -10,6 +10,8 @@"
	Lines    []DiffLine
}

// FileChange is one path's delta inside a commit.
type FileChange struct {
	Path       string // path in the new tree; empty when the file was deleted
	OldPath    string // path in the old tree; empty when the file was added
	Kind       ChangeKind
	Binary     bool
	Hunks      []Hunk
	OldContent string // entire old blob; empty when added or binary
	NewContent string // entire new blob; empty when deleted or binary
}

// DisplayPath is the path shown in the TUI file tree and inspect dump.
func (f FileChange) DisplayPath() string {
	if f.Path != "" {
		return f.Path
	}
	return f.OldPath
}

// LineStats counts added and deleted lines across the file's hunks.
func (f FileChange) LineStats() (add, del int) {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			switch l.Kind {
			case LineAdded:
				add++
			case LineDeleted:
				del++
			}
		}
	}
	return add, del
}

// CommitInfo is the metadata the status bar and credit strip display.
// Author is who wrote the patch; Committer is who created the Git object
// (they differ when an agent commits on someone's behalf, or vice versa).
type CommitInfo struct {
	Hash      string
	ShortHash string
	Author    Person
	Committer Person
	Message   string
	Trailers  []Trailer
}

// Person is a Git identity (author, committer, or a trailer actor).
type Person struct {
	Name  string
	Email string
	When  time.Time
}

func (p Person) String() string {
	name := strings.TrimSpace(p.Name)
	email := strings.TrimSpace(p.Email)
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	default:
		return email
	}
}

func (p Person) IsZero() bool {
	return strings.TrimSpace(p.Name) == "" && strings.TrimSpace(p.Email) == ""
}

func (p Person) SameIdentity(o Person) bool {
	a := strings.ToLower(strings.TrimSpace(p.Email))
	b := strings.ToLower(strings.TrimSpace(o.Email))
	if a != "" && a == b {
		return true
	}
	if a != "" || b != "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(o.Name)) && p.Name != ""
}

// Trailer is one Git commit-message trailer (`Key: value`).
type Trailer struct {
	Key   string
	Value string
}

// CommitDiff is the complete, structured patch for one commit. This is the
// contract between gitengine and the rest of the pipeline.
type CommitDiff struct {
	Commit CommitInfo
	Parent string // empty for the root commit
	Files  []FileChange
}
