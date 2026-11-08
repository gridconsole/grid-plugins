package question

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/testenv"
	"github.com/gridconsole/grid-plugins/internal/where"
)

var start = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

func input() Input {
	return Input{Question: "Which format?", Who: "owner", Options: []string{"CSV", "JSON"}, Recommended: "CSV", Default: "CSV", Applies: start.Add(24 * time.Hour), Blocking: true, AgeAfter: time.Hour}
}
func setup(t *testing.T) (Store, *time.Time) {
	t.Helper()
	now := start
	return Store{TaskDir: t.TempDir(), Now: func() time.Time { return now }}, &now
}
func add(t *testing.T, s Store, in Input) Record {
	t.Helper()
	r, e := s.Add(in)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func list(t *testing.T, s Store) []Record {
	t.Helper()
	r, e := s.List()
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestTick(t *testing.T) {
	for _, tc := range []struct {
		name          string
		elapsed       time.Duration
		blocking      bool
		state         string
		aging, events int
	}{
		{"before aging", time.Hour - time.Nanosecond, true, "open", 0, 0},
		{"first aging", time.Hour, true, "open", 1, 1},
		{"second aging", 2 * time.Hour, true, "open", 2, 2},
		{"aging capped", 20 * time.Hour, true, "open", 2, 2},
		{"nonblocking", 20 * time.Hour, false, "open", 0, 0},
		{"before default", 24*time.Hour - time.Nanosecond, false, "open", 0, 0},
		{"due default", 24 * time.Hour, true, "defaulted", 0, 1},
		{"overdue default", 48 * time.Hour, false, "defaulted", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, now := setup(t)
			in := input()
			in.Blocking = tc.blocking
			add(t, s, in)
			*now = start.Add(tc.elapsed)
			changed, e := s.Tick()
			if e != nil {
				t.Fatal(e)
			}
			r := list(t, s)[0]
			if r.State != tc.state || r.Aging != tc.aging || len(r.History) != tc.events {
				t.Fatalf("unexpected record: %+v", r)
			}
			if (len(changed) > 0) != (tc.events > 0) {
				t.Fatal("wrong changed list")
			}
			before, e := os.ReadFile(filepath.Join(s.TaskDir, "questions", "Q-1.md"))
			if e != nil {
				t.Fatal(e)
			}
			changed, e = s.Tick()
			if e != nil || len(changed) != 0 {
				t.Fatalf("second tick: %v %v", changed, e)
			}
			after, _ := os.ReadFile(filepath.Join(s.TaskDir, "questions", "Q-1.md"))
			if !bytes.Equal(before, after) {
				t.Fatal("idempotent tick rewrote file")
			}
		})
	}
}

func TestTransitions(t *testing.T) {
	for _, tc := range []struct {
		name, action, text, state string
		due                       bool
		events                    int
	}{
		{"option answer", "answer", "JSON", "answered", false, 1},
		{"free answer", "answer", "Use both formats", "answered", false, 1},
		{"late answer", "answer", "JSON", "answered", true, 2},
		{"late same answer", "answer", "CSV", "answered", true, 2},
		{"withdraw", "withdraw", "Not needed", "withdrawn", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, now := setup(t)
			add(t, s, input())
			if tc.due {
				*now = start.Add(24 * time.Hour)
				if _, e := s.Tick(); e != nil {
					t.Fatal(e)
				}
			}
			var r Record
			var e error
			if tc.action == "answer" {
				r, e = s.Answer("Q-1", tc.text, "owner")
			} else {
				r, e = s.Withdraw("Q-1", tc.text, "owner")
			}
			if e != nil {
				t.Fatal(e)
			}
			if r.State != tc.state || len(r.History) != tc.events {
				t.Fatalf("%+v", r)
			}
			if tc.due && r.History[0].Kind != "defaulted" {
				t.Fatal("lost default audit")
			}
			if last := r.History[len(r.History)-1]; last.Who != "owner" {
				t.Fatalf("attributed to %q", last.Who)
			}
			path := filepath.Join(s.TaskDir, "questions", "Q-1.md")
			before, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			*now = now.Add(time.Hour)
			if tc.action == "answer" {
				_, e = s.Answer("Q-1", tc.text, "owner")
			} else {
				_, e = s.Withdraw("Q-1", tc.text, "owner")
			}
			if e != nil {
				t.Fatal(e)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
				t.Fatal("repeat rewrote the question file")
			}
			if tc.action == "answer" {
				if _, e = s.Answer("Q-1", "Different answer", "owner"); e == nil {
					t.Fatal("overwrote human answer")
				}
			} else {
				if _, e = s.Answer("Q-1", "JSON", "owner"); e == nil {
					t.Fatal("answered withdrawn question")
				}
			}
			*now = start.Add(72 * time.Hour)
			changed, e := s.Tick()
			if e != nil || len(changed) != 0 {
				t.Fatalf("tick reopened closed question: %v %v", changed, e)
			}
		})
	}
}

func TestInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Input)
	}{
		{"empty question", func(i *Input) { i.Question = " " }}, {"empty who", func(i *Input) { i.Who = "" }},
		{"no options", func(i *Input) { i.Options = nil }}, {"duplicate options", func(i *Input) { i.Options = []string{"CSV", "CSV"} }},
		{"unknown recommendation", func(i *Input) { i.Recommended = "XML" }}, {"no default", func(i *Input) { i.Default = "" }},
		{"no deadline", func(i *Input) { i.Applies = time.Time{} }}, {"negative aging", func(i *Input) { i.AgeAfter = -time.Hour }},
		{"blank why", func(i *Input) { i.Why = " " }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := setup(t)
			i := input()
			tc.change(&i)
			if _, e := s.Add(i); e == nil {
				t.Fatal("accepted invalid input")
			}
			if _, e := os.Stat(filepath.Join(s.TaskDir, "questions")); !os.IsNotExist(e) {
				t.Fatal("invalid input created storage")
			}
		})
	}
}

func TestStorage(t *testing.T) {
	t.Run("numeric ordering and id allocation", func(t *testing.T) {
		s, _ := setup(t)
		for i := 1; i <= 12; i++ {
			add(t, s, input())
		}
		records := list(t, s)
		for i, r := range records {
			if r.ID != fmt.Sprintf("Q-%d", i+1) {
				t.Fatal("not numeric order")
			}
		}
		data, e := os.ReadFile(filepath.Join(s.TaskDir, "questions", "Q-1.md"))
		if e != nil {
			t.Fatal(e)
		}
		for _, text := range []string{"Default: CSV", "Applies: 2026-11-09T10:00:00.000Z", "CSV (Recommended)", "to: owner", "blocking"} {
			if !bytes.Contains(data, []byte(text)) {
				t.Fatalf("missing %s", text)
			}
		}
	})
	t.Run("task ancestor symlink", func(t *testing.T) {
		s, _ := setup(t)
		link := filepath.Join(t.TempDir(), "task")
		if e := os.Symlink(s.TaskDir, link); e != nil {
			t.Fatal(e)
		}
		s.TaskDir = link
		add(t, s, input())
		if len(list(t, s)) != 1 {
			t.Fatal("missing record")
		}
	})
	for _, target := range []string{"questions", "questions/.lock", "questions/Q-1.md"} {
		t.Run("reject symlink "+target, func(t *testing.T) {
			s, _ := setup(t)
			outside := t.TempDir()
			if target != "questions" {
				if e := os.Mkdir(filepath.Join(s.TaskDir, "questions"), 0700); e != nil {
					t.Fatal(e)
				}
				outside = filepath.Join(outside, "file")
				if e := os.WriteFile(outside, []byte("unchanged"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			if e := os.Symlink(outside, filepath.Join(s.TaskDir, target)); e != nil {
				t.Fatal(e)
			}
			if _, e := s.Add(input()); e == nil {
				t.Fatal("followed storage symlink")
			}
		})
	}
	t.Run("reject traversal", func(t *testing.T) {
		s, _ := setup(t)
		add(t, s, input())
		if _, e := s.Answer("../Q-1", "CSV", ""); e == nil {
			t.Fatal("accepted traversal")
		}
	})
	t.Run("reject corruption", func(t *testing.T) {
		s, _ := setup(t)
		add(t, s, input())
		path := filepath.Join(s.TaskDir, "questions", "Q-1.md")
		if e := os.WriteFile(path, []byte("---\n{}\n---\n"), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := s.Tick(); e == nil {
			t.Fatal("silently skipped corrupt question")
		}
	})
	t.Run("skip old per-recipient file", func(t *testing.T) {
		s, now := setup(t)
		var warn bytes.Buffer
		s.Warn = &warn
		dir := filepath.Join(s.TaskDir, "questions")
		if e := os.Mkdir(dir, 0700); e != nil {
			t.Fatal(e)
		}
		legacy := []byte("## Q-2 · to: sam · not blocking · open\nasked: 2026-03-04T10:30 · by grid-planner\n\nShould the tag box show recent tags?\nOptions: Yes, five / Yes, ten / No\nRecommendation: yes, five, to save typing.\nAnswer: —\n")
		if e := os.WriteFile(filepath.Join(dir, "sam.md"), legacy, 0600); e != nil {
			t.Fatal(e)
		}
		const line = "question: skipping files in questions/ that are not Q-n.md: sam.md\n"
		for _, step := range []struct {
			name string
			call func() error
		}{
			{"list", func() error { _, e := s.List(); return e }},
			{"add", func() error { _, e := s.Add(input()); return e }},
			{"tick", func() error { *now = start.Add(24 * time.Hour); _, e := s.Tick(); return e }},
		} {
			warn.Reset()
			if e := step.call(); e != nil {
				t.Fatalf("%s: %v", step.name, e)
			}
			if warn.String() != line {
				t.Fatalf("%s warning: %q", step.name, warn.String())
			}
		}
		if r := list(t, s); len(r) != 1 || r[0].ID != "Q-1" || r[0].State != "defaulted" {
			t.Fatalf("%+v", r)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "sam.md")); !bytes.Equal(got, legacy) {
			t.Fatal("legacy file changed")
		}
		if e := os.Remove(filepath.Join(dir, "sam.md")); e != nil {
			t.Fatal(e)
		}
		warn.Reset()
		list(t, s)
		if warn.Len() != 0 {
			t.Fatalf("warned without skipped files: %q", warn.String())
		}
	})
	t.Run("empty list does not create storage", func(t *testing.T) {
		s, _ := setup(t)
		if len(list(t, s)) != 0 {
			t.Fatal("not empty")
		}
		if _, e := os.Stat(filepath.Join(s.TaskDir, "questions")); !os.IsNotExist(e) {
			t.Fatal("created folder")
		}
	})
}

func TestTickProcess(t *testing.T) {
	if os.Getenv("GRID_QUESTION_HELPER") != "1" {
		return
	}
	s := Store{TaskDir: os.Getenv("GRID_QUESTION_TASK"), Now: func() time.Time { return start.Add(24 * time.Hour) }}
	// Parent starts both children before releasing their stdin barriers.
	var b [1]byte
	if _, e := os.Stdin.Read(b[:]); e != nil {
		t.Fatal(e)
	}
	changed, e := s.Tick()
	if e != nil {
		t.Fatal(e)
	}
	fmt.Printf("changed=%d\n", len(changed))
}

func TestConcurrentProcessesTick(t *testing.T) {
	s, _ := setup(t)
	add(t, s, input())
	cmds := make([]*exec.Cmd, 2)
	outputs := make([]bytes.Buffer, 2)
	releases := make([]func(), 2)
	for i := range cmds {
		cmd := exec.Command(os.Args[0], "-test.run=^TestTickProcess$")
		cmd.Env = append(testenv.Temp(t, os.Environ()), "GRID_QUESTION_HELPER=1", "GRID_QUESTION_TASK="+s.TaskDir)
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		stdin, e := cmd.StdinPipe()
		if e != nil {
			t.Fatal(e)
		}
		releases[i] = func() {
			if _, e := stdin.Write([]byte("x")); e != nil {
				t.Error(e)
			}
			_ = stdin.Close()
		}
		cmds[i] = cmd
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		})
	}
	for _, release := range releases {
		release()
	}
	count := 0
	for i, cmd := range cmds {
		if e := cmd.Wait(); e != nil {
			t.Fatalf("%v: %s", e, outputs[i].String())
		}
		if strings.Contains(outputs[i].String(), "changed=1") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("default applied by %d processes", count)
	}
	r := list(t, s)[0]
	if r.State != "defaulted" || len(r.History) != 1 || r.History[0].Kind != "defaulted" {
		t.Fatalf("%+v", r)
	}
}

func commandEnv(t *testing.T) (Env, Store, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	task := filepath.Join(home, "projects", "example", "tasks", "T-1-example")
	cwd := filepath.Join(task, "work")
	if e := os.MkdirAll(cwd, 0700); e != nil {
		t.Fatal(e)
	}
	out, errout := &bytes.Buffer{}, &bytes.Buffer{}
	env := Env{Cwd: cwd, Resolver: where.Resolver{GridHome: home}, Stdout: out, Stderr: errout, Now: func() time.Time { return start }}
	return env, Store{TaskDir: task, Now: env.Now}, out, errout
}

func TestQuestionListGolden(t *testing.T) {
	env, s, out, errout := commandEnv(t)
	in := input()
	in.Why = "The export feeds the billing import."
	in.Applies = in.Applies.Add(250*time.Millisecond + 999*time.Microsecond)
	add(t, s, in)
	in = input()
	in.Blocking = false
	in.Who = "reviewer"
	add(t, s, in)
	if _, e := s.Answer("Q-2", "Use both formats", "reviewer"); e != nil {
		t.Fatal(e)
	}
	in.Applies = start
	add(t, s, in)
	if _, e := s.Tick(); e != nil {
		t.Fatal(e)
	}
	in.Applies = start.Add(24 * time.Hour)
	add(t, s, in)
	if _, e := s.Withdraw("Q-4", "No longer needed", ""); e != nil {
		t.Fatal(e)
	}
	if code := Run([]string{"list", "--json"}, env); code != 0 {
		t.Fatalf("code %d: %s", code, errout)
	}
	golden.Assert(t, "question-list", out.String())
}

func TestCommand(t *testing.T) {
	env, s, out, errout := commandEnv(t)
	calls := [][]string{
		{"add", "Which format?", "--who", "owner", "--option", "CSV", "--option", "JSON", "--recommended", "CSV", "--default", "CSV", "--applies", "2026-11-09T10:00:00Z", "--blocking"},

		{"tick", "--task", s.TaskDir},
		{"add", "Ship today?", "--option", "Yes", "--recommended", "Yes", "--default", "Yes", "--applies", "2026-11-09T10:00:00Z"},
		{"withdraw", "Q-2", "--reason", "Shipped"},
	}
	for _, args := range calls {
		out.Reset()
		errout.Reset()
		if code := Run(args, env); code != 0 {
			t.Fatalf("%v: %d %s", args, code, errout)
		}
	}
	if code := RunAnswer([]string{s.TaskDir, "Q-1", "JSON", "--json"}, env); code != 0 {
		t.Fatalf("answer: %d %s", code, errout)
	}
	if r := list(t, s)[0]; r.State != "answered" || r.Answer != "JSON" {
		t.Fatal(r)
	}
	if r := list(t, s)[1]; r.State != "withdrawn" || r.History[0].Who != "cli" || r.History[0].Text != "Shipped" {
		t.Fatal(r)
	}
	for _, args := range [][]string{nil, {"bad"}, {"answer"}, {"withdraw"}, {"list", "extra"}, {"add", "--applies", "bad"}, {"list", "--bad"}} {
		if code := Run(args, env); code != 2 {
			t.Fatalf("%v: code %d", args, code)
		}
	}
	env.Cwd = env.Resolver.GridHome
	if code := Run([]string{"list"}, env); code != 1 {
		t.Fatalf("no task: code %d", code)
	}
	out.Reset()
	if code := Run([]string{"list", "--task", s.TaskDir, "--json"}, env); code != 0 {
		t.Fatalf("explicit task: %d %s", code, errout)
	}
	var records []Record
	if e := json.Unmarshal(out.Bytes(), &records); e != nil || len(records) != 2 {
		t.Fatalf("%v %s", e, out)
	}
}

func TestConcurrentAdds(t *testing.T) {
	s, _ := setup(t)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for range 12 {
		wg.Go(func() { _, e := s.Add(input()); errs <- e })
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	records := list(t, s)
	if len(records) != 12 {
		t.Fatalf("lost questions: %d", len(records))
	}
	for i, r := range records {
		if r.ID != fmt.Sprintf("Q-%d", i+1) {
			t.Fatal("duplicate or missing id")
		}
	}
}
