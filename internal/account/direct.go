package account

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// The direct read: Grid asks the provider's own command for the account's
// windows, with no status line and no session first. Claude Code answers its
// get_usage control request; `codex app-server` answers account/rateLimits/read.
// The provider reads and refreshes its own sign-in, so this package opens no
// credential and keeps no token: the cache below holds use numbers and times.

const (
	directGap      = 2 * time.Minute  // at most one probe per folder in this time
	directBackoff  = 5 * time.Minute  // after a failed probe, no new probe for this long
	directTimeout  = 30 * time.Second // one probe is killed after this
	codexRetryWait = 2 * time.Second  // before the one retry of an error reply
	lockSlack      = 5 * time.Second  // a reader waits for another's probe this long beyond its timeout
)

// The reason lines. They are fixed text: what the provider printed is never
// copied into a note or a cache file.
const (
	reasonSignedOut      = "signed out"
	reasonClaudeVersion  = "usage cannot be read from this Claude Code version"
	reasonCodexVersion   = "usage cannot be read from this Codex version"
	reasonClaudeError    = "Claude Code answered the usage request with an error"
	reasonCodexError     = "Codex answered the usage request with an error"
	reasonTimedOut       = "usage read timed out"
	reasonCannotStart    = "the provider could not be started"
	reasonCannotKeep     = "cannot write the Grid home, so usage is not read"
	reasonNotFoundSuffix = " was not found on PATH"
)

// directReading is the windows one probe read.
type directReading struct {
	five, seven *Reading
}

func (r directReading) ok() bool { return r.five != nil || r.seven != nil }

// directCache is the file kept for one folder beside the status line snapshot:
// numbers and times only. The last good reading stays with its own time At while
// later probes fail; Tried and Failed drive the gap and the back-off.
type directCache struct {
	ConfigDir string   `json:"configDir"`
	Kind      string   `json:"kind"`
	FiveHour  *Reading `json:"five_hour,omitempty"`
	SevenDay  *Reading `json:"seven_day,omitempty"`
	At        string   `json:"at,omitempty"`
	Tried     string   `json:"tried,omitempty"`
	Failed    bool     `json:"failed,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

func (c directCache) good() bool { return c.FiveHour != nil || c.SevenDay != nil }

func directPath(env Env, dir, ext string) string {
	sum := sha256.Sum256([]byte(gridhome.Real(dir)))
	return filepath.Join(env.GridHome, "accounts", "windows", hex.EncodeToString(sum[:4])+ext)
}

func loadDirect(env Env, dir string) directCache {
	var c directCache
	if data, err := os.ReadFile(directPath(env, dir, ".direct.json")); err == nil {
		if json.Unmarshal(data, &c) != nil {
			c = directCache{}
		}
	}
	return c
}

func saveDirect(env Env, dir string, c directCache) error {
	path := directPath(env, dir, ".direct.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	out, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(out, '\n'), 0o600)
}

// directDue says whether a new probe may run: not inside the gap after a read,
// not inside the back-off after a failure. A last attempt dated after now (the
// clock went back) counts as old.
func directDue(env Env, c directCache) bool {
	tried, err := time.Parse(time.RFC3339, c.Tried)
	if err != nil {
		return true
	}
	age := env.now().Sub(tried)
	if age < 0 {
		return true
	}
	if c.Failed {
		return age >= directBackoff
	}
	return age >= directGap
}

func (e Env) probeTimeout() time.Duration {
	if e.ProbeTimeout > 0 {
		return e.ProbeTimeout
	}
	return directTimeout
}

func (e Env) retryWait() time.Duration {
	if e.ProbeRetryWait > 0 {
		return e.ProbeRetryWait
	}
	return codexRetryWait
}

// isHomeFolder is true for ~/.claude and ~/.codex themselves: the provider finds
// those with its folder variable unset.
func isHomeFolder(env Env, kind, dir string) bool {
	return env.Home != "" && env.pathKey(dir) == env.pathKey(filepath.Join(env.Home, "."+kind))
}

// lockDirect takes the folder's probe lock, so one probe runs per folder at a
// time. A second reader waits for the first's probe (it ends within the timeout)
// and then reads its answer from the cache. ok is false when the wait ran out.
func lockDirect(env Env, dir string, wait time.Duration) (unlock func(), ok bool) {
	none := func() {}
	path := directPath(env, dir, ".direct.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return none, false
	}
	f, err := fslock.OpenNoFollow(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return none, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	switch err := fslock.LockExclusiveContext(ctx, f); {
	case err == nil:
		return func() { fslock.Unlock(f); f.Close() }, true
	case errors.Is(err, fslock.ErrUnsupported):
		return none, true // no lock on this system: the gap still bounds the probes
	}
	return none, false
}

// directRead answers the cache of one folder, after a probe when one is due.
// What it returns always carries the last good reading, if there was one, and
// the reason of the last failed attempt.
func directRead(env Env, kind, dir string) directCache {
	if env.GridHome == "" {
		return directCache{} // no cache to keep, so no probe: nothing was tried and there is no reason to give
	}
	c := loadDirect(env, dir)
	if !directDue(env, c) {
		return c
	}
	if _, err := exec.LookPath(kind); err != nil {
		c.Reason = kind + reasonNotFoundSuffix // nothing was tried: nothing is recorded
		return c
	}
	unlock, ok := lockDirect(env, dir, env.probeTimeout()+lockSlack)
	if !ok {
		if c.Reason == "" {
			c.Reason = reasonCannotKeep
		}
		return c
	}
	defer unlock()
	if c = loadDirect(env, dir); !directDue(env, c) {
		return c // another reader probed while this one waited
	}
	reading, reason := probeFolder(env, kind, dir)
	now := env.now().UTC().Format(time.RFC3339)
	c.ConfigDir, c.Kind, c.Tried = gridhome.Real(dir), kind, now
	if reading.ok() {
		c.FiveHour, c.SevenDay, c.At, c.Failed, c.Reason = reading.five, reading.seven, now, false, ""
	} else {
		c.Failed, c.Reason = true, reason
	}
	if err := saveDirect(env, dir, c); err != nil {
		c.Reason = reasonCannotKeep
	}
	return c
}

// probeFolder runs one probe on a folder within one deadline: the Codex retry
// is inside it.
func probeFolder(env Env, kind, dir string) (directReading, string) {
	ctx, cancel := context.WithTimeout(context.Background(), env.probeTimeout())
	defer cancel()
	// Claude Code finds ~/.claude with its variable unset, as a person's own
	// sessions do; Codex is always told its folder.
	child := dir
	if kind == Claude && isHomeFolder(env, kind, dir) {
		child = ""
	}
	if kind == Codex {
		return codexUsage(ctx, env, child)
	}
	return claudeUsage(ctx, env, child)
}

// directFor reads the folders of one account in order, each once, and answers
// the first with a reading and the first reason it met. A folder spelled twice
// (a case-folding volume) is read once.
func directFor(env Env, kind string, dirs []string) (got directCache, reason string) {
	seen := map[string]bool{}
	for _, d := range dirs {
		if d == "" || seen[env.pathKey(d)] {
			continue
		}
		seen[env.pathKey(d)] = true
		c := directRead(env, kind, d)
		if reason == "" {
			reason = c.Reason
		}
		if c.good() {
			return c, c.Reason
		}
	}
	return directCache{}, reason
}
