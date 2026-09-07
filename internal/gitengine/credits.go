package gitengine

import (
	"regexp"
	"strings"
)

// CreditRole is who someone was on a commit, for the review strip.
type CreditRole string

const (
	RoleAuthor    CreditRole = "author"
	RoleCommitter CreditRole = "committer"
	RoleCoAuthor  CreditRole = "co-author"
	RoleReviewed  CreditRole = "reviewed-by"
	RoleSignedOff CreditRole = "signed-off-by"
	RoleAcked     CreditRole = "acked-by"
	RoleTested    CreditRole = "tested-by"
	RoleReported  CreditRole = "reported-by"
	RoleSuggested CreditRole = "suggested-by"
	RoleGenerated CreditRole = "generated-by"
	RoleAssisted  CreditRole = "assisted-by"
)

// Credit is one row in the attribution strip: a person plus how they
// participated. Agent is true when the identity looks like an AI/bot.
type Credit struct {
	Role  CreditRole
	Who   Person
	Agent bool
}

// Label is the short role shown in the TUI (author / co-author / …).
func (r CreditRole) Label() string {
	switch r {
	case RoleAuthor:
		return "author"
	case RoleCommitter:
		return "commit"
	case RoleCoAuthor:
		return "co-author"
	case RoleReviewed:
		return "review"
	case RoleSignedOff:
		return "sign-off"
	case RoleAcked:
		return "ack"
	case RoleTested:
		return "test"
	case RoleReported:
		return "report"
	case RoleSuggested:
		return "suggest"
	case RoleGenerated:
		return "generated"
	case RoleAssisted:
		return "assisted"
	default:
		return string(r)
	}
}

var trailerLine = regexp.MustCompile(`(?i)^([A-Za-z0-9][A-Za-z0-9-]*):[ \t]+(.+)$`)

var trailerRoles = map[string]CreditRole{
	"co-authored-by": RoleCoAuthor,
	"co-author":      RoleCoAuthor,
	"reviewed-by":    RoleReviewed,
	"reviewer":       RoleReviewed,
	"signed-off-by":  RoleSignedOff,
	"acked-by":       RoleAcked,
	"tested-by":      RoleTested,
	"reported-by":    RoleReported,
	"suggested-by":   RoleSuggested,
	"helped-by":      RoleSuggested,
	"generated-by":   RoleGenerated,
	"generated-with": RoleGenerated,
	"made-with":      RoleGenerated,
	"made-by":        RoleGenerated,
	"assisted-by":    RoleAssisted,
	"ai-assisted-by": RoleAssisted,
	"ai-assisted":    RoleAssisted,
}

var agentNeedles = []string{
	"copilot", "cursor", "chatgpt", "openai", "anthropic", "claude",
	"gemini", "codex", "devin", "aider", "coderabbit", "tabnine",
	"codeium", "windsurf", "grok", "cody", "sweepai", "continue.dev",
	"[bot]", "github-actions", "dependabot", "renovatebot", "renovate[",
	"copilot@github", "noreply@cursor", "cursoragent",
}

// ParseTrailers extracts Git trailers from the suffix of a commit message.
func ParseTrailers(msg string) []Trailer {
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	start := end
	for start > 0 {
		line := strings.TrimSpace(lines[start-1])
		if line == "" {
			break
		}
		if !trailerLine.MatchString(line) {
			break
		}
		start--
	}
	if start == end {
		return nil
	}
	out := make([]Trailer, 0, end-start)
	for _, line := range lines[start:end] {
		m := trailerLine.FindStringSubmatch(strings.TrimSpace(line))
		if len(m) != 3 {
			continue
		}
		out = append(out, Trailer{Key: m[1], Value: strings.TrimSpace(m[2])})
	}
	return out
}

// ParsePerson reads `Name <email>`, a bare email, or a bare name.
func ParsePerson(s string) Person {
	s = strings.TrimSpace(s)
	if s == "" {
		return Person{}
	}
	if i := strings.LastIndex(s, "<"); i >= 0 && strings.HasSuffix(s, ">") {
		return Person{
			Name:  strings.TrimSpace(s[:i]),
			Email: strings.TrimSpace(s[i+1 : len(s)-1]),
		}
	}
	if strings.Contains(s, "@") && !strings.ContainsAny(s, " \t") {
		return Person{Email: s}
	}
	return Person{Name: s}
}

// LooksLikeAgent is a conservative hint that this identity is an AI agent
// or bot, so a human reviewer can spot generated work. False negatives
// are preferred over tagging a person as an agent.
func LooksLikeAgent(p Person) bool {
	blob := strings.ToLower(strings.TrimSpace(p.Name) + " " + strings.TrimSpace(p.Email))
	if blob == " " || blob == "" {
		return false
	}
	for _, n := range agentNeedles {
		if strings.Contains(blob, n) {
			return true
		}
	}
	return false
}

// Subject is the first line of the commit message.
func (c CommitInfo) Subject() string {
	line, _, _ := strings.Cut(strings.TrimSpace(c.Message), "\n")
	return strings.TrimSpace(line)
}

// Credits is the ordered attribution list for the review strip.
func (c CommitInfo) Credits() []Credit {
	var out []Credit
	if !c.Author.IsZero() {
		out = append(out, Credit{Role: RoleAuthor, Who: c.Author, Agent: LooksLikeAgent(c.Author)})
	}
	if !c.Committer.IsZero() && !c.Committer.SameIdentity(c.Author) {
		out = append(out, Credit{Role: RoleCommitter, Who: c.Committer, Agent: LooksLikeAgent(c.Committer)})
	}
	seen := make(map[string]bool)
	for _, t := range c.Trailers {
		role, ok := trailerRoles[strings.ToLower(t.Key)]
		if !ok {
			continue
		}
		who := ParsePerson(t.Value)
		if who.IsZero() {
			continue
		}
		key := string(role) + "\x00" + strings.ToLower(who.Email+"|"+who.Name)
		if seen[key] {
			continue
		}
		if role == RoleCoAuthor && who.SameIdentity(c.Author) {
			continue
		}
		seen[key] = true
		agent := LooksLikeAgent(who) || role == RoleGenerated || role == RoleAssisted
		out = append(out, Credit{Role: role, Who: who, Agent: agent})
	}
	return out
}

// HasAgent reports that at least one credited identity looks like an agent.
func (c CommitInfo) HasAgent() bool {
	for _, cr := range c.Credits() {
		if cr.Agent {
			return true
		}
	}
	return false
}
