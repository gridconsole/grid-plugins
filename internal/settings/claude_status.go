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

// ReadClaudeAllowStatus never writes settings or the setup ownership record.
// It follows settings-file symlinks through the same reader used by setup.
func ReadClaudeAllowStatus(path, home, gridHome string) ClaudeAllowStatus {
	failure := func(reason string) ClaudeAllowStatus {
		return ClaudeAllowStatus{Reason: reason}
	}
	if path == "" {
		return failure("cannot locate the Claude settings file")
	}
	file, err := readClaudeFile(path)
	var readErr claudeReadError
	switch {
	case errors.As(err, &readErr):
		return failure(readErr.reason)
	case err != nil:
		return failure("cannot read the Claude settings file")
	case !file.exists:
		return failure("Claude settings file is missing")
	}
	doc, err := parseClaudeObject(file.data)
	if err != nil {
		return failure("Claude settings file is not valid JSON or has an invalid structure")
	}
	rules, _, err := claudeAllowRules(home, gridHome)
	if err != nil {
		return failure(err.Error()) // The shared builder returns path-free reasons.
	}
	status := ClaudeAllowStatus{Present: []string{}, Missing: []string{}}
	for _, rule := range rules {
		values, _, err := claudeValues(doc, rule)
		if err != nil {
			return failure("Claude settings file is not valid JSON or has an invalid structure")
		}
		if slices.Contains(values, rule.Value) {
			status.Present = append(status.Present, rule.Value)
		} else {
			status.Missing = append(status.Missing, rule.Value)
		}
	}
	return status
}
