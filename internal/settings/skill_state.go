package settings

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// ReadSkillsOff is the read-only answer to "which skills did the person turn
// off": the names whose effective value is false after the personal file and then
// the project file are laid over the defaults (every catalog skill on), sorted.
// It writes nothing and takes no lock, as the other readers here take none.
//
// The read looks at the "skills" group only. A settings document, project record
// or project folder that is missing or cannot be read for this purpose (another
// writer's keys, a symlinked folder, a damaged file) turns nothing off at that
// level: the answer never refuses a home the rest of render accepts. A project
// name or id with no record reads the personal level, as a spawned Job's model
// does. Only an empty home is an error.
func ReadSkillsOff(home, projectName string) ([]string, error) {
	if home == "" {
		return nil, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	root := gridhome.Real(home)
	dirs := []string{root}
	if projectName != "" && projectName != gridhome.Personal {
		if records, err := project.Load(root); err == nil {
			for _, rec := range records {
				if rec.Name != projectName && rec.ID != projectName {
					continue
				}
				dir := filepath.Join(root, "projects", rec.Name)
				if gridhome.Real(dir) == dir && dir != root {
					dirs = append(dirs, dir)
				}
				break
			}
		}
	}
	state := map[string]bool{}
	for _, dir := range dirs {
		for name, on := range readSkillGroup(dir) {
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

// readSkillGroup returns the skill values one settings.json holds, or nothing
// when the file is missing or its skills group cannot be read.
func readSkillGroup(dir string) map[string]bool {
	b, err := read(filepath.Join(dir, "settings.json"))
	if err != nil {
		return nil
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(b, &top) != nil {
		return nil
	}
	raw, ok := top["skills"]
	if !ok {
		return nil
	}
	var group map[string]bool
	if json.Unmarshal(raw, &group) != nil {
		return nil
	}
	return group
}
