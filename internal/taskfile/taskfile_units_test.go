package taskfile

import (
	"reflect"
	"strings"
	"testing"
)

func TestScalarFrontmatterValues(t *testing.T) {
	text := strings.Join([]string{
		"---",
		`double: "two words"`,
		`single: 'one'`,
		"plain: bare value",
		"commented: kept # dropped",
		`quotedhash: "issue #12"`,
		"empty:",
		"emptyspace:   ",
		"list: [a, b]",
		"workflow:",
		"  - {step: plan, kind: approve}",
		"---",
		"body line",
	}, "\n")
	got := ScalarFrontmatter(text)
	if got == nil {
		t.Fatal("got nil, want frontmatter")
	}
	want := map[string]any{
		"double":     "two words",
		"single":     "one",
		"plain":      "bare value",
		"commented":  "kept",
		"quotedhash": "issue #12",
		"empty":      "",
		"emptyspace": "",
		// The scalar reader does not split inline lists and ignores list items.
		"list":     "[a, b]",
		"workflow": "",
	}
	if !reflect.DeepEqual(got.Data, want) {
		t.Fatalf("data = %#v; want %#v", got.Data, want)
	}
	if got.Body != "body line" {
		t.Fatalf("body = %q; want %q", got.Body, "body line")
	}
	if got.Odd {
		t.Fatal("odd = true; the scalar reader never sets it")
	}
}

func TestScalarFrontmatterMissingKey(t *testing.T) {
	got := ScalarFrontmatter("---\na: 1\n---\n")
	if got == nil {
		t.Fatal("got nil, want frontmatter")
	}
	if v, ok := got.Data["absent"]; ok {
		t.Fatalf("absent key present with value %#v", v)
	}
	if v, ok := got.Data["a"]; !ok || v != "1" {
		t.Fatalf("a = %#v, %t; want \"1\", true", v, ok)
	}
}

func TestScalarFrontmatterNoBlock(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"empty text", ""},
		{"plain body", "title: x\nbody\n"},
		{"opening only", "---\na: 1\nb: 2\n"},
		{"opening only crlf", "---\r\na: 1\r\n"},
		{"opening line only", "---"},
		{"not at the start", "\n---\na: 1\n---\n"},
		{"indented fence", " ---\na: 1\n---\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScalarFrontmatter(tc.text); got != nil {
				t.Fatalf("got %+v; want nil", got)
			}
		})
	}
}

func TestScalarFrontmatterCRLFAndBody(t *testing.T) {
	got := ScalarFrontmatter("---\r\na: x\r\nb: 'y'\r\n---\r\nrest\r\nmore")
	if got == nil {
		t.Fatal("got nil, want frontmatter")
	}
	if got.Data["a"] != "x" || got.Data["b"] != "y" {
		t.Fatalf("data = %#v; want a=x b=y", got.Data)
	}
	if got.Body != "rest\r\nmore" {
		t.Fatalf("body = %q", got.Body)
	}
}

func TestScalarFrontmatterDuplicateKeyLastWins(t *testing.T) {
	got := ScalarFrontmatter("---\nx: a\nx: b\n---\n")
	if got == nil || got.Data["x"] != "b" {
		t.Fatalf("got %+v; want x=b", got)
	}
}

func TestWorkflowStepsReadsInlineAndBlockLists(t *testing.T) {
	want := []WorkflowStep{{"research", "run"}, {"plan-review", "approve"}, {"build", "work"}}
	for name, text := range map[string]string{
		"inline": "---\nworkflow: [{step: research, kind: run}, {step: plan-review, kind: approve}, {step: build, kind: work}]\n---\n",
		"block":  "---\nworkflow:\n  - {step: research, kind: run}\n  - {step: plan-review, kind: approve}\n  - {step: build, kind: work}\n---\nbody",
	} {
		t.Run(name, func(t *testing.T) {
			if got := WorkflowSteps(text); !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v; want %+v", got, want)
			}
		})
	}
}

func TestWorkflowStepsRefusesUnreadable(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"no frontmatter", "workflow: [{step: a, kind: run}]"},
		{"no workflow key", "---\ntitle: x\n---\n"},
		{"empty list", "---\nworkflow: []\n---\n"},
		{"named workflow string", "---\nworkflow: default\n---\n"},
		{"item not a map", "---\nworkflow: [plan, build]\n---\n"},
		{"missing kind", "---\nworkflow: [{step: a}]\n---\n"},
		{"missing step", "---\nworkflow: [{kind: run}]\n---\n"},
		{"upper case step", "---\nworkflow: [{step: Build, kind: run}]\n---\n"},
		{"step with space", "---\nworkflow: [{step: 'a b', kind: run}]\n---\n"},
		{"step starts with dash", "---\nworkflow: [{step: -a, kind: run}]\n---\n"},
		{"duplicate step", "---\nworkflow: [{step: a, kind: run}, {step: a, kind: approve}]\n---\n"},
		{"one bad item drops all", "---\nworkflow: [{step: a, kind: run}, {step: B, kind: run}]\n---\n"},
		{"odd key elsewhere", "---\nx: {constructor: y}\nworkflow: [{step: a, kind: run}]\n---\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := WorkflowSteps(tc.text); got != nil {
				t.Fatalf("got %+v; want nil", got)
			}
		})
	}
}

