// Package settings owns personal settings, project overrides and setup answers.
package settings

import (
	"fmt"
	"regexp"
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
var models = []string{"haiku", "sonnet", "opus", "fable"}
var efforts = []string{"low", "medium", "high", "xhigh", "max"}
var skills = []string{"brain-keeper", "browser", "deliver", "first-run", "plan-jobs", "questions", "research", "review-job", "run-job", "setup", "signals", "task", "visual-design-review"}
var hooks = []string{"history-log", "home-allow", "host-guard", "job-lint", "needs-you", "only-writer", "plan-guard", "session-check", "session-end", "session-start", "stop-check", "usage-log"}
var levels = []string{"off", "warn", "block"}

// HookEvent is one hook event a rule runs on, named as hooks/hooks.json names it.
// Matcher is that entry's matcher, empty when the entry has none.
type HookEvent struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher,omitempty"`
}

// HookRule is one row of grid hook levels --json --detail: the rule, its level,
// the events it runs on and a one-line description. A rule that no hook event
// runs has an empty Events (job-lint runs in grid job render). Gate is true for
// a rule that a Hook level of Off leaves at its level; it only reports that and
// does not follow the level.
type HookRule struct {
	Name   string      `json:"n"`
	Level  string      `json:"level"`
	Gate   bool        `json:"gate"`
	Events []HookEvent `json:"events"`
	Desc   string      `json:"d"`
}

type hookInfo struct {
	name   string
	events []HookEvent
	desc   string
	gate   bool
}

// sessionStartMatcher is the matcher hooks/hooks.json gives SessionStart.
const sessionStartMatcher = "startup|resume|clear|compact|fork"

// hookInfos holds, for each name in hooks, the events the rule runs on and what it
// does. The events are those the code consults the rule on (internal/hook), each
// named as hooks/hooks.json wires it; TestHookInfoEventsAreWired fails when this
// table and hooks.json disagree.
var hookInfos = []hookInfo{
	{"history-log", []HookEvent{{"PreToolUse", ""}, {"PostToolUse", ""}, {"SubagentStop", ""}}, "Writes file edits, Bash commands, finished subagents and brain reads to the task history.", false},
	{"home-allow", []HookEvent{{"PermissionRequest", ""}, {"PreToolUse", ""}}, "Allows a tool call inside the Grid home so the person is not asked.", true},
	{"host-guard", []HookEvent{}, "Retired: no hook runs it, so its level changes nothing.", true},
	{"job-lint", []HookEvent{}, "Checks a Job when grid job render builds it; lint errors are said on stderr and the Job still renders; off skips the check.", true},
	{"needs-you", []HookEvent{}, "No hook runs it yet, so its level changes nothing.", false},
	{"only-writer", []HookEvent{}, "Retired: no hook runs it, so its level changes nothing.", true},
	{"plan-guard", []HookEvent{{"PreToolUse", ""}, {"PermissionRequest", ""}}, "Keeps a reviewer or checker from writing outside its task folder; block refuses it, warn says it.", true},
	{"session-check", []HookEvent{}, "No hook runs it yet, so its level changes nothing.", false},
	{"session-end", []HookEvent{{"SessionEnd", ""}, {"SubagentStop", ""}}, "Records how a worker's session ended on its Job.", false},
	{"session-start", []HookEvent{{"SessionStart", sessionStartMatcher}}, "Gives a starting session its Grid context.", false},
	{"stop-check", []HookEvent{{"Stop", ""}, {"SubagentStop", ""}}, "Refuses a stop while an agent's Job has no result on the Feed.", true},
	{"usage-log", []HookEvent{{"SessionStart", sessionStartMatcher}, {"SubagentStop", ""}, {"PostToolUse", ""}}, "Records token usage and transcript references of sessions and subagents.", false},
}

// HookRules returns every rule with its level from levels (a name missing there
// reads the default), the events it runs on and its description, in the order of
// HookNames.
func HookRules(levels map[string]string) []HookRule {
	out := make([]HookRule, 0, len(hookInfos))
	for _, h := range hookInfos {
		level := levels[h.name]
		if level == "" {
			level = defaultLevel(h.name)
		}
		out = append(out, HookRule{Name: h.name, Level: level, Gate: h.gate, Events: slices.Clone(h.events), Desc: h.desc})
	}
	return out
}

// retiredHooks are hooks removed from Grid. A settings.json written earlier may
// still name one; the file reads without error and the entry is ignored.
var retiredHooks = []string{"brief-lint", "precompact", "subagent-start", "topic-trigger", "user-prompt"}

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

// checkLevel accepts a known level for any hook: no guard is fixed at one level.
func checkLevel(_, level string) error {
	return allowed("hook level", level, levels)
}

// sdModelID is the shape of a full Claude model id; the check after it is Claude Code's own.
var sdModelID = regexp.MustCompile(`^claude-[a-z0-9.-]+$`)

// sdModelAllowed is the allowed list shown when a model is refused.
const sdModelAllowed = "haiku, sonnet, opus, fable, <name>[1m] or a claude- model id"

// sdNormalizeModel returns the model as stored and whether it is accepted: the
// four names (with an optional [1m]) match in any case and store lower-case; a
// claude- id stores as given, with an optional [1m].
func sdNormalizeModel(m string) (string, bool) {
	base := strings.TrimSuffix(m, "[1m]")
	if lower := strings.ToLower(base); slices.Contains(models, lower) {
		return lower + m[len(base):], true
	}
	return m, sdModelID.MatchString(base)
}

func validateAgent(a Agent) error {
	if a.Model != "" {
		if _, ok := sdNormalizeModel(a.Model); !ok {
			return fmt.Errorf("unknown model %q; allowed: %s", a.Model, sdModelAllowed)
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
	case "stop-check":
		return "block"
	}
	return "warn"
}
