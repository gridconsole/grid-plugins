package cli

import (
	"encoding/json"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkFixture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../testdata/job/task/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPlanCheck(t *testing.T) {
	base := checkFixture(t)
	for _, tc := range []struct{ name, old, next, want string }{
		{"derived fields", "", "", ""},
		{"missing field", "      skills: []\n", "", "missing required field skills"},
		{"missing owner", "owner: intent.md#request", "item: plan.md#step-1", "intent.owner"},
		{"unknown field", "skills: []", "skillz: []", "unknown planned field"},
		{"unknown nested field", "turns_estimate: 8", "turnz_estimate: 8", "unknown field"},
		{"unknown edge field", "after: []", "after: [{job: J-1, why: Needs another result, kinds: done}]", "unknown field"},
		{"run field", "skills: []", "skills: []\n      status: running", "unknown planned field"},
		{"below floor", "risk: []", "risk: [security]", "review_reason is required"},
		{"override", "risk: []", "risk: [security]\n      review_reason: Owner accepts the review exception", ""},
		{"small visible", "risk: []", "risk: []\n      visible: small", ""},
		{"irreversible", "risk: []", "risk: []\n      irreversible: true", "below floor L2"},
		{"zero tests", "verify: {kind: file-exists, path: docs/export.md}", "verify: {command: go test ./..., min_tests: 0}", "min_tests"},
		{"positive tests", "verify: {kind: file-exists, path: docs/export.md}", "verify: {command: go test ./..., min_tests: 1}", ""},
		{"missing target", "after: []", "after: [J-2]", "does not exist"},
		{"self cycle", "after: []", "after: [J-1]", "Job cycle: J-1 -> J-1"},
		{"step field", "  kind: non-code", "  review: L0\n  kind: non-code", "step-level review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			task := planTask(h)
			text := base
			if tc.old != "" {
				if !strings.Contains(text, tc.old) {
					t.Fatal("fixture lacks replacement")
				}
				text = strings.Replace(text, tc.old, tc.next, 1)
			}
			h.write(filepath.Join(task, "plan.md"), text)
			code := h.run(task, "plan", "check")
			if tc.want == "" {
				if code != 0 {
					t.Fatalf("%d: %s %s", code, h.out, h.errb)
				}
			} else if code != 1 || !strings.Contains(h.out.String(), tc.want) {
				t.Fatalf("%d: %s %s, want %s", code, h.out, h.errb, tc.want)
			}
		})
	}
}

func TestPlanCheckDuplicatesAndCycles(t *testing.T) {
	base := checkFixture(t)
	start := strings.Index(base, "- [ ]")
	end := strings.Index(base, "\n\n## Verify")
	block := base[start:end]
	for _, tc := range []struct{ name, text, want string }{
		{"duplicate N", base + "\n" + block, "duplicate item 1"},
		{"duplicate Job", base + "\n" + strings.Replace(block, "- [ ] 1", "- [ ] 2", 1), "duplicate Job J-1"},
		{"two Job cycle", strings.Replace(base, "after: []", "after: [J-2]", 1) + "\n" + strings.ReplaceAll(strings.ReplaceAll(strings.Replace(block, "- [ ] 1", "- [ ] 2", 1), "id: J-1", "id: J-2"), "after: []", "after: [J-1]"), "J-1 -> J-2 -> J-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems := strings.Join(jobgraph.CheckPlan(tc.text), "\n")
			if !strings.Contains(problems, tc.want) {
				t.Fatalf("%s", problems)
			}
		})
	}
	h := newHarness(t)
	task := planTask(h)
	h.write(filepath.Join(task, "plan.md"), base)
	if code := h.run(task, "plan", "check", "T-07", "--json"); code != 0 || !strings.Contains(h.out.String(), `"ok":true`) {
		t.Fatalf("%d: %s %s", code, h.out, h.errb)
	}
	text := strings.ReplaceAll(base, "skills: []", "skillz: []")
	text = strings.Replace(text, "risk: []", "risk: [security]", 1)
	h.write(filepath.Join(task, "plan.md"), text)
	if code := h.run(task, "plan", "check"); code != 1 || !strings.Contains(h.out.String(), "review_reason") || !strings.Contains(h.out.String(), "missing required field skills") {
		t.Fatalf("%d: %s", code, h.out)
	}
}

func TestPlanCheckVerifyCountWarnings(t *testing.T) {
	for _, tc := range []struct {
		command, kind string
		warn          bool
	}{
		{"go test ./...", "code", true},
		{"make test", "code", true},
		{"go test -v ./...", "code", false},
		{"go test -json ./...", "code", false},
		{"go test -v=false ./...", "code", true},
		{"go test -v=true ./...", "code", false},
		{"make test GOFLAGS=-v", "code", false},
		{"go test -v ./one && go test ./two", "code", true},
		{"go test ./one && go test -v ./two", "code", true},
		{"env GOOS=linux go test ./...", "code", true},
		{"node --test", "code", false},
		{"go test ./...", "non-code", false},
	} {
		t.Run(tc.command+"/"+tc.kind, func(t *testing.T) {
			h := newHarness(t)
			task := planTask(h)
			text := strings.Replace(checkFixture(t), "kind: non-code", "kind: "+tc.kind, 1)
			text = strings.Replace(text, "verify: {kind: file-exists, path: docs/export.md}", "verify: {command: '"+tc.command+"', min_tests: 1}", 1)
			h.write(filepath.Join(task, "plan.md"), text)
			if code := h.run(task, "plan", "check"); code != 0 {
				t.Fatalf("warning must not reject plan: %d: %s %s", code, h.out, h.errb)
			}
			if strings.Contains(h.out.String(), "warning: J-1: verify test totals may be unknown") != tc.warn {
				t.Fatalf("warning=%t, output=%s", tc.warn, h.out)
			}
			if code := h.run(task, "plan", "check", "--json"); code != 0 {
				t.Fatalf("JSON exit %d: %s %s", code, h.out, h.errb)
			}
			var report struct {
				OK                 bool `json:"ok"`
				Problems, Warnings []string
			}
			if err := json.Unmarshal(h.out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if !report.OK || len(report.Problems) != 0 || (len(report.Warnings) > 0) != tc.warn {
				t.Fatalf("report: %+v", report)
			}
		})
	}
}
