package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// iaWrRoleLine is the host-policy refusal line of grid workflow write.
const iaWrRoleLine = "grid workflow write: caller role is not allowed to write workflows"

// iaWrHarness returns a harness with a temporary Grid home, named by GRID_HOME,
// and a caller whose GRID_ROLE is role.
func iaWrHarness(t *testing.T, role string) *harness {
	t.Helper()
	h := newHarness(t)
	t.Setenv("GRID_HOME", h.gridHome)
	h.env.Getenv = func(key string) string {
		if key == "GRID_ROLE" {
			return role
		}
		return ""
	}
	return h
}

// iaWrRun runs grid workflow write with text on stdin and returns the exit
// status, stdout and stderr.
func iaWrRun(h *harness, text string, args ...string) (int, string, string) {
	h.env.Stdin = strings.NewReader(text)
	code := h.run(h.root, append([]string{"workflow", "write"}, args...)...)
	return code, h.out.String(), h.errb.String()
}

// A role the host policy refuses keeps main's stderr line and exit 3; with
// --json it prints the person gate object, without --json nothing on stdout.
func TestIaWrRoleRefusal(t *testing.T) {
	want := map[string]string{
		"error":   "person_required",
		"reason":  "role_not_allowed",
		"hint":    "caller role is not allowed to write workflows",
		"code":    "person_required",
		"message": iaWrRoleLine,
	}
	for _, role := range []string{"guest", "agent", "unknown"} {
		t.Run(role, func(t *testing.T) {
			h := iaWrHarness(t, role)
			code, out, errb := iaWrRun(h, cliWorkflow, "--rev", "none", "--file", "-", "--json")
			if code != 3 || errb != iaWrRoleLine+"\n" {
				t.Fatalf("json: exit %d, stderr %q", code, errb)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("json stdout is not one object: %v: %q", err, out)
			}
			if len(got) != len(want) {
				t.Fatalf("json stdout keys: %q", out)
			}
			for key, value := range want {
				if got[key] != value {
					t.Fatalf("json stdout %s = %v, want %q: %q", key, got[key], value, out)
				}
			}
			plainCode, plainOut, plainErr := iaWrRun(h, cliWorkflow, "--rev", "none", "--file", "-")
			if plainCode != 3 || plainOut != "" || plainErr != errb {
				t.Fatalf("plain: exit %d, stdout %q, stderr %q, json stderr %q", plainCode, plainOut, plainErr, errb)
			}
			entries, err := os.ReadDir(h.gridHome)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refusal wrote files: %v, %v", entries, err)
			}
		})
	}
}

// A stale revision keeps exit 3 and its stderr; with --json it prints the
// envelope with code workflow_file_changed.
func TestIaWrChanged(t *testing.T) {
	if CodeWorkflowFileChanged != "workflow_file_changed" {
		t.Fatalf("CodeWorkflowFileChanged = %q", CodeWorkflowFileChanged)
	}
	changed := strings.ReplaceAll(cliWorkflow, "review rounds: 2", "review rounds: 3")
	for _, rev := range []string{"000000000000", "none"} {
		t.Run(rev, func(t *testing.T) {
			h := iaWrHarness(t, "")
			target := filepath.Join(h.gridHome, "workflows.md")
			h.write(target, cliWorkflow)
			code, out, errb := iaWrRun(h, changed, "--rev", rev, "--file", "-", "--json")
			if code != 3 || !strings.Contains(errb, "workflow changed since read") {
				t.Fatalf("json: exit %d, stdout %q, stderr %q", code, out, errb)
			}
			assertErrorOutput(t, out, errb, CodeWorkflowFileChanged)
			plainCode, plainOut, plainErr := iaWrRun(h, changed, "--rev", rev, "--file", "-")
			if plainCode != 3 || plainOut != "" || plainErr != errb {
				t.Fatalf("plain: exit %d, stdout %q, stderr %q, json stderr %q", plainCode, plainOut, plainErr, errb)
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != cliWorkflow {
				t.Fatalf("changed on refusal: %q, %v", data, err)
			}
		})
	}
}

// iaWrReadme returns contract/README.md.
func iaWrReadme(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The contract names the new code in the stable-code table and both refusals
// in the grid workflow write section.
func TestIaWrReadme(t *testing.T) {
	text := iaWrReadme(t)
	_, table, ok := strings.Cut(text, "\n| Stable code | Meaning |\n")
	if !ok {
		t.Fatal("README has no stable-code table")
	}
	rows := 0
	for _, line := range strings.Split(table, "\n") {
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(line, "|")
		if len(cells) > 2 && strings.TrimSpace(cells[1]) == "`workflow_file_changed`" {
			rows++
		}
	}
	if rows != 1 {
		t.Fatalf("stable-code table has %d workflow_file_changed rows, want 1", rows)
	}
	_, rest, ok := strings.Cut(text, "\n### `grid workflow write ")
	if !ok {
		t.Fatal("README has no grid workflow write section")
	}
	section, _, _ := strings.Cut(rest, "\n### ")
	for _, word := range []string{"workflow_file_changed", "person_required", "role_not_allowed"} {
		if !strings.Contains(section, word) {
			t.Errorf("grid workflow write section does not name %s", word)
		}
	}
	if strings.Contains(text, "`workflow_changed`") {
		t.Error("README names `workflow_changed`")
	}
}
