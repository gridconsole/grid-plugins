package brain

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// CheckAuditHash is the finding for a published entry whose bytes no longer
// match the hash the keeper logged when it published them.
const CheckAuditHash = "audit_hash"

// AuditHashCheck is the replaceable host policy (key brain_audit_hash_levels,
// default home and project) for which levels compare entry hashes with the log.
var AuditHashCheck = func(level string) bool { return level == "home" || level == "project" }

// CheckGridIntegrity is the finding for a .grid/ file that does not read as what
// it should be (5.3): a record that does not parse or was edited after it was
// written (its hash is in its name), a version whose bytes do not hash to its file
// name, or a record whose version is gone. It is the only drift finding a vault
// has: a person's edit to a note is adopted by check, never reported.
const CheckGridIntegrity = "grid_integrity"

// CheckSecretShaped is the finding for a line of a note that looks like a secret.
// It names the file and the line only: the finding never carries the line.
const CheckSecretShaped = "secret-shaped"

// recordNameHolds reports whether a record file name (without its folder) carries
// the first 8 hex digits of hash, as writeRecord names it, with or without the
// -<n> it adds when two records share a name.
func recordNameHolds(file, hash string) bool {
	h8 := "00000000"
	if len(hash) >= 8 {
		h8 = hash[:8]
	}
	base := strings.TrimSuffix(file, ".change")
	if strings.HasSuffix(base, "-"+h8) {
		return true
	}
	i := strings.LastIndex(base, "-")
	if i <= 0 || i+1 == len(base) || !strings.HasSuffix(base[:i], "-"+h8) {
		return false
	}
	return strings.Trim(base[i+1:], "0123456789") == ""
}

// vaultCheck is the drift check of a vault, in place of auditDrift: it adopts what
// a person changed, which gives a count and no finding, then reports what is wrong
// under .grid/ and the secret-shaped lines. It writes no proposal and never changes
// a note.
func (s Store) vaultCheck(res *CheckResult) error {
	n, err := s.Adopt()
	if err != nil {
		return err
	}
	res.Adopted = n
	top, err := s.openTop()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer top.Close()
	recs, bad, err := readRecords(top)
	if err != nil {
		return err
	}
	for _, p := range bad {
		res.Findings = append(res.Findings, Finding{Code: CheckGridIntegrity, File: p, Message: "change record does not parse"})
	}
	for _, r := range recs {
		if !recordNameHolds(path.Base(r.name), r.Hash) {
			res.Findings = append(res.Findings, Finding{Code: CheckGridIntegrity, File: r.name, Message: "change record was edited: its hash is not the one in its name"})
		}
	}
	err = fs.WalkDir(top.FS(), versionsDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, fs.ErrNotExist) {
				return nil
			}
			return werr
		}
		if d.IsDir() || !strings.HasSuffix(p, ".version") {
			return nil
		}
		b, rerr := fs.ReadFile(top.FS(), p)
		if rerr != nil {
			return rerr
		}
		if NoteRev(b) != strings.TrimSuffix(path.Base(p), ".version") {
			res.Findings = append(res.Findings, Finding{Code: CheckGridIntegrity, File: p, Message: "version bytes do not hash to its file name"})
		}
		return nil
	})
	if err != nil {
		return err
	}
	st := s.foldRecords(recs)
	var missing []string
	for k, h := range st.live {
		lvl, lvDir := "home", "home"
		if strings.HasPrefix(k.lv, "project/") {
			lvl, lvDir = "project", k.lv[len("project/"):]
			s2 := s
			s2.Project = lvDir
			if _, err := s2.readVersion(top, lvl, k.id, h); err != nil {
				missing = append(missing, versionRel("projects/"+lvDir, k.id, h))
			}
			continue
		}
		if _, err := s.readVersion(top, lvl, k.id, h); err != nil {
			missing = append(missing, versionRel(lvDir, k.id, h))
		}
	}
	sort.Strings(missing)
	for _, p := range missing {
		res.Findings = append(res.Findings, Finding{Code: CheckGridIntegrity, File: p, Message: "version named by a change record is missing"})
	}
	for _, level := range s.levels() {
		if level == "project" && !ValidName(s.Project) {
			continue
		}
		files, err := s.walkLevel(top, level)
		if err != nil {
			return err
		}
		for i := range files {
			f := &files[i]
			if s.loadFile(top, level, f) != nil {
				continue
			}
			res.Checked++
			for ln, line := range strings.Split(string(f.data), "\n") {
				if LooksSecret(line) {
					res.add(CheckSecretShaped, s.levelPrefix(level)+f.path, ln+1, "line looks like a secret; the note is unchanged", "")
				}
			}
		}
	}
	return nil
}

type auditKey struct{ topic, name string }

type auditTip struct {
	c  Change
	at time.Time
}

// auditDrift compares each published entry with the newest hashed audit line
// for it. The log is streamed like Changes, never read whole. Lines written
// before hashes existed, rejections, and lines that do not parse give no hash,
// so they never raise a finding; an entry that is gone is not drift.
func auditDrift(root *os.Root, level string, res *CheckResult) error {
	f, err := openLog(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	tips := map[auditKey]auditTip{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), maxLogLine)
	for sc.Scan() {
		c, at, ok := parseAudit(sc.Text())
		// A rejection publishes nothing, so its hash never stands for an entry.
		if !ok || c.Status == "rejected" || c.Hash == "" || c.Topic == "" {
			continue
		}
		k := auditKey{c.Topic, c.Name}
		// Newest as Changes orders it: by stamp, a later line when stamps tie.
		if prev, seen := tips[k]; seen && at.Before(prev.at) {
			continue
		}
		tips[k] = auditTip{c, at}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("brain log: %w", err)
	}
	keys := make([]auditKey, 0, len(tips))
	for k := range tips {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].topic != keys[j].topic {
			return keys[i].topic < keys[j].topic
		}
		return keys[i].name < keys[j].name
	})
	for _, k := range keys {
		if !ValidName(k.topic) || !ValidName(k.name) {
			continue
		}
		rel := "knowledge/" + k.topic + "/" + k.name + ".md"
		b, err := safefile.Read(root, filepath.FromSlash(rel))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("entry %s: %w", rel, err)
		}
		if now := auditHash(b); now != tips[k].c.Hash {
			res.add(CheckAuditHash, scoped(level, rel), 0,
				fmt.Sprintf("entry changed outside the keeper: v%d was %s, now %s", tips[k].c.Version, tips[k].c.Hash, now), "")
		}
	}
	return nil
}
