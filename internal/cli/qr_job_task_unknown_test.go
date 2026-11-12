package cli

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// qrRig is a harness with one registered project, "example", and no tasks; nothing
// reads or writes a real home.
type qrRig struct {
	h   *harness
	cwd string
}

func qrNewRig(t *testing.T) qrRig {
	t.Helper()
	h := newHarness(t)
	for key, dir := range map[string]string{"HOME": h.home, "GRID_HOME": h.gridHome,
		"CLAUDE_CONFIG_DIR": filepath.Join(h.home, ".claude"), "CODEX_HOME": filepath.Join(h.home, ".codex"),
		"PATH": h.tmp} {
		t.Setenv(key, dir)
	}
	h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
	h.env.Getenv = func(string) string { return "" }
	cwd := filepath.Join(h.root, "checkout")
	h.mkdir(cwd)
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	return qrRig{h: h, cwd: cwd}
}

// qrRun runs argv with --json and without it and returns both answers.
func (r qrRig) qrRun(argv ...string) (jsonExit int, jsonOut, jsonErr string, plainExit int, plainOut, plainErr string) {
	r.h.env.Stdin = strings.NewReader("")
	jsonExit = r.h.run(r.cwd, append(slices.Clone(argv), "--json")...)
	jsonOut, jsonErr = r.h.out.String(), r.h.errb.String()
	r.h.env.Stdin = strings.NewReader("")
	plainExit = r.h.run(r.cwd, argv...)
	plainOut, plainErr = r.h.out.String(), r.h.errb.String()
	return
}

func qrKeys(t *testing.T, out string) []string {
	t.Helper()
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %q: %v", out, err)
	}
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func qrCode(t *testing.T, out string) string {
	t.Helper()
	code, _, _ := ggProjectCode([]byte(out))
	return code
}

func TestQrJobVerbsUnknownTaskKeepObjectAndWriteStderr(t *testing.T) {
	const want = "task T-99 not found in project example"
	for _, argv := range [][]string{
		{"job", "show", "T-99", "J-1"},
		{"job", "render", "T-99", "I-01"},
		{"job", "verify", "J-1", "--task", "T-99"},
	} {
		t.Run(strings.Join(argv[:2], " "), func(t *testing.T) {
			r := qrNewRig(t)
			exit, out, stderr, plainExit, plainOut, plainErr := r.qrRun(argv...)
			if exit != 1 || plainExit != 1 {
				t.Fatalf("exit with --json %d, without %d, want 1 and 1", exit, plainExit)
			}
			if got, wantObj := strings.TrimSpace(r.h.norm(out)), `{"ok":false,"findings":[],"error":"`+want+`","code":"no_such_task"}`; got != wantObj {
				t.Errorf("stdout = %s\nwant %s", got, wantObj)
			}
			if keys := qrKeys(t, out); !slices.Equal(keys, []string{"code", "error", "findings", "ok"}) {
				t.Errorf("keys = %v, want code, error, findings, ok", keys)
			}
			if stderr != "job: "+want+"\n" || stderr != plainErr {
				t.Errorf("stderr with --json %q, without %q, want %q for both", stderr, plainErr, "job: "+want+"\n")
			}
			if plainOut != "" {
				t.Errorf("plain stdout = %q, want empty", plainOut)
			}
		})
	}
}

func TestQrJobLintUnknownTaskAndProject(t *testing.T) {
	r := qrNewRig(t)
	file := filepath.Join(r.h.root, "input.md")
	r.h.write(file, approvedPlan)
	for _, tc := range []struct {
		name string
		argv []string
		code string
	}{
		{"task", []string{"job", "lint", file, "--task", "T-99"}, "no_such_task"},
		{"project", []string{"job", "lint", file, "--project", "absent"}, "no_such_project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, out, stderr, plainExit, plainOut, plainErr := r.qrRun(tc.argv...)
			if exit != 1 || plainExit != 1 {
				t.Fatalf("exit with --json %d, without %d, want 1 and 1", exit, plainExit)
			}
			if got := qrCode(t, out); got != tc.code {
				t.Errorf("code = %q, want %q; stdout %s", got, tc.code, out)
			}
			if !strings.HasPrefix(stderr, "job: ") || stderr != plainErr {
				t.Errorf("stderr with --json %q, without %q, want the same job: line", stderr, plainErr)
			}
			if plainOut != "" {
				t.Errorf("plain stdout = %q, want empty (the refusal, not the lint report)", plainOut)
			}
		})
	}
}

func TestQrJobTraceUnknownProjectWritesStderr(t *testing.T) {
	r := qrNewRig(t)
	exit, out, stderr, plainExit, plainOut, plainErr := r.qrRun("job", "trace", "parity.md", "--project", "absent")
	if exit != 1 || plainExit != 1 {
		t.Fatalf("exit with --json %d, without %d, want 1 and 1", exit, plainExit)
	}
	if got := qrCode(t, out); got != "no_such_project" {
		t.Errorf("code = %q, want no_such_project; stdout %s", got, out)
	}
	var res struct {
		Rows     []json.RawMessage
		Problems []struct{ Text string }
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res.Problems) != 1 || res.Rows == nil {
		t.Errorf("stdout keeps its rows and problems: %q (%v)", out, err)
	} else if want := "job trace: " + res.Problems[0].Text + "\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stderr == "" || stderr != plainErr || plainOut != "" {
		t.Errorf("stderr with --json %q, without %q; plain stdout %q", stderr, plainErr, plainOut)
	}
}

func TestQrTaskDeclineAndRenameUnknownProject(t *testing.T) {
	for _, argv := range [][]string{
		{"task", "decline", "T-99", "--project", "absent", "--reason", "x"},
		{"task", "rename", "T-99", "--project", "absent", "--title", "x"},
	} {
		t.Run(argv[1], func(t *testing.T) {
			r := qrNewRig(t)
			exit, out, stderr, plainExit, plainOut, plainErr := r.qrRun(argv...)
			if exit != 1 || plainExit != 1 {
				t.Fatalf("exit with --json %d, without %d, want 1 and 1", exit, plainExit)
			}
			if got := qrCode(t, out); got != CodeNoSuchProject {
				t.Errorf("code = %q, want no_such_project; stdout %s", got, out)
			}
			if stderr == "" || stderr != plainErr || plainOut != "" {
				t.Errorf("stderr with --json %q, without %q; plain stdout %q", stderr, plainErr, plainOut)
			}
			if !strings.Contains(stderr, "absent") {
				t.Errorf("stderr %q does not name the project", stderr)
			}
		})
	}
}

// A registered project with an unknown task still answers no_such_task.
func TestQrTaskDeclineKnownProjectUnknownTask(t *testing.T) {
	r := qrNewRig(t)
	exit, out, _, _, _, _ := r.qrRun("task", "decline", "T-99", "--project", "example", "--reason", "x")
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	if got := qrCode(t, out); got != CodeNoSuchTask {
		t.Errorf("code = %q, want no_such_task; stdout %s", got, out)
	}
}
