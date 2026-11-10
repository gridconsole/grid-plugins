package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestPersonAnswerScopeAndWorkflowRoles restores the rows of the test of the
// same name that S6b removed with the person gate (base 6ac08ed9), without the
// process-tree setup: a workflow's person_verbs role list is checked against
// GRID_ROLE (default member) before the verb runs. A refusal exits 3 with the
// person_required error and writes nothing.
func TestPersonAnswerScopeAndWorkflowRoles(t *testing.T) {
	for _, verb := range []string{"answer", "question answer"} {
		for _, tc := range []struct {
			name, role, who, policy, reason string
			code                            int
		}{
			{"guest_other_question", "guest", "other", "", "addressed to that guest", 1},
			{"guest_workflow_denied", "guest", "visitor", "member", "agent_env", 3},
			{"lead_workflow_denied", "lead", "visitor", "member", "agent_env", 3},
			{"guest_workflow_allowed", "guest", "visitor", "guest", "", 0},
			{"lead_workflow_allowed", "lead", "visitor", "lead", "", 0},
			{"member_workflow_allowed", "member", "visitor", "member", "", 0},
			{"default_role_is_member", "", "visitor", "member", "", 0},
		} {
			t.Run(verb+"/"+tc.name, func(t *testing.T) {
				h, dir := feedHarness(t)
				q := questionAdd(t, h, dir, tc.who)
				if tc.policy != "" {
					h.write(filepath.Join(dir, "workflow.md"), "---\nperson_verbs: {"+verb+": ["+tc.policy+"]}\n---\n")
				}
				t.Setenv("GRID_ROLE", tc.role)
				t.Setenv("GRID_ACTOR", "visitor")
				t.Setenv("GRID_MEMBER", "visitor")
				before := fileBytes(t, dir)
				args := append(strings.Fields(verb), dir, q.ID, "Yes", "--json")
				if code := h.run(dir, args...); code != tc.code || !strings.Contains(h.out.String()+h.errb.String(), tc.reason) {
					t.Fatalf("answer exit %d: %s %s", code, h.out, h.errb)
				}
				if tc.code == 3 && !strings.Contains(h.out.String(), `"error":"person_required"`) {
					t.Fatalf("refusal without the person_required code: %s", h.out)
				}
				if tc.code != 0 && !reflect.DeepEqual(before, fileBytes(t, dir)) {
					t.Fatal("refused answer changed task")
				}
			})
		}
	}
}

// A role list does not look at the environment's agent markers: a guest is
// refused and an allowed member is admitted whether or not GRID_AGENT is set.
func TestWorkflowRoleListIgnoresAgentEnvironment(t *testing.T) {
	for _, tc := range []struct {
		role string
		code int
	}{{"member", 0}, {"guest", 3}} {
		t.Run(tc.role, func(t *testing.T) {
			h, dir := feedHarness(t)
			q := questionAdd(t, h, dir, "visitor")
			h.write(filepath.Join(dir, "workflow.md"), "---\nperson_verbs: {answer: [member]}\n---\n")
			t.Setenv("GRID_ROLE", tc.role)
			t.Setenv("GRID_AGENT", "builder")
			t.Setenv("GRID_ACTOR", "visitor")
			t.Setenv("GRID_MEMBER", "visitor")
			if code := h.run(dir, "answer", dir, q.ID, "Yes", "--json"); code != tc.code {
				t.Fatalf("exit %d, want %d: %s %s", code, tc.code, h.out, h.errb)
			}
		})
	}
}
