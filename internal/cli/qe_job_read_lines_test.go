package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// qeResultBody is a result that finished: it carries all ten keys of the result schema.
const qeResultBody = `{"outcome":"finished","changed":[],"evidence":[],"acceptance":[],"verified":null,"must_fix":[],"found":[],"proposed_jobs":[],"decisions_needed":[],"knowledge":[]}`

// qeFixture builds the two Jobs a re-plan reads: J-1 posted a result and then exited
// done, and J-2 was posted and cancelled before any render (so it has no brief.md).
func qeFixture(t *testing.T) (*harness, string) {
	t.Helper()
	h, path, _ := orchestratorHarness(t)
	raw, _ := json.Marshal(workerNextBody(t, "J-2"))
	if code := h.run(path, "job", "post", "--type", "job", "--body", string(raw)); code != 0 {
		t.Fatalf("post J-2: %d %s", code, h.errb)
	}
	if code := h.run(path, "job", "cancel", "J-2", "--reason", "dropped"); code != 0 {
		t.Fatalf("cancel J-2: %d %s", code, h.errb)
	}
	return h, path
}

// qeFinishJ1 takes J-1 through assign, ack, a result and a done exit, as the
// "result then done" case of TestJobTreeShowsAnEmptyFinish does.
func qeFinishJ1(t *testing.T, h *harness, path string) {
	t.Helper()
	store := qeStore(path)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("assign: %d %s", code, h.errb)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs["J-1"]
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", qeEpoch(j.Generation, j.Assign.Fence))
	t.Setenv("GRID_FEED_TASK", path)
	for _, args := range [][]string{
		{"job", "ack", "J-1", "--attempt", "1"},
		{"job", "post", "J-1", "--attempt", "1", "--type", "result", "--body", qeResultBody},
		{"job", "post", "J-1", "--attempt", "1", "--type", "worker-exit", "--body", `{"reason":"done"}`},
	} {
		if code := h.run(path, args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
	}
	t.Setenv("GRID_ROLE", "")
}

func qeStore(path string) feed.Store { return feed.Store{TaskDir: path, TaskID: feedTaskID} }

func qeEpoch(generation, fence any) string { return fmt.Sprintf("%v.%v", generation, fence) }

// qeSpans returns every backticked span of text that starts with "grid ".
func qeSpans(text string) []string {
	var out []string
	for _, m := range regexp.MustCompile("`(grid [^`]+)`").FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// qeSection is the text from the first line that starts with from, up to the
// next line that starts with to (or the end).
func qeSection(t *testing.T, text, from, to string) string {
	t.Helper()
	i := strings.Index(text, from)
	if i < 0 {
		t.Fatalf("text has no %q", from)
	}
	rest := text[i:]
	if j := strings.Index(rest[len(from):], to); j >= 0 && to != "" {
		rest = rest[:len(from)+j]
	}
	return rest
}

// qeCut keeps the spans of a section that read a Job's facts: a job tree or a feed read.
func qeCut(t *testing.T, section string) []string {
	t.Helper()
	var out []string
	for _, s := range qeSpans(section) {
		if strings.HasPrefix(s, "grid job tree ") || strings.HasPrefix(s, "grid feed read ") {
			out = append(out, s)
		}
	}
	return out
}

// qeRun fills <task> and <job> in one cut command and runs it.
func qeRun(t *testing.T, h *harness, path, job, cmd string) (int, string) {
	t.Helper()
	cmd = strings.NewReplacer("<task>", feedTaskID, "<job>", job).Replace(cmd)
	args := strings.Fields(strings.TrimPrefix(cmd, "grid "))
	code := h.run(path, args...)
	return code, h.out.String()
}

// The base line of plan-jobs' Re-plan step 1 read jobs/<id>/brief.md, which a Job
// cancelled before its render does not have, so show exited 1 (the text changed for
// that reason). Since round 15 F6 (C-r15-job-show-unsaved) show prints such a Job from
// its Feed record, with the state the Feed folds, and still writes no folder.
func TestQeShowLineExitsZeroOnCancelledJobFromItsFeedRecord(t *testing.T) {
	h, path := qeFixture(t)
	code := h.run(path, "job", "show", "J-2", "--task", feedTaskID)
	if code != 0 || !strings.Contains(h.out.String(), "state: Done (cancelled)\n") {
		t.Fatalf("job show on a cancelled unrendered Job: exit %d, want 0 with its Feed state; stdout %q stderr %q", code, h.out, h.errb)
	}
}

func TestQeReplanReadLines(t *testing.T) {
	skill, ok := gridplugins.SkillFor("plan-jobs")
	if !ok {
		t.Fatal("no plan-jobs skill")
	}
	agent, ok := gridplugins.AgentFor("orchestrator")
	if !ok {
		t.Fatal("no orchestrator agent")
	}
	const result = "grid feed read --task <task> --job <job> --type result"
	sections := map[string]string{
		"plan-jobs Re-plan step 1": qeSection(t, skill, "1. Read ", "\n2. "),
		"orchestrator deliver":     qeSection(t, agent, `When your Job is the one titled "Deliver the task"`, "\n\n"),
		"orchestrator re-plan":     qeSection(t, agent, `When your Job's title starts "Re-plan after"`, "\n\n"),
	}
	cuts := map[string][]string{}
	for name, sec := range sections {
		cuts[name] = qeCut(t, sec)
		has := false
		for _, c := range cuts[name] {
			has = has || c == result
		}
		if !has {
			t.Errorf("%s names no %q; cut commands: %q", name, result, cuts[name])
		}
	}
	if !strings.Contains(strings.Join(cuts["plan-jobs Re-plan step 1"], "\n"), "grid job tree --task <task>") {
		t.Errorf("plan-jobs Re-plan step 1 names no `grid job tree --task <task>`: %q", cuts["plan-jobs Re-plan step 1"])
	}
	if !strings.Contains(strings.Join(cuts["orchestrator re-plan"], "\n"), "grid job tree --task <task>") {
		t.Errorf("orchestrator re-plan names no `grid job tree --task <task>`: %q", cuts["orchestrator re-plan"])
	}
	if t.Failed() {
		return
	}

	h, path := qeFixture(t)
	qeFinishJ1(t, h, path)
	for name, cut := range cuts {
		for _, cmd := range cut {
			for _, job := range []string{"J-1", "J-2"} {
				code, out := qeRun(t, h, path, job, cmd)
				if code != 0 {
					t.Errorf("%s: %q for %s: exit %d: %s", name, cmd, job, code, h.errb)
					continue
				}
				if strings.HasPrefix(cmd, "grid job tree") && job == "J-1" && !strings.Contains(out, "\n  attempt 1") {
					t.Errorf("%s: tree has no `  attempt 1` line for J-1:\n%s", name, out)
				}
				if strings.HasPrefix(cmd, "grid feed read") && job == "J-1" && !strings.Contains(out, `"outcome":"finished"`) {
					t.Errorf("%s: feed read for J-1 lacks the finished outcome:\n%s", name, out)
				}
			}
		}
	}
}
