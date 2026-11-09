package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestPlanHashDigestJSON(t *testing.T) {
	h := newHarness(t)
	dir := planTask(h)
	if code := h.run(dir, "plan", "hash", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var got struct {
		Hash   string `json:"hash"`
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	text := planFixture(t, "plan.md")
	if got.Hash != plan.Hash(text) || got.Digest != plan.Digest(text) {
		t.Fatalf("hash JSON = %s; want hash and full digest", h.out)
	}
	if err := contract.Validate("cmd-plan-hash", h.out.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("cmd-plan-hash", []byte(`{"hash":"abcdef"}`)); err != nil {
		t.Fatalf("legacy hash-only response refused: %v", err)
	}
	if code := h.run(dir, "plan", "hash"); code != 0 || h.out.String() != plan.Hash(text)+"\n" {
		t.Fatalf("plain hash changed: exit %d, %q", code, h.out)
	}
}

func TestApproveDigestCLI(t *testing.T) {
	for _, tc := range []struct {
		name, token string
		code        int
	}{
		{"digest", plan.Digest(approvedPlan), 0},
		{"short", plan.Hash(approvedPlan), 0},
		{"stale-digest", plan.Digest(approvedPlan + "Changed.\n"), 1},
		{"stale-short", plan.Hash(approvedPlan + "Changed.\n"), 1},
		{"seven", "1234567", 2},
		{"sixty-three", strings.Repeat("a", 63), 2},
		{"uppercase-short", "ABCDEF", 2},
		{"uppercase-digest", strings.Repeat("A", 64), 2},
		{"nonhex-short", "abcdeg", 2},
		{"nonhex-digest", strings.Repeat("a", 63) + "g", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			fixture := taskFixture(t, h)
			if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
				t.Fatal(err)
			}
			h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
			before := taskTestSnapshot(t, fixture.Path)
			if code := h.run(h.home, "approve", fixture.ID, "--plan-hash", tc.token, "--json"); code != tc.code {
				t.Errorf("exit %d, want %d: %s", code, tc.code, h.errb)
			}
			if tc.code == 0 {
				var event task.Event
				if err := json.Unmarshal(h.out.Bytes(), &event); err != nil || event.PlanHash != plan.Hash(approvedPlan) || event.PlanDigest != plan.Digest(approvedPlan) {
					t.Fatalf("approval receipt: %s (%v)", h.out, err)
				}
				return
			}
			if tc.code == 1 && (!strings.Contains(h.errb.String(), "plan hash no longer matches") || strings.Contains(h.errb.String(), plan.Digest(approvedPlan))) {
				t.Fatalf("stale-plan reply changed: %s", h.errb)
			}
			after := taskTestSnapshot(t, fixture.Path)
			if tc.code == 1 {
				delete(before, string(filepath.Separator)+".plan.lock")
				delete(after, string(filepath.Separator)+".plan.lock")
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("refused token wrote task data")
			}
		})
	}
}

func TestApproveDigestOnlyUsageExit(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	fixture := taskFixture(t, h)
	if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
	o := writeOptions(h.env, fixture.ID, "approve", "")
	// Exercise the option and the CLI result mapper directly.
	o.PlanHashToken = "digest"
	before := taskTestSnapshot(t, fixture.Path)
	event, err := taskWriter(h.env).Approve(o, plan.Hash(approvedPlan))
	if code := taskWriteResult(h.env, event, err, true, fixture.Path); code != 2 {
		t.Fatalf("digest-only short token: exit %d, want 2: %s", code, h.errb)
	}
	if !reflect.DeepEqual(before, taskTestSnapshot(t, fixture.Path)) {
		t.Fatal("digest-only refusal wrote task data")
	}
}
