package judge

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Migrate turns an old "Decide how" person Job on a stuck review into one decision Job plus the
// cancel of the person Job, leaves a point gate alone, and has nothing to do once it has run.
func TestMigrateHandsAnOldPersonJobToAnAgent(t *testing.T) {
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	pjVbJob(&st, "J-1.1", "checker", "submitted", 1, "J-1", "")
	pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": "STUCK\nthe page needs a login"})
	for id, title := range map[string]string{"J-1.2": "Decide how J-1 continues", "J-5": "Judge J-1 before ship"} {
		pjVbJob(&st, id, "Person", "posted", 1, "J-1.1", "")
		st.Bodies[id]["title"] = title
		st.Bodies[id]["acceptance"] = []any{map[string]any{"id": "A1", "text": "dana decides how J-1 continues", "traces": "owner"}}
	}
	plan, ok := Migrate(st)
	if !ok || len(pjDecisions(plan.Posts)) != 1 || len(plan.Posts) != 2 || plan.Posts[1].Job != "J-1.2" {
		t.Fatalf("plan %+v ok=%v, want the decision Job and the cancel of J-1.2", plan.Posts, ok)
	}
	pjVbApply(t, &st, plan.Posts)
	if st.Jobs["J-1.2"].Status != contract.Cancelled || st.Jobs["J-5"].Status == contract.Cancelled {
		t.Fatalf("J-1.2 %s, J-5 %s", st.Jobs["J-1.2"].Status, st.Jobs["J-5"].Status)
	}
	if again, ok := Migrate(st); ok {
		t.Fatalf("a second Migrate posts %+v, want nothing", again.Posts)
	}
}

// pjMigrateOld is a builder J-1, its review J-1.1 (status, optional verdict result) and the old
// "Decide how" person Job J-1.2 discovered from the review.
func pjMigrateOld(status contract.Status, outcome string) State {
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "submitted", 1, "", "")
	pjVbJob(&st, "J-1.1", "checker", status, 1, "J-1", "")
	if outcome != "" {
		pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": outcome})
	}
	pjVbJob(&st, "J-1.2", "Person", "posted", 1, "J-1.1", "")
	st.Bodies["J-1.2"]["title"] = "Decide how J-1 continues"
	st.Bodies["J-1.2"]["acceptance"] = []any{map[string]any{"id": "A1", "text": "dana decides how J-1 continues", "traces": "owner"}}
	return st
}

// Every path where Person posts nothing still closes the old card: one cancel naming why, no new
// decision Job, and a second Migrate has nothing left.
func TestMigrateCancelsAnOldPersonJobWhenNothingIsPosted(t *testing.T) {
	cases := map[string]struct {
		st   func() State
		want string
	}{
		"accepted":                      {func() State { return pjMigrateOld("submitted", "ACCEPTED\nfine") }, "accepted"},
		"review cancelled after it ran": {func() State { return pjMigrateOld("cancelled", "REJECTED\nstill wrong") }, "gave its result before it was cancelled"},
		"decision already posted": {func() State {
			st := pjMigrateOld("submitted", "STUCK\nneeds a login")
			pjVbJob(&st, "J-1.3", "orchestrator", "posted", 1, "J-1.1", "")
			st.Bodies["J-1.3"]["title"] = "Decide how J-1 continues"
			return st
		}, "J-1.3"},
		"review with no builder": {func() State {
			st := pjMigrateOld("submitted", "STUCK\nneeds a login")
			st.Jobs["J-1.1"].DiscoveredFrom, st.Bodies["J-1.1"]["discovered_from"] = nil, nil
			return st
		}, "no builder"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			st := c.st()
			plan, ok := Migrate(st)
			if !ok || len(plan.Posts) != 1 || plan.Posts[0].Type != "control" || plan.Posts[0].Job != "J-1.2" {
				t.Fatalf("plan %+v ok=%v, want one cancel of J-1.2", plan.Posts, ok)
			}
			if reason := str(plan.Posts[0].Body["reason"]); !strings.Contains(reason, c.want) {
				t.Fatalf("reason %q does not name %q", reason, c.want)
			}
			pjVbApply(t, &st, plan.Posts)
			if st.Jobs["J-1.2"].Status != contract.Cancelled {
				t.Fatalf("J-1.2 is %s, want cancelled", st.Jobs["J-1.2"].Status)
			}
			if again, ok := Migrate(st); ok {
				t.Fatalf("a second Migrate posts %+v, want nothing", again.Posts)
			}
		})
	}
}
