package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

func livenessProject(h *harness) string {
	project := filepath.Join(h.gridHome, "projects", "example")
	h.write(filepath.Join(project, "project.md"), "---\nid: EXAMPLE\n---\n")
	task := filepath.Join(project, "tasks", "T-01-example")
	h.mkdir(task)
	return task
}

func TestLeaseCommands(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	if code := h.run(task, "lease", "status", "--json"); code != 0 {
		t.Fatalf("status: %d %s", code, h.errb)
	}
	var status lease.Status
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || status.Held {
		t.Fatalf("status: %+v %v", status, err)
	}
	project := filepath.Dir(filepath.Dir(task))
	if _, err := os.Stat(filepath.Join(project, "lease")); !os.IsNotExist(err) {
		t.Fatal("status wrote files")
	}
	if code := h.run(task, "lease", "take", "merge", "--holder", "worker@01K9J000000000000000000001"); code != 0 || h.out.String() != "merge lease taken, fence 1\n" {
		t.Fatalf("take: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.tmp, "lease", "take", "merge", "--project", "example", "--holder", "worker@01K9J000000000000000000002"); code != 1 || !strings.HasPrefix(h.errb.String(), "merge lease held by worker@01K9J000000000000000000001 since ") {
		t.Fatalf("refusal: %d %s", code, h.errb)
	}
	if code := h.run(task, "lease", "renew", "merge", "--holder", "worker@01K9J000000000000000000001", "--fence", "1", "--json"); code != 0 {
		t.Fatalf("renew: %d %s", code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || !status.Held || status.Fence != 1 {
		t.Fatalf("renew: %+v %v", status, err)
	}
	if code := h.run(task, "lease", "release", "--holder", "worker@01K9J000000000000000000002", "--fence", "1"); code != 1 {
		t.Fatalf("wrong owner: %d", code)
	}
	if code := h.run(task, "lease", "release", "merge", "--holder", "worker@01K9J000000000000000000001", "--fence", "1", "--json"); code != 0 {
		t.Fatalf("release: %d %s", code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &status); err != nil || status.Held || status.Holder != nil {
		t.Fatalf("release: %+v %v", status, err)
	}
	if code := h.run(task, "lease", "take", "--holder", "worker@01K9J000000000000000000002"); code != 0 || h.out.String() != "merge lease taken, fence 2\n" {
		t.Fatalf("retake: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(task, "lease", "release", "--holder", "worker@01K9J000000000000000000001", "--fence", "1"); code != 1 {
		t.Fatalf("stale owner: %d", code)
	}
}

func TestSessionCommands(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	if code := h.run(task, "session", "list", "--json"); code != 0 || h.out.String() != "[]\n" {
		t.Fatalf("empty list: %d %s %s", code, h.out, h.errb)
	}
	if _, err := os.Stat(filepath.Join(task, "sessions")); !os.IsNotExist(err) {
		t.Fatal("list wrote files")
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := session.Store{TaskDir: task, Now: func() time.Time { return now }}
	record := session.Record{Key: session.Key{SessionID: "session-1", MachineID: "machine-1"}, PID: 42, PIDStart: "start-1"}
	if _, err := store.Start(record); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "session", "show", "session-1"); code != 0 || !strings.Contains(h.out.String(), "state: Liveness unknown\n") {
		t.Fatalf("show: %d %s %s", code, h.out, h.errb)
	}
	if _, err := store.Sleep(record.Key); err != nil {
		t.Fatal(err)
	}
	if code := h.run(h.tmp, "session", "list", "--project", "example", "--json"); code != 0 {
		t.Fatalf("list project: %d %s", code, h.errb)
	}
	var views []sessionView
	if err := json.Unmarshal(h.out.Bytes(), &views); err != nil || len(views) != 1 || views[0].State != session.Sleeping || views[0].Task != "T-01-example" {
		t.Fatalf("list: %+v %v", views, err)
	}
	record.MachineID = "machine-2"
	if _, err := store.Start(record); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "session", "show", "session-1"); code != 1 || !strings.Contains(h.errb.String(), "ambiguous") {
		t.Fatalf("ambiguous: %d %s", code, h.errb)
	}
	if code := h.run(h.tmp, "session", "show", "session-1", "--project", "example", "--task", "T-01", "--machine", "machine-1", "--json"); code != 0 {
		t.Fatalf("show exact: %d %s", code, h.errb)
	}
	var view sessionView
	if err := json.Unmarshal(h.out.Bytes(), &view); err != nil || view.State != session.Sleeping || view.MachineID != "machine-1" {
		t.Fatalf("show exact: %+v %v", view, err)
	}
	if code := h.run(task, "session", "show", "missing"); code != 1 {
		t.Fatalf("missing: %d", code)
	}
}

func TestSessionCommandsReachAPersonalTask(t *testing.T) {
	h := newHarness(t)
	task := filepath.Join(h.gridHome, "personal", "tasks", "T-01-note")
	h.mkdir(task)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := session.Store{TaskDir: task, Now: func() time.Time { return now }}
	record := session.Record{Key: session.Key{SessionID: "session-1", MachineID: "machine-1"}, PID: 42, PIDStart: "start-1"}
	if _, err := store.Start(record); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		cwd  string
		args []string
	}{
		{"inside the task", filepath.Join(task, "artifacts"), []string{"session", "list", "--json"}},
		{"project personal", h.tmp, []string{"session", "list", "--project", "personal", "--json"}},
		{"project Personal from the Grid home", h.gridHome, []string{"session", "list", "--project", "Personal", "--json"}},
	} {
		h.out.Reset()
		if code := h.run(tc.cwd, tc.args...); code != 0 {
			t.Fatalf("%s: %d %s", tc.name, code, h.errb)
		}
		var views []sessionView
		if err := json.Unmarshal(h.out.Bytes(), &views); err != nil || len(views) != 1 || views[0].SessionID != "session-1" || views[0].Task != "T-01-note" {
			t.Fatalf("%s: %+v %v %s", tc.name, views, err, h.out)
		}
	}
	h.out.Reset()
	if code := h.run(h.tmp, "session", "show", "session-1", "--project", "personal", "--task", "T-01", "--json"); code != 0 {
		t.Fatalf("show by id: %d %s", code, h.errb)
	}
	var view sessionView
	if err := json.Unmarshal(h.out.Bytes(), &view); err != nil || view.SessionID != "session-1" {
		t.Fatalf("show by id: %+v %v", view, err)
	}
}

func TestLivenessCommandUsage(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"lease"}, {"lease", "take"}, {"lease", "renew", "--holder", "session-1"},
		{"lease", "release", "--holder", "session-1", "--fence", "bad"},
		{"lease", "take-over"}, {"lease", "take", "task", "--holder", "session-1"},
		{"session"}, {"session", "show"}, {"session", "list", "extra"},
	} {
		if code := h.run(h.tmp, args...); code != 2 {
			t.Errorf("%v: exit %d: %s", args, code, h.errb)
		}
	}
	for _, args := range [][]string{{"session", "list", "--help"}, {"lease", "take", "--help"}} {
		if code := h.run(h.tmp, args...); code != 0 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	h.env.GridHome = ""
	for _, args := range [][]string{{"session", "list"}, {"lease", "status"}} {
		if code := h.run(h.tmp, args...); code != 1 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
}

func TestLeaseCommandWritesContract(t *testing.T) {
	h := newHarness(t)
	task := livenessProject(h)
	if code := h.run(task, "lease", "take", "--holder", "worker@01K9J000000000000000000002"); code != 0 {
		t.Fatalf("take: %d %s", code, h.errb)
	}
	data, err := os.ReadFile(filepath.Join(h.gridHome, "projects", "example", "lease", "merge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("merge-lease", data); err != nil {
		t.Fatal(err)
	}
	var r contract.MergeLease
	if err := json.Unmarshal(data, &r); err != nil || r.Project != "example" {
		t.Fatalf("project: %+v %v", r, err)
	}
}

func TestLeaseRejectsInvalidHolderBeforeWriting(t *testing.T) {
	for _, holder := range []string{"foo", "worker@bad", "worker/name@01K9J000000000000000000002", "worker name@01K9J000000000000000000002", "worker@81K9J000000000000000000002"} {
		t.Run(holder, func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			for _, action := range []string{"take", "renew", "release"} {
				if code := h.run(task, "lease", action, "--holder", holder, "--fence", "1"); code != 2 || !strings.Contains(h.errb.String(), "--holder must be <member>@<machine ULID>") {
					t.Fatalf("%s: exit %d: %s", action, code, h.errb)
				}
			}
			if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "example", "lease")); !os.IsNotExist(err) {
				t.Fatalf("invalid holder created lease files: %v", err)
			}
		})
	}
}

func TestLeaseCommandPreservesStoredGeneration(t *testing.T) {
	for _, held := range []bool{false, true} {
		name := "released"
		if held {
			name = "expired"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			task := livenessProject(h)
			holder := "worker@01K9J000000000000000000002"
			r := contract.MergeLease{Lease: "merge", Project: "example", Gen: 7, Fence: 4, Since: "2000-01-01T00:00:00.000Z", Until: "2000-01-01T00:15:00.000Z"}
			if held {
				r.Holder = &holder
			}
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := contract.Validate("lease", data); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(h.gridHome, "projects", "example", "lease", "merge.json")
			h.write(path, string(data))
			for _, action := range []string{"take", "renew", "release"} {
				if code := h.run(task, "lease", action, "--holder", holder, "--fence", "5"); code != 0 {
					t.Fatalf("%s: exit %d: %s", action, code, h.errb)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := contract.Validate("lease", data); err != nil {
					t.Fatalf("%s: %v", action, err)
				}
				if err := json.Unmarshal(data, &r); err != nil || r.Gen != 7 || r.Fence != 5 {
					t.Fatalf("%s lost generation or fence: %+v %v", action, r, err)
				}
			}
		})
	}
}

func taskLeaseIdentity(t *testing.T) (int, string) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Unix process identity")
	}
	start, err := session.ProcessStart(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return os.Getpid(), contract.FormatTime(start)
}

