package cli

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/projectenv"
)

// bs2Run runs grid project env with --json, so the answer is checked against the contract schema.
func bs2Run(t *testing.T, h *harness, args ...string) map[string]any {
	t.Helper()
	return lnEnvCliRun(t, h, append(args, "--json")...)
}

// bs2Steps reads project alpha's stored steps of environment id through list --json.
func bs2Steps(t *testing.T, h *harness, id string) []any {
	t.Helper()
	answer := bs2Run(t, h, "list", "alpha", "--json")
	for _, it := range answer["items"].([]any) {
		m := it.(map[string]any)
		if m["id"] == id {
			steps, _ := m["steps"].([]any)
			return steps
		}
	}
	t.Fatalf("environment %s is not listed: %v", id, answer)
	return nil
}

func bs2Step(wf, step string) map[string]any { return map[string]any{"wf": wf, "step": step} }

func TestBs2EnvAddStepsRepeat(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	bs2Run(t, h, "add", "alpha", "--id", "dev", "--name", "A", "--kind", "local", "--step", "build:test", "--step", "ship:deploy")
	got := bs2Steps(t, h, "dev")
	want := []any{bs2Step("build", "test"), bs2Step("ship", "deploy")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %v, want %v", got, want)
	}
}

func TestBs2EnvEditSteps(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	bs2Run(t, h, "add", "alpha", "--id", "dev", "--name", "A", "--kind", "local", "--step", "build:test", "--step", "ship:deploy")

	// An edit with no --step keeps the stored steps.
	bs2Run(t, h, "edit", "alpha", "--id", "dev", "--name", "B")
	if got, want := bs2Steps(t, h, "dev"), []any{bs2Step("build", "test"), bs2Step("ship", "deploy")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after an edit without --step: %v, want %v", got, want)
	}

	// Give a stored text to ship:deploy.
	entries, err := projectenv.List(brain.Store{GridHome: h.gridHome, Project: "alpha"})
	if err != nil || len(entries) != 1 {
		t.Fatalf("list: %v %v", entries, err)
	}
	e := entries[0]
	e.Steps[1].Text = "Push the build"
	if err := os.WriteFile(lnEnvCliPath(h, "dev"), []byte(lnEnvCliRender(t, e)), 0o600); err != nil {
		t.Fatal(err)
	}

	// Given steps replace the list; the text stays for the same wf and step.
	bs2Run(t, h, "edit", "alpha", "--id", "dev", "--step", "ship:deploy")
	got := bs2Steps(t, h, "dev")
	want := []any{map[string]any{"wf": "ship", "step": "deploy", "text": "Push the build"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after --step ship:deploy: %v, want %v", got, want)
	}

	// A step the stored list did not hold has no text.
	bs2Run(t, h, "edit", "alpha", "--id", "dev", "--step", "ship:deploy", "--step", "ship:verify")
	got = bs2Steps(t, h, "dev")
	want = []any{map[string]any{"wf": "ship", "step": "deploy", "text": "Push the build"}, bs2Step("ship", "verify")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after a second step: %v, want %v", got, want)
	}

	// One empty --step clears the list; the other fields stay.
	bs2Run(t, h, "edit", "alpha", "--id", "dev", "--step", "")
	if got := bs2Steps(t, h, "dev"); len(got) != 0 {
		t.Fatalf("after --step \"\": %v, want none", got)
	}
	if raw, _ := lnEnvCliRaw(t, h, "dev"); !strings.Contains(raw, "B") {
		t.Fatalf("the name was lost: %s", raw)
	}
}

func TestBs2EnvStepRefused(t *testing.T) {
	for _, bad := range []string{"build", ":test", "build:", ":", "x"} {
		for _, verb := range []string{"add", "edit"} {
			h := lnEnvCliHarness(t, "owner", "bo")
			args := []string{verb, "alpha", "--id", "dev", "--name", "A", "--kind", "local", "--step", bad}
			if verb == "edit" {
				bs2Run(t, h, "add", "alpha", "--id", "dev", "--name", "A", "--kind", "local", "--step", "keep:me")
			}
			before, existed := lnEnvCliRaw(t, h, "dev")
			if code := h.run(h.tmp, append([]string{"project", "env"}, args...)...); code != 2 {
				t.Fatalf("%s --step %q: exit %d, want 2", verb, bad, code)
			}
			if !strings.Contains(h.errb.String(), "--step") {
				t.Fatalf("%s --step %q: message %q does not name --step", verb, bad, h.errb)
			}
			after, existsNow := lnEnvCliRaw(t, h, "dev")
			if after != before || existed != existsNow {
				t.Fatalf("%s --step %q stored something:\n%s\nwas:\n%s", verb, bad, after, before)
			}
		}
	}
}

func TestBs2EnvStepOnlyOnAddAndEdit(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	for _, verb := range []string{"list", "remove"} {
		if code := h.run(h.tmp, "project", "env", verb, "alpha", "--id", "dev", "--step", "a:b"); code != 2 {
			t.Fatalf("%s with --step: exit %d, want 2", verb, code)
		}
	}
}

func TestBs2EnvUsageNamesStep(t *testing.T) {
	if !strings.Contains(projectEnvUsage, "--step WF:STEP") {
		t.Fatalf("usage does not name --step: %s", projectEnvUsage)
	}
}
