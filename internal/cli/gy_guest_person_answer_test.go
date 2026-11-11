package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

const gyGpGenFence = "GRID_LEASE_EPOCH or --lease-epoch must be gen.fence"

// gyGpFixture is the task of orchestratorHarness with dana, eve and Dana granted as
// guests, and a person Job J-2 of kind ask addressed to dana, posted by a member.
func gyGpFixture(t *testing.T) (*harness, string, feed.Store, string) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	gyGpRoles(t, h, "dana", "eve", "Dana")
	t.Setenv("GRID_LEASE", "")
	gyGpPostJob(t, h, path, "J-2", "dana", "ask")
	return h, path, store, epoch
}

func gyGpRoles(t *testing.T, h *harness, people ...string) {
	t.Helper()
	roles := `{"people":[{"id":"member","perm":"Member"}`
	for _, p := range people {
		roles += `,{"id":"` + p + `","perm":"Client"}`
	}
	h.write(filepath.Join(h.gridHome, "projects", "example", "project-roles.json"), roles+`]}`)
}

func gyGpBody(t *testing.T, id, owner, kind string) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, id, nil)), &body); err != nil {
		t.Fatal(err)
	}
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": owner, "item": "i-1", "goal": "Choose."}
	if kind != "" {
		body["kind"] = kind
	}
	out, _ := json.Marshal(body)
	return string(out)
}

func gyGpPostJob(t *testing.T, h *harness, path, id, owner, kind string) {
	t.Helper()
	asPerson(t, "member")
	t.Setenv("GRID_MEMBER", "member")
	if code := h.run(path, "job", "post", "--type", "job", "--job", id, "--body", gyGpBody(t, id, owner, kind)); code != 0 {
		t.Fatalf("post %s: %d %s", id, code, h.errb)
	}
}

func gyGpAs(t *testing.T, member, lease string) {
	t.Helper()
	t.Setenv("GRID_ROLE", "guest")
	t.Setenv("GRID_MEMBER", member)
	t.Setenv("GRID_LEASE", lease)
	t.Setenv("GRID_LEASE_EPOCH", "")
}

// gyGpBytes is the size of every Feed file of the task: a refused write leaves it as it was.
func gyGpBytes(t *testing.T, store feed.Store) int64 {
	t.Helper()
	var n int64
	_ = filepath.Walk(store.Dir(), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".jsonl") {
			n += info.Size()
		}
		return nil
	})
	return n
}

func gyGpAnswers(t *testing.T, store feed.Store) []feed.Record {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: "answer"})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Record
	for _, e := range r.Records {
		out = append(out, e.Record)
	}
	return out
}

// Row 1: dana, a guest, answers the person Job addressed to her with no lease.
func TestGyGpAddressedGuestAnswers(t *testing.T) {
	h, path, store, epoch := gyGpFixture(t)
	gyGpAs(t, "dana", "")
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Yes, go"}`, "--json"); code != 0 {
		t.Fatalf("guest dana: exit %d: %s", code, h.errb)
	}
	st, _ := store.State()
	first := st.Jobs["J-2"].History[0]
	got := gyGpAnswers(t, store)
	if len(got) != 1 || got[0].Author.ID != "dana" || got[0].Author.Role != "guest" || got[0].Re == nil || *got[0].Re != first || got[0].Lease != nil || got[0].Fence != 0 {
		t.Fatalf("want one unleased answer by guest dana re J-2's record, got %+v", got)
	}
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Second thought"}`, "--json"); code != 0 {
		t.Fatalf("second answer: exit %d: %s", code, h.errb)
	}
	if got := gyGpAnswers(t, store); len(got) != 2 {
		t.Fatalf("want a second answer record, got %d", len(got))
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("holder pass: %d %s", code, h.errb)
	}
	st, _ = store.State()
	if string(st.Jobs["J-2"].Status) != "accepted" || st.Jobs["J-2"].Reason != "answered by dana" {
		t.Fatalf("J-2 is %s (%q)", st.Jobs["J-2"].Status, st.Jobs["J-2"].Reason)
	}
	r, _ := store.Read(feed.Filter{})
	unused := false
	for _, e := range r.Records {
		text, _ := e.Body["text"].(string)
		unused = unused || e.Type == "post" && strings.Contains(text, "was not used") && strings.Contains(text, "Second thought")
	}
	if !unused {
		t.Fatal("the second answer has no not-used message")
	}
}

