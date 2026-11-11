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
	"time"

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
	// ReceiptMoved: the receipt became the result of a new approve person Job. The
	// file stays: it matches that result, and the app reads it.
	ReceiptMoved ReceiptOutcome = "moved"
	// ReceiptCurrent: the task already has an approve person Job, which is the
	// record (and the only one the plugin's readers use); nothing was posted and
	// the file stays beside it as the copy grid approve wrote.
	ReceiptCurrent ReceiptOutcome = "current"
	// ReceiptKept: the receipt never approved a plan (no full plan_digest, a gate
	// name, another action); the file is left as it was.
	ReceiptKept ReceiptOutcome = "kept"
)

type receiptData struct {
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

// readReceipt reads the folder's approval.json once. ok is true when it is a plan
// approval that ReceiptToJob moves or retires; otherwise out and why say what the
// caller returns (ReceiptNone, or ReceiptKept with the reason).
func readReceipt(dir string) (r receiptData, ok bool, out ReceiptOutcome, why string, err error) {
	path := filepath.Join(dir, receiptFile)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, false, ReceiptNone, "", nil
	}
	if err != nil {
		return r, false, "", "", err
	}
	if !info.Mode().IsRegular() {
		return r, false, ReceiptKept, "approval.json is not a regular file", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return r, false, "", "", err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, false, "", "", fmt.Errorf("approval.json: %w", err)
	}
	// The readers accepted only a plan receipt with a full digest and no gate.
	switch {
	case r.What != "approve":
		return r, false, ReceiptKept, "approval.json is not an approve receipt", nil
	case r.Gate != "":
		return r, false, ReceiptKept, "approval.json names the gate " + r.Gate + ", not the plan", nil
	case r.PlanHash == "" || !receiptDigest.MatchString(r.PlanDigest):
		return r, false, ReceiptKept, "approval.json has no full plan_digest, so it never approved a plan", nil
	}
	return r, true, "", "", nil
}

// ReceiptPending reports whether ReceiptToJob has something to do in the folder:
// an approval.json that is a plan approval, in a task that has no approve person
// Job yet. A receipt it would keep, or cannot read, or that already sits beside the
// task's Job (the copy grid approve writes), is not pending, so a caller that
// visits every task does not come back to it on every pass.
func ReceiptPending(dir string) bool {
	_, ok, _, _, err := readReceipt(dir)
	if err != nil || !ok {
		return false
	}
	id, err := feed.TaskID(dir)
	if errors.Is(err, feed.ErrNoTaskID) {
		return true // no Feed identity, so no Job
	}
	if err != nil {
		return false
	}
	has, err := hasApproveJob(feed.Store{TaskDir: dir, TaskID: id})
	return err == nil && !has
}

// ReceiptToJob reads the approval.json of one task folder once and turns it into
// the result of a person Job of kind approve in the task's Feed, then removes the
// file. w is the caller's Feed writer: under the task lease (the worker's pass), or
// a person of the team with no lease (grid approve). The migration holds no lease
// of its own and posts as the person or tool that runs it.
//
// The result carries the plan_hash, plan_digest, lands_hash and note of the
// receipt, so the plan revision the person saw is kept; the receipt's own who and
// when go into the result's reason, because the Feed record names the writer. A
// plan that changed since the receipt is still refused by the readers, exactly as
// before. The file is removed only after the Feed holds the record, so a failed
// post leaves the file for the next run, and a repeat run after a crash between
// the post and the removal retires it. A person with no lease cannot write the
// accepting control: the result is an answer that the holder's next pass copies
// into the control, as for any answer to a person Job.
func ReceiptToJob(dir string, w feed.Writer) (ReceiptOutcome, string, error) {
	receipt, ok, out, why, err := readReceipt(dir)
	if err != nil || !ok {
		return out, why, err
	}
	// A task that is approved gets its Feed here when it has none: this is a writer.
	id, err := feed.EnsureTaskID(dir)
	if err != nil {
		return "", "", err
	}
	store := feed.Store{TaskDir: dir, TaskID: id}
	has, err := hasApproveJob(store)
	if err != nil {
		return "", "", err
	}
	if has {
		// The Job is the record. Every reader already preferred it to the file, and
		// the file stays as the copy grid approve wrote beside it.
		return ReceiptCurrent, "the task has an approve person Job", nil
	}
	jobID, err := PostPlanJob(store, w, receipt.Who.ID)
	if err != nil {
		return "", "", err
	}
	reason := "moved from approval.json: approved by " + receipt.Who.ID
	if receipt.When != "" {
		reason += " at " + receipt.When
	}
	fields := map[string]any{"plan_hash": receipt.PlanHash, "plan_digest": receipt.PlanDigest, "reason": reason}
	if receipt.LandsHash != "" {
		fields["lands_hash"] = receipt.LandsHash
	}
	if receipt.Note != "" {
		fields["note"] = receipt.Note
	}
	if orchestrator.PersonWriter(w) {
		_, err = orchestrator.AnswerPersonWith(store, w, jobID, "approved", fields, time.Now())
	} else {
		body := map[string]any{"target": jobID, "status": "accepted", "answer": "approved"}
		for k, v := range fields {
			body[k] = v
		}
		_, err = orchestrator.Write(store, w, feed.Input{Type: "control", Body: body})
	}
	if err != nil {
		return "", "", err
	}
	// The file is not removed: it matches the Job result just posted, and the readers
	// of the task folder (the app) still read it. ReceiptPending is false from here on,
	// because the task now has its approve person Job.
	return ReceiptMoved, jobID, nil
}

