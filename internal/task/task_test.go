package task

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func storeFor(t *testing.T) (Store, string) {
	t.Helper()
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, "home")
	mkdir(t, home)
	return Store{Resolver: where.Resolver{GridHome: filepath.Join(home, ".grid"), Home: home, Ceiling: root},
		Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }}, home
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewReadGolden(t *testing.T) {
	for _, size := range []string{"", "auto", "small", "normal", "large"} {
		t.Run("size="+size, func(t *testing.T) {
			s, home := storeFor(t)
			words := "  Fix: the \"quoted\" label\n\nKeep my spelling & spaces.  \n"
			got, err := s.New(NewOptions{Cwd: home, Words: words, Size: size, Workflow: "Check and ship"})
			if err != nil {
				t.Fatal(err)
			}
			wantSize := size
			if wantSize == "" {
				wantSize = "auto"
			}
			if got.ID != "T-01" || got.Project != "personal" || got.Size != wantSize || got.State != "queued" || got.Intent != words {
				t.Fatalf("unexpected task: %+v", got)
			}
			if want := filepath.Join(home, ".grid", "personal", "tasks", "T-01-fix-the-quoted-label"); got.Path != want {
				t.Fatalf("path %s, want %s", got.Path, want)
			}
			read, err := Read(got.Path)
			if err != nil || !reflect.DeepEqual(read, got) {
				t.Fatalf("read %+v: %v; want %+v", read, err, got)
			}
			inside, err := s.Show(filepath.Join(got.Path, "artifacts"), "")
			if err != nil || !reflect.DeepEqual(inside, got) {
				t.Fatalf("show inside task: %+v, %v", inside, err)
			}
			byID, err := s.Show(home, got.ID)
			if err != nil || !reflect.DeepEqual(byID, got) {
				t.Fatalf("show by id: %+v, %v", byID, err)
			}
			if size == "" {
				data, err := os.ReadFile(filepath.Join(got.Path, "task.md"))
				if err != nil {
					t.Fatal(err)
				}
				golden.Assert(t, "task-record", string(data))
				data, err = os.ReadFile(filepath.Join(got.Path, "intent.md"))
				if err != nil {
					t.Fatal(err)
				}
				golden.Assert(t, "task-intent", string(data))
			}
		})
	}
}

func TestInvalidNewWritesNothing(t *testing.T) {
	for _, tc := range []NewOptions{
		{Words: " \n\t"}, {Words: "Fix", Size: "tiny"}, {Words: "Fix", Workflow: "\nrun"}, {Words: "Fix", Workflow: " "},
	} {
		t.Run(fmt.Sprintf("%q", tc), func(t *testing.T) {
			s, home := storeFor(t)
			tc.Cwd = home
			if _, err := s.New(tc); err == nil {
				t.Fatal("expected validation error")
			}
			if _, err := os.Stat(s.Resolver.GridHome); !os.IsNotExist(err) {
				t.Fatalf("validation wrote to Grid home: %v", err)
			}
		})
	}
}

func TestNextID(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		folders    []string
		bad        bool
	}{
		{name: "empty", want: "T-01"},
		{name: "all projects and personal", want: "T-101", folders: []string{"projects/a/tasks/T-09-old", "projects/b/tasks/T-100-new", "personal/tasks/T-87-personal"}},
		{name: "ignore staging", want: "T-02", folders: []string{"projects/a/tasks/.new-staging", "personal/tasks/T-01-first", "personal/tasks/notes"}},
		{name: "overflow", folders: []string{"personal/tasks/T-18446744073709551615-max"}, bad: true},
		{name: "invalid number", folders: []string{"personal/tasks/T-18446744073709551616-max"}, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := storeFor(t)
			for _, folder := range tc.folders {
				mkdir(t, filepath.Join(s.Resolver.GridHome, folder))
			}
			got, err := nextID(s.Resolver.GridHome)
			if (err != nil) != tc.bad || got != tc.want {
				t.Fatalf("got %q, %v; want %q, error=%t", got, err, tc.want, tc.bad)
			}
		})
	}
}

