package task

import (
	"encoding/json"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// This file tests the readers that turn the plan Job's result into an approval
// (planJobResult, ApprovalFromJob, ApprovalForGate, JobApproval.event), MarkBrief
// and machineOwner. The package cannot import orchestrator, job or cli (cycle:
// job imports task), so the plan Job and its result are posted with the exported
// Feed API alone, the way internal/feed's own tests do. The Q10 receipt fallback
// (approval.json) is not tested here and no test depends on it.

const approvalTestMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

// approvalFeed opens the Feed of a task and returns the store and a member
// writer that holds the task lease at fence 1, after the handover. This is the
// part of orchestrator.Write that this package can do.
func approvalFeed(t *testing.T, task Task) (feed.Store, feed.Writer) {
	t.Helper()
	id, _, err := FeedIdentity(task.Path)
	if err != nil {
		t.Fatal(err)
	}
	store := feed.Store{TaskDir: task.Path, TaskID: id}
	w := feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: "dana@" + approvalTestMachine, Lease: "task", Fence: 1}
	dir := filepath.Join(store.Dir(), "lease")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(feed.Lease{Lease: w.Lease, Holder: &w.Worker, Fence: w.Fence, Gen: w.Gen})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, w.Lease+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	return store, w
}

// approvalJobBody is the body of a person Job, the shape internal/question/job.go
// posts for a question, with the role, kind and continues of the test's choice.
func approvalJobBody(role, kind, continues string) map[string]any {
	var cont any
	if continues != "" {
		cont = continues
	}
	return map[string]any{
		"item": 1, "title": "Approve the plan", "role": role, "kind": kind,
		"skills": []any{}, "review": "L0", "risk": []any{}, "max_rounds": 1,
		"intent": map[string]any{"owner": "dana", "item": "plan.md", "goal": "Approve the plan"},
		"links":  []any{}, "keep": []any{}, "decisions": []any{}, "read": []any{}, "write": []any{}, "out_of_scope": []any{},
		"acceptance": []any{map[string]any{"id": "A1", "text": "dana approves the plan", "traces": "item"}},
		"verify":     map[string]any{"kind": "file-exists", "path": "plan.md"},
		"budget":     map[string]any{"turns_estimate": 1, "reads_before_first_edit": 0},
		"on_blocked": "ask-and-continue", "after": []any{},
		"attempt": 1, "continues": cont, "discovered_from": nil, "close_reason": nil, "checkpoint": nil,
		"review_rounds": 0, "level_raised": nil, "worktree": "none", "posts_back": []any{},
	}
}

// postApproveJob posts the person Job of the approve step.
func postApproveJob(t *testing.T, store feed.Store, w feed.Writer, job, continues string) {
	t.Helper()
	postJobRecord(t, store, w, job, "Person", "approve", continues)
}

func postJobRecord(t *testing.T, store feed.Store, w feed.Writer, job, role, kind, continues string) {
	t.Helper()
	body := approvalJobBody(role, kind, continues)
	body["id"] = job
	if _, err := store.Post(feed.Input{Type: "job", Job: job, Body: body}, w); err != nil {
		t.Fatalf("post Job %s (%s/%s): %v", job, role, kind, err)
	}
	tick()
}

// acceptControl is the control the approve command posts: the Job accepted, with
// the plan hash and digest of text and an answer that tells the results apart.
func acceptControl(store feed.Store, w feed.Writer, job, text, answer string) (feed.Record, error) {
	body := map[string]any{"target": job, "status": "accepted", "reason": "approved", "answer": answer, "plan_hash": plan.Hash(text), "plan_digest": plan.Digest(text)}
	r, err := store.Post(feed.Input{Type: "control", Body: body}, w)
	tick()
	return r, err
}

func mustAccept(t *testing.T, store feed.Store, w feed.Writer, job, text, answer string) feed.Record {
	t.Helper()
	r, err := acceptControl(store, w, job, text, answer)
	if err != nil {
		t.Fatalf("accept Job %s: %v", job, err)
	}
	return r
}

// tick keeps two posts out of one millisecond: a ULID is ordered by time first
// and by random bits inside one millisecond, and the readers pick the greatest.
func tick() { time.Sleep(3 * time.Millisecond) }

