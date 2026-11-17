package brain

// Person edits adopted (docs/research/brain-obsidian.md 5.1-5.4, 6.1, 7.2): one
// immutable change record per change and content-addressed versions in the
// brain's .grid/, the machine-folder cache and local lock, adoption of what a
// person did in Obsidian, and the redo-on-top of an agent write whose rev is
// stale. It all applies to a vault only (the brain folder key set); the old
// knowledge/ layout keeps log.md and numbered versions.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"go.yaml.in/yaml/v4"
)

// ErrLockBusy means the local brain lock is held by another process. A caller
// that must never wait (a read, the hooks) skips its work instead.
var ErrLockBusy = errors.New("brain lock busy")

var errNoMachineDir = errors.New("no machine folder")

// ChangeRecord is one file of <brain>/.grid/changes/. Hash is the rev of the
// bytes the change left (NoteRev); for a deletion it is the rev of the bytes that
// were deleted.
type ChangeRecord struct {
	At         string `yaml:"at" json:"at"`
	Status     string `yaml:"status" json:"status"` // created, edited, renamed, deleted, proposed
	Level      string `yaml:"level" json:"level"`
	Project    string `yaml:"project,omitempty" json:"project,omitempty"`
	ID         string `yaml:"id" json:"id"`
	OldID      string `yaml:"old_id,omitempty" json:"old_id,omitempty"`
	OriginKind string `yaml:"origin_kind" json:"origin_kind"` // person or agent
	OriginID   string `yaml:"origin_id" json:"origin_id"`
	Author     string `yaml:"author" json:"author"`
	Reason     string `yaml:"reason,omitempty" json:"reason,omitempty"`
	Topic      string `yaml:"topic" json:"topic"`
	Hash       string `yaml:"hash" json:"hash"`
	Task       string `yaml:"task,omitempty" json:"task,omitempty"`
	Target     string `yaml:"target,omitempty" json:"target,omitempty"` // a proposal's note
	Machine    string `yaml:"machine" json:"machine"`

	file string // the record's name, for ordering and the check
}

const (
	gridDir     = ".grid"
	changesDir  = ".grid/changes"
	versionsDir = ".grid/versions"
)

// lvKey names a level of the store as records see it: home, or project/<name>.
func (s Store) lvKey(level string) string {
	if level == "project" {
		return "project/" + s.Project
	}
	return "home"
}

// levelPrefix is the folder of a level below the brain folder, with a trailing
// slash, or empty at home.
func (s Store) levelPrefix(level string) string {
	if level == "project" {
		return "projects/" + s.Project + "/"
	}
	return ""
}

func (s Store) personName() string {
	if s.Author != "" {
		return s.Author
	}
	return "person"
}

func (s Store) nowUTC() time.Time {
	t := time.Now()
	if s.Now != nil {
		t = s.Now()
	}
	return t.UTC().Truncate(time.Millisecond)
}

func topicOfID(id string) string {
	if i := strings.Index(id, "/"); i > 0 {
		return NFC(id[:i])
	}
	return "-"
}

// --- the machine folder: the local lock and the adoption cache (7.2) ---

func (s Store) machineBrainDir() (string, error) {
	base := s.MachineDir
	if base == "" && s.GridHome != "" {
		base = filepath.Join(s.GridHome, ".machine")
	}
	if base == "" || s.BrainDir == "" {
		return "", errNoMachineDir
	}
	sum := sha256.Sum256([]byte(gridhome.Real(s.BrainDir)))
	return filepath.Join(base, "brain", hex.EncodeToString(sum[:8])), nil
}

// lockLocal takes the local lock that orders Grid's writers on this machine. With
// try it returns ErrLockBusy at once instead of waiting. It never keeps anyone out
// of a person's save or an agent's own tool.
func (s Store) lockLocal(try bool) (func(), error) {
	dir, err := s.machineBrainDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "brain.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if try {
		err = fslock.TryLockExclusive(f)
	} else {
		err = fslock.LockExclusive(f)
	}
	if err != nil {
		f.Close()
		if errors.Is(err, fslock.ErrLocked) {
			return nil, ErrLockBusy
		}
		return nil, err
	}
	return func() { fslock.Unlock(f); f.Close() }, nil
}

type seenFile struct {
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
	Hash  string `json:"hash"`
}

// seenCache is files.json: "<level key>\x00<path>" to what adoption last saw.
type seenCache map[string]seenFile

func (s Store) loadSeen() seenCache {
	c := seenCache{}
	dir, err := s.machineBrainDir()
	if err != nil {
		return c
	}
	if b, err := os.ReadFile(filepath.Join(dir, "files.json")); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if c == nil {
		c = seenCache{}
	}
	return c
}

func (s Store) saveSeen(c seenCache) {
	dir, err := s.machineBrainDir()
	if err != nil {
		return
	}
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	tmp := filepath.Join(dir, ".files.json.tmp")
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, filepath.Join(dir, "files.json"))
	}
}

