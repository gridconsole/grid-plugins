package brain

import (
	"strings"
	"testing"
)

// Messages the curator sees when a note or link is refused.
const (
	msgNames    = "name and topic must be plain names"
	msgLevel    = "level must be home or project"
	msgStatus   = "status must be proposed, accepted or rejected"
	msgType     = "type is required on one line"
	msgDesc     = "description must be 1 to 200 characters on one line"
	msgSummary  = "summary must be 1 to 3 lines and at most 300 characters"
	msgSource   = "source must name a task, Job or person on one line"
	msgMeta     = "metadata must contain single-line values and an origin id"
	msgOrigin   = "origin kind must be session, job, server or import"
	msgNUL      = "body contains a NUL character"
	msgLines    = "note exceeds 300 body lines"
	msgContents = "note over 60 body lines needs a ## Contents list"
	msgTarget   = "link target must be brain:<level>/<id>, job:<task>/<job> or person:<id>, optionally with #anchor"
	msgWhy      = "link why must explain the benefit in 8 to 160 characters"
)

// rulesLines returns a body of n lines; with a contents list the first two
// lines are the heading and one item.
func rulesLines(n int, contents bool) string {
	if !contents {
		return strings.Repeat("line\n", n)
	}
	return "## Contents\n- [Line](#line)\n" + strings.Repeat("line\n", n-2)
}

func TestRulesNoteValidateRefusals(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatalf("the valid note is refused: %v", err)
	}
	for _, c := range []struct {
		name   string
		change func(*Note)
		want   string
	}{
		{"name traversal", func(n *Note) { n.Name = "../escape" }, msgNames},
		{"name with slash", func(n *Note) { n.Name = "a/b" }, msgNames},
		{"name empty", func(n *Note) { n.Name = "" }, msgNames},
		{"topic with space", func(n *Note) { n.Topic = "my topic" }, msgNames},
		{"level unknown", func(n *Note) { n.Level = "team" }, msgLevel},
		{"level empty", func(n *Note) { n.Level = "" }, msgLevel},
		{"status unknown", func(n *Note) { n.Status = "active" }, msgStatus},
		{"type empty", func(n *Note) { n.Type = "  " }, msgType},
		{"type two lines", func(n *Note) { n.Type = "ref\nerence" }, msgType},
		{"description empty", func(n *Note) { n.Description = "" }, msgDesc},
		{"description 201", func(n *Note) { n.Description = strings.Repeat("x", 201) }, msgDesc},
		{"description two lines", func(n *Note) { n.Description = "a\nb" }, msgDesc},
		{"summary empty", func(n *Note) { n.Summary = "" }, msgSummary},
		{"summary 301", func(n *Note) { n.Summary = strings.Repeat("x", 301) }, msgSummary},
		{"source empty", func(n *Note) { n.Source = "" }, msgSource},
		{"source two lines", func(n *Note) { n.Source = "job:T-01/J-01\nx" }, msgSource},
		{"reason two lines", func(n *Note) { n.Reason = "a\nb" }, msgMeta},
		{"updated two lines", func(n *Note) { n.Updated = "a\nb" }, msgMeta},
		{"superseded_by two lines", func(n *Note) { n.SupersededBy = "a\nb" }, msgMeta},
		{"origin id two lines", func(n *Note) { n.Origin.ID = "a\nb" }, msgMeta},
		{"origin id empty", func(n *Note) { n.Origin.ID = "" }, msgMeta},
		{"origin id blank", func(n *Note) { n.Origin.ID = "   " }, msgMeta},
		{"origin kind unknown", func(n *Note) { n.Origin.Kind = "agent" }, msgOrigin},
		{"origin kind empty", func(n *Note) { n.Origin.Kind = "" }, msgOrigin},
		{"link target", func(n *Note) { n.Links[0].To = "url:x" }, msgTarget},
		{"link why", func(n *Note) { n.Links[0].Why = "see also" }, msgWhy},
		{"link summary", func(n *Note) { n.Links[0].Summary = "" }, msgSummary},
		{"body NUL", func(n *Note) { n.Body = "a\x00b\n" }, msgNUL},
		{"61 lines without contents", func(n *Note) { n.Body = rulesLines(61, false) }, msgContents},
		{"301 lines", func(n *Note) { n.Body = rulesLines(301, true) }, msgLines},
		{"301 lines without contents", func(n *Note) { n.Body = rulesLines(301, false) }, msgLines},
	} {
		t.Run(c.name, func(t *testing.T) {
			n := sample()
			c.change(&n)
			err := n.Validate()
			if err == nil {
				t.Fatal("invalid note passed")
			}
			if err.Error() != c.want {
				t.Fatalf("got %q, want %q", err.Error(), c.want)
			}
		})
	}
}

