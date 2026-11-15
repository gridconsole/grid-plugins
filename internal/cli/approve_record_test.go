package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

const arPlan = "# Plan\n\nShip the parser.\n"

// arTaskFiles writes the files of a waiting task: plan.md, intent.md and a task.md
// that carries the Feed identity when ulid is set.
func arTaskFiles(t *testing.T, path string, ulid bool) {
	t.Helper()
	id := "id: T-07\n"
	if ulid {
		id += "ulid: " + feedTaskID + "\n"
	}
	md := "---\n" + id + "title: Example\nproject: example\nstate: waiting\ncreated: 2026-11-10T08:00:00Z\nupdated: 2026-11-10T08:00:00Z\n---\n"
	for name, text := range map[string]string{"plan.md": arPlan, "intent.md": "# Intent\n\nShip the parser.\n", "task.md": md} {
		if err := os.WriteFile(filepath.Join(path, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// arWaitingTask is a waiting task whose Feed holds the task lease (the epoch is in
// the environment) and one ordinary Job, J-1, but no approve person Job.
func arWaitingTask(t *testing.T) (*harness, string, feed.Store) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	arTaskFiles(t, path, true)
	return h, path, store
}

// arReceipt is the text of an old approval.json for arPlan.
func arReceipt(t *testing.T, patch map[string]any) string {
	t.Helper()
	m := map[string]any{
		"task": "T-07", "who": map[string]any{"id": "dana", "role": "member"}, "when": "2026-11-01T09:00:00Z",
		"what": "approve", "from_state": "waiting", "to_state": "waiting", "note": "looks right",
		"plan_hash": plan.Hash(arPlan), "plan_digest": plan.Digest(arPlan),
	}
	for k, v := range patch {
		m[k] = v
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func arWriteReceipt(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(path, "approval.json"), []byte(arReceipt(t, nil)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// arReceiptStays fails when the folder has no approval.json: grid approve keeps it
// beside the Job result, and the migration of an old receipt leaves it.
func arReceiptStays(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(path, "approval.json")); err != nil {
		t.Fatalf("approval.json is missing: %v", err)
	}
}

// arApproveJobs is the person Jobs of kind approve in the Feed, by id.
func arApproveJobs(t *testing.T, store feed.Store) map[string]map[string]any {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	jobs := map[string]map[string]any{}
	for _, e := range r.Records {
		if e.Body["kind"] == "approve" && e.Body["role"] == "Person" {
			jobs[e.Body["id"].(string)] = e.Body
		}
	}
	return jobs
}

// arAnswers is the answer records of the Feed.
func arAnswers(t *testing.T, store feed.Store) []feed.Entry {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: "answer"})
	if err != nil {
		t.Fatal(err)
	}
	return r.Records
}

// arAccepts is the bodies of the controls that accepted job.
func arAccepts(t *testing.T, store feed.Store, job string) []map[string]any {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, e := range r.Records {
		if e.Body["target"] == job && e.Body["status"] == "accepted" {
			out = append(out, e.Body)
		}
	}
	return out
}

func arStatus(t *testing.T, store feed.Store, job string) string {
	t.Helper()
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.Jobs[job] == nil {
		return ""
	}
	return string(st.Jobs[job].Status)
}

// arResultIs fails unless body is an approval result of arPlan carrying note.
func arResultIs(t *testing.T, body map[string]any, note string) {
	t.Helper()
	if body == nil || body["plan_hash"] != plan.Hash(arPlan) || body["plan_digest"] != plan.Digest(arPlan) || body["note"] != note {
		t.Fatalf("result body %v", body)
	}
	if _, ok := body["lands_hash"]; ok {
		t.Fatalf("this task has no lands rule, so no lands_hash: %v", body)
	}
}

// grid approve with a lease epoch answers the open approve person Job in one
// accepting control that carries plan_hash, plan_digest and the note; the Job
// accepts, no Job is added and no approval.json is written.
func TestApproveRecordOpenJobAnswered(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--note", "checked"); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	accepts := arAccepts(t, store, "J-2")
	if len(accepts) != 1 || accepts[0]["answer"] != "approved" {
		t.Fatalf("accepting controls %v", accepts)
	}
	arResultIs(t, accepts[0], "checked")
	if jobs := arApproveJobs(t, store); len(jobs) != 1 || jobs["J-2"] == nil {
		t.Fatalf("approve Jobs %v", jobs)
	}
	if s := arStatus(t, store, "J-2"); s != "accepted" {
		t.Fatalf("J-2 is %s", s)
	}
	arReceiptStays(t, path)
	history, err := os.ReadFile(filepath.Join(path, "history.md"))
	if err != nil || !strings.Contains(string(history), `"what":"approve"`) {
		t.Fatalf("history line: %v %s", err, history)
	}
}

// A task with no Feed identity and no Job: grid approve by a person with no lease
// gives the task its ULID, posts the approve person Job and answers it, with the
// plan revision the person saw.
func TestApproveRecordNoUlidNoJobCreatesBoth(t *testing.T) {
	h, path := feedHarness(t)
	arTaskFiles(t, path, false)
	h.person()
	if _, err := feed.TaskID(path); !errors.Is(err, feed.ErrNoTaskID) {
		t.Fatalf("the fixture has a Feed identity: %v", err)
	}
	if code := h.run(path, "approve", path, "--note", "first look"); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	id, err := feed.TaskID(path)
	if err != nil || id == "" {
		t.Fatalf("no Feed identity after grid approve: %q %v", id, err)
	}
	store := feed.Store{TaskDir: path, TaskID: id}
	jobs := arApproveJobs(t, store)
	if len(jobs) != 1 || jobs["J-1"] == nil {
		t.Fatalf("approve Jobs %v", jobs)
	}
	answers := arAnswers(t, store)
	if len(answers) != 1 || answers[0].Author.ID != "sam" || answers[0].Body["answer"] != "approved" {
		t.Fatalf("answers %+v", answers)
	}
	arResultIs(t, answers[0].Body, "first look")
	arReceiptStays(t, path)
}

// A person with no lease epoch posts an unleased answer that carries the plan
// revision; nobody has accepted the Job, so the readers see no approval yet. The
// next tick of the task lease holder copies the fields into the accepting control.
func TestApproveRecordPersonWithoutLeaseWaitsForTheHoldersTick(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	h.person()
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--note", "checked"); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	answers := arAnswers(t, store)
	if len(answers) != 1 || answers[0].Author.ID != "sam" {
		t.Fatalf("answers %+v", answers)
	}
	arResultIs(t, answers[0].Body, "checked")
	if got := arAccepts(t, store, "J-2"); len(got) != 0 {
		t.Fatalf("a person with no lease wrote a control: %v", got)
	}
	if _, err := taskstore.ApprovalFromJob(path); !errors.Is(err, taskstore.ErrPlanJobOpen) {
		t.Fatalf("before the tick: %v", err)
	}
	// The holder's pass: the harness holds the task lease through the environment.
	h.env.Getenv = nil
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatalf("tick: %d %s", code, h.errb.String())
	}
	accepts := arAccepts(t, store, "J-2")
	if len(accepts) != 1 {
		t.Fatalf("accepting controls %v", accepts)
	}
	arResultIs(t, accepts[0], "checked")
	a, err := taskstore.ApprovalFromJob(path)
	if err != nil || a.Job != "J-2" || a.PlanDigest != plan.Digest(text) || a.Note != "checked" {
		t.Fatalf("%+v %v", a, err)
	}
	arReceiptStays(t, path)
}

// grid job result --plan-hash answers the named person Job: with a lease epoch in
// one accepting control, with a person's no-lease writer as an answer that the
// holder's tick turns into the control. Both carry the answer text and the note.
func TestApproveRecordJobResultPlanHashLeasedAndPerson(t *testing.T) {
	body := `{"answer":"Go ahead.","note":"checked"}`
	t.Run("leased", func(t *testing.T) {
		h, path, store, text := approveJobHarness(t)
		if code := h.run(path, "job", "result", "J-2", "--plan-hash", plan.Hash(text), "--body", body); code != 0 {
			t.Fatalf("%d %s", code, h.errb.String())
		}
		accepts := arAccepts(t, store, "J-2")
		if len(accepts) != 1 || accepts[0]["answer"] != "Go ahead." {
			t.Fatalf("accepting controls %v", accepts)
		}
		arResultIs(t, accepts[0], "checked")
		if len(arAnswers(t, store)) != 0 {
			t.Fatal("a leased approval wrote an answer record")
		}
		arReceiptStays(t, path)
	})
	t.Run("person without a lease", func(t *testing.T) {
		h, path, store, text := approveJobHarness(t)
		h.person()
		if code := h.run(path, "job", "result", "J-2", "--plan-hash", plan.Digest(text), "--body", body); code != 0 {
			t.Fatalf("%d %s", code, h.errb.String())
		}
		answers := arAnswers(t, store)
		if len(answers) != 1 || answers[0].Author.ID != "sam" || answers[0].Body["answer"] != "Go ahead." {
			t.Fatalf("answers %+v", answers)
		}
		arResultIs(t, answers[0].Body, "checked")
		if got := arAccepts(t, store, "J-2"); len(got) != 0 {
			t.Fatalf("a person with no lease wrote a control: %v", got)
		}
		h.env.Getenv = nil
		if code := h.run(path, "job", "next"); code != 0 {
			t.Fatalf("tick: %d %s", code, h.errb.String())
		}
		accepts := arAccepts(t, store, "J-2")
		if len(accepts) != 1 {
			t.Fatalf("accepting controls %v", accepts)
		}
		arResultIs(t, accepts[0], "checked")
		if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-2" {
			t.Fatalf("%+v %v", a, err)
		}
		arReceiptStays(t, path)
	})
}

// An old approval.json beside no approve Job is moved by grid approve into a new
// person Job and its accepting result, and the file is deleted. Beside an existing
// approve Job it is retired without answering that Job: a stale hash then
// refuses, and the Job is still open.
func TestApproveRecordOldReceiptMovedOrRetired(t *testing.T) {
	t.Run("no Job: moved into a Job", func(t *testing.T) {
		h, path, store := arWaitingTask(t)
		arWriteReceipt(t, path)
		if code := h.run(path, "approve", path); code != 0 {
			t.Fatalf("%d %s", code, h.errb.String())
		}
		arReceiptStays(t, path)
		jobs := arApproveJobs(t, store)
		if len(jobs) != 1 || jobs["J-2"] == nil {
			t.Fatalf("approve Jobs %v (J-1 is the ordinary Job)", jobs)
		}
		accepts := arAccepts(t, store, "J-2")
		if len(accepts) != 1 || !strings.Contains(accepts[0]["reason"].(string), "moved from approval.json") {
			t.Fatalf("accepting controls %v", accepts)
		}
		arResultIs(t, accepts[0], "looks right")
		if s := arStatus(t, store, "J-2"); s != "accepted" {
			t.Fatalf("J-2 is %s", s)
		}
		if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-2" {
			t.Fatalf("%+v %v", a, err)
		}
	})
	t.Run("an approve Job exists: retired, Job left open", func(t *testing.T) {
		h, path, store, _ := approveJobHarness(t)
		arWriteReceipt(t, path)
		if code := h.run(path, "approve", path, "--plan-hash", "000000"); code != 1 || !strings.Contains(h.errb.String(), "plan hash no longer matches") {
			t.Fatalf("%d %s", code, h.errb.String())
		}
		arReceiptStays(t, path)
		if got := arAccepts(t, store, "J-2"); len(got) != 0 {
			t.Fatalf("the receipt answered the Job: %v", got)
		}
		if s := arStatus(t, store, "J-2"); s == "accepted" {
			t.Fatal("J-2 is accepted")
		}
		if jobs := arApproveJobs(t, store); len(jobs) != 1 {
			t.Fatalf("approve Jobs %v", jobs)
		}
	})
}

// A worker's shell with no GRID_MEMBER and no lease epoch is refused with exit 3 and
// nothing is written: no Job result, no answer, no receipt, no history line. A
// person's own terminal with no GRID_MEMBER is not refused: it approves as the
// local owner (TestR23ApproveNeedsNoGridMember); this test used the empty
// environment for the refused case before that, and now names the worker mark.
func TestApproveRecordCallerWithoutMemberWritesNothing(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	before, _ := os.ReadFile(filepath.Join(path, "history.md"))
	// grid job result takes its writer from feedWriter, which falls back to the git
	// or OS user for a person, as grid job post --person does; grid approve is
	// person-only, so it is the one command refused here.
	h.env.Getenv = func(key string) string {
		if key == "GRID_AGENT" {
			return "worker-1"
		}
		return ""
	}
	for _, args := range [][]string{
		{"approve", path, "--plan-hash", plan.Hash(text)},
		{"approve", path},
	} {
		code := h.run(path, args...)
		if code != 3 || !strings.Contains(h.errb.String(), "person of the team") {
			t.Fatalf("%v: %d %s", args, code, h.errb.String())
		}
		if got := arAccepts(t, store, "J-2"); len(got) != 0 {
			t.Fatalf("%v wrote a result %v", args, got)
		}
		if got := arAnswers(t, store); len(got) != 0 {
			t.Fatalf("%v wrote an answer %+v", args, got)
		}
		if jobs := arApproveJobs(t, store); len(jobs) != 1 {
			t.Fatalf("%v changed the Jobs %v", args, jobs)
		}
		if _, err := os.Lstat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%v wrote approval.json: %v", args, err)
		}
		after, _ := os.ReadFile(filepath.Join(path, "history.md"))
		if string(after) != string(before) {
			t.Fatalf("%v wrote history: %s", args, after)
		}
	}
}

