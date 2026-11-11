package question

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// jobCalls records what the stub JobPort saw.
type jobCalls struct {
	mu     sync.Mutex
	posts  []map[string]any
	lease  []feed.Writer
	person []feed.Writer
	closes [][2]string
}

func (c *jobCalls) port(id string, postErr error) JobPort {
	return JobPort{
		Post: func(f feed.Store, lease, person feed.Writer, body map[string]any) (string, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.posts = append(c.posts, body)
			c.lease = append(c.lease, lease)
			c.person = append(c.person, person)
			return id, postErr
		},
		Close: func(f feed.Store, lease, person feed.Writer, job, result string) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.closes = append(c.closes, [2]string{job, result})
			return nil
		},
	}
}

func TestAskPostsPersonJobThenQuestion(t *testing.T) {
	s, _ := setup(t)
	calls := &jobCalls{}
	s.Jobs = calls.port("J-7", nil)
	in := input()
	in.Who = "alice"
	rec, err := s.Ask(in)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Job != "J-7" || rec.ID != "Q-1" {
		t.Fatalf("record job %q id %q", rec.Job, rec.ID)
	}
	if len(calls.posts) != 1 {
		t.Fatalf("Post called %d times", len(calls.posts))
	}
	body := calls.posts[0]
	intent, _ := body["intent"].(map[string]any)
	verify, _ := body["verify"].(map[string]any)
	if body["kind"] != "ask" || body["role"] != "Person" || intent["owner"] != "alice" || verify["path"] != "questions/Q-1.md" {
		t.Fatalf("body %v", body)
	}
	if calls.lease[0].Lease != "task" || calls.person[0].Author.ID != "owner" {
		t.Fatalf("Post got lease %+v person %+v", calls.lease[0], calls.person[0])
	}
	got := list(t, s)
	if len(got) != 1 || got[0].Job != "J-7" || got[0].Post == "" {
		t.Fatalf("question post does not name J-7: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "questions", "Q-1.md")); err != nil {
		t.Fatal(err)
	}
}

func TestAskWithoutJobBehindIt(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		nilPost    bool
		// refusal is the Feed's answer for the question post itself, not this
		// package's: an agent may not post a person gate, a cli writer may.
		refusal string
	}{
		{"nil Post", "owner", true, ""},
		{"agent writer", "agent", false, "role agent may not post person gates"},
		{"cli writer", "cli", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := setup(t)
			calls := &jobCalls{}
			s.Jobs = calls.port("J-7", nil)
			if tc.nilPost {
				s.Jobs.Post = nil
			}
			s.Writer.Author.Role = tc.role
			rec, err := s.Ask(input())
			if len(calls.posts) != 0 {
				t.Fatalf("Post called %d times", len(calls.posts))
			}
			if rec.Job != "" {
				t.Fatalf("record job %q, want the Input's own (empty)", rec.Job)
			}
			if tc.refusal != "" {
				if err == nil || err.Error() != tc.refusal {
					t.Fatalf("err %v, want Feed refusal %q", err, tc.refusal)
				}
				if got := list(t, s); len(got) != 0 {
					t.Fatalf("refused question was posted: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := list(t, s); len(got) != 1 || got[0].Job != "" {
				t.Fatalf("question not posted without a Job: %+v", got)
			}
		})
	}
}

func TestAskPostErrorLeavesNothing(t *testing.T) {
	s, _ := setup(t)
	calls := &jobCalls{}
	boom := errors.New("job store down")
	s.Jobs = calls.port("", boom)
	if _, err := s.Ask(input()); !errors.Is(err, boom) {
		t.Fatalf("err %v, want %v", err, boom)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "questions", "Q-1.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Q-1.md exists after failed Post: %v", err)
	}
	if got := list(t, s); len(got) != 0 {
		t.Fatalf("question post exists after failed Post: %+v", got)
	}
}

