package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

func TestBrainCommandRegistered(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "brain", "index", "--level", "home", "--read-only", "--json"); code != 0 {
		t.Fatalf("brain command exit %d: %s", code, h.errb)
	}
	if !json.Valid(h.out.Bytes()) {
		t.Fatalf("invalid brain JSON: %s", h.out)
	}
}

// brainWithTwoVersions publishes home/decisions/checks twice (v2 through merge-into).
func brainWithTwoVersions(t *testing.T, h *harness) {
	t.Helper()
	if code := h.run(h.home, "brain", "propose", "--level", "home", "--name", "checks", "--topic", "decisions", "--description", "Before changing checks.", "--summary", "Run the checks.", "--source", "job:T-01/J-01", "--body", "# Checks\n"); code != 0 {
		t.Fatalf("propose: %d %s", code, h.errb)
	}
	if code := h.run(h.home, "brain", "accept", "checks", "--level", "home"); code != 0 {
		t.Fatalf("accept: %d %s", code, h.errb)
	}
	dir := t.TempDir()
	b, err := brain.Encode(brain.Note{Name: "second", Topic: "decisions", Type: "reference", Description: "Before changing checks.",
		Summary: "Run the checks.", Source: "job:T-01/J-01", Level: "home", Status: "proposed",
		Origin: brain.Origin{Kind: "job", ID: "T-01/J-01"}, Links: []brain.Link{}, Body: "# Second\n"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(dir, "brain-proposals", "second.md"), string(b))
	store := brain.Store{GridHome: h.gridHome}
	if _, err := store.DecideTask(brain.TaskRef{ID: "T-01", Path: dir}, "second", "merge-into", "brain:home/checks", "", func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestBrainChangesAndRevertByRole(t *testing.T) {
	for _, c := range []struct {
		role    string
		allowed bool
	}{
		{"", true}, {"owner", true}, {"member", true}, {"guest", false}, {"agent", false},
	} {
		t.Run("role="+c.role, func(t *testing.T) {
			h := newHarness(t)
			brainWithTwoVersions(t, h)
			h.env.Getenv = func(key string) string { return map[string]string{"GRID_ROLE": c.role}[key] }
			entry := filepath.Join(h.gridHome, "knowledge", "decisions", "checks.md")
			before, err := os.ReadFile(entry)
			if err != nil {
				t.Fatal(err)
			}
			code := h.run(h.home, "brain", "changes", "--level", "home")
			if c.allowed && (code != 0 || !strings.Contains(h.out.String(), " accepted decisions/checks v2 ")) {
				t.Fatalf("changes: %d %q %q", code, h.out, h.errb)
			}
			if !c.allowed && (code != 3 || h.out.Len() != 0 || h.errb.String() != "grid brain changes: caller role is not allowed to read brain changes\n") {
				t.Fatalf("changes: %d %q %q", code, h.out, h.errb)
			}
			code = h.run(h.home, "brain", "revert", "decisions/checks", "--to", "1", "--level", "home")
			after, err := os.ReadFile(entry)
			if err != nil {
				t.Fatal(err)
			}
			if c.allowed && (code != 0 || h.out.String() != "v3\n" || bytes.Equal(before, after)) {
				t.Fatalf("revert: %d %q %q", code, h.out, h.errb)
			}
			if !c.allowed && (code != 3 || h.out.Len() != 0 || h.errb.String() != "grid brain revert: caller role is not allowed to revert brain entries\n" || !bytes.Equal(before, after)) {
				t.Fatalf("revert: %d %q %q", code, h.out, h.errb)
			}
		})
	}
}
