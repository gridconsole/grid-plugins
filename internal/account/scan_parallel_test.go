package account

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// slowScript is a `claude auth status` that takes as long as the folder's own
// delay file says, then answers with the folder's fake-email. It sets its PATH so
// sleep is found: the test's PATH holds only the fake provider.
const slowScript = `#!/bin/sh
PATH=/bin:/usr/bin
dir="${CLAUDE_CONFIG_DIR:-$FAKE_HOME/.claude}"
read -r delay < "$dir/delay"
sleep "$delay"
read -r email < "$dir/fake-email"
printf '{"loggedIn":true,"email":"%s"}\n' "$email"
`

// slowDelays is one check's time for ~/.claude, ~/.claude-a, ... in the order Scan
// lists them. They fall, so the folders finish in the reverse of the listed order
// and a Scan that answers in finishing order shows it. One after another they take
// 2.6 s; side by side they take the longest, 0.6 s.
var slowDelays = []string{"0.6", "0.5", "0.45", "0.4", "0.35", "0.3"}

const slowBudget = 1300 * time.Millisecond // half of the serial time

// slowMachine signs the fake provider in on ~/.claude and five more folders and
// answers their folders in the order Scan must list them.
func slowMachine(t *testing.T) (Env, []string) {
	t.Helper()
	env, bin := machine(t)
	installFakeClaude(t, bin)
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(slowScript), 0o755); err != nil {
		t.Fatal(err)
	}
	dirs := []string{filepath.Join(env.Home, ".claude")}
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		dirs = append(dirs, filepath.Join(env.Home, ".claude-"+n))
	}
	for i, dir := range dirs {
		login(t, dir, filepath.Base(dir)[1:]+"@example.com")
		write(t, filepath.Join(dir, "delay"), slowDelays[i])
	}
	return env, dirs
}

func TestScanChecksFoldersSideBySideInListOrder(t *testing.T) {
	env, dirs := slowMachine(t)

	start := time.Now()
	got := Scan(env)
	elapsed := time.Since(start)

	if elapsed > slowBudget {
		t.Errorf("scan of %d folders took %v; checked one after another it takes 2.6 s, side by side about 0.6 s", len(dirs), elapsed)
	}
	if len(got) != len(dirs) {
		t.Fatalf("want %d folders, got %+v", len(dirs), got)
	}
	for i, f := range got {
		want := filepath.Base(dirs[i])[1:] + "@example.com"
		if f.Dir != dirs[i] || f.Email != want || f.SignedIn == nil || !*f.SignedIn {
			t.Fatalf("folder %d: want %s with %s signed in, got %+v", i, dirs[i], want, f)
		}
	}
}

// registryLockFree says whether another descriptor could take registry.lock now.
func registryLockFree(t *testing.T, env Env) bool {
	t.Helper()
	dir := filepath.Join(env.GridHome, "accounts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "registry.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	switch err := fslock.TryLockExclusive(f); {
	case err == nil:
		fslock.Unlock(f)
		return true
	case errors.Is(err, fslock.ErrLocked):
		return false
	default:
		t.Fatal(err)
		return false
	}
}

func TestStatusScansBeforeItTakesTheRegistryLock(t *testing.T) {
	env, dirs := slowMachine(t)

	type answer struct {
		accounts []Account
		err      error
		elapsed  time.Duration
	}
	done := make(chan answer, 1)
	start := time.Now()
	go func() {
		a, err := Status(env, true)
		done <- answer{a, err, time.Since(start)}
	}()

	time.Sleep(150 * time.Millisecond) // every check is still running: the shortest takes 0.3 s
	if !registryLockFree(t, env) {
		t.Error("registry.lock is held while the folders are being checked")
	}
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.elapsed > slowBudget {
		t.Errorf("status of %d folders took %v; one check is 0.6 s", len(dirs), got.elapsed)
	}
	if len(got.accounts) != len(dirs) {
		t.Fatalf("want one account for each of %d logins, got %+v", len(dirs), got.accounts)
	}
}