// Row 2: every other guest keeps today's refusal, and the Feed is left as it was.
func TestGyGpGuestRefusals(t *testing.T) {
	h, path, store, epoch := gyGpFixture(t)
	gyGpPostJob(t, h, path, "J-4", "dana", "approve")
	// A Job record cannot carry an empty intent.owner (the schema refuses it), so the
	// empty-identity row is a guest with no GRID_MEMBER on dana's Job.
	gyGpPostJob(t, h, path, "J-6", "plan.md#approve", "ask")
	gyGpPostJob(t, h, path, "J-7", "dana", "ask")
	hash := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, member, job string
		extra            []string
		want             string
	}{
		{"another guest", "eve", "J-2", nil, gyGpGenFence},
		{"different case", "Dana", "J-2", nil, gyGpGenFence},
		{"empty member", "", "J-2", nil, "project is not registered"},
		{"pointer owner", "dana", "J-6", nil, gyGpGenFence},
		{"plan approval", "dana", "J-4", nil, gyGpGenFence},
		{"plan approval with plan hash", "dana", "J-4", []string{"--plan-hash", hash}, gyGpGenFence},
		{"ask with plan hash", "dana", "J-7", []string{"--plan-hash", hash}, gyGpGenFence},
		{"worker Job", "dana", "J-1", nil, gyGpGenFence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gyGpAs(t, tc.member, "")
			before := gyGpBytes(t, store)
			args := append([]string{"job", "result", tc.job, "--body", `{"answer":"Yes, go"}`}, tc.extra...)
			if code := h.run(path, args...); code == 0 {
				t.Fatalf("exit 0: %s", h.out)
			}
			t.Logf("stderr: %s", strings.TrimSpace(h.errb.String()))
			if !strings.Contains(h.errb.String(), tc.want) {
				t.Fatalf("stderr %q lacks %q", h.errb, tc.want)
			}
			if gyGpBytes(t, store) != before || len(gyGpAnswers(t, store)) != 0 {
				t.Fatal("a refused answer changed the Feed")
			}
		})
	}
	t.Run("job post --person", func(t *testing.T) {
		gyGpAs(t, "dana", "")
		before := gyGpBytes(t, store)
		if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-9", "--body", gyGpBody(t, "J-9", "dana", "ask")); code == 0 {
			t.Fatal("a guest posted a person Job")
		}
		t.Logf("stderr: %s", strings.TrimSpace(h.errb.String()))
		if gyGpBytes(t, store) != before {
			t.Fatal("a refused post changed the Feed")
		}
	})
	t.Run("no grant", func(t *testing.T) {
		gyGpRoles(t, h)
		gyGpAs(t, "dana", "")
		before := gyGpBytes(t, store)
		for _, extra := range [][]string{nil, {"--project", "example"}, {"--task", path}} {
			args := append([]string{"job", "result", "J-2", "--body", `{"answer":"Yes, go"}`}, extra...)
			if code := h.run(path, args...); code == 0 {
				t.Fatalf("%v: exit 0", extra)
			}
			t.Logf("%v stderr: %s", extra, strings.TrimSpace(h.errb.String()))
			if strings.Contains(h.errb.String(), "gen.fence") || gyGpBytes(t, store) != before {
				t.Fatalf("%v: stderr %q or the Feed changed", extra, h.errb)
			}
		}
		gyGpRoles(t, h, "dana", "eve", "Dana")
	})
	t.Run("accepted Job", func(t *testing.T) {
		gyGpAs(t, "dana", "")
		if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Yes, go"}`); code != 0 {
			t.Fatalf("%d %s", code, h.errb)
		}
		asHolder(t, epoch)
		if code := h.run(path, "job", "next", "--json"); code != 0 {
			t.Fatalf("%d %s", code, h.errb)
		}
		gyGpAs(t, "dana", "")
		before, n := gyGpBytes(t, store), len(gyGpAnswers(t, store))
		if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"again"}`); code == 0 {
			t.Fatal("an accepted person Job took a second answer")
		}
		t.Logf("stderr: %s", strings.TrimSpace(h.errb.String()))
		if gyGpBytes(t, store) != before || len(gyGpAnswers(t, store)) != n {
			t.Fatal("a refused answer changed the Feed")
		}
	})
}

// Row 4: GRID_LEASE=task never reaches the guest path, for the addressee or for anyone else.
func TestGyGpGuestTaskLease(t *testing.T) {
	h, path, store, _ := gyGpFixture(t)
	for _, member := range []string{"eve", "dana"} {
		for _, extra := range [][]string{nil, {"--lease", "task"}} {
			gyGpAs(t, member, "task")
			before := gyGpBytes(t, store)
			args := append([]string{"job", "result", "J-2", "--body", `{"answer":"Yes, go"}`}, extra...)
			if code := h.run(path, args...); code == 0 {
				t.Fatalf("%s %v: exit 0", member, extra)
			}
			if !strings.Contains(h.errb.String(), gyGpGenFence) || gyGpBytes(t, store) != before || len(gyGpAnswers(t, store)) != 0 {
				t.Fatalf("%s %v: stderr %q, or the Feed changed", member, extra, h.errb)
			}
		}
	}
}
