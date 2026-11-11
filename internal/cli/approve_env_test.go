package cli

import "path/filepath"

// memberShell is Env.Getenv for a person at a terminal who approves: the member
// id of the team and nothing else. grid approve records the approval as a Feed
// write, so the caller must be a person of the team (GRID_MEMBER) or hold a lease.
func memberShell(key string) string {
	if key == "GRID_MEMBER" {
		return "sam"
	}
	return ""
}

// person makes the harness a person at a terminal who approves: memberShell, and
// a machine id the Feed accepts (the harness machine, 01LAPTOP, is not a ULID).
func (h *harness) person() {
	h.t.Helper()
	h.env.Getenv = memberShell
	h.write(filepath.Join(h.gridHome, "machine"), feedMachine+"\n")
}