// The worker pass moves the old approval.json of a task that is only read (no
// approve Job, no open Job) into its approve person Job, under the task lease the
// pass takes, so the approval takes effect at once. This runs the real
// `grid worker next` in a child process, as TestWorkerNextProcess does.
func TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead(t *testing.T) {
	h, path := feedHarness(t)
	arTaskFiles(t, path, true)
	arWriteReceipt(t, path)
	store := feed.Store{TaskDir: path, TaskID: feedTaskID}
	// A second task with a posted Job gives the pass an event to wait on: one pass walks the task
	// folders in name order, so T-08-second is handled, and its spawn line printed, after T-07-example.
	second := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-08-second")
	h.write(filepath.Join(second, "task.md"), "---\nid: "+secondTaskID+"\n---\n")
	ownerPost(t, second, secondTaskID, "J-1")
	pid, start := taskLeaseIdentity(t)

	var stderr bytes.Buffer
	cmd, lines := koWnStart(t, h.gridHome, pid, start, &stderr)
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if sp := koWnSpawn(t, lines, &stderr); sp["task"] != secondTaskID {
		t.Fatalf("the first spawn line is not T-08-second's: %v\nstderr: %s", sp, &stderr)
	}
	// The pass has moved the receipt of T-07-example, so one read of the Feed is enough.
	jobs := arApproveJobs(t, store)
	if len(jobs) != 1 || jobs["J-1"] == nil {
		t.Fatalf("approve Jobs %v", jobs)
	}
	accepts := arAccepts(t, store, "J-1")
	if len(accepts) != 1 || !strings.Contains(accepts[0]["reason"].(string), "moved from approval.json") {
		t.Fatalf("accepting controls %v", accepts)
	}
	arResultIs(t, accepts[0], "looks right")
	if s := arStatus(t, store, "J-1"); s != "accepted" {
		t.Fatalf("J-1 is %s", s)
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-1" || a.PlanDigest != plan.Digest(arPlan) {
		t.Fatalf("%+v %v", a, err)
	}
}
