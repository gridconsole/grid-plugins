package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// jaWaitingHarness is approveJobHarness with the task `running`, as the worker
// leaves it: plan.md and an open approve person Job J-2.
func jaWaitingHarness(t *testing.T) (*harness, string, feed.Store, string) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	text := "# Plan\n\nShip the parser.\n"
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "intent.md"), []byte("# Intent\n\nShip the parser.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	md := "---\nid: T-07\nulid: " + feedTaskID + "\ntitle: Example\nproject: example\nstate: running\ncreated: 2026-11-10T08:00:00Z\nupdated: 2026-11-10T08:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(path, "task.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, "J-2", nil)), &body); err != nil {
		t.Fatal(err)
	}
	body["kind"] = "approve"
	out, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", string(out)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	return h, path, store, text
}

// jaWaitingStep5Sentence is the sentence of step 5 that holds the waiting command.
func jaWaitingStep5Sentence(t *testing.T, text string) string {
	t.Helper()
	i := strings.Index(text, "`grid task status <task> waiting`")
	if i < 0 {
		t.Fatal("agents/orchestrator.md lacks `grid task status <task> waiting`")
	}
	start := strings.LastIndex(text[:i], ". ") + 2
	end := i + strings.Index(text[i:], ". ") + 1
	return text[start:end]
}

func jaWaitingLine(t *testing.T, text, prefix string) string {
	t.Helper()
	i := strings.Index(text, prefix)
	if i < 0 {
		t.Fatalf("agents/orchestrator.md lacks a line that starts %q", prefix)
	}
	line := text[i:]
	if j := strings.Index(line, "\n"); j >= 0 {
		line = line[:j]
	}
	return line
}

// Rows 1 and 2: the step 5 sentence names the waiting command with approve and
// Person, sits between the post sentence and the deliver Job sentence, and skips
// the move when the task is already waiting.
func TestJaWaitingStep5SentenceMovesTheTaskToWaiting(t *testing.T) {
	text := orchestratorText(t)
	s := jaWaitingStep5Sentence(t, text)
	for _, want := range []string{"approve", "Person", "`grid task status <task> waiting`"} {
		if !strings.Contains(s, want) {
			t.Errorf("the sentence lacks %q: %s", want, s)
		}
	}
	post := strings.Index(text, "Post each Job with `grid job post")
	cmd := strings.Index(text, "`grid task status <task> waiting`")
	last := strings.Index(text, "In your first Job, post one last Job")
	if post < 0 || last < 0 || !(post < cmd && cmd < last) {
		t.Errorf("order wrong: post %d, command %d, last Job %d", post, cmd, last)
	}
	if !(strings.Contains(s, "unless") || strings.Contains(s, "already")) || !strings.Contains(s, "`waiting`") {
		t.Errorf("the sentence does not name waiting as the state that skips the move: %s", s)
	}
}

// Row 3: the Deliver line moves the task to running before grid deliver.
func TestJaWaitingDeliverLineMovesTheTaskToRunning(t *testing.T) {
	line := jaWaitingLine(t, orchestratorText(t), "When your Job is the one titled \"Deliver the task\"")
	run := strings.Index(line, "`grid task status <task> running`")
	deliver := strings.Index(line, "`grid deliver --task <task>")
	if run < 0 || deliver < 0 || run > deliver {
		t.Fatalf("running at %d, deliver at %d: %s", run, deliver, line)
	}
}

// Row 4: the commands in the text, run as written, take a running task to
// waiting, let grid approve through, and back to running.
func TestJaWaitingTextCommandsDriveTheCLI(t *testing.T) {
	text := orchestratorText(t)
	re := regexp.MustCompile("`grid task status <task> ([a-z]+)`")
	cmds := map[string][]string{}
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		cmds[m[1]] = []string{"task", "status", "<task>", m[1]}
	}
	if len(cmds) == 0 {
		t.Fatal("no `grid task status <task> <state>` in the text")
	}
	h, path, store, plantext := jaWaitingHarness(t)
	status := func(state string) int {
		args, ok := cmds[state]
		if !ok {
			t.Fatalf("the text has no command for %s", state)
		}
		args = append([]string(nil), args...)
		for i, a := range args {
			if a == "<task>" {
				args[i] = path
			}
		}
		return h.run(path, args...)
	}
	approve := func() int { return h.run(path, "approve", path, "--plan-hash", plan.Hash(plantext)) }

	if code := approve(); code == 0 || !strings.Contains(h.errb.String(), "expected waiting") {
		t.Fatalf("approve before waiting: %d %s", code, h.errb.String())
	}
	if code := status("waiting"); code != 0 {
		t.Fatalf("waiting: %d %s", code, h.errb.String())
	}
	if code := approve(); code != 0 {
		t.Fatalf("approve after waiting: %d %s", code, h.errb.String())
	}
	if body := approvalResult(t, store); body == nil || body["plan_digest"] == nil {
		t.Fatalf("no plan_digest in %v", body)
	}
	if code := status("waiting"); code == 0 {
		t.Fatal("a second waiting command exited 0")
	}
	if code := status("running"); code != 0 {
		t.Fatalf("running: %d %s", code, h.errb.String())
	}
	md, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil || !strings.Contains(string(md), "state: running") {
		t.Fatalf("task.md: %v\n%s", err, md)
	}
}
