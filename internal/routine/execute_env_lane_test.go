package routine

import (
	"context"
	"os/exec"
	"reflect"
	"slices"
	"testing"
)

// recordedEnv runs one routine through ProcessExecutor with the given Getenv and
// returns the environment the child would have started with. No process starts:
// runCommand is injected, as in limits_test.go.
func recordedEnv(t *testing.T, get func(string) string) []string {
	t.Helper()
	var env []string
	calls := 0
	p := ProcessExecutor{
		Home:   "/home/person",
		Getenv: get,
		runCommand: func(cmd *exec.Cmd) error {
			calls++
			env = append([]string(nil), cmd.Env...)
			return nil
		},
	}
	if err := p.Execute(context.Background(), Store{Project: "example", GridHome: "/grid/home"}, Entry{Run: "signals"}); err != nil || calls != 1 {
		t.Fatalf("execute: %v, calls %d", err, calls)
	}
	return env
}

func TestRoutineChildEnvKeepsTheWindowsVariables(t *testing.T) {
	// The nine keys the routine child already had, then the five this lane adds.
	keys := []string{"PATH", "USERPROFILE", "SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL",
		"APPDATA", "LOCALAPPDATA", "USERNAME", "PATHEXT", "ComSpec"}
	got := recordedEnv(t, func(key string) string { return "v-" + key })
	want := []string{"HOME=/home/person", "GRID_HOME=/grid/home"}
	for _, key := range keys {
		want = append(want, key+"=v-"+key)
	}
	for _, entry := range want {
		if !slices.Contains(got, entry) {
			t.Errorf("child environment is missing %q", entry)
		}
	}
	if len(got) != len(want) {
		t.Errorf("child environment has %d entries, want %d: %q", len(got), len(want), got)
	}
}

func TestRoutineChildEnvEmptyValuesStayOut(t *testing.T) {
	got := recordedEnv(t, func(key string) string {
		if key == "PATH" {
			return "/usr/bin"
		}
		return ""
	})
	want := []string{"HOME=/home/person", "GRID_HOME=/grid/home", "PATH=/usr/bin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("child environment %q, want %q", got, want)
	}
}