func TestProjectRegistration(t *testing.T) {
	for _, kind := range []string{"folder", "git", "multi-repo", "explicit"} {
		t.Run(kind, func(t *testing.T) {
			s, home := storeFor(t)
			cwd := filepath.Join(filepath.Dir(home), "work", "example")
			mkdir(t, cwd)
			option := NewOptions{Cwd: cwd, Words: "Add search"}
			switch kind {
			case "git":
				testgit.Init(t, cwd, "https://example.com/org/example.git", true)
			case "multi-repo":
				for _, name := range []string{"api", "ui"} {
					path := filepath.Join(cwd, name)
					mkdir(t, path)
					testgit.Init(t, path, "https://example.com/org/"+name+".git", true)
				}
			case "explicit":
				if err := project.Write(s.Resolver.GridHome, project.Record{Name: "example", ID: "01EXAMPLE", Keys: []project.Key{{Key: "remote:example.com/org/example"}}}); err != nil {
					t.Fatal(err)
				}
				option.Project, option.Cwd = "01EXAMPLE", home
			}
			first, err := s.New(option)
			if err != nil {
				t.Fatal(err)
			}
			second, err := s.New(option)
			if err != nil || first.Project != "example" || second.Project != first.Project || second.ID != "T-02" {
				t.Fatalf("first %+v, second %+v, error %v", first, second, err)
			}
			records, err := project.Load(s.Resolver.GridHome)
			if err != nil || len(records) != 1 || records[0].ID == "" {
				t.Fatalf("records %+v, error %v", records, err)
			}
			rec := records[0]
			if kind == "explicit" && (rec.ID != "01EXAMPLE" || len(rec.Keys) != 1) {
				t.Fatalf("existing record changed: %+v", rec)
			}
			if kind == "git" && (len(rec.Repos) != 1 || rec.Repos[0].Path != "." || rec.Repos[0].Remote != "example.com/org/example") {
				t.Fatalf("git record %+v", rec)
			}
			if kind == "multi-repo" && len(rec.Repos) != 2 {
				t.Fatalf("multi-repo record %+v", rec)
			}
			s.Resolver.Machine = gridhome.Machine(s.Resolver.GridHome)
			if _, err := s.Show(first.Path, first.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(cwd, "task.md")); !os.IsNotExist(err) {
				t.Fatalf("task data in working folder: %v", err)
			}
		})
	}
}

