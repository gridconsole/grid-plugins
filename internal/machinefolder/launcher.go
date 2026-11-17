package machinefolder

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// ErrRunning is returned by Launcher.Start when a launcher of the same login
// already holds the lock.
var ErrRunning = errors.New("a launcher of this login is already running")

// CurrentLogin names the login that runs grid, as one plain file name: the OS
// user name, else USER or USERNAME. It is "" when none is known.
func CurrentLogin(getenv func(string) string) string {
	if u, err := user.Current(); err == nil && SafeLogin(u.Username) != "" {
		return SafeLogin(u.Username)
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if s := SafeLogin(getenv(key)); s != "" {
			return s
		}
	}
	return ""
}

// SafeLogin turns a login name into one plain path element: letters, digits and
// . _ - stay, anything else (a Windows DOMAIN\user, a space) becomes _, and
// leading dots go. It is "" when nothing is left.
func SafeLogin(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.TrimLeft(b.String(), ".")
}

// Launcher is one login's launcher files: launcher/<login>/pid, lock and log. A
// shared folder holds one such folder per login, so one launcher per login runs at
// once and no login removes, locks or rewrites another's files: every call here
// stays inside this login's folder.
type Launcher struct {
	folder Folder
	login  string
	dir    string
	lock   *os.File
}

// Launcher returns the launcher files of login. It makes nothing on disk.
func (f Folder) Launcher(login string) (*Launcher, error) {
	if login != SafeLogin(login) || login == "" {
		return nil, errors.New("login is not one plain name: " + login)
	}
	dir, err := f.Join("launcher", login)
	if err != nil {
		return nil, err
	}
	return &Launcher{folder: f, login: login, dir: dir}, nil
}

// Login is the login these files belong to.
func (l *Launcher) Login() string { return l.login }

// Dir is launcher/<login>.
func (l *Launcher) Dir() string { return l.dir }

// PIDPath is the file holding the launcher's process id.
func (l *Launcher) PIDPath() string { return filepath.Join(l.dir, "pid") }

// LockPath is the file the running launcher holds a lock on.
func (l *Launcher) LockPath() string { return filepath.Join(l.dir, "lock") }

// LogPath is the launcher's log.
func (l *Launcher) LogPath() string { return filepath.Join(l.dir, "log") }

// Start takes this login's lock and writes pid. A second Start under the same
// login, while the first still runs, returns ErrRunning; a leftover lock file of
// a launcher that died holds nothing and Start takes it over. Another login's
// folder is never opened.
func (l *Launcher) Start(pid int) error {
	if l.lock != nil {
		return ErrRunning
	}
	lf, err := l.folder.OpenCreate(l.LockPath(), os.O_RDWR)
	if err != nil {
		return err
	}
	if err := fslock.TryLockExclusive(lf); err != nil {
		lf.Close()
		if errors.Is(err, fslock.ErrLocked) {
			return ErrRunning
		}
		return err
	}
	if err := l.folder.WriteFile(l.PIDPath(), []byte(strconv.Itoa(pid)+"\n")); err != nil {
		fslock.Unlock(lf)
		lf.Close()
		return err
	}
	l.lock = lf
	return nil
}

// Stop removes this login's pid file and drops the lock. The lock file stays: a
// launcher that starts at the same moment must not lock a file that was just
// unlinked.
func (l *Launcher) Stop() error {
	lf := l.lock
	if lf == nil {
		return nil
	}
	l.lock = nil
	err := os.Remove(l.PIDPath())
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	fslock.Unlock(lf)
	return errors.Join(err, lf.Close())
}

// OpenLog opens this login's log for appending, making it when it is missing.
func (l *Launcher) OpenLog() (*os.File, error) {
	return l.folder.OpenCreate(l.LogPath(), os.O_WRONLY|os.O_APPEND)
}

// Logins lists the logins that have a launcher folder, read only.
func (f Folder) Logins() ([]string, error) {
	root, err := f.LauncherRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var logins []string
	for _, e := range entries {
		if e.IsDir() {
			logins = append(logins, e.Name())
		}
	}
	sort.Strings(logins)
	return logins, nil
}
