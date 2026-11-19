package judge

import (
	"strings"
	"testing"
)

// pjDecisions are the agent decision Job posts among posts: role orchestrator, no person.
func pjDecisions(posts []Post) (out []Post) {
	for _, p := range posts {
		if p.Type == "job" && p.Person == "" && str(p.Body["role"]) == "orchestrator" {
			out = append(out, p)
		}
	}
	return out
}

// from is what the decision Job is discovered from: the review, or the builder when the review never ran.
func pjDecisionCases() map[string]func() (State, string, string, string) {
	return map[string]func() (State, string, string, string){
		"second rejection": func() (State, string, string, string) { return pjVbChain(), "J-2", "J-1.2", "still fails" },
		"stuck": func() (State, string, string, string) {
			st := pjVbState()
			pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
			pjVbJob(&st, "J-1.1", "checker", "submitted", 1, "J-1", "")
			pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": "STUCK\nthe page needs a login"})
			return st, "J-1", "J-1.1", "the page needs a login"
		},
		"no verdict": func() (State, string, string, string) {
			st := pjVbState()
			pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
			pjVbJob(&st, "J-1.1", "reviewer", "submitted", 1, "J-1", "")
			pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": "looks mostly fine, unsure"})
			return st, "J-1", "J-1.1", "looks mostly fine, unsure"
		},
		"cancelled before it ran": func() (State, string, string, string) {
			st := pjVbCancelled()
			st.Jobs["J-4.1"].ExitCode = []byte(`"prepare_failed: no agent"`)
			return st, "J-4", "J-4.1", "prepare_failed: no agent"
		},
	}
}

// Each of the four cases posts one agent decision Job and no person Job; a repeat posts nothing.
func TestJudgeFourCasesPostAnAgentDecisionNotAPersonJob(t *testing.T) {
	for name, mk := range pjDecisionCases() {
		t.Run(name, func(t *testing.T) {
			st, builder, review, text := mk()
			plan, err := Person(st, builder, review, "owner-1", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if got := pjVbPersons(plan.Posts); len(got) != 0 {
				t.Fatalf("person Jobs %+v, want none", got)
			}
			got := pjDecisions(plan.Posts)
			from := review
			if name == "cancelled before it ran" {
				from = builder
			}
			if len(got) != 1 || got[0].Body["discovered_from"] != from {
				t.Fatalf("posts %+v, want one orchestrator decision Job discovered from %s", plan.Posts, from)
			}
			goal := pjReasonText(got[0].Body)
			for _, want := range []string{builder, text, "fresh rework", "Re-plan or split", "Cancel", "grid question ask", "recommended"} {
				if !strings.Contains(goal, want) {
					t.Fatalf("goal %q does not name %q", goal, want)
				}
			}
			pjVbApply(t, &st, plan.Posts)
			if again, _ := Person(st, builder, review, "owner-1", "", ""); len(again.Posts) != 0 {
				t.Fatalf("a repeat posts %+v, want none", again.Posts)
			}
		})
	}
}

// The workflow's point gate stays a person Job.
func TestJudgePointGateStaysAPersonJob(t *testing.T) {
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	plan, err := Person(st, "J-1", "", "owner-1", "", "merge")
	if err != nil {
		t.Fatal(err)
	}
	if got := pjVbPersons(plan.Posts); len(got) != 1 || got[0].Person != "owner-1" || len(pjDecisions(plan.Posts)) != 0 {
		t.Fatalf("posts %+v, want one person Job and no decision Job", plan.Posts)
	}
}

// An open person Job of the old kind is cancelled by the next pass, which posts the decision Job.
func TestJudgeOldOpenPersonJobIsConverted(t *testing.T) {
	st := pjVbChain()
	old := personBody("J-1.3", "J-2", "J-1.2", "Decide how J-2 continues", "g", "owner-1", nil)
	pjVbApply(t, &st, []Post{{Task: "T-1", Type: "job", Job: "J-1.3", Body: old, Person: "owner-1"}})
	st.Jobs["J-1.3"].Role, st.Jobs["J-1.3"].Title, st.Jobs["J-1.3"].Status = "Person", "Decide how J-2 continues", "posted"
	plan, err := Person(st, "J-2", "J-1.2", "owner-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var cancels int
	for _, p := range plan.Posts {
		if p.Type == "control" && p.Body["target"] == "J-1.3" && p.Body["status"] == "cancelled" {
			cancels++
		}
	}
	if len(pjDecisions(plan.Posts)) != 1 || cancels != 1 {
		t.Fatalf("posts %+v, want one decision Job and one cancel of J-1.3", plan.Posts)
	}
}
