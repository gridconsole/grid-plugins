package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// The completed journal stays outside the home, so even edits to migration.log
// cannot erase the evidence used to refuse a destructive rollback.
func statePath(home string) string { return home + ".migration-state" }

func capture(root string) (map[string]digest, error) {
	s, err := snapshot(root)
	if err != nil {
		return nil, err
	}
	out := make(map[string]digest, len(s.m))
	for _, rel := range s.keys {
		e := s.m[rel]
		d := digest{Mode: uint32(e.Mode), Size: e.Size, Link: e.Link, ModTime: e.ModTime}
		if e.Mode.IsDir() {
			info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, err
			}
			d.ModTime = info.ModTime().UnixNano()
		}
		if e.Mode.IsRegular() {
			d.SHA256, err = hashFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, err
			}
		}
		out[rel] = d
	}
	return out, nil
}

// verifyDigests compares both the path set and file contents. Published files
// also carry mtimes, so rewriting identical bytes still prevents rollback.
func verifyDigests(root string, digs map[string]digest, times bool) error {
	want := tree{}
	for rel, d := range digs {
		want[rel] = entry{Mode: os.FileMode(d.Mode), Size: d.Size, Link: d.Link, ModTime: d.ModTime}
	}
	s := snap{m: want, keys: sortedKeys(want)}
	if err := verifyEntries(root, s, digs, true); err != nil {
		return err
	}
	if times {
		actual, err := snapshot(root)
		if err != nil {
			return err
		}
		for rel, d := range digs {
			if os.FileMode(d.Mode).IsDir() && d.ModTime != 0 {
				info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
				if err != nil {
					return err
				}
				if info.ModTime().UnixNano() != d.ModTime {
					return fmt.Errorf("directory metadata changed in %s", root)
				}
			}
		}
		if !equalMeta(actual, s) {
			return fmt.Errorf("file metadata changed in %s", root)
		}
	}
	return nil
}

// finishPublication does what follows the swap: it repairs the pointers outside the
// home and writes or removes the rollback receipt. A rerun repeats it after a crash and
// never refuses over a home written since, which would leave the journal stuck for good;
// a home that changed only gets no receipt, so rollback is not offered for it, and a
// rollback leaves it alone.
func finishPublication(j *journal) error {
	if j.Rollback && exists(j.Previous) && !j.PreviousVerified {
		if err := verifyDigests(j.Previous, j.Before, true); err != nil {
			return fmt.Errorf("rollback refused: displaced new home was written after migration: %w", err)
		}
		// Once verified, the displaced home may be deleted piece by piece; a crash in
		// the middle leaves a part of it that must not be checked again.
		j.PreviousVerified = true
		if err := writeJournal(*j); err != nil {
			return err
		}
		reached("previous-verified")
	}
	intact := true
	if len(j.Published) > 0 && !(j.RemoveHome && !exists(j.Home)) {
		intact = verifyDigests(j.Home, j.Published, true) == nil
	}
	if !j.Rollback {
		// The worktrees must be where the pointers will name them before the pointers change.
		if err := placeExternals(j.Externals); err != nil {
			return err
		}
	}
	if err := applyPointers(j.Pointers); err != nil {
		return err
	}
	if j.Rollback && j.ExternalsVerified {
		// The pointers name the old place again; the copies in the machine folder were
		// checked unchanged before the swap and are removed now. A rerun repeats this.
		for _, x := range j.Externals {
			if err := os.RemoveAll(x.Target); err != nil {
				return err
			}
		}
		reached("worktrees-removed")
	}
	if j.Rollback {
		if j.RemoveHome && intact && exists(j.Home) {
			// Only the empty directory that we staged may be removed here.
			if err := os.Remove(j.Home); err != nil {
				return err
			}
		}
		if err := os.Remove(statePath(j.Home)); err != nil && !os.IsNotExist(err) {
			return err
		}
		syncDir(filepath.Dir(j.Home))
	} else if intact {
		data, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return err
		}
		if err := atomicfile.Write(statePath(j.Home), append(data, '\n'), 0o600); err != nil {
			return err
		}
	}
	reached("receipt-written")
	return nil
}

