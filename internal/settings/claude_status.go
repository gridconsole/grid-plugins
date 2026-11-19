package settings

import (
	"errors"
	"slices"
)

// ClaudeAllowStatus reports exact matches for setup's allow rules. Reason is a
// path-free failure to inspect the settings; Present and Missing are otherwise
// populated in setup order. A missing block is not a read or parse failure.
type ClaudeAllowStatus struct {
	Present []string
	Missing []string
	Reason  string
}

// ClaudeDenyStatus is the same report for setup's permissions.deny rules.
type ClaudeDenyStatus = ClaudeAllowStatus

// ClaudeSandboxStatus is State, one of "on", "off" or "unknown", and, when
// unknown, a path-free Reason.
type ClaudeSandboxStatus struct {
	State  string
	Reason string
}

// readClaudeSettings reads and parses the settings file. A non-empty reason is
// path-free; missing is true when the file does not exist (the reason is then
// the missing-file reason).
func readClaudeSettings(path string) (doc claudeObject, reason string, missing bool) {
	if path == "" {
		return nil, "cannot locate the Claude settings file", false
	}
	file, err := readClaudeFile(path)
	var readErr claudeReadError
	switch {
	case errors.As(err, &readErr):
		return nil, readErr.reason, false
	case err != nil:
		return nil, "cannot read the Claude settings file", false
	case !file.exists:
		return nil, "Claude settings file is missing", true
	}
	doc, err = parseClaudeObject(file.data)
	if err != nil {
		return nil, "Claude settings file is not valid JSON or has an invalid structure", false
	}
	return doc, "", false
}

// ReadClaudeAllowStatus never writes settings or the setup ownership record.
// It follows settings-file symlinks through the same reader used by setup.
func ReadClaudeAllowStatus(path, home, gridHome string) ClaudeAllowStatus {
	doc, reason, _ := readClaudeSettings(path)
	if reason != "" {
		return ClaudeAllowStatus{Reason: reason}
	}
	rules, _, err := claudeAllowRules(home, gridHome)
	if err != nil {
		return ClaudeAllowStatus{Reason: err.Error()} // The shared builder returns path-free reasons.
	}
	return claudeRuleStatus(doc, rules)
}

// ReadClaudeDenyStatus checks the permissions.deny rules of the setup block. Setup
// writes none now, so it finds none missing; the projects folder argument is kept
// for the callers that still pass it. It never writes anything and gives the same
// path-free reasons as ReadClaudeAllowStatus.
func ReadClaudeDenyStatus(path, home, gridHome, _ string) ClaudeDenyStatus {
	doc, reason, _ := readClaudeSettings(path)
	if reason != "" {
		return ClaudeDenyStatus{Reason: reason}
	}
	block, err := claudeBlock(home, gridHome)
	if err != nil {
		return ClaudeDenyStatus{Reason: err.Error()}
	}
	var rules []ClaudeRule
	for _, rule := range block {
		if rule.Section == "permissions" && rule.Key == "deny" {
			rules = append(rules, rule)
		}
	}
	return claudeRuleStatus(doc, rules)
}

func claudeRuleStatus(doc claudeObject, rules []ClaudeRule) ClaudeAllowStatus {
	status := ClaudeAllowStatus{Present: []string{}, Missing: []string{}}
	for _, rule := range rules {
		values, _, err := claudeValues(doc, rule)
		if err != nil {
			return ClaudeAllowStatus{Reason: "Claude settings file is not valid JSON or has an invalid structure"}
		}
		if slices.Contains(values, rule.Value) {
			status.Present = append(status.Present, rule.Value)
		} else {
			status.Missing = append(status.Missing, rule.Value)
		}
	}
	return status
}

// ReadClaudeSandbox reports whether sandbox.enabled is true in the settings. A
// missing file or an absent key is off; a path that cannot be located, a file
// that cannot be read, or invalid JSON is unknown. It never writes.
func ReadClaudeSandbox(path string) ClaudeSandboxStatus {
	doc, reason, missing := readClaudeSettings(path)
	if missing {
		return ClaudeSandboxStatus{State: "off"}
	}
	if reason != "" {
		return ClaudeSandboxStatus{State: "unknown", Reason: reason}
	}
	on, err := claudeSandboxOn(doc)
	switch {
	case err != nil:
		return ClaudeSandboxStatus{State: "unknown", Reason: "Claude settings file has an invalid sandbox setting"}
	case on:
		return ClaudeSandboxStatus{State: "on"}
	}
	return ClaudeSandboxStatus{State: "off"}
}
