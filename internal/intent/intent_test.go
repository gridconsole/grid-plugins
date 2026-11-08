package intent

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/testenv"
)

func testStore(t *testing.T) Store {
	t.Helper()
	return Store{TaskDir: t.TempDir(), Now: func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }}
}

func enqueue(t *testing.T, s Store, verb string, args map[string]string) Record {
	t.Helper()
	r, err := s.Enqueue(verb, "task", args, "member", "")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestVerbs(t *testing.T) {
	for _, tc := range []struct {
		verb string
		args map[string]string
	}{
		{"start", map[string]string{"agent": "planner"}}, {"stop", nil}, {"send", map[string]string{"text": "continue"}},
		{"pause", nil}, {"resume", nil}, {"handback", nil}, {"switch", map[string]string{"agent": "builder"}}, {"move", nil},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			s := testStore(t)
			r := enqueue(t, s, tc.verb, tc.args)
			if !validID(r.ID) || r.Verb != tc.verb || r.Who != "member" || r.Target != "task" || !r.Created.Equal(s.now()) {
				t.Fatalf("record: %+v", r)
			}
			files, err := filepath.Glob(filepath.Join(s.TaskDir, "intents", "*.json"))
			if err != nil || len(files) != 1 {
				t.Fatalf("files: %v %v", files, err)
			}
			rows, err := s.List(true)
			if err != nil || len(rows) != 1 || rows[0].State != "pending" {
				t.Fatalf("list: %v %v", rows, err)
			}
		})
	}
}

func TestInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		verb, target, who string
		args              map[string]string
	}{
		{"unknown", "task", "member", nil}, {"start", "", "member", nil}, {"start", "task", "", nil},
		{"send", "task", "member", nil}, {"switch", "task", "member", nil}, {"stop", "task", "member", map[string]string{"text": "bad"}},
	} {
		s := testStore(t)
		if _, err := s.Enqueue(tc.verb, tc.target, tc.args, tc.who, ""); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
		if _, err := os.Stat(filepath.Join(s.TaskDir, "intents")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid request wrote files")
		}
	}
}

