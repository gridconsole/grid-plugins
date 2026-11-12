package settings

import (
	"errors"
	"path/filepath"
	"sort"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// ReadSkillsOff is the read-only answer to "which skills did the person turn
// off": the names whose effective value is false after the personal file and then
// the project file are laid over the defaults (every catalog skill on), sorted.
// It writes nothing and takes no lock, as the other readers here take none. A
// project name or id with no record reads the personal level, as a spawned
// Job's model does; any other error is returned.
func ReadSkillsOff(home, projectName string) ([]string, error) {
	if home == "" {
		return nil, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	root := gridhome.Real(home)
	dirs := []string{root}
	if projectName != "" && projectName != gridhome.Personal {
		records, err := project.Load(root)
		if err != nil {
			return nil, err
		}
		for _, rec := range records {
			if rec.Name != projectName && rec.ID != projectName {
				continue
			}
			dir := filepath.Join(root, "projects", rec.Name)
			if gridhome.Real(dir) != dir {
				return nil, errors.New("project settings directory must not be a symlink")
			}
			if dir != root {
				dirs = append(dirs, dir)
			}
			break
		}
	}
	state := map[string]bool{}
	for _, dir := range dirs {
		d, err := readDocument(dir)
		if err != nil {
			return nil, err
		}
		for name, on := range d.Skills {
			state[name] = on
		}
	}
	var off []string
	for name, on := range state {
		if !on {
			off = append(off, name)
		}
	}
	sort.Strings(off)
	return off, nil
}
