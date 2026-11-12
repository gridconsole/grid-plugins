package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// tfFoundBuilder is the builder of J-2: the kb rig with J-2 posted, rendered (job show reads the
// saved brief) and assigned, and the agent's environment holding the J-2 lease and its tenure
// started by ack. It returns the rig and the attempt.
func tfFoundBuilder(t *testing.T) (*agentTask, int) {
	t.Helper()
	a := newAgentTask(t)
	// The basic Job record points at plan.md and a repo this rig lacks, so render would refuse it;
	// point its intent at an intent.md the rig writes and read nothing.
	a.h.write(filepath.Join(a.path, "intent.md"), "# Intent\n\nUnderstand the request.\n")
	body := workerNextBody(t, "J-2")
	body["intent"] = map[string]any{"owner": "intent.md#L1", "item": "intent.md#L1", "goal": "intent.md#L1"}
	body["read"] = []string{}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if code := a.h.run(a.path, "job", "post", "--type", "job", "--body", string(raw)); code != 0 {
		t.Fatalf("post J-2: exit %d %s", code, a.h.errb)
	}
	a.h.out.Reset()
	a.h.errb.Reset()
	if code := a.h.run(a.path, "job", "render", a.path, "J-2", "--json"); code != 0 {
		t.Fatalf("render J-2: exit %d %s %s", code, a.h.out, a.h.errb)
	}
	a.h.out.Reset()
	a.h.errb.Reset()
	if code := a.h.run(a.path, "job", "next", "--json"); code != 0 {
		t.Fatalf("job next: exit %d %s %s", code, a.h.out, a.h.errb)
	}
	state, err := a.store.State()
	if err != nil || state.Jobs["J-2"] == nil || state.Jobs["J-2"].Assign == nil {
		t.Fatalf("J-2 should be assigned: %+v %v", state.Jobs["J-2"], err)
	}
	t.Setenv("GRID_LEASE", "J-2")
	t.Setenv("GRID_LEASE_EPOCH", "0."+strconv.FormatInt(state.Jobs["J-2"].Assign.Fence, 10))
	t.Setenv("GRID_TASK_EPOCH", a.epoch())
	attempt := state.Jobs["J-2"].Attempt
	if code := a.h.run(a.path, "job", "ack", "J-2", "--attempt", strconv.Itoa(attempt), "--json"); code != 0 {
		t.Fatalf("ack J-2: exit %d %s", code, a.h.errb)
	}
	return a, attempt
}

// tfFoundShow runs grid job show <task> J-2 and returns its output.
func tfFoundShow(t *testing.T, a *agentTask, args ...string) string {
	t.Helper()
	a.h.out.Reset()
	a.h.errb.Reset()
	if code := a.h.run(a.path, append([]string{"job", "show", a.path, "J-2"}, args...)...); code != 0 {
		t.Fatalf("job show J-2 %v: exit %d %s %s", args, code, a.h.out, a.h.errb)
	}
	return a.h.out.String()
}

// tfFoundRecord is the record field of grid job show <task> J-2 --json.
func tfFoundRecord(t *testing.T, a *agentTask) string {
	t.Helper()
	var shown map[string]any
	if err := json.Unmarshal([]byte(tfFoundShow(t, a, "--json")), &shown); err != nil {
		t.Fatalf("job show --json: %v %s", err, a.h.out)
	}
	record, _ := shown["record"].(string)
	return record
}

// tfFoundSkillBody is the found body run-job SKILL.md gives on the line that names --type found:
// the text from its first { to its last }.
func tfFoundSkillBody(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../skills/run-job/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, "--type found") {
			continue
		}
		from, to := strings.Index(line, "{"), strings.LastIndex(line, "}")
		if from < 0 || to < from {
			t.Fatalf("the line that names --type found gives no body: %s", line)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(line[from:to+1]), &body); err != nil {
			t.Fatalf("the found body on that line is not JSON: %v: %s", err, line[from:to+1])
		}
		return body
	}
	t.Fatal("SKILL.md has no line that names --type found")
	return nil
}

// Row 1: job show --json prints the id of the Job's job record as record; the text output does not.
func TestTfFoundShowRecord(t *testing.T) {
	a, _ := tfFoundBuilder(t)
	want := kbRoleJobRecord(t, a, "J-2")
	if got := tfFoundRecord(t, a); got != want {
		t.Fatalf("job show J-2 --json: record %q, want the job record %q: %s", got, want, a.h.out)
	}
	if text := tfFoundShow(t, a); strings.Contains(text, want) || !strings.HasSuffix(text, "state: Started\n") {
		t.Fatalf("the text output should not name the record id and should end with the state line: %s", text)
	}
}

// Row 2: the builder holding J-2's lease posts a found with the run-job body, re being the record
// field of job show --json.
func TestTfFoundBuilderPosts(t *testing.T) {
	a, attempt := tfFoundBuilder(t)
	record := tfFoundRecord(t, a)
	body := tfFoundSkillBody(t)
	body["job"], body["re"], body["at"], body["text"] = "J-2", record, "main.go:10", "The handler reads the token from the query string."
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	a.h.errb.Reset()
	if code := a.h.run(a.path, "job", "post", "--task", a.path, "--job", "J-2", "--attempt", strconv.Itoa(attempt), "--type", "found", "--body", string(raw)); code != 0 {
		t.Fatalf("found post under the J-2 lease: exit %d %s (body %s)", code, a.h.errb, raw)
	}
	found := kbRoleFound(t, a)
	if len(found) != 1 {
		t.Fatalf("found records: %+v", found)
	}
	if re, _ := found[0].Body["re"].(string); re != record || re != kbRoleJobRecord(t, a, "J-2") {
		t.Fatalf("found re %q, want the job record %q", re, record)
	}
	if kind, _ := found[0].Body["kind"].(string); kind != "security" {
		t.Fatalf("found kind %q, want security", kind)
	}
	state, err := a.store.State()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Jobs["J-2"].NeedsLead {
		t.Fatalf("J-2 should need the lead: %+v", state.Jobs["J-2"])
	}
}

// Row 3: the line of run-job SKILL.md that names --type found gives the body: the six keys the
// schema requires, re naming grid job show --json.
func TestTfFoundSkillBody(t *testing.T) {
	body := tfFoundSkillBody(t)
	raw, err := os.ReadFile("../../contract/schemas/found.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		AllOf []struct {
			Properties struct {
				Body struct {
					OneOf []struct {
						Required []string `json:"required"`
					} `json:"oneOf"`
				} `json:"body"`
			} `json:"properties"`
		} `json:"allOf"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	var required []string
	for _, part := range schema.AllOf {
		for _, one := range part.Properties.Body.OneOf {
			if slices.Contains(one.Required, "re") {
				required = one.Required
			}
		}
	}
	if len(required) != 6 {
		t.Fatalf("found.json requires %v, want six keys", required)
	}
	for _, key := range required {
		if _, ok := body[key]; !ok {
			t.Errorf("the SKILL.md body lacks %q: %v", key, body)
		}
	}
	if len(body) != len(required) {
		t.Errorf("the SKILL.md body has keys %v, the schema requires %v", body, required)
	}
	if re, _ := body["re"].(string); !strings.Contains(re, "grid job show --json") {
		t.Errorf("re %q should name grid job show --json", re)
	}
	if body["kind"] != "security" || body["class"] != nil {
		t.Errorf("kind %v class %v, want security and null", body["kind"], body["class"])
	}
}
