// Package brain stores Markdown knowledge and proposals outside repositories.
package brain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Origin is supplied by the caller's session or authenticated source, never by
// note text. Unknown imports must use external; local CLI sessions use agent.
type Origin struct {
	Kind string             `json:"kind"`
	ID   string             `json:"id"`
	Tier contract.TrustTier `json:"tier"`
}

// Link keeps the destination's summary with the reason to follow this edge.
type Link struct {
	To      string `json:"to"`
	Rel     string `json:"rel"`
	Summary string `json:"summary"`
	Why     string `json:"why"`
}

type Note struct {
	Name         string `json:"name"`
	Topic        string `json:"topic"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	Summary      string `json:"summary"`
	Updated      string `json:"updated"`
	Source       string `json:"source"`
	SupersededBy string `json:"superseded_by"`
	Links        []Link `json:"links"`
	Level        string `json:"level"`
	Status       string `json:"status"`
	Reason       string `json:"reason"`
	Origin       Origin `json:"origin"`
	Body         string `json:"body"`
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidName checks a single portable path component.
func ValidName(s string) bool {
	return len(s) <= 100 && namePattern.MatchString(s) && !strings.Contains(s, "..")
}

var relations = strings.Fields("depends-on explains decided-by supersedes duplicates related blocks continues validates same-files uses owns decides approves knows")

func line(s string) bool { return !strings.ContainsAny(s, "\r\n\x00") }
func size(s string) int  { return utf8.RuneCountInString(s) }

// Validate checks the common note schema, not a config topic's executable schema.
func (n Note) Validate() error {
	if !ValidName(n.Name) || !ValidName(n.Topic) {
		return fmt.Errorf("name and topic must be plain names")
	}
	if n.Level != "home" && n.Level != "project" {
		return fmt.Errorf("level must be home or project")
	}
	if n.Status != "proposed" && n.Status != "accepted" && n.Status != "rejected" {
		return fmt.Errorf("status must be proposed, accepted or rejected")
	}
	if strings.TrimSpace(n.Type) == "" || !line(n.Type) {
		return fmt.Errorf("type is required on one line")
	}
	if strings.TrimSpace(n.Description) == "" || size(n.Description) > 200 || !line(n.Description) {
		return fmt.Errorf("description must be 1 to 200 characters on one line")
	}
	if err := summary(n.Summary); err != nil {
		return err
	}
	if strings.TrimSpace(n.Source) == "" || !line(n.Source) {
		return fmt.Errorf("source must name a task, Job or person on one line")
	}
	if !line(n.Reason) || !line(n.Updated) || !line(n.SupersededBy) || !line(n.Origin.ID) || strings.TrimSpace(n.Origin.ID) == "" {
		return fmt.Errorf("metadata must contain single-line values and an origin id")
	}
	switch n.Origin.Kind {
	case "session", "job", "server", "import":
	default:
		return fmt.Errorf("origin kind must be session, job, server or import")
	}
	switch n.Origin.Tier {
	case contract.PersonTeam, contract.PersonGuest, contract.Agent, contract.External:
	default:
		return fmt.Errorf("origin tier is required")
	}
	for _, l := range n.Links {
		if err := l.Validate(); err != nil {
			return err
		}
	}
	if strings.ContainsRune(n.Body, 0) {
		return fmt.Errorf("body contains a NUL character")
	}
	lines := strings.Split(strings.TrimRight(n.Body, "\n"), "\n")
	if len(lines) > 300 {
		return fmt.Errorf("note exceeds 300 body lines")
	}
	if len(lines) > 60 && !hasContents(lines) {
		return fmt.Errorf("note over 60 body lines needs a ## Contents list")
	}
	return nil
}

func hasContents(lines []string) bool {
	inside := false
	for _, l := range lines {
		if l == "## Contents" {
			inside = true
			continue
		}
		if inside && strings.HasPrefix(l, "#") {
			return false
		}
		if inside && (strings.HasPrefix(strings.TrimSpace(l), "- ") || strings.HasPrefix(strings.TrimSpace(l), "* ")) {
			return true
		}
	}
	return false
}

func summary(s string) error {
	if strings.TrimSpace(s) == "" || size(s) > 300 || strings.Count(s, "\n") > 2 || strings.ContainsAny(s, "\r\x00") {
		return fmt.Errorf("summary must be 1 to 3 lines and at most 300 characters")
	}
	return nil
}

func (l Link) Validate() error {
	base, anchor, anchored := strings.Cut(l.To, "#")
	kind, id, ok := strings.Cut(base, ":")
	parts := strings.Split(id, "/")
	valid := ok && (!anchored || ValidName(anchor))
	switch kind {
	case "brain":
		valid = valid && len(parts) == 2 && (parts[0] == "home" || parts[0] == "project") && ValidName(parts[1])
	case "job":
		valid = valid && len(parts) == 2 && ValidName(parts[0]) && ValidName(parts[1])
	case "person":
		valid = valid && ValidName(id)
	default:
		valid = false
	}
	if !valid {
		return fmt.Errorf("link target must be brain:<level>/<id>, job:<task>/<job> or person:<id>, optionally with #anchor")
	}
	known := false
	for _, rel := range relations {
		known = known || rel == l.Rel
	}
	if !known {
		return fmt.Errorf("unknown link relation %q", l.Rel)
	}
	why := strings.TrimSpace(l.Why)
	if !line(why) || size(why) < 8 || size(why) > 160 || strings.EqualFold(why, "see also") || strings.EqualFold(why, "related") || strings.EqualFold(why, parts[len(parts)-1]) || strings.EqualFold(why, l.To) {
		return fmt.Errorf("link why must explain the benefit in 8 to 160 characters")
	}
	return summary(l.Summary)
}

// Secret detection is deliberately conservative and never includes a matched
// value in an error. It is a local gate, not a complete credential detector.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)https?://[^\s/@:]+:[^\s/@]+@`),
	regexp.MustCompile(`(?im)(?:password|passwd|token|api[_-]?key|secret|authorization)\s*["']?\s*[:=]\s*\S+`),
	regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|AKIA[A-Z0-9]{16}|sk-[A-Za-z0-9_-]{20,})\b`),
}

func hasSecret(n Note) bool {
	b, _ := Encode(n)
	for _, p := range secretPatterns {
		if p.Match(b) {
			return true
		}
	}
	return false
}