func seenKey(lv, path string) string { return lv + "\x00" + path }

// --- records and versions in <brain>/.grid/ (5.2) ---

func (s Store) openTop() (*os.Root, error) {
	if s.BrainDir == "" {
		return nil, fs.ErrNotExist
	}
	return os.OpenRoot(s.BrainDir)
}

func (s Store) levelDirName(level string) string {
	if level == "project" {
		return "projects/" + s.Project
	}
	return "home"
}

func versionRel(levelDir, id, hash string) string {
	return versionsDir + "/" + levelDir + "/" + id + "/" + hash + ".version"
}

// storeVersion keeps data under its rev. Storing the same bytes twice is a no-op.
func (s Store) storeVersion(top *os.Root, level, id string, data []byte) error {
	rel := versionRel(s.levelDirName(level), id, NoteRev(data))
	if _, err := top.Lstat(filepath.FromSlash(rel)); err == nil {
		return nil
	}
	return atomicWrite(top, filepath.FromSlash(rel), data)
}

func (s Store) readVersion(top *os.Root, level, id, rev string) ([]byte, error) {
	return fs.ReadFile(top.FS(), versionRel(s.levelDirName(level), id, rev))
}

func jsonString(v string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

var machineName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// writeRecord stores one immutable record:
// .grid/changes/<yyyy-mm>/<utc-time>-<machine>-<hash8>.change.
func (s Store) writeRecord(top *os.Root, r ChangeRecord) error {
	t := s.nowUTC()
	r.At = contract.FormatTime(t)
	r.Machine = machineName.ReplaceAllString(s.MachineID, "-")
	if r.Machine == "" {
		r.Machine = "local"
	}
	if r.Project == "" && r.Level == "project" {
		r.Project = s.Project
	}
	var b strings.Builder
	for _, kv := range [][2]string{{"at", r.At}, {"status", r.Status}, {"level", r.Level}, {"project", r.Project},
		{"id", r.ID}, {"old_id", r.OldID}, {"origin_kind", r.OriginKind}, {"origin_id", r.OriginID}, {"author", r.Author},
		{"reason", r.Reason}, {"topic", r.Topic}, {"hash", r.Hash}, {"task", r.Task}, {"target", r.Target}, {"machine", r.Machine}} {
		if kv[1] != "" {
			b.WriteString(kv[0] + ": " + jsonString(kv[1]) + "\n")
		}
	}
	h8 := "00000000"
	if len(r.Hash) >= 8 {
		h8 = r.Hash[:8]
	}
	dir := changesDir + "/" + t.Format("2006-01")
	stem := t.Format("20060102T150405.000Z") + "-" + r.Machine + "-" + h8
	for n := 1; ; n++ {
		name := stem + ".change"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.change", stem, n)
		}
		rel := filepath.FromSlash(dir + "/" + name)
		if _, err := top.Lstat(rel); err == nil {
			continue
		}
		return atomicWrite(top, rel, []byte(b.String()))
	}
}

type recFile struct {
	ChangeRecord
	at   time.Time
	name string
	raw  []byte
}

// readRecords reads every record of the brain, oldest first by time, then machine,
// then file name. A file that does not parse is returned in bad.
func readRecords(top *os.Root) (recs []recFile, bad []string, err error) {
	err = fs.WalkDir(top.FS(), changesDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, fs.ErrNotExist) {
				return nil
			}
			return werr
		}
		if d.IsDir() || !strings.HasSuffix(p, ".change") {
			return nil
		}
		raw, rerr := fs.ReadFile(top.FS(), p)
		if rerr != nil {
			return rerr
		}
		var r ChangeRecord
		at, terr := time.Time{}, error(nil)
		if yerr := yaml.Load(raw, &r); yerr != nil {
			bad = append(bad, p)
			return nil
		}
		if at, terr = time.Parse(contract.TimeLayout, r.At); terr != nil || r.ID == "" || r.Status == "" {
			bad = append(bad, p)
			return nil
		}
		recs = append(recs, recFile{ChangeRecord: r, at: at, name: p, raw: raw})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.SliceStable(recs, func(i, j int) bool {
		if !recs[i].at.Equal(recs[j].at) {
			return recs[i].at.Before(recs[j].at)
		}
		if recs[i].Machine != recs[j].Machine {
			return recs[i].Machine < recs[j].Machine
		}
		return recs[i].name < recs[j].name
	})
	return recs, bad, nil
}

type recKey struct{ lv, id string }

func (s Store) recKeyOf(r ChangeRecord, id string) recKey {
	lv := "home"
	if r.Level == "project" {
		lv = "project/" + r.Project
	}
	return recKey{lv, id}
}

// recState is what the records say: the live notes and their bytes, every rev ever
// recorded for an id, and the newest record for each rev.
type recState struct {
	live   map[recKey]string
	hashes map[recKey]map[string]bool
	byRev  map[recKey]map[string]ChangeRecord
	moved  map[recKey]recKey // old id to new id, from renamed records
}

