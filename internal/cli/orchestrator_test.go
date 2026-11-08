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

func TestBlockedPublicationNoticeUsesCLIStderrOnce(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	text, err := os.ReadFile("../../testdata/job/task/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(path, "plan.md"), strings.ReplaceAll(string(text), "J-1", "J-2"))
	// Migrated history is not a revision-bound approval receipt.
	h.write(filepath.Join(path, "history.md"), "- plan accepted\n")
	for tick := range 3 {
		if code := h.run(path, "feed", "tick", "--json"); code != 0 {
			t.Fatalf("tick %d: %d %s", tick+1, code, h.errb)
		}
		want := ""
		if tick == 0 {
			want = "plan publication skipped: workflow requires approval of the current plan; run grid approve before Jobs can post\n"
		}
		if got := h.errb.String(); got != want {
			t.Errorf("tick %d stderr = %q, want %q", tick+1, got, want)
		}
		if !json.Valid(h.out.Bytes()) {
			t.Fatalf("notice corrupted JSON stdout: %s", h.out)
		}
	}
	jobs, err := store.Read(feed.Filter{Type: "job"})
	if err != nil || len(jobs.Records) != 1 {
		t.Fatalf("unapproved plan added Jobs: %+v %v", jobs, err)
	}
}

// A Job session never answers: grid job post refuses the answer type for every
// role, GRID_ROLE unset included (the CLI then writes as member, which a person
// gate admits), and the gate stays open.
func TestJobPostRefusesAnswer(t *testing.T) {
	for _, role := range []string{"", "worker", "member", "owner"} {
		t.Run("role="+role, func(t *testing.T) {
			h, path, store := orchestratorHarness(t)
			q := questionAdd(t, h, path, "member")
			if code := h.run(path, "job", "next", "--json"); code != 0 {
				t.Fatalf("next %d: %s", code, h.errb)
			}
			state, err := store.State()
			if err != nil || state.Jobs["J-1"].Assign == nil {
				t.Fatalf("J-1 not assigned: %+v %v", state.Jobs["J-1"], err)
			}
			t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
			t.Setenv("GRID_ROLE", "worker")
			if code := h.run(path, "job", "ack", "J-1", "--attempt", "1", "--json"); code != 0 {
				t.Fatalf("ack %d: %s", code, h.errb)
			}
			t.Setenv("GRID_ROLE", role)
			if code := h.run(path, "job", "post", "J-1", "--attempt", "1", "--type", "answer", "--re", q.Post, "--body", `{"answer":"Yes"}`); code != 1 || !strings.Contains(h.errb.String(), "grid answer") {
				t.Fatalf("job post %d: %s", code, h.errb)
			}
			read, err := store.Read(feed.Filter{Type: "answer"})
			if err != nil || len(read.Records) != 0 {
				t.Fatalf("refused answer posted: %+v %v", read, err)
			}
			state, err = store.State()
			if err != nil {
				t.Fatal(err)
			}
			if !state.Questions[q.Post].Open || state.Questions[q.Post].Applied != nil || len(state.NeedsYou) != 1 {
				t.Fatalf("job post answer closed the gate: %+v", state)
			}
		})
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
	args := []string{"job", "next", "--json"}
	if os.Getenv("GRID_TEST_NEXT_WAIT") == "1" {
		args = append(args, "--wait")
	}
	os.Exit(Run(args, env))
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
	pid, start, stop := stoppableTaskProcess(t)
	args := append(taskTakeArgs(pid, start), "--json")
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
	stop()
	pid, start = taskLeaseIdentity(t)
	args = append(taskTakeArgs(pid, start), "--json")
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
			pid, start := taskLeaseIdentity(t)
			args := append(taskTakeArgs(pid, start), "--json")
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

func TestJobNextInterruptExits130(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatal(h.errb)
	}
	projection := filepath.Join(path, "feed", "state.json")
	if err := os.Remove(projection); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestJobNextProcess$")
	cmd.Env = append(os.Environ(), "GRID_TEST_NEXT_PROCESS=1", "GRID_TEST_NEXT_WAIT=1", "GRID_TEST_TASK="+path, "GRID_TEST_ROOT="+h.root, "GRID_HOME="+h.gridHome)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(projection); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("child did not enter wait: %s", &output)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	err := cmd.Wait()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 130 {
		t.Fatalf("Ctrl-C exit: %v, output: %s", err, &output)
	}
	if strings.Contains(output.String(), "context canceled") {
		t.Fatal(output.String())
	}
}

func TestTaskAdoptProcess(t *testing.T) {
	if os.Getenv("GRID_TEST_ADOPT_PROCESS") != "1" {
		return
	}
	env := OSEnv()
	env.Cwd = gridhome.Real(os.Getenv("GRID_TEST_TASK"))
	env.Ceiling = gridhome.Real(os.Getenv("GRID_TEST_ROOT"))
	env.TempRoots = nil
	pid, start := taskLeaseIdentity(t)
	os.Exit(Run(append(taskTakeArgs(pid, start), "--adopt", "--expected-epoch", "0.1"), env))
}

func TestTwoProcessesAdoptOneTaskTenure(t *testing.T) {
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	pid, start, stop := stoppableTaskProcess(t)
	if code := h.run(path, taskTakeArgs(pid, start)...); code != 0 {
		t.Fatal(h.errb)
	}
	stop()
	cmds := make([]*exec.Cmd, 2)
	outputs := make([]bytes.Buffer, 2)
	for n := range cmds {
		cmd := exec.Command(os.Args[0], "-test.run=^TestTaskAdoptProcess$")
		cmd.Env = append(os.Environ(), "GRID_TEST_ADOPT_PROCESS=1", "GRID_TEST_TASK="+path, "GRID_TEST_ROOT="+h.root, "GRID_HOME="+h.gridHome)
		cmd.Stdout, cmd.Stderr = &outputs[n], &outputs[n]
		cmds[n] = cmd
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	wins := 0
	for n, cmd := range cmds {
		if err := cmd.Wait(); err == nil {
			wins++
		} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("unexpected adoption failure %v: %s", err, &outputs[n])
		}
	}
	if wins != 1 {
		t.Fatalf("%d contenders granted: %s %s", wins, &outputs[0], &outputs[1])
	}
	store := feed.Store{TaskDir: path, TaskID: feedTaskID}
	grant, err := (lease.TaskStore{Feed: store}).Read("task")
	if err != nil || grant.Fence != 2 {
		t.Fatal(grant, err)
	}
	read, err := store.Read(feed.Filter{Type: "handover"})
	if err != nil || len(read.Records) != 2 {
		t.Fatal(read, err)
	}
}

func TestWorkerCanBootstrapAndAdoptTaskCLI(t *testing.T) {
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	t.Setenv("GRID_MEMBER", "worker@team")
	t.Setenv("GRID_ROLE", "worker")
	pid, start, stop := stoppableTaskProcess(t)
	args := taskTakeArgs(pid, start)
	if code := h.run(path, args...); code != 0 {
		t.Fatalf("worker bootstrap: %d %s", code, h.errb)
	}
	stop()
	pid, start = taskLeaseIdentity(t)
	args = taskTakeArgs(pid, start)
	if code := h.run(path, append(args, "--adopt", "--expected-epoch", "0.1")...); code != 0 {
		t.Fatalf("worker adoption: %d %s", code, h.errb)
	}
	t.Setenv("GRID_LEASE_EPOCH", "0.2")
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatalf("worker pass: %d %s", code, h.errb)
	}
}

func TestTaskBootstrapRejectsUnsupportedRoleBeforeGrant(t *testing.T) {
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	t.Setenv("GRID_ROLE", "guest")
	pid, start := taskLeaseIdentity(t)
	if code := h.run(path, taskTakeArgs(pid, start)...); code == 0 {
		t.Fatal("guest acquired task")
	}
	if _, err := os.Stat(filepath.Join(path, "feed", "lease", "task.json")); !os.IsNotExist(err) {
		t.Fatal("failed handover left a granted lease", err)
	}
}

// The old fixtures used a fabricated start time; L43F requires a live grantee.
func stoppableTaskProcess(t *testing.T) (int, string, func()) {
	t.Helper()
	taskLeaseIdentity(t)
	cmd := exec.Command("sh", "-c", "read line")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	stop := func() {
		if !stopped {
			stopped = true
			_ = in.Close()
			_ = cmd.Wait()
		}
	}
	t.Cleanup(stop)
	return cmd.Process.Pid, contract.FormatTime(mustTaskStart(t, cmd.Process.Pid)), stop
}
