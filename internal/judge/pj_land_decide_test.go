package judge

import (
	"strings"
	"testing"
)

// Decide raises one agent decision Job from the builder itself, under its own title: a decision
// of another title from the same builder (a lost review's) does not stand in for it, and a repeat
// posts nothing.
func TestJudgeDecideRaisesOneDecisionFromTheBuilder(t *testing.T) {
	st := pjVbCancelled()
	st.Jobs["J-4"].Status = "accepted"
	lost, err := Person(st, "J-4", "J-4.1", "", "", "")
	if err != nil || len(pjDecisions(lost.Posts)) != 1 {
		t.Fatalf("the lost review's decision: %+v %v", lost, err)
	}
	pjVbApply(t, &st, lost.Posts)

	title := "Decide how J-4 continues: its accepted rebase does not land"
	plan, err := Decide(st, "J-4", title, "the land still conflicts")
	if err != nil {
		t.Fatal(err)
	}
	got := pjDecisions(plan.Posts)
	if len(got) != 1 || len(plan.Posts) != 1 || got[0].Body["discovered_from"] != "J-4" || got[0].Body["title"] != title {
		t.Fatalf("posts %+v, want one decision Job from J-4 titled %q", plan.Posts, title)
	}
	if got[0].Job == lost.Posts[0].Job || !strings.HasPrefix(got[0].Job, "J-4.") {
		t.Fatalf("the decision Job is %s, want a new child of J-4 beside %s", got[0].Job, lost.Posts[0].Job)
	}
	goal := str(got[0].Body["intent"].(map[string]any)["goal"])
	for _, want := range []string{"the land still conflicts", "fresh rework", "Cancel J-4", "grid question ask"} {
		if !strings.Contains(goal, want) {
			t.Fatalf("goal %q does not name %q", goal, want)
		}
	}
	pjVbApply(t, &st, plan.Posts)
	if again, err := Decide(st, "J-4", title, "the land still conflicts"); err != nil || len(again.Posts) != 0 {
		t.Fatalf("a repeat posts %+v (%v), want none", again.Posts, err)
	}
}