func taskTakeArgs(pid int, start string) []string {
	return []string{"lease", "take", "task", "--pid", strconv.Itoa(pid), "--pid-start", start}
}

func TestTaskLeaseTakeStatusRelease(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h, path := feedHarness(t)
	store := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: feedTaskID}}
	if code := h.run(path, "lease", "status", "task"); code != 0 || h.out.String() != "task lease free\n" {
		t.Fatalf("free: %d %s %s", code, h.out, h.errb)
	}
	args := taskTakeArgs(pid, start)
	if code := h.run(path, args...); code != 0 || h.out.String() != "task lease taken, fence 1\nepoch 0.1\n" {
		t.Fatalf("take: %d %s %s", code, h.out, h.errb)
	}
	raw, err := os.ReadFile(filepath.Join(store.Feed.Dir(), "lease", "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("lease", raw); err != nil {
		t.Fatal(err)
	}
	records, err := store.Feed.Read(feed.Filter{})
	if err != nil || len(records.Records) != 1 {
		t.Fatalf("records: %+v %v", records, err)
	}
	r := records.Records[0]
	wire, _ := json.Marshal(r.Record)
	if r.Type != "handover" || r.Fence != 1 || r.Seq != 1 || r.Verdict != "counted" {
		t.Fatalf("handover: %+v", r)
	}
	if err := contract.Validate("handover", wire); err != nil {
		t.Fatal(err)
	}
	if code := h.run(path, args...); code != 1 || !strings.Contains(h.errb.String(), "task lease held by member@"+feedMachine+" since ") {
		t.Fatalf("held: %d %s", code, h.errb)
	}
	unchanged, err := os.ReadFile(filepath.Join(store.Feed.Dir(), "lease", "task.json"))
	if err != nil || !bytes.Equal(raw, unchanged) {
		t.Fatal("held lease changed")
	}
	if code := h.run(path, "lease", "status", "task", "--json"); code != 0 || !strings.Contains(h.out.String(), `"process":"alive"`) {
		t.Fatalf("status: %d %s %s", code, h.out, h.errb)
	}
	for _, tc := range []struct {
		name, member, epoch string
		code                int
	}{
		{"stale fence", "member", "0.2", 1}, {"stale gen", "member", "1.1", 1},
		{"other holder", "other", "0.1", 1}, {"correct", "member", "0.1", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GRID_MEMBER", tc.member)
			code := h.run(path, "lease", "release", "task", "--epoch", tc.epoch, "--json")
			if code != tc.code {
				t.Fatalf("release: %d %s", code, h.errb)
			}
			if code == 1 && !strings.Contains(h.errb.String(), "member@"+feedMachine) {
				t.Fatalf("missing current holder: %s", h.errb)
			}
		})
	}
	tenure, err := store.Read("task")
	if err != nil || tenure.Holder != nil {
		t.Fatalf("release: %+v %v", tenure, err)
	}
	if code := h.run(path, append(args, "--json")...); code != 0 || !strings.Contains(h.out.String(), `"fence":2`) {
		t.Fatalf("retake: %d %s %s", code, h.out, h.errb)
	}
}

