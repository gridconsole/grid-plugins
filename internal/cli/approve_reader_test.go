package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/project"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// landJobHarness is approveJobHarness (task T-07 with an open person Job J-2 of
// an approve step) in a project with a deploy rule, so grid land can run.
func landJobHarness(t *testing.T) (*harness, string, string, string) {
	t.Helper()
	h, path, _, text := approveJobHarness(t)
	repo := gridhome.Real(t.TempDir())
	if err := project.Write(h.gridHome, project.Record{Name: "example", ID: "example", Folders: []project.Folder{{Machine: feedMachine, Path: repo}}}); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(h.gridHome, "projects", "example", "knowledge", "deploy", "lands.md"), "---\nlands:\n  target: local\n  branch: main\n  command: printf landed > outcome\n  needs-approve: true\n---\n")
	return h, path, repo, text
}

func landed(t *testing.T, repo string) bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "outcome"))
	return err == nil && string(b) == "landed"
}

func attachApproved(t *testing.T, h *harness) bool {
	t.Helper()
	h.out.Reset()
	if code := h.run(h.home, "task", "attach", "T-07", "--session", "terminal-example", "--json"); code != 0 {
		t.Fatalf("attach: %d %s", code, h.errb.String())
	}
	var got struct {
		Approved bool `json:"approved"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, h.out.String())
	}
	return got.Approved
}

// Land refuses a task whose plan Job has no accepted result, with land's
// rejection (exit 3) and its "plan has not been approved" message, and lands the
// task once the Job result matches plan.md.
func TestLandReadsThePlanJobResult(t *testing.T) {
	h, path, repo, text := landJobHarness(t)
	if code := h.run(repo, "land", "T-07"); code != 3 || !strings.Contains(h.errb.String(), "plan has not been approved") || !strings.Contains(h.errb.String(), "plan Job J-2 has no result yet") {
		t.Fatalf("land before approval: %d %s", code, h.errb.String())
	}
	if landed(t, repo) {
		t.Fatal("land ran without an accepted Job result")
	}
	h.errb.Reset()
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb.String())
	}
	h.errb.Reset()
	if code := h.run(repo, "land", "T-07"); code != 0 || !landed(t, repo) {
		t.Fatalf("land after approval: %d %s", code, h.errb.String())
	}
}

// The Job result is the only answer: with the approval.json receipt gone, land
// still lands on the Job result; the gate name does not matter to the reader.
func TestJobResultAnswersWithoutApprovalJSONAndForAnyGateName(t *testing.T) {
	h, path, _, text := landJobHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb.String())
	}
	if err := os.Remove(filepath.Join(path, "approval.json")); err != nil {
		t.Fatal(err)
	}
	if !taskstore.Approved(path) {
		t.Fatal("Approved ignored the Job result")
	}
	s := taskstore.Store{Resolver: resolver(h.env)}
	for _, gate := range []string{"plan", "ship", "preview"} {
		if e, err := s.ApprovalForGate(path, "T-07", gate); err != nil || e.PlanHash != plan.Hash(text) || e.PlanDigest != plan.Digest(text) || e.Who.ID == "" || e.When == "" {
			t.Fatalf("gate %s: %+v %v", gate, e, err)
		}
	}
}

// plan.md changes after the Job result: Approved, attach approved and land all
// say not approved, with "Changed since you approved".
func TestPlanChangeAfterJobResultIsNotApproved(t *testing.T) {
	h, path, repo, text := landJobHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb.String())
	}
	if !taskstore.Approved(path) || !attachApproved(t, h) {
		t.Fatal("not approved right after the Job result")
	}
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(text+"\nOne more step.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if taskstore.Approved(path) {
		t.Fatal("Approved after plan.md changed")
	}
	if attachApproved(t, h) {
		t.Fatal("attach approved after plan.md changed")
	}
	h.errb.Reset()
	if code := h.run(repo, "land", "T-07"); code == 0 || !strings.Contains(h.errb.String(), "Changed since you approved") || landed(t, repo) {
		t.Fatalf("land after plan.md changed: %d %s", code, h.errb.String())
	}
}

// After plan.md changes the old result is stale. Approving the new plan must
// post a new result on the plan Job; approval.json at the new hash alone does
// not approve it, because the Job result is the answer.
func TestReapprovalAfterPlanChangePostsANewResult(t *testing.T) {
	h, path, _, text := landJobHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb.String())
	}
	changed := text + "\nOne more step.\n"
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	// No Feed writer: approval.json moves to the new hash and no result is posted.
	t.Setenv("GRID_MEMBER", "")
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(changed)); code != 0 {
		t.Fatalf("approve without a writer: %d %s", code, h.errb.String())
	}
	if taskstore.Approved(path) {
		t.Fatal("approved from a new approval.json with an old Job result")
	}
	if _, err := taskstore.ApprovalFromJob(path); err == nil || !strings.Contains(err.Error(), "Changed since you approved") {
		t.Fatalf("reader message: %v", err)
	}
	t.Setenv("GRID_MEMBER", "member")
	h.errb.Reset()
	if code := h.run(path, "approve", path); code != 0 {
		t.Fatalf("repeat approve: %d %s", code, h.errb.String())
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.PlanHash != plan.Hash(changed) || a.PlanDigest != plan.Digest(changed) || !taskstore.Approved(path) {
		t.Fatalf("%+v %v", a, err)
	}
}

// approval.json is written but the Job has no result (the post failed): readers
// say not approved and name the plan Job, and a repeat grid approve posts the
// missing result.
func TestApprovalJSONWithoutJobResultIsRecoverable(t *testing.T) {
	h, path, repo, text := landJobHarness(t)
	t.Setenv("GRID_MEMBER", "")
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 || !strings.Contains(h.errb.String(), "J-2 stays open") {
		t.Fatalf("approve without a writer: %d %s", code, h.errb.String())
	}
	if _, err := os.Stat(filepath.Join(path, "approval.json")); err != nil {
		t.Fatal(err)
	}
	if taskstore.Approved(path) || attachApproved(t, h) {
		t.Fatal("approved from approval.json alone, with a plan Job that has no result")
	}
	if _, err := taskstore.ApprovalFromJob(path); err == nil || !strings.Contains(err.Error(), "plan Job J-2 has no result yet") {
		t.Fatalf("reader message: %v", err)
	}
	h.errb.Reset()
	if code := h.run(repo, "land", "T-07"); code != 3 || !strings.Contains(h.errb.String(), "plan Job J-2 has no result yet") || landed(t, repo) {
		t.Fatalf("land: %d %s", code, h.errb.String())
	}
	// The repeat, with a writer, posts the result; approval.json already matches.
	t.Setenv("GRID_MEMBER", "member")
	h.errb.Reset()
	if code := h.run(path, "approve", path); code != 0 {
		t.Fatalf("repeat approve: %d %s", code, h.errb.String())
	}
	if !taskstore.Approved(path) || !attachApproved(t, h) {
		t.Fatal("not approved after the repeat grid approve")
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-2" || a.PlanDigest != plan.Digest(text) {
		t.Fatalf("%+v %v", a, err)
	}
}
