package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

// hostBlock sets the host-guard level of the temp Grid home to block, as a person
// would with `grid hook level host-guard block`, so a test keeps covering the deny.
func hostBlock(f *fixture) {
	f.t.Helper()
	if err := (settings.Store{Home: f.grid}).SetHook("host-guard", "block"); err != nil {
		f.t.Fatal(err)
	}
}

// hostSetup is setup with host-guard at block: the tests that check the deny itself
// start here, so they keep covering it now that the default is warn.
func hostSetup(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	hostBlock(f)
	return f
}

// hostKind is one kind of file the host guard names: the path a file tool is given
// and the one plain sentence the guard says about it. Each sentence names the file
// and the grid verb an agent uses instead.
type hostKind struct {
	name, path, want string
}

func hostKinds(f *fixture) []hostKind {
	task := filepath.Join(f.projects, "p", "tasks", "T-01")
	record := func(p string) hostKind {
		return hostKind{"task record " + strings.TrimPrefix(p, task+string(filepath.Separator)), p,
			p + " is a task record that grid writes; use the grid task or grid question command instead of the Write or Edit tool"}
	}
	settingsPath := filepath.Join(f.grid, "settings.json")
	feed := filepath.Join(task, "feed", "writer.0.jsonl")
	hostPath := filepath.Join(f.grid, ".host", "approval")
	return []hostKind{
		record(filepath.Join(task, "task.md")),
		record(filepath.Join(task, "questions", "Q-1.md")),
		record(filepath.Join(task, "approval.json")),
		record(filepath.Join(task, "plan-revisions", "1.md")),
		record(filepath.Join(f.projects, "p", "workflows.md")),
		{"settings", settingsPath, settingsPath + " holds the person's Grid settings; grid hook level and grid settings change it"},
		{"feed", feed, feed + " is the task's Feed; post to it with grid feed or grid job"},
		{"host folder", hostPath, hostPath + " is in Grid's host folder, which only Grid writes"},
	}
}

// With no settings file, a Write to a task record is warned about and not refused:
// the decision has no behavior, one warning names the file, and the output is a
// systemMessage with no permission decision.
func TestHostGuardWarnsByDefault(t *testing.T) {
	f := setup(t)
	if _, err := os.Stat(filepath.Join(f.grid, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("the temp Grid home must have no settings.json: %v", err)
	}
	d := EvaluateHost(f.input("Write", filepath.Join(f.projects, "p", "tasks", "T-01", "task.md")), f.root, f.env)
	if d.Behavior == "deny" {
		t.Fatalf("default host-guard refused the write: %+v", d)
	}
	if d.Behavior != "" || len(d.Warnings) != 1 || !strings.Contains(d.Warnings[0], "task.md") {
		t.Fatalf("want no behavior and one warning that names task.md, got %+v", d)
	}
	out := string(d.Bytes())
	if !strings.Contains(out, `"systemMessage":"host-guard: `) || strings.Contains(out, "permissionDecision") {
		t.Fatalf("output must carry the warning and no decision: %s", out)
	}
	for _, k := range hostKinds(f) {
		t.Run(k.name, func(t *testing.T) {
			d := EvaluateHost(f.input("Write", k.path), f.root, f.env)
			if d.Behavior != "" || len(d.Warnings) != 1 || d.Warnings[0] != "host-guard: "+k.want {
				t.Fatalf("want no behavior and the warning %q, got %+v", "host-guard: "+k.want, d)
			}
		})
	}
}

// A person who sets host-guard to block gets the deny, with the same sentence.
func TestHostGuardDeniesAtBlock(t *testing.T) {
	f := setup(t)
	hostBlock(f)
	for _, k := range hostKinds(f) {
		for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
			t.Run(k.name+"/"+tool, func(t *testing.T) {
				d := EvaluateHost(f.input(tool, k.path), f.root, f.env)
				expect(t, d, "deny")
				if d.Reason != k.want || len(d.Warnings) != 0 {
					t.Fatalf("want the deny %q and no warning, got %+v", k.want, d)
				}
			})
		}
	}
}

// The texts name the file and an agent's own verb. They never say gate, held,
// attacker or that an agent cannot turn a guard off, and they never point an agent
// at a person's verb.
func TestHostGuardTextsAreOnePlainSentence(t *testing.T) {
	f := setup(t)
	hostBlock(f)
	for _, k := range hostKinds(f) {
		t.Run(k.name, func(t *testing.T) {
			d := EvaluateHost(f.input("Write", k.path), f.root, f.env)
			expect(t, d, "deny")
			low := strings.ToLower(d.Reason)
			for _, word := range []string{"gate", "held", "attacker", "turn a guard off", "grid approve", "grid answer", "never by an agent"} {
				if strings.Contains(low, word) {
					t.Fatalf("%q in %q", word, d.Reason)
				}
			}
			if strings.Count(d.Reason, ".") != strings.Count(k.path, ".") {
				t.Fatalf("want one sentence with no full stop of its own: %q", d.Reason)
			}
		})
	}
}
