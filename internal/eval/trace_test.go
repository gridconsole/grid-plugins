package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "trace", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func traceCheck(t *testing.T, raw string, in traceInput) CheckResult {
	t.Helper()
	var task Task
	checks := loadChecks(t, raw)
	return evaluate(context.Background(), task, checks[0], t.TempDir(), nil, nil, in)
}

func loadChecks(t *testing.T, raw string) []Check {
	t.Helper()
	task, err := loadSuiteWith(t, raw)
	if err != nil {
		t.Fatal(err)
	}
	return task.Checks
}

// loadSuiteWith writes a one-task suite whose checks.json is raw and loads it.
func loadSuiteWith(t *testing.T, raw string) (Task, error) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"task.json": `{"schema_version":1,"id":"demo","title":"Demo","tier":"b","size":"tiny"}`,
		"prompt.md": "Do it.\n", "setup.sh": "true\n", "checks.json": raw,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := LoadSuite(filepath.Dir(dir))
	if err != nil {
		return Task{}, err
	}
	return tasks[0], nil
}

func want(t *testing.T, r CheckResult, status, reason string) {
	t.Helper()
	if r.Status != status || !strings.Contains(r.Reason, reason) || r.Scorer != "code" {
		t.Fatalf("want %s %q, got status=%s scorer=%s reason=%q", status, reason, r.Status, r.Scorer, r.Reason)
	}
}

func TestToolUsed(t *testing.T) {
	good, bad := traceInput{data: fixture(t, "good.jsonl")}, traceInput{data: fixture(t, "no-skill.jsonl")}
	skill := `[{"id":"c","type":"tool_used","tool":"Skill","input_contains":{"skill":"execute"}}]`
	absent := `[{"id":"c","type":"tool_used","tool":"Skill","input_contains":{"skill":"execute"},"absent":true}]`
	for _, row := range []struct {
		name, check string
		in          traceInput
		status      string
	}{
		{"present passes", skill, good, "passed"},
		{"missing fails", skill, bad, "failed"},
		{"absent with call fails", absent, good, "failed"},
		{"absent without call passes", absent, bad, "passed"},
		{"substring must match", `[{"id":"c","type":"tool_used","tool":"Skill","input_contains":{"skill":"views"}}]`, good, "failed"},
		{"field must exist", `[{"id":"c","type":"tool_used","tool":"Skill","input_contains":{"nope":"execute"}}]`, good, "failed"},
		{"name is exact", `[{"id":"c","type":"tool_used","tool":"Ski"}]`, good, "failed"},
		{"count_min 2 with one call fails", `[{"id":"c","type":"tool_used","tool":"Skill","count_min":2}]`, good, "failed"},
		{"count_min 2 with two calls passes", `[{"id":"c","type":"tool_used","tool":"Read","count_min":2}]`, good, "passed"},
		{"count_min 3 with two calls fails", `[{"id":"c","type":"tool_used","tool":"Read","count_min":3}]`, good, "failed"},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := traceCheck(t, row.check, row.in)
			if r.Status != row.status || r.Scorer != "code" {
				t.Fatalf("want %s, got %s (%s)", row.status, r.Status, r.Reason)
			}
		})
	}
}

func TestToolOrder(t *testing.T) {
	order := `[{"id":"c","type":"tool_order","before":{"tool":"Agent","input_contains":{"subagent_type":"reviewer"}},"after":{"tool":"Edit","input_contains":{"new_string":"status: waiting"}}}]`
	calls, err := DefaultTraceParser(fixture(t, "good.jsonl"))
	if err != nil || calls[3].Name != "Agent" || calls[5].Name != "Edit" || len(calls) != 6 {
		t.Fatalf("fixture shape: %v %+v", err, calls)
	}
	want(t, traceCheck(t, order, traceInput{data: fixture(t, "good.jsonl")}), "passed", "before at call 3, after at call 5")
	want(t, traceCheck(t, order, traceInput{data: fixture(t, "swapped.jsonl")}), "failed", "before at call 4, after at call 2")
	// Either call missing fails.
	want(t, traceCheck(t, order, traceInput{data: fixture(t, "no-skill.jsonl")}), "failed", "no call matches before")
	noEdit := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"grid-reviewer"}}]}}` + "\n")
	want(t, traceCheck(t, order, traceInput{data: noEdit}), "failed", "no call matches after")
}

func TestNoTraceIsAnError(t *testing.T) {
	plain := traceInput{data: fixture(t, "plain.txt")}
	for _, raw := range []string{
		`[{"id":"c","type":"tool_used","tool":"Skill"}]`,
		`[{"id":"c","type":"tool_used","tool":"Skill","absent":true}]`,
		`[{"id":"c","type":"tool_order","before":{"tool":"A"},"after":{"tool":"B"}}]`,
	} {
		want(t, traceCheck(t, raw, plain), "error", "no trace")
		want(t, traceCheck(t, raw, traceInput{}), "error", "no trace")
	}
	// A run that made no tool call is a real trace: an absent check passes.
	empty := traceInput{data: []byte(`{"type":"system","subtype":"init"}` + "\n")}
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Skill","absent":true}]`, empty), "passed", "called 0 times")
}

