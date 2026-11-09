package cli

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

	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"go.yaml.in/yaml/v4"
)

const actID = "S-01ARZ3NDEKTSV4RRFFQ69G5FAV"
const actBody = "BODY-MUST-NEVER-ENTER-INTENT\n---\nstatus: new\n"
const actUntil = "2030-04-05T06:07"

func assertSignalErrorJSON(t *testing.T, output, kind, message string) {
	t.Helper()
	want, err := json.Marshal(struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}{kind, message})
	if err != nil {
		t.Fatal(err)
	}
	if output != string(want)+"\n" {
		t.Fatalf("stdout %q, want %s", output, want)
	}
}

func TestSignalActErrorOutput(t *testing.T) {
	for _, tc := range []struct {
		name, status, act, role, kind, message string
		code                                   int
		turnFirst                              bool
	}{
		{name: "guest", status: "new", act: "note", role: "guest", code: 3, kind: "refused", message: "guests cannot act on signals"},
		{name: "state refusal", status: "done", act: "note", code: 3, kind: "refused", message: "this note is already decided (done)"},
		{name: "lock loser", status: "new", act: "turn", turnFirst: true, code: 3, kind: "refused", message: "this note is already decided (done)"},
		{name: "bad input", status: "new", act: "unknown", code: 2, kind: "invalid", message: `unknown signal act "unknown"`},
		{name: "path input", status: "new", code: 2, kind: "invalid", message: "unknown signal act"},
	} {
		for _, asJSON := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/json=%t", tc.name, asJSON), func(t *testing.T) {
				h, path, _ := actFixture(t, tc.status)
				act := tc.act
				if tc.name == "path input" {
					act = path
				}
				if tc.turnFirst && h.run(h.root, actArgs("turn")...) != 0 {
					t.Fatal(h.errb)
				}
				before, _ := actRead(t, path)
				h.env.Getenv = func(key string) string {
					if key == "GRID_ROLE" {
						return tc.role
					}
					return ""
				}
				args := actArgs(act)
				if !asJSON {
					args = args[:len(args)-1]
				}
				stderrMessage := tc.message
				if tc.name == "path input" {
					stderrMessage = fmt.Sprintf("unknown signal act %q", act)
				}
				if code := h.run(h.root, args...); code != tc.code || h.errb.String() != "grid: "+stderrMessage+"\n" {
					t.Fatalf("exit %d, stderr %q; want %d, %q", code, h.errb.String(), tc.code, "grid: "+stderrMessage+"\n")
				}
				if asJSON {
					assertSignalErrorJSON(t, h.out.String(), tc.kind, tc.message)
				} else if h.out.Len() != 0 {
					t.Fatalf("non-JSON stdout %q", h.out.String())
				}
				after, _ := actRead(t, path)
				if !bytes.Equal(before, after) {
					t.Fatal("failed act wrote note")
				}
			})
		}
	}
}

func TestSignalActArgumentErrorJSON(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		args          []string
	}{
		{"unknown flag before JSON", "invalid signal arguments", []string{actID, "--unknown", "--json"}},
		{"unknown flag after JSON", "invalid signal arguments", []string{actID, "--json", "--unknown"}},
		{"missing flag value", "invalid signal arguments", []string{actID, "--json", "--act"}},
		{"missing id", "wrong number of arguments", []string{"--json"}},
		{"extra id", "wrong number of arguments", []string{actID, actID, "--json"}},
		{"JSON disabled", "", []string{actID, "--unknown", "--json", "--json=false"}},
		{"JSON as option value", "", []string{actID, "--unknown", "--act", "--json"}},
		{"JSON after delimiter", "", []string{actID, "--unknown", "--", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path, before := actFixture(t, "new")
			if code := h.run(h.root, append([]string{"signal", "act"}, tc.args...)...); code != 2 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if h.errb.Len() == 0 {
				t.Fatal("missing stderr diagnostic")
			}
			if tc.message != "" {
				assertSignalErrorJSON(t, h.out.String(), "invalid", tc.message)
			} else if h.out.Len() != 0 {
				t.Fatalf("non-JSON stdout %q", h.out.String())
			}
			after, _ := actRead(t, path)
			if !bytes.Equal(before, after) {
				t.Fatal("bad arguments wrote note")
			}
		})
	}
}

