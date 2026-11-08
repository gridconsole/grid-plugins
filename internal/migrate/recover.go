package migrate

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// A migration moves the old Grid home aside and puts the staged one in its place,
// two renames with a moment between them in which no home exists. The journal is
// written before the first of them, so a run that stops there can be finished or
// undone from what the disk shows. Beside the Grid home, <home> being its path:
//
//	<home>.migrate-lock       persistent advisory lock held by one running migrate
//	<home>.migrate-journal    what the run is doing, written before each phase
//	<home>.migrate-stage-ID   the new home, built and synced before it is journaled
//	<home>.migrate-previous-ID the old home, between the two renames and until the end
//	<home>-backup/DATE-ID     the verified copy of the old data; never removed

func lockPath(home string) string    { return home + ".migrate-lock" }
func journalPath(home string) string { return home + ".migrate-journal" }

// afterStep is a test hook, called once a step of an apply or a recovery is durable.
// A test that exits the process in it leaves the disk as a crash at that step would.
var afterStep func(step string)

func reached(step string) {
	if afterStep != nil {
		afterStep(step)
	}
}

func randomToken() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // no randomness, no safe temporary names
	}
	return hex.EncodeToString(b[:])
}

// The lock inode stays in place; the kernel releases ownership after a crash.
type lock struct{ file *os.File }

func acquireLock(path string) (*lock, error) {
	f, err := fslock.OpenNoFollow(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return &lock{file: f}, nil
}

func (l *lock) release() { fslock.Unlock(l.file); _ = l.file.Close() }

const (
	phasePreparing  = "preparing"  // backup and staged copy are being made; the home is untouched
	phasePublishing = "publishing" // the staged copy is complete; the swap may have begun
)

type journal struct {
	Version  int             `json:"version"`
	Phase    string          `json:"phase"`
	Source   string          `json:"source"`
	Home     string          `json:"home"`
	Backup   string          `json:"backup"`
	Stage    string          `json:"stage"`
	Previous string          `json:"previous"`
	HadHome  bool            `json:"had_home"`
	Pointers []pointerChange `json:"pointers,omitempty"`
}

// writeJournal replaces the journal atomically, so it is never half written.
func writeJournal(j journal) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return task.AtomicWrite(journalPath(j.Home), append(data, '\n'))
}

func removeJournal(home string) error {
	if err := os.Remove(journalPath(home)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	syncDir(filepath.Dir(home))
	return nil
}

func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func exists(path string) bool { _, err := os.Lstat(path); return err == nil }

// recovery is what an interrupted run left behind and what a rerun will do about it.
type recovery struct {
	j      journal
	action string // discard, forward, back or finalize
	text   string
}

// findRecovery reads the journal beside home and decides from the folders that
// exist. It writes nothing. A state it cannot explain is an error, never a guess.
func findRecovery(home string) (*recovery, error) {
	data, err := os.ReadFile(journalPath(home))
	if errors.Is(err, fs.ErrNotExist) {
		if prev, _ := filepath.Glob(home + ".migrate-previous-*"); len(prev) > 0 {
			return nil, fmt.Errorf("%s holds old Grid data from an interrupted migration but no journal names it; move it back to %s by hand", prev[0], home)
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j journal
	if err := json.Unmarshal(data, &j); err != nil || j.Version != 1 || j.Home != home {
		return nil, fmt.Errorf("cannot read the migration journal %s; check %s and the folders beside it, then remove the journal", journalPath(home), home)
	}
	h, p, s := exists(j.Home), exists(j.Previous), exists(j.Stage)
	r := &recovery{j: j}
	switch {
	case j.Phase != phasePublishing && !p:
		r.action = "discard"
		r.text = "an earlier run stopped while it was copying; its backup and staged copy are removed, and the Grid home was never touched"
	case j.Phase != phasePublishing:
		return nil, stuck(j, "the old home is set aside before the staged copy was complete")
	case h && s && !p && j.HadHome:
		r.action = "discard"
		r.text = "an earlier run stopped just before it swapped the folders; the staged copy is removed and the Grid home is untouched"
	case !h && p && s:
		r.action = "forward"
		r.text = "an earlier run stopped between the two renames and there is no Grid home; the staged copy is moved into place and the old home removed (backup " + j.Backup + ")"
	case !h && !p && s && !j.HadHome:
		r.action = "forward"
		r.text = "an earlier run stopped before the staged copy was moved into place; it is moved now (backup " + j.Backup + ")"
	case !h && p && !s:
		r.action = "back"
		r.text = "an earlier run stopped between the two renames and the staged copy is gone; the old home is put back"
	case h && !s:
		r.action = "finalize"
		r.text = "an earlier run published the new Grid home and stopped before it cleaned up; the leftovers are removed (backup " + j.Backup + ")"
	default:
		return nil, stuck(j, "the folders do not match any step of a migration")
	}
	return r, nil
}

func stuck(j journal, why string) error {
	return fmt.Errorf("cannot recover the interrupted migration (%s). Journal %s; home %s, set-aside %s, staged %s, backup %s. Nothing was changed", why, journalPath(j.Home), j.Home, j.Previous, j.Stage, j.Backup)
}

// run does what findRecovery decided. Every step can itself be interrupted: the
// next run looks at the disk again and carries on from there.
func (r *recovery) run() (Result, error) {
	j := r.j
	res := Result{Backup: j.Backup, Recovered: r.text}
	step := func(name string, err error) error {
		if err == nil {
			reached(name)
		}
		return err
	}
	var err error
	switch r.action {
	case "discard":
		if !exists(filepath.Join(j.Backup, "manifest.json")) {
			err = os.RemoveAll(j.Backup) // an incomplete backup is of no use
		}
		if err == nil {
			err = step("recover-discarded", os.RemoveAll(j.Stage))
		}
		res.Rerun = true
	case "forward":
		err = step("recover-renamed", os.Rename(j.Stage, j.Home))
		if err == nil {
			err = applyPointers(j.Pointers)
		}
		if err == nil {
			err = os.RemoveAll(j.Previous)
		}
		res.Changed = true
	case "back":
		err = step("recover-renamed", os.Rename(j.Previous, j.Home))
		if err == nil {
			err = os.RemoveAll(j.Stage)
		}
		res.Rerun = true
	case "finalize":
		err = applyPointers(j.Pointers)
		if err == nil {
			err = os.RemoveAll(j.Previous)
		}
		res.Changed = true
	}
	if err == nil {
		syncDir(filepath.Dir(j.Home))
		err = removeJournal(j.Home)
	}
	if err != nil {
		return Result{}, fmt.Errorf("recover the interrupted migration: %w", err)
	}
	return res, nil
}

// cleanStrays removes staged copies that no journal names. It runs under the lock.
func cleanStrays(home string) {
	stray, _ := filepath.Glob(home + ".migrate-stage-*")
	for _, s := range stray {
		os.RemoveAll(s)
	}
}
