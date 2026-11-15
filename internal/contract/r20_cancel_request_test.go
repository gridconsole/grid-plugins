package contract

import (
	"encoding/json"
	"reflect"
	"testing"
)

// r20CrRequest is a person's unleased cancel request for J-1 (D-r19 rule point 2):
// author role, no lease, gen 0 and fence 0, body {target, status, reason,
// close_reason}.
func r20CrRequest(t *testing.T, id int, role string) Post {
	t.Helper()
	p := move(t, id, Cancelled)
	p.Author.ID, p.Author.Role = "dana", role
	p.Tier = TierFor(role)
	p.Worker = "dana@01K9J000000000000000000002"
	p.Lease, p.Gen, p.Fence = nil, 0, 0
	return p
}

func r20CrValid(t *testing.T, p Post) error {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return Validate("control", data)
}

// N6: the fold counts a person's unleased cancel request and leaves the Job it
// names as it was; only the holder's leased control moves it.
func TestR20CrN6FoldLeavesJobForRequest(t *testing.T) {
	without := Fold(base(t))
	request := r20CrRequest(t, 4, "owner")
	with := Fold(append(base(t), request))
	if v := verdict(with, request.ID); v.Kind != "counted" {
		t.Fatalf("the request is %q (%s), want counted", v.Kind, v.Reason)
	}
	got, want := with.Jobs["J-1"], without.Jobs["J-1"]
	if got.Status != Running || !reflect.DeepEqual(got, want) {
		t.Fatalf("the request moved J-1: got %+v, want %+v", got, want)
	}
	// The holder's leased cancel still closes the Job.
	leased := move(t, 5, Cancelled)
	if s := Fold(append(base(t), request, leased)); s.Jobs["J-1"].Status != Cancelled {
		t.Fatalf("the leased cancel left J-1 %s", s.Jobs["J-1"].Status)
	}
}

// N3, N4 and the roles: the leaseless cancel branch takes owner, member and lead,
// never a guest or an agent role.
func TestR20CrSchemaRoles(t *testing.T) {
	for _, role := range []string{"owner", "member", "lead"} {
		if err := r20CrValid(t, r20CrRequest(t, 4, role)); err != nil {
			t.Fatalf("%s: the leaseless cancel was refused: %v", role, err)
		}
	}
	for _, role := range []string{"guest", "worker", "cli", "agent", "reviewer", "external"} {
		if err := r20CrValid(t, r20CrRequest(t, 4, role)); err == nil {
			t.Fatalf("%s: the schema took a leaseless cancel", role)
		}
	}
}

// N5 and the pins: only status cancelled, gen 0, fence 0, a Job id target and the
// four body fields are taken without a lease.
func TestR20CrN5SchemaShape(t *testing.T) {
	if err := r20CrValid(t, r20CrRequest(t, 4, "member")); err != nil {
		t.Fatalf("the leaseless cancel was refused: %v", err)
	}
	for _, status := range []Status{Posted, Ready, Running, Submitted, Accepted, Stale, Parked, Rejected, Status("conflict")} {
		p := r20CrRequest(t, 4, "member")
		changeBody(t, &p, func(v map[string]any) { v["status"] = status })
		if err := r20CrValid(t, p); err == nil {
			t.Fatalf("status %s was taken without a lease", status)
		}
	}
	for name, mutate := range map[string]func(*Post){
		"gen 1":           func(p *Post) { p.Gen = 1 },
		"fence 1":         func(p *Post) { p.Fence = 1 },
		"ULID target":     func(p *Post) { changeBody(t, p, func(v map[string]any) { v["target"] = "01ARZ3NDEKTSV4RRFFQ69G5FAV" }) },
		"no close_reason": func(p *Post) { changeBody(t, p, func(v map[string]any) { delete(v, "close_reason") }) },
		"no reason":       func(p *Post) { changeBody(t, p, func(v map[string]any) { delete(v, "reason") }) },
		"attempt":         func(p *Post) { changeBody(t, p, func(v map[string]any) { v["attempt"] = 1 }) },
		"answer":          func(p *Post) { changeBody(t, p, func(v map[string]any) { v["answer"] = "x" }) },
	} {
		p := r20CrRequest(t, 4, "member")
		mutate(&p)
		if err := r20CrValid(t, p); err == nil {
			t.Fatalf("%s: the schema took it without a lease", name)
		}
	}
}
