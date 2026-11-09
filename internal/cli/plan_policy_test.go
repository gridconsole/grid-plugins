package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestApproveWorkflowPlanHashToken(t *testing.T) {
	for _, tc := range []struct {
		name, workflow, token, message string
		code                           int
	}{
		{"digest-short", "---\nplan_hash_token: digest\n---\n", plan.Hash(approvedPlan), "grid: invalid task edit: plan_hash_token digest requires 64 lowercase hex digits\n", 2},
		{"digest-full", "---\nplan_hash_token: digest\n---\n", plan.Digest(approvedPlan), "", 0},
		{"invalid", "---\nplan_hash_token: nonsense\n---\n", plan.Digest(approvedPlan), "grid approve: workflow.md: plan_hash_token must be any or digest\n", 3},
		{"broken", "---\nplan_hash_token: [\n---\n", plan.Digest(approvedPlan), "grid approve: workflow.md: plan_hash_token must be any or digest\n", 3},
		{"default-short", "", plan.Hash(approvedPlan), "", 0},
		{"any-short", "---\nplan_hash_token: any\n---\n", plan.Hash(approvedPlan), "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			fixture := taskFixture(t, h)
			if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
				t.Fatal(err)
			}
			h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
			if tc.workflow != "" {
				h.write(filepath.Join(fixture.Path, "workflow.md"), tc.workflow)
			}
			before := taskTestSnapshot(t, fixture.Path)
			code := h.run(h.home, "approve", fixture.ID, "--plan-hash", tc.token, "--json")
			if code != tc.code || h.errb.String() != tc.message {
				t.Errorf("exit %d, want %d; stderr %q, want %q", code, tc.code, h.errb.String(), tc.message)
			}
			if tc.code != 0 {
				if !reflect.DeepEqual(before, taskTestSnapshot(t, fixture.Path)) {
					t.Error("refused approval wrote task data or a receipt")
				}
				return
			}
			var event task.Event
			if err := json.Unmarshal(h.out.Bytes(), &event); err != nil || event.PlanDigest != plan.Digest(approvedPlan) {
				t.Fatalf("approval receipt missing digest: %s (%v)", h.out, err)
			}
		})
	}
}

func TestApproveWorkflowGuestGateFirst(t *testing.T) {
	for _, policy := range []string{"digest", "nonsense"} {
		t.Run(policy, func(t *testing.T) {
			h := newHarness(t)
			h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			fixture := taskFixtureProject(t, h, "example")
			h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nplan_hash_token: "+policy+"\n---\n")
			h.env.Getenv = func(key string) string {
				if key == "GRID_ROLE" {
					return "guest"
				}
				return ""
			}
			before := taskTestSnapshot(t, fixture.Path)
			if code := h.run(h.home, "approve", fixture.ID, "--plan-hash", plan.Hash(approvedPlan)); code != 3 || h.errb.String() != "grid approve: "+personHint+" (reason: agent_env).\n" {
				t.Fatalf("person gate must run first: exit %d, stderr %q", code, h.errb.String())
			}
			if !reflect.DeepEqual(before, taskTestSnapshot(t, fixture.Path)) {
				t.Fatal("guest refusal wrote task data")
			}
		})
	}
}

func TestTaskShowPlanDigest(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	for _, withPlan := range []bool{false, true} {
		if withPlan {
			h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
		}
		if code := h.run(h.home, "task", "show", fixture.ID, "--json"); code != 0 {
			t.Fatalf("task show exit %d: %s", code, h.errb)
		}
		if err := contract.Validate("cmd-task-show", h.out.Bytes()); err != nil {
			t.Fatal(err)
		}
		var shown map[string]any
		if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil {
			t.Fatal(err)
		}
		if withPlan {
			if code := h.run(fixture.Path, "plan", "hash", "--json"); code != 0 {
				t.Fatalf("plan hash exit %d: %s", code, h.errb)
			}
			var hashed struct{ Hash, Digest string }
			if err := json.Unmarshal(h.out.Bytes(), &hashed); err != nil {
				t.Fatal(err)
			}
			if len(hashed.Digest) != 64 || shown["plan_digest"] != hashed.Digest || shown["plan_hash"] != hashed.Hash {
				t.Errorf("task show plan_digest = %v; want %s; plan_hash = %v; want %s", shown["plan_digest"], hashed.Digest, shown["plan_hash"], hashed.Hash)
			}
		} else if shown["plan_digest"] != nil || shown["plan_hash"] != nil {
			t.Error("task without a plan exposed plan tokens")
		}
		if code := h.run(h.home, "task", "show", fixture.ID); code != 0 {
			t.Fatalf("task show exit %d: %s", code, h.errb)
		}
		if withPlan {
			if !strings.Contains(h.out.String(), "plan_hash: "+plan.Hash(approvedPlan)+"\nplan digest: "+plan.Digest(approvedPlan)+"\n") {
				t.Error("task show text is missing the adjacent plan hash and plan digest lines")
			}
		} else if strings.Contains(h.out.String(), "plan_hash:") || strings.Contains(h.out.String(), "plan digest:") {
			t.Error("task without a plan printed plan tokens")
		}
	}
}