// logicalChanges drops a record that repeats the one before it for the same note:
// the same hash, status and old id is one change however many machines recorded it
// (5.2). A note that goes A, B, A, B is four changes, not two.
func logicalChanges(recs []recFile) []recFile {
	last := map[string]string{}
	var out []recFile
	for _, r := range recs {
		k := r.Level + "\x00" + r.Project + "\x00" + r.ID
		sig := r.Hash + "\x00" + r.Status + "\x00" + r.OldID
		if last[k] == sig {
			continue
		}
		last[k] = sig
		out = append(out, r)
	}
	return out
}

// foldRecords reads the records as one logical history: two records with the same
// level, id, hash, status and old id are one change (5.2, duplicate adoptions).
func (s Store) foldRecords(recs []recFile) recState {
	st := recState{live: map[recKey]string{}, hashes: map[recKey]map[string]bool{},
		byRev: map[recKey]map[string]ChangeRecord{}, moved: map[recKey]recKey{}}
	for _, r := range logicalChanges(recs) {
		k := s.recKeyOf(r.ChangeRecord, r.ID)
		switch r.Status {
		case "deleted":
			delete(st.live, k)
			continue
		case "renamed":
			old := s.recKeyOf(r.ChangeRecord, r.OldID)
			delete(st.live, old)
			st.moved[old] = k
		}
		st.live[k] = r.Hash
		if st.hashes[k] == nil {
			st.hashes[k] = map[string]bool{}
			st.byRev[k] = map[string]ChangeRecord{}
		}
		st.hashes[k][r.Hash] = true
		// A Grid write (agent) for a path and hash beats an adoption (person),
		// whichever came first (4.3).
		if prev, ok := st.byRev[k][r.Hash]; !ok || prev.OriginKind != "agent" || r.OriginKind == "agent" {
			st.byRev[k][r.Hash] = r.ChangeRecord
		}
	}
	return st
}

// --- the walk and adoption (5.1) ---

type diskFile struct {
	path  string // slash path below the level root
	id    string
	size  int64
	mtime int64
	data  []byte
	hash  string
}

// walkLevel lists the .md files of one level of the vault, Proposals/ included.
// It skips dot-folders and dot-files, projects/ at home level, and anything that
// is not a regular file: adoption never follows a link.
func (s Store) walkLevel(top *os.Root, level string) ([]diskFile, error) {
	start := "."
	if level == "project" {
		if !ValidName(s.Project) {
			return nil, nil
		}
		start = "projects/" + s.Project
	}
	var out []diskFile
	err := fs.WalkDir(top.FS(), start, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, fs.ErrNotExist) {
				return fs.SkipDir
			}
			return werr
		}
		if p == start {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if level == "home" && p == "projects" {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.EqualFold(filepath.Ext(name), ".md") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		rel := p
		if level == "project" {
			rel = strings.TrimPrefix(p, start+"/")
		}
		out = append(out, diskFile{path: rel, id: NFC(rel[:len(rel)-3]), size: info.Size(), mtime: info.ModTime().UnixNano()})
		return nil
	})
	return out, err
}

func (s Store) loadFile(top *os.Root, level string, f *diskFile) error {
	b, err := fs.ReadFile(top.FS(), s.levelPrefix(level)+f.path)
	if err != nil {
		return err
	}
	f.data, f.hash = b, NoteRev(b)
	return nil
}

// Adopt records every person change in the levels the store reads and returns how
// many it recorded (5.1). It takes the local lock and waits for it, as a Grid
// write does. A store without a vault adopts nothing.
func (s Store) Adopt() (int, error) {
	if !s.Vault() {
		return 0, nil
	}
	unlock, err := s.lockLocal(false)
	if errors.Is(err, errNoMachineDir) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer unlock()
	return s.adoptLocked(s.levels())
}

// adoptForRead is Adopt for a read verb: best effort, never waits for the lock and
// never changes what the read returns or fails with.
func (s Store) adoptForRead() {
	if !s.Vault() {
		return
	}
	unlock, err := s.lockLocal(true)
	if err != nil {
		return
	}
	defer unlock()
	_, _ = s.adoptLocked(s.levels())
}

func (s Store) adoptLocked(levels []string) (int, error) {
	top, err := s.openTop()
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer top.Close()
	cache := s.loadSeen()
	total := 0
	for _, level := range levels {
		if level == "project" && !ValidName(s.Project) {
			continue
		}
		n, err := s.adoptLevel(top, level, cache)
		total += n
		if err != nil {
			s.saveSeen(cache)
			return total, err
		}
	}
	s.saveSeen(cache)
	return total, nil
}

