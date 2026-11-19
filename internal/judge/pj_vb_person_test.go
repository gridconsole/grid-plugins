package judge

import (
	"encoding/json"
	"strings"
	"testing"
)

// pjVbCancelled is the state of the app's exitRig: J-4 submitted, its review J-4.1 cancelled.
func pjVbCancelled() State {
	st := pjVbState()
	pjVbJob(&st, "J-4", "builder", "submitted", 1, "", "")
	pjVbJob(&st, "J-4.1", "reviewer", "cancelled", 1, "J-4", "")
	return st
}

// pjVbPersons are the person Job posts among posts.
func pjVbPersons(posts []Post) (out []Post) {
	for _, p := range posts {
		if p.Type == "job" && p.Person != "" {
			out = append(out, p)
		}
	}
	return out
}

// Ported from the app's judge_cancelled_review_test.go:66: a review cancelled at spawn leaves its
// builder with no verdict; three calls (two wakes and none) raise exactly one decision Job for an
// agent (no person Job), discovered from the builder, naming the reason and the choices. Nothing accepts J-4.
func TestJudgeReviewCancelledAtSpawnRaisesOneDecisionJob(t *testing.T) {
	st := pjVbCancelled()
	st.Jobs["J-4.1"].ExitCode = json.RawMessage(`"prepare_failed: the agent could not be built"`)
	var all []Post
	for _, wake := range []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV/1", "01ARZ3NDEKTSV4RRFFQ69G5FAW/1", ""} {
		plan, err := Person(st, "J-4", "J-4.1", "owner-1", wake, "")
		if err != nil {
			t.Fatal(err)
		}
		pjVbApply(t, &st, plan.Posts)
		all = append(all, plan.Posts...)
	}
	got := pjDecisions(all)
	if len(got) != 1 || len(pjVbPersons(all)) != 0 {
		t.Fatalf("%d decision Jobs after three calls (posts %+v), want exactly one and no person Job", len(got), all)
	}
	p := got[0]
	goal := str(p.Body["intent"].(map[string]any)["goal"])
	if p.Body["discovered_from"] != "J-4" || p.Body["role"] != "orchestrator" {
		t.Fatalf("decision Job %+v, want an orchestrator Job discovered from J-4", p)
	}
	for _, want := range []string{"J-4", "J-4.1", "prepare_failed: the agent could not be built", "Accept J-4", "fresh rework"} {
		if !strings.Contains(goal, want) {
			t.Fatalf("goal %q does not name %q", goal, want)
		}
	}
	if st.Jobs["J-4"].Status != "submitted" {
		t.Fatalf("J-4 is %s, want submitted: nothing accepts it without the person", st.Jobs["J-4"].Status)
	}
	for _, q := range all {
		if q.Body["role"] == "reviewer" || q.Body["status"] == "accepted" {
			t.Fatalf("post %+v: no second review and no accept were wanted", q)
		}
	}
}

// Ported from the app's judge_cancelled_review_test.go:101: a review cancelled after it gave its
// verdict raises no person Job, whether the call names the review or only the builder.
func TestJudgeReviewCancelledAfterAVerdictRaisesNoPersonJob(t *testing.T) {
	st := pjVbCancelled()
	pjVbRecord(&st, "R1", "result", "J-4.1", 1, map[string]any{"verdict": "REJECTED"})
	for _, from := range []string{"J-4.1", ""} {
		plan, err := Person(st, "J-4", from, "owner-1", "01ARZ3NDEKTSV4RRFFQ69G5FAV/1", "")
		if err != nil {
			t.Fatal(err)
		}
		if got := pjVbPersons(plan.Posts); len(got) != 0 {
			t.Fatalf("person Jobs %+v for a review that gave its verdict, want none", got)
		}
		if plan.Note == "" {
			t.Fatalf("no post and no note: say why nothing was posted")
		}
	}
}

// The decision Job after a second rejection carries the reviewer's text and comes from the review;
// STUCK raises one with the reviewer's reason; each is raised once.
func TestPjVbDecisionAfterASecondRejectionAndStuck(t *testing.T) {
	st := pjVbChain()
	plan, err := Person(st, "J-2", "J-1.2", "owner-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	got := pjDecisions(plan.Posts)
	if len(got) != 1 || got[0].Job != "J-1.3" || got[0].Body["discovered_from"] != "J-1.2" {
		t.Fatalf("posts %+v, want the decision Job J-1.3 discovered from J-1.2", plan.Posts)
	}
	if goal := str(got[0].Body["intent"].(map[string]any)["goal"]); !strings.Contains(goal, "rejected it again after its rework") || !strings.Contains(goal, "still fails") {
		t.Fatalf("goal %q, want the second rejection and the reviewer's text", goal)
	}
	pjVbApply(t, &st, plan.Posts)
	if again, _ := Person(st, "J-2", "J-1.2", "owner-1", "", ""); len(again.Posts) != 0 {
		t.Fatalf("a repeat posts %+v, want none", again.Posts)
	}

	st = pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	pjVbJob(&st, "J-1.1", "checker", "submitted", 1, "J-1", "")
	pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": "STUCK\nthe page needs a login"})
	plan, err = Person(st, "J-1", "J-1.1", "owner-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := pjDecisions(plan.Posts); len(got) != 1 || !strings.Contains(str(got[0].Body["intent"].(map[string]any)["goal"]), "the page needs a login") {
		t.Fatalf("posts %+v, want one decision Job carrying the reviewer's reason", plan.Posts)
	}
}
