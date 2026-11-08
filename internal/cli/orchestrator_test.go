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

	"github.com/gridconsole/grid-plugins/internal/contract"
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
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
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

func TestFeedPostCannotBypassExitedWorker(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatal(h.errb)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
	if code := h.run(path, "job", "ack", "J-1", "--attempt", "1"); code != 0 {
		t.Fatal(h.errb)
	}
	if code := h.run(path, "job", "post", "J-1", "--attempt", "1", "--type", "worker-exit", "--body", `{"reason":"done"}`); code != 0 {
		t.Fatal(h.errb)
	}
	before, _ := store.Read(feed.Filter{})
	body, err := os.ReadFile("../../contract/fixtures/result/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	body = fixture.Body
	if code := h.run(path, "feed", "post", "--type", "result", "--job", "J-1", "--attempt", "1", "--body", string(body)); code == 0 {
		t.Fatal("feed post accepted result after worker-exit")
	}
	after, _ := store.Read(feed.Filter{})
	if len(before.Records) != len(after.Records) {
		t.Fatal("refused post changed feed")
	}
}

func TestTakeTaskCLI(t *testing.T) {
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	args := []string{"lease", "take", "task", "--pid", fmt.Sprint(os.Getpid()), "--pid-start", "2000-01-01T00:00:00.000Z", "--json"}
	if code := h.run(path, args...); code != 0 {
		t.Fatalf("first grant: %d %s", code, h.errb)
	}
	var first lease.Tenure
	if err := json.Unmarshal(h.out.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.Fence != 1 {
		t.Fatal(first)
	}
	if code := h.run(path, args...); code == 0 {
		t.Fatal("silently replaced holder")
	}
	adopt := append(append([]string{}, args...), "--adopt", "--expected-epoch", "0.1")
	if code := h.run(path, adopt...); code != 0 {
		t.Fatalf("adoption: %d %s", code, h.errb)
	}
	var second lease.Tenure
	if err := json.Unmarshal(h.out.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second.Fence <= first.Fence {
		t.Fatal(second)
	}
	if code := h.run(path, adopt...); code == 0 {
		t.Fatal("stale adoption replaced newer tenure")
	}
	store := feed.Store{TaskDir: path, TaskID: feedTaskID}
	records, err := store.Read(feed.Filter{Type: "handover"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records.Records) != 2 || records.Records[1].Body["adopted"] != "session-gone" {
		t.Fatal(records)
	}
	if code := h.run(path, "lease", "take", "task", "--help"); code != 0 || !strings.Contains(h.errb.String(), "adoption") || !strings.Contains(h.errb.String(), "takeover") {
		t.Fatalf("help: %d %s %s", code, h.out, h.errb)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", second.Fence))
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatal(h.errb)
	}
}

func TestTaskTakeoverCLI(t *testing.T) {
	for _, tc := range []struct {
		name, role, since string
		adopt, want       bool
	}{
		{"no evidence", "member", "", false, false},
		{"too recent", "member", contract.FormatTime(time.Now()), false, false},
		{"agent", "worker", contract.FormatTime(time.Now().Add(-16 * time.Minute)), false, false},
		{"person", "member", contract.FormatTime(time.Now().Add(-16 * time.Minute)), false, true},
		{"remote adoption", "member", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path := feedHarness(t)
			h.env.Machine = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
			args := []string{"lease", "take", "task", "--pid", fmt.Sprint(os.Getpid()), "--pid-start", "2000-01-01T00:00:00.000Z", "--json"}
			if code := h.run(path, args...); code != 0 {
				t.Fatalf("first: %d %s", code, h.errb)
			}
			h.env.Machine = feedMachine
			t.Setenv("GRID_ROLE", tc.role)
			mode := "--takeover"
			if tc.adopt {
				mode = "--adopt"
			}
			args = append(args, mode, "--expected-epoch", "0.1")
			if tc.since != "" {
				args = append(args, "--server-offline-since", tc.since)
			}
			if code := h.run(path, args...); (code == 0) != tc.want {
				t.Fatalf("takeover: %d %s", code, h.errb)
			}
		})
	}
}

func TestTaskAdoptionRefusesLiveAndUnknownCLI(t *testing.T) {
	cmd := exec.Command("ps", "-o", "lstart=", "-p", fmt.Sprint(os.Getpid()))
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	stamp, err := time.Parse("Mon Jan 2 15:04:05 2006", strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{contract.FormatTime(stamp), "unknown"} {
		t.Run(start, func(t *testing.T) {
			h, path := feedHarness(t)
			h.env.Machine = feedMachine
			args := []string{"lease", "take", "task", "--pid", fmt.Sprint(os.Getpid()), "--pid-start", contract.FormatTime(stamp)}
			if code := h.run(path, args...); code != 0 {
				t.Fatal(h.errb)
			}
			if start == "unknown" {
				file := filepath.Join(path, "feed", "lease", "task.json")
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var grant map[string]any
				if err := json.Unmarshal(raw, &grant); err != nil {
					t.Fatal(err)
				}
				grant["pid"] = nil
				grant["pid_start"] = nil
				raw, _ = json.Marshal(grant)
				if err := os.WriteFile(file, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if code := h.run(path, append(args, "--adopt", "--expected-epoch", "0.1")...); code == 0 {
				t.Fatal("adopted live or unknown process")
			}
		})
	}
}