func TestTruncatedTraceIsAnError(t *testing.T) {
	cut := traceInput{data: fixture(t, "good.jsonl"), truncated: true}
	for _, raw := range []string{
		`[{"id":"c","type":"tool_used","tool":"Skill"}]`,
		`[{"id":"c","type":"tool_used","tool":"Skill","absent":true}]`,
		`[{"id":"c","type":"tool_order","before":{"tool":"Agent"},"after":{"tool":"Edit"}}]`,
	} {
		want(t, traceCheck(t, raw, cut), "error", "trace truncated")
	}
}

func TestExistingChecksIgnoreTrace(t *testing.T) {
	r := traceCheck(t, `[{"id":"c","type":"command_passes","command":["true"]}]`, traceInput{truncated: true})
	want(t, r, "passed", "")
}

func TestTraceCheckLoad(t *testing.T) {
	for name, raw := range map[string]string{
		"missing tool":        `[{"id":"needs-tool","type":"tool_used"}]`,
		"non-string value":    `[{"id":"num","type":"tool_used","tool":"Skill","input_contains":{"n":3}}]`,
		"missing after":       `[{"id":"no-after","type":"tool_order","before":{"tool":"A"}}]`,
		"missing before":      `[{"id":"no-before","type":"tool_order","after":{"tool":"A"}}]`,
		"matcher no tool":     `[{"id":"m","type":"tool_order","before":{"tool":"A"},"after":{}}]`,
		"negative count":      `[{"id":"neg","type":"tool_used","tool":"A","count_min":-1}]`,
		"absent with count":   `[{"id":"both","type":"tool_used","tool":"A","absent":true,"count_min":2}]`,
		"mixed fields":        `[{"id":"mix","type":"tool_used","tool":"A","path":"x"}]`,
		"unknown field":       `[{"id":"unk","type":"tool_used","tool":"A","tools":"B"}]`,
		"order with tool":     `[{"id":"ot","type":"tool_order","tool":"A","before":{"tool":"A"},"after":{"tool":"B"}}]`,
		"empty substring":     `[{"id":"empty","type":"tool_used","tool":"A","input_contains":{"f":""}}]`,
		"unmatched with type": `[{"id":"typo","type":"tool_usd","tool":"A"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadSuiteWith(t, raw)
			if err == nil {
				t.Fatal("load succeeded")
			}
			var id string
			for _, c := range []string{"needs-tool", "num", "no-after", "no-before", "m", "neg", "both", "mix", "unk", "ot", "empty", "typo"} {
				if strings.Contains(raw, `"`+c+`"`) {
					id = c
				}
			}
			// Unknown JSON fields fail in the strict decoder before checks have ids.
			if name != "unknown field" && !strings.Contains(err.Error(), "check "+id) {
				t.Fatalf("error does not name check %s: %v", id, err)
			}
		})
	}
	if _, err := loadSuiteWith(t, `[{"id":"ok","type":"tool_used","tool":"Skill","input_contains":{"skill":"x"},"count_min":2},{"id":"ord","type":"tool_order","before":{"tool":"A"},"after":{"tool":"B","input_contains":{"f":"g"}}}]`); err != nil {
		t.Fatal(err)
	}
}

func traceTask(checks ...Check) Task {
	return Task{SchemaVersion: 1, ID: "trace-demo", Title: "Trace", Tier: "b", Size: "tiny", Prompt: "go", Setup: "true", Checks: checks}
}

const toolLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"grid:execute"}}]}}`

// A real stream-json run is far larger than the stored stdout cap.
func TestTraceComesFromTheFullStdout(t *testing.T) {
	agent := []string{"sh", "-c", `yes '{"type":"system","note":"filler"}' | head -c 204800; printf '\n%s\n' "$1"`, "agent", toolLine}
	checks := []Check{
		{ID: "skill", Type: "tool_used", Tool: "Skill", InputContains: map[string]any{"skill": "execute"}},
		{ID: "no-trace-file", Type: "command_passes", Command: []string{"sh", "-c", "test ! -e trace.jsonl && test ! -e ../trace.jsonl.x"}},
	}
	runner := testRunner(t)
	runner.Agent = agent
	r := runTask(t, runner, traceTask(checks...))
	if r.Checks[0].Status != "passed" || r.Checks[1].Status != "passed" || !r.AgentResult.Truncated || len(r.AgentResult.Stdout) != outputLimit {
		t.Fatalf("full stdout not used: %+v", r)
	}
	// The stored stdout alone lacks the call, so the trace did not come from it.
	if strings.Contains(r.AgentResult.Stdout, "tool_use") {
		t.Fatal("the stored stdout holds the call; the cap row proves nothing")
	}

	runner = testRunner(t)
	runner.Agent, runner.TraceLimit = agent, 1024
	r = runTask(t, runner, traceTask(checks[0]))
	if r.Checks[0].Status != "error" || r.Checks[0].Reason != "trace truncated" || r.Status != "error" {
		t.Fatalf("limit not applied: %+v", r.Checks[0])
	}
}

func TestTraceStaysOutOfTheWorkspace(t *testing.T) {
	runner := testRunner(t)
	runner.Agent = []string{"sh", "-c", `printf '%s\n' "$1"; ls -A > seen.txt`, "agent", toolLine}
	check := Check{ID: "skill", Type: "tool_used", Tool: "Skill"}
	listing := Check{ID: "listing", Type: "command_passes", Command: []string{"sh", "-c", "! grep -q trace seen.txt"}}
	r := runTask(t, runner, traceTask(check, listing))
	if r.Checks[0].Status != "passed" || r.Checks[1].Status != "passed" {
		t.Fatalf("trace visible in the workspace or not read: %+v", r.Checks)
	}
}

func TestCustomTraceParser(t *testing.T) {
	parser := func(data []byte) ([]ToolCall, error) {
		calls := []ToolCall{}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			name, _, ok := strings.Cut(line, "\t")
			if !ok {
				return nil, ErrNoTrace
			}
			calls = append(calls, ToolCall{Index: len(calls), Name: name})
		}
		return calls, nil
	}
	runner := testRunner(t)
	runner.Agent = []string{"sh", "-c", `printf 'Read\t{}\nSkill\t{}\n'`}
	runner.TraceParser = parser
	r := runTask(t, runner, traceTask(Check{ID: "skill", Type: "tool_used", Tool: "Skill"}))
	if r.Checks[0].Status != "passed" {
		t.Fatalf("custom parser not used: %+v", r.Checks[0])
	}
	runner.TraceParser = nil
	r = runTask(t, runner, traceTask(Check{ID: "skill", Type: "tool_used", Tool: "Skill"}))
	if r.Checks[0].Status != "error" || r.Checks[0].Reason != "no trace" {
		t.Fatalf("default parser should not read the tab format: %+v", r.Checks[0])
	}
}

func TestFakeAgentHasNoTrace(t *testing.T) {
	task := traceTask(Check{ID: "skill", Type: "tool_used", Tool: "Skill"})
	task.ID = "clean-csv"
	task.Setup = "mkdir inputs; printf 'a\\nb\\nb\\n' > inputs/items.csv"
	r := runTask(t, testRunner(t), task)
	if r.Checks[0].Status != "error" || r.Checks[0].Reason != "no trace" {
		t.Fatalf("fake agent run: %+v", r.Checks[0])
	}
}
