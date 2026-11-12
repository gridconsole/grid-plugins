package cli

import (
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

const qiDecision = "Which export format? Options: csv, json. Recommendation: csv."

// qiSentence is the sentence of the questions skill that gives a worker without
// GRID_TASK_EPOCH its fallback, cut at the first ". " outside backticks.
func qiSentence(t *testing.T) string {
	t.Helper()
	text, ok := gridplugins.SkillFor("questions")
	if !ok {
		t.Fatal("no questions skill")
	}
	i := strings.Index(text, "An agent without GRID_TASK_EPOCH")
	if i < 0 {
		t.Fatal("the questions skill has no sentence for an agent without GRID_TASK_EPOCH")
	}
	rest := text[i:]
	ticks := false
	for j := 0; j < len(rest); j++ {
		switch {
		case rest[j] == '`':
			ticks = !ticks
		case !ticks && rest[j] == '.' && j+1 < len(rest) && (rest[j+1] == ' ' || rest[j+1] == '\n'):
			return rest[:j+1]
		}
	}
	t.Fatal("the sentence has no end")
	return ""
}

// qiFallbackCommand is the last backticked `grid job ...` command in the sentence.
func qiFallbackCommand(t *testing.T) string {
	t.Helper()
	parts := strings.Split(qiSentence(t), "`")
	cmd := ""
	for k := 1; k < len(parts); k += 2 {
		if strings.HasPrefix(parts[k], "grid job ") {
			cmd = parts[k]
		}
	}
	if cmd == "" {
		t.Fatal("the sentence names no grid job command")
	}
	return cmd
}

// qiWorkerEnv is the env of TestKb3QAskByWorkerIsRefusedNoJob.
func qiWorkerEnv(t *testing.T) (*harness, string, feed.Store) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", "")
	return h, path, store
}

// The fallback the skill used to give is refused by the Feed: a worker's role
// is not one a Job record may carry, so no person Job is posted.
func TestQiOldFallbackRefused(t *testing.T) {
	h, path, store := qiWorkerEnv(t)
	code := h.run(path, "job", "post", "--task", path, "--person", "dana", "--body", personBody(t, "J-2", nil))
	t.Logf("exit %d, stderr: %s", code, h.errb)
	if code == 0 {
		t.Fatalf("a worker posted a person Job: %s", h.out)
	}
	jobs, _ := store.Read(feed.Filter{Type: "job"})
	if len(jobs.Records) != 1 {
		t.Fatalf("want only the harness Job, got %d", len(jobs.Records))
	}
}

// The same post with the worker's own Job epoch, as a running worker has it,
// is refused too and posts no Job.
func TestQiOldFallbackRefusedWithJobEpoch(t *testing.T) {
	h, path, store, _, _ := assignedExitHarness(t, true)
	t.Setenv("GRID_LEASE", "J-1")
	code := h.run(path, "job", "post", "--task", path, "--person", "dana", "--body", personBody(t, "J-2", nil))
	t.Logf("exit %d, stderr: %s", code, h.errb)
	if code == 0 {
		t.Fatalf("a worker posted a person Job: %s", h.out)
	}
	jobs, _ := store.Read(feed.Filter{Type: "job"})
	if len(jobs.Records) != 1 {
		t.Fatalf("want only the harness Job, got %d", len(jobs.Records))
	}
}

// The command the skill names as the worker's fallback runs as the worker of an
// acknowledged Job and leaves the decision in the Job's result.
func TestQiWorkerFallbackRuns(t *testing.T) {
	cmd := qiFallbackCommand(t)
	h, path, store, _, _ := assignedExitHarness(t, true)
	body := `{"outcome":"finished","changed":[],"evidence":[],"acceptance":[],"verified":null,"must_fix":[],"found":[],"proposed_jobs":[],"decisions_needed":["` + qiDecision + `"],"knowledge":[]}`
	fields := strings.Fields(cmd)
	if len(fields) < 3 || fields[0] != "grid" {
		t.Fatalf("not a grid command: %q", cmd)
	}
	args := []string{}
	for _, f := range fields[1:] {
		switch f {
		case "<task>":
			f = path
		case "<job>":
			f = "J-1"
		case "<json>":
			f = body
		}
		args = append(args, f)
	}
	t.Setenv("GRID_ATTEMPT", "1")
	if code := h.run(path, args...); code != 0 {
		t.Fatalf("%q: exit %d: %s", cmd, code, h.errb)
	}
	h.out.Reset()
	if code := h.run(path, "feed", "read", "--task", path, "--job", "J-1", "--type", "result", "--json"); code != 0 {
		t.Fatalf("feed read: exit %d: %s", code, h.errb)
	}
	if got := h.out.String(); !strings.Contains(got, qiDecision) {
		t.Fatalf("the result lacks the decision: %s", got)
	}
	jobs, _ := store.Read(feed.Filter{Type: "job"})
	if len(jobs.Records) != 1 {
		t.Fatalf("want only the harness Job, got %d", len(jobs.Records))
	}
}