func deadTaskProcess(t *testing.T) (int, string) {
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
	t.Cleanup(func() { _ = in.Close(); _ = cmd.Process.Kill() })
	start, err := session.ProcessStart(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := in.Close(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	return cmd.Process.Pid, contract.FormatTime(start)
}

// liveTaskProcess starts a child that runs until the test ends.
func liveTaskProcess(t *testing.T) (int, string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", "read line")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close(); _ = cmd.Wait() })
	return cmd.Process.Pid, contract.FormatTime(mustTaskStart(t, cmd.Process.Pid))
}

// unknownProbe puts a ps first on PATH whose answer for pid does not parse, so
// that pid probes unknown while every other process is still probed by the real ps.
func unknownProbe(t *testing.T, pid int) {
	t.Helper()
	ps, err := exec.LookPath("ps")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nfor a; do [ \"$a\" = %d ] && { echo unparseable; exit 0; }; done\nexec %s \"$@\"\n", pid, ps)
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestTaskLeaseAdoptionAndStatus(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	deadPID, deadStart := deadTaskProcess(t)
	livePID, liveStart := liveTaskProcess(t)
	for _, tc := range []struct {
		name, machine, state string
		pid                  int
		start                string
		unknown              bool
		code                 int
	}{
		{"local dead", feedMachine, "dead", deadPID, deadStart, false, 0},
		{"local alive", feedMachine, "alive", pid, start, false, 1},
		{"remote dead", "01ARZ3NDEKTSV4RRFFQ69G5FAX", "remote", deadPID, deadStart, false, 1},
		{"local unknown", feedMachine, "unknown", livePID, liveStart, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path := feedHarness(t)
			store := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: feedTaskID}}
			old := feed.Writer{Author: feed.Author{ID: "old", Role: "member"}, Worker: "old@" + tc.machine, Lease: "task", Gen: 3}
			first, err := store.TakeTask(old, tc.pid, tc.start)
			if err != nil {
				t.Fatal(err)
			}
			old.Fence = first.Fence
			if _, err := store.Feed.Handover(feed.Input{}, old); err != nil {
				t.Fatal(err)
			}
			if tc.unknown {
				// The holder probes unknown while the taker's own identity still probes alive.
				unknownProbe(t, tc.pid)
			}
			if code := h.run(path, "lease", "status", "task"); code != 0 || !strings.Contains(h.out.String(), "process: "+tc.state+"\n") {
				t.Fatalf("status: %d %s %s", code, h.out, h.errb)
			}
			if code := h.run(path, append(taskTakeArgs(pid, start), "--json")...); code != tc.code {
				t.Fatalf("take: %d %s %s", code, h.out, h.errb)
			}
			if tc.code != 0 {
				if !strings.Contains(h.errb.String(), "held by "+old.Worker) {
					t.Fatal(h.errb)
				}
				current, err := store.Read("task")
				if err != nil || current.Fence != first.Fence || *current.Holder != old.Worker {
					t.Fatalf("refusal changed lease: %+v %v", current, err)
				}
				return
			}
			var grant struct {
				Gen, Fence int64
				Adopted    string
			}
			if err := json.Unmarshal(h.out.Bytes(), &grant); err != nil || grant.Gen != 3 || grant.Fence != 2 || grant.Adopted != "session-gone" {
				t.Fatalf("grant: %+v %v", grant, err)
			}
			records, err := store.Feed.Read(feed.Filter{Type: "handover"})
			if err != nil || len(records.Records) != 2 {
				t.Fatalf("records: %+v %v", records, err)
			}
			var adopted feed.Record
			for _, r := range records.Records {
				if r.Fence == 2 {
					adopted = r.Record
				}
			}
			if adopted.Body["adopted"] != "session-gone" || adopted.Body["from"] != old.Worker || adopted.Seq != 1 {
				t.Fatalf("adoption: %+v", adopted)
			}
			b, _ := json.Marshal(adopted)
			if err := contract.Validate("handover", b); err != nil {
				t.Fatal(err)
			}
			log, err := os.ReadFile(filepath.Join(store.Feed.Dir(), "lease", "takeovers.jsonl"))
			if err != nil || !bytes.Contains(log, []byte(old.Worker)) {
				t.Fatalf("log: %s %v", log, err)
			}
		})
	}
}

