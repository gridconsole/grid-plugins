package orchestrator

import (
	"errors"
	"strings"
	"testing"
)

// beCheckerTools is the tools: line of agents/checker.md, in order.
var beCheckerTools = []string{"Read", "Grep", "Glob", "Bash", "mcp__claude-in-chrome__*", "mcp__Claude_Browser__*"}

const beCheckerToolsJSON = `"tools":["Read","Grep","Glob","Bash","mcp__claude-in-chrome__*","mcp__Claude_Browser__*"]`

const beNineKeys = "assign,attempt,fence,gen,holder,id,task,task_dir,task_epoch"

// Row 1: a Job of role checker is spawned with the tools the Tools callback answers for its role, in order;
// a Job whose role answers none has no tools key.
func TestBeSpawnCarriesCheckerTools(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "checker", "builder")
	s := h.stream(1234, nil)
	s.Tools = func(_, role string) ([]string, error) {
		if role == "checker" {
			return beCheckerTools, nil
		}
		return nil, nil
	}
	h.pass(s)
	raw := h.out.String()
	if !strings.Contains(raw, beCheckerToolsJSON) {
		t.Errorf("spawn lines %q lack %s", raw, beCheckerToolsJSON)
	}
	if n := strings.Count(raw, `"tools"`); n != 1 {
		t.Errorf("%d tools keys on the lines, want 1 (the builder Job answers none): %q", n, raw)
	}
	got := map[string]string{}
	for _, sp := range kinds(h.lines(), "spawn") {
		got[sp["id"].(string)] = spawnKeys(sp)
	}
	if want := beNineKeys + ",tools"; got["J-1"] != want {
		t.Errorf("checker spawn keys %q, want %q", got["J-1"], want)
	}
	if got["J-2"] != beNineKeys {
		t.Errorf("builder spawn keys %q, want the nine", got["J-2"])
	}
}

// An empty answer leaves the key out, as an empty model does.
func TestBeSpawnEmptyToolsKeepsTheNineKeys(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder")
	s := h.stream(1234, nil)
	s.Tools = func(string, string) ([]string, error) { return []string{}, nil }
	h.pass(s)
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 {
		t.Fatalf("%d spawn lines, want 1", len(spawns))
	}
	if got := spawnKeys(spawns[0]); got != beNineKeys {
		t.Errorf("spawn keys %s, want %s", got, beNineKeys)
	}
}

// An error costs the spawn its tools key and one Diag line, never the Job.
func TestBeSpawnToolsErrorGoesOutWithoutTools(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "checker")
	s := h.stream(1234, nil)
	s.Tools = func(string, string) ([]string, error) { return beCheckerTools, errors.New("agent file unreadable") }
	h.pass(s)
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 {
		t.Fatalf("%d spawn lines, want 1 (the Job is not skipped)", len(spawns))
	}
	if got := spawnKeys(spawns[0]); got != beNineKeys {
		t.Errorf("spawn keys %s, want the nine without tools", got)
	}
	diag := strings.TrimSpace(h.diag.String())
	if strings.Count(diag, "\n") != 0 || !strings.Contains(diag, "agent file unreadable") || !strings.Contains(diag, h.ids[0]) || !strings.Contains(diag, "J-1") || !strings.Contains(diag, "checker") {
		t.Fatalf("want one Diag line naming the task, the Job, the role and the error, got %q", diag)
	}
}