func (s Store) adoptLevel(top *os.Root, level string, cache seenCache) (int, error) {
	files, err := s.walkLevel(top, level)
	if err != nil {
		return 0, err
	}
	lv := s.lvKey(level)
	onDisk := map[string]bool{}
	var changed []*diskFile
	for i := range files {
		f := &files[i]
		onDisk[f.id] = true
		if c, ok := cache[seenKey(lv, f.path)]; ok && c.Size == f.size && c.MTime == f.mtime {
			continue
		}
		if err := s.loadFile(top, level, f); err != nil {
			continue
		}
		changed = append(changed, f)
	}
	vanished := false
	for k := range cache {
		if strings.HasPrefix(k, lv+"\x00") && !onDisk[NFC(strings.TrimSuffix(strings.TrimPrefix(k, lv+"\x00"), k[len(k)-3:]))] {
			vanished = true
			break
		}
	}
	if len(changed) == 0 && !vanished {
		return 0, nil
	}
	recs, _, err := readRecords(top)
	if err != nil {
		return 0, err
	}
	st := s.foldRecords(recs)
	// Notes the records call live that are no longer on disk.
	var gone []string
	for k := range st.live {
		if k.lv == lv && !onDisk[k.id] {
			gone = append(gone, k.id)
		}
	}
	sort.Strings(gone)
	var fresh []*diskFile
	for _, f := range changed {
		if st.live[recKey{lv, f.id}] != f.hash {
			fresh = append(fresh, f)
		}
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].id < fresh[j].id })
	n := 0
	base := ChangeRecord{Level: level, OriginKind: "person", OriginID: s.personName(), Author: s.personName()}
	taken := map[string]bool{}
	for _, f := range fresh {
		rec := base
		rec.ID, rec.Hash, rec.Topic = f.id, f.hash, topicOfID(f.id)
		for _, g := range gone {
			if !taken[g] && st.live[recKey{lv, g}] == f.hash {
				taken[g] = true
				rec.Status, rec.OldID = "renamed", g
				break
			}
		}
		if rec.Status == "" {
			rec.Status = "created"
			if _, live := st.live[recKey{lv, f.id}]; live {
				rec.Status = "edited"
			}
		}
		if err := s.storeVersion(top, level, f.id, f.data); err != nil {
			return n, err
		}
		if err := s.writeRecord(top, rec); err != nil {
			return n, err
		}
		n++
	}
	for _, g := range gone {
		if taken[g] {
			continue
		}
		rec := base
		rec.Status, rec.ID, rec.Hash, rec.Topic = "deleted", g, st.live[recKey{lv, g}], topicOfID(g)
		if err := s.writeRecord(top, rec); err != nil {
			return n, err
		}
		n++
	}
	for k := range cache {
		if strings.HasPrefix(k, lv+"\x00") {
			delete(cache, k)
		}
	}
	for _, f := range files {
		h := f.hash
		if h == "" {
			// Unchanged by the cache: keep what it holds.
			continue
		}
		cache[seenKey(lv, f.path)] = seenFile{Size: f.size, MTime: f.mtime, Hash: h}
	}
	return n, nil
}

// AdoptPath adopts one file by its absolute path, for the PreToolUse hook (4.3).
// It never waits: a busy lock is ErrLockBusy and nothing is adopted. A path that
// is not a note of the brain, or a file that does not exist yet, is nothing to
// adopt. The file is always hashed; the size-and-time shortcut is not used.
func (s Store) AdoptPath(abs string) (int, error) {
	level, st, rel, ok := s.vaultPath(abs)
	if !ok {
		return 0, nil
	}
	unlock, err := st.lockLocal(true)
	if err != nil {
		return 0, err
	}
	defer unlock()
	top, err := st.openTop()
	if err != nil {
		return 0, err
	}
	defer top.Close()
	data, err := fs.ReadFile(top.FS(), st.levelPrefix(level)+rel)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	rec, err := st.recordIfNew(top, level, rel, data, ChangeRecord{OriginKind: "person", OriginID: st.personName(), Author: st.personName()}, false)
	if err != nil || !rec {
		return 0, err
	}
	return 1, nil
}

// RecordAgentWrite records the bytes an agent's own Edit or Write left in a note
// (the PostToolUse step, 4.3). Those bytes are agent bytes. It never waits.
func (s Store) RecordAgentWrite(abs, originID string) (int, error) {
	level, st, rel, ok := s.vaultPath(abs)
	if !ok {
		return 0, nil
	}
	unlock, err := st.lockLocal(true)
	if err != nil {
		return 0, err
	}
	defer unlock()
	top, err := st.openTop()
	if err != nil {
		return 0, err
	}
	defer top.Close()
	data, err := fs.ReadFile(top.FS(), st.levelPrefix(level)+rel)
	if err != nil {
		return 0, nil
	}
	rec, err := st.recordIfNew(top, level, rel, data, ChangeRecord{OriginKind: "agent", OriginID: originID, Author: originID}, true)
	if err != nil || !rec {
		return 0, err
	}
	return 1, nil
}

