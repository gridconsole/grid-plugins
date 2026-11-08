// Package settings owns personal settings, project overrides and setup answers.
package settings

import (
	"fmt"
	"slices"
	"strings"
)

// Agent holds an agent's model and effort; an omitted field inherits its value.
type Agent struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

type agentDefault struct{ name, model, effort string }

// Defaults follow plugin-design.md:234-246; values follow settings.html:2278.
var agents = []agentDefault{
	{"grid-builder", "sonnet", "medium"}, {"grid-checker", "sonnet", "low"},
	{"grid-curator", "opus", "medium"}, {"grid-designer", "sonnet", "medium"},
	{"grid-library", "haiku", "low"}, {"grid-operator", "sonnet", "medium"},
	{"grid-planner", "opus", "medium"}, {"grid-researcher", "opus", "medium"},
	{"grid-reviewer", "opus", "medium"}, {"grid-runner", "haiku", "low"},
	{"grid-scout", "haiku", "low"},
}
var models = []string{"haiku", "sonnet", "opus"}
var efforts = []string{"low", "medium", "high", "xhigh", "max"}
var skills = []string{"brain-keeper", "browser", "deliver", "job", "orchestrate", "plan", "questions", "research", "setup", "signals", "task", "visual-design-review"}
var hooks = []string{"brief-lint", "history-log", "home-allow", "host-guard", "job-lint", "needs-you", "only-writer", "plan-guard", "precompact", "session-check", "session-end", "session-start", "stop-check", "subagent-start", "topic-trigger", "usage-log", "user-prompt"}
var levels = []string{"off", "warn", "block"}

// neverOff are the guards that hold the person's gates and the writers' fence
// (plugin-design.md:90, :92): they always block, whatever a settings file says.
var neverOff = []string{"brief-lint", "host-guard", "only-writer", "plan-guard"}

// AlwaysBlock reports whether a hook is a guard that cannot be turned off or lowered.
func AlwaysBlock(name string) bool { return slices.Contains(neverOff, name) }

// AgentNames returns the supported agents in stable order.
func AgentNames() []string {
	out := make([]string, len(agents))
	for i, a := range agents {
		out[i] = a.name
	}
	return out
}

// SkillNames returns the supported skills in stable order.
func SkillNames() []string { return slices.Clone(skills) }

// HookNames returns the supported hooks in stable order.
func HookNames() []string { return slices.Clone(hooks) }
func allowed(kind, value string, choices []string) error {
	if !slices.Contains(choices, value) {
		return fmt.Errorf("unknown %s %q; allowed: %s", kind, value, strings.Join(choices, ", "))
	}
	return nil
}

// checkLevel accepts a known level, and only block for a guard that always blocks.
func checkLevel(name, level string) error {
	if err := allowed("hook level", level, levels); err != nil {
		return err
	}
	if AlwaysBlock(name) && level != "block" {
		return fmt.Errorf("%s always blocks and cannot be set to %s; allowed: block", name, level)
	}
	return nil
}
func validateAgent(a Agent) error {
	if a.Model != "" {
		if err := allowed("model", a.Model, models); err != nil {
			return err
		}
	}
	if a.Effort != "" {
		if err := allowed("effort", a.Effort, efforts); err != nil {
			return err
		}
	}
	if a.Model == "" && a.Effort == "" {
		return fmt.Errorf("agent settings require model or effort; allowed: model, effort")
	}
	return nil
}
func defaultLevel(name string) string {
	switch name {
	case "brief-lint", "host-guard", "job-lint", "only-writer", "plan-guard", "stop-check":
		return "block"
	}
	return "warn"
}