func TestTaskLeaseHandoverRecovery(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	for _, adopt := range []bool{false, true} {
		t.Run(fmt.Sprint("adopt=", adopt), func(t *testing.T) {
			h, path := feedHarness(t)
			store := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: feedTaskID}}
			fence := int64(1)
			if adopt {
				deadPID, deadStart := deadTaskProcess(t)
				w := feed.Writer{Worker: "old@" + feedMachine, Lease: "task"}
				if _, err := store.TakeTask(w, deadPID, deadStart); err != nil {
					t.Fatal(err)
				}
				fence = 2
			}
			// This role cannot post handover, so the grant succeeds and its post fails.
			t.Setenv("GRID_ROLE", "worker")
			args := taskTakeArgs(pid, start)
			if code := h.run(path, args...); code != 1 || !strings.Contains(h.out.String(), fmt.Sprintf("epoch 0.%d", fence)) || h.errb.Len() == 0 {
				t.Fatalf("failure: %d %s %s", code, h.out, h.errb)
			}
			before, err := store.Read("task")
			if err != nil || before.Fence != fence {
				t.Fatalf("lost grant: %+v %v", before, err)
			}
			t.Setenv("GRID_ROLE", "member")
			if code := h.run(path, args...); code != 0 {
				t.Fatalf("recovery: %d %s", code, h.errb)
			}
			after, err := store.Read("task")
			if err != nil || after.Fence != before.Fence || after.Since != before.Since {
				t.Fatalf("recovery changed grant: %+v %v", after, err)
			}
			records, err := store.Feed.Read(feed.Filter{Type: "handover"})
			if err != nil || len(records.Records) != 1 || records.Records[0].Fence != fence {
				t.Fatalf("handover: %+v %v", records, err)
			}
			if adopt && records.Records[0].Body["adopted"] != "session-gone" {
				t.Fatal("recovery lost adoption")
			}
			if code := h.run(path, args...); code != 1 || !strings.Contains(h.errb.String(), "held by") {
				t.Fatalf("duplicate: %d %s", code, h.errb)
			}
		})
	}
}