// recordIfNew records data as the current bytes of the note at rel unless a record
// already says so. An agent record is written even when a person adoption of the
// same bytes exists: the Grid write beats the adoption (4.3).
func (s Store) recordIfNew(top *os.Root, level, rel string, data []byte, base ChangeRecord, agent bool) (bool, error) {
	id := NFC(rel[:len(rel)-3])
	lv := s.lvKey(level)
	recs, _, err := readRecords(top)
	if err != nil {
		return false, err
	}
	st := s.foldRecords(recs)
	k := recKey{lv, id}
	rev := NoteRev(data)
	if prev, ok := st.byRev[k][rev]; ok && (!agent || prev.OriginKind == "agent") {
		s.touchSeen(top, level, rel, rev)
		return false, nil
	}
	base.Level, base.ID, base.Hash, base.Topic = level, id, rev, topicOfID(id)
	base.Status = "created"
	if _, live := st.live[k]; live {
		base.Status = "edited"
	}
	if err := s.storeVersion(top, level, id, data); err != nil {
		return false, err
	}
	if err := s.writeRecord(top, base); err != nil {
		return false, err
	}
	s.touchSeen(top, level, rel, rev)
	return true, nil
}

func (s Store) touchSeen(top *os.Root, level, rel, rev string) {
	info, err := top.Stat(filepath.FromSlash(s.levelPrefix(level) + rel))
	if err != nil {
		return
	}
	c := s.loadSeen()
	c[seenKey(s.lvKey(level), rel)] = seenFile{Size: info.Size(), MTime: info.ModTime().UnixNano(), Hash: rev}
	s.saveSeen(c)
}

// vaultPath maps an absolute path to the level, the store for that level, and the
// slash path below the level root. A path below projects/<p>/ is project level of
// p. ok is false outside the vault, for dot-paths and for a file that is not .md.
func (s Store) vaultPath(abs string) (level string, st Store, rel string, ok bool) {
	if !s.Vault() || abs == "" {
		return "", s, "", false
	}
	r := ""
	for _, root := range []string{s.BrainDir, gridhome.Real(s.BrainDir)} {
		for _, p := range []string{abs, gridhome.Real(abs)} {
			if x, err := filepath.Rel(root, p); err == nil && x != "." && x != ".." && !strings.HasPrefix(x, ".."+string(filepath.Separator)) {
				r = x
			}
		}
	}
	if r == "" {
		return "", s, "", false
	}
	r = filepath.ToSlash(r)
	st, level = s, "home"
	if rest, found := strings.CutPrefix(r, "projects/"); found {
		p, below, cut := strings.Cut(rest, "/")
		if !cut || !ValidName(p) {
			return "", s, "", false
		}
		st.Project, level, r = p, "project", below
	}
	for _, part := range strings.Split(r, "/") {
		if strings.HasPrefix(part, ".") || part == "" {
			return "", s, "", false
		}
	}
	if !strings.EqualFold(filepath.Ext(r), ".md") || !utf8Valid(r) {
		return "", s, "", false
	}
	return level, st, r, true
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "") == s }

// --- what the records say about a note (listing, search, rename) ---

type originIndex struct {
	st recState
	s  Store
}

// originIndex reads the records once for a listing or a search.
func (s Store) originIndex() *originIndex {
	top, err := s.openTop()
	if err != nil {
		return nil
	}
	defer top.Close()
	recs, _, err := readRecords(top)
	if err != nil {
		return nil
	}
	return &originIndex{st: s.foldRecords(recs), s: s}
}

// apply sets a row's origin, author and updated from the record whose hash is the
// rev of the file's bytes (4.3). With no such record the bytes are a person's.
func (ix *originIndex) apply(r *ListRow, level, id string) {
	if ix == nil {
		return
	}
	k := recKey{ix.s.lvKey(level), id}
	rec, ok := ix.st.byRev[k][r.Rev]
	if !ok {
		return
	}
	r.Origin = rec.OriginKind
	if rec.OriginKind == "agent" {
		r.Author = rec.OriginID
	} else if rec.Author != "" {
		r.Author = rec.Author
	}
	r.Updated = rec.At
}

// renamedTo follows renamed records from an old id to the id it has now.
func (ix *originIndex) renamedTo(level, id string) (string, bool) {
	if ix == nil {
		return "", false
	}
	k := recKey{ix.s.lvKey(level), id}
	moved := false
	for i := 0; i < 32; i++ {
		next, ok := ix.st.moved[k]
		if !ok || next == k {
			break
		}
		k, moved = next, true
	}
	return k.id, moved
}