func jobStatus(t *testing.T, store feed.Store, job string) string {
	t.Helper()
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs[job]
	if j == nil {
		t.Fatalf("Job %s is not in the folded state", job)
	}
	return string(j.Status)
}

// approvedFixture is a waiting task with planV1 on disk and a plan Job J-1 that
// dana accepted for planV1.
func approvedFixture(t *testing.T) (Store, Task, feed.Store, feed.Writer, feed.Record) {
	t.Helper()
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	store, w := approvalFeed(t, task)
	postApproveJob(t, store, w, "J-1", "")
	accept := mustAccept(t, store, w, "J-1", planV1, "approved")
	return s, task, store, w, accept
}

// The helper is the base of every approval row: after its two posts the Feed
// fold shows the Job accepted, which planJobResult requires (gates.go:293).
func TestApprovalReadHelperFoldsJobAccepted(t *testing.T) {
	_, task := waitingFixture(t)
	store, w := approvalFeed(t, task)
	postApproveJob(t, store, w, "J-1", "")
	if got := jobStatus(t, store, "J-1"); got == "accepted" {
		t.Fatalf("Job is accepted before its result is posted")
	}
	r := mustAccept(t, store, w, "J-1", planV1, "approved")
	if got := jobStatus(t, store, "J-1"); got != "accepted" {
		t.Fatalf("Job folds as %q after the accepted control, want accepted", got)
	}
	if r.ID == "" || r.Type != "control" {
		t.Fatalf("control record: %+v", r)
	}
}