func actFixture(t *testing.T, status string) (*harness, string, []byte) {
	t.Helper()
	h := newHarness(t)
	h.record(project.Record{Name: "example", ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"})
	path := filepath.Join(h.gridHome, "projects", "example", "notes", actID+".md")
	data := "---\nid: " + actID + "\nsource: typed\ntitle: A signal title\nstatus: " + status + "\ncaptured: 2020-01-01T00:00:00.000Z\nupdated: 2020-01-01T00:00:00.000Z\ntier: external\nbody_kind: data\nbecame: T-09\nuntil: " + actUntil + "\n---\n" + actBody
	h.write(path, data)
	return h, path, []byte(data)
}
func actRead(t *testing.T, path string) ([]byte, map[string]string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	head, body, ok := bytes.Cut(data[4:], []byte("\n---\n"))
	if !ok || string(body) != actBody {
		t.Fatal("note body changed")
	}
	var n map[string]string
	if err := yaml.Load(head, &n); err != nil {
		t.Fatal(err)
	}
	return data, n
}
func actArgs(act string) []string {
	a := []string{"signal", "act", actID, "--project", "example", "--act", act, "--json"}
	if act == "add" {
		a = append(a, "--task", "T-12")
	}
	if act == "snooze" {
		a = append(a, "--until", actUntil)
	}
	return a
}
func TestSignalActMatrix(t *testing.T) {
	for _, status := range []string{"new", "placed", "snoozed", "kept", "dismissed", "done", "acted"} {
		for _, act := range []string{"turn", "add", "note", "snooze", "dismiss", "wake", "reopen"} {
			t.Run(status+"/"+act, func(t *testing.T) {
				h, path, before := actFixture(t, status)
				effective := status
				if effective == "acted" {
					effective = "done"
				}
				open := effective == "new" || effective == "placed" || effective == "snoozed"
				unchanged := act == "note" && effective == "kept" || act == "dismiss" && effective == "dismissed"
				message := ""
				switch {
				case act == "wake" && effective != "snoozed":
					message = "this note is not snoozed"
				case act == "reopen" && open:
					message = "this note is not decided"
				case act != "wake" && act != "reopen" && !open && !unchanged:
					message = "this note is already decided (" + effective + ")"
				}
				wantCode := 0
				if message != "" {
					wantCode = 3
					message = "grid: " + message + "\n"
				}
				code := h.run(h.root, actArgs(act)...)
				if code != wantCode || h.errb.String() != message {
					t.Fatalf("exit %d, stderr %q; want %d, %q", code, h.errb.String(), wantCode, message)
				}
				after, n := actRead(t, path)
				if message != "" || unchanged {
					if !bytes.Equal(before, after) {
						t.Fatal("refused or unchanged act wrote note")
					}
				}
				if message != "" {
					assertSignalErrorJSON(t, h.out.String(), "refused", strings.TrimSuffix(strings.TrimPrefix(message, "grid: "), "\n"))
					return
				}
				wantStatus, wantBecame, wantUntil := effective, "T-09", ""
				switch act {
				case "turn":
					wantStatus, wantBecame = "done", "T-01"
				case "add":
					wantStatus, wantBecame = "done", "T-12"
				case "note":
					wantStatus = "kept"
				case "dismiss":
					wantStatus = "dismissed"
				case "snooze":
					wantStatus, wantUntil = "snoozed", actUntil
				case "wake":
					wantStatus = "new"
				case "reopen":
					wantStatus, wantBecame = "new", ""
				}
				if unchanged {
					wantUntil = actUntil
				}
				var r map[string]string
				if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
					t.Fatal(err)
				}
				for k, w := range map[string]string{"status": wantStatus, "became": wantBecame, "until": wantUntil} {
					if n[k] != w || r[k] != w {
						t.Errorf("%s: note %q result %q, want %q", k, n[k], r[k], w)
					}
				}
				wantResult := "updated"
				if unchanged {
					wantResult = "unchanged"
				}
				if r["result"] != wantResult {
					t.Errorf("result %q", r["result"])
				}
				if !unchanged && n["updated"] == "2020-01-01T00:00:00.000Z" {
					t.Error("updated timestamp not rewritten")
				}
				tasks, err := filepath.Glob(filepath.Join(h.gridHome, "projects", "example", "tasks", "T-*"))
				if err != nil {
					t.Fatal(err)
				}
				wantTasks := 0
				if act == "turn" {
					wantTasks = 1
				}
				if len(tasks) != wantTasks {
					t.Fatalf("tasks %d, want %d", len(tasks), wantTasks)
				}
				if act == "turn" {
					made, err := task.Read(tasks[0])
					if err != nil {
						t.Fatal(err)
					}
					want := "A signal title\n\nSignal: projects/example/notes/" + actID + ".md"
					if made.Intent != want || made.Project != "example" || made.ID != r["became"] {
						t.Fatalf("unexpected task intent %q, project %q, id %q", made.Intent, made.Project, made.ID)
					}
				}
			})
		}
	}
}

func TestSignalActBadInput(t *testing.T) {
	cases := [][]string{{"--act", ""}, {"--act", "unknown"}, {"--act", "add"}, {"--act", "add", "--task", "12"}, {"--act", "snooze"}, {"--act", "snooze", "--until", "tomorrow"}, {"--act", "note", "--until", actUntil}, {"--act", "dismiss", "--task", "T-12"}, {"--act", "turn", "--later"}, {"--act", "note", "--until="}, {"--act", "note", "--task="}, {"--act", "note", "--later=false"}}
	for _, a := range []string{"turn", "add", "note", "snooze", "dismiss", "wake", "reopen"} {
		cases = append(cases, []string{"--act", a, "--later"})
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h, path, before := actFixture(t, "new")
			a := append([]string{"signal", "act", actID, "--project", "example"}, args...)
			if code := h.run(h.root, a...); code != 2 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			after, _ := actRead(t, path)
			if !bytes.Equal(before, after) {
				t.Fatal("bad input wrote note")
			}
		})
	}
	for _, id := range []string{"12", "S-../bad", "../" + actID} {
		t.Run(id, func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			if code := h.run(h.root, "signal", "act", id, "--project", "example"); code != 2 || h.errb.String() != "grid: invalid signal id\n" {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
		})
	}
}
func TestSignalActDefaultAndAlias(t *testing.T) {
	for _, verb := range []string{"act", "dismiss"} {
		t.Run(verb, func(t *testing.T) {
			h, path, _ := actFixture(t, "new")
			a := []string{"signal", verb, actID, "--project", "example", "--json"}
			if code := h.run(h.root, a...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			before, n := actRead(t, path)
			want := "kept"
			if verb == "dismiss" {
				want = "dismissed"
			}
			if n["status"] != want {
				t.Fatalf("status %q, want %q", n["status"], want)
			}
			if code := h.run(h.root, a...); code != 0 || !strings.Contains(h.out.String(), `"result":"unchanged"`) {
				t.Fatalf("repeat exit %d: %s %s", code, h.out, h.errb)
			}
			after, _ := actRead(t, path)
			if !bytes.Equal(before, after) {
				t.Fatal("repeat wrote note")
			}
		})
	}
	h, path, _ := actFixture(t, "new")
	if code := h.run(h.root, "signal", "dismiss", actID, "--project", "example"); code != 0 {
		t.Fatal(h.errb)
	}
	_, alias := actRead(t, path)
	h, path, _ = actFixture(t, "new")
	if code := h.run(h.root, actArgs("dismiss")...); code != 0 {
		t.Fatal(h.errb)
	}
	_, explicit := actRead(t, path)
	delete(alias, "updated")
	delete(explicit, "updated")
	a, _ := json.Marshal(alias)
	b, _ := json.Marshal(explicit)
	if !bytes.Equal(a, b) {
		t.Fatal("dismiss alias differs")
	}
}
func TestSignalActFailures(t *testing.T) {
	for _, kind := range []string{"missing", "invalid-status", "task-failure", "unregistered"} {
		t.Run(kind, func(t *testing.T) {
			h, path, before := actFixture(t, "new")
			want := 1
			switch kind {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				want = 3
			case "invalid-status":
				h.write(path, strings.Replace(string(before), "status: new", "status: unknown", 1))
				before, _ = os.ReadFile(path)
			case "task-failure":
				h.write(filepath.Join(h.gridHome, "projects", "example", "tasks"), "not a directory")
			case "unregistered":
				if err := os.Remove(filepath.Join(h.gridHome, "projects", "example", "project.md")); err != nil {
					t.Fatal(err)
				}
			}
			code := h.run(h.root, actArgs("turn")...)
			if code != want {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if kind == "missing" {
				if h.errb.String() != "grid: no such note\n" {
					t.Fatal(h.errb)
				}
				return
			}
			after, _ := actRead(t, path)
			if !bytes.Equal(before, after) {
				t.Fatal("failed act wrote note")
			}
		})
	}
}
func TestSignalActGuestBeforeRead(t *testing.T) {
	for _, act := range []string{"turn", "add", "note", "snooze", "dismiss", "wake", "reopen", "alias"} {
		t.Run(act, func(t *testing.T) {
			h, path, _ := actFixture(t, "new")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			h.mkdir(path)
			h.env.Getenv = func(k string) string {
				if k == "GRID_ROLE" {
					return "guest"
				}
				return ""
			}
			a := actArgs(act)
			if act == "alias" {
				a = []string{"signal", "dismiss", actID, "--project", "example"}
			}
			if code := h.run(h.root, a...); code != 3 || h.errb.String() != "grid: guests cannot act on signals\n" {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
		})
	}
}
func TestSignalActUntil(t *testing.T) {
	for _, input := range []string{actUntil, "2030-04-05T06:07:59+02:00"} {
		t.Run(input, func(t *testing.T) {
			h, path, _ := actFixture(t, "new")
			want := actUntil
			if parsed, err := time.Parse(time.RFC3339, input); err == nil {
				want = parsed.In(time.Local).Format("2006-01-02T15:04")
			}
			if code := h.run(h.root, "signal", "act", actID, "--project", "example", "--act", "snooze", "--until", input); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			_, n := actRead(t, path)
			if n["until"] != want {
				t.Fatalf("until %q, want %q", n["until"], want)
			}
		})
	}
}
func TestSignalTurnProcess(t *testing.T) {
	if os.Getenv("GRID_SIGNAL_TEST_PROCESS") != "1" {
		return
	}
	var paths []string
	if err := json.Unmarshal([]byte(os.Getenv("GRID_SIGNAL_TEST_ENV")), &paths); err != nil {
		panic(err)
	}
	env := Env{Home: paths[0], GridHome: paths[1], Cwd: paths[2], Ceiling: paths[2], Machine: paths[3], TempRoots: []string{paths[4]}}
	env.Stdout, env.Stderr = os.Stdout, os.Stderr
	if err := os.WriteFile(os.Getenv("GRID_SIGNAL_TEST_READY"), nil, 0o600); err != nil {
		panic(err)
	}
	os.Exit(Run(actArgs("turn"), env))
}
func TestSignalConcurrentTurnProcesses(t *testing.T) {
	h, path, _ := actFixture(t, "new")
	envJSON, err := json.Marshal([]string{h.home, h.gridHome, h.root, h.env.Machine, h.tmp})
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := localdata.Lock(h.gridHome, ".signals.lock")
	if err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()
	var cmds [2]*exec.Cmd
	var outs, errs [2]bytes.Buffer
	for i := range cmds {
		cmds[i] = exec.Command(exe, "-test.run=^TestSignalTurnProcess$")
		cmds[i].Env = append(os.Environ(), "GRID_SIGNAL_TEST_PROCESS=1", "GRID_SIGNAL_TEST_ENV="+string(envJSON), "GRID_SIGNAL_TEST_READY="+filepath.Join(h.root, fmt.Sprintf("ready-%d", i)))
		cmds[i].Stdout = &outs[i]
		cmds[i].Stderr = &errs[i]
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmds[i].Process.Kill(); _ = cmds[i].Wait() })
	}
	// Both processes have reached Run, but neither may read state or create a
	// task until the parent releases the same persistent signal lock.
	for i := range cmds {
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(h.root, fmt.Sprintf("ready-%d", i))); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("child did not start")
			}
			time.Sleep(time.Millisecond * 10)
		}
	}
	time.Sleep(100 * time.Millisecond)
	before, n := actRead(t, path)
	if n["status"] != "new" {
		t.Fatal("act bypassed the signal lock")
	}
	tasks, _ := filepath.Glob(filepath.Join(h.gridHome, "projects", "example", "tasks", "T-*"))
	if len(tasks) != 0 {
		t.Fatal("task created outside the signal lock")
	}
	unlock()
	locked = false
	wins := 0
	for i, c := range cmds {
		err := c.Wait()
		if err == nil {
			wins++
			continue
		}
		if c.ProcessState.ExitCode() != 3 || errs[i].String() != "grid: this note is already decided (done)\n" {
			t.Fatalf("loser: %v, %s", err, &errs[i])
		}
		assertSignalErrorJSON(t, outs[i].String(), "refused", "this note is already decided (done)")
	}
	if wins != 1 {
		t.Fatalf("winners %d", wins)
	}
	paths, err := filepath.Glob(filepath.Join(h.gridHome, "projects", "example", "tasks", "T-*"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("tasks %d: %v", len(paths), err)
	}
	after, n := actRead(t, path)
	if bytes.Equal(before, after) {
		t.Fatal("turn did not write note")
	}
	if n["status"] != "done" || n["became"] != "T-01" {
		t.Fatalf("note %+v", n)
	}
}

