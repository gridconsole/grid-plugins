package braincmd

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// A Job agent runs with GRID_ROLE=worker. brain-keeper tells the curator to run
// `grid brain changes`, `supersede` and `revert`, so those three admit a worker;
// a guest and any other role stay refused. write, rename and drop keep their rule.

const rfCurReason = "moved to the v2 pipeline"

// rfCurSeed is a brain with decisions/checks at three versions and two accepted
// home notes, deploy and deploy-v2, written as the empty local role.
func rfCurSeed(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.threeVersions(t)
	h.svNotes(t, "deploy", "deploy-v2")
	return h
}

// rfCurCalls is one call of each verb, against the notes rfCurSeed wrote, with
// the stderr line its refusal prints (without the newline).
var rfCurCalls = []struct {
	verb string
	args []string
	line string
}{
	{"changes", []string{"changes", "--level", "home"}, "grid brain changes: caller role is not allowed to read brain changes"},
	{"supersede", []string{"supersede", "deploy", "--by", "deploy-v2", "--reason", rfCurReason}, "grid brain supersede: caller role is not allowed to supersede brain entries"},
	{"revert", []string{"revert", "decisions/checks", "--to", "1", "--level", "home", "--reason", "put back after review"}, "grid brain revert: caller role is not allowed to revert brain entries"},
}

// A worker gets exit 0 and the effect from each of the three verbs.
func TestRfCurWorkerVerbs(t *testing.T) {
	t.Run("changes", func(t *testing.T) {
		h := rfCurSeed(t)
		// It lists what it lists for the local owner, byte for byte.
		if code := h.run("changes", "--level", "home", "--json"); code != 0 {
			t.Fatalf("changes as the local role: %d %s", code, &h.err)
		}
		owner := h.out.String()
		h.env.Role = "worker"
		if code := h.run("changes", "--level", "home", "--json"); code != 0 || h.err.Len() != 0 {
			t.Fatalf("changes as worker: exit %d, stdout %q, stderr %q, want exit 0", code, &h.out, &h.err)
		}
		if h.out.String() != owner {
			t.Fatalf("changes as worker differs from the owner's:\n%s\nwant\n%s", &h.out, owner)
		}
		seen := map[string]bool{}
		for _, c := range h.changes(t).Changes {
			seen[c.Name] = true
		}
		for _, name := range []string{"checks", "deploy", "deploy-v2"} {
			if !seen[name] {
				t.Fatalf("changes as worker does not list %s: %s", name, owner)
			}
		}
	})

	t.Run("supersede", func(t *testing.T) {
		h := rfCurSeed(t)
		h.env.Role = "worker"
		if code := h.run("supersede", "deploy", "--by", "deploy-v2", "--reason", rfCurReason); code != 0 {
			t.Fatalf("supersede as worker: exit %d, stdout %q, stderr %q, want exit 0", code, &h.out, &h.err)
		}
		if got := h.svRead(t, "deploy"); !strings.Contains(got, "superseded_by: brain:home/deploy-v2") {
			t.Fatalf("the old note carries no superseded_by:\n%s", got)
		}
		for _, c := range h.changes(t).Changes {
			if c.Status == "superseded" && c.Name == "deploy" && strings.Contains(c.Reason, rfCurReason) {
				return
			}
		}
		t.Fatalf("the log has no superseded line for deploy:\n%s", &h.out)
	})

	t.Run("revert", func(t *testing.T) {
		h := rfCurSeed(t)
		h.env.Role = "worker"
		if h.published(t) == h.storedCopy(t, 1) {
			t.Fatal("the seed already publishes version 1")
		}
		if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home", "--reason", "put back after review"); code != 0 {
			t.Fatalf("revert as worker: exit %d, stdout %q, stderr %q, want exit 0", code, &h.out, &h.err)
		}
		if h.published(t) != h.storedCopy(t, 1) {
			t.Fatalf("the entry does not hold version 1's bytes:\n%s\nwant\n%s", h.published(t), h.storedCopy(t, 1))
		}
		for _, c := range h.changes(t).Changes {
			if c.Status == "reverted" && c.Name == "checks" && c.Reason == "revert to v1: put back after review" {
				return
			}
		}
		t.Fatalf("the log has no reverted line for checks:\n%s", &h.out)
	})
}

