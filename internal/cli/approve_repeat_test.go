package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

// A person with no lease presses grid approve twice for the same plan, putting
// task.md back to waiting (with its ulid line) between the presses, as the app's
// real-plugin test does. The second press returns the first receipt and writes
// nothing: approval.json is byte-identical, history.md has one approve line and
// the Feed has one answer for the plan Job.
func TestApproveRepeatByAPersonWithoutLeaseWritesAndPostsNothing(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	h.person()
	taskMD, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	press := func() string {
		t.Helper()
		if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--json"); code != 0 {
			t.Fatalf("approve: exit %d: %s", code, h.errb)
		}
		return h.out.String()
	}
	firstOut := press()
	receipt, err := os.ReadFile(filepath.Join(path, "approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "task.md"), taskMD, 0o644); err != nil {
		t.Fatal(err)
	}
	repeatOut := press()
	again, err := os.ReadFile(filepath.Join(path, "approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(receipt, again) {
		t.Fatalf("repeat approve wrote: approval.json\nfirst:  %s\nrepeat: %s", receipt, again)
	}
	if firstOut != repeatOut {
		t.Fatalf("repeat approve --json differs:\nfirst:  %s\nrepeat: %s", firstOut, repeatOut)
	}
	history, err := os.ReadFile(filepath.Join(path, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(history), `"what":"approve"`); n != 1 {
		t.Fatalf("history.md has %d approve lines, want 1:\n%s", n, history)
	}
	if answers := arAnswers(t, store); len(answers) != 1 {
		t.Fatalf("the Feed has %d answers for the plan Job, want 1: %+v", len(answers), answers)
	}
}