// decorate gives each vault row its origin, adds the proposals of Proposals/ and
// sets conflict on a note with a waiting proposal against it (3.4, 6.3).
func (s Store) decorate(rows []ListRow) []ListRow {
	ix := s.originIndex()
	targets := map[string][]string{}
	var extra []ListRow
	top, err := s.openTop()
	if err == nil {
		defer top.Close()
		for _, level := range s.levels() {
			if level == "project" && !ValidName(s.Project) {
				continue
			}
			files, err := s.walkLevel(top, level)
			if err != nil {
				continue
			}
			for i := range files {
				f := &files[i]
				if !strings.HasPrefix(f.id, "Proposals/") || s.loadFile(top, level, f) != nil {
					continue
				}
				vn, perr := ParseVault(f.data)
				// Only a draft with front matter and a status is a proposal; a stray
				// file in Proposals/ is not listed.
				if perr != nil || vn.BadFront || frontString(vn.Front["status"]) == "" {
					continue
				}
				r := s.row(vaultFile{level: level, path: f.path, rel: s.levelPrefix(level) + f.path, id: f.id, kind: "proposal", topic: "Proposals",
					data: f.data, note: vn, mod: time.Unix(0, f.mtime)}, nil)
				r.Task = frontString(vn.Front["task"])
				ix.apply(&r, level, f.id)
				if frontString(vn.Front["status"]) == "proposed" {
					if t := frontString(vn.Front["target"]); t != "" {
						targets["brain:"+level+"/"+NFC(t)] = append(targets["brain:"+level+"/"+NFC(t)], r.ID)
					}
				}
				extra = append(extra, r)
			}
		}
	}
	for i := range rows {
		if lv, id, ok := strings.Cut(strings.TrimPrefix(rows[i].ID, "brain:"), "/"); ok {
			ix.apply(&rows[i], lv, id)
		}
		if c := targets[rows[i].ID]; len(c) > 0 {
			sort.Strings(c)
			rows[i].Conflict = c
		}
	}
	return append(rows, extra...)
}

// --- three-way merge by front-matter key and body section (5.4 step 3) ---

type mUnit struct{ key, text string }

var sectionHead = regexp.MustCompile(`^#{1,2}[ \t]`)

func uniqKey(seen map[string]int, k string) string {
	seen[k]++
	if seen[k] > 1 {
		return fmt.Sprintf("%s#%d", k, seen[k])
	}
	return k
}

func bodyUnits(body string, seen map[string]int) []mUnit {
	lines := strings.SplitAfter(body, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	var out []mUnit
	key, fenced := "body:lead", false
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 || key != "body:lead" {
			out = append(out, mUnit{uniqKey(seen, key), cur.String()})
		}
		cur.Reset()
	}
	for _, l := range lines {
		t := strings.TrimRight(l, "\r\n")
		if strings.HasPrefix(strings.TrimLeft(t, " "), "```") || strings.HasPrefix(strings.TrimLeft(t, " "), "~~~") {
			fenced = !fenced
		}
		if !fenced && sectionHead.MatchString(t) {
			flush()
			key = "body:" + strings.TrimSpace(t)
		}
		cur.WriteString(l)
	}
	flush()
	return out
}

// splitUnits cuts a note into units that join back to its exact bytes: the opening
// line, each front-matter key with the lines up to the next key, the closing line,
// then the body from one heading to the next.
func splitUnits(data []byte) []mUnit {
	seen := map[string]int{}
	var out []mUnit
	if f, ok := splitRaw(data); ok {
		out = append(out, mUnit{"fm:open", f.bom + f.open})
		spans := frontSpans(f.front)
		first := len(f.front)
		if len(spans) > 0 {
			first = spans[0].start
		}
		if first > 0 {
			out = append(out, mUnit{"fm:lead", strings.Join(f.front[:first], "")})
		}
		for i, sp := range spans {
			end := len(f.front)
			if i+1 < len(spans) {
				end = spans[i+1].start
			}
			out = append(out, mUnit{uniqKey(seen, "fm:"+sp.key), strings.Join(f.front[sp.start:end], "")})
		}
		out = append(out, mUnit{"fm:close", f.close})
		return append(out, bodyUnits(f.body, seen)...)
	}
	s := string(data)
	if strings.HasPrefix(s, "\xef\xbb\xbf") {
		out = append(out, mUnit{"bom", s[:3]})
		s = s[3:]
	}
	return append(out, bodyUnits(s, seen)...)
}

type unitMap struct {
	order []string
	text  map[string]string
}

func toMap(us []mUnit) unitMap {
	m := unitMap{text: map[string]string{}}
	for _, u := range us {
		m.order = append(m.order, u.key)
		m.text[u.key] = u.text
	}
	return m
}

