package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// Round 20, lane C-r20-cancel-request-cli. These tests pin the CLI half of
// docs/decisions/D-r19-person-cancel-holder.md, option C: `grid job cancel` by a person of the
// team writes the cancel under the task lease when the person is the holder, and saves an
// unleased cancel request when anyone else holds it, or nobody does. N1 is in
// r18_person_job_cancel_test.go, where it replaces TestR18PersonOfAnotherMemberStaysStale.

const r20ClSaved = "Saved. The cancel will be carried out on the next pass of the task's orchestrator."

// r20ClControls is every control record of the task, oldest first.
func r20ClControls(t *testing.T, a *agentTask) []feed.Record {
	t.Helper()
	r, err := a.store.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]feed.Record, 0, len(r.Records))
	for _, e := range r.Records {
		out = append(out, e.Record)
	}
	return out
}

// r20ClRecords counts every record of the task: a refusal must leave it as it was.
func r20ClRecords(t *testing.T, a *agentTask) int {
	t.Helper()
	r, err := a.store.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return len(r.Records)
}

// r20ClCancel runs `grid job cancel` for J-1 with a fresh stdout and stderr and returns the exit code.
func r20ClCancel(a *agentTask, extra ...string) int {
	a.h.out.Reset()
	a.h.errb.Reset()
	return r18Cancel(a, extra...)
}

// N10: the person is the holder, so the cancel is written at once under the lease and no request is
// saved. TestR18PersonCancelsRunningJobWithoutAnEpoch pins the leased control; this one pins that
// no unleased control is left behind and that the person is not told to wait.
func TestR20ClN10HolderPersonCancelsAtOnce(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "worker") // the member the lease holder worker@<machine> is named for
	if code := r20ClCancel(a); code != 0 {
		t.Fatalf("the holder's own cancel: exit %d %s", code, a.h.errb)
	}
	if strings.Contains(a.h.errb.String(), "Saved.") {
		t.Fatalf("the holder's cancel was saved as a request: %q", a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %q, want cancelled at once", got)
	}
	for _, c := range r20ClControls(t, a) {
		if c.Lease == nil && c.Body["status"] == "cancelled" {
			t.Fatalf("an unleased cancel request was written: %+v", c)
		}
	}
	if c := a.lastControl(t); c.Lease == nil || *c.Lease != "task" || c.Author.ID != "worker" || c.Author.Role != "owner" {
		t.Fatalf("want a leased cancel by the person: %+v", c)
	}
}

// R1: no --reason is refused with exit 1 before anything is written, for each role of the team.
func TestR20ClR1RequestNeedsAReason(t *testing.T) {
	a, _ := kbRoleAgent(t)
	for _, role := range []string{"owner", "member", "lead"} {
		r18PersonAtTerminal(t, "dana")
		t.Setenv("GRID_ROLE", role)
		before := r20ClRecords(t, a)
		a.h.out.Reset()
		a.h.errb.Reset()
		code := a.h.run(a.path, "job", "cancel", "--task", a.path, "--job", "J-1")
		if code != 1 || !strings.Contains(a.h.errb.String(), "cancel requires --reason") {
			t.Fatalf("%s: exit %d %s, want 1 cancel requires --reason", role, code, a.h.errb)
		}
		if got := r20ClRecords(t, a); got != before {
			t.Fatalf("%s: %d records before and %d after a refused cancel", role, before, got)
		}
		if got := a.status(t, "J-1"); got != contract.Running {
			t.Fatalf("%s: J-1 is %q, want running", role, got)
		}
	}
}

// R2: a guest with no epoch is not a person of the team, so the CLI does not read the lease for it and
// saves no request. The cli.go guest gate (cli.go:241) does not take `job`, and workflowRoleRefusal
// (cli.go:271) does not refuse `job cancel` for a guest, so the refusal a guest sees comes from
// orchestratorWriter: exit 1, "GRID_LEASE_EPOCH or --lease-epoch must be gen.fence". Nothing is written.
func TestR20ClR2GuestIsRefusedWithoutAnEpoch(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "visitor")
	t.Setenv("GRID_ROLE", "guest")
	before := r20ClRecords(t, a)
	code := r20ClCancel(a)
	if code != 1 || !strings.Contains(a.h.errb.String(), "GRID_LEASE_EPOCH or --lease-epoch must be gen.fence") {
		t.Fatalf("a guest with no epoch: exit %d %s, want 1 and the gen.fence refusal", code, a.h.errb)
	}
	if strings.Contains(a.h.errb.String(), "Saved.") {
		t.Fatalf("a guest's cancel was saved as a request: %q", a.h.errb)
	}
	if got := r20ClRecords(t, a); got != before {
		t.Fatalf("%d records before and %d after a guest's refused cancel", before, got)
	}
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q, want running", got)
	}
}

