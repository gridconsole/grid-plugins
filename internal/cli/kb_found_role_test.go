package cli

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// kbRoleAgent is the J-1 agent of newAgentTask with its Job lease set and its tenure started by ack,
// as the app starts every Job agent: GRID_ROLE=worker and GRID_LEASE=J-1.
func kbRoleAgent(t *testing.T) (*agentTask, int) {
	t.Helper()
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	state, err := a.store.State()
	if err != nil || state.Jobs["J-1"] == nil {
		t.Fatal(state, err)
	}
	attempt := state.Jobs["J-1"].Attempt
	if code := a.h.run(a.path, "job", "ack", "J-1", "--attempt", strconv.Itoa(attempt), "--json"); code != 0 {
		t.Fatalf("ack J-1: exit %d %s", code, a.h.errb)
	}
	return a, attempt
}

// kbRoleJobRecord is the id of the job record that posted the Job id.
func kbRoleJobRecord(t *testing.T, a *agentTask, id string) string {
	t.Helper()
	read, err := a.store.Read(feed.Filter{Type: "job", Job: id})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("job record of %s: %+v %v", id, read, err)
	}
	return read.Records[0].ID
}

// kbRolePost runs grid job post --type found under the Job lease named by --job, with a found body
// on the Job onJob, in the shape of contract/fixtures/found/valid/basic.json.
func kbRolePost(t *testing.T, a *agentTask, attempt int, job, onJob, kind string) int {
	t.Helper()
	body, err := json.Marshal(map[string]any{"job": onJob, "re": kbRoleJobRecord(t, a, onJob), "at": "main.go:10",
		"text": "The handler reads the token from the query string.", "kind": kind, "class": nil})
	if err != nil {
		t.Fatal(err)
	}
	return a.h.run(a.path, "job", "post", "--task", a.path, "--job", job, "--attempt", strconv.Itoa(attempt), "--type", "found", "--body", string(body))
}

// kbRoleFound is the found records the store holds.
func kbRoleFound(t *testing.T, a *agentTask) []feed.Entry {
	t.Helper()
	read, err := a.store.Read(feed.Filter{Type: "found"})
	if err != nil {
		t.Fatal(err)
	}
	return read.Records
}

// Row 1: a Job agent posts a security found under its own Job lease; the fold marks the Job for the lead and wakes on it.
func TestKbRoleWorkerPostsSecurityFound(t *testing.T) {
	for _, role := range []string{"worker", "agent"} {
		t.Run("role="+role, func(t *testing.T) {
			a, attempt := kbRoleAgent(t)
			t.Setenv("GRID_ROLE", role)
			if code := kbRolePost(t, a, attempt, "J-1", "J-1", "security"); code != 0 {
				t.Fatalf("found post as %s under the J-1 lease: exit %d %s", role, code, a.h.errb)
			}
			found := kbRoleFound(t, a)
			if len(found) != 1 {
				t.Fatalf("found records: %+v", found)
			}
			r := found[0]
			if r.Type != "found" || r.Author.Role != role || r.Lease == nil || *r.Lease != "J-1" {
				t.Fatalf("found record: type %q role %q lease %v", r.Type, r.Author.Role, r.Lease)
			}
			state, err := a.store.State()
			if err != nil {
				t.Fatal(err)
			}
			if !state.Jobs["J-1"].NeedsLead {
				t.Fatalf("J-1 should need the lead: %+v", state.Jobs["J-1"])
			}
			if !slices.Contains(state.Wakes, r.ID) {
				t.Fatalf("wakes %v should hold the found %s", state.Wakes, r.ID)
			}
		})
	}
}

// Row 2: body.job names the Job the found is about, which need not be the Job whose lease writes it.
func TestKbRoleFoundNamesAnotherJob(t *testing.T) {
	a, attempt := kbRoleAgent(t)
	if code := a.postJob(t, a.path, "J-2"); code != 0 {
		t.Fatalf("post J-2: exit %d %s", code, a.h.errb)
	}
	if code := kbRolePost(t, a, attempt, "J-1", "J-2", "security"); code != 0 {
		t.Fatalf("security found on J-2 under the J-1 lease: exit %d %s", code, a.h.errb)
	}
	state, err := a.store.State()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Jobs["J-2"].NeedsLead || state.Jobs["J-1"].NeedsLead {
		t.Fatalf("NeedsLead: J-2 %v (want true), J-1 %v (want false)", state.Jobs["J-2"].NeedsLead, state.Jobs["J-1"].NeedsLead)
	}
	wakes := len(state.Wakes)
	if code := kbRolePost(t, a, attempt, "J-1", "J-1", "bug"); code != 0 {
		t.Fatalf("bug found on J-1: exit %d %s", code, a.h.errb)
	}
	if n := len(kbRoleFound(t, a)); n != 2 {
		t.Fatalf("found records: %d, want 2", n)
	}
	state, err = a.store.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.Jobs["J-1"].NeedsLead || len(state.Wakes) != wakes {
		t.Fatalf("a bug found should set no NeedsLead and add no wake: J-1 %v, wakes %v (had %d)", state.Jobs["J-1"].NeedsLead, state.Wakes, wakes)
	}
}

// Row 3: other roles stay refused by the role enum, and the lease is still the agent's own Job.
func TestKbRoleGuestStillRefused(t *testing.T) {
	for _, role := range []string{"guest", "member", "owner", "curator", "host", "external"} {
		t.Run("role="+role, func(t *testing.T) {
			a, attempt := kbRoleAgent(t)
			t.Setenv("GRID_ROLE", role)
			if code := kbRolePost(t, a, attempt, "J-1", "J-1", "security"); code == 0 {
				t.Fatalf("found post as %s should be refused", role)
			}
			// A guest or external is refused before the schema is read (task not found); every other role reaches the role enum.
			if role != "guest" && role != "external" && !strings.Contains(a.h.errb.String(), "outside enum") {
				t.Fatalf("found post as %s should be refused by the role enum: %s", role, a.h.errb)
			}
			if found := kbRoleFound(t, a); len(found) != 0 {
				t.Fatalf("a refused post stored found records: %+v", found)
			}
		})
	}
	t.Run("lease=J-2", func(t *testing.T) {
		a, attempt := kbRoleAgent(t)
		if code := a.postJob(t, a.path, "J-2"); code != 0 {
			t.Fatalf("post J-2: exit %d %s", code, a.h.errb)
		}
		if code := kbRolePost(t, a, attempt, "J-2", "J-2", "security"); code != 3 || !strings.Contains(a.h.errb.String(), feed.ErrStaleLease.Error()) {
			t.Fatalf("found post under J-2, a Job the agent does not hold: exit %d %s", code, a.h.errb)
		}
		if found := kbRoleFound(t, a); len(found) != 0 {
			t.Fatalf("a refused post stored found records: %+v", found)
		}
	})
}
