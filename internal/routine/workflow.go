package routine

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// workflowLimits reads optional defaults from workflow.md in the routine's
// scope. Project routines never fall back to home policy. Existing workflow
// fields and body are left untouched; only these three front matter keys apply.
func (s Store) workflowLimits() (LimitPolicy, error) {
	var policy LimitPolicy
	path := "workflow.md"
	if s.Project != "" {
		if !brain.ValidName(s.Project) {
			return policy, errors.New("invalid routine project name")
		}
		path = filepath.Join("projects", s.Project, path)
	}
	root, err := os.OpenRoot(s.GridHome)
	if errors.Is(err, os.ErrNotExist) {
		return policy, nil
	}
	if err != nil {
		return policy, errors.New("cannot read routine workflow policy")
	}
	defer root.Close()
	raw, err := safefile.Read(root, path)
	if errors.Is(err, os.ErrNotExist) {
		return policy, nil
	}
	if err != nil {
		return policy, errors.New("cannot read routine workflow policy")
	}
	front, _, err := mdfile.Split(raw)
	if err != nil {
		return policy, errors.New("invalid routine workflow front matter")
	}
	if err := mdfile.Load(front, &policy, false); err != nil {
		return policy, errors.New("invalid routine workflow front matter")
	}
	_, err = policy.EffectiveLimits()
	return policy, err
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
