package brain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
)

// WaitingProposal is a read-only keeper queue row, not a decision or a brain note.
type WaitingProposal struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
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

// Waiting includes proposals in the selected brain levels and, when supplied,
// the task's legacy proposal folder. Missing folders are empty; malformed files
// are errors, so delivery never treats unreadable proposals as settled.
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
		rows, err := waitingAt(root, "brain:"+level+"/")
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
		rows, err := waitingAt(root, "task:")
		root.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func waitingAt(root *os.Root, prefix string) ([]WaitingProposal, error) {
	out := []WaitingProposal{}
	if err := noSymlinks(root, "brain-proposals"); err != nil {
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
		b, err := read(root, path)
		if err != nil {
			return nil, err
		}
		doc, err := frontmatter.Parse(string(b))
		if err != nil {
			return nil, fmt.Errorf("proposal %s: %w", entry.Name(), err)
		}
		var n Note
		if err = frontmatter.Into(doc.Data, &n); err != nil {
			return nil, fmt.Errorf("proposal %s: %w", entry.Name(), err)
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
		out = append(out, WaitingProposal{ID: strings.TrimSuffix(entry.Name(), ".md"), Path: prefix + filepath.ToSlash(path), Topic: topic, Reason: strings.Join(strings.Fields(reason), " ")})
	}
	return out, nil
}
