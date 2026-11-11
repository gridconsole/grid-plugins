package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

// kuJaAskArgs is the question an orchestrator Job agent asks.
var kuJaAskArgs = []string{"question", "ask", "Which export format?", "--who", "dana", "--option", "csv", "--option", "json",
	"--recommended", "csv", "--default", "csv", "--applies", "2099-01-01T00:00:00Z", "--json"}

// kuJaAsk runs the ask from the task folder as the agent for J-1 given taskEpoch,
// and returns its exit code and, when it printed one, the question record.
func kuJaAsk(t *testing.T, a *agentTask, taskEpoch string) (question.Record, int) {
	t.Helper()
	a.agentEnv(t, taskEpoch)
	code := a.h.run(a.path, kuJaAskArgs...)
	var r question.Record
	if code == 0 {
		if err := json.Unmarshal(a.h.out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
	}
	return r, code
}

// kuJaRecords reads every record of one type from the task's Feed.
func kuJaRecords(t *testing.T, a *agentTask, typ string) []feed.Entry {
	t.Helper()
	r, err := a.store.Read(feed.Filter{Type: typ})
	if err != nil {
		t.Fatal(err)
	}
	return r.Records
}

// Row 1: an agent under a Job lease that holds the task epoch asks as the task
// holder, so the question post and its person Job are posted as a task-lease ask's are.
func TestKuJaOrchestratorAskPostsPersonJob(t *testing.T) {
	a := newAgentTask(t)
	q, code := kuJaAsk(t, a, a.epoch())
	if code != 0 {
		t.Fatalf("ask by the Job agent with GRID_TASK_EPOCH: exit %d %s; jobs on the Feed: %d", code, a.h.errb, len(kuJaRecords(t, a, "job")))
	}
	if q.ID != "Q-1" || q.Job != "J-2" {
		t.Fatalf("ask: want Q-1 with job J-2, got %+v", q)
	}
	rec, ok := recordOf(t, a.store, "job", "J-2")
	if !ok || rec.Body["role"] != "Person" || rec.Body["kind"] != "ask" {
		t.Fatalf("person Job record: %v %+v", ok, rec)
	}
	posts := kuJaRecords(t, a, "question")
	if len(posts) != 1 || posts[0].Body["job"] != "J-2" {
		t.Fatalf("want one question post naming J-2: %+v", posts)
	}
	file, err := os.ReadFile(filepath.Join(a.path, "questions", "Q-1.md"))
	if err != nil || !strings.Contains(string(file), `"job": "J-2"`) {
		t.Fatalf("questions/Q-1.md: %v %s", err, file)
	}
}

// Row 2: after the ask, a Job the same agent posts with J-2 in its after list
// waits for the answer, and a Job without it is ready.
func TestKuJaLaterJobWaitsOnTheAnswer(t *testing.T) {
	a := newAgentTask(t)
	if q, code := kuJaAsk(t, a, a.epoch()); code != 0 || q.Job != "J-2" {
		t.Errorf("ask by the Job agent: exit %d job %q %s", code, q.Job, a.h.errb)
	}
	for _, j := range []struct {
		id    string
		after []any
	}{{"J-3", []any{"J-2"}}, {"J-4", nil}} {
		if code := a.h.run(a.path, "job", "post", "--type", "job", "--job", j.id, "--body", personBody(t, j.id, j.after)); code != 0 {
			t.Fatalf("post %s by the Job agent: exit %d %s", j.id, code, a.h.errb)
		}
	}
	ready := readyNow(t, a.store)
	if !slices.Contains(ready, "J-4") || slices.Contains(ready, "J-3") {
		t.Fatalf("while Q-1 is open J-4 must be ready and J-3 must wait: %v", ready)
	}
}

// Row 3: a stale task epoch is refused by the task write's own stale-lease
// check, and nothing is left on the Feed.
func TestKuJaStaleTaskEpochPostsNothing(t *testing.T) {
	a := newAgentTask(t)
	old := a.epoch()
	a.retake(t)
	if _, code := kuJaAsk(t, a, old); code == 0 || !strings.Contains(a.h.errb.String(), feed.ErrStaleLease.Error()) {
		t.Fatalf("ask with the stale task epoch %s: want the stale-lease refusal, got exit %d %s", old, code, a.h.errb)
	}
	if posts := kuJaRecords(t, a, "question"); len(posts) != 0 {
		t.Fatalf("want no question post: %+v", posts)
	}
	if jobs := kuJaRecords(t, a, "job"); len(jobs) != 1 {
		t.Fatalf("want only J-1, got %d job records", len(jobs))
	}
}

// Row 4: withdraw keeps its writer: GRID_TASK_EPOCH changes nothing about it.
func TestKuJaWithdrawKeepsItsWriter(t *testing.T) {
	a := newAgentTask(t)
	asPerson(t, "owner")
	t.Setenv("GRID_LEASE", "")
	t.Setenv("GRID_TASK_EPOCH", "")
	if code := a.h.run(a.path, kuJaAskArgs...); code != 0 {
		t.Fatalf("ask by the owner: exit %d %s", code, a.h.errb)
	}
	withdraw := func(taskEpoch string) (int, string) {
		t.Setenv("GRID_MEMBER", "worker")
		t.Setenv("GRID_ROLE", "worker")
		a.agentEnv(t, taskEpoch)
		code := a.h.run(a.path, "question", "withdraw", "Q-1", "--reason", "x")
		return code, a.h.errb.String()
	}
	plainCode, plainErr := withdraw("")
	taskCode, taskErr := withdraw(a.epoch())
	if plainCode != taskCode || plainErr != taskErr {
		t.Fatalf("withdraw with GRID_TASK_EPOCH: exit %d %q; without: exit %d %q", taskCode, taskErr, plainCode, plainErr)
	}
	t.Logf("withdraw with and without GRID_TASK_EPOCH: exit %d %q", taskCode, taskErr)
}
