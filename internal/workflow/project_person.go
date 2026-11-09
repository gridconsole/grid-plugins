package workflow

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// ProjectPersonVerbs reads a resolved project's policy, then the workspace's.
// A set value replaces the inherited policy wholly, including an empty list.
// The person-gate floor remains independent of these replaceable defaults.
func ProjectPersonVerbs(home, project string) (map[string][]string, error) {
	if project == "" || project == "." || project == ".." || filepath.Base(project) != project || filepath.IsAbs(project) {
		return nil, fmt.Errorf("%w: invalid project", ErrPersonVerbs)
	}
	home = gridhome.Real(home)
	for _, path := range []string{filepath.Join("projects", project, "workflows.md"), "workflows.md"} {
		text, _, err := readFile(home, path)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrPersonVerbs, fileReason(err))
		}
		policy, err := personVerbs([]byte(text))
		if err != nil {
			if !errors.Is(err, ErrPersonVerbs) {
				err = fmt.Errorf("%w: malformed front matter", ErrPersonVerbs)
			}
			return nil, err
		}
		if policy != nil {
			return policy, nil
		}
	}
	return map[string][]string{"signal act turn": nil, "signal act dismiss": nil, "signal act snooze": nil}, nil
}
