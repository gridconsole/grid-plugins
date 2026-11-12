package cli

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
)

// qdPostSpan is a backticked `grid job post ...` that carries a Job body.
var qdPostSpan = regexp.MustCompile("`(grid job post [^`]*--body <json>[^`]*)`")

type qdLine struct {
	file string // "plan-jobs" or "orchestrator"
	n    int    // 1-based line in that file
	cmd  string // the backticked command, placeholders unfilled
}

// qdPostLines cuts every backticked `grid job post ... --body <json>` out of the plan-jobs skill and the orchestrator agent.
func qdPostLines(t *testing.T) []qdLine {
	t.Helper()
	skill, ok := gridplugins.SkillFor("plan-jobs")
	if !ok {
		t.Fatal("no skills/plan-jobs/SKILL.md")
	}
	agent, ok := gridplugins.AgentFor("orchestrator")
	if !ok {
		t.Fatal("no agents/orchestrator.md")
	}
	var out []qdLine
	for _, src := range []struct{ name, text string }{{"plan-jobs", skill}, {"orchestrator", agent}} {
		for i, line := range strings.Split(src.text, "\n") {
			for _, m := range qdPostSpan.FindAllStringSubmatch(line, -1) {
				out = append(out, qdLine{src.name, i + 1, m[1]})
			}
		}
	}
	return out
}

// TestQdJobPostLinesPostJobs runs each Job-post line of the text as written: it must
// exit 0 and `job tree` must then list the posted Job.
func TestQdJobPostLinesPostJobs(t *testing.T) {
	lines := qdPostLines(t)
	if len(lines) != 3 {
		t.Fatalf("want 3 lines that post a Job body (plan-jobs Re-plan step 3, plan-jobs Run, orchestrator step 5), got %d: %v", len(lines), lines)
	}
	for _, l := range lines {
		l := l
		t.Run(l.file+":"+l.cmd, func(t *testing.T) {
			h, path, _ := orchestratorHarness(t)
			raw, err := json.Marshal(workerNextBody(t, "J-2"))
			if err != nil {
				t.Fatal(err)
			}
			cmd := strings.NewReplacer("<task>", feedTaskID).Replace(l.cmd)
			args := strings.Fields(cmd)[1:] // drop "grid"
			for i, a := range args {
				if a == "<json>" {
					args[i] = string(raw)
				}
			}
			if code := h.run(path, args...); code != 0 {
				t.Fatalf("%s line %d: %q exits %d, stderr: %s", l.file, l.n, l.cmd, code, strings.TrimSpace(h.errb.String()))
			}
			if code := h.run(path, "job", "tree", "--task", feedTaskID); code != 0 {
				t.Fatalf("job tree: %d %s", code, h.errb)
			}
			found := false
			for _, row := range strings.Split(h.out.String(), "\n") {
				if strings.HasPrefix(row, "J-2") {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s line %d: job tree does not list J-2 after %q:\n%s", l.file, l.n, l.cmd, h.out)
			}
		})
	}
}

// TestQdReplanKeepPostsNoEdit holds the Re-plan keep step to what the CLI can do:
// no verb edits a posted Job, so the waiter is cancelled and posted again.
func TestQdReplanKeepPostsNoEdit(t *testing.T) {
	skill, ok := gridplugins.SkillFor("plan-jobs")
	if !ok {
		t.Fatal("no skills/plan-jobs/SKILL.md")
	}
	i := strings.Index(skill, "## Re-plan Job")
	if i < 0 {
		t.Fatal("plan-jobs has no Re-plan Job section")
	}
	var step3 string
	for _, l := range strings.Split(skill[i:], "\n") {
		if strings.HasPrefix(l, "3. ") {
			step3 = l
			break
		}
	}
	if step3 == "" {
		t.Fatal("Re-plan Job has no step 3")
	}
	if strings.Contains(step3, "point the waiter") {
		t.Errorf("Re-plan step 3 still asks to point the waiter's after: %s", step3)
	}
	if !strings.Contains(step3, "a posted Job is never edited") {
		t.Errorf("Re-plan step 3 lacks %q: %s", "a posted Job is never edited", step3)
	}
	if !strings.Contains(orchestratorText(t), "that does not wait on the dead Job") {
		t.Error("orchestrator.md lost \"that does not wait on the dead Job\"")
	}
}
