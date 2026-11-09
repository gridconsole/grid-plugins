package settings

import (
	"errors"
	"io"
	"os"
	"slices"

	"github.com/gridconsole/grid-plugins/internal/fslock"
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
// Unlike setup, the health check refuses to follow a settings-file symlink.
func ReadClaudeAllowStatus(path, home, gridHome string) ClaudeAllowStatus {
	failure := func(reason string) ClaudeAllowStatus {
		return ClaudeAllowStatus{Reason: reason}
	}
	if path == "" {
		return failure("cannot locate the Claude settings file")
	}
	st, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return failure("Claude settings file is missing")
	case err != nil:
		return failure("cannot read the Claude settings file")
	case st.Mode()&os.ModeSymlink != 0:
		return failure("Claude settings file is a symlink")
	case !st.Mode().IsRegular():
		return failure("Claude settings file is not a regular file")
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return failure("cannot read the Claude settings file")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return failure("Claude settings file changed while reading")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return failure("cannot read the Claude settings file")
	}
	doc, err := parseClaudeObject(data)
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
