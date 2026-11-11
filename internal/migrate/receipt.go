package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// receiptFile is the unsigned approval receipt an older plugin wrote beside the
// plan. The approve person Job's result is the approval record; this file is a
// second record that ReceiptToJob moves into the Feed once and then retires.
const receiptFile = "approval.json"

var receiptDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ReceiptOutcome says what ReceiptToJob did with the task folder.
type ReceiptOutcome string

const (
	// ReceiptNone: the folder has no approval.json; nothing was read or written.
	ReceiptNone ReceiptOutcome = "none"
	// ReceiptMoved: the receipt became the result of a new approve person Job, and
	// the file was removed.
	ReceiptMoved ReceiptOutcome = "moved"
	// ReceiptRetired: the task already has an approve person Job, which is the
	// record (and was already the only one the readers used); the file was
	// removed and nothing was posted.
	ReceiptRetired ReceiptOutcome = "retired"
	// ReceiptKept: the receipt never approved a plan (no full plan_digest, a gate
	// name, another action); the file is left as it was.
	ReceiptKept ReceiptOutcome = "kept"
)

// ReceiptToJob reads the approval.json of one task folder once and turns it into
// the result of a person Job of kind approve in the task's Feed, then removes the
// file. w is the caller's Feed writer under the task lease: the migration holds no
// lease of its own and posts as the person or tool that runs it.
//
// The result carries the plan_hash, plan_digest, lands_hash and note of the
// receipt, so the plan revision the person saw is kept; the receipt's own who and
// when go into the result's reason, because the Feed record names the writer. A
// plan that changed since the receipt is still refused by the readers, exactly as
// before. The file is removed only after
// the Feed holds the record, so a failed post leaves the file for the next run,
// and a repeat run after a crash between the post and the removal retires it.
func ReceiptToJob(dir string, w feed.Writer) (ReceiptOutcome, string, error) {
	path := filepath.Join(dir, receiptFile)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ReceiptNone, "", nil
	}
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		return ReceiptKept, "approval.json is not a regular file", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	var receipt struct {
		Task       string          `json:"task"`
		Who        contract.Author `json:"who"`
		When       string          `json:"when"`
		What       string          `json:"what"`
		Note       string          `json:"note"`
		LandsHash  string          `json:"lands_hash"`
		PlanHash   string          `json:"plan_hash"`
		PlanDigest string          `json:"plan_digest"`
		Gate       string          `json:"gate"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		return "", "", fmt.Errorf("approval.json: %w", err)
	}
	// The readers accepted only a plan receipt with a full digest and no gate.
	switch {
	case receipt.What != "approve":
		return ReceiptKept, "approval.json is not an approve receipt", nil
	case receipt.Gate != "":
		return ReceiptKept, "approval.json names the gate " + receipt.Gate + ", not the plan", nil
	case receipt.PlanHash == "" || !receiptDigest.MatchString(receipt.PlanDigest):
		return ReceiptKept, "approval.json has no full plan_digest, so it never approved a plan", nil
	}
	// A caller that holds a task lease writer already has the task's id.
	id, err := feed.TaskID(dir)
	if err != nil {
		return "", "", err
	}
	store := feed.Store{TaskDir: dir, TaskID: id}
	snap, err := orchestrator.Load(store)
	if err != nil {
		return "", "", err
	}
	highest, planned := 0, false
	for _, it := range snap.Items {
		if n, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(it.ID, ".", 2)[0], "J-")); err == nil && n > highest {
			highest = n
		}
		j := snap.State.Jobs[it.ID]
		if it.IsPerson() && it.Kind == "approve" && j != nil && j.Status != contract.Cancelled && j.Status != contract.Rejected {
			planned = true
		}
	}
	if planned {
		// The Job is the record. Every reader already preferred it to the file.
		return ReceiptRetired, "the task has an approve person Job", os.Remove(path)
	}
	jobID := "J-" + strconv.Itoa(highest+1)
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: jobID, Body: PlanJobBody(jobID, receipt.Who.ID)}); err != nil {
		return "", "", err
	}
	reason := "moved from approval.json: approved by " + receipt.Who.ID
	if receipt.When != "" {
		reason += " at " + receipt.When
	}
	body := map[string]any{"target": jobID, "status": "accepted", "reason": reason, "answer": "approved", "plan_hash": receipt.PlanHash, "plan_digest": receipt.PlanDigest}
	if receipt.LandsHash != "" {
		body["lands_hash"] = receipt.LandsHash
	}
	if receipt.Note != "" {
		body["note"] = receipt.Note
	}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "control", Body: body}); err != nil {
		return "", "", err
	}
	return ReceiptMoved, jobID, os.Remove(path)
}

// PlanJobBody is the record of a person Job of kind approve for the plan: the
// step a person answers with the plan revision they saw. owner is the person.
func PlanJobBody(id, owner string) map[string]any {
	if owner == "" {
		owner = "owner"
	}
	return map[string]any{
		"id": id, "kind": "approve", "item": 1, "title": "Approve the plan", "role": job.RolePerson,
		"skills": []string{}, "review": "L1", "risk": []string{}, "max_rounds": 2,
		"intent":       map[string]any{"owner": owner, "item": "plan.md#approve", "goal": "plan.md#approve"},
		"links":        []any{},
		"keep":         []string{},
		"decisions":    []string{},
		"read":         []string{"plan.md"},
		"write":        []string{},
		"out_of_scope": []string{},
		"acceptance":   []any{map[string]any{"id": "A1", "text": "The plan revision is approved", "traces": "item"}},
		"verify":       map[string]any{"command": "grid plan hash", "min_tests": 1},
		"budget":       map[string]any{"turns_estimate": 1, "reads_before_first_edit": 1},
		"on_blocked":   "ask-and-continue", "after": []any{}, "attempt": 1,
		"continues": nil, "discovered_from": nil, "close_reason": nil, "checkpoint": nil,
		"review_rounds": 0, "level_raised": nil, "worktree": nil,
		"posts_back": []string{"result"},
	}
}
