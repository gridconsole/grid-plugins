package brain

import (
	"reflect"
	"strings"
	"testing"
)

func sample() Note {
	return Note{
		Name: "checks", Topic: "conventions", Type: "reference",
		Description: "Read before changing the checks.", Summary: "Run the project checks before delivery.",
		Source: "job:T-01/J-01", Level: "project", Status: "proposed",
		Origin: Origin{Kind: "job", ID: "T-01/J-01", Tier: "agent"},
		Links:  []Link{{To: "job:T-01/J-01", Rel: "decided-by", Summary: "The job that established the checks.", Why: "Explains which checks cover this change."}},
		Body:   "# Checks\n\nRun the project checks.\n",
	}
}

func TestMarkdownRoundTrip(t *testing.T) {
	n := sample()
	n.Description = `Read "quoted" text: # and [brackets].`
	n.Summary = "First line.\nSecond line."
	b, err := Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(b)
	if err != nil || !reflect.DeepEqual(got, n) {
		t.Fatalf("round trip: %#v, %v", got, err)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownHandwritten(t *testing.T) {
	n, err := Parse([]byte("---\nname: checks\nsummary: |-\n  First line.\n  Second line.\nlinks:\n  - to: 'brain:project/checks#commands'\n    rel: explains\n    summary: The checks.\n    why: Explains how to run the checks.\norigin:\n  kind: job\n  id: T-01/J-01\n  tier: agent\n---\nBody\n"))
	if err != nil || n.Summary != "First line.\nSecond line." || len(n.Links) != 1 || n.Origin.Tier != "agent" {
		t.Fatalf("parse: %#v, %v", n, err)
	}
	if err := n.Links[0].Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedMarkdown(t *testing.T) {
	for _, input := range []string{
		"name: x", "---\nname: x", "---\nname: a\nname: b\n---\n",
		"---\nname: &anchor x\n---\n", "---\nunknown: x\n---\n",
		"---\nlinks: [{\"to\":\"person:example\",\"extra\":true}]\n---\n",
	} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("accepted malformed front matter %q", input)
		}
	}
}

func TestNoteValidation(t *testing.T) {
	for name, change := range map[string]func(*Note){
		"traversal":    func(n *Note) { n.Name = "../escape" },
		"no source":    func(n *Note) { n.Source = "" },
		"summary":      func(n *Note) { n.Summary = strings.Repeat("x", 301) },
		"description":  func(n *Note) { n.Description = strings.Repeat("x", 201) },
		"status":       func(n *Note) { n.Status = "active" },
		"tier":         func(n *Note) { n.Origin.Tier = "unknown" },
		"why":          func(n *Note) { n.Links[0].Why = "see also" },
		"link summary": func(n *Note) { n.Links[0].Summary = "" },
		"relation":     func(n *Note) { n.Links[0].Rel = "informs" },
		"target":       func(n *Note) { n.Links[0].To = "brain:home/../escape" },
		"contents":     func(n *Note) { n.Body = strings.Repeat("line\n", 61) },
		"hard cap":     func(n *Note) { n.Body = "## Contents\n" + strings.Repeat("line\n", 300) },
	} {
		t.Run(name, func(t *testing.T) {
			n := sample()
			change(&n)
			if err := n.Validate(); err == nil {
				t.Fatal("invalid note passed")
			}
		})
	}
}

func TestSecretDetection(t *testing.T) {
	for _, text := range []string{"password: example-value", "https://example:example@example.com/path", "-----BEGIN PRIVATE KEY-----"} {
		n := sample()
		n.Body = text
		if !hasSecret(n) {
			t.Errorf("missed secret shape %q", text)
		}
	}
	if hasSecret(sample()) {
		t.Fatal("ordinary note detected as secret")
	}
}

func TestLongNoteRequiresContentsList(t *testing.T) {
	n := sample()
	n.Body = "## Contents\n" + strings.Repeat("Text.\n", 65)
	if err := n.Validate(); err == nil {
		t.Fatal("heading without contents list accepted")
	}
	n.Body = "## Contents\n- [Checks](#checks)\n\n## Checks {#checks}\n" + strings.Repeat("Text.\n", 65)
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
}