func TestTaskLeaseRecoveryNeedsTheGrantedProcess(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	otherPID, otherStart := liveTaskProcess(t)
	h, path := feedHarness(t)
	store := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: feedTaskID}}
	// This role cannot post handover, so the grant succeeds and its post fails.
	t.Setenv("GRID_ROLE", "worker")
	if code := h.run(path, taskTakeArgs(pid, start)...); code != 1 || !strings.Contains(h.out.String(), "epoch 0.1") {
		t.Fatalf("failure: %d %s %s", code, h.out, h.errb)
	}
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, taskTakeArgs(otherPID, otherStart)...); code != 1 || !strings.Contains(h.errb.String(), "held by member@"+feedMachine) {
		t.Fatalf("other process: %d %s %s", code, h.out, h.errb)
	}
	current, err := store.Read("task")
	if err != nil || current.Fence != 1 || *current.PID != pid {
		t.Fatalf("other process changed the grant: %+v %v", current, err)
	}
	records, err := store.Feed.Read(feed.Filter{Type: "handover"})
	if err != nil || len(records.Records) != 0 {
		t.Fatalf("other process posted the handover: %+v %v", records, err)
	}
	if code := h.run(path, taskTakeArgs(pid, start)...); code != 0 {
		t.Fatalf("recovery: %d %s", code, h.errb)
	}
}

