package question

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
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
	return fixtureStore(t, t.TempDir(), func() time.Time { return now }), &now
}

const questionTaskID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
const questionMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

func fixtureStore(t *testing.T, dir string, now func() time.Time) Store {
	t.Helper()
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + questionMachine}
	c := w
	c.Lease = "task"
	c.Fence = 1
	if err := os.MkdirAll(filepath.Join(dir, "feed", "lease"), 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(feed.Lease{Lease: "task", Holder: &w.Worker, Fence: 1})
	if err := os.WriteFile(filepath.Join(dir, "feed", "lease", "task.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("---\nid: "+questionTaskID+"\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := feed.Store{TaskDir: dir, TaskID: questionTaskID, Now: now}
	if _, err := f.Handover(feed.Input{Body: map[string]any{}}, c); err != nil {
		t.Fatal(err)
	}
	return Store{TaskDir: dir, TaskID: questionTaskID, Writer: w, ControlWriter: c, Now: now}
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
				r, e = s.Answer("Q-1", tc.text)
			} else {
				r, e = s.Withdraw("Q-1", tc.text)
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
				_, e = s.Answer("Q-1", tc.text)
			} else {
				_, e = s.Withdraw("Q-1", tc.text)
			}
			if e != nil {
				t.Fatal(e)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
				t.Fatal("repeat rewrote the question file")
			}
			if tc.action == "answer" {
				if _, e = s.Answer("Q-1", "Different answer"); e == nil {
					t.Fatal("overwrote human answer")
				}
			} else {
				if _, e = s.Answer("Q-1", "JSON"); e == nil {
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
		{"duplicate options", func(i *Input) { i.Options = []string{"CSV", "CSV"} }},
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
		if _, e := s.Answer("../Q-1", "CSV"); e == nil {
			t.Fatal("accepted traversal")
		}
	})
	t.Run("repair corruption", func(t *testing.T) {
		s, _ := setup(t)
		add(t, s, input())
		path := filepath.Join(s.TaskDir, "questions", "Q-1.md")
		if e := os.WriteFile(path, []byte("---\n{}\n---\n"), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := s.Tick(); e != nil {
			t.Fatal(e)
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
	t.Run("empty list is empty", func(t *testing.T) {
		s, _ := setup(t)
		if len(list(t, s)) != 0 {
			t.Fatal("not empty")
		}

	})
}

func TestTickProcess(t *testing.T) {
	if os.Getenv("GRID_QUESTION_HELPER") != "1" {
		return
	}
	s := Store{TaskDir: os.Getenv("GRID_QUESTION_TASK"), TaskID: questionTaskID, ControlWriter: feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + questionMachine, Lease: "task", Fence: 1}, Now: func() time.Time { return start.Add(24 * time.Hour) }}
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
	s := fixtureStore(t, task, env.Now)
	env.Writer = func(control bool) (feed.Writer, error) {
		if control {
			return s.ControlWriter, nil
		}
		return s.Writer, nil
	}
	return env, s, out, errout
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
	if _, e := s.Answer("Q-2", "Use both formats"); e != nil {
		t.Fatal(e)
	}
	in.Applies = start
	add(t, s, in)
	if _, e := s.Tick(); e != nil {
		t.Fatal(e)
	}
	in.Applies = start.Add(24 * time.Hour)
	add(t, s, in)
	if _, e := s.Withdraw("Q-4", "No longer needed"); e != nil {
		t.Fatal(e)
	}
	if code := Run([]string{"list", "--json"}, env); code != 0 {
		t.Fatalf("code %d: %s", code, errout)
	}
	var records []Record
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatal(err)
	}
	for i := range records {
		records[i].Post = questionTaskID
		for j := range records[i].History {
			records[i].History[j].Post = questionTaskID
		}
	}
	data, _ := json.Marshal(records)
	golden.Assert(t, "question-list", string(data)+"\n")
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
	if r := list(t, s)[1]; r.State != "withdrawn" || r.History[0].Who != "owner" || r.History[0].Text != "Shipped" {
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

func TestFileOnlyQuestionIsPreserved(t *testing.T) {
	for _, missingFeed := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-feed-%t", missingFeed), func(t *testing.T) {
			s, _ := setup(t)
			if missingFeed {
				if err := os.RemoveAll(filepath.Join(s.TaskDir, "feed")); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Join(s.TaskDir, "questions")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			original := []byte("---\nid: Q-1\nstate: open\n---\n\nFile-only question with an unsaved answer.\n")
			path := filepath.Join(dir, "Q-1.md")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			var warn bytes.Buffer
			s.Warn = &warn
			for _, action := range []string{"list", "add", "answer", "tick", "sync"} {
				warn.Reset()
				switch action {
				case "list":
					records := list(t, s)
					if len(records) != 1 || records[0].ID != "Q-1" || records[0].State != "not in the Feed" {
						t.Fatalf("unbacked question not listed: %+v", records)
					}
				case "add":
					if r := add(t, s, input()); r.ID != "Q-2" {
						t.Fatalf("reserved id reused: %s", r.ID)
					}
				case "answer":
					if _, err := s.Answer("Q-1", "Yes"); err == nil || !strings.Contains(err.Error(), "not in the Feed") {
						t.Fatalf("unbacked answer: %v", err)
					}
					if _, err := s.Answer("Q-2", "Yes"); err != nil {
						t.Fatal(err)
					}
				case "tick":
					if _, err := s.Tick(); err != nil {
						t.Fatal(err)
					}
				case "sync":
					if _, err := s.Sync(); err != nil {
						t.Fatal(err)
					}
				}
				if !strings.Contains(warn.String(), "Q-1.md is not in the Feed") {
					t.Fatalf("%s missing warning: %s", action, &warn)
				}
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatalf("%s changed unbacked file: %q %v", action, got, err)
				}
			}
		})
	}
}

func taskSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		out[path] = fmt.Sprintf("%v %v", info.Mode(), info.ModTime())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[path] += string(data)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestListDoesNotWrite(t *testing.T) {
	for _, scenario := range []string{"no questions", "missing feed", "damaged projection", "missing projection"} {
		t.Run(scenario, func(t *testing.T) {
			s, _ := setup(t)
			if scenario == "missing feed" {
				if err := os.RemoveAll(filepath.Join(s.TaskDir, "feed")); err != nil {
					t.Fatal(err)
				}
			}
			if strings.Contains(scenario, "projection") {
				add(t, s, input())
				path := filepath.Join(s.TaskDir, "questions", "Q-1.md")
				if scenario == "damaged projection" {
					if err := os.WriteFile(path, []byte("hand edited"), 0600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(s.TaskDir, "questions", ".lock")); err != nil {
					t.Fatal(err)
				}
			}
			before := taskSnapshot(t, s.TaskDir)
			records := list(t, s)
			if !reflect.DeepEqual(before, taskSnapshot(t, s.TaskDir)) {
				t.Fatal("list changed task files or directories")
			}
			if strings.Contains(scenario, "projection") && (len(records) != 1 || records[0].Question != input().Question) {
				t.Fatalf("list did not use Feed: %+v", records)
			}
		})
	}
}

func TestReadDefaultsWithoutTick(t *testing.T) {
	s, now := setup(t)
	add(t, s, input())
	*now = input().Applies
	before := taskSnapshot(t, s.TaskDir)
	r := list(t, s)[0]
	if r.State != "defaulted" || r.Answer != input().Default || len(r.History) != 0 {
		t.Fatalf("default needs tick: %+v", r)
	}
	if !reflect.DeepEqual(before, taskSnapshot(t, s.TaskDir)) {
		t.Fatal("default read wrote files")
	}
	if _, err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	r, err := s.Answer(r.ID, "A late human answer")
	if err != nil || r.State != "answered" || len(r.History) != 1 {
		t.Fatalf("late answer without tick: %+v %v", r, err)
	}
}

func TestCLIRoleCanTick(t *testing.T) {
	s, now := setup(t)
	add(t, s, input())
	*now = input().Applies
	s.ControlWriter.Author.Role = "cli"
	changed, err := s.Tick()
	if err != nil || len(changed) != 1 {
		t.Fatalf("cli tick: %+v %v", changed, err)
	}
	r := list(t, s)[0]
	if r.State != "defaulted" || len(r.History) != 1 || r.History[0].Role != "cli" {
		t.Fatalf("cli audit: %+v", r)
	}
}

func TestLeaseHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"withdraw", "--help"}, {"tick", "--help"}} {
		env, _, out, errout := commandEnv(t)
		if code := Run(args, env); code != 0 {
			t.Fatalf("help %v: %d", args, code)
		}
		want := "GRID_LEASE=task"
		if args[0] == "withdraw" {
			want = "A person may withdraw without a lease"
		}
		if !strings.Contains(out.String()+errout.String(), want) {
			t.Fatalf("missing task lease help: %s%s", out, errout)
		}
		golden.Assert(t, "question-help-"+strings.TrimLeft(args[0], "-"), out.String()+errout.String())
	}
}

