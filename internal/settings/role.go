package settings

import (
	"slices"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
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
	return s.ModelForWith(role, Agent{})
}

// ModelForWith is ModelFor with the model and effort of the brain's agent for the
// role laid between the catalog default and the personal values: catalog, then
// brain, then personal, then project. An empty brain field inherits. A role with
// no catalog agent answers the brain values as they are, and reads no settings
// file; personal and project values are kept per catalog agent.
func (s Store) ModelForWith(role string, brain Agent) (model, effort string, err error) {
	name, ok := AgentForRole(role)
	if !ok {
		return brain.Model, brain.Effort, nil
	}
	dir, err := s.directory()
	if err != nil {
		return "", "", err
	}
	doc := empty()
	for _, a := range agents {
		if a.name == name {
			doc.Agents[name] = Agent{a.model, a.effort}
		}
	}
	merge(&doc, Document{Agents: map[string]Agent{name: brain}})
	// The layers Effective reads, one at a time, so the brain's values sit under them.
	dirs := []string{gridhome.Real(s.Home)}
	if dir != dirs[0] {
		dirs = append(dirs, dir)
	}
	for _, dir := range dirs {
		d, err := readDocument(dir)
		if err != nil {
			return "", "", err
		}
		merge(&doc, d)
	}
	a := doc.Agents[name]
	return a.Model, a.Effort, nil
}
