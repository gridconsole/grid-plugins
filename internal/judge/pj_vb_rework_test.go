package judge

import (
	"regexp"
	"strings"
	"testing"
)

// Ported from the app's judge_rework_newjob_test.go:28: the rework of a rejected Job is posted under
// a Job id other than the rejected one and other than any id the Feed holds, and its body holds only
// keys the job record allows, read here from contract/schemas/job-record.json itself.
func TestReworkPostsANewJobIDWithOnlyJobRecordKeys(t *testing.T) {
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	st.Bodies["J-1"] = pjVbFixture(t, "J-1")
	st.Bodies["J-1"]["status"], st.Bodies["J-1"]["created_at"], st.Bodies["J-1"]["order"] = "submitted", "2026-11-01T10:00:00.000Z", 3
	pjVbJob(&st, "J-1.1", "reviewer", "submitted", 1, "J-1", "")
	pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": "REJECTED", "must_fix": []any{map[string]any{"row": "A1", "at": "main.go:4"}}})
	pjVbJob(&st, "J-2", "builder", "posted", 1, "", "")

	plan, err := Rework(st, "J-1", "J-1.1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var rework *Post
	for i, p := range plan.Posts {
		if p.Type == "job" {
			if rework != nil {
				t.Fatalf("posts %+v: want one rework job post", plan.Posts)
			}
			rework = &plan.Posts[i]
		}
	}
	if rework == nil {
		t.Fatalf("posts %+v: want a rework job post", plan.Posts)
	}
	id := rework.Job
	if id == "" || id == "J-1" {
		t.Fatalf("the rework is posted as %q: want a Job id other than the rejected J-1", id)
	}
	if id == "J-2" {
		t.Fatalf("the rework took J-2, a Job id the Feed already holds")
	}
	recordKeys, rowKeys := pjVbSchemaKeys(t)
	body := rework.Body
	for k := range body {
		if !recordKeys[k] {
			t.Errorf("the rework body has key %q, which job-record.json does not allow", k)
		}
	}
	for _, k := range []string{"status", "created_at", "order"} {
		if _, ok := body[k]; ok {
			t.Errorf("the rework body keeps %q of the reworked Job", k)
		}
	}
	if body["id"] != id || body["continues"] != "J-1" || body["discovered_from"] != "J-1.1" {
		t.Fatalf("the rework body names id %v continues %v discovered_from %v: want %s continuing J-1 from J-1.1", body["id"], body["continues"], body["discovered_from"], id)
	}
	rows, _ := body["acceptance"].([]any)
	if len(rows) == 0 {
		t.Fatalf("the rework body has no acceptance rows")
	}
	row, _ := rows[len(rows)-1].(map[string]any)
	for k := range row {
		if !rowKeys[k] {
			t.Errorf("the must-fix row has key %q, which job-record.json does not allow", k)
		}
	}
	if !regexp.MustCompile(`^A[1-9][0-9]*$`).MatchString(str(row["id"])) || str(row["text"]) == "" || !regexp.MustCompile(`^(item|owner|decisions/.+|brain:.+)$`).MatchString(str(row["traces"])) {
		t.Errorf("the must-fix row is %v: want id A<n>, a text and traces as the schema reads them", row)
	}
	if !strings.Contains(str(row["text"]), "Must-fix from review round 1:\n- A1: main.go:4") {
		t.Errorf("the must-fix row text is %q", row["text"])
	}
	last := plan.Posts[len(plan.Posts)-1]
	if last.Type != "control" || last.Body["target"] != "J-1" || last.Body["status"] != "rejected" || last.Body["close_reason"] != "reworked as "+id {
		t.Fatalf("the last post is %+v, want the control that rejects J-1 as reworked as %s", last, id)
	}
	// A second call after the rework Job is on the Feed posts no second Job, only the control again.
	pjVbApply(t, &st, plan.Posts[:1])
	again, err := Rework(st, "J-1", "J-1.1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Posts) != 1 || again.Posts[0].Type != "control" || again.Posts[0].ID() != last.ID() {
		t.Fatalf("a repeat posts %+v, want only the same control", again.Posts)
	}
}

// Row 8 (V27): the review Job's base is the submitted SHA of the reviewed attempt; a rework Job's
// base is the task head's SHA, or the target's tip when the task has no head.
func TestPjVbBaseIsTheSubmittedShaThenTheHeadOrTheTarget(t *testing.T) {
	const old, cur, head = "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222", "3333333333333333333333333333333333333333"
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 2, "", "")
	st.Bodies["J-1"]["base"] = "main"
	pjVbRecord(&st, "S1", "submit", "J-1", 1, map[string]any{"sha": old})
	pjVbRecord(&st, "S2", "submit", "J-1", 2, map[string]any{"sha": cur})
	pjVbRecord(&st, "S3", "submit", "J-1", 1, map[string]any{"sha": old})

	review, err := Review(st, "J-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Posts) != 1 || review.Posts[0].Body["base"] != cur {
		t.Fatalf("review posts %+v, want one Job based on the attempt-2 submit %s", review.Posts, cur)
	}
	pjVbApply(t, &st, review.Posts)
	pjVbRecord(&st, "V1", "result", "J-1.1", 1, map[string]any{"outcome": "REJECTED\nMust-fix:\n- row 1"})

	plan, err := Rework(st, "J-1", "J-1.1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Posts[0].Body["base"]; got != "main" {
		t.Fatalf("rework base %v with no head, want the target main", got)
	}
	if plan, err = Rework(st, "J-1", "J-1.1", "", "release"); err != nil || plan.Posts[0].Body["base"] != "release" {
		t.Fatalf("rework base %v (%v) with target release, want release", plan.Posts[0].Body["base"], err)
	}
	pjVbRecord(&st, "H1", "head", "", 0, map[string]any{"sha": head, "ref": "refs/grid/T-1/head"})
	st.Head = "H1"
	if plan, err = Rework(st, "J-1", "J-1.1", "", "release"); err != nil || plan.Posts[0].Body["base"] != head {
		t.Fatalf("rework base %v (%v) with a task head, want the head %s", plan.Posts[0].Body["base"], err, head)
	}
}