// Rollback restores the home that existed before migration. It never modifies
// the backup or an external --from source. Offline writers must remain stopped.
// Changes since publication are refused, including added files and same-content
// rewrites. An interrupted rollback is recovered through the ordinary journal.
func Rollback(home string) (Result, error) {
	home = gridhome.Real(home)
	lk, err := acquireLock(lockPath(home))
	if err != nil {
		return Result{}, err
	}
	defer lk.release()
	reached("rollback-locked")
	if rec, err := findRecovery(home); err != nil {
		return Result{}, err
	} else if rec != nil {
		if !rec.j.Rollback {
			return Result{}, fmt.Errorf("finish the interrupted migration with grid migrate --apply before rollback")
		}
		return rec.run()
	}
	raw, err := os.ReadFile(statePath(home))
	if err != nil {
		return Result{}, fmt.Errorf("no completed migration journal available for rollback: %w", err)
	}
	var old journal
	if err := json.Unmarshal(raw, &old); err != nil || !validJournal(old, home) || old.Rollback || len(old.Published) == 0 {
		return Result{}, fmt.Errorf("invalid completed migration journal; nothing restored")
	}
	if err := verifyDigests(home, old.Published, true); err != nil {
		return Result{}, fmt.Errorf("rollback refused: new home was written after migration: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(old.Backup, "manifest.json"))
	if err != nil {
		return Result{}, fmt.Errorf("rollback backup unavailable: %w", err)
	}
	if hashBytes(data) != old.ManifestSHA {
		return Result{}, fmt.Errorf("rollback backup manifest checksum differs")
	}
	var man manifest
	if err := json.Unmarshal(data, &man); err != nil {
		return Result{}, err
	}
	if man.Home != home || man.Backup != old.Backup || man.Source != old.Source {
		return Result{}, fmt.Errorf("rollback backup does not match journal")
	}
	for kind, digs := range map[string]map[string]digest{"source": man.SourceFiles, "home": man.HomeFiles, "plugin-sessions": man.PluginFiles} {
		if len(digs) > 0 {
			if err := verifyDigests(filepath.Join(old.Backup, kind), digs, false); err != nil {
				return Result{}, fmt.Errorf("rollback backup checksum verification: %w", err)
			}
		}
	}
	if err := verifyExternals(old.Externals); err != nil {
		return Result{}, err
	}
	token := randomToken()
	j := journal{Version: 1, Phase: phasePreparing, Source: old.Source, Home: home, Backup: old.Backup,
		Stage: home + ".migrate-stage-" + token, Previous: home + ".migrate-previous-" + token,
		HadHome: true, Rollback: true, RemoveHome: !old.HadHome, Before: old.Published}
	for _, x := range old.Externals {
		x.Stage = "" // built and placed by the migration; a rollback only removes the target
		j.Externals = append(j.Externals, x)
	}
	for _, c := range old.Pointers {
		c.Before, c.After = c.After, c.Before
		j.Pointers = append(j.Pointers, c)
	}
	if err := checkPointers(j.Pointers, false); err != nil {
		return Result{}, err
	}
	if err := writeJournal(j); err != nil {
		return Result{}, err
	}
	reached("rollback-preparing")
	// From here failures leave the journal intact; a retry discards an incomplete
	// stage or finishes the publication, exactly as it does for migration.
	if j.RemoveHome {
		if err := os.Mkdir(j.Stage, 0700); err != nil {
			return Result{}, err
		}
	} else {
		kind, digs := "home", man.HomeFiles
		if old.Source == home {
			kind, digs = "source", man.SourceFiles
		}
		source := filepath.Join(old.Backup, kind)
		s, err := snapshot(source)
		if err != nil {
			return Result{}, err
		}
		if _, err := backupTree(j.Stage, s, source); err != nil {
			return Result{}, err
		}
		if err := verifyDigests(j.Stage, digs, false); err != nil {
			return Result{}, err
		}
	}
	j.Published, err = capture(j.Stage)
	if err != nil {
		return Result{}, err
	}
	reached("rollback-staged")
	// Check again after the potentially long copy, before any displacement.
	if err := verifyDigests(home, old.Published, true); err != nil {
		return Result{}, fmt.Errorf("rollback refused: new home was written after migration: %w", err)
	}
	if err := checkPointers(j.Pointers, false); err != nil {
		return Result{}, err
	}
	if err := verifyExternals(j.Externals); err != nil {
		return Result{}, err
	}
	j.ExternalsVerified = len(j.Externals) > 0
	j.Phase = phasePublishing
	if err := writeJournal(j); err != nil {
		return Result{}, err
	}
	reached("rollback-publishing")
	if err := os.Rename(home, j.Previous); err != nil {
		return Result{}, err
	}
	syncDir(filepath.Dir(home))
	reached("rollback-displaced")
	if err := os.Rename(j.Stage, home); err != nil {
		return Result{}, err
	}
	syncDir(filepath.Dir(home))
	reached("rollback-published")
	if err := finishPublication(&j); err != nil {
		return Result{}, err
	}
	if err := os.RemoveAll(j.Previous); err != nil {
		return Result{}, err
	}
	reached("rollback-previous-removed")
	if err := removeJournal(home); err != nil {
		return Result{}, err
	}
	reached("rollback-done")
	return Result{Backup: j.Backup, Changed: true}, nil
}

// verifyExternals checks that the worktree folders in the machine folder still hold
// what the migration put there. A worktree used since is work a rollback must not delete.
func verifyExternals(xs []externalMove) error {
	for _, x := range xs {
		if err := verifyDigests(x.Target, x.Digests, true); err != nil {
			return fmt.Errorf("rollback refused: worktrees in the machine folder were written after migration (%s): %w", x.Target, err)
		}
	}
	return nil
}

// validExternals restricts what a journal may name outside the home: a folder one
// level below a folder called worktrees, with a plain name, not inside the home, and
// a stage folder beside it made by this migrator.
func validExternals(xs []externalMove, home string) bool {
	for _, x := range xs {
		dir, name := filepath.Split(x.Target)
		dir = filepath.Clean(dir)
		if !filepath.IsAbs(x.Target) || filepath.Clean(x.Target) != x.Target || filepath.Base(dir) != "worktrees" || name == "" || name == "." || name == ".." ||
			within(x.Target, home) || within(home, x.Target) {
			return false
		}
		if x.Stage != "" && (filepath.Clean(x.Stage) != x.Stage || filepath.Dir(x.Stage) != dir || !strings.HasPrefix(filepath.Base(x.Stage), externalStagePrefix) || len(filepath.Base(x.Stage)) <= len(externalStagePrefix)) {
			return false
		}
	}
	return true
}

// Restrict mutation targets to the siblings allocated by this migrator. A corrupt
// journal must not turn recovery into deletion of arbitrary paths.
func validJournal(j journal, home string) bool {
	sibling := func(p, prefix string) bool {
		return filepath.Clean(p) == p && filepath.Dir(p) == filepath.Dir(home) && strings.HasPrefix(p, home+prefix) && len(p) > len(home+prefix)
	}
	return validExternals(j.Externals, home) && j.Version == 1 && j.Home == home && filepath.IsAbs(j.Source) &&
		(j.Phase == phasePreparing || j.Phase == phasePublishing) &&
		sibling(j.Stage, ".migrate-stage-") && sibling(j.Previous, ".migrate-previous-") &&
		filepath.Clean(j.Backup) == j.Backup && filepath.Dir(j.Backup) == home+"-backup"
}

// RestoredFormat describes the version now present, including a pre-version home.
func RestoredFormat(home string) (string, error) {
	data, err := os.ReadFile(filepath.Join(home, "VERSION"))
	if errors.Is(err, os.ErrNotExist) {
		return "unversioned", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
