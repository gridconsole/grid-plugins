package orchestrator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
)

// grantNote is the front matter of a grant note. Agents, posts, kinds and tasks
// take one name or a list; until is RFC 3339.
type grantNote struct {
	Agents any    `yaml:"agents"`
	Posts  any    `yaml:"posts"`
	Kinds  any    `yaml:"kinds"`
	Tasks  any    `yaml:"tasks"`
	Until  string `yaml:"until"`
}

// LoadGrants reads the grant notes of the brain folder brainDir:
// grid/people/<person>/grants/<name>.md, written by the person
// (run-jobs-launcher.md Q8). The person and the link come from where the note
// lives, and the role from the person's own note grid/people/<person>.md. A
// note that does not read is left out, so it covers nothing: the answers that
// name it count as their agents' own. A brain folder without people has none.
func LoadGrants(brainDir string) ([]contract.Grant, error) {
	if brainDir == "" {
		return nil, nil
	}
	root, err := os.OpenRoot(brainDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	people, err := fs.ReadDir(root.FS(), "grid/people")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []contract.Grant
	for _, p := range people {
		if !p.IsDir() || strings.HasPrefix(p.Name(), ".") {
			continue
		}
		person := p.Name()
		dir := path.Join("grid/people", person, "grants")
		notes, err := fs.ReadDir(root.FS(), dir)
		if err != nil {
			continue
		}
		role := grantPersonRole(root, person)
		for _, n := range notes {
			name := n.Name()
			if !n.Type().IsRegular() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, ".") {
				continue
			}
			data, err := root.ReadFile(path.Join(dir, name))
			if err != nil {
				continue
			}
			front, _, err := mdfile.Split(data)
			if err != nil {
				continue
			}
			var g grantNote
			if mdfile.Load(front, &g, false) != nil {
				continue
			}
			out = append(out, contract.Grant{
				Link:   path.Join(dir, strings.TrimSuffix(name, ".md")),
				Person: person,
				Role:   role,
				Agents: grantNames(g.Agents),
				Posts:  grantNames(g.Posts),
				Kinds:  grantNames(g.Kinds),
				Tasks:  grantNames(g.Tasks),
				Until:  strings.TrimSpace(g.Until),
			})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Link < out[b].Link })
	return out, nil
}

// grantPersonRole is the role in the person's own note, "" when it does not read.
func grantPersonRole(root *os.Root, person string) string {
	data, err := root.ReadFile(path.Join("grid/people", person+".md"))
	if err != nil {
		return ""
	}
	front, _, err := mdfile.Split(data)
	if err != nil {
		return ""
	}
	var p struct {
		Role string `yaml:"role"`
	}
	if mdfile.Load(front, &p, false) != nil {
		return ""
	}
	return strings.TrimSpace(p.Role)
}

// grantNames reads one name or a list of names.
func grantNames(v any) []string {
	var out []string
	switch x := v.(type) {
	case string:
		if s := strings.TrimSpace(x); s != "" {
			out = append(out, s)
		}
	case []any:
		for _, item := range x {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}
