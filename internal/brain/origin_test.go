package brain

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// oldNote is a note in the format written before the trust tiers were dropped:
// its origin block still carries a tier key.
func oldNote(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "note-origin-tier.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n  tier: agent\n") {
		t.Fatal("fixture lost its tier key")
	}
	return data
}

func TestOldNoteWithRetiredOriginKeyDecodesAndValidates(t *testing.T) {
	n, err := Parse(oldNote(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := n.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if n.Origin != (Origin{Kind: "job", ID: "T-01/J-01"}) {
		t.Fatalf("origin: %+v", n.Origin)
	}
	out, err := Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "tier") {
		t.Fatalf("tier key written back:\n%s", out)
	}
}

func TestOriginIsKindAndIDOnly(t *testing.T) {
	n := sample()
	if err := n.Validate(); err != nil {
		t.Fatalf("a note with no tier must validate: %v", err)
	}
	b, err := Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "tier") {
		t.Fatalf("tier written:\n%s", b)
	}
	got, err := Parse(b)
	if err != nil || !reflect.DeepEqual(got, n) {
		t.Fatalf("round trip: %#v, %v", got, err)
	}
}

func TestRetiredOriginKeyIsIgnoredWhateverItsValue(t *testing.T) {
	for _, tier := range []string{"person-team", "person-guest", "agent", "external", "unknown"} {
		n, err := Parse([]byte("---\norigin: {kind: server, id: x, tier: " + tier + "}\n---\n"))
		if err != nil || n.Origin != (Origin{Kind: "server", ID: "x"}) {
			t.Errorf("tier %s: %+v, %v", tier, n.Origin, err)
		}
	}
}

func TestOriginStaysStrictAboutOtherKeys(t *testing.T) {
	for _, origin := range []string{
		"{kind: job, id: x, extra: 1}", "{kind: job, id: x, id: y}", "{kind: job, id: x, tier: a, tier: b}", "[job, x]",
	} {
		if _, err := Parse([]byte("---\norigin: " + origin + "\n---\n")); err == nil {
			t.Errorf("accepted origin %s", origin)
		}
	}
}

func TestAcceptRewritesOldNoteWithoutRetiredKey(t *testing.T) {
	s := testStore(t)
	n := sample()
	p, err := s.Propose(n, n.Origin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.GridHome, p.Path), oldNote(t), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := s.Accept(n.Level, n.Name)
	if err != nil || e.Note.Status != "accepted" || e.Note.Origin != n.Origin {
		t.Fatalf("accept: %+v, %v", e, err)
	}
	b, err := os.ReadFile(filepath.Join(s.GridHome, e.Path))
	if err != nil || strings.Contains(string(b), "tier") {
		t.Fatalf("accepted note keeps the tier key: %s %v", b, err)
	}
}