// Row 1: the plan Job's accepted result of the current plan.md is the approval,
// and the gate name is ignored.
func TestApprovalReadFromPlanJob(t *testing.T) {
	s, task, _, _, accept := approvedFixture(t)
	got, err := ApprovalFromJob(task.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Job != "J-1" || got.Record != accept.ID || got.Who.ID != "dana" || got.Answer != "approved" || got.PlanHash != plan.Hash(planV1) || got.PlanDigest != plan.Digest(planV1) {
		t.Fatalf("JobApproval: %+v (control %s)", got, accept.ID)
	}
	if got.Who != accept.Author || got.When != accept.At {
		t.Fatalf("Who/When come from the Feed record: %+v vs %+v %s", got, accept.Author, accept.At)
	}
	want, err := s.ApprovalForGate("", task.ID, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if want.What != "approve" || want.Task != task.ID || want.PlanDigest != got.PlanDigest || want.PlanHash != got.PlanHash || want.Who != got.Who || want.When != got.When {
		t.Fatalf("plan gate event: %+v", want)
	}
	for _, gate := range []string{"ship", "preview", "anything"} {
		ev, err := s.ApprovalForGate("", task.ID, gate)
		if err != nil {
			t.Fatalf("gate %q: %v", gate, err)
		}
		if !reflect.DeepEqual(ev, want) {
			t.Fatalf("gate %q: %+v, want %+v", gate, ev, want)
		}
	}
	if ev, err := s.Approval("", task.ID); err != nil || !reflect.DeepEqual(ev, want) {
		t.Fatalf("Approval: %+v %v", ev, err)
	}
	if !Approved(task.Path) || !ApprovedDigest(task.Path, plan.Hash(planV1), plan.Digest(planV1)) {
		t.Fatal("Approved and ApprovedDigest do not agree with the plan Job's result")
	}
}

// Row 2: a plan edited after the approval is not approved.
func TestApprovalReadChangedSinceApproved(t *testing.T) {
	s, task, _, _, _ := approvedFixture(t)
	setPlan(t, task, planV2)
	got, err := ApprovalFromJob(task.Path)
	if err == nil || err.Error() != errChangedSinceApproved.Error() || !errors.Is(err, errChangedSinceApproved) {
		t.Fatalf("ApprovalFromJob: %+v %v", got, err)
	}
	if !strings.Contains(err.Error(), "Changed since you approved") {
		t.Fatalf("error text: %v", err)
	}
	for _, gate := range []string{"plan", "ship", "preview"} {
		ev, err := s.ApprovalForGate("", task.ID, gate)
		if !errors.Is(err, errChangedSinceApproved) {
			t.Fatalf("gate %q after an edit: %+v %v", gate, ev, err)
		}
	}
	if Approved(task.Path) {
		t.Fatal("Approved after an edit")
	}
	// A plan.md that is gone matches no approval either.
	if err := os.Remove(filepath.Join(task.Path, "plan.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := ApprovalFromJob(task.Path); err == nil {
		t.Fatal("ApprovalFromJob with no plan.md")
	}
	if _, err := s.ApprovalForGate("", task.ID, "plan"); !errors.Is(err, plan.ErrNoPlan) {
		t.Fatalf("ApprovalForGate with no plan.md: %v", err)
	}
}

// Row 3: when a second accepted result follows a second approve, the greatest
// record id wins. If the Feed refuses a second accepted result on one Job, the
// refusal is pinned instead.
func TestApprovalReadLatestResultWins(t *testing.T) {
	_, task, store, w, first := approvedFixture(t)
	setPlan(t, task, planV2)
	second, err := acceptControl(store, w, "J-1", planV2, "second")
	if err != nil {
		t.Fatalf("the Feed refuses a second accepted result on one Job: %v", err)
	}
	if second.ID <= first.ID {
		t.Fatalf("second record %s is not after the first %s", second.ID, first.ID)
	}
	got, err := ApprovalFromJob(task.Path)
	if err != nil {
		t.Fatalf("the later result is of the current plan.md: %v", err)
	}
	if got.Record != second.ID || got.Answer != "second" || got.PlanDigest != plan.Digest(planV2) {
		t.Fatalf("returned %+v, want the later record %s", got, second.ID)
	}
	// And the other way round: the older result is the one that no longer matches.
	setPlan(t, task, planV1)
	if got, err := ApprovalFromJob(task.Path); !errors.Is(err, errChangedSinceApproved) || got.Record != second.ID {
		t.Fatalf("plan back to v1: %+v %v", got, err)
	}
}

// Row 4: a reopen posts a plan Job that continues the first. Results recorded
// before it no longer approve the plan; the new Job's own result does.
func TestApprovalReadReopenVoidsOlderResults(t *testing.T) {
	s, task, store, w, _ := approvedFixture(t)
	postApproveJob(t, store, w, "J-2", "J-1")
	got, err := planJobResult(task.Path)
	if err == nil || !strings.Contains(err.Error(), "the plan Job J-2 has no result yet; run grid approve to post it") || strings.Contains(err.Error(), "J-1") {
		t.Fatalf("after the reopen: %+v %v", got, err)
	}
	if got.Record != "" {
		t.Fatalf("an old result still approves the plan: %+v", got)
	}
	if _, err := ApprovalFromJob(task.Path); err == nil || !strings.Contains(err.Error(), "J-2") {
		t.Fatalf("ApprovalFromJob after the reopen: %v", err)
	}
	if _, err := s.ApprovalForGate("", task.ID, "plan"); err == nil || !strings.Contains(err.Error(), "the plan Job J-2 has no result yet") {
		t.Fatalf("ApprovalForGate after the reopen: %v", err)
	}
	if Approved(task.Path) {
		t.Fatal("Approved after the reopen, before its result")
	}
	accept := mustAccept(t, store, w, "J-2", planV1, "again")
	got, err = ApprovalFromJob(task.Path)
	if err != nil || got.Job != "J-2" || got.Record != accept.ID || got.Answer != "again" {
		t.Fatalf("after the new Job is accepted: %+v %v", got, err)
	}
}

// Row 5: what is not a plan approval.
func TestApprovalReadNotAPlanApproval(t *testing.T) {
	t.Run("accepted control without a plan_digest", func(t *testing.T) {
		_, task := waitingFixture(t)
		store, w := approvalFeed(t, task)
		postApproveJob(t, store, w, "J-1", "")
		body := map[string]any{"target": "J-1", "status": "accepted", "reason": "answered", "answer": "yes"}
		if _, err := store.Post(feed.Input{Type: "control", Body: body}, w); err != nil {
			t.Fatal(err)
		}
		if got := jobStatus(t, store, "J-1"); got != "accepted" {
			t.Fatalf("Job folds as %q, want accepted", got)
		}
		got, err := planJobResult(task.Path)
		if err == nil || errors.Is(err, ErrNoPlanJob) || !strings.Contains(err.Error(), "the plan Job J-1 has no result yet; run grid approve to post it") {
			t.Fatalf("%+v %v", got, err)
		}
	})
	t.Run("a Job of another kind", func(t *testing.T) {
		_, task := waitingFixture(t)
		store, w := approvalFeed(t, task)
		postJobRecord(t, store, w, "J-1", "Person", "review", "")
		mustAccept(t, store, w, "J-1", planV1, "approved")
		if got := jobStatus(t, store, "J-1"); got != "accepted" {
			t.Fatalf("Job folds as %q, want accepted", got)
		}
		if _, err := planJobResult(task.Path); !errors.Is(err, ErrNoPlanJob) {
			t.Fatalf("a review Job approved the plan: %v", err)
		}
		if Approved(task.Path) {
			t.Fatal("Approved counted the review Job")
		}
	})
	t.Run("an approve Job whose role is not Person", func(t *testing.T) {
		_, task := waitingFixture(t)
		store, w := approvalFeed(t, task)
		postJobRecord(t, store, w, "J-1", "builder", "approve", "")
		mustAccept(t, store, w, "J-1", planV1, "approved")
		if _, err := planJobResult(task.Path); !errors.Is(err, ErrNoPlanJob) {
			t.Fatalf("a Builder approve Job approved the plan: %v", err)
		}
	})
	t.Run("no Job in the Feed", func(t *testing.T) {
		_, task := waitingFixture(t)
		approvalFeed(t, task)
		if _, err := planJobResult(task.Path); !errors.Is(err, ErrNoPlanJob) {
			t.Fatalf("empty Feed: %v", err)
		}
		if _, err := ApprovalFromJob(task.Path); !errors.Is(err, ErrNoPlanJob) {
			t.Fatalf("ApprovalFromJob on an empty Feed: %v", err)
		}
	})
	t.Run("no Feed at all", func(t *testing.T) {
		_, task := waitingFixture(t)
		if _, err := planJobResult(task.Path); !errors.Is(err, ErrNoPlanJob) {
			t.Fatalf("task with no Feed: %v", err)
		}
	})
	t.Run("task.md without a ULID", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "task.md"), "---\nid: T-01\nstate: waiting\nintent: Example\n---\n")
		if _, err := planJobResult(dir); !errors.Is(err, ErrNoPlanJob) {
			t.Fatalf("task.md without a ULID: %v", err)
		}
		if _, err := ApprovalFromJob(dir); !errors.Is(err, ErrNoPlanJob) {
			t.Fatalf("ApprovalFromJob without a ULID: %v", err)
		}
	})
}

