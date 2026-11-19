package account

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Each fake `claude auth status` drops a start marker for its folder, waits for
// the condition its test cares about (at most 5 s, then it leaves a timeout marker
// and fails), and answers with the folder's fake-email. PATH is set so sleep is
// found: the test's PATH holds only the fake provider. The tests read markers and
// results, never the clock.
const scriptHead = `#!/bin/sh
PATH=/bin:/usr/bin
marks=@MARKS@
dir="${CLAUDE_CONFIG_DIR:-$FAKE_HOME/.claude}"
name="${dir##*/}"
read -r email < "$dir/fake-email"
touch "$marks/start-$name"
wait_for() {
	i=0
	while [ "$i" -lt 100 ]; do
		check && return 0
		sleep 0.05
		i=$((i+1))
	done
	touch "$marks/timeout-$name"
	exit 1
}
`

const scriptTail = `printf '{"loggedIn":true,"email":"%s"}\n' "$email"
`

// peerScript waits until another folder's check has started: a Scan that runs one
// check at a time never gets there.
const peerScript = scriptHead + `
check() {
	for f in "$marks"/start-*; do
		[ "$f" != "$marks/start-$name" ] && [ -e "$f" ] && return 0
	done
	return 1
}
wait_for
` + scriptTail

// holdScript waits for the test to create the release marker.
const holdScript = scriptHead + `
check() { [ -e "$marks/release" ]; }
wait_for
` + scriptTail

// capScript counts the checks running at once (run markers), logs every count it
// sees, and waits until 8 run together.
const capScript = scriptHead + `
touch "$marks/run-$name"
check() {
	set -- "$marks"/run-*
	echo "$#" >> "$marks/seen-$name"
	[ "$#" -ge 8 ] && touch "$marks/reached"
	[ -e "$marks/reached" ]
}
wait_for
rm -f "$marks/run-$name"
` + scriptTail

// folderMachine signs the fake provider in on ~/.claude and one more folder for
// each suffix, with the given script, and returns the folders in the order Scan
// must list them.
func folderMachine(t *testing.T, script string, suffixes ...string) (Env, []string, string) {
	t.Helper()
	env, bin := machine(t)
	installFakeClaude(t, bin)
	marks := t.TempDir()
	body := strings.ReplaceAll(script, "@MARKS@", marks)
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	dirs := []string{filepath.Join(env.Home, ".claude")}
	for _, n := range suffixes {
		dirs = append(dirs, filepath.Join(env.Home, ".claude-"+n))
	}
	for _, dir := range dirs {
		login(t, dir, filepath.Base(dir)[1:]+"@example.com")
	}
	return env, dirs, marks
}

func noTimeouts(t *testing.T, marks string) {
	t.Helper()
	if m, _ := filepath.Glob(filepath.Join(marks, "timeout-*")); len(m) > 0 {
		t.Errorf("checks waited in vain, so they did not run side by side: %v", m)
	}
}

func checkListOrder(t *testing.T, got []Folder, dirs []string) {
	t.Helper()
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

func TestScanChecksFoldersSideBySideInListOrder(t *testing.T) {
	env, dirs, marks := folderMachine(t, peerScript, "a", "b", "c", "d", "e")

	got := Scan(env)

	noTimeouts(t, marks)
	checkListOrder(t, got, dirs)
}

func TestScanRunsAtMostEightChecksAtOnce(t *testing.T) {
	env, dirs, marks := folderMachine(t, capScript, "a", "b", "c", "d", "e", "f", "g", "h", "i")

	got := Scan(env)

	noTimeouts(t, marks)
	checkListOrder(t, got, dirs)
	peak := 0
	seen, _ := filepath.Glob(filepath.Join(marks, "seen-*"))
	for _, p := range seen {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Fields(string(b)) {
			n, err := strconv.Atoi(line)
			if err != nil {
				t.Fatal(err)
			}
			peak = max(peak, n)
		}
	}
	if peak != 8 {
		t.Errorf("most checks seen running at once: want 8, got %d", peak)
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
	env, dirs, marks := folderMachine(t, holdScript, "a", "b", "c", "d", "e")

	type answer struct {
		accounts []Account
		err      error
	}
	done := make(chan answer, 1)
	go func() {
		a, err := Status(env, true)
		done <- answer{a, err}
	}()

	// Every check has started and is held until release, so the scan is under way.
	deadline := time.Now().Add(5 * time.Second)
	for {
		started, _ := filepath.Glob(filepath.Join(marks, "start-*"))
		if len(started) == len(dirs) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d checks started", len(started), len(dirs))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !registryLockFree(t, env) {
		t.Error("registry.lock is held while the folders are being checked")
	}
	write(t, filepath.Join(marks, "release"), "")
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	noTimeouts(t, marks)
	if len(got.accounts) != len(dirs) {
		t.Fatalf("want one account for each of %d logins, got %+v", len(dirs), got.accounts)
	}
}