func TestSignalActRolesAndTaskAuthor(t *testing.T) {
	for _, role := range []string{"", "owner", "member", "agent", "cli", "guest", "external", "unknown"} {
		t.Run(role, func(t *testing.T) {
			h, path, before := actFixture(t, "new")
			h.env.Getenv = func(key string) string {
				switch key {
				case "GRID_ROLE":
					return role
				case "GRID_ACTOR":
					return "signal-author"
				}
				return ""
			}
			refused := role == "guest" || role == "external" || role == "unknown"
			code := h.run(h.root, actArgs("turn")...)
			if refused {
				if code != 3 || h.errb.String() != "grid: guests cannot act on signals\n" {
					t.Fatalf("exit %d: %s", code, h.errb)
				}
				after, _ := actRead(t, path)
				if !bytes.Equal(before, after) {
					t.Fatal("refused caller wrote note")
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			paths, err := filepath.Glob(filepath.Join(h.gridHome, "projects", "example", "tasks", "T-*"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("tasks %d: %v", len(paths), err)
			}
			made, err := task.Read(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			wantRole := role
			if wantRole == "" {
				wantRole = "member"
			}
			if made.Owner != "signal-author" || made.OwnerRole != wantRole {
				t.Fatalf("task author %q role %q", made.Owner, made.OwnerRole)
			}
		})
	}
}
