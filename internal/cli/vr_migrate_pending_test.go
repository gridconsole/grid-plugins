package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vrPendWant is what a grid command says while a migration journal is open.
const vrPendWant = "A migration of ~/.absent stopped part way. Run: grid migrate --apply"

// vrPendHarness is a machine in the gap between migrate's two renames: the Grid home
// is absent and the journal sits beside it. The journal's content does not matter
// to the gate, which only asks whether the file is there.
func vrPendHarness(t *testing.T, withJournal bool) *harness {
	t.Helper()
	h := newHarness(t)
	h.env.GridHome = filepath.Join(h.home, ".absent")
	if withJournal {
		h.write(h.env.GridHome+".migrate-journal", "{}\n")
	}
	return h
}

func vrPendHomeAbsent(t *testing.T, h *harness, after string) {
	t.Helper()
	if _, err := os.Lstat(h.env.GridHome); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("after %s the Grid home exists (stat error %v)", after, err)
	}
}

func TestVrPendGridLeavesTheHomeAloneWhileAJournalIsOpen(t *testing.T) {
	t.Run("task new is refused and says to run migrate", func(t *testing.T) {
		h := vrPendHarness(t, true)
		code := h.run(h.tmp, "task", "new", "x")
		t.Logf("grid task new x: exit %d, stdout %q, stderr %q", code, h.out, h.errb)
		if code != 1 {
			t.Errorf("exit %d, want 1", code)
		}
		if !strings.Contains(h.errb.String(), "stopped part way. Run: grid migrate --apply") {
			t.Errorf("stderr %q does not say to run grid migrate --apply", h.errb)
		}
		if got := strings.TrimSpace(h.errb.String()); got != vrPendWant {
			t.Errorf("stderr %q, want %q", got, vrPendWant)
		}
		if h.out.Len() != 0 {
			t.Errorf("stdout %q, want empty", h.out)
		}
		vrPendHomeAbsent(t, h, "task new")
	})

	t.Run("a session start hook exits 0 with the one context line and makes no home", func(t *testing.T) {
		h := vrPendHarness(t, true)
		h.env.Stdin = strings.NewReader(`{"cwd":"` + h.tmp + `"}`)
		code := h.run(h.tmp, "hook", "sessionstart")
		t.Logf("grid hook sessionstart: exit %d, stdout %q, stderr %q", code, h.out, h.errb)
		want := `{"hookSpecificOutput":{"additionalContext":"` + vrPendWant + `","hookEventName":"SessionStart"}}` + "\n"
		if code != 0 || h.out.String() != want || h.errb.Len() != 0 {
			t.Errorf("exit %d, stdout %q, stderr %q; want 0, %q and nothing on stderr", code, h.out, h.errb, want)
		}
		vrPendHomeAbsent(t, h, "hook sessionstart")
	})

	t.Run("version still runs", func(t *testing.T) {
		h := vrPendHarness(t, true)
		if code := h.run(h.tmp, "version"); code != 0 {
			t.Errorf("exit %d, stderr %q; want 0", code, h.errb)
		}
		vrPendHomeAbsent(t, h, "version")
	})

	t.Run("a journal beside a home that exists refuses as well", func(t *testing.T) {
		h := vrPendHarness(t, true)
		h.mkdir(h.env.GridHome)
		h.write(filepath.Join(h.env.GridHome, "VERSION"), "1\n")
		code := h.run(h.tmp, "task", "new", "x")
		if code != 1 || strings.TrimSpace(h.errb.String()) != vrPendWant {
			t.Errorf("exit %d, stderr %q; want 1 and %q", code, h.errb, vrPendWant)
		}
	})

	t.Run("doctor is not refused and reports the migration", func(t *testing.T) {
		h := vrPendHarness(t, true)
		code := h.run(h.tmp, "doctor")
		if code != 1 || !strings.Contains(h.out.String(), "fail migration: a migration of ~/.absent stopped part way") {
			t.Errorf("exit %d, stdout %q, stderr %q", code, h.out, h.errb)
		}
	})
}

// With no journal the same commands behave as before: no refusal from the gate.
func TestVrPendNoJournalLeavesTheCommandsAsTheyWere(t *testing.T) {
	h := vrPendHarness(t, false)
	if code := h.run(h.tmp, "version"); code != 0 {
		t.Fatalf("version: exit %d, stderr %q", code, h.errb)
	}
	h.env.Stdin = strings.NewReader(`{"cwd":"` + h.tmp + `"}`)
	if code := h.run(h.tmp, "hook", "sessionstart"); code != 0 {
		t.Fatalf("hook sessionstart: exit %d, stderr %q", code, h.errb)
	}
	if h.out.Len() == 0 {
		t.Errorf("hook sessionstart said nothing with no journal; it gives the session its context")
	}
	code := h.run(h.tmp, "task", "new", "x")
	if strings.Contains(h.errb.String(), "stopped part way") {
		t.Errorf("task new (exit %d) refused with no journal: %q", code, h.errb)
	}
}
