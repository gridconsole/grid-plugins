package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Record is one registered account. The registry file holds these and nothing
// else: an email, a name and a folder, never a secret.
type Record struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Dir   string `json:"dir"`   // ~/.<id> spelled with a leading ~/ so the file survives a moved home
	Added string `json:"added"` // "auto" when Grid saw the email, "person" when they added it
}

type registryFile struct {
	Accounts []Record `json:"accounts"`
}

// Account is one entry of the account_status shape.
type Account struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Dir      string `json:"dir"`
	Default  bool   `json:"default"`
	SignedIn *bool  `json:"signedIn"`
	Checked  string `json:"checked"`
	Plan     string `json:"plan"`
	Added    string `json:"added"`
}

func (e Env) tilde(path string) string {
	if e.Home == "" {
		return path
	}
	rel, err := filepath.Rel(filepath.Clean(e.Home), path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	if rel == "." {
		return "~"
	}
	return "~/" + filepath.ToSlash(rel)
}

func (e Env) expand(p string) string {
	if p == "~" {
		return e.Home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(e.Home, filepath.FromSlash(p[2:]))
	}
	if e.goos() == "windows" && strings.HasPrefix(p, `~\`) {
		parts := strings.FieldsFunc(p[2:], func(r rune) bool { return r == '\\' || r == '/' })
		return filepath.Join(append([]string{e.Home}, parts...)...)
	}
	return filepath.Clean(p)
}

// pathKey is the key two spellings of a folder are compared by: the cleaned
// path, lower-cased on macOS and Windows, whose default volumes treat
// ~/.Claude-ann and ~/.claude-ann as one folder. A case-sensitive APFS volume
// would see two folders that differ only in case as one; that is accepted.
// Linux compares the path as spelled.
func (e Env) pathKey(p string) string {
	p = filepath.Clean(p)
	if g := e.goos(); g == "darwin" || g == "windows" {
		return strings.ToLower(p)
	}
	return p
}

func registryPath(env Env) string {
	return filepath.Join(env.GridHome, "accounts", "registry.json")
}

func loadRecords(env Env) ([]Record, error) {
	data, err := os.ReadFile(registryPath(env))
	if errors.Is(err, fs.ErrNotExist) {
		return []Record{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f registryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s is not readable JSON (%v); it was left as it is", registryPath(env), err)
	}
	if f.Accounts == nil {
		f.Accounts = []Record{}
	}
	return f.Accounts, nil
}

func saveRecords(env Env, recs []Record) error {
	data, err := json.MarshalIndent(registryFile{recs}, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(registryPath(env), append(data, '\n'), 0o600)
}

// lockRegistry serialises registrations, so one email is registered once even
// when two calls scan at the same moment.
func lockRegistry(env Env) (func(), error) {
	dir := filepath.Join(env.GridHome, "accounts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "registry.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() { fslock.Unlock(f); f.Close() }, nil
}

var slugRuns = regexp.MustCompile(`[^a-z0-9]+`)

func slugOf(email string) string {
	local, _, _ := strings.Cut(email, "@")
	s := strings.Trim(slugRuns.ReplaceAllString(strings.ToLower(local), "-"), "-")
	if s == "" {
		return "account"
	}
	return s
}

func nameFor(email string) string {
	if len(email) > 40 {
		local, _, _ := strings.Cut(email, "@")
		return local
	}
	return email
}

func findByEmail(recs []Record, kind, email string) int {
	for i, r := range recs {
		if r.Kind == kind && strings.EqualFold(r.Email, email) {
			return i
		}
	}
	return -1
}

func newID(recs []Record, kind, email string) string {
	base := kind + "-" + slugOf(email)
	id := base
	for n := 2; ; n++ {
		taken := false
		for _, r := range recs {
			taken = taken || r.ID == id
		}
		if !taken {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

// registerNew adds a record for every scanned email no record has, once. A new
// email seen on a home folder gets its own folder ~/.<id> with the config linked
// in; a folder of its own that already carries the email is recorded as it is.
func registerNew(env Env, recs []Record, folders []Folder) ([]Record, bool, error) {
	changed := false
	for _, f := range folders {
		if f.Email == "" || findByEmail(recs, f.Kind, f.Email) >= 0 {
			continue
		}
		id := newID(recs, f.Kind, f.Email)
		dir := f.Dir
		used := false
		for _, r := range recs {
			used = used || env.pathKey(env.expand(r.Dir)) == env.pathKey(f.Dir)
		}
		if f.Home || used {
			res, err := MakeFolder(env, f.Kind, id)
			if err != nil {
				return recs, changed, err
			}
			dir = res.Dir
		}
		recs = append(recs, Record{ID: id, Kind: f.Kind, Name: nameFor(f.Email), Email: f.Email, Dir: env.tilde(dir), Added: "auto"})
		changed = true
	}
	return recs, changed, nil
}

// foldRecords groups the records that are one account: the same kind, and the
// same folder, the same id or the same login email (letters of any case). The
// registry can hold such twins (an old record and one made from the email's other
// spelling, or a record a person added for a folder that was already known), and
// each used to show as a row of its own. The groups come in the registry order of
// their first record. In a group the record whose own folder is signed in comes
// first, because that is the one that can be read; otherwise the first record
// does. Nothing here writes the registry: a read folds, the file is left as it is.
func foldRecords(env Env, recs []Record, byDir map[string]Folder) [][]Record {
	root := make([]int, len(recs))
	for i := range root {
		root[i] = i
	}
	find := func(i int) int {
		for root[i] != i {
			root[i] = root[root[i]]
			i = root[i]
		}
		return i
	}
	same := func(a, b Record) bool {
		if a.Kind != b.Kind {
			return false
		}
		if a.ID == b.ID || env.pathKey(env.expand(a.Dir)) == env.pathKey(env.expand(b.Dir)) {
			return true
		}
		ea, eb := normEmail(a.Email), normEmail(b.Email)
		return ea != "" && ea == eb
	}
	for i := range recs {
		for j := i + 1; j < len(recs); j++ {
			if same(recs[i], recs[j]) {
				root[find(j)] = find(i)
			}
		}
	}
	var order []int
	members := map[int][]Record{}
	for i, r := range recs {
		k := find(i)
		if _, ok := members[k]; !ok {
			order = append(order, k)
		}
		members[k] = append(members[k], r)
	}
	out := make([][]Record, 0, len(order))
	for _, k := range order {
		g := members[k]
		for i, r := range g {
			if f, ok := byDir[env.pathKey(env.expand(r.Dir))]; ok && f.SignedIn != nil && *f.SignedIn {
				g[0], g[i] = g[i], g[0]
				break
			}
		}
		out = append(out, g)
	}
	return out
}

// accountGroup is one account row with what it folded: every id and folder of
// the records it stands for, the representative's first.
type accountGroup struct {
	Account
	ids, dirs, emails []string
}

// groupAccounts joins the records with what the scan saw, one entry for each
// account (see foldRecords). The default tag is never stored: an account carries
// it when its email is the one signed in on ~/.claude (or ~/.codex), so it moves
// when /login changes that email.
func groupAccounts(env Env, recs []Record, folders []Folder) []accountGroup {
	byDir := map[string]Folder{}
	home := map[string]Folder{}
	for _, f := range folders {
		byDir[env.pathKey(f.Dir)] = f
		if f.Home {
			home[f.Kind] = f
		}
	}
	checked := env.now().UTC().Format(time.RFC3339)
	out := []accountGroup{}
	for _, members := range foldRecords(env, recs, byDir) {
		r := members[0]
		a := Account{ID: r.ID, Kind: r.Kind, Name: r.Name, Email: r.Email, Dir: env.expand(r.Dir), Checked: checked, Added: r.Added}
		h, hasHome := home[r.Kind]
		g := accountGroup{}
		for _, m := range members {
			g.ids = append(g.ids, m.ID)
			g.dirs = append(g.dirs, env.expand(m.Dir))
			g.emails = append(g.emails, m.Email)
			a.Default = a.Default || (hasHome && h.Email != "" && strings.EqualFold(h.Email, m.Email))
		}
		own, ok := byDir[env.pathKey(a.Dir)]
		switch {
		case !ok:
			a.SignedIn = boolPtr(false)
		case own.SignedIn != nil:
			a.SignedIn = boolPtr(*own.SignedIn)
		}
		a.Plan = own.Plan
		if a.Plan == "" && a.Default {
			a.Plan = h.Plan
		}
		g.Account = a
		out = append(out, g)
	}
	return out
}

// describe is the account rows: groupAccounts without what each folded.
func describe(env Env, recs []Record, folders []Folder) []Account {
	out := []Account{}
	for _, g := range groupAccounts(env, recs, folders) {
		out = append(out, g.Account)
	}
	return out
}

// Status scans the folders and answers every registered account. With register
// true, a login email no record has is registered first (once, under the registry
// lock) and gets its own folder; with false nothing is written.
func Status(env Env, register bool) ([]Account, error) {
	if env.GridHome == "" {
		return nil, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	if register {
		unlock, err := lockRegistry(env)
		if err != nil {
			return nil, err
		}
		defer unlock()
	}
	recs, err := loadRecords(env)
	if err != nil {
		return nil, err
	}
	folders := Scan(env)
	if register {
		var changed bool
		if recs, changed, err = registerNew(env, recs, folders); err != nil {
			return nil, err
		}
		if changed {
			if err := saveRecords(env, recs); err != nil {
				return nil, err
			}
		}
	}
	return describe(env, recs, folders), nil
}

// windowsPlan is one row of the windows answer before it is read: the id it
// shows, the ids it also answers to (the ids of the records it folded), and the
// folders that may hold its numbers, in the order they are read.
type windowsPlan struct {
	id, kind string
	ids      []string
	dirs     []string
	onDisk   bool // one of dirs is a folder the scan found
}

// maxProbes bounds how many folders are read at once.
const maxProbes = 8

// Windows answers the account_windows shape for every account, or for the one
// whose id is given. A default account reads the numbers of ~/.claude, where its
// sessions run, then those of its own folder. Each account is one row, however
// many records or folders carry its login (see foldRecords): a folder with the
// login of an account adds no row of its own, it is another place to read that
// account. The folders are read side by side, so one that hangs costs one timeout
// and holds up no other; with an id only that account is read.
func Windows(env Env, id string) ([]AccountWindows, error) {
	recs := []Record{}
	if env.GridHome != "" {
		var err error
		if recs, err = loadRecords(env); err != nil {
			return nil, err
		}
	}
	folders := Scan(env)
	home := map[string]string{}
	spelled := map[string]string{} // pathKey -> the scanned folder's own spelling
	for _, f := range folders {
		if f.Home {
			home[f.Kind] = f.Dir
		}
		if _, ok := spelled[env.pathKey(f.Dir)]; !ok {
			spelled[env.pathKey(f.Dir)] = f.Dir
		}
	}
	covered := map[string]bool{}
	byLogin := map[string]int{} // kind and login -> index in plans
	loginKey := func(kind, email string) string { return kind + "\x00" + normEmail(email) }
	var plans []windowsPlan
	for _, g := range groupAccounts(env, recs, folders) {
		a := g.Account
		dirs := g.dirs
		if a.Default {
			dirs = append([]string{home[a.Kind]}, dirs...)
		}
		// The numbers are kept per spelling, so the scanned spelling of an
		// account's folder is read too when it differs from the record's.
		onDisk := false
		for _, d := range dirs {
			s, ok := spelled[env.pathKey(d)]
			if ok && s != d {
				dirs = append(dirs, s)
			}
			onDisk = onDisk || ok
			covered[env.pathKey(d)] = true
		}
		for _, e := range g.emails {
			if normEmail(e) != "" {
				byLogin[loginKey(a.Kind, e)] = len(plans)
			}
		}
		plans = append(plans, windowsPlan{id: a.ID, kind: a.Kind, ids: g.ids, dirs: dirs, onDisk: onDisk})
	}
	for _, f := range folders {
		if covered[env.pathKey(f.Dir)] {
			continue
		}
		covered[env.pathKey(f.Dir)] = true
		// A folder with the login of an account that already has a folder of its
		// own is another place to read that account, not another account. When the
		// account's folder is not there (on Linux ~/.Claude-ann is not the record's
		// ~/.claude-ann), the folder stays a row of its own, as it always did.
		if i, ok := byLogin[loginKey(f.Kind, f.Email)]; ok && normEmail(f.Email) != "" && plans[i].onDisk {
			plans[i].dirs = append(plans[i].dirs, f.Dir)
			continue
		}
		fid := strings.TrimPrefix(filepath.Base(f.Dir), ".")
		if normEmail(f.Email) != "" {
			byLogin[loginKey(f.Kind, f.Email)] = len(plans)
		}
		plans = append(plans, windowsPlan{id: fid, kind: f.Kind, ids: []string{fid}, dirs: []string{f.Dir}, onDisk: true})
	}
	if id != "" {
		var chosen []windowsPlan
		for _, p := range plans {
			for _, pid := range p.ids {
				if pid == id {
					chosen = append(chosen, p)
					break
				}
			}
		}
		if len(chosen) == 0 {
			return nil, fmt.Errorf("no account %q", id)
		}
		plans = chosen[:1]
	}
	out := make([]AccountWindows, len(plans))
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxProbes)
	for i, p := range plans {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			out[i] = windowsFor(env, p.id, p.kind, p.dirs)
		}()
	}
	wg.Wait()
	return out, nil
}
