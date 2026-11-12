package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// xhNeedsYou reads feed/needs-you.json as the app does: the file, not a fold.
func xhNeedsYou(t *testing.T, store feed.Store) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store.Dir(), "needs-you.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		t.Fatalf("needs-you.json does not decode: %v\n%s", err, data)
	}
	return ids
}

func xhHas(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

// Row 1: an answer posted through `grid job post` leaves Needs you at once, with
// no `grid feed state` and no tick between the post and the read.
func TestXhJobPostRefolds(t *testing.T) {
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
	if listed := xhNeedsYou(t, store); !xhHas(listed, q.Post) {
		t.Fatalf("before the answer, needs-you.json lists %v, want %s (question add refolds it)", listed, q.Post)
	}
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "J-1", "--attempt", "1", "--type", "answer", "--re", q.Post, "--body", `{"answer":"Yes"}`); code != 0 {
		t.Fatalf("job post %d: %s", code, h.errb)
	}
	if h.errb.Len() != 0 {
		t.Fatalf("a good post and refold print nothing on stderr: %q", h.errb)
	}
	if listed := xhNeedsYou(t, store); xhHas(listed, q.Post) {
		t.Fatalf("needs-you.json still lists the answered question %s: %v", q.Post, listed)
	}
	// What the file says is what a fold says.
	after, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(xhNeedsYou(t, store), ","), strings.Join(after.NeedsYou, ","); got != want {
		t.Fatalf("needs-you.json %q, fold %q", got, want)
	}
}

// `grid feed post` takes the same path; a question the post opens shows at once too.
func TestXhFeedPostOpensQuestionAtOnce(t *testing.T) {
	h, task := feedHarness(t)
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	if code := h.run(task, "feed", "post", "--type", "question", "--body", `{"question":"Continue?","who":"member"}`, "--json"); code != 0 {
		t.Fatalf("post %d: %s", code, h.errb)
	}
	var out struct {
		Post feed.Record `json:"post"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if listed := xhNeedsYou(t, store); !xhHas(listed, out.Post.ID) {
		t.Fatalf("needs-you.json lists %v, want the new question %s", listed, out.Post.ID)
	}
}

// Row 2: the post is done when the fold fails. The command still exits 0 with the
// post JSON on stdout, and stderr says the refold failed.
func TestXhFeedPostRefoldError(t *testing.T) {
	h, task := feedHarness(t)
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	if err := os.MkdirAll(filepath.Join(store.Dir(), "needs-you.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	code := h.run(task, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"example"}`, "--json")
	if code != 0 {
		t.Fatalf("feed post exit %d, want 0: %s", code, h.errb)
	}
	var out struct {
		Post feed.Record `json:"post"`
		Tier string      `json:"tier"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil || out.Post.ID == "" || out.Post.Type != "post" {
		t.Fatalf("stdout is not the post JSON: %q %v", h.out, err)
	}
	found := false
	for _, line := range strings.Split(h.errb.String(), "\n") {
		if strings.HasPrefix(line, "refold: ") {
			found = true
		}
	}
	if !found {
		t.Fatalf("stderr has no line starting %q: %q", "refold: ", h.errb)
	}
	read, err := store.Read(feed.Filter{Type: "post"})
	if err != nil || len(read.Records) != 1 || read.Records[0].ID != out.Post.ID {
		t.Fatalf("the post is not stored: %+v %v", read, err)
	}
}