func TestWorkflowStepsKeepsUnknownKind(t *testing.T) {
	got := WorkflowSteps("---\nworkflow: [{step: a, kind: typo}]\n---\n")
	if want := []WorkflowStep{{"a", "typo"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
}

func steps(pairs ...string) []WorkflowStep {
	var out []WorkflowStep
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, WorkflowStep{pairs[i], pairs[i+1]})
	}
	return out
}

func TestPlanApprovalIndex(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps []WorkflowStep
		want  int
	}{
		{"nil", nil, -1},
		{"empty", []WorkflowStep{}, -1},
		{"no approve step", steps("research", "run", "build", "work"), -1},
		{"only a ship approval after run", steps("build", "run", "sign-off", "approve"), -1},
		{"only a ship approval after work", steps("build", "work", "sign-off", "approve"), -1},
		{"approve first", steps("sign-off", "approve", "build", "run"), 0},
		{"approve before work", steps("sign-off", "approve", "build", "work"), 0},
		{"named plan after run", steps("research", "run", "plan-review", "approve"), 1},
		{"plan name with later steps", steps("research", "run", "my-plan-gate", "approve", "build", "work"), 1},
		{"unknown kinds do not count as run", steps("a", "typo", "gate", "approve"), 1},
		{"several approvals, first is plan", steps("plan", "approve", "build", "run", "sign-off", "approve"), 0},
		{"several approvals, ship then named plan", steps("build", "run", "sign-off", "approve", "plan-again", "approve"), 2},
		{"several approvals, two named plans", steps("research", "run", "plan-a", "approve", "plan-b", "approve"), 1},
		{"several approvals before run, first wins", steps("first", "approve", "second", "approve", "build", "run"), 0},
		{"ship approval then another ship approval", steps("build", "run", "ship-a", "approve", "ship-b", "approve"), -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlanApprovalIndex(tc.steps); got != tc.want {
				t.Fatalf("index = %d; want %d", got, tc.want)
			}
		})
	}
}

func TestPlanApprovalIndexFromWorkflowText(t *testing.T) {
	text := "---\nworkflow:\n  - {step: build, kind: run}\n  - {step: sign-off, kind: approve}\n  - {step: plan-two, kind: approve}\n---\n"
	got := WorkflowSteps(text)
	if len(got) != 3 {
		t.Fatalf("steps = %+v; want 3", got)
	}
	if i := PlanApprovalIndex(got); i != 2 {
		t.Fatalf("index = %d; want 2", i)
	}
	if !PlanApprovalRequired(text) {
		t.Fatal("plan approval required = false; want true when the index is found")
	}
}

func TestPlanApprovalRequiredAgreesWithIndex(t *testing.T) {
	wrap := func(w string) string { return "---\nworkflow: " + w + "\n---\n" }
	for _, tc := range []struct {
		name, workflow string
		index          int
		required       bool
	}{
		{"no approve step", "[{step: a, kind: run}, {step: b, kind: work}]", -1, false},
		{"ship approval only", "[{step: a, kind: run}, {step: ship, kind: approve}]", -1, false},
		{"approve first", "[{step: ok, kind: approve}, {step: a, kind: run}]", 0, true},
		{"several approvals", "[{step: plan, kind: approve}, {step: a, kind: run}, {step: ship, kind: approve}]", 0, true},
		{"named plan after run", "[{step: a, kind: run}, {step: ship, kind: approve}, {step: plan-b, kind: approve}]", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := wrap(tc.workflow)
			s := WorkflowSteps(text)
			if i := PlanApprovalIndex(s); i != tc.index {
				t.Fatalf("index = %d; want %d", i, tc.index)
			}
			if got := PlanApprovalRequired(text); got != tc.required {
				t.Fatalf("required = %t; want %t", got, tc.required)
			}
			if (PlanApprovalIndex(s) >= 0) && !PlanApprovalRequired(text) {
				t.Fatal("index found but plan approval not required")
			}
		})
	}
}

func TestPlanApprovalRequiredUnreadableWorkflow(t *testing.T) {
	// No approve step is found, but the workflow cannot be read, so the
	// documented default is to require plan approval.
	for _, tc := range []struct{ name, text string }{
		{"no frontmatter", "body only"},
		{"no workflow key", "---\ntitle: x\n---\n"},
		{"empty list", "---\nworkflow: []\n---\n"},
		{"named workflow", "---\nworkflow: default\n---\n"},
		{"malformed step", "---\nworkflow: [{step: A, kind: run}]\n---\n"},
		{"duplicate step", "---\nworkflow: [{step: a, kind: run}, {step: a, kind: run}]\n---\n"},
		{"unknown kind", "---\nworkflow: [{step: a, kind: run}, {step: b, kind: typo}]\n---\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !PlanApprovalRequired(tc.text) {
				t.Fatal("required = false; want true")
			}
		})
	}
}

func TestHistoryLineVia(t *testing.T) {
	for _, tc := range []struct {
		name   string
		detail string
		via    []string
		want   string
	}{
		{"no via argument", "d", nil, "- t · ev · d · by who"},
		{"no via argument, no detail", "", nil, "- t · ev · by who"},
		{"via given", "d", []string{"app"}, "- t · ev · d · by who · via app"},
		{"via given, no detail", "", []string{"app"}, "- t · ev · by who · via app"},
		{"empty via is left out", "d", []string{""}, "- t · ev · d · by who"},
		{"via is not trimmed or escaped", "", []string{" a\nb "}, "- t · ev · by who · via  a\nb "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HistoryLine("t", "ev", tc.detail, "who", tc.via...); got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}
