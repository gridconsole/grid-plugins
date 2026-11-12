package brain

import (
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// AgentFor is the markdown of the brain agent that runs a Job of the given role,
// and the level it came from. The names tried, in order, are the catalog agent
// of the role (grid-planner for orchestrator), grid-<role> and <role>; for each
// name the project level is read before the home level. The first file found
// wins. A file whose front matter does not parse, or holds a key outside the
// ones `grid brain check` allows for an agent, is skipped as that check would
// flag it, and the search goes on to the next level and name. Line endings come
// back as \n.
func (s Store) AgentFor(role string) (md, level string, ok bool) {
	for _, name := range saAgentNames(role) {
		for _, lv := range s.levels() {
			if text, found := s.saAgentAt(lv, name); found {
				return text, lv, true
			}
		}
	}
	return "", "", false
}

// saAgentNames lists the agent file names for a role, without repeats or names
// the brain would refuse.
func saAgentNames(role string) []string {
	role = contract.NormalizeRole(role)
	if role == "" {
		return nil
	}
	var names []string
	add := func(n string) {
		if ValidName(n) && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	if n, ok := settings.AgentForRole(role); ok {
		add(n)
	}
	add("grid-" + role)
	add(role)
	return names
}

// saAgentAt reads knowledge/agents/<name>.md at one level and answers it only
// when `grid brain check` would accept its front matter.
func (s Store) saAgentAt(level, name string) (string, bool) {
	b, err := s.ReadConfig(level, "agents", name)
	if err != nil {
		return "", false
	}
	keys, err := frontKeys(b)
	if err != nil {
		return "", false
	}
	for _, k := range keys {
		if !agentFields[k.name] {
			return "", false
		}
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n"), true
}