// Row 7: the receipt Event of a JobApproval.
func TestApprovalReadEventMapsEveryField(t *testing.T) {
	a := JobApproval{
		Job: "J-9", Record: "01ARZ3NDEKTSV4RRFFQ69G5FAX", Who: contract.Author{ID: "dana", Role: "member"},
		When: "2026-11-11T10:00:00.000Z", Answer: "approved", PlanHash: "hash-1", PlanDigest: "digest-1",
		LandsHash: "lands-1", Note: "Reviewed",
	}
	got := a.event(Task{ID: "T-7"})
	want := Event{Task: "T-7", Who: a.Who, When: a.When, What: "approve", Note: "Reviewed", LandsHash: "lands-1", PlanHash: "hash-1", PlanDigest: "digest-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event: %+v, want %+v", got, want)
	}
	if got.What != "approve" || got.Task != "T-7" || got.Who.ID != "dana" || got.Gate != "" {
		t.Fatalf("event fields: %+v", got)
	}
}

func briefDir(t *testing.T) (dir, sub string) {
	t.Helper()
	dir = t.TempDir()
	sub = filepath.Join(dir, "subagents")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, sub
}

func readBrief(t *testing.T, sub, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(sub, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Row 8: MarkBrief advances a briefed or running brief and keeps every other byte.
func TestMarkBriefAdvancesUnfinishedBrief(t *testing.T) {
	for _, from := range []string{"running", "briefed"} {
		for _, to := range []string{"done", "blocked"} {
			t.Run(from+" to "+to, func(t *testing.T) {
				dir, sub := briefDir(t)
				before := "---\nid: B-1\nowner: dana\nstatus: " + from + "\nupdated: 2026-11-11\n---\n\nBody keeps status: " + from + "\n"
				write(t, filepath.Join(sub, "b.md"), before)
				if err := MarkBrief(dir, "b.md", to); err != nil {
					t.Fatal(err)
				}
				want := strings.Replace(before, "status: "+from+"\nupdated", "status: "+to+"\nupdated", 1)
				if got := readBrief(t, sub, "b.md"); got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			})
		}
	}
}

// A brief that is already done or blocked is left as is, and says nothing.
func TestMarkBriefKeepsFinishedBrief(t *testing.T) {
	for _, from := range []string{"done", "blocked"} {
		dir, sub := briefDir(t)
		before := "---\nid: B-1\nstatus: " + from + "\n---\nBody\n"
		write(t, filepath.Join(sub, "b.md"), before)
		for _, to := range []string{"blocked", "done"} {
			if err := MarkBrief(dir, "b.md", to); err != nil {
				t.Fatalf("%s to %s: %v", from, to, err)
			}
			if got := readBrief(t, sub, "b.md"); got != before {
				t.Fatalf("%s to %s changed the brief: %q", from, to, got)
			}
		}
	}
}

// A front matter with no status key is left as is. The status line must start at
// column 0 and be unquoted for the parser to read it; the forms it does not read
// are left alone, and the form it reads is also the form the regexp rewrites, so
// the "brief status is missing" branch (brief.go:62-65) cannot be reached.
func TestMarkBriefNoStatusKeyIsLeftAlone(t *testing.T) {
	cases := map[string]string{
		"no key":       "---\nid: B-1\n---\nBody\n",
		"empty status": "---\nid: B-1\nstatus:\n---\nBody\n",
		"quoted key":   "---\nid: B-1\n\"status\": running\n---\nBody\n",
		"indented key": "---\nid: B-1\n  status: running\n---\nBody\n",
		"other status": "---\nid: B-1\nstatus: paused\n---\nBody\n",
	}
	for name, before := range cases {
		t.Run(name, func(t *testing.T) {
			dir, sub := briefDir(t)
			write(t, filepath.Join(sub, "b.md"), before)
			if err := MarkBrief(dir, "b.md", "done"); err != nil {
				t.Fatalf("got %v, want nil", err)
			}
			if got := readBrief(t, sub, "b.md"); got != before {
				t.Fatalf("changed the brief: %q", got)
			}
		})
	}
	// A separator the parser trims and the regexp passes over as part of the
	// value still matches: the regexp finds every line the parser reads as status.
	t.Run("odd separator is rewritten", func(t *testing.T) {
		dir, sub := briefDir(t)
		write(t, filepath.Join(sub, "b.md"), "---\nid: B-1\nstatus: running\n---\nBody\n")
		if err := MarkBrief(dir, "b.md", "done"); err != nil {
			t.Fatal(err)
		}
		if got := readBrief(t, sub, "b.md"); got != "---\nid: B-1\nstatus: done\n---\nBody\n" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestMarkBriefRefusals(t *testing.T) {
	t.Run("name and state", func(t *testing.T) {
		dir, sub := briefDir(t)
		before := "---\nstatus: running\n---\n"
		write(t, filepath.Join(sub, "b.md"), before)
		for _, tc := range []struct{ name, state string }{
			{"../x.md", "running"}, {"../x.md", "done"}, {"sub/b.md", "done"}, {"b.md", "running"}, {"b.md", "briefed"}, {"b.md", ""},
		} {
			err := MarkBrief(dir, tc.name, tc.state)
			if err == nil || err.Error() != "invalid brief transition" {
				t.Fatalf("%q to %q: %v", tc.name, tc.state, err)
			}
		}
		if got := readBrief(t, sub, "b.md"); got != before {
			t.Fatalf("a refused call changed the brief: %q", got)
		}
	})
	t.Run("subagents is a symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("creating a symlink needs a privilege that a Windows test host may not give")
		}
		dir := t.TempDir()
		real := filepath.Join(t.TempDir(), "real")
		if err := os.Mkdir(real, 0o755); err != nil {
			t.Fatal(err)
		}
		before := "---\nstatus: running\n---\n"
		write(t, filepath.Join(real, "b.md"), before)
		if err := os.Symlink(real, filepath.Join(dir, "subagents")); err != nil {
			t.Skipf("cannot make a symlink here: %v", err)
		}
		err := MarkBrief(dir, "b.md", "done")
		if err == nil || err.Error() != "brief folder must be a real directory" {
			t.Fatalf("got %v", err)
		}
		if got := readBrief(t, real, "b.md"); got != before {
			t.Fatalf("changed a brief behind a symlink: %q", got)
		}
	})
	t.Run("subagents is a file", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "subagents"), "not a folder")
		err := MarkBrief(dir, "b.md", "done")
		if err == nil || err.Error() != "brief folder must be a real directory" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("missing folder and missing brief", func(t *testing.T) {
		dir := t.TempDir()
		if err := MarkBrief(dir, "b.md", "done"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("no subagents folder: %v", err)
		}
		dir, _ = briefDir(t)
		if err := MarkBrief(dir, "b.md", "done"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("no brief: %v", err)
		}
	})
	t.Run("front matter", func(t *testing.T) {
		for name, before := range map[string]string{
			"none":         "Just text, status: running\n",
			"unterminated": "---\nstatus: running\nBody\n",
			"odd key":      "---\n__proto__: x\nstatus: running\n---\nBody\n",
		} {
			dir, sub := briefDir(t)
			write(t, filepath.Join(sub, "b.md"), before)
			err := MarkBrief(dir, "b.md", "done")
			if err == nil || err.Error() != "invalid brief metadata" {
				t.Fatalf("%s: %v", name, err)
			}
			if got := readBrief(t, sub, "b.md"); got != before {
				t.Fatalf("%s: changed the brief: %q", name, got)
			}
		}
	})
}

// A brief over 1 MiB is refused untouched; one of exactly 1 MiB is still marked.
func TestMarkBriefSizeLimit(t *testing.T) {
	head := "---\nstatus: running\n---\n"
	for _, tc := range []struct {
		name string
		size int
		ok   bool
	}{{"1 MiB + 1 byte", maxBrief + 1, false}, {"1 MiB", maxBrief, true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir, sub := briefDir(t)
			before := head + strings.Repeat("x", tc.size-len(head))
			write(t, filepath.Join(sub, "b.md"), before)
			err := MarkBrief(dir, "b.md", "done")
			got := readBrief(t, sub, "b.md")
			if tc.ok {
				if err != nil || got != strings.Replace(before, "status: running", "status: done", 1) {
					t.Fatalf("a 1 MiB brief: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "over 1 MiB") {
				t.Fatalf("got %v", err)
			}
			if got != before {
				t.Fatal("a refused brief was changed")
			}
		})
	}
}

// CRLF front matter keeps its line ends: the rewrite stops before the \r.
func TestMarkBriefKeepsCRLF(t *testing.T) {
	dir, sub := briefDir(t)
	before := "---\r\nid: B-1\r\nstatus: running\r\nowner: dana\r\n---\r\nBody\r\nmore\r\n"
	write(t, filepath.Join(sub, "b.md"), before)
	if err := MarkBrief(dir, "b.md", "blocked"); err != nil {
		t.Fatal(err)
	}
	want := "---\r\nid: B-1\r\nstatus: blocked\r\nowner: dana\r\n---\r\nBody\r\nmore\r\n"
	if got := readBrief(t, sub, "b.md"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Row 9: the account that runs Grid is the OS user. Only the first branch can be
// reached on a host where user.Current works; the $USER and "owner" fallbacks
// need user.Current to fail, which a test cannot cause.
func TestMachineOwnerIsTheCurrentUser(t *testing.T) {
	u, err := user.Current()
	if err != nil || strings.TrimSpace(u.Username) == "" {
		t.Skipf("user.Current does not answer on this host: %v", err)
	}
	got := machineOwner()
	if got == "" || got != strings.TrimSpace(u.Username) {
		t.Fatalf("machineOwner = %q, want %q", got, strings.TrimSpace(u.Username))
	}
}
