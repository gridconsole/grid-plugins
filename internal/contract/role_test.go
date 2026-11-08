package contract

import "testing"

func TestRolePrefix(t *testing.T) {
	for _, role := range []string{"owner", "member", "lead", "guest", "external", "worker", "agent", "cli", "builder", "unknown"} {
		for _, spelling := range []string{role, "grid-" + role, " grid-" + role + " "} {
			if got := NormalizeRole(spelling); got != role {
				t.Errorf("NormalizeRole(%q)=%q, want %q", spelling, got, role)
			}
			if got := TierFor(spelling); got != TierFor(role) {
				t.Errorf("TierFor(%q)=%q, want %q", spelling, got, TierFor(role))
			}
		}
	}
	if NormalizeRole("grid-") != "grid-" {
		t.Fatal("bare prefix became an empty local role")
	}
	if NormalizeRole(" GRID-WORKER ") != "worker" {
		t.Fatal("role case was not normalized")
	}
	if NormalizeRole("grid-grid-worker") != "grid-worker" {
		t.Fatal("only one optional prefix is removed")
	}
}
