package feed

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// State folds the task through the contract fold. The reader keeps its own
// concerns: held records wait, bad and ignored lines stay out, and records past
// an ended tenure reach the fold marked stale.
func (s Store) State() (contract.State, error) {
	read, err := s.Read(Filter{})
	if err != nil {
		return contract.State{}, err
	}
	return contract.Fold(posts(read)), nil
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
func (s Store) SaveState() (contract.State, error) {
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
	if err := lock(f, true); err != nil {
		return contract.State{}, err
	}
	defer unlock(f)
	state, err := s.State()
	if err != nil {
		return contract.State{}, err
	}
	if err := writeProjection(s.Dir(), "state.json", state); err != nil {
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
	f, err := os.CreateTemp(dir, ".projection-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}
