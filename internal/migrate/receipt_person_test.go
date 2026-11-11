package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

func personWriter() feed.Writer {
	return feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: "dana@" + receiptMachine}
}

// grid approve runs as a person of the team with no lease, while a worker may hold
// the task lease. The move posts the Job unleased and an answer that carries the
// receipt's fields; the holder's next pass turns the answer into the accepted result.
func TestReceiptToJobWithAPersonWriterPostsAnAnswerTheHoldersPassAccepts(t *testing.T) {
	dir, holder := receiptFolder(t, "")
	if err := os.WriteFile(filepath.Join(dir, "approval.json"), []byte(receiptJSON(t, nil)), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome, job, err := ReceiptToJob(dir, personWriter())
	if err != nil || outcome != ReceiptMoved || job != "J-1" {
		t.Fatalf("%q %q %v", outcome, job, err)
	}
	if gone(t, filepath.Join(dir, "approval.json")) {
		t.Fatal("approval.json was removed")
	}
	store := feed.Store{TaskDir: dir, TaskID: receiptTaskID}
	jobs, err := store.Read(feed.Filter{Type: "job"})
	if err != nil || len(jobs.Records) != 1 || jobs.Records[0].Lease != nil {
		t.Fatalf("the Job must be posted without a lease: %+v %v", jobs.Records, err)
	}
	answers, err := store.Read(feed.Filter{Type: "answer"})
	if err != nil || len(answers.Records) != 1 {
		t.Fatalf("answers: %+v %v", answers, err)
	}
	body := answers.Records[0].Body
	if body["plan_hash"] != plan.Hash(receiptPlan) || body["plan_digest"] != plan.Digest(receiptPlan) || body["lands_hash"] != "ab12" || body["note"] != "looks right" || body["answer"] != "approved" {
		t.Fatalf("answer body %v", body)
	}
	if _, err := taskstore.ApprovalFromJob(dir); err == nil {
		t.Fatal("approved before the holder's pass")
	}
	if _, err := (orchestrator.Store{Feed: store, Writer: holder, Owner: "owner", Assignee: holder.Worker}).Tick(); err != nil {
		t.Fatal(err)
	}
	a, err := taskstore.ApprovalFromJob(dir)
	if err != nil || a.Job != "J-1" || a.PlanHash != plan.Hash(receiptPlan) || a.PlanDigest != plan.Digest(receiptPlan) || a.LandsHash != "ab12" || a.Note != "looks right" {
		t.Fatalf("%+v %v", a, err)
	}
}

// PostPlanJob numbers the new Job after the highest Job of the Feed, for either writer.
func TestPostPlanJobNumbersAfterTheHighestJob(t *testing.T) {
	dir, w := receiptFolder(t, "")
	store := feed.Store{TaskDir: dir, TaskID: receiptTaskID}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: "J-4", Body: PlanJobBody("J-4", "dana")}); err != nil {
		t.Fatal(err)
	}
	if id, err := PostPlanJob(store, w, "dana"); err != nil || id != "J-5" {
		t.Fatalf("leased: %q %v", id, err)
	}
	if id, err := PostPlanJob(store, personWriter(), "dana"); err != nil || id != "J-6" {
		t.Fatalf("person with no lease: %q %v", id, err)
	}
	st, err := store.State()
	if err != nil || st.Jobs["J-5"] == nil || st.Jobs["J-6"] == nil {
		t.Fatalf("Jobs after the posts: %+v %v", st.Jobs, err)
	}
	if _, err := PostPlanJob(store, feed.Writer{Author: feed.Author{ID: "x", Role: "worker"}, Worker: "x@" + receiptMachine}, "dana"); err == nil {
		t.Fatal("a writer with no lease and no person role posted a Job")
	}
}

// ReceiptPending is true only for a receipt ReceiptToJob would move or retire.
func TestReceiptPendingOnlyForAPlanReceipt(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		want bool
	}{
		"no file":      {"", false},
		"plan receipt": {receiptJSON(t, nil), true},
		"gate receipt": {receiptJSON(t, map[string]any{"gate": "ship"}), false},
		"no digest":    {receiptJSON(t, map[string]any{"plan_digest": nil}), false},
		"not approve":  {receiptJSON(t, map[string]any{"what": "reopen"}), false},
		"corrupt json": {"{not json", false},
	} {
		t.Run(name, func(t *testing.T) {
			dir, _ := receiptFolder(t, tc.text)
			if got := ReceiptPending(dir); got != tc.want {
				t.Fatalf("ReceiptPending = %v, want %v", got, tc.want)
			}
		})
	}
}