func TestJobBody(t *testing.T) {
	long := strings.Repeat("é", 200)
	in := Input{Question: long, Who: "alice"}
	title := jobBody("Q-1", in)["title"].(string)
	if r := []rune(title); len(r) != 120 || string(r[119:]) != "…" || string(r[:5]) != "Q-1: " {
		t.Fatalf("title has %d runes: %q", len(r), title)
	}
	// exactly 120 runes is kept whole; 121 is cut
	if title := jobBody("Q-1", Input{Question: strings.Repeat("a", 115)})["title"].(string); len([]rune(title)) != 120 || strings.HasSuffix(title, "…") {
		t.Fatalf("120-rune title changed: %q", title)
	}
	if title := jobBody("Q-1", Input{Question: strings.Repeat("a", 116)})["title"].(string); len([]rune(title)) != 120 || !strings.HasSuffix(title, "…") {
		t.Fatalf("121-rune title not cut: %q", title)
	}

	body := jobBody("Q-2", Input{Question: "  Which\tformat  to\n use?  ", Who: "bob"})
	if body["title"] != "Q-2: Which format to use?" {
		t.Fatalf("title %q", body["title"])
	}
	intent := body["intent"].(map[string]any)
	if intent["goal"] != "Which\tformat  to\n use?" || intent["owner"] != "bob" || intent["item"] != "questions/Q-2.md" {
		t.Fatalf("intent %v", intent)
	}
	acceptance := body["acceptance"].([]any)[0].(map[string]any)
	if acceptance["id"] != "A1" || acceptance["text"] != "bob answers Q-2" {
		t.Fatalf("acceptance %v", acceptance)
	}
	if verify := body["verify"].(map[string]any); verify["kind"] != "file-exists" || verify["path"] != "questions/Q-2.md" {
		t.Fatalf("verify %v", verify)
	}
	if body["kind"] != JobKind || body["role"] != "Person" {
		t.Fatalf("kind/role %v %v", body["kind"], body["role"])
	}
	for item, want := range map[int]int{0: 1, -1: 1, 1: 1, 3: 3} {
		if got := jobBody("Q-1", Input{Question: "q", Item: item})["item"]; got != want {
			t.Fatalf("item %d gave %v, want %d", item, got, want)
		}
	}
}

func TestSettleJobViaAnswerAndWithdraw(t *testing.T) {
	t.Run("answer closes once, repeat closes again without a second event", func(t *testing.T) {
		s, _ := setup(t)
		calls := &jobCalls{}
		s.Jobs = calls.port("J-7", nil)
		if _, err := s.Ask(input()); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Answer("Q-1", "JSON"); err != nil {
			t.Fatal(err)
		}
		if want := [][2]string{{"J-7", "JSON"}}; !slices.Equal(calls.closes, want) {
			t.Fatalf("closes %v, want %v", calls.closes, want)
		}
		if _, err := s.Answer("Q-1", "JSON"); err != nil {
			t.Fatal(err)
		}
		if want := [][2]string{{"J-7", "JSON"}, {"J-7", "JSON"}}; !slices.Equal(calls.closes, want) {
			t.Fatalf("closes after repeat %v, want %v", calls.closes, want)
		}
		if r := list(t, s)[0]; len(r.History) != 1 {
			t.Fatalf("repeat posted a second event: %+v", r.History)
		}
	})
	t.Run("withdraw", func(t *testing.T) {
		s, _ := setup(t)
		calls := &jobCalls{}
		s.Jobs = calls.port("J-7", nil)
		if _, err := s.Ask(input()); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Withdraw("Q-1", "Not needed"); err != nil {
			t.Fatal(err)
		}
		if want := [][2]string{{"J-7", "withdrawn: Not needed"}}; !slices.Equal(calls.closes, want) {
			t.Fatalf("closes %v, want %v", calls.closes, want)
		}
	})
	t.Run("defaulted result is the default text", func(t *testing.T) {
		s, now := setup(t)
		calls := &jobCalls{}
		s.Jobs = calls.port("J-7", nil)
		if _, err := s.Ask(input()); err != nil {
			t.Fatal(err)
		}
		*now = start.Add(24 * time.Hour)
		if _, err := s.Tick(); err != nil {
			t.Fatal(err)
		}
		if want := [][2]string{{"J-7", "CSV"}}; !slices.Equal(calls.closes, want) {
			t.Fatalf("closes %v, want %v", calls.closes, want)
		}
	})
	t.Run("no job makes no call", func(t *testing.T) {
		s, _ := setup(t)
		calls := &jobCalls{}
		s.Jobs = calls.port("J-7", nil)
		add(t, s, input()) // Add posts no Job
		if _, err := s.Answer("Q-1", "JSON"); err != nil {
			t.Fatal(err)
		}
		if len(calls.closes) != 0 || len(calls.posts) != 0 {
			t.Fatalf("calls %+v", calls)
		}
	})
	t.Run("nil Close makes no call", func(t *testing.T) {
		s, _ := setup(t)
		calls := &jobCalls{}
		s.Jobs = calls.port("J-7", nil)
		if _, err := s.Ask(input()); err != nil {
			t.Fatal(err)
		}
		s.Jobs.Close = nil
		if _, err := s.Answer("Q-1", "JSON"); err != nil {
			t.Fatal(err)
		}
		if len(calls.closes) != 0 {
			t.Fatalf("closes %v", calls.closes)
		}
	})
}

