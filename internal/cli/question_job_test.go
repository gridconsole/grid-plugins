package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/question"
)

// askJob runs `grid question ask` as a person with no lease epoch and returns the
// question record, whose job names the person Job that was posted.
func askJob(t *testing.T, h *harness, task string) question.Record {
	t.Helper()
	asPerson(t, "member")
	args := []string{"question", "ask", "Which export format?", "--who", "dana", "--option", "csv", "--option", "json",
		"--recommended", "csv", "--default", "csv", "--applies", "2099-01-01T00:00:00Z", "--json"}
	if code := h.run(task, args...); code != 0 {
		t.Fatalf("ask %d: %s", code, h.errb)
	}
	var r question.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	h.out.Reset()
	return r
}

func readyNow(t *testing.T, store feed.Store) []string {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	r, err := orchestrator.Ready(snap, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r.Ready
}

// ask posts a person Job (to dana, with the choices on its question post). A Job
// that lists it in after waits; a Job that does not carries on. The file is the
// view of the Job, and answer posts the Job's result when the holder's pass runs.
func TestQuestionAskIsPersonJobAnswerClosesIt(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	q := askJob(t, h, path)
	if q.Job != "J-2" || q.ID != "Q-1" {
		t.Fatalf("ask: %+v", q)
	}
	rec, ok := recordOf(t, store, "job", q.Job)
	if !ok || rec.Body["role"] != "Person" || rec.Body["kind"] != "ask" || rec.Lease != nil || rec.Fence != 0 {
		t.Fatalf("person Job record: %v %+v", ok, rec)
	}
	if after, _ := rec.Body["after"].([]any); len(after) != 0 {
		t.Fatalf("the question Job waits for %v; it must hold only its dependents", after)
	}
	if owner, _ := rec.Body["intent"].(map[string]any)["owner"].(string); owner != "dana" {
		t.Fatalf("Job is not for dana: %v", rec.Body["intent"])
	}
	posts, _ := store.Read(feed.Filter{Type: "question"})
	if len(posts.Records) != 1 || posts.Records[0].Body["options"].([]any)[1] != "json" || posts.Records[0].Body["job"] != q.Job {
		t.Fatalf("question post: %+v", posts.Records)
	}
	// J-3 needs the answer; J-4 does not.
	for id, after := range map[string][]any{"J-3": {q.Job}, "J-4": nil} {
		if code := h.run(path, "job", "post", "--type", "job", "--job", id, "--body", personBody(t, id, after)); code != 0 {
			t.Fatalf("post %s %d: %s", id, code, h.errb)
		}
	}
	ready := readyNow(t, store)
	if !slices.Contains(ready, "J-4") || slices.Contains(ready, "J-3") {
		t.Fatalf("while the question is open the Job outside the after list must be ready and the one inside must wait: %v", ready)
	}
	file, err := os.ReadFile(filepath.Join(path, "questions", "Q-1.md"))
	if err != nil || !strings.Contains(string(file), `"job": "J-2"`) || !strings.Contains(string(file), "- csv (Recommended)") {
		t.Fatalf("file view: %v %s", err, file)
	}
	// answer posts the Job's result: the holder's pass accepts it and J-3 carries on.
	asPerson(t, "member")
	if code := h.run(h.gridHome, "answer", path, q.ID, "json"); code != 0 {
		t.Fatalf("answer %d: %s", code, h.errb)
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("pass %d: %s", code, h.errb)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.Jobs["J-2"].Status != "accepted" || st.Jobs["J-3"].Status != "assigned" || st.Questions[q.Post].Open {
		t.Fatalf("J-2 %s, J-3 %s, question open %v", st.Jobs["J-2"].Status, st.Jobs["J-3"].Status, st.Questions[q.Post].Open)
	}
	file, _ = os.ReadFile(filepath.Join(path, "questions", "Q-1.md"))
	if !strings.Contains(string(file), "Answer: json") || !strings.Contains(string(file), `"state": "answered"`) {
		t.Fatalf("file did not follow the Feed: %s", file)
	}
}

// withdraw closes the person Job, so the Jobs behind it carry on.
func TestQuestionWithdrawClosesTheJob(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	epoch := os.Getenv("GRID_LEASE_EPOCH")
	q := askJob(t, h, path)
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-3", "--body", personBody(t, "J-3", []any{q.Job})); code != 0 {
		t.Fatalf("post %d: %s", code, h.errb)
	}
	if slices.Contains(readyNow(t, store), "J-3") {
		t.Fatal("J-3 is ready while its question is open")
	}
	if code := h.run(path, "question", "withdraw", q.ID, "--reason", "Settled in the call"); code != 0 {
		t.Fatalf("withdraw %d: %s", code, h.errb)
	}
	asHolder(t, epoch)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("pass %d: %s", code, h.errb)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.Jobs["J-2"].Status != "accepted" || st.Jobs["J-3"].Status != "assigned" || st.Questions[q.Post].Open {
		t.Fatalf("J-2 %s, J-3 %s, question open %v", st.Jobs["J-2"].Status, st.Jobs["J-3"].Status, st.Questions[q.Post].Open)
	}
	file, _ := os.ReadFile(filepath.Join(path, "questions", "Q-1.md"))
	if !strings.Contains(string(file), `"state": "withdrawn"`) {
		t.Fatalf("file did not follow the Feed: %s", file)
	}
}

// An agent without a lease posts no Job.
func TestQuestionAskByAgentPostsNoJob(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "agent")
	t.Setenv("GRID_LEASE_EPOCH", "")
	args := []string{"question", "ask", "Which export format?", "--who", "dana", "--default", "csv", "--applies", "2099-01-01T00:00:00Z", "--json"}
	// The Feed already refuses an agent a person gate (the same on main for add);
	// ask adds no refusal of its own and leaves no half-posted Job behind.
	if code := h.run(path, args...); code == 0 || !strings.Contains(h.errb.String(), "may not post person gates") {
		t.Fatalf("ask %d: %s", code, h.errb)
	}
	jobs, _ := store.Read(feed.Filter{Type: "job"})
	if len(jobs.Records) != 1 {
		t.Fatalf("want only the harness Job, got %d", len(jobs.Records))
	}
}
