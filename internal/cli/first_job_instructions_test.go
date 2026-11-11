package cli

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/contract"
)

func orchestratorText(t *testing.T) string {
	t.Helper()
	text, ok := gridplugins.AgentFor("orchestrator")
	if !ok {
		t.Fatal("no agents/orchestrator.md")
	}
	return text
}

// Row 1: the text the render pastes into the first Job names the result command
// and plan.md.
func TestOrchestratorTextTellsTheFirstJobWhatToDo(t *testing.T) {
	text := orchestratorText(t)
	for _, want := range []string{"grid job result --task <task> --job <job> --body <json>", "plan.md", "grid plan write", "no result posted"} {
		if !strings.Contains(text, want) {
			t.Errorf("AgentFor(orchestrator) lacks %q", want)
		}
	}
	if i, j := strings.Index(text, "grid job result"), strings.Index(text, "8. Stop."); i < 0 || j < i {
		t.Errorf("Stop must come after the result step (result at %d, stop at %d)", i, j)
	}
	if !strings.Contains(strings.Split(text, "\n---\n")[0], "tools: Read, Grep, Glob, Bash, Skill, Write") {
		t.Error("front matter tools line lacks Write")
	}
}

var jsonFence = regexp.MustCompile("(?s)```json\n(.*?)\n```")

// Row 2: the example result body, cut out of the text, passes the result schema.
func TestOrchestratorExampleResultValidates(t *testing.T) {
	m := jsonFence.FindStringSubmatch(orchestratorText(t))
	if m == nil {
		t.Fatal("no fenced json block in agents/orchestrator.md")
	}
	var body any
	if err := json.Unmarshal([]byte(m[1]), &body); err != nil {
		t.Fatalf("example body is not JSON: %v", err)
	}
	post, err := json.Marshal(map[string]any{
		"id": "01K9J000000000000000000001", "task": "01K9J000000000000000000001",
		"type": "result", "type_v": 2,
		"author": map[string]any{"id": "worker", "role": "worker"},
		"tier":   "agent", "worker": "worker@01K9J000000000000000000002",
		"lease": "J-1", "gen": 0, "fence": 2, "seq": 1,
		"at": "2026-11-08T10:00:00.000Z", "re": nil, "after": []any{},
		"job": "J-1", "attempt": 1, "wake": nil, "body": body,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("result", post); err != nil {
		t.Fatalf("example body fails contract/schemas/result.json: %v", err)
	}
	// The example answers every acceptance row of the first Job.
	var rows struct {
		Acceptance []struct{ ID string } `json:"acceptance"`
	}
	if err := json.Unmarshal([]byte(m[1]), &rows); err != nil {
		t.Fatal(err)
	}
	if want := len(firstJob()["acceptance"].([]map[string]any)); len(rows.Acceptance) != want {
		t.Fatalf("example has %d acceptance rows, the first Job has %d", len(rows.Acceptance), want)
	}
}

// Row 3: every acceptance line and the verify of firstJob() is named by the text.
func TestOrchestratorTextNamesTheFirstJobsAcceptanceAndVerify(t *testing.T) {
	text := orchestratorText(t)
	job := firstJob()
	for _, a := range job["acceptance"].([]map[string]any) {
		if line := a["text"].(string); !strings.Contains(text, line) {
			t.Errorf("agent text does not carry acceptance %v: %q", a["id"], line)
		}
	}
	verify := job["verify"].(map[string]any)
	if kind, path := verify["kind"].(string), verify["path"].(string); !strings.Contains(text, "`"+kind+"` on `"+path+"`") {
		t.Errorf("agent text does not name verify %s on %s", kind, path)
	}
	for _, w := range job["write"].([]string) {
		if !strings.Contains(text, "`"+w+"`") {
			t.Errorf("agent text does not name write path %s", w)
		}
	}
}

// Rows 4 to 6 through the command: an empty finish prints "no result posted" and
// --json carries no_result; a result before the exit, and a crash, print neither.
func TestJobTreeShowsAnEmptyFinish(t *testing.T) {
	for _, tc := range []struct {
		name, exit string
		result     bool
		want       bool
	}{
		{"empty finish", `{"reason":"done"}`, false, true},
		{"result then done", `{"reason":"done"}`, true, false},
		{"crash", `{"reason":"crash"}`, false, false},
		{"cancelled", `{"reason":"cancelled"}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path, _, _, _ := assignedExitHarness(t, true)
			run := func(args ...string) string {
				t.Helper()
				h.out.Reset()
				h.errb.Reset()
				if code := h.run(path, args...); code != 0 {
					t.Fatalf("%v: exit %d: %s", args, code, h.errb)
				}
				return h.out.String()
			}
			if tc.result {
				run("job", "post", "J-1", "--attempt", "1", "--type", "result", "--body", `{"outcome":"finished","changed":[],"evidence":[],"acceptance":[],"verified":null,"must_fix":[],"found":[],"proposed_jobs":[],"decisions_needed":[],"knowledge":[]}`)
			}
			run("job", "post", "J-1", "--attempt", "1", "--type", "worker-exit", "--body", tc.exit)
			text := run("job", "tree")
			if got := strings.Contains(text, "no result posted"); got != tc.want {
				t.Fatalf("text tree shows no result posted = %v, want %v:\n%s", got, tc.want, text)
			}
			if tc.want {
				for _, line := range strings.Split(text, "\n") {
					if strings.HasPrefix(line, "  attempt 1") && !strings.HasSuffix(line, " no result posted") {
						t.Fatalf("attempt line does not end with the text: %q", line)
					}
				}
			}
			var rows []map[string]any
			if err := json.Unmarshal([]byte(run("job", "tree", "--json")), &rows); err != nil || len(rows) != 1 {
				t.Fatalf("%v %s", err, h.out)
			}
			attempt := rows[0]["attempts"].([]any)[0].(map[string]any)
			if tc.want && attempt["no_result"] != true {
				t.Fatalf("--json attempt lacks no_result true: %v", attempt)
			}
			if _, has := attempt["no_result"]; has != tc.want {
				t.Fatalf("--json no_result key present = %v, want %v: %v", has, tc.want, attempt)
			}
		})
	}
}