func TestTaskLeaseSessionAndUsage(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	for _, tc := range []struct{ name, start string }{{"RFC3339", start}, {"lstart", mustTaskStart(t, pid).Format("Mon Jan _2 15:04:05 2006")}} {
		t.Run(tc.name, func(t *testing.T) {
			h, path := feedHarness(t)
			h.write(filepath.Join(h.gridHome, "projects", "example", "project.md"), "---\nid: EXAMPLE\n---\n")
			_, err := (session.Store{TaskDir: path}).Start(session.Record{Key: session.Key{SessionID: "session-1", MachineID: feedMachine}, PID: pid, PIDStart: tc.start})
			if err != nil {
				t.Fatal(err)
			}
			if code := h.run(h.home, "lease", "take", "task", "--task", "T-07", "--project", "example", "--session", "session-1", "--json"); code != 0 {
				t.Fatalf("session: %d %s", code, h.errb)
			}
			got, err := (lease.TaskStore{Feed: feed.Store{TaskDir: path}}).Read("task")
			if err != nil || *got.PIDStart != start || *got.PID != pid {
				t.Fatalf("identity: %+v %v", got, err)
			}
		})
	}
	h, path := feedHarness(t)
	for _, args := range [][]string{
		{"take", "task"}, {"take", "task", "--pid", "1"}, {"take", "task", "--pid", "-1", "--pid-start", start},
		{"take", "task", "--pid", "1", "--pid-start", "proc:1"}, {"take", "task", "--session", "x", "--pid", "1"},
		{"release", "task"}, {"renew", "task"},
	} {
		if code := h.run(path, append([]string{"lease"}, args...)...); code != 2 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
	}
	if code := h.run(path, "lease", "take", "task", "--session", "missing"); code != 1 {
		t.Fatalf("missing session: %d", code)
	}
}

func mustTaskStart(t *testing.T, pid int) time.Time {
	t.Helper()
	start, err := session.ProcessStart(pid)
	if err != nil {
		t.Fatal(err)
	}
	return start
}

func TestTaskLeaseTakeProcess(t *testing.T) {
	if os.Getenv("GRID_TEST_TAKE_PROCESS") != "1" {
		return
	}
	env := OSEnv()
	env.Cwd = gridhome.Real(os.Getenv("GRID_TEST_TASK"))
	env.Ceiling = gridhome.Real(os.Getenv("GRID_TEST_ROOT"))
	env.TempRoots = nil
	pid, _ := strconv.Atoi(os.Getenv("GRID_TEST_PID"))
	if os.Getenv("GRID_TEST_TAKE_CRASH") == "1" {
		afterTaskGrant = func() { os.Exit(3) }
	}
	os.Exit(Run(taskTakeArgs(pid, os.Getenv("GRID_TEST_PID_START")), env))
}

func TestTaskLeaseAdoptionSurvivesACrashBeforeItsHandover(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	deadPID, deadStart := deadTaskProcess(t)
	h, path := feedHarness(t)
	store := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: feedTaskID}}
	old := feed.Writer{Worker: "old@" + feedMachine, Lease: "task"}
	if _, err := store.TakeTask(old, deadPID, deadStart); err != nil {
		t.Fatal(err)
	}
	// The child exits as soon as its adoption is granted, before it posts the handover.
	cmd := exec.Command(os.Args[0], "-test.run=^TestTaskLeaseTakeProcess$")
	cmd.Env = append(os.Environ(), "GRID_TEST_TAKE_PROCESS=1", "GRID_TEST_TAKE_CRASH=1", "GRID_TEST_TASK="+path, "GRID_TEST_ROOT="+h.root, "GRID_HOME="+h.gridHome, "GRID_TEST_PID="+strconv.Itoa(pid), "GRID_TEST_PID_START="+start)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("crash: %v %s", err, out)
	}
	granted, err := store.Read("task")
	if err != nil || granted.Holder == nil || *granted.Holder != "member@"+feedMachine || granted.Fence != 2 {
		t.Fatalf("grant: %+v %v", granted, err)
	}
	if records, err := store.Feed.Read(feed.Filter{Type: "handover"}); err != nil || len(records.Records) != 0 {
		t.Fatalf("crashed take posted: %+v %v", records, err)
	}
	if code := h.run(path, taskTakeArgs(pid, start)...); code != 0 {
		t.Fatalf("retry: %d %s", code, h.errb)
	}
	records, err := store.Feed.Read(feed.Filter{Type: "handover"})
	if err != nil || len(records.Records) != 1 {
		t.Fatalf("retry handover: %+v %v", records, err)
	}
	if r := records.Records[0]; r.Fence != 2 || r.Body["adopted"] != "session-gone" || r.Body["from"] != old.Worker {
		t.Fatalf("retry lost the adoption: %+v", r)
	}
}

