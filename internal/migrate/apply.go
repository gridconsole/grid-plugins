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

// Result tells what Apply did. Backup names the immutable backup made by a
// successful migration. Recovered says what was done about an interrupted earlier
// run; Rerun is set when that run was undone, so the home is still the old one and
// migrate must run again.
type Result struct {
	Backup    string
	Changed   bool
	Recovered string
	Rerun     bool
}

type digest struct {
	SHA256  string `json:"sha256,omitempty"`
	Mode    uint32 `json:"mode"`
	Size    int64  `json:"size,omitempty"`
	Link    string `json:"link,omitempty"`
	ModTime int64  `json:"mtime_ns,omitempty"`
}

// manifest is the backup's manifest.json and, copied, the new home's migration.log.
// HomeFiles is set only when the home was backed up apart from the source.
type manifest struct {
	Format      int               `json:"format"`
	Source      string            `json:"source"`
	Home        string            `json:"home"`
	Backup      string            `json:"backup"`
	Actions     []Action          `json:"actions"`
	SourceFiles map[string]digest `json:"source_files"`
	HomeFiles   map[string]digest `json:"home_files,omitempty"`
	PluginData  string            `json:"plugin_data,omitempty"`
	PluginFiles map[string]digest `json:"plugin_files,omitempty"`
}

// unchanged compares sizes, times and modes, not contents: it runs several times,
// and the backup carries the checksums.
func (p *Plan) unchanged() error {
	if err := checkPointers(p.pointers, false); err != nil {
		return err
	}
	if p.pluginData != "" {
		current, err := snapshot(p.pluginData)
		if err != nil {
			return err
		}
		if !equalMeta(current, p.plugin) {
			return fmt.Errorf("CLAUDE_PLUGIN_DATA sessions changed since planning; stop writers and run migrate again")
		}
	}
	src, err := snapshot(p.From)
	if err != nil {
		return err
	}
	dst := src
	if p.From != p.Home {
		if dst, err = snapshot(p.Home); err != nil {
			return err
		}
	}
	if !equalMeta(src, p.source) || !equalMeta(dst, p.destination) {
		return fmt.Errorf("data changed since planning; stop writers and run migrate again")
	}
	return nil
}

// Apply refuses unresolved plans and stale snapshots, backs up before touching
// the home, then publishes a fully prepared sibling directory. Run offline: the
// lock serializes migrators, not old application writers. External sources stay
// intact; old named homes inside Home are removed from the staged replacement.
//
// A run that stopped part way is finished or undone first, from its journal; see
// recover.go for the states.
func (p *Plan) Apply() (Result, error) {
	if p.recovery == nil {
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
	}
	parent := filepath.Dir(p.Home)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return Result{}, err
	}
	lk, err := acquireLock(lockPath(p.Home))
	if err != nil {
		return Result{}, fmt.Errorf("cannot acquire migration lock: %w", err)
	}
	defer lk.release()
	reached("locked")
	rec, err := findRecovery(p.Home)
	if err != nil {
		return Result{}, err
	}
	if rec != nil {
		if rec.j.Rollback {
			return Result{}, fmt.Errorf("an interrupted rollback is pending; run grid migrate --rollback")
		}
		return rec.run()
	}
	if p.recovery != nil {
		return Result{}, fmt.Errorf("the interrupted migration was already dealt with; run grid migrate again")
	}
	cleanStrays(p.Home)
	if err := p.unchanged(); err != nil {
		return Result{}, err
	}
	return p.publish(parent)
}

