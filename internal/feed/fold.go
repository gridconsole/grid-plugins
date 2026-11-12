package feed

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// State folds the task through the contract fold. The reader keeps its own
// concerns: held records wait, bad and ignored lines stay out, and records past
// an ended tenure reach the fold marked stale.
func (s Store) State() (contract.State, error) {
	allowed := s.WithdrawAllowed
	if allowed == nil {
		allowed = QuestionWithdrawReadPolicy(s.TaskDir, s.Warn)
	}
	read, err := s.Read(Filter{})
	if err != nil {
		return contract.State{}, err
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	state, _ := FoldReadAt(read, now, allowed)
	return state, nil
}

// posts converts the admitted entries of a read to contract posts.
func posts(read ReadResult) []contract.Post {
	out := []contract.Post{}
	for _, e := range read.Records {
		if e.Verdict != "counted" && e.Verdict != "stale" && e.Verdict != "unknown" {
			continue
		}
		body, err := json.Marshal(e.Body)
		if err != nil {
			continue
		}
		r := e.Record
		out = append(out, contract.Post{ID: r.ID, Task: r.Task, Type: r.Type, TypeV: r.TypeV, Author: r.Author, Tier: r.Tier, Worker: r.Worker, Lease: r.Lease, Gen: r.Gen, Fence: r.Fence, Seq: r.Seq, At: r.At, Re: r.Re, After: r.After, Job: r.Job, Attempt: r.Attempt, Wake: r.Wake, Body: body, Stale: e.Verdict == "stale"})
	}
	return out
}

// SaveState serializes concurrent folds and atomically publishes each projection.
// Readers needing an indivisible snapshot should use State or state.json, which
// includes needs_you as well as the separately published needs-you.json.
//
// SaveState carries no observations of its own, so it keeps the ones the last
// tick wrote: for each Job whose assign_post and status are unchanged, the
// "liveness" already in state.json stays. A post front end refolds through here
// and must not blank the Agents view until the next tick.
func (s Store) SaveState() (contract.State, error) { return s.saveState(nil, true, false) }

// RefoldPublished is the refold a post front end runs after its post. It keeps
// SaveState's liveness, and it refreshes only what a reader could trust: state.json
// if the file is there, needs-you.json if the file is there or the fold has a
// question to list. A task that has published nothing and has no open question gets
// no new file, so a post adds its one record to feed/ and nothing else. A missing
// needs-you.json is not stale: a reader folds when it is absent.
func (s Store) RefoldPublished() (contract.State, error) { return s.saveState(nil, true, true) }

// SaveStateLiveness overlays holder observations for display only, never for folding.
// The map is the whole overlay: an empty one says nothing is observed and drops
// what state.json held, so a tick that sees no holder clears the old values.
func (s Store) SaveStateLiveness(liveness map[string]string) (contract.State, error) {
	return s.saveState(liveness, false, false)
}

// published says whether feed/<name> is there, whatever it is.
func (s Store) published(name string) bool {
	_, err := os.Lstat(filepath.Join(s.Dir(), name))
	return err == nil
}

func (s Store) saveState(liveness map[string]string, keep, onlyPublished bool) (contract.State, error) {
	if err := s.prepare(); err != nil {
		return contract.State{}, err
	}
	if err := os.MkdirAll(filepath.Join(s.Dir(), ".cache"), 0o700); err != nil {
		return contract.State{}, err
	}
	f, err := os.OpenFile(filepath.Join(s.Dir(), ".cache", "fold.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return contract.State{}, err
	}
	defer f.Close()
	if err := fslock.LockExclusive(f); err != nil {
		return contract.State{}, err
	}
	defer fslock.Unlock(f)
	state, err := s.State()
	if err != nil {
		return contract.State{}, err
	}
	if keep && len(liveness) == 0 {
		// Still under fold.lock, the only lock any writer of state.json takes.
		liveness = keptLiveness(s.Dir(), state)
	}
	var projection any = state
	if len(liveness) > 0 {
		b, err := json.Marshal(state)
		if err != nil {
			return contract.State{}, err
		}
		var view map[string]any
		if err := json.Unmarshal(b, &view); err != nil {
			return contract.State{}, err
		}
		jobs := view["jobs"].(map[string]any)
		for id, value := range liveness {
			if j, ok := jobs[id].(map[string]any); ok {
				j["liveness"] = value
			}
		}
		projection = view
	}
	if !onlyPublished || s.published("state.json") {
		if err := writeProjection(s.Dir(), "state.json", projection); err != nil {
			return contract.State{}, err
		}
	}
	if !onlyPublished || s.published("needs-you.json") || len(state.NeedsYou) > 0 {
		if err := writeProjection(s.Dir(), "needs-you.json", state.NeedsYou); err != nil {
			return contract.State{}, err
		}
	}
	return state, nil
}

// keptLiveness returns the "liveness" values state.json holds for Jobs that are
// the same in state: same assign_post (the same assignment, so the same holder)
// and same status. A Job that was reassigned or moved on is not listed, so an
// observation of its old holder does not outlive it. A missing or undecodable
// file keeps nothing; it is not an error, the next tick writes the values again.
func keptLiveness(dir string, state contract.State) map[string]string {
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return nil
	}
	var old struct {
		Jobs map[string]struct {
			Liveness   string `json:"liveness"`
			AssignPost string `json:"assign_post"`
			Status     string `json:"status"`
		} `json:"jobs"`
	}
	if json.Unmarshal(data, &old) != nil {
		return nil
	}
	kept := map[string]string{}
	for id, was := range old.Jobs {
		now, ok := state.Jobs[id]
		if ok && was.Liveness != "" && was.AssignPost == now.AssignPost && was.Status == string(now.Status) {
			kept[id] = was.Liveness
		}
	}
	return kept
}

func writeProjection(dir, name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(dir, name)
	old, err := os.ReadFile(path)
	if err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicfile.Write(path, data, 0o600)
}

// FoldRead folds precisely one reader snapshot, without reading the Feed again.
func FoldRead(read ReadResult) contract.State { return contract.Fold(posts(read)) }
