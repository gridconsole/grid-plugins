package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
)

const proofPlan = "# Plan 1\n- [x] 1 Check result\n  proof: check\n  jobs: []\n- [ ] 2 Show the page\n  proof: screenshot\n  jobs: []\n"

func proofTask(t *testing.T) (*harness, string, string) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	tk := taskFixture(t, h)
	h.write(filepath.Join(tk.Path, "plan.md"), proofPlan)
	return h, tk.ID, tk.Path
}

func TestProofAddText(t *testing.T) {
	h, id, dir := proofTask(t)
	if code := h.run(h.home, "proof", "add", "1", "--text", "All checks pass", "--note", "make test", "--by", "alex", "--task", id, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var r proof.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatalf("json %q: %v", h.out.String(), err)
	}
	if r.Kind != "check" || r.By != "alex" || r.Note != "make test" || !strings.HasPrefix(r.File, "proof/i-1/") || !strings.HasSuffix(r.File, ".txt") {
		t.Errorf("record %+v", r)
	}
	saved, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(r.File)))
	if err != nil || string(saved) != "All checks pass" {
		t.Errorf("saved proof %q, %v", saved, err)
	}
	// Plain output prints the saved path.
	if code := h.run(h.home, "proof", "add", "1", "--text", "again", "--task", id); code != 0 {
		t.Fatalf("plain exit %d: %s", code, h.errb)
	}
	if got := strings.TrimSpace(h.out.String()); !strings.HasPrefix(got, "proof/i-1/") {
		t.Errorf("plain stdout %q", got)
	}
}

func TestProofAddFile(t *testing.T) {
	h, id, dir := proofTask(t)
	h.write(filepath.Join(h.home, "shot.png"), "png-bytes")
	// A relative file resolves against the caller's folder.
	if code := h.run(h.home, "proof", "add", "2", "shot.png", "--kind", "screenshot", "--task", id, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var r proof.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil || r.Kind != "screenshot" || !strings.HasSuffix(r.File, ".png") {
		t.Fatalf("record %+v, %v", r, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(r.File))); err != nil || string(b) != "png-bytes" {
		t.Errorf("saved %q, %v", b, err)
	}
}

func TestProofIndexLists(t *testing.T) {
	h, id, _ := proofTask(t)
	if code := h.run(h.home, "proof", "index", "--task", id); code != 0 {
		t.Fatalf("empty index exit %d: %s", code, h.errb)
	}
	if got, want := h.out.String(), "i-1: check proof missing\ni-2: screenshot proof missing\n"; got != want {
		t.Errorf("before add %q, want %q", got, want)
	}
	if code := h.run(h.home, "proof", "add", "1", "--text", "ok", "--task", id); code != 0 {
		t.Fatalf("add exit %d: %s", code, h.errb)
	}
	if code := h.run(h.home, "proof", "index", "--task", id); code != 0 {
		t.Fatalf("index exit %d: %s", code, h.errb)
	}
	if got, want := h.out.String(), "i-1: check proof saved\ni-2: screenshot proof missing\n"; got != want {
		t.Errorf("after add %q, want %q", got, want)
	}
	if code := h.run(h.home, "proof", "index", "--task", id, "--json"); code != 0 {
		t.Fatalf("json exit %d: %s", code, h.errb)
	}
	var idx proof.Index
	if err := json.Unmarshal(h.out.Bytes(), &idx); err != nil || len(idx.Items) != 2 {
		t.Fatalf("index %q: %v", h.out.String(), err)
	}
	if !idx.Items[0].Proven || len(idx.Items[0].Evidence) != 1 || idx.Items[1].Proven {
		t.Errorf("items %+v", idx.Items)
	}
}

func TestProofMissingFile(t *testing.T) {
	h, id, dir := proofTask(t)
	if code := h.run(h.home, "proof", "add", "1", "no-such-file.log", "--kind", "log", "--task", id); code == 0 {
		t.Fatal("missing proof file succeeded")
	}
	if h.errb.Len() == 0 || h.out.Len() != 0 {
		t.Errorf("stdout %q stderr %q", h.out.String(), h.errb.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "proof", "i-1", "proof.json")); !os.IsNotExist(err) {
		t.Errorf("a failed add left proof.json: %v", err)
	}
	// An unknown plan item and a missing text/file are refused the same way.
	if code := h.run(h.home, "proof", "add", "9", "--text", "x", "--task", id); code == 0 || !strings.Contains(h.errb.String(), "unknown plan item") {
		t.Errorf("unknown item: exit %d %q", code, h.errb.String())
	}
	if code := h.run(h.home, "proof", "add", "1", "--task", id); code == 0 || !strings.Contains(h.errb.String(), "supply one file or --text") {
		t.Errorf("no evidence: exit %d %q", code, h.errb.String())
	}
}

func TestProofUsage(t *testing.T) {
	h, id, _ := proofTask(t)
	for name, args := range map[string][]string{
		"bad op":           {"proof", "bogus"},
		"add no item":      {"proof", "add", "--task", id},
		"add three":        {"proof", "add", "1", "a", "b", "--task", id},
		"index an operand": {"proof", "index", "1", "--task", id},
	} {
		if code := h.run(h.home, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
}

func TestProofGuestCall(t *testing.T) {
	h, id, dir := proofTask(t)
	h.env.Getenv = func(key string) string {
		if key == "GRID_ROLE" {
			return "guest"
		}
		return ""
	}
	for _, args := range [][]string{
		{"proof", "add", "1", "--text", "guest proof", "--task", id},
		{"proof", "index", "--task", id},
	} {
		if code := h.run(h.home, args...); code != 1 {
			t.Fatalf("%v: guest exit %d, want 1: %s", args, code, h.errb)
		}
		if !strings.Contains(h.errb.String(), "task not found") || h.out.Len() != 0 {
			t.Errorf("%v: stdout %q stderr %q", args, h.out.String(), h.errb.String())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "proof")); !os.IsNotExist(err) {
		t.Errorf("a guest call left a proof folder: %v", err)
	}
}