func (p *Plan) publish(parent string) (Result, error) {
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
	token := randomToken()
	j := journal{
		Version: 1, Phase: phasePreparing, Source: p.From, Home: p.Home,
		Backup:   filepath.Join(backupRoot, time.Now().UTC().Format("20060102T150405Z")+"-"+token),
		Stage:    p.Home + ".migrate-stage-" + token,
		Previous: p.Home + ".migrate-previous-" + token,
		HadHome:  len(p.destination.m) > 0,
		Pointers: p.pointers,
	}
	if err := writeJournal(j); err != nil {
		return Result{}, err
	}
	reached("journal-preparing")
	result := Result{Backup: j.Backup}
	// A failed backup is retained for inspection and is never used for publication.
	fail := func(err error) (Result, error) {
		os.RemoveAll(j.Stage)
		if rerr := removeJournal(p.Home); rerr != nil {
			err = fmt.Errorf("%w; remove %s by hand", err, journalPath(p.Home))
		}
		return result, fmt.Errorf("migration not published (backup %s): %w", j.Backup, err)
	}
	if err := os.Mkdir(j.Backup, 0700); err != nil {
		return fail(err)
	}
	man := manifest{Format: gridhome.FormatVersion, Source: p.From, Home: p.Home, Backup: j.Backup, Actions: p.Actions}
	var err error
	if man.SourceFiles, err = backupTree(filepath.Join(j.Backup, "source"), p.source, p.From); err != nil {
		return fail(err)
	}
	if p.From != p.Home && len(p.destination.m) > 0 {
		if man.HomeFiles, err = backupTree(filepath.Join(j.Backup, "home"), p.destination, p.Home); err != nil {
			return fail(err)
		}
	}
	if len(p.plugin.m) > 0 {
		man.PluginData = p.pluginData
		if man.PluginFiles, err = backupTree(filepath.Join(j.Backup, "plugin-sessions"), p.plugin, p.pluginData); err != nil {
			return fail(err)
		}
	}
	data, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return fail(err)
	}
	data = append(data, '\n')
	// The manifest comes last: its presence marks the backup complete.
	if err := writeSynced(filepath.Join(j.Backup, "manifest.json"), data); err != nil {
		return fail(err)
	}
	syncDir(j.Backup)
	syncDir(filepath.Dir(j.Backup))
	reached("backed-up")
	// The backup is complete and is never written again after this point.
	if err := p.stage(j.Stage, data); err != nil {
		return fail(err)
	}
	if err := p.unchanged(); err != nil {
		return fail(err)
	}
	j.ManifestSHA = hashBytes(data)
	if j.Published, err = capture(j.Stage); err != nil {
		return fail(err)
	}
	reached("staged")
	j.Phase = phasePublishing
	if err := writeJournal(j); err != nil {
		return fail(err)
	}
	reached("journal-publishing")
	// Keep the original directory until publication succeeds. Ordinary failures
	// restore it; a process death between the two renames is finished or undone by
	// the next run from the journal.
	if j.HadHome {
		if err := os.Rename(p.Home, j.Previous); err != nil {
			return fail(err)
		}
		syncDir(parent)
		reached("displaced")
	}
	if err := os.Rename(j.Stage, p.Home); err != nil {
		if j.HadHome {
			if restoreErr := os.Rename(j.Previous, p.Home); restoreErr != nil {
				return result, fmt.Errorf("migration not published (backup %s): %v; restore %s to %s by hand or rerun grid migrate --apply: %w", j.Backup, err, j.Previous, p.Home, restoreErr)
			}
		}
		return fail(err)
	}
	syncDir(parent)
	reached("published")
	result.Changed = true
	if err := finishPublication(&j); err != nil {
		return result, err
	}
	if j.HadHome {
		if err := os.RemoveAll(j.Previous); err != nil {
			return result, fmt.Errorf("migration published; remove old home %s: %w", j.Previous, err)
		}
	}
	reached("previous-removed")
	if err := removeJournal(p.Home); err != nil {
		return result, fmt.Errorf("migration published; remove %s: %w", journalPath(p.Home), err)
	}
	reached("done")
	return result, nil
}

// backupTree copies a scanned folder into root, checks the copy against the plan by
// reading it back, and returns the digest of every entry.
func backupTree(root string, s snap, base string) (map[string]digest, error) {
	digs, err := writeEntries(root, s.keys, func(rel string) entry {
		e := s.m[rel]
		e.Src = filepath.Join(base, filepath.FromSlash(rel))
		return e
	})
	if err != nil {
		return nil, err
	}
	if err := verifyEntries(root, s, digs, true); err != nil {
		return nil, fmt.Errorf("backup verification failed: %w", err)
	}
	return digs, nil
}

// stage builds the new home beside the old one, with the machine id made once and
// written into the machine file and every record, then adds migration.log.
func (p *Plan) stage(dir string, log []byte) error {
	out := make(tree, len(p.result)+1)
	for k, v := range p.result {
		out[k] = v
	}
	machine := p.machine
	if machine == "" {
		var seed [32]byte
		if _, err := rand.Read(seed[:]); err != nil {
			return err
		}
		machine = legacyID(string(seed[:]))
		out["machine"] = dataEntry([]byte(machine+"\n"), 0600)
	}
	for _, pr := range p.records {
		text, err := pr.render(machine)
		if err != nil {
			return err
		}
		e := out[pr.dst]
		e.Data, e.Size = text, int64(len(text))
		out[pr.dst] = e
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	keys := sortedKeys(out)
	if _, err := writeEntries(dir, keys, func(rel string) entry { return out[rel] }); err != nil {
		return err
	}
	if err := verifyEntries(dir, snap{m: out, keys: keys}, nil, false); err != nil {
		return fmt.Errorf("staged copy verification failed: %w", err)
	}
	if err := writeSynced(filepath.Join(dir, "migration.log"), log); err != nil {
		return err
	}
	syncDir(dir)
	syncDir(filepath.Dir(dir))
	return nil
}
