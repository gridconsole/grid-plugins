package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ggPersonIsolate points every home variable under the harness temp root and
// fails when HOME is not under it.
func ggPersonIsolate(t *testing.T, h *harness) {
	t.Helper()
	t.Setenv("HOME", h.home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(h.home, ".codex"))
	t.Setenv("GRID_HOME", h.gridHome)
	if !strings.HasPrefix(os.Getenv("HOME"), h.root) {
		t.Fatalf("HOME is not under the temp root")
	}
}

// ggPersonCheckRefusal runs the refusal with --json and again without it, checks
// the plain run is byte-equal on stderr and exit with empty stdout, and returns
// the decoded object and the --json stderr.
func ggPersonCheckRefusal(t *testing.T, h *harness, cwd string, args ...string) (map[string]string, string) {
	t.Helper()
	if code := h.run(cwd, append(append([]string{}, args...), "--json")...); code != 3 {
		t.Fatalf("--json exit %d: %s %s", code, h.out, h.errb)
	}
	var obj map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &obj); err != nil {
		t.Fatalf("stdout %q: %v", h.out, err)
	}
	jsonErr := h.errb.String()
	if code := h.run(cwd, args...); code != 3 {
		t.Fatalf("plain exit %d: %s %s", code, h.out, h.errb)
	}
	if h.errb.String() != jsonErr {
		t.Fatalf("plain stderr %q differs from --json stderr %q", h.errb, jsonErr)
	}
	if h.out.Len() != 0 {
		t.Fatalf("plain stdout not empty: %q", h.out)
	}
	return obj, jsonErr
}

func ggPersonWant(t *testing.T, obj map[string]string, stderr, reason, hint string) {
	t.Helper()
	if obj["error"] != "person_required" || obj["reason"] != reason || obj["hint"] != hint {
		t.Fatalf("object %v, want error person_required, reason %s, hint %q", obj, reason, hint)
	}
	if obj["code"] != "person_required" {
		t.Fatalf("code %q, want person_required (object %v)", obj["code"], obj)
	}
	if want := strings.TrimSuffix(stderr, "\n"); obj["message"] != want || want == "" {
		t.Fatalf("message %q, want stderr without final newline %q", obj["message"], want)
	}
}

func TestGgPersonRefusalCheckHasCode(t *testing.T) {
	for name, tc := range map[string]struct {
		set    map[string]string
		reason string
	}{
		"worker role":   {map[string]string{"GRID_ROLE": "worker"}, "agent_role"},
		"unknown role":  {map[string]string{"GRID_ROLE": "reviewer"}, "agent_role"},
		"agent marker":  {map[string]string{"GRID_ROLE": "owner", "GRID_AGENT": "a1"}, "agent_worker"},
		"worker id":     {map[string]string{"GRID_WORKER_ID": "w1"}, "agent_worker"},
		"worker marker": {map[string]string{"GRID_ROLE": "member", "GRID_WORKER": "w1"}, "agent_worker"},
	} {
		t.Run(name, func(t *testing.T) {
			h := personCheckHarness(t, tc.set)
			ggPersonIsolate(t, h)
			obj, stderr := ggPersonCheckRefusal(t, h, h.home, "person", "check")
			ggPersonWant(t, obj, stderr, tc.reason, personCheckHint)
		})
	}
}

func TestGgPersonRefusalRoleHasCode(t *testing.T) {
	h, dir := feedHarness(t)
	ggPersonIsolate(t, h)
	q := questionAdd(t, h, dir, "visitor")
	h.write(filepath.Join(dir, "workflow.md"), "---\nperson_verbs: {answer: [member, lead]}\n---\n")
	t.Setenv("GRID_ROLE", "guest")
	t.Setenv("GRID_ACTOR", "visitor")
	t.Setenv("GRID_MEMBER", "visitor")
	obj, stderr := ggPersonCheckRefusal(t, h, dir, "answer", dir, q.ID, "Yes")
	ggPersonWant(t, obj, stderr, "role_not_allowed", roleRefusalHint+"; allowed: member, lead")
}
