package migrate

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// Result names the immutable backup made by a successful migration.
type Result struct {
	Backup  string
	Changed bool
}
type digest struct {
	SHA256 string `json:"sha256,omitempty"`
	Mode   uint32 `json:"mode"`
}
type journal struct {
	Format      int               `json:"format"`
	Source      string            `json:"source"`
	Home        string            `json:"home"`
	Backup      string            `json:"backup"`
	Actions     []Action          `json:"actions"`
	SourceFiles map[string]digest `json:"source_files"`
	HomeFiles   map[string]digest `json:"home_files"`
}

func digests(t tree) map[string]digest {
	out := map[string]digest{}
	for k, e := range t {
		d := digest{Mode: uint32(e.Mode)}
		if !e.Mode.IsDir() {
			d.SHA256 = checksum(e)
		}
		out[k] = d
	}
	return out
}
func (p *Plan) unchanged() error {
	src, err := snapshot(p.From)
	if err != nil {
		return err
	}
	dst, err := snapshot(p.Home)
	if err != nil {
		return err
	}
	if !equal(src, p.source) || !equal(dst, p.destination) {
		return fmt.Errorf("data changed since planning; stop writers and run migrate again")
	}
	return nil
}

// Apply refuses unresolved plans and stale snapshots, backs up before touching
// the home, then publishes a fully prepared sibling directory. Run offline: the
// lock serializes migrators, not old application writers. External sources stay
// intact; old named homes inside Home are removed from the staged replacement.
func (p *Plan) Apply() (Result, error) {
	if current, err := versionAt(p.Home); err != nil {
		return Result{}, err
	} else if current {
		return Result{}, nil
	}
	if len(p.Conflicts) > 0 || len(p.Unmapped) > 0 {
		return Result{}, fmt.Errorf("migration blocked: resolve %d conflicts and %d unmapped items", len(p.Conflicts), len(p.Unmapped))
	}
	if p.result == nil {
		return Result{}, fmt.Errorf("invalid migration plan")
	}
	if err := p.unchanged(); err != nil {
		return Result{}, err
	}
	parent := filepath.Dir(p.Home)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return Result{}, err
	}
	lock := p.Home + ".migrate-lock"
	if err := os.Mkdir(lock, 0700); err != nil {
		return Result{}, fmt.Errorf("cannot acquire migration lock %s: %w", lock, err)
	}
	defer os.Remove(lock)
	if err := p.unchanged(); err != nil {
		return Result{}, err
	}
	backupRoot := p.Home + "-backup"
	// Refuse a symlink here so a backup can never overwrite an unrelated tree.
	if fi, err := os.Lstat(backupRoot); err == nil {
		if !fi.IsDir() {
			return Result{}, fmt.Errorf("backup root is not a directory: %s", backupRoot)
		}
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}
	if err := os.MkdirAll(backupRoot, 0700); err != nil {
		return Result{}, err
	}
	backup, err := os.MkdirTemp(backupRoot, time.Now().UTC().Format("20060102T150405Z")+"-")
	if err != nil {
		return Result{}, err
	}
	result := Result{Backup: backup}
	// A failed backup is retained for inspection and is never used for publication.
	fail := func(err error) (Result, error) {
		return result, fmt.Errorf("migration not published (backup %s): %w", backup, err)
	}
	if err := writeTree(filepath.Join(backup, "source"), p.source); err != nil {
		return fail(err)
	}
	if p.From != p.Home && len(p.destination) > 0 {
		if err := writeTree(filepath.Join(backup, "home"), p.destination); err != nil {
			return fail(err)
		}
	}
	backed, err := snapshot(filepath.Join(backup, "source"))
	if err != nil {
		return fail(err)
	}
	if !equal(backed, p.source) {
		return fail(fmt.Errorf("backup verification failed"))
	}
	if p.From != p.Home && len(p.destination) > 0 {
		backed, err = snapshot(filepath.Join(backup, "home"))
		if err != nil {
			return fail(err)
		}
		if !equal(backed, p.destination) {
			return fail(fmt.Errorf("home backup verification failed"))
		}
	}
	log := journal{gridhome.FormatVersion, p.From, p.Home, backup, p.Actions, digests(p.source), digests(p.destination)}
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fail(err)
	}
	data = append(data, '\n')
	if err := writeSynced(filepath.Join(backup, "manifest.json"), data, 0600); err != nil {
		return fail(err)
	}
	// The backup is complete and is never written again after this point.
	stage, err := os.MkdirTemp(parent, ".grid-migrate-stage-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(stage)
	output := tree{}
	for k, v := range p.result {
		output[k] = v
	}
	if e := output["machine"]; string(e.Data) == "<allocated on apply>\n" {
		var seed [32]byte
		if _, err := rand.Read(seed[:]); err != nil {
			return fail(err)
		}
		e.Data = []byte(legacyID(string(seed[:])) + "\n")
		output["machine"] = e
	}
	if err := writeTree(stage, output); err != nil {
		return fail(err)
	}
	if err := writeSynced(filepath.Join(stage, "migration.log"), data, 0600); err != nil {
		return fail(err)
	}
	if err := p.unchanged(); err != nil {
		return fail(err)
	}
	// Keep the original directory until publication succeeds. Ordinary failures
	// restore it; a process death in this short rename window requires recovery.
	displaced := filepath.Join(lock, "previous-home")
	hadHome := len(p.destination) > 0
	if hadHome {
		if err := os.Rename(p.Home, displaced); err != nil {
			return fail(err)
		}
	}
	if err := os.Rename(stage, p.Home); err != nil {
		if hadHome {
			if restoreErr := os.Rename(displaced, p.Home); restoreErr != nil {
				return fail(fmt.Errorf("publish: %v; restore %s manually: %w", err, displaced, restoreErr))
			}
		}
		return fail(err)
	}
	result.Changed = true
	if hadHome {
		if err := os.RemoveAll(displaced); err != nil {
			return result, fmt.Errorf("migration published; remove old staged home %s: %w", displaced, err)
		}
	}
	return result, nil
}
