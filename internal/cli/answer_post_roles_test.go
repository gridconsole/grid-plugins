package cli

import (
	"fmt"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// An answer post is an ordinary post. Whether it closes a question gate is decided
// by the gate's role admission (owner, member, lead; a guest only on a question
// addressed to that guest), never by the path it was posted through or by the
// process that posted it. A role the gate does not admit stores the post and leaves
// the gate open.
func TestFeedPostAnswerRoleAdmission(t *testing.T) {
	for _, tc := range []struct {
		role, member string
		code         int
		closes       bool
	}{
		{"", "", 0, true},
		{"member", "", 0, true},
		{"owner", "", 0, true},
		{"lead", "", 0, true},
		{"guest", "visitor", 0, true},
		{"guest", "other", 3, false},
		{"agent", "", 0, false},
		{"cli", "", 0, false},
	} {
		t.Run("role="+tc.role+"/member="+tc.member, func(t *testing.T) {
			h, task := feedHarness(t)
			q := questionAdd(t, h, task, "visitor")
			t.Setenv("GRID_ROLE", tc.role)
			if tc.member != "" {
				t.Setenv("GRID_MEMBER", tc.member)
			}
			code := h.run(task, "feed", "post", "--type", "answer", "--re", q.Post, "--body", `{"answer":"Yes"}`)
			if code != tc.code {
				t.Fatalf("feed post exit %d, want %d: %s", code, tc.code, h.errb)
			}
			store := feed.Store{TaskDir: task, TaskID: feedTaskID}
			read, err := store.Read(feed.Filter{Type: "answer"})
			if err != nil {
				t.Fatal(err)
			}
			if stored := len(read.Records) == 1; stored != (tc.code == 0) {
				t.Fatalf("answer records stored: %d, exit %d", len(read.Records), code)
			}
			state, err := store.State()
			if err != nil {
				t.Fatal(err)
			}
			if closed := !state.Questions[q.Post].Open; closed != tc.closes {
				t.Fatalf("gate closed = %v, want %v: %+v", closed, tc.closes, state.Questions[q.Post])
			}
		})
	}
}

// grid job post carries the same rule: the answer type is not refused for any role,
// and the gate's role admission alone decides whether the post closes it.
func TestJobPostAnswerRoleAdmission(t *testing.T) {
	for _, tc := range []struct {
		role   string
		closes bool
	}{
		{"", true},
		{"member", true},
		{"owner", true},
		{"worker", false},
	} {
		t.Run("role="+tc.role, func(t *testing.T) {
			h, path, store := orchestratorHarness(t)
			q := questionAdd(t, h, path, "member")
			if code := h.run(path, "job", "next", "--json"); code != 0 {
				t.Fatalf("next %d: %s", code, h.errb)
			}
			state, err := store.State()
			if err != nil || state.Jobs["J-1"].Assign == nil {
				t.Fatalf("J-1 not assigned: %+v %v", state.Jobs["J-1"], err)
			}
			t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
			t.Setenv("GRID_ROLE", "worker")
			if code := h.run(path, "job", "ack", "J-1", "--attempt", "1", "--json"); code != 0 {
				t.Fatalf("ack %d: %s", code, h.errb)
			}
			t.Setenv("GRID_ROLE", tc.role)
			if code := h.run(path, "job", "post", "J-1", "--attempt", "1", "--type", "answer", "--re", q.Post, "--body", `{"answer":"Yes"}`); code != 0 {
				t.Fatalf("job post %d: %s", code, h.errb)
			}
			read, err := store.Read(feed.Filter{Type: "answer"})
			if err != nil || len(read.Records) != 1 {
				t.Fatalf("answer post not stored: %+v %v", read, err)
			}
			state, err = store.State()
			if err != nil {
				t.Fatal(err)
			}
			if closed := !state.Questions[q.Post].Open; closed != tc.closes {
				t.Fatalf("gate closed = %v, want %v: %+v", closed, tc.closes, state.Questions[q.Post])
			}
		})
	}
}
