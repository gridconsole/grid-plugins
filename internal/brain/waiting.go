package brain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// WaitingProposal is a read-only keeper queue row, not a decision or a brain note.
type WaitingProposal struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
	// names is the note's source and origin id: the text that says whose proposal it is.
	names string
}

// WithKeeperGate shares the proposal writers' locks while checking the queue
// and raising a delivery gate. The callback must not write brain proposals.
func (s Store) WithKeeperGate(taskDir string, fn func() error) error {
	levels := s.levels()
	var visit func(int) error
	visit = func(i int) error {
		if i == len(levels) {
			root, err := os.OpenRoot(taskDir)
			if err != nil {
				return err
			}
			defer root.Close()
			return locked(root, fn)
		}
		root, err := s.scope(levels[i], true)
		if err != nil {
			return err
		}
		defer root.Close()
		return locked(root, func() error { return visit(i + 1) })
	}
	return visit(0)
}

// WaitingForTask is the part of the queue that holds back a task's ship gate:
// the task's own proposals, every proposal in its project's store, and the
// home-level proposals whose source or origin id names the task (T-NN) or its
// project. The home store is shared by every project, so an unrelated waiting
// proposal there does not stop this task; `grid brain keep` still lists it.
func (s Store) WaitingForTask(taskDir, taskID string) ([]WaitingProposal, error) {
	rows, err := s.Waiting(taskDir)
	if err != nil {
		return nil, err
	}
	out := rows[:0]
	for _, row := range rows {
		if !strings.HasPrefix(row.Path, "brain:home/") || row.naming(taskID) || (s.Project != "" && row.naming(s.Project)) {
			out = append(out, row)
		}
	}
	return out, nil
}

var nameToken = regexp.MustCompile(`[A-Za-z0-9._-]+`)

// naming reports whether a proposal's source or origin id carries the given
// task id or project name as a whole token. A task id also matches a folder
// name such as T-07-title, and T-07/J-01 splits at the slash.
func (p WaitingProposal) naming(name string) bool {
	if name == "" {
		return false
	}
	for _, token := range nameToken.FindAllString(p.names, -1) {
		if token == name || strings.HasPrefix(token, name+"-") && taskID.MatchString(name) {
			return true
		}
	}
	return false
}

var taskID = regexp.MustCompile(`^T-[0-9]+$`)

// Waiting includes proposals in the selected brain levels and, when supplied,
// the task's legacy proposal folder. Missing folders are empty; malformed files
// are errors, so delivery never treats unreadable proposals as settled. Project
// and home proposals are read with Parse, exactly as accept reads them; only the
// task folder's older proposals keep the lenient reader.
func (s Store) Waiting(taskDir string) ([]WaitingProposal, error) {
	out := []WaitingProposal{}
	for _, level := range s.levels() {
		root, err := s.scope(level, false)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		rows, err := waitingAt(root, "brain:"+level+"/", level)
		root.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	if taskDir != "" {
		root, err := os.OpenRoot(taskDir)
		if err != nil {
			return nil, err
		}
		rows, err := waitingAt(root, "task:", "")
		root.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// waitingAt lists one proposals folder. A level means a brain store, whose notes
// must parse strictly; an empty level is the task folder's lenient legacy reader.
func waitingAt(root *os.Root, prefix, level string) ([]WaitingProposal, error) {
	out := []WaitingProposal{}
	if err := safefile.NoSymlinks(root, "brain-proposals"); err != nil {
		return nil, err
	}
	dir, err := root.Open("brain-proposals")
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join("brain-proposals", entry.Name())
		id := strings.TrimSuffix(entry.Name(), ".md")
		var n Note
		if level != "" {
			if !ValidName(id) {
				return nil, fmt.Errorf("proposal %s: name is not a plain name", entry.Name())
			}
			if n, err = readProposal(root, level, id); err != nil {
				return nil, fmt.Errorf("proposal %s: %w", entry.Name(), err)
			}
		} else {
			b, err := safefile.Read(root, path)
			if err != nil {
				return nil, err
			}
			doc, err := frontmatter.Parse(string(b))
			if err != nil {
				return nil, fmt.Errorf("proposal %s: %w", entry.Name(), err)
			}
			if err = frontmatter.Into(doc.Data, &n); err != nil {
				return nil, fmt.Errorf("proposal %s: %w", entry.Name(), err)
			}
		}
		switch n.Status {
		case "accepted", "merged", "declined", "rejected":
			continue
		case "proposed":
		default:
			return nil, fmt.Errorf("proposal %s has an invalid or missing status", entry.Name())
		}
		topic := strings.Join(strings.Fields(n.Topic), " ")
		if topic == "" {
			topic = "unassigned"
		}
		reason := n.Reason
		if strings.TrimSpace(reason) == "" {
			reason = n.Summary
		}
		if strings.TrimSpace(reason) == "" {
			reason = n.Description
		}
		if strings.TrimSpace(reason) == "" {
			reason = "Waiting for keeper review"
		}
		out = append(out, WaitingProposal{ID: id, Path: prefix + filepath.ToSlash(path), Topic: topic, Reason: strings.Join(strings.Fields(reason), " "), names: n.Source + " " + n.Origin.ID})
	}
	return out, nil
}
