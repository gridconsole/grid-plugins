package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// An invalid inbox policy keeps its object and exit status, and gains the
// stable code and the signal-style message that the other signal objects carry.
func TestSignalMoveInvalidPolicyCode(t *testing.T) {
	h, _, _ := actFixture(t, "placed")
	h.record(project.Record{Name: "personal"})
	h.env.Getenv = func(string) string { return "" }
	h.write(filepath.Join(h.gridHome, "workflows.md"), "---\ninbox_workspace: shared\n---\n# Workflows\n")
	code := h.run(h.home, "signal", "move", actID, "--project", "example", "--to", "-", "--json")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("stdout %q: %v", h.out.String(), err)
	}
	want := map[string]string{
		"error":   "invalid_policy",
		"key":     "inbox_workspace",
		"code":    "invalid_policy",
		"message": "invalid inbox_workspace in workspace workflow (only personal is supported)",
	}
	if len(got) != len(want) {
		t.Fatalf("object %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if h.errb.String() != "grid signal move: "+want["message"]+"\n" {
		t.Errorf("stderr changed: %q", h.errb.String())
	}
}

// The move handler runs the person gate itself, so its refusal keeps the code too.
func TestSignalMovePersonGateCode(t *testing.T) {
	h, _, _ := actFixture(t, "new")
	h.record(project.Record{Name: "second"})
	h.env.Getenv = func(string) string { return "" }
	h.env.Procs = testProcs("codex")
	h.write(filepath.Join(h.gridHome, "projects", "example", "workflows.md"), "---\nperson_verbs: [signal move]\n---\n")
	args := []string{"signal", "move", actID, "--project", "example", "--to", "second", "--json"}
	if code := h.run(h.home, args...); code != 3 {
		t.Fatalf("exit %d, want 3: %s %s", code, h.out, h.errb)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("stdout %q: %v", h.out.String(), err)
	}
	if got["error"] != "person_required" || got["code"] != CodePersonRequired || got["message"] == "" || got["reason"] == "" {
		t.Errorf("refusal object %v", got)
	}
}
