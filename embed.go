// Package gridplugins holds the plugin files that code must carry with it.
package gridplugins

import (
	"embed"
	"strings"
)

// OrchestratorAgent is agents/orchestrator.md: what a Job of role orchestrator
// tells the agent that runs it. The Job render copies its body into the prompt,
// so a worker that starts claude without --agent still runs the orchestrator.
//
//go:embed agents/orchestrator.md
var OrchestratorAgent string

//go:embed agents/*.md
var agents embed.FS

// AgentFor is the text of agents/<role>.md for a normalized role, and false when
// the role has no agent file. A role is a plain name, never a path.
func AgentFor(role string) (string, bool) {
	if role == "" || strings.ContainsAny(role, "/\\.") {
		return "", false
	}
	b, err := agents.ReadFile("agents/" + role + ".md")
	if err != nil {
		return "", false
	}
	return string(b), true
}