// R7a: a person who names a Job lease and no epoch gets no request and no lease read: the Job lease is
// not the task lease, so orchestratorWriter refuses the empty epoch, exit 1.
func TestR20ClR7aPersonJobLeaseNoEpoch(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	before := r20ClRecords(t, a)
	code := r20ClCancel(a, "--lease", "J-1")
	if code != 1 || !strings.Contains(a.h.errb.String(), "gen.fence") {
		t.Fatalf("a person with --lease J-1 and no epoch: exit %d %s, want 1 gen.fence", code, a.h.errb)
	}
	if got := r20ClRecords(t, a); got != before {
		t.Fatalf("%d records before and %d after the refused cancel", before, got)
	}
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q, want running", got)
	}
}

// R8: the target Job is not in the task: exit 1 and nothing written.
func TestR20ClR8UnknownJob(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	before := r20ClRecords(t, a)
	a.h.out.Reset()
	a.h.errb.Reset()
	code := a.h.run(a.path, "job", "cancel", "--task", a.path, "--job", "J-99", "--reason", "scope withdrawn")
	if code != 1 || !strings.Contains(a.h.errb.String(), "grid job cancel: no Job J-99 in this task") {
		t.Fatalf("an unknown Job: exit %d %s, want 1 and no Job J-99", code, a.h.errb)
	}
	if got := r20ClRecords(t, a); got != before {
		t.Fatalf("%d records before and %d after the refused request", before, got)
	}
}

