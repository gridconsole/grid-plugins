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
		allowed = QuestionWithdrawReadPolicy(s.TaskDir)
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
func (s Store) SaveState() (contract.State, error) { return s.SaveStateLiveness(nil) }

// SaveStateLiveness overlays holder observations for display only, never for folding.
func (s Store) SaveStateLiveness(liveness map[string]string) (contract.State, error) {
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
	if err := writeProjection(s.Dir(), "state.json", projection); err != nil {
		return contract.State{}, err
	}
	if err := writeProjection(s.Dir(), "needs-you.json", state.NeedsYou); err != nil {
		return contract.State{}, err
	}
	return state, nil
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
