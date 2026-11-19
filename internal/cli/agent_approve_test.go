package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// agentJobEnv is the environment the runner gives a Job agent it launches
// (runner.agentEnv): role worker, the holder's member and worker id, the task
// epoch of the holder, and its own Job's lease and epoch, which are never the task's.
func agentJobEnv(path, taskEpoch string) func(string) string {
	env := map[string]string{
		"GRID_ROLE": "worker", "GRID_MEMBER": "member", "GRID_WORKER_ID": "member@" + feedMachine,
		"GRID_TASK": path, "GRID_JOB": "J-1", "GRID_ATTEMPT": "1",
		"GRID_LEASE": "J-1", "GRID_LEASE_EPOCH": "3.7", "GRID_FENCE": "7", "GRID_TASK_EPOCH": taskEpoch,
	}
	return func(key string) string { return env[key] }
}

// The owner let agents approve (19 Nov), with no signing: an agent's grid approve
// counts like a person's. A Job agent the runner launched approves in its own
// environment; its answer is recorded as an agent's of its member and names its Job, and the
// holder's next pass makes it the approval, as for a person with no lease.
func TestAgentApproveCountsAfterTheHoldersPass(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	h.env.Getenv = agentJobEnv(path, os.Getenv("GRID_LEASE_EPOCH"))
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("agent approve exit %d: %s", code, h.errb)
	}
	answers := arAnswers(t, store)
	if len(answers) != 1 || answers[0].Author.ID != "member" || answers[0].Author.Role != "agent" || answers[0].Body["reason"] != "approved by agent J-1" {
		t.Fatalf("answers %+v", answers)
	}
	// The holder's pass: the harness holds the task lease through the environment.
	h.env.Getenv = nil
	if code := h.run(path, "job", "next"); code != 0 {
		t.Fatalf("tick: %d %s", code, h.errb)
	}
	accepts := arAccepts(t, store, "J-2")
	if len(accepts) != 1 || accepts[0]["reason"] != "approved by agent J-1" || accepts[0]["plan_digest"] != plan.Digest(text) {
		t.Fatalf("accepting controls %v", accepts)
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.Job != "J-2" || a.PlanDigest != plan.Digest(text) {
		t.Fatalf("approval %+v %v", a, err)
	}
}

// An agent Grid did not launch names itself with GRID_AGENT; it needs no member
// id (the record's author is the account's), and the reason names the agent.
func TestAgentApproveFromAMarkedShell(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	h.env.Getenv = func(key string) string { return map[string]string{"GRID_AGENT": "claude", "USER": "sam"}[key] }
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("agent approve exit %d: %s", code, h.errb)
	}
	answers := arAnswers(t, store)
	if len(answers) != 1 || answers[0].Author.ID != "sam" || answers[0].Author.Role != "agent" || answers[0].Body["reason"] != "approved by agent claude" {
		t.Fatalf("answers %+v", answers)
	}
}

// grid approve --agent is the flag form of GRID_AGENT, for an agent whose shell
// refuses an env prefix.
func TestAgentApproveWithTheAgentFlag(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	h.env.Getenv = func(key string) string { return map[string]string{"USER": "sam"}[key] }
	if code := h.run(path, "approve", path, "--agent", " ", "--plan-hash", plan.Hash(text)); code != 2 {
		t.Fatalf("empty --agent exit %d: %s", code, h.errb)
	}
	if code := h.run(path, "approve", path, "--agent", "claude", "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("agent approve exit %d: %s", code, h.errb)
	}
	answers := arAnswers(t, store)
	if len(answers) != 1 || answers[0].Author.ID != "sam" || answers[0].Author.Role != "agent" || answers[0].Body["reason"] != "approved by agent claude" {
		t.Fatalf("answers %+v", answers)
	}
}

