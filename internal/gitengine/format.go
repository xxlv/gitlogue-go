package gitengine

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Format renders a human-readable inspect dump of the commit patch.
func (d *CommitDiff) Format() string {
	var b strings.Builder
	d.Fprint(&b)
	return b.String()
}

// Fprint writes the inspect dump to w.
func (d *CommitDiff) Fprint(w io.Writer) {
	if d == nil {
		return
	}

	fmt.Fprintf(w, "commit %s\n", d.Commit.Hash)
	fmt.Fprintf(w, "Author:    %s\n", d.Commit.Author)
	if !d.Commit.Committer.IsZero() && !d.Commit.Committer.SameIdentity(d.Commit.Author) {
		fmt.Fprintf(w, "Committer: %s\n", d.Commit.Committer)
	}
	when := d.Commit.Author.When
	if when.IsZero() {
		when = d.Commit.Committer.When
	}
	if !when.IsZero() {
		fmt.Fprintf(w, "Date:      %s\n", when.Format(time.RFC1123Z))
	}
	if credits := d.Commit.Credits(); len(credits) > 0 {
		fmt.Fprintln(w, "Credits:")
		for _, cr := range credits {
			tag := ""
			if cr.Agent {
				tag = "  [agent]"
			}
			fmt.Fprintf(w, "  %-10s %s%s\n", cr.Role.Label(), cr.Who, tag)
		}
	}
	if d.Parent != "" {
		fmt.Fprintf(w, "Parent: %s\n", d.Parent)
	} else {
		fmt.Fprintf(w, "Parent: (root commit)\n")
	}
	fmt.Fprintln(w)
	for _, line := range strings.Split(strings.TrimSuffix(d.Commit.Message, "\n"), "\n") {
		fmt.Fprintf(w, "    %s\n", line)
	}
	fmt.Fprintln(w)

	insertions, deletions := 0, 0
	for _, f := range d.Files {
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				switch l.Kind {
				case LineAdded:
					insertions++
				case LineDeleted:
					deletions++
				}
			}
		}
	}
	fmt.Fprintf(w, " %d file(s) changed, %d insertion(s)(+), %d deletion(s)(-)\n",
		len(d.Files), insertions, deletions)

	for _, f := range d.Files {
		writeFile(w, f)
	}
}

func writeFile(w io.Writer, f FileChange) {
	title := f.DisplayPath()
	if f.Kind == ChangeRenamed {
		title = f.OldPath + " => " + f.Path
	}
	fmt.Fprintf(w, "\n=== %s [%s] ===\n", title, f.Kind)
	if f.Binary {
		fmt.Fprintln(w, "(binary file)")
		return
	}
	if len(f.Hunks) == 0 {
		fmt.Fprintln(w, "(no textual hunks)")
		return
	}
	for _, h := range f.Hunks {
		fmt.Fprintln(w, h.Header)
		for _, l := range h.Lines {
			fmt.Fprintf(w, "%c%s\n", l.Kind.Prefix(), l.Content)
		}
	}
}
