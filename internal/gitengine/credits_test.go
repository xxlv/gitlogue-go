package gitengine

import (
	"strings"
	"testing"
	"time"
)

func TestParseTrailers(t *testing.T) {
	t.Parallel()
	msg := "fix parser\n\nMore detail.\n\nCo-authored-by: Copilot <copilot@github.com>\nReviewed-by: Chen <chen@example.com>\nGenerated-by: Cursor\n"
	got := ParseTrailers(msg)
	if len(got) != 3 {
		t.Fatalf("trailers = %d, want 3: %+v", len(got), got)
	}
	if got[0].Key != "Co-authored-by" || !strings.Contains(got[0].Value, "Copilot") {
		t.Fatalf("first = %+v", got[0])
	}
	if ParseTrailers("no trailers here\n") != nil && len(ParseTrailers("no trailers here\n")) != 0 {
		t.Fatalf("plain message should have no trailers")
	}
}

func TestParsePerson(t *testing.T) {
	t.Parallel()
	p := ParsePerson("Alice Reviewer <alice@example.com>")
	if p.Name != "Alice Reviewer" || p.Email != "alice@example.com" {
		t.Fatalf("got %+v", p)
	}
	if ParsePerson("Cursor").Name != "Cursor" {
		t.Fatal("bare name")
	}
}

func TestLooksLikeAgent(t *testing.T) {
	t.Parallel()
	if !LooksLikeAgent(Person{Name: "Copilot", Email: "copilot@github.com"}) {
		t.Fatal("copilot should be agent")
	}
	if !LooksLikeAgent(Person{Name: "Cursor"}) {
		t.Fatal("Cursor should be agent")
	}
	if LooksLikeAgent(Person{Name: "Alice", Email: "alice@example.com"}) {
		t.Fatal("human must not be tagged agent")
	}
}

func TestCreditsAuthorCommitterTrailers(t *testing.T) {
	t.Parallel()
	c := CommitInfo{
		Author:    Person{Name: "Alice", Email: "alice@example.com", When: time.Unix(0, 0)},
		Committer: Person{Name: "Bob", Email: "bob@example.com"},
		Message:   "fix logs",
		Trailers: []Trailer{
			{Key: "Co-authored-by", Value: "Copilot <copilot@github.com>"},
			{Key: "Reviewed-by", Value: "Chen <chen@example.com>"},
			{Key: "Generated-by", Value: "Cursor"},
		},
	}
	got := c.Credits()
	if len(got) != 5 {
		t.Fatalf("credits = %d, want 5: %+v", len(got), got)
	}
	if got[0].Role != RoleAuthor || got[0].Agent {
		t.Fatalf("author = %+v", got[0])
	}
	if got[1].Role != RoleCommitter {
		t.Fatalf("committer = %+v", got[1])
	}
	if !c.HasAgent() {
		t.Fatal("expected agent among credits")
	}
	var roles []string
	for _, cr := range got {
		roles = append(roles, string(cr.Role))
	}
	if strings.Join(roles, ",") != "author,committer,co-author,reviewed-by,generated-by" {
		t.Fatalf("roles = %s", strings.Join(roles, ","))
	}
	if !got[2].Agent || !got[4].Agent {
		t.Fatalf("copilot and generated-by should be agent: %+v", got)
	}
}

func TestCreditsSkipsMatchingCommitter(t *testing.T) {
	t.Parallel()
	c := CommitInfo{
		Author:    Person{Name: "Alice", Email: "alice@example.com"},
		Committer: Person{Name: "Alice", Email: "alice@example.com"},
	}
	got := c.Credits()
	if len(got) != 1 || got[0].Role != RoleAuthor {
		t.Fatalf("got %+v", got)
	}
}

func TestSubject(t *testing.T) {
	t.Parallel()
	c := CommitInfo{Message: "fix parser\n\nbody\n"}
	if c.Subject() != "fix parser" {
		t.Fatalf("subject = %q", c.Subject())
	}
}
