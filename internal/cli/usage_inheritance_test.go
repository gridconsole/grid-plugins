package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/usage"
)

func TestUsageWorkflowEditsAfterSave(t *testing.T) {
	for _, firstSave := range []string{"threshold", "notice"} {
		t.Run(firstSave, func(t *testing.T) {
			h, task := usageTask(t)
			policy := func(values string) {
				h.write(filepath.Join(task, "workflow.md"), "---\nusage_notice_thresholds: {tokens: "+values+"}\n---\n")
			}
			run := func(args ...string) {
				t.Helper()
				if code := h.run(task, append([]string{"usage"}, args...)...); code != 0 {
					t.Fatal(code, h.errb)
				}
			}
			post := func(delta string, want []float64) {
				t.Helper()
				run("post", "--job", "lead", "--tokens", delta, "--json")
				if h.errb.Len() != 0 {
					t.Fatal(h.errb)
				}
				var out usage.PostResult
				if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				got := []float64{}
				for _, n := range out.Notices {
					got = append(got, n.Body["threshold"].(float64))
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("notices = %v, want %v", got, want)
				}
			}
			h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\n")
			policy("[90]")
			switch firstSave {
			case "threshold":
				run("threshold", "set", "--dollars", "25", "--json")
			case "notice":
				run("post", "--job", "lead", "--tokens", "80", "--json")
			}
			if !strings.Contains(h.errb.String(), "usage_notice_thresholds.tokens") {
				t.Fatal("missing invalid-policy warning", h.errb)
			}
			policy("[90, 100]")
			if firstSave == "notice" {
				post("0", []float64{})
			} else {
				post("90", []float64{90})
			}
			// Remove and restore an already-fired percentage, retaining its lifetime ID.
			policy("[90, 100]")
			if firstSave == "notice" {
				post("10", []float64{90})
			} else {
				post("0", []float64{})
			}
			policy("[80, 90, 95, 100]")
			if firstSave == "notice" {
				post("5", []float64{95})
			} else {
				post("5", []float64{80, 95})
			}
			post("5", []float64{100})
			post("0", []float64{})
			run("threshold", "show", "--json")
			var c usage.Configuration
			if err := json.Unmarshal(h.out.Bytes(), &c); err != nil {
				t.Fatal(err)
			}
			if len(c.Fired) != 4 {
				t.Fatalf("lost fired bookkeeping: %v", c.Fired)
			}
			if firstSave == "threshold" && !reflect.DeepEqual(c.Dollars, []float64{25}) {
				t.Fatal("lost explicit dollar thresholds", c.Dollars)
			}
		})
	}
}

func TestUsageWorkflowUnsafeFileDefaults(t *testing.T) {
	for _, kind := range []string{"oversize", "symlink", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			h, task := usageTask(t)
			path := filepath.Join(task, "workflow.md")
			text := "---\nusage_notice_thresholds: {tokens: [90, 100]}\n---\n"
			if kind == "oversize" {
				h.write(path, text+strings.Repeat("x", (1<<20)+1-len(text)))
			} else {
				target := filepath.Join(t.TempDir(), "policy.md")
				if kind == "symlink" {
					h.write(target, text)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\n")
			if code := h.run(task, "usage", "post", "--job", "lead", "--tokens", "80", "--json"); code != 0 {
				t.Fatal(code, h.errb)
			}
			if !strings.Contains(h.errb.String(), "usage_notice_thresholds: cannot read workflow.md") || strings.Contains(h.errb.String(), task) {
				t.Fatal("missing path-free refusal", h.errb)
			}
			var out usage.PostResult
			if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if len(out.Notices) != 1 || out.Notices[0].Body["threshold"] != float64(80) {
				t.Fatalf("default not retained: %+v", out)
			}
		})
	}
}

func TestUsageThresholdPrecedenceHelp(t *testing.T) {
	h, task := usageTask(t)
	if code := h.run(task, "usage", "--help"); code != 0 {
		t.Fatal(code, h.errb)
	}
	if !strings.Contains(h.out.String(), "Per unit: a threshold set on the task wins, then the workflow's usage thresholds, then the default.") {
		t.Fatal("help omits threshold precedence")
	}
}
