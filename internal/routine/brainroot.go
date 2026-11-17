package routine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// vaultConfigDir is the folder of a config topic below the brain folder:
// grid/<topic> at home, projects/<project>/grid/<topic> at project level.
func vaultConfigDir(level, project, topic string) string {
	if level == "project" {
		return path.Join("projects", project, "grid", topic)
	}
	return path.Join("grid", topic)
}

// readVault reads one entry of a config topic from the brain folder's grid/
// folder. A missing folder or entry wraps fs.ErrNotExist, so the caller can fall
// back to knowledge/, where the writers still put new entries.
func readVault(brainDir, level, project, topic, name string) ([]byte, error) {
	if level == "project" && !brain.ValidName(project) {
		return nil, fmt.Errorf("project level needs a valid project name")
	}
	if !brain.ValidName(name) || name == "TOPIC" {
		return nil, fmt.Errorf("config needs a config topic and a plain entry name")
	}
	root, err := os.OpenRoot(brainDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return safefile.Read(root, path.Join(vaultConfigDir(level, project, topic), name+".md"))
}

// vaultNames lists the entries of a config topic in the brain folder's grid/
// folder, the same way brain.Store.ConfigNames lists knowledge/.
func vaultNames(brainDir, level, project, topic string) ([]string, error) {
	names := []string{}
	if level == "project" && !brain.ValidName(project) {
		return nil, fmt.Errorf("project level needs a valid project name")
	}
	root, err := os.OpenRoot(brainDir)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir := vaultConfigDir(level, project, topic)
	if err := safefile.NoSymlinks(root, dir); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(root.FS(), dir)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("config paths cannot be symbolic links")
		}
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "TOPIC.md" {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names, nil
}

// readEntry reads a routine: from the brain folder's grid/routines when the
// store carries the brain folder, falling back to knowledge/routines for an
// entry the folder does not hold; with no brain folder, knowledge/routines alone.
func (s Store) readEntry(name string) ([]byte, error) {
	if s.BrainDir != "" {
		b, err := readVault(s.BrainDir, s.level(), s.Project, "routines", name)
		if !errors.Is(err, fs.ErrNotExist) {
			return b, err
		}
	}
	return s.brain().ReadConfig(s.level(), "routines", name)
}

// entryNames lists the routines of the level: those of grid/routines first and
// those of knowledge/routines that grid/routines does not hold.
func (s Store) entryNames() ([]string, error) {
	names, err := s.brain().ConfigNames(s.level(), "routines")
	if err != nil || s.BrainDir == "" {
		return names, err
	}
	more, err := vaultNames(s.BrainDir, s.level(), s.Project, "routines")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, n := range append(more, names...) {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}
