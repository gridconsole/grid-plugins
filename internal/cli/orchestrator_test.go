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

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

func orchestratorHarness(t *testing.T) (*harness, string, feed.Store) {
	t.Helper()
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	store := feed.Store{TaskDir: path, TaskID: feedTaskID}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", w.Fence))
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: body}, w); err != nil {
		t.Fatal(err)
	}
	return h, path, store
}
func TestOrchestratorCommands(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	if code := h.run(path, "job", "next", "--json"); code != 0 || !strings.Contains(h.out.String(), `"state":"assigned"`) {
		t.Fatalf("%d %s %s", code, h.out, h.errb)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs["J-1"]
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", j.Assign.Fence))
	if code := h.run(path, "job", "ack", "J-1", "--attempt", "1", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if code := h.run(path, "job", "post", "J-1", "--attempt", "1", "--type", "checkpoint", "--body", `{"job":"J-1","step":"tests","commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	for _, verb := range []string{"tree", "blocked"} {
		if code := h.run(path, "job", verb, "--json"); code != 0 {
			t.Fatalf("%d %s", code, h.errb)
		}
	}
	t.Setenv("GRID_ROLE", "member")
	t.Setenv("GRID_LEASE_EPOCH", "0.1")
	if code := h.run(path, "job", "cancel", "J-1", "--reason", "scope withdrawn"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	state, err = store.State()
	if err != nil || state.Jobs["J-1"].Status != "cancelled" {
		t.Fatal(state, err)
	}
	if code := h.run(path, "feed", "tick", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
}
func TestJobNextProcess(t *testing.T) {
	if os.Getenv("GRID_TEST_NEXT_PROCESS") != "1" {
		return
	}
	env := OSEnv()
	env.Cwd = gridhome.Real(os.Getenv("GRID_TEST_TASK"))
	env.Ceiling = gridhome.Real(os.Getenv("GRID_TEST_ROOT"))
	env.TempRoots = nil
	os.Exit(Run([]string{"job", "next", "--json"}, env))
}
func TestTwoProcessesJobNext(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	cmds := make([]*exec.Cmd, 2)
	outputs := make([]*bytes.Buffer, 2)
	for n := range cmds {
		cmds[n] = exec.Command(os.Args[0], "-test.run=^TestJobNextProcess$")
		cmds[n].Env = append(os.Environ(), "GRID_TEST_NEXT_PROCESS=1", "GRID_TEST_TASK="+path, "GRID_TEST_ROOT="+h.root, "GRID_HOME="+h.gridHome)
		outputs[n] = &bytes.Buffer{}
		cmds[n].Stdout = outputs[n]
		cmds[n].Stderr = outputs[n]
		if err := cmds[n].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for n, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("%v: %s", err, outputs[n])
		}
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs["J-1"]
	if j.Attempt != 1 || j.Assign == nil {
		t.Fatalf("%+v", j)
	}
	records, err := store.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	assigns := 0
	for _, r := range records.Records {
		if r.Body["assign"] != nil {
			assigns++
		}
	}
	if assigns != 1 {
		t.Fatalf("%d assigns", assigns)
	}
	if _, err := os.Stat(filepath.Join(path, "spawned", "J-1.json")); err != nil {
		t.Fatal(err)
	}
}
