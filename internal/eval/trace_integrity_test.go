package eval

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The agent's HOME is <attempt>/home, so ../trace.jsonl is within its reach.
const forgedLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Forged","input":{}}]}}`

func TestAgentCannotForgeTheTrace(t *testing.T) {
	runner := testRunner(t)
	runner.Agent = []string{"sh", "-c", `printf '%s\n' "$1" > "$HOME/../trace.jsonl"`, "agent", forgedLine}
	r := runTask(t, runner, traceTask(Check{ID: "forged", Type: "tool_used", Tool: "Forged"}))
	if r.Checks[0].Status != "error" || r.Checks[0].Reason != "no trace" {
		t.Fatalf("forged file was read: %+v", r.Checks[0])
	}
}

// The runner writes no trace file, so the agent's bounded wait (up to 2s) for a copy of
// its stdout never ends early; it only gives a reintroduced trace.jsonl time to appear
// before the overwrite. The 5s task timeout leaves the full wait a 3s margin.
func TestOverwrittenTraceFileIsNotRead(t *testing.T) {
	runner := testRunner(t)
	runner.Timeout = 5 * time.Second
	runner.Agent = []string{"sh", "-c", `printf '%s\n' "$1"; n=0; until grep -q Skill "$HOME/../trace.jsonl" 2>/dev/null || [ $n -ge 40 ]; do n=$((n+1)); sleep 0.05; done; printf '%s\n' "$2" > "$HOME/../trace.jsonl"`, "agent", toolLine, forgedLine}
	r := runTask(t, runner, traceTask(
		Check{ID: "absent", Type: "tool_used", Tool: "Forged", Absent: true},
		Check{ID: "real", Type: "tool_used", Tool: "Skill"},
	))
	if r.Checks[0].Status != "passed" || r.Checks[1].Status != "passed" {
		t.Fatalf("overwritten file was read: %+v", r.Checks)
	}
}

func TestTopLevelFiltersSubagentCalls(t *testing.T) {
	in := traceInput{data: fixture(t, "nested.jsonl")}
	// Read: one call at top level (index 1 of 2 top-level calls), one in the subagent.
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Read","count_min":2}]`, in), "passed", "called 2 times")
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Read","count_min":2,"top_level":true}]`, in), "failed", "called 1 times, need 2")
	// Skill is called only inside the subagent.
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Skill"}]`, in), "passed", "called 1 times")
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Skill","top_level":true}]`, in), "failed", "called 0 times")
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Skill","absent":true,"top_level":true}]`, in), "passed", "called 0 times")
	// tool_order: the before call exists only inside the subagent.
	order := `[{"id":"c","type":"tool_order","before":{"tool":"Skill"},"after":{"tool":"Read","input_contains":{"file_path":"y"}}%s}]`
	want(t, traceCheck(t, fmt.Sprintf(order, ""), in), "passed", "before at call 2, after at call 3")
	want(t, traceCheck(t, fmt.Sprintf(order, `,"top_level":true`), in), "failed", "no call matches before")
	// Indexes are recounted over top-level calls: Agent 0, Read 1.
	want(t, traceCheck(t, `[{"id":"c","type":"tool_order","before":{"tool":"Agent"},"after":{"tool":"Read"},"top_level":true}]`, in), "passed", "before at call 0, after at call 1")
}

func TestTopLevelLoad(t *testing.T) {
	for _, raw := range []string{
		`[{"id":"nb","type":"tool_used","tool":"A","top_level":"yes"}]`,
		`[{"id":"nb","type":"command_passes","command":["true"],"top_level":true}]`,
	} {
		if _, err := loadSuiteWith(t, raw); err == nil {
			t.Fatalf("load succeeded: %s", raw)
		}
	}
	if _, err := loadSuiteWith(t, `[{"id":"ok","type":"tool_used","tool":"A","top_level":true},{"id":"ord","type":"tool_order","top_level":false,"before":{"tool":"A"},"after":{"tool":"B"}}]`); err != nil {
		t.Fatal(err)
	}
}

func TestParserKeepsParentID(t *testing.T) {
	calls, err := parseStreamJSON(fixture(t, "nested.jsonl"))
	if err != nil || len(calls) != 4 || calls[0].Parent != "" || calls[1].Parent != "toolu_a" || calls[3].Parent != "" {
		t.Fatalf("calls: %+v, %v", calls, err)
	}
}

func TestTopLevelErrorsWhenTraceHasNoParentKey(t *testing.T) {
	in := traceInput{data: []byte(toolLine + "\n" + toolLine + "\n")}
	r := traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Skill","absent":true,"top_level":true}]`, in)
	if r.Status != "error" || !strings.Contains(r.Reason, "parent_tool_use_id") {
		t.Fatalf("top_level passed without the key: %+v", r)
	}
	// Without top_level the same trace is still read.
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Skill","count_min":2}]`, in), "passed", "called 2 times")
}

func TestTopLevelStillPassesWithNullParentKey(t *testing.T) {
	line := `{"type":"assistant","parent_tool_use_id":null,"message":{"content":[{"type":"tool_use","name":"Skill","input":{}}]}}`
	in := traceInput{data: []byte(line + "\n")}
	want(t, traceCheck(t, `[{"id":"c","type":"tool_used","tool":"Skill","top_level":true}]`, in), "passed", "called 1 times")
}
