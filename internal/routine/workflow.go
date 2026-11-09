package routine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// WorkflowPolicyError identifies inherited policy without naming a routine,
// exposing supplied values, or including a filesystem path.
type WorkflowPolicyError struct {
	Level string
	Err   error
}

func (e *WorkflowPolicyError) Error() string { return e.Level + " workflows.md: " + e.Err.Error() }
func (e *WorkflowPolicyError) Unwrap() error { return e.Err }

// workflowLimits overlays project keys on workspace keys. Each source is
// validated even when higher-precedence keys mask it. Entry overrides are
// applied separately so inherited values are never saved in the entry.
func (s Store) workflowLimits() (LimitPolicy, error) {
	var policy LimitPolicy
	if s.Project != "" && !brain.ValidName(s.Project) {
		return policy, errors.New("invalid routine project name")
	}
	root, err := os.OpenRoot(s.GridHome)
	if errors.Is(err, os.ErrNotExist) {
		return policy, nil
	}
	if err != nil {
		return policy, &WorkflowPolicyError{Level: "workspace", Err: errors.New("cannot read routine workflow policy")}
	}
	defer root.Close()
	policy, err = readWorkflowLimits(root, "workflows.md", "workspace")
	if err != nil || s.Project == "" {
		return policy, err
	}
	project, err := readWorkflowLimits(root, filepath.Join("projects", s.Project, "workflows.md"), "project")
	if err != nil {
		return policy, err
	}
	for _, d := range limitDefinitions {
		if n := *d.field(&project); n.Kind != 0 {
			*d.field(&policy) = n
		}
	}
	return policy, nil
}

func readWorkflowLimits(root *os.Root, path, level string) (LimitPolicy, error) {
	var policy LimitPolicy
	fail := func(err error) (LimitPolicy, error) {
		return policy, &WorkflowPolicyError{Level: level, Err: err}
	}
	raw, err := safefile.Read(root, path)
	if errors.Is(err, os.ErrNotExist) {
		return policy, nil
	}
	if err != nil {
		return fail(errors.New("cannot read routine workflow policy"))
	}
	// Existing template-only workflows.md files have no policy front matter.
	if !hasFrontMatter(raw) {
		return policy, nil
	}
	front, _, err := mdfile.Split(raw)
	if err != nil {
		return fail(errors.New("invalid routine workflow front matter"))
	}
	if strings.TrimSpace(string(front)) == "" {
		return policy, nil
	}
	if err := mdfile.Load(front, &policy, false); err != nil {
		return fail(errors.New("invalid routine workflow front matter"))
	}
	if _, err := policy.EffectiveLimits(); err != nil {
		return fail(err)
	}
	return policy, nil
}

func hasFrontMatter(raw []byte) bool {
	first, _, _ := strings.Cut(string(raw), "\n")
	return strings.TrimSpace(first) == "---"
}

// LegacyLimitWarnings reports old workspace/project policy locations once per
// file. These files never supply limits or block a run, and task files are not
// inspected. Reads retain the same bounds and symlink restrictions as policy.
func (s Store) LegacyLimitWarnings() []string {
	if s.Project != "" && !brain.ValidName(s.Project) {
		return nil
	}
	root, err := os.OpenRoot(s.GridHome)
	if err != nil {
		return nil
	}
	defer root.Close()
	paths := []string{"workflow.md"}
	if s.Project != "" {
		paths = append(paths, filepath.Join("projects", s.Project, "workflow.md"))
	}
	var warnings []string
	for _, path := range paths {
		raw, err := safefile.Read(root, path)
		if err != nil || !hasFrontMatter(raw) {
			continue
		}
		front, _, err := mdfile.Split(raw)
		if err != nil {
			continue
		}
		var fields map[string]any
		if err := mdfile.Load(front, &fields, false); err != nil {
			continue
		}
		for key := range fields {
			if strings.HasPrefix(key, "routine_") {
				label := filepath.ToSlash(path)
				target := strings.TrimSuffix(label, "workflow.md") + "workflows.md"
				warnings = append(warnings, label+" contains ignored routine_* keys; move them to "+target)
				break
			}
		}
	}
	return warnings
}

func (s Store) resolveLimits(e Entry) (Entry, error) {
	var err error
	e.workflowPolicy, err = s.workflowLimits()
	if err != nil {
		return e, err
	}
	e.Limits, err = e.EffectiveLimits()
	return e, err
}
