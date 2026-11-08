package land

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// ErrRejected is a final rejection: review and approve again before retrying.
var ErrRejected = errors.New("land refused")

func ruleHash(rule Rule) string {
	data, _ := json.Marshal(rule)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Approve binds the deploy rule snapshot to the same receipt as the plan hash.
// Projects without a deploy topic retain the ordinary task approval behavior.
func (s Store) Approve(o task.WriteOptions, hash string) (task.Event, error) {
	t, err := s.tasks().Show(o.Cwd, o.ID)
	if err != nil {
		return task.Event{}, err
	}
	if t.Project != gridhome.Personal {
		rule, err := s.rule(t.Project)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return task.Event{}, err
		}
		if err == nil {
			o.LandsHash = ruleHash(rule)
		}
	}
	return s.tasks().Approve(o, hash)
}

// gate reads the named workflow's front matter until a shared workflow reader
// exists. An absent selection or absent land-approval field retains the plan gate;
// a selected but unreadable workflow is an error, never an approval bypass.
func (s Store) gate(t task.Task) (string, error) {
	if t.Workflow == "" {
		return "plan", nil
	}
	if strings.ContainsAny(t.Workflow, `/\\`) || t.Workflow == "." || t.Workflow == ".." {
		return "", fmt.Errorf("invalid workflow name %q", t.Workflow)
	}
	root, err := os.OpenRoot(s.Resolver.GridHome)
	if err != nil {
		return "", err
	}
	defer root.Close()
	path := filepath.Join("projects", t.Project, "knowledge", "workflows", t.Workflow+".md")
	data, err := safefile.Read(root, path)
	if err != nil {
		return "", fmt.Errorf("workflow %q: %w", t.Workflow, err)
	}
	front, _, err := mdfile.Split(data)
	if err != nil {
		return "", err
	}
	var fields map[string]any
	if err := mdfile.Load(front, &fields, false); err != nil {
		return "", err
	}
	value, present := fields["land-approval"]
	if !present {
		return "plan", nil
	}
	gate, ok := value.(string)
	if ok && (gate == "plan" || gate == "ship" || gate == "none") {
		return gate, nil
	}
	return "", fmt.Errorf("workflow land-approval must be plan, ship or none")
}
