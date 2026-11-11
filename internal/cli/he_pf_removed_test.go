package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// hePfRefused runs args and asserts the unknown-sub answer: exit 2 and the message
// on stderr. With --json the same refusal is also the invalid_argument envelope on
// stdout; without it stdout stays empty.
func hePfRefused(t *testing.T, h *harness, cwd, message string, args ...string) {
	t.Helper()
	code := h.run(cwd, args...)
	asJSON := false
	for _, a := range args {
		asJSON = asJSON || a == "--json"
	}
	envelope := strings.Contains(h.out.String(), `"code":"invalid_argument"`) && strings.Contains(h.out.String(), "unknown")
	if code != 2 || !strings.Contains(h.errb.String(), message) || envelope != asJSON || (!asJSON && h.out.Len() != 0) {
		t.Fatalf("%v: exit %d, want 2 with %q\nstdout: %s\nstderr: %s", args, code, message, h.out, h.errb)
	}
}

// TestHePfRemovedSubs holds the removal of `grid plan diff` and `grid feed fold`
// (docs/needed.md:41): both are unknown subs, and `plan write` and `feed state`
// still answer.
func TestHePfRemovedSubs(t *testing.T) {
	t.Run("plan diff is an unknown action", func(t *testing.T) {
		h := newHarness(t)
		task := planTask(h)
		const message = `plan: unknown action "diff"; use check, hash, items, write or migrate`
		hePfRefused(t, h, h.tmp, message, "plan", "diff", "3", "4", "--task", task, "--json")
		hePfRefused(t, h, h.tmp, message, "plan", "diff", "--help")
		if code := h.run(h.tmp, "plan"); code != 2 || strings.Contains(h.errb.String(), "diff") || !strings.Contains(h.errb.String(), "grid plan write <file>") {
			t.Fatalf("plan usage: exit %d: %s", code, h.errb)
		}
		if code := h.run(h.tmp, "plan", "--help"); code != 0 || strings.Contains(h.out.String(), "diff") {
			t.Fatalf("plan --help: exit %d: %s", code, h.out)
		}
	})

	t.Run("feed fold is an unknown command", func(t *testing.T) {
		h, task := feedHarness(t)
		const message = `grid feed: unknown command "fold"`
		hePfRefused(t, h, task, message, "feed", "fold")
		hePfRefused(t, h, task, message, "feed", "fold", "--json")
		if code := h.run(task, "feed"); code != 2 || strings.Contains(h.errb.String(), "fold") || !strings.Contains(h.errb.String(), "usage: grid feed post|read|state|types|handover [flags]") {
			t.Fatalf("feed usage: exit %d: %s", code, h.errb)
		}
		if code := h.run(task, "feed", "--help"); code != 0 || strings.Contains(h.out.String(), "fold") {
			t.Fatalf("feed --help: exit %d: %s", code, h.out)
		}
	})

	t.Run("plan write still answers", func(t *testing.T) {
		h := newHarness(t)
		task := planTask(h)
		h.write(filepath.Join(h.tmp, "draft.md"), planFixture(t, "revised.md"))
		if code := h.run(h.tmp, "plan", "write", "draft.md", "--task", task); code != 0 || !strings.Contains(h.out.String(), "plan.md: revision 4") {
			t.Fatalf("plan write: exit %d: %s %s", code, h.out, h.errb)
		}
	})

	t.Run("feed state still answers", func(t *testing.T) {
		h, task := feedHarness(t)
		if code := h.run(task, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"example"}`, "--json"); code != 0 {
			t.Fatalf("feed post: exit %d: %s", code, h.errb)
		}
		if code := h.run(task, "feed", "state", "--json"); code != 0 || !strings.Contains(h.out.String(), `"by_ref"`) {
			t.Fatalf("feed state: exit %d: %s %s", code, h.out, h.errb)
		}
	})
}
