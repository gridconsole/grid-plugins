package projectenv

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

// vaultDir is the folder of the project's environments below the brain folder:
// projects/<project>/grid/environments.
func vaultDir(project string) string {
	return path.Join("projects", project, "grid", topic)
}

// readVault reads one environment from the brain folder's grid/ folder. A
// missing folder or entry wraps fs.ErrNotExist, so the caller can fall back to
// knowledge/, where Save still writes new entries.
func readVault(brainDir, project, name string) ([]byte, error) {
	if !brain.ValidName(project) || !brain.ValidName(name) || name == "TOPIC" {
		return nil, fmt.Errorf("config needs a valid project and a plain entry name")
	}
	root, err := os.OpenRoot(brainDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return safefile.Read(root, path.Join(vaultDir(project), name+".md"))
}

// vaultNames lists the environments in the brain folder's grid/ folder, the same
// way brain.Store.ConfigNames lists knowledge/.
func vaultNames(brainDir, project string) ([]string, error) {
	names := []string{}
	if !brain.ValidName(project) {
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
	dir := vaultDir(project)
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

// readEntry reads an environment: from the brain folder's grid/environments when
// the store names the brain folder, falling back to knowledge/environments for
// one the folder does not hold.
func readEntry(s brain.Store, name string) ([]byte, error) {
	if s.Vault() {
		data, err := readVault(s.BrainDir, s.Project, name)
		if !errors.Is(err, fs.ErrNotExist) {
			return data, err
		}
	}
	return s.ReadConfig(level, topic, name)
}

// entryNames lists the environments: those of grid/environments first and those
// of knowledge/environments that grid/environments does not hold.
func entryNames(s brain.Store) ([]string, error) {
	names, err := s.ConfigNames(level, topic)
	if err != nil || !s.Vault() {
		return names, err
	}
	more, err := vaultNames(s.BrainDir, s.Project)
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
