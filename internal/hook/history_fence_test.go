package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// bash runs the real pretooluse hook as a builder agent in its code folder.
func (f *holdFixture) bash(t *testing.T, command string) holdResult {
	t.Helper()
	b, err := json.Marshal(hookio.Input{HookEventName: "PreToolUse", ToolName: "Bash", Cwd: f.code, SessionID: "session-demo", AgentID: "agent-1", AgentType: "grid:grid-builder", ToolInput: map[string]any{"command": command}})
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"CLAUDE_PLUGIN_DATA": f.data, "GRID_PROJECTS": filepath.Join(f.home, "projects"), "GRID_HOME": f.home, "GRID_AGENT": "1", "CLAUDECODE": "1"}
	var out, stderr bytes.Buffer
	if code := Run(Options{Event: "pretooluse", Stdin: bytes.NewReader(b), Stdout: &out, Stderr: &stderr, Resolver: f.resolver, Cwd: f.code, Getenv: func(k string) string { return env[k] }}); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got := holdResult{out: out.String(), stderr: stderr.String()}
	if out.Len() > 0 {
		var decoded struct {
			Output struct {
				Behavior string `json:"permissionDecision"`
				Reason   string `json:"permissionDecisionReason"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatalf("%s: %v", out.String(), err)
		}
		got.behavior, got.reason = decoded.Output.Behavior, decoded.Output.Reason
	}
	return got
}

// A task with no approval.json is released by a dated "plan accepted" line in
// history.md, so an agent that can append one releases its own plan. The hook
// refuses the Bash write, and the plan stays held.
func TestAgentBashAppendToHistoryDoesNotReleaseThePlan(t *testing.T) {
	const line = "- 2026-11-09T10:00 · plan accepted · x · by abdu"
	for _, tc := range []struct {
		name, command string
		deny          bool
	}{
		{"append", "echo '" + line + "' >> HISTORY", true},
		{"overwrite", "echo '" + line + "' > HISTORY", true},
		{"tee", "echo '" + line + "' | tee -a HISTORY", true},
		{"sed", "sed -i '$a " + line + "' HISTORY", true},
		{"python", `python3 -c "open('HISTORY', 'a').write('` + line + `')"`, true},
		{"cd then append", "cd TASK && echo '" + line + "' >> history.md", true},
		{"task.md control", "echo '" + line + "' >> TASK/task.md", true},
		{"approval.json control", "echo '" + line + "' >> TASK/approval.json", true},
		{"read", "tail -n 5 HISTORY", false},
		{"lookalike", "echo x >> HISTORY.bak", false},
		{"own writer", "grid history add 'checked the plan' --task T-01-demo", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := holdSetup(t)
			history := filepath.Join(f.task, "history.md")
			if err := os.WriteFile(history, []byte("- 2026-01-01T00:00 · created · demo · by person\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			command := strings.ReplaceAll(strings.ReplaceAll(tc.command, "HISTORY", history), "TASK", f.task)
			got := f.bash(t, command)
			if denied := got.behavior == "deny"; denied != tc.deny {
				msg := "want deny=" + strconv.FormatBool(tc.deny) + " for " + command + ", got " + got.out + got.reason
				if !denied {
					// Run what the hook allowed, to show what it did to the plan.
					if sh, err := exec.LookPath("sh"); err == nil {
						_ = exec.Command(sh, "-c", command).Run()
					}
					text, _ := os.ReadFile(filepath.Join(f.task, "task.md"))
					after, _ := os.ReadFile(history)
					msg += "; plan held after it: " + strconv.FormatBool(task.PlanHeld(f.task, string(text), string(after)))
				}
				t.Fatal(msg)
			}
			if tc.deny && (got.stderr != "" || !strings.Contains(got.reason, "never by an agent")) {
				t.Fatalf("want the gate reason for %q, got %+v", command, got)
			}
		})
	}
}
