package brain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// TaskDecision is a durable receipt, also used to finish interrupted decisions.
type TaskDecision struct {
	Task     string `json:"task"`
	Proposal string `json:"proposal"`
	Action   string `json:"action"`
	Target   string `json:"target"`
	Note     string `json:"note"`
	At       string `json:"at"`
}
type decisionWrite struct {
	TaskDecision
	Entry        Note   `json:"entry"`
	ProposalNote Note   `json:"proposal_note"`
	Before       []byte `json:"before"`
	Noted        bool   `json:"noted"`
	Done         bool   `json:"done"`
}

// TaskRef names a resolved task folder; brain cannot import internal/task,
// which reads brain proposals at delivery.
type TaskRef struct{ ID, Path string }

// DecideTask files a task proposal through the same keeper gates as Accept.
// The receipt precedes publication, so retries neither duplicate merges nor
// overwrite an intervening edit. Completed proposals remain as audit records.
// history appends the task's history line through the task writer; it runs
// after the level lock is released, since delivery takes .task.lock first.
func (s Store) DecideTask(t TaskRef, proposal, action, target, note string, history func(string) error) (TaskDecision, error) {
	if !ValidName(proposal) {
		return TaskDecision{}, fmt.Errorf("proposal id must be a plain name")
	}
	if action != "accept" && action != "reject" && action != "merge-into" {
		return TaskDecision{}, fmt.Errorf("decision must be accept, reject or merge-into")
	}
	if !line(note) || size(note) > 300 {
		return TaskDecision{}, fmt.Errorf("note must be at most 300 characters on one line")
	}
	level, name, err := splitID(target)
	if err != nil {
		return TaskDecision{}, err
	}
	if level == "" {
		level = "project"
		if s.Project == "" {
			level = "home"
		}
	}
	target = "brain:" + level + "/" + name
	tr, err := os.OpenRoot(t.Path)
	if err != nil {
		return TaskDecision{}, err
	}
	defer tr.Close()
	if err := safefile.NoSymlinks(tr, ".brain-decide.lock"); err != nil {
		return TaskDecision{}, err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(t.Path, ".brain-decide.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return TaskDecision{}, err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return TaskDecision{}, err
	}
	root, err := s.scope(level, true)
	if err != nil {
		return TaskDecision{}, err
	}
	defer root.Close()
	var result TaskDecision
	var pending *decisionWrite
	err = locked(root, func() error {
		proposalPath := filepath.Join("brain-proposals", proposal+".md")
		receipt := filepath.Join("brain-proposals", ".decisions", proposal+".json")
		var w decisionWrite
		b, err := safefile.Read(tr, receipt)
		if err == nil {
			if err = json.Unmarshal(b, &w); err != nil {
				return err
			}
			if w.Task != t.ID || w.Proposal != proposal || w.Action != action || w.Target != target || w.Note != note {
				return fmt.Errorf("proposal already has a different decision")
			}
		} else if errors.Is(err, fs.ErrNotExist) {
			b, err = safefile.Read(tr, proposalPath)
			if err != nil {
				return err
			}
			n, err := Parse(b)
			if err != nil {
				return err
			}
			if n.Name != proposal || n.Status != "proposed" {
				return fmt.Errorf("expected matching proposed note")
			}
			if n.Level != level {
				return fmt.Errorf("proposal level disagrees with destination")
			}
			w.TaskDecision = TaskDecision{t.ID, proposal, action, target, note, s.stamp()}
			n.Updated = w.At
			if action == "reject" {
				n.Status = "rejected"
				n.Reason = note
				if n.Reason == "" {
					n.Reason = "Rejected by curator."
				}
				if hasSecret(n) {
					n = redacted(n, n.Reason)
				}
			} else {
				if reason := gate(root, n); reason != "" {
					return fmt.Errorf("proposal refused: %s", reason)
				}
				n.Status = "accepted"
				accepted := n
				accepted.Name = name
				entries, _, err := scan(root, level)
				if err != nil {
					return err
				}
				var existing *Note
				for _, e := range entries {
					if e.Name == name {
						copy := e
						existing = &copy
						break
					}
				}
				if action == "merge-into" {
					if existing == nil {
						return fmt.Errorf("merge target %s does not exist", target)
					}
					if reason := gate(root, *existing); reason != "" {
						return fmt.Errorf("merge target refused: %s", reason)
					}
					accepted = *existing
					path := filepath.Join("knowledge", accepted.Topic, name+".md")
					w.Before, err = safefile.Read(root, path)
					if err != nil {
						return err
					}
					accepted.Body = strings.TrimRight(accepted.Body, "\n") + "\n\n" + n.Body
					accepted.Links = append(accepted.Links, n.Links...)
					accepted.Source += "; " + n.Source
				} else if existing != nil {
					return fmt.Errorf("note %s already exists; use merge-into", target)
				}
				accepted.Updated = w.At
				dest := filepath.Join(s.GridHome, "knowledge", accepted.Topic)
				if level == "project" {
					dest = filepath.Join(s.GridHome, "projects", s.Project, "knowledge", accepted.Topic)
				}
				rel, err := filepath.Rel(dest, filepath.Join(t.Path, "task.md"))
				if err != nil {
					return err
				}
				accepted.Body = strings.TrimRight(accepted.Body, "\n") + fmt.Sprintf("\n\nSource: [%s](%s), proposal `%s`.\n", t.ID, filepath.ToSlash(rel), proposal)
				if reason := gate(root, accepted); reason != "" {
					return fmt.Errorf("finished note refused: %s", reason)
				}
				w.Entry = accepted
				n.Reason = action + " " + target
				if note != "" {
					n.Reason += "; " + note
				}
			}
			w.ProposalNote = n
			b, err = json.Marshal(w)
			if err != nil {
				return err
			}
			if err = atomicWrite(tr, receipt, b); err != nil {
				return err
			}
		} else {
			return err
		}
		result = w.TaskDecision
		if w.Done {
			return nil
		}
		if action != "reject" {
			path := filepath.Join("knowledge", w.Entry.Topic, w.Entry.Name+".md")
			after, err := Encode(w.Entry)
			if err != nil {
				return err
			}
			before, err := safefile.Read(root, path)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if !bytes.Equal(before, after) {
				if !bytes.Equal(before, w.Before) {
					return fmt.Errorf("destination changed since decision; refusing to overwrite it")
				}
				if err = atomicWrite(root, path, after); err != nil {
					return err
				}
			}
		}
		if err := logDecision(root, w.ProposalNote); err != nil {
			return err
		}
		b, err = Encode(w.ProposalNote)
		if err != nil {
			return err
		}
		if err = atomicWrite(tr, proposalPath, b); err != nil {
			return err
		}
		pending = &w
		return nil
	})
	receipt := filepath.Join("brain-proposals", ".decisions", proposal+".json")
	if err == nil && pending != nil && !pending.Noted {
		detail := "brain decided: " + proposal + " " + action + " " + target
		if note != "" {
			detail += "; " + note
		}
		if err = history(detail); err == nil {
			pending.Noted = true
			var b []byte
			if b, err = json.Marshal(pending); err == nil {
				err = atomicWrite(tr, receipt, b)
			}
		}
	}
	if err == nil && pending != nil {
		if action != "reject" {
			_, err = s.RebuildIndex()
		}
		if err == nil {
			pending.Done = true
			var b []byte
			b, err = json.Marshal(pending)
			if err == nil {
				err = atomicWrite(tr, receipt, b)
			}
		}
	}
	return result, err
}