// merge3 redoes the agent's change on top of the current bytes. base is what the
// agent read, agent what it wrote, cur what is on disk now. A unit (a front-matter
// key or a body section) only one side changed takes that side; a unit both
// changed differently is a conflict, returned by name with ok false.
func merge3(base, agent, cur []byte) (out []byte, conflict string, ok bool) {
	b, a, c := toMap(splitUnits(base)), toMap(splitUnits(agent)), toMap(splitUnits(cur))
	type res struct{ key, text string }
	var result []res
	keep := map[string]bool{}
	for _, k := range c.order {
		av, aok := a.text[k]
		bv, bok := b.text[k]
		cv := c.text[k]
		switch {
		case aok && av == cv, aok == bok && av == bv:
			result = append(result, res{k, cv})
		case bv == cv && bok:
			if aok {
				result = append(result, res{k, av})
			}
		default:
			return nil, strings.TrimPrefix(strings.TrimPrefix(k, "fm:"), "body:"), false
		}
		keep[k] = true
	}
	for i, k := range a.order {
		if _, inC := c.text[k]; inC {
			continue
		}
		if _, inB := b.text[k]; inB {
			// The agent kept or changed what the person deleted.
			if b.text[k] != a.text[k] {
				return nil, strings.TrimPrefix(strings.TrimPrefix(k, "fm:"), "body:"), false
			}
			continue
		}
		at := 0
		for j := i - 1; j >= 0; j-- {
			found := -1
			for x, r := range result {
				if r.key == a.order[j] {
					found = x
				}
			}
			if found >= 0 {
				at = found + 1
				break
			}
		}
		result = append(result[:at], append([]res{{k, a.text[k]}}, result[at:]...)...)
	}
	var buf strings.Builder
	for i, r := range result {
		buf.WriteString(r.text)
		if i+1 < len(result) && r.text != "" && !strings.HasSuffix(r.text, "\n") {
			buf.WriteString("\n")
		}
	}
	return []byte(buf.String()), "", true
}

// CheckGridIntegrity is the finding for a .grid/ file that does not read as what
// it should be: a record that does not parse, or a record whose version is gone.
// It is the only drift finding left for a vault: a person's edit is adopted, not
// reported (7.1).
const CheckGridIntegrity = "grid_integrity"

// CheckSecretShaped is the finding for a line of a note that looks like a secret.
// It names the file and the line only: the finding never carries the line.
const CheckSecretShaped = "secret-shaped"

// vaultCheck adopts what a person changed, then reports what is wrong in the
// vault. It writes no proposal and never changes a note.
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

// --- the version check of a Grid write to a vault (5.4) ---

// vaultNoteID checks the id of a note an agent writes: a readable path id with no
// .md, no hidden part and not below Proposals/ (those are written by Grid only).
func vaultNoteID(id string) bool {
	if !ValidID(id) || strings.HasSuffix(strings.ToLower(id), ".md") || strings.HasPrefix(id, "Proposals/") || id == "Proposals" {
		return false
	}
	for _, p := range strings.Split(id, "/") {
		if strings.HasPrefix(p, ".") {
			return false
		}
	}
	return true
}

// secretLineNotIn reports whether data holds a secret-shaped line that old does
// not: a line a person already had in their note is theirs, never refused.
func secretLineNotIn(old, data []byte) bool {
	had := map[string]bool{}
	for _, l := range strings.Split(string(old), "\n") {
		had[l] = true
	}
	for _, l := range strings.Split(string(data), "\n") {
		if !had[l] && LooksSecret(l) {
			return true
		}
	}
	return false
}