func TestTaskLeaseConcurrentCLI(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	for _, processes := range []bool{false, true} {
		t.Run(fmt.Sprint("processes=", processes), func(t *testing.T) {
			h, path := feedHarness(t)
			outputs := [2]bytes.Buffer{}
			codes := [2]int{}
			if processes {
				cmds := [2]*exec.Cmd{}
				for i := range cmds {
					cmds[i] = exec.Command(os.Args[0], "-test.run=^TestTaskLeaseTakeProcess$")
					cmds[i].Env = append(os.Environ(), "GRID_TEST_TAKE_PROCESS=1", "GRID_TEST_TASK="+path, "GRID_TEST_ROOT="+h.root, "GRID_HOME="+h.gridHome, "GRID_TEST_PID="+strconv.Itoa(pid), "GRID_TEST_PID_START="+start)
					cmds[i].Stdout, cmds[i].Stderr = &outputs[i], &outputs[i]
					if err := cmds[i].Start(); err != nil {
						t.Fatal(err)
					}
				}
				for i, cmd := range cmds {
					if err := cmd.Wait(); err != nil {
						var exit *exec.ExitError
						if !errors.As(err, &exit) {
							t.Fatal(err)
						}
						codes[i] = exit.ExitCode()
					}
				}
			} else {
				var wg sync.WaitGroup
				ready := make(chan struct{})
				for i := range codes {
					wg.Add(1)
					go func() {
						defer wg.Done()
						env := h.env
						env.Cwd = path
						env.Stdout, env.Stderr = &outputs[i], &outputs[i]
						<-ready
						codes[i] = Run(taskTakeArgs(pid, start), env)
					}()
				}
				close(ready)
				wg.Wait()
			}
			wins := 0
			for i, code := range codes {
				if code == 0 {
					wins++
				} else if code != 1 || !strings.Contains(outputs[i].String(), "held by") {
					t.Fatalf("loser: %d %s", code, &outputs[i])
				}
			}
			seq, err := os.ReadFile(filepath.Join(path, "feed", "lease", "seq"))
			if wins != 1 || err != nil || string(seq) != "1\n" {
				t.Fatalf("wins=%d seq=%q err=%v outputs=%v", wins, seq, err, outputs)
			}
		})
	}
}