// Guests stay answer-and-comment only (an owner decision about guest people), so
// a guest is refused even with an agent mark in its environment.
func TestAgentApproveLeavesAGuestRefused(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	h.env.Getenv = func(key string) string {
		return map[string]string{"GRID_ROLE": "guest", "GRID_MEMBER": "visitor", "GRID_AGENT": "claude"}[key]
	}
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 3 || !strings.Contains(h.errb.String(), "person of the team") {
		t.Fatalf("guest approve exit %d: %s", code, h.errb)
	}
	if got := arAnswers(t, store); len(got) != 0 {
		t.Fatalf("answers %+v", got)
	}
	if got := arAccepts(t, store, "J-2"); len(got) != 0 {
		t.Fatalf("accepting controls %v", got)
	}
}

// An agent approves the setup commands an agent's record held.
func TestAgentApprovesTheHeldSetupCommands(t *testing.T) {
	h := psNew(t)
	got := h.psRecord(psAnswers)
	h.env.Getenv = psEnv(h, "worker", "sam")
	if code := h.run(h.home, "project", "settings", "approve", "example", "--hash", got.Hash); code != 0 {
		t.Fatalf("agent approve exit %d: %s", code, h.errb)
	}
	if answers := h.psFile("setup-answers.json"); answers["starts"] != "make up" || answers["deployCommand"] != "make deploy" {
		t.Fatalf("setup-answers.json = %v", answers)
	}
	if h.psFile("setup-pending.json") != nil {
		t.Fatal("approve left setup-pending.json")
	}
}

// Delivered work: an agent answers the ship person Job grid deliver posted, and
// the holder's next pass of grid worker next accepts it and lands the task, as
// for a person with no lease (TestShipUnleasedAnswerLandsInWorkerPass).
func TestAgentShipAnswerLandsInWorkerPass(t *testing.T) {
	env, dir, laneSHA, store, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	agent := env
	agent.Getenv = func(key string) string {
		switch key {
		case "GRID_AGENT":
			return "codex"
		case "GRID_MEMBER":
			return "dana"
		case "PATH", "HOME":
			return os.Getenv(key)
		}
		return ""
	}
	if code := Run([]string{"approve", "T-01"}, agent); code != 0 {
		t.Fatalf("agent approve: %d %s", code, stderr)
	}
	answers := arAnswers(t, store)
	if len(answers) != 1 || answers[0].Author.ID != "dana" || answers[0].Author.Role != "agent" || answers[0].Body["reason"] != "approved by agent codex" {
		t.Fatalf("answers %+v", answers)
	}

	worker := env
	worker.Getenv = func(key string) string {
		switch key {
		case "GRID_MEMBER":
			return "dana"
		case "GRID_ROLE":
			return "member"
		case "PATH", "HOME":
			return os.Getenv(key)
		}
		return ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := time.AfterFunc(5*time.Second, cancel)
	defer stop.Stop()
	worker.Context = ctx
	worker.Stdout = &bytes.Buffer{}
	go func() {
		for ctx.Err() == nil {
			if strings.Contains(shipHistory(t, dir), " · land landed · ") {
				cancel()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	pid, start := taskLeaseIdentity(t)
	holder := "dana@" + env.Machine
	tenure, err := json.Marshal(lease.Tenure{
		Lease: feed.Lease{Lease: "task", Holder: &holder, Gen: 0, Fence: 1},
		Since: contract.FormatTime(time.Now()), PID: &pid, PIDStart: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), "lease", "task.json"), append(tenure, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--member", holder, "--role", "member",
		"worker", "next", "--json", "--pid", strconv.Itoa(pid), "--pid-start", start}
	if code := Run(args, worker); code != 130 {
		t.Fatalf("worker next: exit %d, want 130 (stopped after the land); stderr %s", code, stderr)
	}
	if _, snap := approveJobs(t, store); acceptedShipJob(snap) == "" {
		t.Fatalf("no ship Job is accepted after the worker pass; stderr %s", stderr)
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != laneSHA {
		t.Fatalf("main is %s after the worker pass, want the delivered %s; stderr %s", got, laneSHA, stderr)
	}
}