// A guest, and any role the policy does not name, still gets exit 3 and the
// refusal line, writes nothing, and prints nothing on stdout without --json.
func TestRfCurGuestStillRefused(t *testing.T) {
	for _, role := range []string{"guest", "other", "agent", "Worker", "worker,guest"} {
		for _, v := range rfCurCalls {
			t.Run(role+"/"+v.verb, func(t *testing.T) {
				h := rfCurSeed(t)
				h.env.Role = role
				before := svTree(t, h)
				code := h.run(v.args...)
				if code != 3 || h.out.Len() != 0 || h.err.String() != v.line+"\n" {
					t.Fatalf("exit=%d stdout=%q stderr=%q, want exit 3, no stdout, stderr %q", code, &h.out, &h.err, v.line+"\n")
				}
				if code := h.run(append(slices.Clone(v.args), "--json")...); code != 3 || h.err.String() != v.line+"\n" {
					t.Fatalf("--json: exit=%d stderr=%q, want exit 3 and %q", code, &h.err, v.line+"\n")
				}
				iaBrObject(t, v.verb, h.out.String(), v.line)
				if after := svTree(t, h); !maps.Equal(before, after) {
					t.Fatalf("a refused %s changed the brain folder", v.verb)
				}
			})
		}
	}
}

func TestRfCurPolicyDefaults(t *testing.T) {
	for _, c := range []struct {
		role   string
		curate bool // ChangesReadAllowed and CurateAllowed
		revert bool // RevertAllowed
	}{
		{"", true, true}, {"  ", true, true}, {"owner", true, true}, {"member", true, true}, {" member ", true, true},
		{"worker", true, false}, {" worker ", true, false},
		{"guest", false, false}, {"agent", false, false}, {"Owner", false, false}, {"Worker", false, false},
		{"admin", false, false}, {"worker,guest", false, false}, {"workers", false, false},
	} {
		if got := brain.ChangesReadAllowed(c.role); got != c.curate {
			t.Errorf("ChangesReadAllowed(%q) = %v, want %v", c.role, got, c.curate)
		}
		if got := brain.CurateAllowed(c.role); got != c.curate {
			t.Errorf("CurateAllowed(%q) = %v, want %v", c.role, got, c.curate)
		}
		if got := brain.RevertAllowed(c.role); got != c.revert {
			t.Errorf("RevertAllowed(%q) = %v, want %v", c.role, got, c.revert)
		}
	}
}

// revert and supersede check CurateAllowed and not RevertAllowed: replacing one
// policy moves those two verbs only when it is CurateAllowed.
func TestRfCurPoliciesAreSeparate(t *testing.T) {
	deny := func(string) bool { return false }
	t.Run("curate denied", func(t *testing.T) {
		saved := brain.CurateAllowed
		brain.CurateAllowed = deny
		t.Cleanup(func() { brain.CurateAllowed = saved })
		for _, v := range rfCurCalls[1:] {
			h := rfCurSeed(t)
			if code := h.run(v.args...); code != 3 || h.err.String() != v.line+"\n" {
				t.Errorf("%s as the local role with CurateAllowed denying: exit %d, stderr %q, want exit 3 and %q", v.verb, code, &h.err, v.line+"\n")
			}
		}
	})
	t.Run("revert policy denied", func(t *testing.T) {
		saved := brain.RevertAllowed
		brain.RevertAllowed = deny
		t.Cleanup(func() { brain.RevertAllowed = saved })
		for _, v := range rfCurCalls[1:] {
			h := rfCurSeed(t)
			h.env.Role = "worker"
			if code := h.run(v.args...); code != 0 {
				t.Errorf("%s as worker with RevertAllowed denying: exit %d, stderr %q, want exit 0", v.verb, code, &h.err)
			}
		}
	})
}

// write, rename and drop keep RevertAllowed, which refuses a worker; the write
// and rename cases are pinned in lo_write_anyone_test.go, ke_write_test.go and
// ke_rename_test.go, and drop and restore here.
func TestRfCurWorkerStillRefusedOnDropAndRestore(t *testing.T) {
	for _, v := range iaBrVerbs[3:] {
		t.Run(v.verb, func(t *testing.T) {
			h := rfCurSeed(t)
			h.env.Role = "worker"
			before := svTree(t, h)
			if code := h.run(v.args...); code != 3 || h.out.Len() != 0 || h.err.String() != v.line+"\n" {
				t.Fatalf("%s as worker: exit=%d stdout=%q stderr=%q, want exit 3, no stdout, stderr %q", v.verb, code, &h.out, &h.err, v.line+"\n")
			}
			if after := svTree(t, h); !maps.Equal(before, after) {
				t.Fatalf("a refused %s changed the brain folder", v.verb)
			}
		})
	}
}
