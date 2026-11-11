package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func planDigestRenderRecords(t *testing.T) planDigestTranscript {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(planDigestFixture(), "transcript.json"))
	if err != nil {
		t.Fatal(err)
	}
	var transcript planDigestTranscript
	if err := json.Unmarshal(data, &transcript); err != nil {
		t.Fatal(err)
	}
	return transcript
}

func planDigestRenderStep(t *testing.T, transcript planDigestTranscript, scenario string, number int) planDigestStep {
	t.Helper()
	for _, s := range transcript.Scenarios {
		if s.Name != scenario {
			continue
		}
		if number < 1 || number > len(s.Steps) {
			t.Fatalf("scenario %s has no step %d", scenario, number)
		}
		step := s.Steps[number-1]
		if len(step.Argv) < 2 || step.Argv[0] != "job" || step.Argv[1] != "render" {
			t.Fatalf("%s step %d is %v, want job render", scenario, number, step.Argv)
		}
		return step
	}
	t.Fatalf("no scenario %s", scenario)
	return planDigestStep{}
}

// Job render no longer depends on plan approval, so no recorded render step may
// still carry the removed refusal.
func TestPlanDigestRenderRecordsNoApprovalRefusal(t *testing.T) {
	transcript := planDigestRenderRecords(t)
	for _, scenario := range transcript.Scenarios {
		for i, step := range scenario.Steps {
			if len(step.Argv) < 2 || step.Argv[0] != "job" || step.Argv[1] != "render" {
				continue
			}
			if strings.Contains(step.StderrContains, "plan needs approval") {
				t.Errorf("%s step %d records stderr %q", scenario.Name, i+1, step.StderrContains)
			}
		}
	}
}

func TestPlanDigestRenderRecordsDigestAndStale(t *testing.T) {
	step := planDigestRenderStep(t, planDigestRenderRecords(t), "digest-and-stale", 7)
	if step.Exit != 1 {
		t.Errorf("digest-and-stale step 7 exit = %d, want 1", step.Exit)
	}
	if want := "already has different saved inputs"; !strings.Contains(step.StderrContains, want) {
		t.Errorf("digest-and-stale step 7 stderr_contains = %q, want %q", step.StderrContains, want)
	}
}

func TestPlanDigestRenderRecordsLegacyReapprove(t *testing.T) {
	step := planDigestRenderStep(t, planDigestRenderRecords(t), "legacy-reapprove", 3)
	if step.Exit != 0 {
		t.Errorf("legacy-reapprove step 3 exit = %d, want 0", step.Exit)
	}
	if step.StderrContains != "" {
		t.Errorf("legacy-reapprove step 3 stderr_contains = %q, want none", step.StderrContains)
	}
}
