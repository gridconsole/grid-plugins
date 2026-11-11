package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

const keepLink = `{"to":"job:T-01/J-01","rel":"decided-by","summary":"The source job.","why":"Explains why these checks were chosen."}`

func (h *harness) keepRecord(t *testing.T, name string) brain.Entry {
	t.Helper()
	code := h.run("keep", "--level", "home", "--name", name, "--topic", "decisions", "--description", "Before changing checks.", "--summary", "Run the checks.", "--source", "job:T-01/J-01", "--body", "# Checks\nRun make test.\n", "--link", keepLink, "--json")
	if code != 0 {
		t.Fatalf("keep record exit %d: %s", code, &h.err)
	}
	var entry brain.Entry
	if err := json.Unmarshal(h.out.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestKeepRecordsAProposalWithLinksAndReasons(t *testing.T) {
	h := newHarness(t)
	e := h.keepRecord(t, "checks")
	if e.Note.Status != "proposed" || len(e.Note.Links) != 1 || e.Note.Links[0].Why == "" || e.Note.Links[0].To != "job:T-01/J-01" {
		t.Fatalf("proposal: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(h.env.GridHome, "brain-proposals", "checks.md")); err != nil {
		t.Fatalf("proposal file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")); err == nil {
		t.Fatal("a recorded proposal must not be a note yet")
	}
	// A link with no reason is refused, so a proposal never carries a bare link.
	code := h.run("keep", "--level", "home", "--name", "bare", "--topic", "decisions", "--description", "Before changing checks.", "--summary", "Run the checks.", "--source", "job:T-01/J-01", "--body", "x", "--link", `{"to":"job:T-01/J-01","rel":"decided-by","summary":"The source job."}`)
	if code == 0 {
		t.Fatal("link without a reason accepted")
	}
	// Bare keep still lists, and lists the new proposal.
	if code := h.run("keep", "--level", "home"); code != 0 || !strings.Contains(h.out.String(), "checks") {
		t.Fatalf("list: %d %s %s", code, &h.out, &h.err)
	}
}

func TestKeepAcceptWritesALinkedNote(t *testing.T) {
	h := newHarness(t)
	h.keepRecord(t, "checks")
	if code := h.run("keep", "accept", "checks", "--level", "home", "--json"); code != 0 {
		t.Fatalf("keep accept exit %d: %s", code, &h.err)
	}
	if code := h.run("show", "checks", "--level", "home", "--json"); code != 0 {
		t.Fatalf("show: %d %s", code, &h.err)
	}
	var e brain.Entry
	if err := json.Unmarshal(h.out.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Note.Status != "accepted" || len(e.Note.Links) != 1 || e.Note.Links[0].Why == "" {
		t.Fatalf("accepted note: %+v", e.Note)
	}
	if _, err := os.Stat(filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")); err != nil {
		t.Fatalf("note file: %v", err)
	}
}

func TestKeepRefuseRecordsTheReasonAndWritesNoNote(t *testing.T) {
	h := newHarness(t)
	h.keepRecord(t, "checks")
	if code := h.run("keep", "refuse", "checks", "--level", "home", "--why", "Already covered by another note."); code != 0 {
		t.Fatalf("keep refuse exit %d: %s", code, &h.err)
	}
	if _, err := os.Stat(filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")); err == nil {
		t.Fatal("a refused proposal became a note")
	}
	if code := h.run("show", "checks", "--level", "home", "--proposal", "--json"); code != 0 || !strings.Contains(h.out.String(), "rejected") || !strings.Contains(h.out.String(), "Already covered by another note.") {
		t.Fatalf("refusal not recorded: %d %s %s", code, &h.out, &h.err)
	}
	// Refusing without a why still records a refusal.
	h.keepRecord(t, "second")
	if code := h.run("keep", "refuse", "second", "--level", "home"); code != 0 {
		t.Fatalf("refuse without why: %d %s", code, &h.err)
	}
}
