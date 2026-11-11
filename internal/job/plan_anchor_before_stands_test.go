package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

const fdAnchorRev3 = "# Plan, rev 1 · 000000\n\n## Goal {#goal}\nShip the thing.\nKeep it small.\n\n## Steps {#steps}\n- [ ] 1 Build it\n  kind: build\n  jobs: []\n\n## Where it stands\n\nWaiting for approval.\n"

// Values computed on main (06047047) for fdAnchorRev3: the approval must not move.
const (
	fdAnchorRev3Digest = "8d8cf1a9d9bfd6bdcb01d689496fbb152de07ac41eb8ef8a2af94a3741f10c4a"
	fdAnchorRev3Hash   = "d902f6"
)

func fdAnchorApproved(t *testing.T, text string) jobtest.Home {
	t.Helper()
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	if err := os.WriteFile(filepath.Join(h.Task, "plan.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	return h
}

func fdAnchorResolve(h jobtest.Home, pointer string) (Source, error) {
	return (&Files{Task: h.Task}).Resolve(pointer)
}

func TestFdAnchorStepsBeforeStands(t *testing.T) {
	h := fdAnchorApproved(t, fdAnchorRev3)
	for _, pointer := range []string{"plan.md#steps", "task:plan.md#steps"} {
		s, err := fdAnchorResolve(h, pointer)
		if err != nil {
			t.Fatalf("%s: %v", pointer, err)
		}
		if !strings.Contains(s.Text, "- [ ] 1 Build it") || !strings.Contains(s.Text, "jobs: []") {
			t.Errorf("%s: text lacks the step: %q", pointer, s.Text)
		}
		if strings.Contains(s.Text, "Waiting for approval") || strings.Contains(s.Text, "Where it stands") {
			t.Errorf("%s: text holds Where-it-stands prose: %q", pointer, s.Text)
		}
	}
}

func TestFdAnchorGoalBeforeStands(t *testing.T) {
	text := "# Plan, rev 1 · 000000\n\n## Goal {#goal}\nShip the thing.\nKeep it small.\n\n## Where it stands\n\nWaiting for approval.\n"
	h := fdAnchorApproved(t, text)
	s, err := fdAnchorResolve(h, "plan.md#goal")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.Text, "Keep it small.") || strings.Contains(s.Text, "Waiting for approval") {
		t.Errorf("goal text: %q", s.Text)
	}
}

func TestFdAnchorCustomIDBeforeStands(t *testing.T) {
	text := "# Plan, rev 1 · 000000\n\n## Scope {#my-scope}\nOnly this.\n\n## Where it stands\nLive.\n"
	h := fdAnchorApproved(t, text)
	s, err := fdAnchorResolve(h, "plan.md#my-scope")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.Text, "Only this.") || strings.Contains(s.Text, "Live.") {
		t.Errorf("scope text: %q", s.Text)
	}
}

func TestFdAnchorTrailingBlankLinesNoStands(t *testing.T) {
	text := "# Plan, rev 1 · 000000\n\n## Goal\nShip it.\n\n## Steps {#steps}\n- [ ] 1 Build it\n  kind: build\n  jobs: []\n\n"
	h := fdAnchorApproved(t, text)
	s, err := fdAnchorResolve(h, "plan.md#steps")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.Text, "- [ ] 1 Build it") {
		t.Errorf("steps text: %q", s.Text)
	}
}

func TestFdAnchorStandsStillRefused(t *testing.T) {
	h := fdAnchorApproved(t, fdAnchorRev3)
	// Line 12 is the heading, line 14 the "Waiting for approval." prose.
	for _, pointer := range []string{"plan.md#where-it-stands", "plan.md#L14", "plan.md#L13-L14", "plan.md#L10-L14"} {
		s, err := fdAnchorResolve(h, pointer)
		if err == nil || !strings.Contains(err.Error(), "outside the approval digest") || s.Text != "" {
			t.Errorf("%s: want refusal without text, got err=%v text=%q", pointer, err, s.Text)
		}
	}
}

func TestFdAnchorApprovalUnchanged(t *testing.T) {
	if got := plan.Digest(fdAnchorRev3); got != fdAnchorRev3Digest {
		t.Errorf("Digest = %s, want %s", got, fdAnchorRev3Digest)
	}
	if got := plan.Hash(fdAnchorRev3); got != fdAnchorRev3Hash {
		t.Errorf("Hash = %s, want %s", got, fdAnchorRev3Hash)
	}
}
