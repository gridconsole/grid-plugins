package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func asSecondPerson(t *testing.T) {
	t.Helper()
	t.Setenv("GRID_MEMBER", "sam")
	t.Setenv("GRID_ROLE", "owner")
	t.Setenv("GRID_LEASE_EPOCH", "")
}

// Row 1: a fresh task has no Feed folder; a person posts its first Job.
func TestPersonFirstJobOnFreshTask(t *testing.T) {
	h, path := feedHarness(t)
	asPerson(t, "member")
	if _, err := os.Stat(filepath.Join(path, "feed")); !os.IsNotExist(err) {
		t.Fatalf("the Feed folder exists before the first post: %v", err)
	}
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-1", "--body", personBody(t, "J-1", nil)); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	info, err := os.Stat(filepath.Join(path, "feed"))
	if err != nil || !info.IsDir() {
		t.Fatalf("the Feed folder was not made: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		t.Fatalf("the Feed folder is %o, want 700", info.Mode().Perm())
	}
	if code := h.run(path, "feed", "read", "--type", "job", "--json"); code != 0 || !strings.Contains(h.out.String(), `"id":"J-1"`) {
		t.Fatalf("%d the Job is not in feed read: %s %s", code, h.out, h.errb)
	}
}

// Row 2: two people answer one person Job before a tick; the second one reads in
// feed read that the answer was not used.
func TestSecondPersonSeesTheUnusedAnswerInFeedRead(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	asPerson(t, "member")
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Call it Lark."}`); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	time.Sleep(1100 * time.Millisecond)
	asSecondPerson(t)
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Call it Wren."}`); code != 0 {
		t.Fatalf("the second answer exited %d: %s", code, h.errb)
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if st, _ := store.State(); string(st.Jobs["J-2"].Status) != "accepted" {
		t.Fatalf("J-2 is %s", st.Jobs["J-2"].Status)
	}
	asSecondPerson(t)
	if code := h.run(path, "feed", "read"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	out := h.out.String()
	if !strings.Contains(out, "dana answered J-2 at ") || !strings.Contains(out, "sam to J-2") || !strings.Contains(out, "was not used") || !strings.Contains(out, "Call it Wren.") {
		t.Fatalf("feed read does not tell sam the answer was not used:\n%s", out)
	}
}

// Row 3: an unleased answer says it is saved and pending, on stderr, with exit 0
// and pending: true in --json. A leased answer says nothing of the kind.
func TestUnleasedAnswerSaysItIsPending(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	asPerson(t, "member")
	for _, id := range []string{"J-2", "J-3", "J-4"} {
		if code := h.run(path, "job", "post", "--person", "dana", "--job", id, "--body", personBody(t, id, nil)); code != 0 {
			t.Fatalf("%d %s", code, h.errb)
		}
	}
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Call it Lark."}`); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if e := h.errb.String(); !strings.Contains(e, "Saved") || !strings.Contains(e, "next pass") {
		t.Fatalf("stderr does not say the answer is pending: %q", e)
	}
	if strings.Contains(h.out.String(), "Saved") || strings.Contains(h.out.String(), "pending") {
		t.Fatalf("the hint is on stdout: %q", h.out)
	}
	if code := h.run(path, "job", "result", "J-3", "--body", `{"answer":"Call it Lark."}`, "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	var got struct {
		ID      string `json:"id"`
		Pending bool   `json:"pending"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.ID == "" || !got.Pending {
		t.Fatalf("--json lacks id and pending: true: %s (%v)", h.out, err)
	}
	if !strings.Contains(h.errb.String(), "Saved") {
		t.Fatalf("--json dropped the stderr text: %q", h.errb)
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "result", "J-4", "--body", `{"answer":"Call it Lark."}`, "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if strings.Contains(h.out.String(), "pending") || strings.Contains(h.errb.String(), "Saved") {
		t.Fatalf("a leased answer is not pending: %s %s", h.out, h.errb)
	}
}