func TestReserveGate(t *testing.T) {
	marker := func(t *testing.T, s Store, id string) os.FileInfo {
		t.Helper()
		info, err := os.Stat(filepath.Join(s.TaskDir, "questions", id+".feed"))
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	reserve := func(t *testing.T, s Store, minimum int) string {
		t.Helper()
		var got string
		if err := s.ReserveGate(minimum, func(id string) error { got = id; return nil }); err != nil {
			t.Fatal(err)
		}
		return got
	}
	t.Run("empty folder", func(t *testing.T) {
		s, _ := setup(t)
		if id := reserve(t, s, 1); id != "Q-1" {
			t.Fatalf("id %s", id)
		}
		info := marker(t, s, "Q-1")
		if info.Size() != 0 {
			t.Fatalf("marker size %d", info.Size())
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatalf("marker mode %v", info.Mode())
		}
	})
	t.Run("after an existing question and minimum", func(t *testing.T) {
		s, _ := setup(t)
		if err := os.MkdirAll(filepath.Join(s.TaskDir, "questions"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(s.TaskDir, "questions", "Q-2.md"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		if id := reserve(t, s, 1); id != "Q-3" {
			t.Fatalf("id %s, want Q-3", id)
		}
		if id := reserve(t, s, 5); id != "Q-5" {
			t.Fatalf("id %s, want Q-5", id)
		}
	})
	t.Run("publish error keeps the marker", func(t *testing.T) {
		s, _ := setup(t)
		boom := errors.New("feed down")
		if err := s.ReserveGate(1, func(string) error { return boom }); !errors.Is(err, boom) {
			t.Fatalf("err %v", err)
		}
		marker(t, s, "Q-1")
		if id := reserve(t, s, 1); id != "Q-2" {
			t.Fatalf("id %s, want Q-2 (no reuse)", id)
		}
	})
	t.Run("concurrent callers get distinct ids", func(t *testing.T) {
		s, _ := setup(t)
		ids := make(chan string, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := s.ReserveGate(1, func(id string) error { ids <- id; return nil }); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		close(ids)
		var got []string
		for id := range ids {
			got = append(got, id)
		}
		slices.Sort(got)
		if !slices.Equal(got, []string{"Q-1", "Q-2"}) {
			t.Fatalf("ids %v", got)
		}
	})
	t.Run("range exhausted", func(t *testing.T) {
		if strconv.IntSize != 64 {
			t.Skip("needs 64-bit int")
		}
		s, _ := setup(t)
		dir := filepath.Join(s.TaskDir, "questions")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "Q-9223372036854775807.md"), nil, 0600); err != nil {
			t.Fatal(err)
		}
		called := false
		err := s.ReserveGate(1, func(string) error { called = true; return nil })
		if err == nil || err.Error() != "question id range exhausted" {
			t.Fatalf("err %v", err)
		}
		if called {
			t.Fatal("publish called")
		}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".feed") {
				t.Fatalf("marker written: %s", e.Name())
			}
		}
	})
}

func TestRunAnswerUsageAndAnswer(t *testing.T) {
	for _, flagArg := range []string{"--help", "-h"} {
		env, _, out, errout := commandEnv(t)
		if code := RunAnswer([]string{flagArg}, env); code != 0 || !strings.Contains(out.String(), "Usage: grid answer") || errout.Len() != 0 {
			t.Fatalf("%s: code %d out %q err %q", flagArg, code, out, errout)
		}
	}
	env, s, out, errout := commandEnv(t)
	if code := RunAnswer([]string{"T-1", "Q-1"}, env); code != 2 || !strings.Contains(errout.String(), "Usage: grid answer") || out.Len() != 0 {
		t.Fatalf("short args: code %d out %q err %q", code, out, errout)
	}
	add(t, s, input())
	errout.Reset()
	if code := RunAnswer([]string{s.TaskDir, "Q-1", "JSON"}, env); code != 0 {
		t.Fatalf("answer: %d %s", code, errout)
	}
	if r := list(t, s)[0]; r.State != "answered" || r.Answer != "JSON" {
		t.Fatalf("%+v", r)
	}
}

func TestDefaultWithdrawAllowed(t *testing.T) {
	for role, want := range map[string]bool{"owner": true, "member": true, "guest": false, "agent": false, "cli": false, "": false} {
		if got := DefaultWithdrawAllowed(role); got != want {
			t.Errorf("DefaultWithdrawAllowed(%q) = %v, want %v", role, got, want)
		}
	}
}
