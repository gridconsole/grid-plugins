package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/usage"
)

func TestUsageWorkflowThresholds(t *testing.T) {
	for _, tc := range []struct {
		name, policy, taskTokens, warning string
		tokens, dollars, minutes          []float64
	}{
		{name: "workflow", policy: "tokens: [90, 100]", tokens: []float64{90, 100}, dollars: []float64{50, 80, 100}, minutes: []float64{50, 80, 100}},
		{name: "task-wins", policy: "tokens: [90, 100]", taskTokens: "[50,80,100]", tokens: []float64{50, 80, 100}, dollars: []float64{50, 80, 100}, minutes: []float64{50, 80, 100}},
		{name: "task-clear-wins", policy: "tokens: [90, 100]", taskTokens: "[]", tokens: []float64{}, dollars: []float64{50, 80, 100}, minutes: []float64{50, 80, 100}},
		{name: "omitted-token", policy: "dollars: [90, 100]", tokens: []float64{80, 100}, dollars: []float64{90, 100}, minutes: []float64{50, 80, 100}},
		{name: "invalid-token", policy: "tokens: [90]\n  dollars: [90, 100]", warning: "usage_notice_thresholds.tokens: last value must be 100", tokens: []float64{80, 100}, dollars: []float64{90, 100}, minutes: []float64{50, 80, 100}},
		{name: "seam-limit", policy: "tokens: [10, 20, 30, 40, 50, 60, 70, 80, 100]", warning: "usage_notice_thresholds tokens: at most 8 thresholds per unit", tokens: []float64{80, 100}, dollars: []float64{50, 80, 100}, minutes: []float64{50, 80, 100}},
		{name: "multiple-invalid-units", policy: "tokens: [90]\n  minutes: []", warning: "usage_notice_thresholds.minutes: must be a nonempty list of numbers\nusage_notice_thresholds.tokens: last value must be 100", tokens: []float64{80, 100}, dollars: []float64{50, 80, 100}, minutes: []float64{50, 80, 100}},
		{name: "invalid-shape", policy: "tokens: invalid", warning: "usage_notice_thresholds.tokens: must be a nonempty list of numbers", tokens: []float64{80, 100}, dollars: []float64{50, 80, 100}, minutes: []float64{50, 80, 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, task := usageTask(t)
			h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\n")
			h.write(filepath.Join(task, "workflow.md"), "---\nusage_notice_thresholds:\n  "+tc.policy+"\n---\n")
			if tc.taskTokens != "" {
				h.write(filepath.Join(task, "usage", "thresholds.json"), `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"tokens":`+tc.taskTokens+`,"budget":{},"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{}}`)
			}
			checkWarning := func() {
				t.Helper()
				want := ""
				if tc.warning != "" {
					want = "usage: warning: " + tc.warning + "\n"
				}
				if h.errb.String() != want {
					t.Errorf("stderr = %q, want %q", h.errb.String(), want)
				}
			}
			if code := h.run(task, "usage", "threshold", "show", "--json"); code != 0 {
				t.Fatal(code, h.errb)
			}
			checkWarning()
			var c usage.Configuration
			if err := json.Unmarshal(h.out.Bytes(), &c); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.Tokens, tc.tokens) || !reflect.DeepEqual(c.Dollars, tc.dollars) || !reflect.DeepEqual(c.Minutes, tc.minutes) {
				t.Errorf("effective thresholds = tokens %v dollars %v minutes %v; want %v %v %v", c.Tokens, c.Dollars, c.Minutes, tc.tokens, tc.dollars, tc.minutes)
			}
			previous := 0
			for _, total := range []int{50, 79, 80, 89, 90, 100, 101} {
				if code := h.run(task, "usage", "post", "--job", "lead", "--tokens", strconv.Itoa(total-previous), "--json"); code != 0 {
					t.Fatal(code, h.errb)
				}
				checkWarning()
				var out usage.PostResult
				if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				got, want := []float64{}, []float64{}
				for _, n := range out.Notices {
					got = append(got, n.Body["threshold"].(float64))
				}
				for _, p := range tc.tokens {
					if float64(previous) < p && p <= float64(total) {
						want = append(want, p)
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("total %d: notices %v, want %v", total, got, want)
				}
				if out.Totals.Tokens != uint64(total) {
					t.Fatalf("usage lost: %+v", out.Totals)
				}
				previous = total
			}
		})
	}
}

func TestUsageWorkflowConfigure(t *testing.T) {
	h, task := usageTask(t)
	h.write(filepath.Join(task, "workflow.md"), "---\nusage_notice_thresholds: {tokens: [90, 100], minutes: [90, 100]}\n---\n")
	setTaskBudget(t, h, task, usage.Change{Dollars: []float64{100}})
	if code := h.run(task, "usage", "threshold", "show", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	var c usage.Configuration
	if err := json.Unmarshal(h.out.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Tokens, []float64{90, 100}) || !reflect.DeepEqual(c.Minutes, []float64{90, 100}) {
		t.Fatalf("Configure ignored workflow: tokens %v minutes %v", c.Tokens, c.Minutes)
	}
	if code := h.run(task, "usage", "post", "--job", "lead", "--dollars", "50", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	var out usage.PostResult
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Notices) != 1 || out.Notices[0].Body["threshold"] != float64(50) {
		t.Fatalf("omitted dollars lost default: %+v", out.Notices)
	}
}

func TestUsageWorkflowPreservesNotices(t *testing.T) {
	h, task := usageTask(t)
	h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\n")
	if code := h.run(task, "usage", "post", "--job", "lead", "--tokens", "80", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	files, err := filepath.Glob(filepath.Join(task, "feed", "*.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatal("missing notice files", err)
	}
	before := map[string][]byte{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		before[file] = data
	}
	// Recover an existing notice even when its fired configuration was not saved.
	if err := os.Remove(filepath.Join(task, "usage", "thresholds.json")); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(task, "workflow.md"), "---\nusage_notice_thresholds: {tokens: [80, 90, 100]}\n---\n")
	for _, amount := range []string{"0", "0"} {
		if code := h.run(task, "usage", "post", "--job", "lead", "--tokens", amount, "--json"); code != 0 || h.errb.Len() != 0 {
			t.Fatal(code, h.errb)
		}
	}
	for file, want := range before {
		got, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("existing notice file changed", err)
		}
	}
	rows, err := (feed.Store{TaskDir: task, TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}).Read(feed.Filter{Type: "usage-notice"})
	if err != nil || len(rows.Records) != 1 {
		t.Fatalf("lost or duplicated notices: count %d: %v", len(rows.Records), err)
	}
	for _, row := range rows.Records {
		if row.Verdict != "counted" {
			t.Fatal(row.Verdict)
		}
	}
}