func TestListAndReadErrors(t *testing.T) {
	s, home := storeFor(t)
	first, err := s.New(NewOptions{Cwd: home, Words: "First task"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.New(NewOptions{Cwd: home, Words: "Second task"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(first.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(first.Path, "task.md"), strings.Replace(string(data), "state: queued", "state: closed", 1))
	for _, tc := range []struct {
		state string
		ids   []string
	}{{"", []string{"T-01", "T-02"}}, {"closed", []string{"T-01"}}, {"queued", []string{"T-02"}}, {"missing", []string{}}} {
		t.Run(tc.state, func(t *testing.T) {
			got, err := s.List(home, "personal", tc.state)
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, task := range got {
				ids = append(ids, task.ID)
			}
			if !reflect.DeepEqual(ids, tc.ids) {
				t.Fatalf("ids %v, want %v", ids, tc.ids)
			}
		})
	}
	for _, id := range []string{"", "../T-01", "T-99", "T-01-first-task"} {
		if _, err := s.Show(home, id); err == nil {
			t.Fatalf("show %q should fail", id)
		}
	}
	write(t, filepath.Join(second.Path, "task.md"), "---\ntitle: [invalid\n---\n")
	if _, err := s.List(home, "", ""); err == nil {
		t.Fatal("malformed task was silently skipped")
	}
}

func TestFold(t *testing.T) {
	valid := "---\ntitle: 'A: quoted title'\nproject: personal\nsize: normal\nworkflow: ''\nstate: queued\ncreated: 2026-01-02T03:04:05Z\n---\nbody\n"
	for _, tc := range []struct {
		name, text string
		bad        bool
	}{
		{"valid", valid, false},
		{"crlf", strings.ReplaceAll(valid, "\n", "\r\n"), false},
		{"no front matter", "title: Example", true},
		{"unclosed", "---\ntitle: Example", true},
		{"invalid yaml", "---\ntitle: [\n---\n", true},
		{"mismatched id", strings.Replace(valid, "title:", "id: T-02\ntitle:", 1), true},
		{"invalid time", strings.Replace(valid, "2026-01-02T03:04:05Z", "yesterday", 1), true},
		{"missing state", strings.Replace(valid, "state: queued\n", "", 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Fold("T-01-example", tc.text, "words\r\n")
			if (err != nil) != tc.bad {
				t.Fatalf("fold %+v, %v", got, err)
			}
			if !tc.bad && (got.ID != "T-01" || got.Intent != "words\r\n") {
				t.Fatalf("fold %+v", got)
			}
		})
	}
}

func TestNewRefusesSymlinksAndFormat(t *testing.T) {
	for _, kind := range []string{"symlink", "old format", "new format", "bad format", "unknown project"} {
		t.Run(kind, func(t *testing.T) {
			s, home := storeFor(t)
			o := NewOptions{Cwd: home, Words: "Example task"}
			switch kind {
			case "symlink":
				mkdir(t, s.Resolver.GridHome)
				outside := filepath.Join(home, "outside")
				mkdir(t, outside)
				if err := os.Symlink(outside, filepath.Join(s.Resolver.GridHome, "personal")); err != nil {
					t.Fatal(err)
				}
			case "old format":
				write(t, filepath.Join(s.Resolver.GridHome, "VERSION"), "0\n")
			case "new format":
				write(t, filepath.Join(s.Resolver.GridHome, "VERSION"), "99\n")
			case "bad format":
				write(t, filepath.Join(s.Resolver.GridHome, "VERSION"), "bad\n")
			case "unknown project":
				o.Project = "missing"
			}
			if _, err := s.New(o); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}

// The child waits for a byte from the parent before touching the shared home.
// Two independently running processes repeatedly contend for one allocation lock.
func TestTwoProcessIDRace(t *testing.T) {
	if os.Getenv("GRID_TASK_RACE_CHILD") == "1" {
		var start [1]byte
		if _, err := io.ReadFull(os.Stdin, start[:]); err != nil {
			t.Fatal(err)
		}
		home := os.Getenv("GRID_TASK_RACE_HOME")
		cwd := os.Getenv("GRID_TASK_RACE_CWD")
		s := Store{Resolver: where.Resolver{GridHome: filepath.Join(home, ".grid"), Home: home, Ceiling: filepath.Dir(home)}}
		for i := 0; i < 12; i++ {
			proj := ""
			if i%2 == 0 {
				proj = "personal"
			}
			if _, err := s.New(NewOptions{Cwd: cwd, Words: "Race task", Project: proj}); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	s, home := storeFor(t)
	cwd := filepath.Join(filepath.Dir(home), "example")
	mkdir(t, cwd)
	type child struct {
		cmd   *exec.Cmd
		start io.WriteCloser
		out   bytes.Buffer
	}
	children := make([]child, 2)
	for i := range children {
		c := &children[i]
		c.cmd = exec.Command(os.Args[0], "-test.run=^TestTwoProcessIDRace$", "-test.timeout=30s")
		c.cmd.Env = append(os.Environ(), "GRID_TASK_RACE_CHILD=1", "GRID_TASK_RACE_HOME="+home, "GRID_TASK_RACE_CWD="+cwd)
		c.cmd.Stdout, c.cmd.Stderr = &c.out, &c.out
		var err error
		c.start, err = c.cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.cmd.Process.Kill() })
	}
	for i := range children {
		if _, err := children[i].start.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		_ = children[i].start.Close()
	}
	for i := range children {
		if err := children[i].cmd.Wait(); err != nil {
			t.Fatalf("child %d: %v\n%s", i, err, &children[i].out)
		}
	}
	s.Resolver.Machine = gridhome.Machine(s.Resolver.GridHome)
	seen := map[string]bool{}
	for _, proj := range []string{"personal", "example"} {
		tasks, err := s.List(home, proj, "")
		if err != nil || len(tasks) != 12 {
			t.Fatalf("%s: %d tasks, error %v", proj, len(tasks), err)
		}
		for _, task := range tasks {
			if seen[task.ID] || task.Intent != "Race task" {
				t.Fatalf("duplicate id or incomplete task: %+v", task)
			}
			seen[task.ID] = true
		}
	}
	for i := 1; i <= 24; i++ {
		if !seen[fmt.Sprintf("T-%02d", i)] {
			t.Fatalf("missing id T-%02d", i)
		}
	}
}
