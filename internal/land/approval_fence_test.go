package land

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestAgentCannotForgeShipApproval(t *testing.T) {
	s, dir, repo := fixture(t)
	setWorkflow(t, s, dir, "ship")
	p, _, _, err := s.prepare(repo, "T-01")
	if err != nil {
		t.Fatal(err)
	}
	// Use a receipt that land would accept if the agent's Write were allowed.
	data, err := json.Marshal(task.Event{Task: "T-01", What: "approve", Gate: "ship", PlanHash: p.Hash, LandsHash: ruleHash(p.Rule)})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "approval-ship.json")
	input := hookio.Input{
		Cwd: repo, HookEventName: "PreToolUse", AgentType: "grid-builder", ToolName: "Write",
		ToolInput: map[string]any{"file_path": path, "content": string(data)},
	}
	decision := guard.EvaluateHost(input, repo, map[string]string{"GRID_HOME": s.Resolver.GridHome})
	if decision.Behavior != "deny" {
		t.Errorf("agent Write was not refused: %+v", decision)
		write(t, path, string(data))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("agent receipt exists: %v", err)
	}
	if _, err := s.Run(context.Background(), repo, "T-01", false); !errors.Is(err, ErrRejected) {
		t.Errorf("land must reject missing ship approval: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "outcome")); !os.IsNotExist(err) {
		t.Errorf("deploy command ran: %v", err)
	}
	if t.Failed() {
		return
	}
	// A subsequent person approval releases the same gate and command.
	b, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "task.md"), strings.Replace(string(b), "state: waiting", "gate: ship\nstate: waiting", 1))
	approve(t, s)
	if _, err := s.Run(context.Background(), repo, "T-01", false); err != nil {
		t.Fatalf("person approval did not release land: %v", err)
	}
}
