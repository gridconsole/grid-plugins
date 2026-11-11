package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/question"
)

// jwLoDeadline is only the failure path of a wait the fixed code never reaches.
const jwLoDeadline = 30 * time.Second

// jwLoLocks reports each lock call that is about to wait on .orchestrator.lock
// or on questions/.lock. The hook only compares the path and sends without
// blocking.
func jwLoLocks(t *testing.T) <-chan string {
	t.Helper()
	events := make(chan string, 16)
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		name := filepath.Base(path)
		switch {
		case name == ".orchestrator.lock":
		case name == ".lock" && filepath.Base(filepath.Dir(path)) == "questions":
			name = "questions/.lock"
		default:
			return
		}
		select {
		case events <- name:
		default:
		}
	}))
	return events
}

// jwLoHoldPass holds <task>/feed/.orchestrator.lock exclusive on its own
// descriptor, as a holder pass does, until release runs (at the latest at cleanup).
func jwLoHoldPass(t *testing.T, store feed.Store) (release func()) {
	t.Helper()
	f, err := fslock.OpenNoFollow(filepath.Join(store.Dir(), ".orchestrator.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			fslock.Unlock(f)
			f.Close()
		})
	}
	t.Cleanup(release)
	return release
}

// jwLoCycle runs a person's command while the pass lock is held. Once the
// command waits for .orchestrator.lock, a holder's ReserveGate asks for the
// questions lock; if that has to wait, the two wait for each other for good.
// It returns the command's exit code after the pass lock is let go.
func jwLoCycle(t *testing.T, h *harness, path string, store feed.Store, cwd string, args ...string) int {
	t.Helper()
	events := jwLoLocks(t)
	release := jwLoHoldPass(t, store)
	deadline := time.After(jwLoDeadline)
	done := make(chan int, 1)
	go func() { done <- h.run(cwd, args...) }()
	for waiting := true; waiting; {
		select {
		case name := <-events:
			waiting = name != ".orchestrator.lock"
		case code := <-done:
			t.Fatalf("%v ended with %d while the pass held .orchestrator.lock: %s", args, code, h.errb)
		case <-deadline:
			t.Fatalf("timed out waiting for %v to wait on .orchestrator.lock", args)
		}
	}
	reserved := make(chan error, 1)
	go func() {
		reserved <- question.Store{TaskDir: path, TaskID: feedTaskID}.ReserveGate(1, func(string) error { return nil })
	}()
	verb := args[0]
	if verb == "question" {
		verb += " " + args[1]
	}
	cycle := ""
	for waiting := true; waiting; {
		select {
		case name := <-events:
			if name == "questions/.lock" {
				cycle, waiting = "the holder's ReserveGate waits on questions/.lock while grid "+verb+" holds it and waits on .orchestrator.lock", false
			}
		case err := <-reserved:
			if err != nil {
				t.Errorf("ReserveGate: %v", err)
			}
			reserved, waiting = nil, false
		case <-deadline:
			t.Fatal("timed out waiting for the holder's ReserveGate")
		}
	}
	// Let both goroutines finish before the test ends, also on a cycle.
	release()
	code := -1
	select {
	case code = <-done:
	case <-deadline:
		t.Fatalf("timed out waiting for %v after the pass lock was let go", args)
	}
	if reserved != nil {
		select {
		case <-reserved:
		case <-deadline:
			t.Fatal("timed out waiting for the holder's ReserveGate after the pass lock was let go")
		}
	}
	if cycle != "" {
		t.Fatalf("lock cycle: %s", cycle)
	}
	return code
}

// Row 1: a person's ask posts its Job without holding the questions lock.
func TestJwLoAskWaitsForThePassWithoutTheQuestionsLock(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	asPerson(t, "member")
	args := []string{"question", "ask", "Which export format?", "--who", "dana", "--option", "csv", "--option", "json",
		"--recommended", "csv", "--default", "csv", "--applies", "2099-01-01T00:00:00Z", "--json"}
	if code := jwLoCycle(t, h, path, store, path, args...); code != 0 {
		t.Fatalf("ask %d: %s", code, h.errb)
	}
	var r question.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatalf("ask --json: %v: %s", err, h.out)
	}
	if !regexp.MustCompile(`^Q-[1-9][0-9]*$`).MatchString(r.ID) || r.Job == "" {
		t.Fatalf("ask --json has no Q id or no Job: %s", h.out)
	}
}

// Row 2: a person's answer closes the Job without holding the questions lock.
func TestJwLoAnswerWaitsForThePassWithoutTheQuestionsLock(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	q := askJob(t, h, path)
	asPerson(t, "member")
	if code := jwLoCycle(t, h, path, store, h.gridHome, "answer", path, q.ID, "json"); code != 0 {
		t.Fatalf("answer %d: %s", code, h.errb)
	}
	file, err := os.ReadFile(filepath.Join(path, "questions", q.ID+".md"))
	if err != nil || !strings.Contains(string(file), `"state": "answered"`) {
		t.Fatalf("answer did not reach the file: %v %s", err, file)
	}
}