func TestSendCoalescing(t *testing.T) {
	s := testStore(t)
	now := s.now()
	s.Now = func() time.Time { return now }
	args := map[string]string{"text": "hello"}
	first := enqueue(t, s, "send", args)
	if second := enqueue(t, s, "send", args); second.ID != first.ID {
		t.Fatal("duplicate send")
	}
	claim, err := s.Claim(first.ID, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(first.ID, claim.Token, ""); err != nil {
		t.Fatal(err)
	}
	if second := enqueue(t, s, "send", args); second.ID != first.ID {
		t.Fatal("completed send duplicated")
	}
	for _, change := range []struct{ target, who, text string }{{"other", "member", "hello"}, {"task", "other", "hello"}, {"task", "member", "different"}} {
		r, err := s.Enqueue("send", change.target, map[string]string{"text": change.text}, change.who, "")
		if err != nil || r.ID == first.ID {
			t.Fatalf("incorrect coalescing: %v %v", r, err)
		}
	}
	now = now.Add(SendWindow)
	if r := enqueue(t, s, "send", args); r.ID == first.ID {
		t.Fatal("window did not expire")
	}
	r, err := s.Enqueue("send", "task", args, "member", "request-1")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Enqueue("send", "task", args, "member", "request-1")
	if err != nil || again.ID != r.ID {
		t.Fatal("explicit key not coalesced")
	}
	if _, err := s.Enqueue("send", "task", map[string]string{"text": "changed"}, "member", "request-1"); err == nil {
		t.Fatal("key conflict accepted")
	}
	if a, b := enqueue(t, s, "start", nil), enqueue(t, s, "start", nil); a.ID == b.ID {
		t.Fatal("coalesced start")
	}
}

func TestClaimLifecycle(t *testing.T) {
	s := testStore(t)
	now := s.now()
	s.Now = func() time.Time { return now }
	r := enqueue(t, s, "start", nil)
	first, err := s.Claim(r.ID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(r.ID, "worker-b"); !errors.Is(err, ErrClaimed) {
		t.Fatal(err)
	}
	rows, err := s.List(true)
	if err != nil || len(rows) != 0 {
		t.Fatalf("pending: %v %v", rows, err)
	}
	now = now.Add(ClaimTimeout)
	if _, err := s.Done(r.ID, first.Token, ""); !errors.Is(err, ErrClaim) {
		t.Fatalf("expired done: %v", err)
	}
	second, err := s.Claim(r.ID, "worker-b")
	if err != nil {
		t.Fatal(err)
	}
	if first.Token == second.Token {
		t.Fatal("token reused")
	}
	if _, err := s.Done(r.ID, first.Token, ""); !errors.Is(err, ErrClaim) {
		t.Fatalf("stale done: %v", err)
	}
	if _, err := s.Done(r.ID, second.Token, "agent unavailable"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(r.ID, second.Token, "agent unavailable"); err != nil {
		t.Fatal("done retry:", err)
	}
	if _, err := s.Claim(r.ID, "worker-c"); !errors.Is(err, ErrDone) {
		t.Fatal(err)
	}
	rows, err = s.List(false)
	if err != nil || len(rows) != 1 || rows[0].State != "done" || rows[0].Done.Error != "agent unavailable" {
		t.Fatalf("done: %v %v", rows, err)
	}
}

func TestRenewKeepsClaimPastTimeout(t *testing.T) {
	s := testStore(t)
	now := s.now()
	s.Now = func() time.Time { return now }
	r := enqueue(t, s, "start", nil)
	first, err := s.Claim(r.ID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	// Renew every four minutes: the claim outlives its original five-minute timeout.
	for range 3 {
		now = now.Add(ClaimTimeout - time.Minute)
		renewed, err := s.Renew(r.ID, first.Token)
		if err != nil {
			t.Fatal(err)
		}
		if renewed.Token != first.Token || renewed.Who != "worker-a" || !renewed.Created.Equal(first.Created) || !renewed.Until.Equal(now.Add(ClaimTimeout)) {
			t.Fatalf("renewed claim: %+v from %+v", renewed, first)
		}
		if _, err := s.Claim(r.ID, "worker-b"); !errors.Is(err, ErrClaimed) {
			t.Fatalf("second claimer after renew: %v", err)
		}
	}
	if rows, err := s.List(true); err != nil || len(rows) != 0 {
		t.Fatalf("renewed claim listed as pending: %v %v", rows, err)
	}
	rows, err := s.List(false)
	if err != nil || len(rows) != 1 || rows[0].State != "claimed" || rows[0].Claim == nil || !rows[0].Claim.Until.Equal(now.Add(ClaimTimeout)) {
		t.Fatalf("claim after renew: %+v %v", rows, err)
	}
	if _, err := s.Done(r.ID, first.Token, ""); err != nil {
		t.Fatalf("done after renews: %v", err)
	}
	if _, err := s.Renew(r.ID, first.Token); !errors.Is(err, ErrDone) {
		t.Fatalf("renew after done: %v", err)
	}
}

func TestRenewRefusals(t *testing.T) {
	s := testStore(t)
	now := s.now()
	s.Now = func() time.Time { return now }
	r := enqueue(t, s, "start", nil)
	other := enqueue(t, s, "start", nil)
	if _, err := s.Renew(r.ID, "01ARZ3NDEKTSV4RRFFQ69G5FAV"); !errors.Is(err, ErrClaim) {
		t.Fatalf("renew without a claim: %v", err)
	}
	first, err := s.Claim(r.ID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"empty": "", "malformed": "nope", "unknown": "01ARZ3NDEKTSV4RRFFQ69G5FAV"} {
		if _, err := s.Renew(r.ID, token); !errors.Is(err, ErrClaim) {
			t.Fatalf("%s token: %v", name, err)
		}
	}
	if _, err := s.Renew(other.ID, first.Token); !errors.Is(err, ErrClaim) {
		t.Fatalf("token from another intent: %v", err)
	}
	// Past the timeout the claim is gone: the holder cannot revive it, and a second
	// claimer's token fences the first one's renew.
	now = now.Add(ClaimTimeout)
	if _, err := s.Renew(r.ID, first.Token); !errors.Is(err, ErrClaim) {
		t.Fatalf("expired renew: %v", err)
	}
	second, err := s.Claim(r.ID, "worker-b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Renew(r.ID, first.Token); !errors.Is(err, ErrClaim) {
		t.Fatalf("stale renew after reclaim: %v", err)
	}
	if _, err := s.Renew(r.ID, second.Token); err != nil {
		t.Fatalf("current holder renew: %v", err)
	}
	if _, err := s.Renew("../../outside", second.Token); !errors.Is(err, ErrClaim) {
		t.Fatalf("traversal: %v", err)
	}
	if temps, _ := filepath.Glob(filepath.Join(s.TaskDir, "intents", ".intent-*")); len(temps) != 0 {
		t.Fatalf("temporary files left: %v", temps)
	}
}

func TestRenewMissingDirectory(t *testing.T) {
	s := testStore(t)
	if _, err := s.Renew("01ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAW"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

// The helper exits without Done, simulating loss of the worker after its claim.
func TestClaimProcess(t *testing.T) {
	if os.Getenv("GRID_INTENT_TEST_PROCESS") != "1" {
		return
	}
	if _, err := io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
		os.Exit(4)
	}
	s := Store{TaskDir: os.Getenv("GRID_INTENT_TEST_DIR")}
	claim, err := s.Claim(os.Getenv("GRID_INTENT_TEST_ID"), "worker")
	if errors.Is(err, ErrClaimed) {
		os.Exit(3)
	}
	if err != nil {
		os.Exit(4)
	}
	if claim.Token == "" {
		os.Exit(4)
	}
	os.Exit(0)
}

func TestTwentyProcessesAndCrashRecovery(t *testing.T) {
	s := Store{TaskDir: t.TempDir()}
	r := enqueue(t, s, "start", nil)
	const count = 20
	cmds := make([]*exec.Cmd, count)
	gates := make([]io.WriteCloser, count)
	for i := range cmds {
		cmd := exec.Command(os.Args[0], "-test.run=^TestClaimProcess$")
		cmd.Env = append(testenv.Temp(t, os.Environ()), "GRID_INTENT_TEST_PROCESS=1", "GRID_INTENT_TEST_DIR="+s.TaskDir, "GRID_INTENT_TEST_ID="+r.ID)
		gate, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds[i], gates[i] = cmd, gate
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	}
	for _, gate := range gates {
		if _, err := gate.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		_ = gate.Close()
	}
	wins := 0
	for _, cmd := range cmds {
		err := cmd.Wait()
		if err == nil {
			wins++
			continue
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 3 {
			t.Fatalf("helper: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("winners: %d, want exactly one", wins)
	}
	rows, err := s.List(false)
	if err != nil || len(rows) != 1 || rows[0].Claim == nil {
		t.Fatalf("claim: %v %v", rows, err)
	}
	first := rows[0].Claim
	s.Now = func() time.Time { return first.Until }
	rows, err = s.List(true)
	if err != nil || len(rows) != 1 {
		t.Fatalf("reclaimable: %v %v", rows, err)
	}
	next, err := s.Claim(r.ID, "replacement")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(r.ID, first.Token, ""); !errors.Is(err, ErrClaim) {
		t.Fatalf("old worker: %v", err)
	}
	if _, err := s.Done(r.ID, next.Token, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPartialClaimRecovery(t *testing.T) {
	s := Store{TaskDir: t.TempDir()}
	r := enqueue(t, s, "start", nil)
	path := filepath.Join(s.TaskDir, "intents", r.ID+".claim")
	if err := os.WriteFile(path, []byte(`{"token":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(r.ID, "worker"); !errors.Is(err, ErrClaimed) {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return info.ModTime().Add(ClaimTimeout + time.Second) }
	if _, err := s.Claim(r.ID, "worker"); err != nil {
		t.Fatal(err)
	}
}

func TestPublishWithoutHardLinks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		errno    syscall.Errno
		fallback bool
	}{
		{"ENOTSUP", syscall.ENOTSUP, true},
		{"EOPNOTSUPP", syscall.EOPNOTSUPP, true},
		{"ENOSYS", syscall.ENOSYS, true},
		{"EPERM on FAT", syscall.EPERM, true},
		{"EACCES", syscall.EACCES, false},
		{"EEXIST", syscall.EEXIST, false},
		{"EIO", syscall.EIO, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			links := 0
			s.Link = func(oldname, newname string) error {
				links++
				return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: tc.errno}
			}
			r, err := s.Enqueue("start", "task", nil, "member", "")
			dir := filepath.Join(s.TaskDir, "intents")
			if links != 1 {
				t.Fatalf("link calls: %d", links)
			}
			if !tc.fallback {
				if !errors.Is(err, tc.errno) {
					t.Fatalf("error: %v", err)
				}
				if files, _ := filepath.Glob(filepath.Join(dir, "*.json")); len(files) != 0 {
					t.Fatalf("failed publish left %v", files)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				rows, err := s.List(true)
				if err != nil || len(rows) != 1 || rows[0].ID != r.ID {
					t.Fatalf("list: %v %v", rows, err)
				}
				claim, err := s.Claim(r.ID, "worker")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.Done(r.ID, claim.Token, ""); err != nil {
					t.Fatal(err)
				}
				rows, err = s.List(false)
				if err != nil || len(rows) != 1 || rows[0].State != "done" {
					t.Fatalf("done through fallback: %v %v", rows, err)
				}
			}
			if temps, _ := filepath.Glob(filepath.Join(dir, ".intent-*")); len(temps) != 0 {
				t.Fatalf("temporary files left: %v", temps)
			}
		})
	}
}

func TestFallbackNeverReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "01ARZ3NDEKTSV4RRFFQ69G5FAV.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsupported := func(string, string) error { return syscall.ENOTSUP }
	if err := publish(path, Record{ID: "other"}, unsupported); !errors.Is(err, os.ErrExist) {
		t.Fatalf("publish over existing file: %v", err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "original" {
		t.Fatalf("existing file changed: %q %v", b, err)
	}
	if temps, _ := filepath.Glob(filepath.Join(dir, ".intent-*")); len(temps) != 0 {
		t.Fatalf("temporary files left: %v", temps)
	}
}

func TestPathsAndMissing(t *testing.T) {
	s := testStore(t)
	rows, err := s.List(false)
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "intents")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("list created directory")
	}
	if _, err := s.Claim("01ARZ3NDEKTSV4RRFFQ69G5FAV", "worker"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if _, err := s.Claim("../../outside", "worker"); err == nil {
		t.Fatal("accepted traversal")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(s.TaskDir, "intents")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue("stop", "task", nil, "member", ""); err == nil {
		t.Fatal("followed intents symlink")
	}
}

func TestSymlinkFiles(t *testing.T) {
	for _, suffix := range []string{".json", ".claim", ".done.json", ".lock"} {
		t.Run(suffix, func(t *testing.T) {
			s := testStore(t)
			r := enqueue(t, s, "start", nil)
			path := filepath.Join(s.TaskDir, "intents", r.ID+suffix)
			if suffix == ".lock" {
				path = filepath.Join(s.TaskDir, "intents", ".lock")
			}
			_ = os.Remove(path)
			outside := filepath.Join(t.TempDir(), "untouched")
			if err := os.WriteFile(outside, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Claim(r.ID, "worker"); err == nil {
				t.Fatal("followed symlink")
			}
			b, err := os.ReadFile(outside)
			if err != nil || string(b) != "original" {
				t.Fatal("changed external file")
			}
		})
	}
}

func TestULIDs(t *testing.T) {
	s := testStore(t)
	seen := map[string]bool{}
	for range 100 {
		id, err := newID(s.now())
		if err != nil || !validID(id) || seen[id] {
			t.Fatal(id, err)
		}
		seen[id] = true
	}
	for _, id := range []string{strings.Repeat("Z", 26), strings.Repeat("I", 26), strings.Repeat("0", 25)} {
		if validID(id) {
			t.Fatal("invalid ULID accepted", id)
		}
	}
}
