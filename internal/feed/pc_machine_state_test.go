package feed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// Row 3: two logins fold the same task and share one machine folder. Each keeps its
// own feed; both publish state.json to tasks/<id>/state.json in the machine folder
// by atomic rename. A reader never decodes a partial file, under -race.
func TestSaveStateTwoLoginsShareOneMachineFolder(t *testing.T) {
	machine := filepath.Join(t.TempDir(), "machine")
	folder := machinefolder.Folder{Path: machine, Source: machinefolder.SourceEnv, SharedGroup: "grid"}
	logins := []Store{testStore(t), testStore(t)}
	for i := range logins {
		logins[i].Machine = folder
	}
	const rounds = 15
	writers := make([]Writer, len(logins))
	inputs := make([][]Input, len(logins))
	for i, s := range logins {
		writers[i] = taskWriter(t, s)
		for r := 0; r < rounds; r++ {
			id := fmt.Sprintf("J-%d", 100*i+r+1) // each login folds different Jobs, so the shared file changes hands
			inputs[i] = append(inputs[i], Input{Type: "job", Job: id, Body: contractBody(t, "job", map[string]any{"id": id, "title": "Job " + id})})
		}
	}

	statePath := filepath.Join(machine, "tasks", testTask, "state.json")
	var stop atomic.Bool
	var readers sync.WaitGroup
	var reads, partial atomic.Int64
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for !stop.Load() {
				b, err := os.ReadFile(statePath)
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				reads.Add(1)
				var st contract.State
				if err != nil || json.Unmarshal(b, &st) != nil {
					partial.Add(1)
				}
			}
		}()
	}
	var writersDone sync.WaitGroup
	errs := make([]error, len(logins))
	for i := range logins {
		writersDone.Add(1)
		go func(i int) {
			defer writersDone.Done()
			for _, in := range inputs[i] {
				if _, err := logins[i].Post(in, writers[i]); err != nil {
					errs[i] = err
					return
				}
				if _, err := logins[i].SaveState(); err != nil {
					errs[i] = err
					return
				}
			}
		}(i)
	}
	writersDone.Wait()
	stop.Store(true)
	readers.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("login %d: %v", i, err)
		}
	}
	if reads.Load() == 0 {
		t.Fatal("the readers never saw state.json, so the test proved nothing")
	}
	if partial.Load() != 0 {
		t.Fatalf("%d of %d reads saw a partial state.json", partial.Load(), reads.Load())
	}

	st, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != machinefolder.SharedFileMode {
		t.Errorf("state.json mode %v, want %v", st.Mode().Perm(), os.FileMode(machinefolder.SharedFileMode))
	}
	if dir, err := os.Stat(filepath.Dir(statePath)); err != nil || dir.Mode()&(os.ModePerm|os.ModeSetgid) != machinefolder.SharedDirMode {
		t.Errorf("tasks/<id> mode %v %v, want %v", dir, err, machinefolder.SharedDirMode)
	}
	for i, s := range logins {
		if _, err := os.Stat(filepath.Join(s.Dir(), "state.json")); err == nil {
			t.Errorf("login %d: feed/state.json was written although the machine folder is on", i)
		}
		if _, err := os.Stat(filepath.Join(s.Dir(), "needs-you.json")); err != nil {
			t.Errorf("login %d: needs-you.json stays in the feed folder: %v", i, err)
		}
	}
}

// Row 2: with no machine folder state.json is feed/state.json, as before.
func TestSaveStateUnconfiguredWritesFeedStateJSON(t *testing.T) {
	s := testStore(t)
	foldJob(t, s, taskWriter(t, s), "J-1")
	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(s.Dir(), "state.json"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("feed/state.json: %v %v, want 0600", st, err)
	}
}
