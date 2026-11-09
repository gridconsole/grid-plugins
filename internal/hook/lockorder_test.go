package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// start runs one hook in its own goroutine and returns its log when it ends.
func (f *eventFixture) start(t *testing.T, event string, in hookio.Input) <-chan string {
	t.Helper()
	if in.Cwd == "" {
		in.Cwd = f.dir
	}
	if in.SessionID == "" {
		in.SessionID = "session-demo"
	}
	in.HookEventName = event
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	o := f.o
	o.Event, o.Stdin = event, bytes.NewReader(b)
	var out, log bytes.Buffer
	o.Stdout, o.Stderr = &out, &log
	done := make(chan string, 1)
	go func() {
		Run(o)
		done <- log.String()
	}()
	return done
}

// lockOrderFinishes is the review's P4 with the real hook. The test plays
// attach's lock order: it holds .task.lock (attach.go withWrite), lets the hook
// run, then takes the session lock the way sessions.Start does. A hook that
// waits for .task.lock while it holds the session lock blocks both sides.
func lockOrderFinishes(t *testing.T, f *eventFixture, event string, in hookio.Input, want string) {
	t.Helper()
	lock, err := fslock.OpenNoFollow(filepath.Join(f.dir, ".task.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	held := true
	release := func() {
		if held {
			fslock.Unlock(lock)
			held = false
		}
	}
	defer release()
	hookDone := f.start(t, event, in)
	time.Sleep(500 * time.Millisecond)
	attachDone := make(chan error, 1)
	go func() {
		key := session.Key{SessionID: "attach-demo", MachineID: "machine-demo"}
		_, err := (session.Store{TaskDir: f.dir}).Start(session.Record{Key: key, PID: 123, PIDStart: "start", Lease: "J-1", Holder: "member@example.com", Fence: 2, Attempt: 1})
		attachDone <- err
	}()
	deadline := time.After(10 * time.Second)
	select {
	case err := <-attachDone:
		if err != nil {
			t.Errorf("attach side: %v", err)
		}
	case <-deadline:
		t.Errorf("P4: attach still blocked on the session lock after 10 s while the hook waits for .task.lock")
		release()
		drain := time.After(10 * time.Second)
		for _, c := range []<-chan string{hookDone} {
			select {
			case <-c:
			case <-drain:
			}
		}
		select {
		case <-attachDone:
		case <-drain:
		}
		return
	}
	release()
	select {
	case log := <-hookDone:
		if log != "" {
			t.Errorf("hook log: %s", log)
		}
	case <-deadline:
		t.Fatal("P4: the hook did not finish within 10 s")
	}
	b, _ := os.ReadFile(filepath.Join(f.dir, "history.md"))
	if !strings.Contains(string(b), want) {
		t.Errorf("history lacks %q after the hook: %s", want, b)
	}
}

func acceptedNote(t *testing.T, f *eventFixture) string {
	t.Helper()
	s := brain.Store{GridHome: f.home}
	n := brain.Note{Name: "cache", Topic: "decisions", Type: "convention", Description: "Use when changing cache configuration.", Summary: "Keep cache settings explicit.", Source: "task:T-01", Level: "home", Body: "Body.", Origin: brain.Origin{Tier: contract.PersonTeam, Kind: "session", ID: "session-demo"}}
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	e, err := s.Accept("home", "cache")
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(f.home, e.Path)
}

// One lock order: .task.lock is never taken while the session lock is held, so
// each history-writing hook and attach both finish within 10 s.
func TestHookAndAttachLockOrderFinishes(t *testing.T) {
	t.Run("bash history", func(t *testing.T) {
		f := eventSetup(t)
		in := hookio.Input{ToolName: "Bash", ToolUseID: "tool-1", ToolInput: map[string]any{"command": "make notes"}}
		if _, log, code := f.run(t, "PreToolUse", in); code != 0 || log != "" {
			t.Fatalf("before: %d %s", code, log)
		}
		if err := os.WriteFile(filepath.Join(f.dir, "notes.md"), []byte("changed"), 0o600); err != nil {
			t.Fatal(err)
		}
		lockOrderFinishes(t, f, "PostToolUse", in, "file written · notes.md")
	})
	t.Run("brain read", func(t *testing.T) {
		f := eventSetup(t)
		path := acceptedNote(t, f)
		lockOrderFinishes(t, f, "PostToolUse", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": path}}, "brain read")
	})
	t.Run("brain recall", func(t *testing.T) {
		f := eventSetup(t)
		acceptedNote(t, f)
		lockOrderFinishes(t, f, "UserPromptSubmit", hookio.Input{Prompt: "/grid:plan"}, "brain recall")
	})
}