// hasApproveJob reports whether the Feed holds a person Job of kind approve that is
// not cancelled or rejected.
func hasApproveJob(store feed.Store) (bool, error) {
	snap, err := orchestrator.Load(store)
	if err != nil {
		return false, err
	}
	for _, it := range snap.Items {
		j := snap.State.Jobs[it.ID]
		if it.IsPerson() && it.Kind == "approve" && j != nil && j.Status != contract.Cancelled && j.Status != contract.Rejected {
			return true, nil
		}
	}
	return false, nil
}

// PostPlanJob posts a new person Job of kind approve for the plan, with the next
// free Job number, and returns its id. A person of the team with no lease posts it
// unleased; every other writer posts it under the task lease. owner is the person
// the Job is for.
//
// The number is one above the highest J-<n> of any `job` record on the Feed,
// whatever the fold made of it (counted, held, ignored or bad), and of the Jobs
// the snapshot counts: a held record's id is still its own, and the fold keeps
// the first record of an id in its order, so a held one that resolves later could
// take the id of this Job. The Feed is read again under .orchestrator.lock,
// which the post holds, and a writer that took the id in between makes this one
// pick again; each round sees one more taken id, so the loop ends.
func PostPlanJob(store feed.Store, w feed.Writer, owner string) (string, error) {
	for {
		id, err := nextPlanJobID(store)
		if err != nil {
			return "", err
		}
		in := feed.Input{Type: "job", Job: id, Body: PlanJobBody(id, owner)}
		var checkErr error
		taken := func() bool {
			held, err := planJobIDTaken(store, id)
			if err != nil {
				// done cannot return an error: stop the post and report it below.
				checkErr = err
				return true
			}
			return held
		}
		skipped := false
		if orchestrator.PersonWriter(w) {
			_, skipped, err = orchestrator.PostPersonUnless(store, w, in, taken)
		} else {
			_, err = orchestrator.WriteUnless(store, w, in, func() bool {
				skipped = taken()
				return skipped
			})
		}
		if checkErr != nil {
			return "", checkErr
		}
		if err != nil || !skipped {
			return id, err
		}
	}
}

// jobNumber is the J-<n> number of a Job id; a dotted child id counts by the
// part before the dot. ok is false for an id that is not of that form.
func jobNumber(id string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(id, ".", 2)[0], "J-"))
	return n, err == nil
}

// planJobIDs is the body id of every `job` entry of the Feed, whatever its
// verdict.
func planJobIDs(store feed.Store) ([]string, error) {
	read, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, e := range read.Records {
		if e.Type != "job" {
			continue
		}
		if id, ok := e.Body["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// nextPlanJobID is one above the highest J-<n> of the Feed's `job` records and
// of the Jobs the snapshot counts.
func nextPlanJobID(store feed.Store) (string, error) {
	snap, err := orchestrator.Load(store)
	if err != nil {
		return "", err
	}
	ids, err := planJobIDs(store)
	if err != nil {
		return "", err
	}
	for _, it := range snap.Items {
		ids = append(ids, it.ID)
	}
	highest := 0
	for _, id := range ids {
		if n, ok := jobNumber(id); ok && n > highest {
			highest = n
		}
	}
	return "J-" + strconv.Itoa(highest+1), nil
}

// planJobIDTaken reports whether a `job` entry of the Feed, with any verdict,
// carries id.
func planJobIDTaken(store feed.Store, id string) (bool, error) {
	ids, err := planJobIDs(store)
	if err != nil {
		return false, err
	}
	for _, have := range ids {
		if have == id {
			return true, nil
		}
	}
	return false, nil
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
