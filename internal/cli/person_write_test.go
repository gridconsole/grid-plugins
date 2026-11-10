package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// asPerson runs the next commands as a person with no lease epoch; asHolder as
// the task holder orchestratorHarness made.
func asPerson(t *testing.T, role string) {
	t.Helper()
	t.Setenv("GRID_MEMBER", "dana")
	t.Setenv("GRID_ROLE", role)
	t.Setenv("GRID_LEASE_EPOCH", "")
}

func asHolder(t *testing.T, epoch string) {
	t.Helper()
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", "member")
	t.Setenv("GRID_LEASE_EPOCH", epoch)
}

func recordOf(t *testing.T, store feed.Store, typ, jobID string) (feed.Record, bool) {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: typ})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Records {
		if id, _ := e.Body["id"].(string); id == jobID {
			return e.Record, true
		}
	}
	return feed.Record{}, false
}

// Row 1: the worker holds the task lease; a person posts a Job with no epoch and
// one pass of the holder shows it in job tree.
func TestPersonJobPostWhileTaskLeaseHeld(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	asPerson(t, "member")
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	rec, ok := recordOf(t, store, "job", "J-2")
	if !ok || rec.Lease != nil || rec.Fence != 0 || rec.Author.ID != "dana" {
		t.Fatalf("want an unleased record by dana, got %v lease=%v fence=%d by %s", ok, rec.Lease, rec.Fence, rec.Author.ID)
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	h.out.Reset()
	if code := h.run(path, "job", "tree"); code != 0 || !strings.Contains(h.out.String(), "J-2  Person  posted") {
		t.Fatalf("%d tree: %s %s", code, h.out, h.errb)
	}
}

// Row 2: the task lease is held; a person answers a person Job with no epoch,
// the holder's next pass closes it, and the Job behind it is assigned.
func TestPersonAnswerWhileTaskLeaseHeld(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	asPerson(t, "member")
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-3", "--body", personBody(t, "J-3", []any{"J-2"})); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	asPerson(t, "member")
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Call it Lark."}`); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if st, _ := store.State(); string(st.Jobs["J-2"].Status) != "posted" {
		t.Fatalf("J-2 is %s before the holder's pass", st.Jobs["J-2"].Status)
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if string(st.Jobs["J-2"].Status) != "accepted" || string(st.Jobs["J-3"].Status) != "assigned" {
		t.Fatalf("J-2 is %s and J-3 is %s", st.Jobs["J-2"].Status, st.Jobs["J-3"].Status)
	}
	r, _ := store.Read(feed.Filter{})
	said := false
	for _, e := range r.Records {
		text, _ := e.Body["text"].(string)
		said = said || e.Type == "post" && strings.Contains(text, "dana answered J-2 at ") && strings.Contains(text, "Call it Lark.")
	}
	if !said {
		t.Fatal("who, when and the answer are not in the Feed")
	}
}

// Row 4: an agent with no lease epoch cannot post a Job or answer a person Job.
func TestAgentCannotWriteAsPersonWithoutLease(t *testing.T) {
	for _, role := range []string{"worker", "builder", "agent"} {
		t.Run(role, func(t *testing.T) {
			h, path, store := orchestratorHarness(t)
			epoch := os.Getenv("GRID_LEASE_EPOCH")
			asPerson(t, "member")
			if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); code != 0 {
				t.Fatalf("%d %s", code, h.errb)
			}
			asPerson(t, role)
			if code := h.run(path, "job", "post", "--type", "job", "--job", "J-3", "--body", personBody(t, "J-3", nil)); code == 0 {
				t.Fatalf("%s posted a Job without the task lease", role)
			}
			if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-4", "--body", personBody(t, "J-4", nil)); code == 0 {
				t.Fatalf("%s posted a person Job without the task lease", role)
			}
			if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Call it Lark."}`); code == 0 {
				t.Fatalf("%s answered a person Job without the task lease", role)
			}
			asHolder(t, epoch)
			if code := h.run(path, "job", "next", "--json"); code != 0 {
				t.Fatalf("%d %s", code, h.errb)
			}
			st, _ := store.State()
			if st.Jobs["J-3"] != nil || st.Jobs["J-4"] != nil || string(st.Jobs["J-2"].Status) != "posted" {
				t.Fatalf("%s changed the Feed: J-3=%v J-4=%v J-2=%s", role, st.Jobs["J-3"], st.Jobs["J-4"], st.Jobs["J-2"].Status)
			}
		})
	}
}
