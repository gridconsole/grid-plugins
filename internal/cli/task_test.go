package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func TestTaskNewFeedCommands(t *testing.T) {
	h := newHarness(t)
	h.env.Machine = "" // Let task new allocate the real machine identity too.
	for _, key := range []string{"GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_WAKE", "GRID_WORKER", "GRID_FEED_TASK", "GRID_JOB", "GRID_ATTEMPT"} {
		t.Setenv(key, "")
	}
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", "owner") // Personal is owner-only (L67R).
	if code := h.run(h.home, "task", "new", "Use the Feed", "--json"); code != 0 {
		t.Fatalf("new: %d %s", code, h.errb)
	}
	var created struct{ ID, Path string }
	if err := json.Unmarshal(h.out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "T-01" {
		t.Fatalf("human id: %q", created.ID)
	}
	if code := h.run(h.home, "feed", "state", "--task", created.ID, "--project", "personal", "--json"); code != 0 {
		t.Fatalf("state: %d %s", code, h.errb)
	}
	stableID, err := feed.TaskID(created.Path)
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(created.Path, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"Ready"}`, "--json"); code != 0 {
		t.Fatalf("post: %d %s", code, h.errb)
	}
	var posted struct{ Post feed.Record }
	if err := json.Unmarshal(h.out.Bytes(), &posted); err != nil {
		t.Fatal(err)
	}
	if posted.Post.Task != stableID {
		t.Fatalf("post task: %q, want %q", posted.Post.Task, stableID)
	}
	if code := h.run(created.Path, "feed", "state", "--json"); code != 0 || !strings.Contains(h.out.String(), posted.Post.ID) {
		t.Fatalf("state after post: %d %s %s", code, h.out, h.errb)
	}
	// job next needs a granted task lease even when there are no jobs yet.
	machine, err := feed.MachineID(h.gridHome)
	if err != nil {
		t.Fatal(err)
	}
	store := feed.Store{TaskDir: created.Path, TaskID: stableID}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + machine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(h.home, "job", "next", "--task", created.ID, "--project", "personal", "--lease-epoch", fmt.Sprintf("0.%d", grant.Fence), "--json"); code != 0 {
		t.Fatalf("next: %d %s", code, h.errb)
	}
	if after, err := feed.TaskID(created.Path); err != nil || after != stableID {
		t.Fatalf("stable id changed: %q, %v", after, err)
	}
}

func taskFixture(t *testing.T, h *harness) task.Task {
	t.Helper()
	return taskFixtureProject(t, h, "")
}

func taskFixtureProject(t *testing.T, h *harness, name string) task.Task {
	t.Helper()
	s := task.Store{Resolver: resolver(h.env), Now: func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	}}
	got, err := s.New(task.NewOptions{Cwd: h.home, Project: name, Words: "Fix the label\n\nKeep these exact words.\n", Size: "small", Workflow: "Check and ship", Who: "alex", Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTaskGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"task-new-json", []string{"task", "new", "Fix the label", "--json"}},
		{"task-new-plain", []string{"task", "new", "Fix the label"}},
		{"task-show-json", []string{"task", "show", "T-01", "--json"}},
		{"task-show-plain", []string{"task", "show", "T-01"}},
		{"task-list-json", []string{"task", "list", "--json"}},
		{"task-list-plain", []string{"task", "list"}},
		{"task-list-filtered-json", []string{"task", "list", "--state", "closed", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.args[1] != "new" {
				taskFixture(t, h)
			}
			if code := h.run(h.home, tc.args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, tc.name, h.norm(h.out.String()))
		})
	}
}

func TestTaskFlagsAndIntent(t *testing.T) {
	for _, tc := range []struct {
		name, words, size, workflow string
		args                        []string
	}{
		{"flags after words", "  Keep: my words\n\nexactly. ", "large", "Review and publish", []string{"  Keep: my words\n\nexactly. ", "--size", "large", "--workflow", "Review and publish", "--json"}},
		{"flags before words", "Example task", "small", "", []string{"--size", "small", "--json", "Example task"}},
		{"mixed flags", "Example task", "normal", "docs", []string{"--size=normal", "Example task", "--workflow=docs", "--json"}},
		{"protected words", "--fix this", "auto", "", []string{"--json", "--", "--fix this"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, append([]string{"task", "new"}, tc.args...)...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			var output struct{ ID, Path string }
			if err := json.Unmarshal(h.out.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			got, err := task.Read(output.Path)
			if err != nil || got.ID != output.ID || got.Intent != tc.words || got.Size != tc.size || got.Workflow != tc.workflow {
				t.Fatalf("got %+v, error %v", got, err)
			}
			if code := h.run(filepath.Join(output.Path, "artifacts"), "task", "show", "--json"); code != 0 {
				t.Fatalf("show inside task: %s", h.errb)
			}
			var shown task.Task
			if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil || shown.Intent != tc.words {
				t.Fatalf("show %+v: %v", shown, err)
			}
		})
	}
}

func TestTaskNewRecordsTheOwnerFromTheEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name        string
		env         map[string]string
		who, role   string
		wantMachine bool
	}{
		{name: "GRID_WHO and GRID_ROLE", env: map[string]string{"GRID_WHO": "alex", "GRID_ROLE": "lead"}, who: "alex", role: "lead"},
		{name: "GRID_ACTOR names the owner when GRID_WHO is not set", env: map[string]string{"GRID_ACTOR": "alex"}, who: "alex", role: "member"},
		{name: "GRID_WHO wins over GRID_ACTOR", env: map[string]string{"GRID_WHO": "alex", "GRID_ACTOR": "sam"}, who: "alex", role: "member"},
		{name: "GRID_WHO alone is a member", env: map[string]string{"GRID_WHO": "alex"}, who: "alex", role: "member"},
		{name: "nothing set is the machine owner", env: map[string]string{}, wantMachine: true, role: "owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			h.env.Getenv = func(k string) string { return tc.env[k] }
			if code := h.run(h.home, "task", "new", "Add a search label", "--project", "example", "--json"); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			var output struct{ ID, Path string }
			if err := json.Unmarshal(h.out.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			got, err := task.Read(output.Path)
			if err != nil || got.Owner == "" || got.OwnerRole != tc.role || (!tc.wantMachine && got.Owner != tc.who) {
				t.Fatalf("got %+v, %v; want owner %q role %q", got, err, tc.who, tc.role)
			}
		})
	}
}

func TestTaskProjectAndPersonalSelection(t *testing.T) {
	for _, selection := range []string{"inferred", "explicit name", "explicit id", "personal"} {
		t.Run(selection, func(t *testing.T) {
			h := newHarness(t)
			cwd := filepath.Join(h.root, "work", "example")
			h.mkdir(cwd)
			testgit.Init(t, cwd, "https://example.com/org/example.git", true)
			args := []string{"task", "new", "Add search", "--json"}
			wantProject := "example"
			switch selection {
			case "explicit name", "explicit id":
				h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
				arg := "example"
				if selection == "explicit id" {
					arg = "01EXAMPLE"
				}
				args = append(args, "--project", arg)
				cwd = h.home
			case "personal":
				args = append(args, "--project", "Personal")
				wantProject = "personal"
			}
			if code := h.run(cwd, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			var output struct{ ID, Path string }
			if err := json.Unmarshal(h.out.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			got, err := task.Read(output.Path)
			if err != nil || got.Project != wantProject {
				t.Fatalf("task %+v, %v", got, err)
			}
			if code := h.run(h.home, "task", "list", "--project", wantProject, "--state", "queued", "--json"); code != 0 {
				t.Fatalf("list exit %d: %s", code, h.errb)
			}
			var list []task.Task
			if err := json.Unmarshal(h.out.Bytes(), &list); err != nil || len(list) != 1 || list[0].ID != output.ID {
				t.Fatalf("list %+v, %v", list, err)
			}
			if code := h.run(output.Path, "task", "show", "--json"); code != 0 {
				t.Fatalf("show exit %d: %s", code, h.errb)
			}
			if code := h.run(h.home, "task", "show", output.ID, "--json"); code != 0 {
				t.Fatalf("show from another folder exit %d: %s", code, h.errb)
			}
		})
	}
}

func TestTaskUsageAndFailures(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"task"}, 2},
		{[]string{"task", "unknown"}, 2},
		{[]string{"task", "new"}, 2},
		{[]string{"task", "new", "one", "two"}, 2},
		{[]string{"task", "new", " "}, 2},
		{[]string{"task", "new", "Example", "--size", "tiny"}, 2},
		{[]string{"task", "new", "Example", "--workflow"}, 2},
		{[]string{"task", "new", "Example", "--unknown"}, 2},
		{[]string{"task", "new", "Example", "--project", "missing"}, 1},
		{[]string{"task", "show"}, 1},
		{[]string{"task", "show", "T-99", "--json"}, 1},
		{[]string{"task", "show", "T-01", "T-02"}, 2},
		{[]string{"task", "list", "extra"}, 2},
		{[]string{"task", "list", "--project", "missing"}, 1},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, tc.args...); code != tc.code || h.errb.Len() == 0 {
				t.Fatalf("exit %d; stdout %q; stderr %q", code, h.out, h.errb)
			}
			errorCode := ""
			if jsonRequested(tc.args) {
				errorCode = CodeNoSuchTask
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), errorCode)
		})
	}
}

func TestTaskReadsDoNotWrite(t *testing.T) {
	h := newHarness(t)
	h.env.GridHome = filepath.Join(h.home, ".absent")
	if code := h.run(h.home, "task", "list", "--json"); code != 0 || h.out.String() != "[]\n" {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.home, "task", "show", "T-01"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
		t.Fatalf("reads created Grid home: %v", err)
	}
	h.env.GridHome = ""
	for _, args := range [][]string{{"task", "new", "Example"}, {"task", "show"}, {"task", "list"}} {
		if code := h.run(h.home, args...); code != 1 || !strings.Contains(h.errb.String(), "GRID_HOME") {
			t.Fatalf("%v: exit %d, %s", args, code, h.errb)
		}
	}
}

func TestTaskAttachGolden(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		name := "task-attach-plain"
		args := []string{"task", "attach", "T-01", "--session", "terminal-example"}
		if asJSON {
			name = "task-attach-json"
			args = append(args, "--json")
		}
		t.Run(name, func(t *testing.T) {
			h := newTaskAttachHarness(t)
			h.env.Getenv = func(string) string { return "" }
			taskFixture(t, h)
			h.write(filepath.Join(h.gridHome, "machine"), "01ARZ3NDEKTSV4RRFFQ69G5FAV\n")
			if code := h.run(h.home, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, name, h.out.String())
		})
	}
}

func TestTaskAttachSessionSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{"explicit before", map[string]string{"GRID_SESSION_ID": "grid", "CLAUDE_SESSION_ID": "claude"}, []string{"--session", "explicit", "T-01"}, "explicit"},
		{"explicit after", map[string]string{"GRID_SESSION_ID": "grid"}, []string{"T-01", "--session=explicit"}, "explicit"},
		{"grid before claude", map[string]string{"GRID_SESSION_ID": "grid", "CLAUDE_SESSION_ID": "claude"}, []string{"T-01"}, "grid"},
		{"claude fallback", map[string]string{"CLAUDE_SESSION_ID": "claude"}, []string{"T-01"}, "claude"},
		{"generated", map[string]string{}, []string{"T-01"}, ""},
		{"protected operand", map[string]string{}, []string{"--session", "protected", "--", "T-01"}, "protected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTaskAttachHarness(t)
			h.env.Getenv = func(key string) string { return tc.env[key] }
			task := taskFixture(t, h)
			args := append([]string{"task", "attach", "--json"}, tc.args...)
			if code := h.run(h.home, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			records, err := (session.Store{TaskDir: task.Path}).List()
			if err != nil || len(records) != 1 {
				t.Fatalf("records: %+v %v", records, err)
			}
			got := records[0]
			if tc.want != "" && got.SessionID != tc.want || tc.want == "" && !feed.IsID(got.SessionID) {
				t.Fatalf("id: %s", got.SessionID)
			}
			if got.PID != os.Getppid() || got.PIDStart == "" || got.PIDStart != migrate.ProcessStart(os.Getppid()) || got.Where != "terminal" || got.Cwd != h.home || !feed.IsID(got.MachineID) {
				t.Fatalf("session: %+v", got)
			}
			started := got.Started
			if code := h.run(h.home, "task", "attach", task.ID, "--session", got.SessionID); code != 0 {
				t.Fatalf("resume: %s", h.errb)
			}
			resumed, err := (session.Store{TaskDir: task.Path}).Show(got.Key)
			if err != nil || !resumed.Started.Equal(started) {
				t.Fatalf("resume: %+v %v", resumed, err)
			}
		})
	}
}

func TestTaskAttachErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		code  int
		cause string
	}{
		{"missing task", nil, 2, "Usage:"},
		{"extra operand", []string{"T-01", "T-02"}, 2, "Usage:"},
		{"unknown task", []string{"T-99"}, 1, "T-99"},
		{"closed task", []string{"T-01"}, 1, "grid task reopen T-01"},
		{"unknown flag", []string{"T-01", "--unknown"}, 2, "flag provided"},
		{"file out of scope", []string{"T-01", "--file", "example.txt"}, 2, "flag provided"},
		{"note out of scope", []string{"T-01", "--note", "example"}, 2, "flag provided"},
		{"missing session", []string{"T-01", "--session"}, 2, "flag needs an argument"},
		{"bad session", []string{"T-01", "--session", "../outside"}, 1, "session and machine ids"},
		{"bad runtime", []string{"T-01", "--runtime", "other"}, 2, "--runtime must be claude"},
		{"protected dash", []string{"--", "-task"}, 1, "invalid task id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTaskAttachHarness(t)
			h.env.Getenv = func(string) string { return "" }
			got := taskFixture(t, h)
			if tc.name == "closed task" {
				if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: got.ID, Action: "close"}); err != nil {
					t.Fatal(err)
				}
			}
			if code := h.run(h.home, append([]string{"task", "attach"}, tc.args...)...); code != tc.code || !strings.Contains(h.errb.String(), tc.cause) {
				t.Fatalf("exit %d, stderr %s", code, h.errb)
			}
		})
	}
}

func TestTaskAttachOtherProject(t *testing.T) {
	h := newTaskAttachHarness(t)
	h.env.Getenv = func(string) string { return "" }
	personal := taskFixture(t, h)
	h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	other, err := taskWriter(h.env).New(task.NewOptions{Cwd: h.home, Project: "example", Words: "Other project task"})
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(personal.Path, "task", "attach", other.ID, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var got task.Attachment
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.ID != other.ID || got.Title != other.Title {
		t.Fatalf("result: %+v %v", got, err)
	}
	records, err := (session.Store{TaskDir: other.Path}).List()
	if err != nil || len(records) != 1 {
		t.Fatalf("records: %+v %v", records, err)
	}
}

func TestTaskAttachTakeoverAdvice(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		t.Run(fmt.Sprint(asJSON), func(t *testing.T) {
			h := newTaskAttachHarness(t)
			h.env.Getenv = func(string) string { return "" }
			task := taskFixture(t, h)
			sessions := session.Store{TaskDir: task.Path}
			rec, err := sessions.Start(session.Record{Key: session.Key{SessionID: "other", MachineID: "other-machine"}, PID: 43, PIDStart: "start-43", Lease: "J-1", Holder: "member@other-machine", Fence: 1, Attempt: 1})
			if err != nil {
				t.Fatal(err)
			}
			rec, err = sessions.Reconcile(rec.Key, session.Observation{Process: session.Alive, PID: rec.PID, PIDStart: rec.PIDStart})
			if err != nil {
				t.Fatal(err)
			}
			before, err := sessions.ReadHeartbeat(rec)
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"task", "attach", task.ID}
			if asJSON {
				args = append(args, "--json")
			}
			if code := h.run(h.home, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			advice := h.out.String()
			if asJSON {
				var got struct {
					Holders []string `json:"holders"`
				}
				if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || len(got.Holders) != 1 || got.Holders[0] != rec.Holder {
					t.Fatalf("holders in JSON: %s %v", h.out, err)
				}
				advice = h.errb.String()
			}
			if !strings.Contains(advice, "grid lease take-over") || !strings.Contains(advice, rec.Holder) {
				t.Fatalf("missing advice: %s", advice)
			}
			after, err := sessions.ReadHeartbeat(rec)
			if err != nil || after != before {
				t.Fatalf("heartbeat changed: %+v %v", after, err)
			}
		})
	}
}

func TestTaskAttachAgentCannotSwitchTasks(t *testing.T) {
	for _, marker := range []string{"GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER", "GRID_ROLE"} {
		t.Run(marker, func(t *testing.T) {
			h := newTaskAttachHarness(t)
			first := taskFixture(t, h)
			second, err := taskWriter(h.env).New(task.NewOptions{Cwd: h.home, Project: "personal", Words: "Other task"})
			if err != nil {
				t.Fatal(err)
			}
			h.env.Getenv = func(key string) string {
				if key == marker {
					return "grid-builder"
				}
				if key == "GRID_TASK" {
					return first.Path
				}
				return ""
			}
			if code := h.run(second.Path, "task", "attach", second.ID); code != 1 || !strings.Contains(h.errb.String(), "another task's folder") {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if _, err := os.Stat(filepath.Join(second.Path, "sessions")); !os.IsNotExist(err) {
				t.Fatal("agent wrote other task")
			}
		})
	}
}

func newTaskAttachHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.env.Machine = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	return h
}
