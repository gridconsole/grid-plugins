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

//go:embed skills/*/SKILL.md
var skills embed.FS

// SkillFor is the text of skills/<name>/SKILL.md, and false when the plugin
// carries no skill of that name. A name is a plain directory name, never a path.
func SkillFor(name string) (string, bool) {
	if name == "" || strings.ContainsAny(name, "/\\.") {
		return "", false
	}
	b, err := skills.ReadFile("skills/" + name + "/SKILL.md")
	if err != nil {
		return "", false
	}
	return string(b), true
}

//go:embed agent-templates/grid-*.md
var templates embed.FS

// AgentFor is the text of the file that defines a normalized role: agents/<role>.md,
// else the role's template agent-templates/grid-<role>.md, so a role a Job can name
// has one definition and no role is left without instructions. It answers false when
// the role has neither file. A role is a plain name, never a path.
func AgentFor(role string) (string, bool) {
	if role == "" || strings.ContainsAny(role, "/\\.") {
		return "", false
	}
	if b, err := agents.ReadFile("agents/" + role + ".md"); err == nil {
		return string(b), true
	}
	if b, err := templates.ReadFile("agent-templates/grid-" + role + ".md"); err == nil {
		return string(b), true
	}
	return "", false
}
