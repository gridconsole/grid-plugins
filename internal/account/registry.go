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
	return filepath.Clean(p)
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
			used = used || env.expand(r.Dir) == filepath.Clean(f.Dir)
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

// describe joins the records with what the scan saw. The default tag is never
// stored: a record carries it when its email is the one signed in on ~/.claude
// (or ~/.codex), so it moves when /login changes that email.
func describe(env Env, recs []Record, folders []Folder) []Account {
	byDir := map[string]Folder{}
	home := map[string]Folder{}
	for _, f := range folders {
		byDir[f.Dir] = f
		if f.Home {
			home[f.Kind] = f
		}
	}
	checked := env.now().UTC().Format(time.RFC3339)
	out := []Account{}
	for _, r := range recs {
		a := Account{ID: r.ID, Kind: r.Kind, Name: r.Name, Email: r.Email, Dir: env.expand(r.Dir), Checked: checked, Added: r.Added}
		h, hasHome := home[r.Kind]
		a.Default = hasHome && h.Email != "" && strings.EqualFold(h.Email, r.Email)
		own, ok := byDir[a.Dir]
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
		out = append(out, a)
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

// Windows answers the account_windows shape for every account, or for the one
// whose id is given. A default account reads the numbers of ~/.claude, where its
// sessions run, then those of its own folder.
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
	for _, f := range folders {
		if f.Home {
			home[f.Kind] = f.Dir
		}
	}
	covered := map[string]bool{}
	out := []AccountWindows{}
	for _, a := range describe(env, recs, folders) {
		dirs := []string{a.Dir}
		covered[a.Dir] = true
		if a.Default {
			dirs = []string{home[a.Kind], a.Dir}
			covered[home[a.Kind]] = true
		}
		out = append(out, windowsFor(env, a.ID, a.Kind, dirs))
	}
	for _, f := range folders {
		if !covered[f.Dir] {
			out = append(out, windowsFor(env, strings.TrimPrefix(filepath.Base(f.Dir), "."), f.Kind, []string{f.Dir}))
		}
	}
	if id == "" {
		return out, nil
	}
	for _, w := range out {
		if w.ID == id {
			return []AccountWindows{w}, nil
		}
	}
	return nil, fmt.Errorf("no account %q", id)
}