func TestTaskLeaseReachabilityWithFeedTaskFixture(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h, _ := feedHarness(t)
	h.env.Machine = feedMachine
	if code := h.run(h.home, "task", "new", "Build example feature", "--json"); code != 0 {
		t.Fatalf("task new: %d %s", code, h.errb)
	}
	var created struct{ Path string }
	if err := json.Unmarshal(h.out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	// Main's task new still emits only a display id. Supply the stable-id
	// fixture required by all existing Feed commands without changing creation.
	taskPath := filepath.Join(created.Path, "task.md")
	raw, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	h.write(taskPath, strings.Replace(string(raw), "---\n", "---\nulid: "+feedTaskID+"\n", 1))
	t.Log("fixture prerequisite: task new does not yet write the stable Feed ULID")
	draft := filepath.Join(h.home, "draft.md")
	h.write(draft, "# Plan\n\nRecall: nothing found\n\n## Steps\n\n- [ ] 1 Build example feature\n  needs: none\n  kind: build\n  jobs:\n    - id: J-1\n      write: [main.go]\n      after: []\n      review: L1\n      verify: {command: go test ./..., min_tests: 1}\n\n## Verify\n\nExample feature passes its checks.\n")
	if code := h.run(created.Path, "plan", "write", draft); code != 0 {
		t.Fatalf("plan write: %d %s", code, h.errb)
	}
	if code := h.run(created.Path, taskTakeArgs(pid, start)...); code != 0 {
		t.Fatalf("take: %d %s", code, h.errb)
	}
	parts := strings.Split(strings.TrimSpace(h.out.String()), "\n")
	epoch := strings.TrimPrefix(parts[len(parts)-1], "epoch ")
	// Plan write stores a checklist; main's Orchestrator reads Jobs from Feed.
	// With the stable-id fixture, next now reaches the lease guard successfully.
	if code := h.run(created.Path, "job", "next", "--lease-epoch", epoch); code != 0 || h.out.String() != "Nothing ready.\n" {
		t.Fatalf("next before publication: %d %s %s", code, h.out, h.errb)
	}
	// Publish the plan's Job through the existing shell command, not a Go grant.
	body, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(created.Path, "feed", "post", "--type", "job", "--job", "J-1", "--body", string(body), "--lease", "task", "--lease-epoch", epoch); code != 0 {
		t.Fatalf("publish Job: %d %s", code, h.errb)
	}
	if code := h.run(created.Path, "job", "next", "--lease-epoch", epoch, "--json"); code != 0 || !strings.Contains(h.out.String(), `"id":"J-1"`) || !strings.Contains(h.out.String(), `"state":"assigned"`) {
		t.Fatalf("ready Job: %d %s %s", code, h.out, h.errb)
	}
	t.Log("task new -> plan write -> lease take -> job next reaches the Orchestrator; publishing the Job via feed post makes J-1 ready")
}

func TestTaskLeaseTakeRefusesIdentityNotAlive(t *testing.T) {
	pid, _ := taskLeaseIdentity(t)
	deadPID, deadStart := deadTaskProcess(t)
	shifted := contract.FormatTime(mustTaskStart(t, pid).Add(2 * time.Second))
	for _, tc := range []struct {
		name, state string
		args        []string
	}{
		{"wrong start", "dead", taskTakeArgs(pid, shifted)},
		{"reaped child", "dead", taskTakeArgs(deadPID, deadStart)},
		{"session with wrong start", "dead", []string{"lease", "take", "task", "--session", "session-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path := feedHarness(t)
			if _, err := (session.Store{TaskDir: path}).Start(session.Record{Key: session.Key{SessionID: "session-1", MachineID: feedMachine}, PID: pid, PIDStart: shifted}); err != nil {
				t.Fatal(err)
			}
			if code := h.run(path, tc.args...); code != 1 || !strings.Contains(h.errb.String(), "is not a live process on this host ("+tc.state+")") {
				t.Fatalf("take: %d %s %s", code, h.out, h.errb)
			}
			if _, err := os.Stat(filepath.Join(path, "feed", "lease", "task.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refused take wrote a lease: %v", err)
			}
			if records, err := (feed.Store{TaskDir: path, TaskID: feedTaskID}).Read(feed.Filter{}); err != nil || len(records.Records) != 0 {
				t.Fatalf("refused take posted: %+v %v", records, err)
			}
		})
	}
}

func TestTaskLeaseReleaseFree(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h, path := feedHarness(t)
	leaseFile := filepath.Join(path, "feed", "lease", "task.json")
	if code := h.run(path, "lease", "release", "task", "--epoch", "0.1"); code != 1 || h.errb.String() != "grid: no task lease is held\n" {
		t.Fatalf("never taken: %d %s", code, h.errb)
	}
	if _, err := os.Stat(leaseFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("release created the lease: %v", err)
	}
	if code := h.run(path, taskTakeArgs(pid, start)...); code != 0 {
		t.Fatalf("take: %d %s", code, h.errb)
	}
	for i, want := range []int{0, 1} {
		if code := h.run(path, "lease", "release", "task", "--epoch", "0.1"); code != want {
			t.Fatalf("release %d: %d %s", i, code, h.errb)
		}
	}
	if h.errb.String() != "grid: no task lease is held\n" {
		t.Fatalf("released twice: %s", h.errb)
	}
}

func TestTaskLeaseTakeReportsGrantErrors(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h, path := feedHarness(t)
	// A fence counter that cannot be opened fails the grant on a free lease; adoption must not hide it.
	h.mkdir(filepath.Join(path, "feed", "lease", "seq"))
	if code := h.run(path, taskTakeArgs(pid, start)...); code != 1 || strings.Contains(h.errb.String(), "held by") || !strings.Contains(h.errb.String(), "seq") {
		t.Fatalf("grant error: %d %s", code, h.errb)
	}
}