// vaultWrite is Write for a vault. Under the local lock it adopts what a person
// changed, then compares the rev the caller read. On a match the caller's bytes
// are written. On a mismatch the caller's change is redone on top of the current
// bytes, unit by unit; only a front-matter key or a body section both sides
// changed, differently, makes a proposal in Proposals/ and leaves the note alone.
func (s Store) vaultWrite(level, topic, name, expect string, data []byte, origin Origin, reason string) (WriteResult, error) {
	id := NFC(topic + "/" + name)
	if !vaultNoteID(id) {
		return WriteResult{}, keWrInvalid("%q is not a note id", id)
	}
	if level == "project" && !ValidName(s.Project) {
		return WriteResult{}, fmt.Errorf("level project needs a project")
	}
	if !keWrRevForm(expect) {
		return WriteResult{}, fmt.Errorf("expect must be none or a rev of 16 lower-case hex digits")
	}
	if len(data) > keWrMaxNote {
		return WriteResult{}, keWrInvalid("a note is at most 1 MiB")
	}
	if origin.Kind == "" || strings.TrimSpace(origin.ID) == "" {
		return WriteResult{}, fmt.Errorf("the change record needs an origin")
	}
	unlock, err := s.lockLocal(false)
	if err != nil {
		return WriteResult{}, err
	}
	defer unlock()
	if _, err := s.adoptLocked([]string{level}); err != nil {
		return WriteResult{}, err
	}
	top, err := s.openTop()
	if err != nil {
		return WriteResult{}, err
	}
	defer top.Close()
	rel := s.levelPrefix(level) + id + ".md"
	osRel := filepath.FromSlash(rel)
	if err := keWrNoLink(top, osRel); err != nil {
		return WriteResult{}, err
	}
	var cur []byte
	exists := false
	info, err := top.Lstat(osRel)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return WriteResult{}, err
	case !info.Mode().IsRegular():
		return WriteResult{}, fmt.Errorf("%s: %w", rel, ErrLinkedTarget)
	default:
		if cur, err = fs.ReadFile(top.FS(), rel); err != nil {
			return WriteResult{}, err
		}
		exists = true
	}
	curRev := "none"
	if exists {
		curRev = NoteRev(cur)
	}
	final := data
	if curRev != expect {
		var base []byte
		if expect != "none" {
			b, err := s.readVersion(top, level, id, expect)
			if err != nil {
				// What the caller read was never recorded, so its change cannot be
				// told from the person's: the old answer, a stale rev.
				return WriteResult{}, &ConflictError{Rev: curRev}
			}
			base = b
		}
		if !exists {
			return s.vaultPropose(top, level, id, expect, curRev, data, origin, reason, "the note was deleted")
		}
		merged, key, ok := merge3(base, data, cur)
		if !ok {
			return s.vaultPropose(top, level, id, expect, curRev, data, origin, reason, key)
		}
		final = merged
	}
	if secretLineNotIn(cur, final) {
		return WriteResult{}, keWrInvalid("a line looks like a secret")
	}
	res := WriteResult{Status: "edited", Rev: NoteRev(final), Path: filepath.Join(s.BrainDir, osRel)}
	if !exists {
		res.Status = "created"
	}
	if exists && bytes.Equal(final, cur) {
		res.Status = "unchanged"
		return res, nil
	}
	if err := s.storeVersion(top, level, id, final); err != nil {
		return WriteResult{}, err
	}
	if err := atomicWrite(top, osRel, final); err != nil {
		return WriteResult{}, err
	}
	// Whoever runs the verb, the bytes came from Grid's writer; a person's edits
	// come in by adoption only.
	if err := s.writeRecord(top, ChangeRecord{Level: level, Status: res.Status, ID: id, OriginKind: "agent", OriginID: origin.ID,
		Author: origin.ID, Reason: reason, Topic: topicOfID(id), Hash: res.Rev}); err != nil {
		return WriteResult{}, err
	}
	s.touchSeen(top, level, id+".md", res.Rev)
	return res, nil
}

// fenceFor is a code fence longer than any run of backticks in text.
func fenceFor(text string) string {
	n, run := 3, 0
	for _, r := range text {
		if r == '`' {
			run++
			if run >= n {
				n = run + 1
			}
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", n)
}

// vaultPropose keeps a change that cannot be redone on top of the person's bytes as
// a draft in Proposals/ and leaves the note exactly as it is.
func (s Store) vaultPropose(top *os.Root, level, id, expect, curRev string, data []byte, origin Origin, reason, key string) (WriteResult, error) {
	last := id[strings.LastIndex(id, "/")+1:]
	stem := last + "-" + NoteRev(data)[:8]
	var b strings.Builder
	b.WriteString("---\nstatus: proposed\n")
	for _, kv := range [][2]string{{"target", id}, {"level", level}, {"base", expect}, {"current", curRev},
		{"conflict", key}, {"author", origin.ID}, {"reason", reason}} {
		if kv[1] != "" {
			b.WriteString(kv[0] + ": " + jsonString(kv[1]) + "\n")
		}
	}
	text := string(data)
	fence := fenceFor(text)
	b.WriteString("---\n# Proposed change to " + last + "\n\nThe note changed while this change was being made, and both cannot hold")
	if key != "" {
		b.WriteString(" (" + key + ")")
	}
	b.WriteString(". The note was not changed. The agent's version of the whole note:\n\n" + fence + "markdown\n" + text)
	if !strings.HasSuffix(text, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(fence + "\n")
	out := []byte(b.String())
	pid := "Proposals/" + stem
	for n := 2; ; n++ {
		have, err := fs.ReadFile(top.FS(), s.levelPrefix(level)+pid+".md")
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err == nil && bytes.Equal(have, out) {
			break
		}
		pid = fmt.Sprintf("Proposals/%s-%d", stem, n)
	}
	prel := s.levelPrefix(level) + pid + ".md"
	if err := s.storeVersion(top, level, pid, out); err != nil {
		return WriteResult{}, err
	}
	if err := atomicWrite(top, filepath.FromSlash(prel), out); err != nil {
		return WriteResult{}, err
	}
	if err := s.writeRecord(top, ChangeRecord{Level: level, Status: "proposed", ID: pid, OriginKind: "agent", OriginID: origin.ID,
		Author: origin.ID, Reason: reason, Topic: "Proposals", Hash: NoteRev(out), Target: id}); err != nil {
		return WriteResult{}, err
	}
	s.touchSeen(top, level, pid+".md", NoteRev(out))
	return WriteResult{Status: "proposed", Rev: curRev, Proposed: filepath.Join(s.BrainDir, filepath.FromSlash(prel))}, nil
}
