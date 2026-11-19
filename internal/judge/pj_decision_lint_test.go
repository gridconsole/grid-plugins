package judge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// pjReasonText is the prose of a decision Job: the acceptance rows' text, where the
// reason it was raised and the choices live (intent.goal is a pointer).
func pjReasonText(body map[string]any) string {
	var b strings.Builder
	rows, _ := body["acceptance"].([]any)
	for _, r := range rows {
		if row, ok := r.(map[string]any); ok {
			b.WriteString(str(row["text"]) + "\n")
		}
	}
	return b.String()
}

// A decision Job points its goal at the reviewed Job's verdict, so it lints with no
// job.intent finding, and keeps its reason in an acceptance row.
func TestDecisionBodyGoalIsAPrecisePointer(t *testing.T) {
	body := decisionBody("J-4.4", "J-4", "J-4.2", "Decide how J-4 continues", "The review of J-4 rejected it.", map[string]any{})
	if goal := str(body["intent"].(map[string]any)["goal"]); goal != "job:J-4.2#verdict" {
		t.Fatalf("goal %q, want job:J-4.2#verdict", goal)
	}
	if !strings.Contains(pjReasonText(body), "rejected it") {
		t.Fatalf("the reason is not in the acceptance rows: %v", body["acceptance"])
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range lint.CheckJSON(data, lint.Options{}) {
		if f.Rule == "job.intent" {
			t.Fatalf("lint finding %+v", f)
		}
	}
}
