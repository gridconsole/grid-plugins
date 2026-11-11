package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// ioqUnknownRef is a record id no Feed in these tests holds, so a record that
// lists it in after is held until a record with that id arrives.
const ioqUnknownRef = "01ARZ3NDEKTSV4RRFFQ69G5FAZ"

const ioqAskTitle = "Q-1: Which export format?"

// ioqBody is personBody with its own title, so two records for one Job id can be told apart.
func ioqBody(t *testing.T, id, title string) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, id, nil)), &body); err != nil {
		t.Fatal(err)
	}
	body["title"] = title
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// ioqPostJob posts a `job` record for id as the person, held when after is set.
func ioqPostJob(t *testing.T, h *harness, path, id, title, after string) {
	t.Helper()
	asPerson(t, "member")
	args := []string{"feed", "post", "--type", "job", "--job", id}
	if after != "" {
		args = append(args, "--after", after)
	}
	if code := h.run(path, append(args, "--body", ioqBody(t, id, title))...); code != 0 {
		t.Fatalf("post %s %d: %s", id, code, h.errb)
	}
}

// ioqResolve posts the record that ioqUnknownRef names, so the held record resolves.
func ioqResolve(t *testing.T, h *harness, path string) {
	t.Helper()
	asPerson(t, "member")
	if code := h.run(path, "feed", "post", "--type", "post", "--id", ioqUnknownRef, "--body", `{"ref":"task","kind":"message","text":"here"}`); code != 0 {
		t.Fatalf("resolve %d: %s", code, h.errb)
	}
}

// ioqJobEntry is the job record whose title is title, as store.Read shows it.
func ioqJobEntry(t *testing.T, store feed.Store, title string) feed.Entry {
	t.Helper()
	read, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range read.Records {
		if e.Body["title"] == title {
			return e
		}
	}
	t.Fatalf("no job record titled %q", title)
	return feed.Entry{}
}

// ioqFoldVerdict is the fold's verdict for a record id, as orchestrator.Load reports it.
func ioqFoldVerdict(t *testing.T, store feed.Store, recordID string) (kind, reason string) {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range snap.State.Verdicts {
		if v.ID == recordID {
			return v.Kind, v.Reason
		}
	}
	t.Fatalf("no fold verdict for %s", recordID)
	return "", ""
}

// ioqHeldJ2 posts a member's J-2 that waits for a record the Feed does not hold yet.
func ioqHeldJ2(t *testing.T, h *harness, path string, store feed.Store) feed.Entry {
	t.Helper()
	ioqPostJob(t, h, path, "J-2", "Member J-2", ioqUnknownRef)
	held := ioqJobEntry(t, store, "Member J-2")
	if held.Verdict != "held" {
		t.Fatalf("J-2 record verdict %q, want held", held.Verdict)
	}
	return held
}

// A question's Job takes an id no job record holds, a held one included.
func TestIoqAskSkipsHeldJobID(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	ioqHeldJ2(t, h, path, store)
	if q := askJob(t, h, path); q.Job != "J-3" {
		t.Fatalf("ask took %s while a held record holds J-2; want J-3", q.Job)
	}
}

// When the held record resolves, both Jobs stand: the member's J-2 keeps its
// title and the question's own Job is the one the question names.
func TestIoqHeldJobResolvesBesideQuestionJob(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	ioqHeldJ2(t, h, path, store)
	q := askJob(t, h, path)
	if q.Job != "J-3" {
		t.Errorf("ask took %s; want J-3", q.Job)
	}
	ioqResolve(t, h, path)
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if got := snap.State.Jobs["J-2"]; got == nil || got.Title != "Member J-2" {
		t.Errorf("J-2 after resolve: %+v", got)
	}
	if got := snap.State.Jobs["J-3"]; got == nil || got.Title != ioqAskTitle {
		t.Errorf("J-3 after resolve: %+v", got)
	}
	posts, err := store.Read(feed.Filter{Type: "question"})
	if err != nil || len(posts.Records) != 1 || posts.Records[0].Body["job"] != "J-3" {
		t.Errorf("question post: %v %+v", err, posts.Records)
	}
	ask := ioqJobEntry(t, store, ioqAskTitle)
	if kind, reason := ioqFoldVerdict(t, store, ask.ID); kind != "counted" {
		t.Errorf("the question's Job record is %s (%s) in the fold; want counted", kind, reason)
	}
}

// An old Feed that already holds the pair folds as it always did: the record
// with the lower seq is the Job, and the later one is ignored.
func TestIoqOldFeedPairFoldsAsToday(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	ioqPostJob(t, h, path, "J-2", "First J-2", ioqUnknownRef)
	ioqPostJob(t, h, path, "J-2", "Second J-2", "")
	ioqResolve(t, h, path)
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if got := snap.State.Jobs["J-2"]; got == nil || got.Title != "First J-2" {
		t.Fatalf("J-2 title: %+v, want the first record's", got)
	}
	second := ioqJobEntry(t, store, "Second J-2")
	kind, reason := ioqFoldVerdict(t, store, second.ID)
	if kind != "ignored" || !strings.Contains(reason, "already exists") {
		t.Fatalf("second J-2 record: %s (%s), want ignored", kind, reason)
	}
}