// Without a ULID in task.md there is no Feed: files list as written, contents
// that do not decode are kept, and only a failed read is an error.
func TestListWithoutFeedID(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "questions"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("---\nid: T-01\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "questions", "Q-2.md"), []byte("not a question file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "questions", "Q-1.md"), []byte("---\n{\"id\":\"Q-1\",\"question\":\"Which?\",\"state\":\"answered\",\"answer\":\"CSV\"}\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	got, err := (Store{TaskDir: dir, Warn: &warn}).List()
	if err != nil || len(got) != 2 || got[0].ID != "Q-1" || got[0].State != "answered" || got[1].ID != "Q-2" || got[1].State != "not in the Feed" {
		t.Fatalf("list: %+v %v", got, err)
	}
	if !strings.Contains(warn.String(), "Q-2.md does not decode") {
		t.Fatalf("warning: %q", warn.String())
	}
	if err := os.Mkdir(filepath.Join(dir, "questions", "Q-3.md"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{TaskDir: dir}).List(); err == nil {
		t.Fatal("a non-regular question file listed")
	}
}

func TestWithdrawPolicyRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		agent      bool
		allowed    func(string) bool
		want       string
	}{
		{name: "default guest", role: "guest", want: `Q-1 role "guest" may not withdraw`},
		{name: "custom owner only", role: "member", allowed: func(role string) bool { return role == "owner" }, want: `Q-1 role "member" may not withdraw`},
		{name: "agent cannot bypass policy", role: "agent", allowed: func(string) bool { return true }, want: "Q-1 agent shell requires a task lease to withdraw"},
		{name: "worker shell", role: "owner", agent: true, want: "Q-1 agent shell requires a task lease to withdraw"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := setup(t)
			q := add(t, s, input())
			s.ControlWriter = feed.Writer{}
			s.Writer.Author.Role = tc.role
			s.AgentShell = tc.agent
			s.WithdrawAllowed = tc.allowed
			if _, err := s.Withdraw(q.ID, "Not needed"); !errors.Is(err, ErrWithdrawRefused) || err.Error() != tc.want {
				t.Fatalf("refusal: %v; want %q", err, tc.want)
			}
			if got := list(t, s)[0]; got.State != "open" || len(got.History) != 0 {
				t.Fatalf("refusal changed question: %+v", got)
			}
		})
	}
}

func TestWithdrawCustomPolicyAllowsPerson(t *testing.T) {
	s, _ := setup(t)
	q := add(t, s, input())
	s.ControlWriter = feed.Writer{}
	s.Writer.Author.Role = "guest"
	s.WithdrawAllowed = func(role string) bool { return role == "guest" }
	r, err := s.Withdraw(q.ID, "Resolved")
	if err != nil || r.State != "withdrawn" {
		t.Fatalf("custom withdrawal: %+v %v", r, err)
	}
	if got := list(t, s)[0]; got.State != "withdrawn" || got.History[0].Role != "guest" {
		t.Fatalf("custom projection: %+v", got)
	}
}

func TestWithdrawErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		code   int
		stderr string
	}{
		{[]string{"withdraw"}, 2, "question: withdraw requires <Q-id>\n"},
		{[]string{"withdraw", "Q-0"}, 2, "question: question id must be Q- followed by a positive number\n"},
		{[]string{"withdraw", "Q-2"}, 1, "question: question Q-2 does not exist\n"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			env, store, out, stderr := commandEnv(t)
			add(t, store, input())
			if code := Run(tc.args, env); code != tc.code || stderr.String() != tc.stderr || out.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, out.String(), stderr.String())
			}
		})
	}
}