func TestRulesNoteValidateAccepts(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(*Note)
	}{
		{"valid note", func(n *Note) {}},
		{"level home", func(n *Note) { n.Level = "home" }},
		{"status accepted", func(n *Note) { n.Status = "accepted" }},
		{"status rejected", func(n *Note) { n.Status = "rejected" }},
		{"origin kind session", func(n *Note) { n.Origin.Kind = "session" }},
		{"origin kind server", func(n *Note) { n.Origin.Kind = "server" }},
		{"origin kind import", func(n *Note) { n.Origin.Kind = "import" }},
		{"description 200", func(n *Note) { n.Description = strings.Repeat("x", 200) }},
		{"description 200 multibyte characters", func(n *Note) { n.Description = strings.Repeat("é", 200) }},
		{"summary 300", func(n *Note) { n.Summary = strings.Repeat("x", 300) }},
		{"no links", func(n *Note) { n.Links = nil }},
		{"60 lines without contents", func(n *Note) { n.Body = rulesLines(60, false) }},
		{"61 lines with contents", func(n *Note) { n.Body = rulesLines(61, true) }},
		{"300 lines with contents", func(n *Note) { n.Body = rulesLines(300, true) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			n := sample()
			c.change(&n)
			if err := n.Validate(); err != nil {
				t.Fatalf("valid note refused: %v", err)
			}
		})
	}
}

func TestRulesHasContents(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
		want  bool
	}{
		{"dash item", []string{"## Contents", "- [A](#a)"}, true},
		{"star item", []string{"## Contents", "* [A](#a)"}, true},
		{"indented item", []string{"## Contents", "  - [A](#a)"}, true},
		{"blank line before the item", []string{"## Contents", "", "- [A](#a)"}, true},
		{"item after other text", []string{"# T", "## Contents", "Intro.", "- [A](#a)", "## A {#a}"}, true},
		{"another heading first", []string{"## Contents", "## Next", "- [A](#a)"}, false},
		{"another heading level first", []string{"## Contents", "### Sub", "- [A](#a)"}, false},
		{"heading without any item", []string{"## Contents", "Text.", "Text."}, false},
		{"no contents heading", []string{"# T", "- [A](#a)"}, false},
		{"list before the heading", []string{"- [A](#a)", "## Contents", "Text."}, false},
		{"other heading level", []string{"### Contents", "- [A](#a)"}, false},
		{"dash without a space", []string{"## Contents", "-A"}, false},
		{"empty", nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := hasContents(c.lines); got != c.want {
				t.Fatalf("hasContents(%q) = %v, want %v", c.lines, got, c.want)
			}
		})
	}
}

func TestRulesSummary(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want bool // true when accepted
	}{
		{"one line", "One line.", true},
		{"300 characters", strings.Repeat("x", 300), true},
		{"300 multibyte characters", strings.Repeat("é", 300), true},
		{"3 lines", "a\nb\nc", true},
		{"empty", "", false},
		{"blank", " \n ", false},
		{"301 characters", strings.Repeat("x", 301), false},
		{"4 lines", "a\nb\nc\nd", false},
		{"carriage return", "a\r\nb", false},
		{"NUL", "a\x00b", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := summary(c.in)
			if c.want && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !c.want && (err == nil || err.Error() != msgSummary) {
				t.Fatalf("got %v, want %q", err, msgSummary)
			}
		})
	}
}

func rulesLink(to string) Link {
	return Link{To: to, Rel: "explains", Summary: "What the destination holds.", Why: "Shows how the checks are chosen."}
}

func TestRulesLinkTargets(t *testing.T) {
	for _, to := range []string{
		"brain:home/a", "brain:project/a#sec", "job:T-01/J-2", "person:sam",
		"brain:project/a.b_c-1", "job:T-01/J-2#result", "person:sam#top",
	} {
		if err := rulesLink(to).Validate(); err != nil {
			t.Errorf("%q refused: %v", to, err)
		}
	}
	for _, to := range []string{
		"brain:team/a", "brain:home/a/b", "url:x", "brain:home/a#bad anchor",
		"", "plain", "brain:home", "brain:home/", "brain:home/../a", "brain:home/a#",
		"job:T-01", "job:T-01/J-2/x", "person:", "person:a/b", "https://example.com/x",
	} {
		err := rulesLink(to).Validate()
		if err == nil || err.Error() != msgTarget {
			t.Errorf("%q: got %v, want %q", to, err, msgTarget)
		}
	}
}

