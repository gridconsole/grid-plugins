package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/question"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// The role refusal names the roles that may run the verb and the reason word.
func TestRoleRefusalNamesAllowedRoles(t *testing.T) {
	h, dir := feedHarness(t)
	q := questionAdd(t, h, dir, "visitor")
	h.write(filepath.Join(dir, "workflow.md"), "---\nperson_verbs: {answer: [member, lead]}\n---\n")
	t.Setenv("GRID_ROLE", "guest")
	t.Setenv("GRID_ACTOR", "visitor")
	t.Setenv("GRID_MEMBER", "visitor")
	if code := h.run(dir, "answer", dir, q.ID, "Yes", "--json"); code != 3 {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	got := h.out.String() + h.errb.String()
	for _, want := range []string{"allowed: member, lead", "role_not_allowed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("refusal lacks %q: %s", want, got)
		}
	}
	if strings.Contains(got, "agent_env") || strings.Contains(got, "outside Claude Code") {
		t.Fatalf("refusal still names the shell guard: %s", got)
	}
}

// guestWriteVerb's early refusals print the stores' own words.
func TestGuestWriteRefusalMatchesStores(t *testing.T) {
	if got := question.WithdrawRefusalText("Q-1", "guest"); got != `Q-1 role "guest" may not withdraw` {
		t.Fatalf("question store text %q", got)
	}
	h, dir := feedHarness(t)
	t.Setenv("GRID_ROLE", "guest")
	if code := h.run(dir, "task", "delete", "T-1"); code != 3 || !strings.Contains(h.errb.String(), task.MoveRefusal) {
		t.Fatalf("task delete exit %d: %s", code, h.errb)
	}
}
