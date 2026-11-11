package settings

import (
	"slices"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// AgentForRole is the catalog agent that runs a Job of the given role: grid-<role>
// when the catalog has it, and grid-planner for role orchestrator, the agent that
// plans and decides. A role with no catalog agent answers false.
func AgentForRole(role string) (string, bool) {
	role = contract.NormalizeRole(role)
	if role == "orchestrator" {
		return "grid-planner", true
	}
	name := "grid-" + role
	if !slices.Contains(AgentNames(), name) {
		return "", false
	}
	return name, true
}

// ModelFor is the model and effort a Job of the given role runs on: the catalog
// default of its agent, then the personal values, then the project's. A role with
// no catalog agent answers two empty strings, which a spawn leaves out so claude
// keeps its own default.
func (s Store) ModelFor(role string) (model, effort string, err error) {
	name, ok := AgentForRole(role)
	if !ok {
		return "", "", nil
	}
	doc, err := s.Effective()
	if err != nil {
		return "", "", err
	}
	a := doc.Agents[name]
	return a.Model, a.Effort, nil
}
