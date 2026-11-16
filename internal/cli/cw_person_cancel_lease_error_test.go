package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// Lane C-cw-person-cancel-request-edges. A person of the team cancels a Running Job with no epoch:
// orchestrator.go reads the task lease, and writes under it only when the read succeeds and the
// holder is the person's own <member>@<machine>. When the read fails, the cancel is saved as an
// unleased request, as when another holder holds the lease. The tests in
// r20_cancel_request_cli_test.go cover the holder, another holder and nobody; these cover a lease
// file that Read refuses.

// cwPcLeasePath is the task lease file of the task: <task>/feed/lease/task.json.
func cwPcLeasePath(a *agentTask) string {
	return filepath.Join(a.store.Dir(), "lease", "task.json")
}

// cwPcInvalidJSON is a lease file that is not JSON at all.
func cwPcInvalidJSON(t *testing.T, a *agentTask) {
	t.Helper()
	if err := os.WriteFile(cwPcLeasePath(a), []byte("{\"lease\": \"task\", "), 0o600); err != nil {
		t.Fatal(err)
	}
}

// cwPcOtherLease is the lease file the task holds, rewritten as the lease of J-1 with the same
// holder: it passes the schema and Read refuses it with "lease identity mismatch".
func cwPcOtherLease(t *testing.T, a *agentTask) {
	t.Helper()
	b, err := os.ReadFile(cwPcLeasePath(a))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil || m["lease"] != "task" {
		t.Fatalf("the lease file is not the task lease: %v %s", err, b)
	}
	// The Job-lease shape of contract/schemas/lease.json (attempt, no pid), so that only the
	// name is wrong; the holder is still the task's holder.
	m["lease"], m["attempt"], m["pid"], m["pid_start"] = "J-1", 1, nil, nil
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cwPcLeasePath(a), out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Row 1: Read of the task lease fails, so the person's cancel is an unleased request: exit 0,
// the words of a saved request, one new control by the person with no lease and fence 0, and J-1
// stays Running. The member `worker` is the one the lease holder worker@<machine> is named for: with
// a good lease its cancel is written at once (TestR20ClN10HolderPersonCancelsAtOnce), so a leased
// control here would mean a read error was taken for "the holder is me".
func TestCwPcLeaseReadErrorSavesTheRequest(t *testing.T) {
	for _, bad := range []struct {
		name   string
		break_ func(*testing.T, *agentTask)
		want   string // part of the error Read gives
	}{
		{"invalid JSON", cwPcInvalidJSON, ""},
		{"another lease", cwPcOtherLease, "lease identity mismatch"},
	} {
		for _, member := range []string{"worker", "dana"} {
			t.Run(bad.name+"/member="+member, func(t *testing.T) {
				a, _ := kbRoleAgent(t)
				bad.break_(t, a)
				// The precondition: Read refuses the file.
				if _, err := (lease.TaskStore{Feed: a.store}).Read("task"); err == nil || !strings.Contains(err.Error(), bad.want) {
					t.Fatalf("Read of the broken lease: %v, want an error holding %q", err, bad.want)
				}
				r18PersonAtTerminal(t, member)
				before := r20ClControls(t, a)
				if code := r20ClCancel(a); code != 0 {
					t.Fatalf("a cancel with an unreadable task lease: exit %d %s, want 0", code, a.h.errb)
				}
				if !strings.Contains(a.h.errb.String(), r20ClSaved) {
					t.Fatalf("stderr does not say the request was saved: %q", a.h.errb)
				}
				if got := a.status(t, "J-1"); got != contract.Running {
					t.Fatalf("J-1 is %q after the request alone, want running", got)
				}
				after := r20ClControls(t, a)
				if len(after) != len(before)+1 {
					t.Fatalf("want one new control, got %d before and %d after", len(before), len(after))
				}
				c := after[len(after)-1]
				if c.Lease != nil || c.Fence != 0 || c.Author.ID != member || c.Author.Role != "owner" ||
					c.Body["status"] != "cancelled" || c.Body["target"] != "J-1" {
					t.Fatalf("want an unleased cancel request by %s: %+v", member, c)
				}
				for _, c := range after[len(before):] {
					if c.Lease != nil {
						t.Fatalf("a leased control was written: %+v", c)
					}
				}
			})
		}
	}
}
