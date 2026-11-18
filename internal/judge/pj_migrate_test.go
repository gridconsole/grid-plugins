package judge

import (
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
