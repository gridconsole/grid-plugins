package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func attachOptions(id string) AttachOptions {
	return AttachOptions{ID: id, SessionID: "terminal-1", PID: 42, PIDStart: "start-42"}
}

func TestAttachResume(t *testing.T) {
	s, task := writeFixture(t)
	before, err := os.Stat(filepath.Join(task.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(task.Path, "task.md"))
	write(t, filepath.Join(task.Path, "history.md"), "- old history\n")
	var started time.Time
	for i := range 2 {
		now := time.Date(2026, 1, 2, 3, 4+i, 5, 0, time.UTC)
		s.Now = func() time.Time { return now }
		got, err := s.Attach(attachOptions(task.ID))
		if err != nil {
			t.Fatal(err)
		}
		rec, err := (session.Store{TaskDir: task.Path}).Show(got.Session)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			started = rec.Started
		}
		if rec.Where != "terminal" || rec.Runtime != "claude" || rec.PID != 42 || rec.PIDStart != "start-42" || !rec.Started.Equal(started) || !rec.Last.Equal(now) || rec.State != session.LivenessUnknown || !feed.IsID(rec.MachineID) {
			t.Fatalf("session: %+v", rec)
		}
		if got.ID != task.ID || got.Title != task.Title || got.State != task.State || got.Approved || got.OpenQuestions != 0 {
			t.Fatalf("result: %+v", got)
		}
	}
	after, _ := os.Stat(filepath.Join(task.Path, "task.md"))
	data, _ := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if !os.SameFile(before, after) || !bytes.Equal(data, original) {
		t.Fatal("attach rewrote task.md")
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) != 3 || lines[0] != "- old history" {
		t.Fatalf("history: %v %v", lines, err)
	}
	for _, line := range lines[1:] {
		var event Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "- ")), &event); err != nil {
			t.Fatal(err)
		}
		if event.What != "attach" || event.Task != task.ID || event.FromState != task.State || event.ToState != task.State {
			t.Fatalf("event: %+v", event)
		}
	}
}

func TestAttachRefusals(t *testing.T) {
	for _, tc := range []struct{ name, cause string }{
		{"unknown", "T-99"}, {"closed", "grid task reopen T-01"}, {"invalid session", "session and machine ids"}, {"runtime", "Claude"}, {"pid", "parent process"}, {"history symlink", "regular"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := writeFixture(t)
			o := attachOptions(task.ID)
			switch tc.name {
			case "unknown":
				o.ID = "T-99"
			case "closed":
				if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
					t.Fatal(err)
				}
			case "invalid session":
				o.SessionID = "../escape"
			case "runtime":
				o.Runtime = "other"
			case "pid":
				o.PIDStart = ""
			case "history symlink":
				outside := filepath.Join(filepath.Dir(task.Path), "outside")
				write(t, outside, "untouched")
				if err := os.Symlink(outside, filepath.Join(task.Path, "history.md")); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
			if _, err := s.Attach(o); err == nil || !strings.Contains(err.Error(), tc.cause) {
				t.Fatalf("got %v; want %s", err, tc.cause)
			}
			after, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
			if !bytes.Equal(before, after) {
				t.Fatal("refusal wrote history")
			}
			records, err := (session.Store{TaskDir: task.Path}).List()
			if err != nil || len(records) != 0 {
				t.Fatalf("refusal wrote session: %+v %v", records, err)
			}
		})
	}
}

func TestAttachHolders(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		remote, job, running, ended, same bool
		warning, refused                  bool
	}{
		{name: "remote running", remote: true, running: true, warning: true},
		{name: "local running", running: true},
		{name: "local Job", job: true, running: true, warning: true},
		{name: "remote unknown", remote: true},
		{name: "ended Job", remote: true, job: true, running: true, ended: true},
		{name: "same Job key", job: true, running: true, same: true, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := writeFixture(t)
			machine, err := feed.MachineID(s.Resolver.GridHome)
			if err != nil {
				t.Fatal(err)
			}
			if tc.remote {
				machine = "remote-machine"
			}
			rec := session.Record{Key: session.Key{SessionID: "other", MachineID: machine}, PID: 43, PIDStart: "start-43"}
			if tc.same {
				rec.SessionID = "terminal-1"
			}
			if tc.job {
				rec.Lease, rec.Holder, rec.Fence, rec.Attempt = "J-1", "member@"+machine, 1, 1
			}
			sessions := session.Store{TaskDir: task.Path, Now: s.Now}
			if rec, err = sessions.Start(rec); err != nil {
				t.Fatal(err)
			}
			if tc.running {
				if rec, err = sessions.Reconcile(rec.Key, session.Observation{Process: session.Alive, PID: rec.PID, PIDStart: rec.PIDStart}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.ended {
				if _, err := sessions.End(rec.Key, session.Finished); err != nil {
					t.Fatal(err)
				}
			}
			// Include both a lease file and the Job heartbeat in the unchanged snapshot.
			write(t, filepath.Join(task.Path, "leases", "task.json"), "lease sentinel\n")
			before := attachSnapshot(t, task.Path)
			got, err := s.Attach(attachOptions(task.ID))
			if (err != nil) != tc.refused {
				t.Fatalf("got %+v, %v", got, err)
			}
			if (len(got.Holders) > 0) != tc.warning {
				t.Fatalf("holders: %+v", got.Holders)
			}
			if tc.warning {
				want := rec.Holder
				if want == "" {
					want = rec.SessionID + "@" + machine
				}
				if got.Holders[0] != want {
					t.Fatalf("holder %q; want %q", got.Holders[0], want)
				}
			}
			after := attachSnapshot(t, task.Path)
			for path, data := range before {
				if !bytes.Equal(data, after[path]) {
					t.Fatalf("changed existing file %s", path)
				}
			}
			for path := range after {
				if _, ok := before[path]; !ok && (strings.HasPrefix(path, "feed/") || strings.HasPrefix(path, "leases/")) {
					t.Fatalf("created lease/liveness file: %s", path)
				}
			}
		})
	}
}

func attachSnapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "history.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		out[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAttachFacts(t *testing.T) {
	s, task := waitingFixture(t)
	hash := setPlan(t, task, planV1)
	if _, err := s.Approve(WriteOptions{ID: task.ID}, hash); err != nil {
		t.Fatal(err)
	}
	questions := question.Store{TaskDir: task.Path, Now: s.Now}
	for i := range 2 {
		q, err := questions.Add(question.Input{Question: "Which option?", Who: "local", Options: []string{"one", "two"}, Recommended: "one", Default: "one", Applies: s.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if _, err := questions.Answer(q.ID, "two", "local"); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := s.Attach(attachOptions(task.ID))
	if err != nil || !got.Approved || got.OpenQuestions != 1 {
		t.Fatalf("facts: %+v %v", got, err)
	}
	setPlan(t, task, planV2)
	got, err = s.Attach(attachOptions(task.ID))
	if err != nil || got.Approved {
		t.Fatalf("changed plan: %+v %v", got, err)
	}
}

func TestAttachHistoryRecovery(t *testing.T) {
	s, task := writeFixture(t)
	data, _ := json.Marshal(map[string][]byte{"history.md": []byte("- recovered attach\n")})
	write(t, filepath.Join(task.Path, journalName), string(data))
	if _, err := s.Attach(attachOptions(task.ID)); err != nil {
		t.Fatal(err)
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) != 2 || lines[0] != "- recovered attach" {
		t.Fatalf("history: %v %v", lines, err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, journalName)); !os.IsNotExist(err) {
		t.Fatal("journal remains")
	}
}

func TestConcurrentAttach(t *testing.T) {
	if os.Getenv("GRID_ATTACH_CHILD") == "1" {
		s := Store{Resolver: where.Resolver{GridHome: os.Getenv("GRID_ATTACH_HOME")}}
		for i := range 20 {
			o := attachOptions("T-01")
			o.SessionID = fmt.Sprintf("terminal-%s-%d", os.Getenv("GRID_ATTACH_WRITER"), i)
			if _, err := s.Attach(o); err != nil {
				t.Fatal(err)
			}
			fmt.Println(o.SessionID)
		}
		return
	}
	s, task := writeFixture(t)
	original, _ := os.ReadFile(filepath.Join(task.Path, "task.md"))
	type child struct {
		cmd *exec.Cmd
		out bytes.Buffer
	}
	children := []*child{{}, {}}
	for i, c := range children {
		c.cmd = exec.Command(os.Args[0], "-test.run=^TestConcurrentAttach$")
		c.cmd.Env = append(os.Environ(), "GRID_ATTACH_CHILD=1", "GRID_ATTACH_HOME="+s.Resolver.GridHome, fmt.Sprintf("GRID_ATTACH_WRITER=%d", i))
		c.cmd.Stdout, c.cmd.Stderr = &c.out, &c.out
		if err := c.cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	receipts := map[string]int{}
	for _, c := range children {
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("child: %v %s", err, &c.out)
		}
		for _, line := range strings.Split(c.out.String(), "\n") {
			if strings.HasPrefix(line, "terminal-") {
				receipts[line]++
			}
		}
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) != 40 {
		t.Fatalf("history: %d %v", len(lines), err)
	}
	for _, line := range lines {
		var event Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "- ")), &event); err != nil {
			t.Fatal(err)
		}
		if event.What != "attach" || event.FromState != "queued" || event.ToState != "queued" || receipts[event.Note] != 1 {
			t.Fatalf("bad receipt: %+v", event)
		}
		receipts[event.Note]--
	}
	for id, count := range receipts {
		if count != 0 {
			t.Fatalf("lost receipt %s", id)
		}
	}
	data, _ := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if !bytes.Equal(data, original) {
		t.Fatal("task.md changed")
	}
	records, err := (session.Store{TaskDir: task.Path}).List()
	if err != nil || len(records) != 40 {
		t.Fatalf("sessions: %d %v", len(records), err)
	}
}

func TestAttachAgentBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, bound     string
		inside, allowed bool
	}{
		{name: "assigned id", bound: "same-id", allowed: true},
		{name: "assigned path", bound: "same-path", allowed: true},
		{name: "inside assigned task", inside: true, allowed: true},
		{name: "different id", bound: "other-id"},
		{name: "different path", bound: "other-path"},
		{name: "unbound"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := writeFixture(t)
			other, err := s.New(NewOptions{Cwd: s.Resolver.Home, Project: "personal", Words: "Other task"})
			if err != nil {
				t.Fatal(err)
			}
			o := attachOptions(task.ID)
			o.Agent, o.Cwd = true, s.Resolver.Home
			if tc.inside {
				o.Cwd = filepath.Join(task.Path, "artifacts")
			}
			switch tc.bound {
			case "same-id":
				o.BoundTask = task.ID
			case "same-path":
				o.BoundTask = task.Path
			case "other-id":
				o.BoundTask = other.ID
			case "other-path":
				o.BoundTask = other.Path
			}
			_, err = s.Attach(o)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed %t, error %v", tc.allowed, err)
			}
			if !tc.allowed {
				if !strings.Contains(err.Error(), "agent cannot attach") {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(task.Path, "sessions")); !os.IsNotExist(err) {
					t.Fatal("refusal wrote sessions")
				}
				if _, err := os.Stat(filepath.Join(task.Path, "history.md")); !os.IsNotExist(err) {
					t.Fatal("refusal wrote history")
				}
			}
		})
	}
}