func TestRulesLinkRelation(t *testing.T) {
	for _, rel := range []string{"cites", "", "Explains", "informs"} {
		l := rulesLink("brain:home/a")
		l.Rel = rel
		want := `unknown link relation "` + rel + `"`
		if err := l.Validate(); err == nil || err.Error() != want {
			t.Errorf("rel %q: got %v, want %q", rel, err, want)
		}
	}
	for _, rel := range relations {
		l := rulesLink("brain:home/a")
		l.Rel = rel
		if err := l.Validate(); err != nil {
			t.Errorf("rel %q refused: %v", rel, err)
		}
	}
	if len(relations) == 0 {
		t.Fatal("no relations to check")
	}
}

func TestRulesLinkWhy(t *testing.T) {
	const target = "brain:home/deploy-notes"
	for _, c := range []struct{ name, to, why string }{
		{"see also", "brain:home/a", "see also"},
		{"see also padded", "brain:home/a", "  see also  "},
		{"Related", "brain:home/a", "Related"},
		{"related padded to length", "brain:home/a", "RELATED"},
		{"target name", target, "deploy-notes"},
		{"target name any case", target, "Deploy-Notes"},
		{"target name on an anchored link", target + "#sec", "deploy-notes"},
		{"target itself", target, target},
		{"target itself in any case", target, "BRAIN:home/deploy-notes"},
		{"7 characters", "brain:home/a", "abcdefg"},
		{"empty", "brain:home/a", ""},
		{"blank", "brain:home/a", "          "},
		{"161 characters", "brain:home/a", strings.Repeat("x", 161)},
		{"newline inside", "brain:home/a", "Explains the\nreason here"},
		{"carriage return inside", "brain:home/a", "Explains the\rreason here"},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := rulesLink(c.to)
			l.Why = c.why
			if err := l.Validate(); err == nil || err.Error() != msgWhy {
				t.Fatalf("got %v, want %q", err, msgWhy)
			}
		})
	}
	for _, c := range []struct{ name, to, why string }{
		{"says what the reader gains", target, "Shows which steps deploy."},
		{"20 characters", "brain:home/a", "Explains the choice."},
		{"8 characters", "brain:home/a", "abcdefgh"},
		{"160 characters", "brain:home/a", strings.Repeat("x", 160)},
		{"target name inside a longer why", target, "Read deploy-notes before a release."},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := rulesLink(c.to)
			l.Why = c.why
			if err := l.Validate(); err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
	if got := size("Explains the choice."); got != 20 {
		t.Fatalf("the 20 character case is %d characters", got)
	}
}

func TestRulesHasSecret(t *testing.T) {
	// pattern is the index into secretPatterns that must match the text.
	for _, c := range []struct {
		name    string
		text    string
		pattern int
	}{
		{"private key header", "-----BEGIN PRIVATE KEY-----", 0},
		{"rsa private key header", "-----BEGIN RSA PRIVATE KEY-----", 0},
		{"url with credentials", "see https://u:p@host/path", 1},
		{"api_key assignment", "api_key = abc", 2},
		{"password colon", "password: example-value", 2},
		{"quoted secret", `secret": "abc"`, 2},
		{"ghp token", "ghp_" + strings.Repeat("a", 20), 3},
		{"long ghp token", "ghp_" + strings.Repeat("Z9", 20), 3},
		{"aws key id", "AKIA" + strings.Repeat("A", 16), 3},
		{"sk token", "sk-" + strings.Repeat("a", 20), 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !secretPatterns[c.pattern].MatchString(c.text) {
				t.Errorf("pattern %d does not match %q", c.pattern, c.text)
			}
			n := sample()
			n.Body = c.text
			if !hasSecret(n) {
				t.Errorf("secret in the body not caught: %q", c.text)
			}
		})
	}
	if len(secretPatterns) != 4 {
		t.Errorf("%d secret patterns, the cases cover 4", len(secretPatterns))
	}
	for _, text := range []string{
		"we keep a token budget per run",
		"ghp_" + strings.Repeat("a", 19),
		"https://example.com/path and user@example.com",
		"the password policy is described in the handbook",
	} {
		n := sample()
		n.Body = text
		if hasSecret(n) {
			t.Errorf("harmless text caught: %q", text)
		}
	}
}