// R9: the target Job is already closed: exit 1 and nothing written. The Job is cancelled the way the
// worker does it, with the task epoch the Job agent holds, then a person asks again.
func TestR20ClR9ClosedJob(t *testing.T) {
	a, _ := kbRoleAgent(t)
	if code := a.cancelJob(t, "J-1"); code != 0 {
		t.Fatalf("the agent's cancel under its task epoch: exit %d %s", code, a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %q, want cancelled", got)
	}
	r18PersonAtTerminal(t, "dana")
	before := r20ClRecords(t, a)
	code := r20ClCancel(a)
	if code != 1 || !strings.Contains(a.h.errb.String(), "grid job cancel: J-1 is already cancelled") {
		t.Fatalf("a closed Job: exit %d %s, want 1 and already cancelled", code, a.h.errb)
	}
	if got := r20ClRecords(t, a); got != before {
		t.Fatalf("%d records before and %d after the refused request", before, got)
	}
}

// R10: the same member on another machine. The holder is worker@<this machine>; the person is
// worker on another machine id. No machine is compared, so this is N1: exit 0 and one unleased
// control whose writer is <member>@<the person's machine>. Before the change the writer
// worker@<other machine> was refused as a stale lease.
func TestR20ClR10PersonOnAnotherMachine(t *testing.T) {
	a, _ := kbRoleAgent(t)
	const other = "01ARZ3NDEKTSV4RRFFQ69G5FAX"
	a.h.write(filepath.Join(a.h.gridHome, "machine"), other+"\n")
	a.h.env.Machine = other
	r18PersonAtTerminal(t, "worker")
	before := r20ClControls(t, a)
	if code := r20ClCancel(a); code != 0 {
		t.Fatalf("a person on another machine: exit %d %s, want 0", code, a.h.errb)
	}
	if !strings.Contains(a.h.errb.String(), r20ClSaved) {
		t.Fatalf("stderr does not say the request was saved: %q", a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q after the request alone, want running", got)
	}
	after := r20ClControls(t, a)
	if len(after) != len(before)+1 {
		t.Fatalf("want one new control, got %d before and %d after", len(before), len(after))
	}
	c := after[len(after)-1]
	if c.Lease != nil || c.Fence != 0 || c.Author.ID != "worker" || c.Worker != "worker@"+other {
		t.Fatalf("want an unleased request by worker@%s: lease %v fence %d author %+v worker %s", other, c.Lease, c.Fence, c.Author, c.Worker)
	}
}

// Nobody holds the task lease: the lease file names no holder, so the person's cancel is saved as a
// request too (decision rule point 2) and the Job stays Running until a holder's pass.
func TestR20ClNobodyHoldsTheRequestIsSaved(t *testing.T) {
	a, _ := kbRoleAgent(t)
	if err := a.ts.ReleaseTask(a.w); err != nil {
		t.Fatal(err)
	}
	r18PersonAtTerminal(t, "worker")
	before := r20ClControls(t, a)
	if code := r20ClCancel(a); code != 0 || !strings.Contains(a.h.errb.String(), r20ClSaved) {
		t.Fatalf("a cancel with no holder: exit %d %s, want 0 and saved", code, a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q, want running until a holder's pass", got)
	}
	if after := r20ClControls(t, a); len(after) != len(before)+1 || after[len(after)-1].Lease != nil {
		t.Fatalf("want one new unleased control, got %d before and %+v after", len(before), after)
	}
}

// N1 with --json prints the request with pending true, as an unleased answer does.
func TestR20ClRequestJSONIsPending(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	if code := r20ClCancel(a, "--json"); code != 0 {
		t.Fatalf("exit %d %s", code, a.h.errb)
	}
	var out struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Pending bool   `json:"pending"`
	}
	if err := json.Unmarshal(a.h.out.Bytes(), &out); err != nil || out.ID == "" || out.Type != "control" || !out.Pending {
		t.Fatalf("want the request with pending true: %v %+v\n%s", err, out, a.h.out)
	}
	if c := a.lastControl(t); c.ID != out.ID {
		t.Fatalf("the printed id %s is not the saved request %s", out.ID, c.ID)
	}
}

// N1 then N2 end to end, in process: a person who does not hold the lease saves a request with the CLI,
// then the holder's pass (`grid feed tick`, under the holder's env with the task epoch) writes the
// leased control. J-1 ends Cancelled, by a leased control whose author is the holder and whose re is
// the request id; the request stays the person's record.
func TestR20ClRequestThenHolderPassCancelsTheJob(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	if code := r20ClCancel(a, "--json"); code != 0 {
		t.Fatalf("the request: exit %d %s", code, a.h.errb)
	}
	var req struct {
		ID      string `json:"id"`
		Pending bool   `json:"pending"`
	}
	if err := json.Unmarshal(a.h.out.Bytes(), &req); err != nil || req.ID == "" || !req.Pending {
		t.Fatalf("the request: %v %s", err, a.h.out)
	}
	if got := a.status(t, "J-1"); got != contract.Running {
		t.Fatalf("J-1 is %q after the request alone, want running", got)
	}
	// The holder: member worker, role worker (the app's value, which the CLI writes as cli), the task
	// epoch it holds, and no Job lease.
	t.Setenv("GRID_MEMBER", "worker")
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE", "")
	t.Setenv("GRID_TASK_EPOCH", "")
	t.Setenv("GRID_LEASE_EPOCH", a.epoch())
	a.h.out.Reset()
	a.h.errb.Reset()
	if code := a.h.run(a.path, "feed", "tick", "--task", a.path, "--json"); code != 0 {
		t.Fatalf("the holder's pass: exit %d %s", code, a.h.errb)
	}
	if got := a.status(t, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %q after the holder's pass, want cancelled", got)
	}
	var carried []feed.Record
	for _, c := range r20ClControls(t, a) {
		if c.Re != nil && *c.Re == req.ID {
			carried = append(carried, c)
		}
	}
	if len(carried) != 1 {
		t.Fatalf("want one control whose re is the request %s, got %d: %+v", req.ID, len(carried), carried)
	}
	c := carried[0]
	if c.Lease == nil || *c.Lease != "task" || c.Fence != a.w.Fence || c.Worker != "worker@"+feedMachine || c.Author.ID != "worker" || c.ID == req.ID {
		t.Fatalf("want the holder's leased control: %+v", c)
	}
	if c.Body["status"] != "cancelled" || c.Body["target"] != "J-1" || c.Body["close_reason"] != "scope withdrawn" {
		t.Fatalf("the leased control's body: %v", c.Body)
	}
	// The request is still the person's, unleased, authored by dana.
	for _, r := range r20ClControls(t, a) {
		if r.ID == req.ID && (r.Lease != nil || r.Author.ID != "dana" || r.Author.Role != "owner") {
			t.Fatalf("the request changed: %+v", r)
		}
	}
}

// `grid job cancel --help` holds the decision's text for a person (D-r19-person-cancel-holder.md:160-164)
// verbatim and keeps the agent and typed-epoch lines (:42-43 of the old help) as they were.
func TestR20ClJobCancelHelpIsTheDecisionsText(t *testing.T) {
	h, path := feedHarness(t)
	if code := h.run(path, "job", "cancel", "--help"); code != 0 {
		t.Fatalf("job cancel --help: exit %d %s", code, h.errb)
	}
	help := h.errb.String() + h.out.String()
	for _, want := range []string{
		"A person (role owner, member or lead) types no lease and no epoch. When the task lease is\n" +
			"held by your own <member>@<machine> (GRID_MEMBER on this machine), the cancel is written\n" +
			"at once under it. When another holder holds it (the machine's worker, worker@<machine>, by\n" +
			"default), the cancel is saved as your request and the holder writes it under its lease on\n" +
			"its next pass; `grid job show <job>` then shows it Cancelled. Both exit 0.\n",
		"An agent cancels with the GRID_TASK_EPOCH its worker gave it. A --lease-epoch typed on the\n" +
			"command line is used as typed, so a guessed or old one is refused as a stale lease (exit 3).\n",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("job cancel --help does not hold:\n%s\nin:\n%s", want, help)
		}
	}
	for _, gone := range []string{"For another member the lease names a different holder", "reads the task lease"} {
		if strings.Contains(help, gone) {
			t.Fatalf("job cancel --help still holds the old text %q:\n%s", gone, help)
		}
	}
}
