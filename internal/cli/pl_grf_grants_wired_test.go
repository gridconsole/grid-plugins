package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

const plGrfLink = "grid/people/dana/grants/answers"

// plGrfAgentAnswers sets up, through the CLI, a person Job J-2 for dana beside
// J-1. The agent J-1 is assigned to acks and answers J-2 for dana under plGrfLink
// at a fixed time. When grant is set, the brain folder the Grid config names
// holds dana's grant note and her own note. Then the holder's pass runs as
// `grid job next`, and J-2's status is returned.
func plGrfAgentAnswers(t *testing.T, grant bool) contract.Status {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	vault := filepath.Join(h.root, "vault")
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: h.gridHome, Brain: vault, Member: "member"}}}
	if err := gridconfig.Write(gridconfig.Path(os.Getenv, h.home), cfg); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(vault, "grid", "people", "dana.md"), "---\nrole: member\n---\nDana.\n")
	if grant {
		h.write(filepath.Join(vault, filepath.FromSlash(plGrfLink)+".md"), "---\nagents: worker\nposts: answer\ntasks: all\nuntil: 2030-01-02T00:00:00Z\n---\nAnswers for me.\n")
	}
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("person Job: %d %s", code, h.errb)
	}
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("first pass: %d %s", code, h.errb)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j1, j2 := state.Jobs["J-1"], state.Jobs["J-2"]
	if j1 == nil || j1.Assign == nil || j2 == nil || j2.Status != contract.Posted {
		t.Fatalf("J-1 is not assigned or J-2 is not posted: %+v", state.Jobs)
	}
	to := j1.Assign.To
	w := feed.Writer{Author: feed.Author{ID: to[:strings.IndexByte(to, '@')], Role: "worker"}, Worker: to, Lease: "J-1", Fence: j1.Assign.Fence, Gen: j1.Generation}
	agent := store
	agent.Now = func() time.Time { return time.Date(2030, 1, 1, 10, 0, 0, 0, time.UTC) }
	if _, err := orchestrator.Write(agent, w, feed.Input{Type: "ack", Job: "J-1", Attempt: j1.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"answer": "Call it Lark.", "for": "dana", "by": w.Author.ID, "grant": "[[" + plGrfLink + "]]"}
	if _, err := orchestrator.Write(agent, w, feed.Input{Type: "answer", Re: j2.History[0], Job: "J-1", Attempt: j1.Attempt, Body: body}); err != nil {
		t.Fatalf("the agent's answer was refused: %v", err)
	}
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("the holder's pass: %d %s", code, h.errb)
	}
	if state, err = store.State(); err != nil {
		t.Fatal(err)
	}
	return state.Jobs["J-2"].Status
}

// Fix row 1: `grid job next` reads the grants from the brain folder's
// grid/people, so an agent's answer for dana under her covering grant closes her
// person Job as hers; the same answer without the grant note folds as the
// agent's own and leaves the Job posted, and nothing is refused.
func TestPlGrfCLIPassFoldsAGrantedAnswerAsThePersons(t *testing.T) {
	if got := plGrfAgentAnswers(t, true); got != contract.Accepted {
		t.Fatalf("with dana's grant in the brain J-2 is %s, want accepted as dana's answer", got)
	}
}

func TestPlGrfCLIPassFoldsAnUngrantedAnswerAsTheAgents(t *testing.T) {
	if got := plGrfAgentAnswers(t, false); got != contract.Posted {
		t.Fatalf("without a grant J-2 is %s, want posted: the answer is the agent's own", got)
	}
}
