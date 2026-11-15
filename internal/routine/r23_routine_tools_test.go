package routine

import (
	"context"
	"os/exec"
	"reflect"
	"testing"
)

// A routine names the read tools of its mail, chat or calendar source in its own
// front matter, `tools`, and the routine adds them to its allow list as written.
// Without the key the argv is the one TestProcessRoutineLimits pins.
func r23ToolsEntry(t *testing.T, tools string) Entry {
	t.Helper()
	data := []byte("---\nname: inbox\nevery: 1h\nrun: signals\nmachine: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" + tools + "---\n")
	e, err := decode("inbox", data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return e
}

func r23RunArgv(t *testing.T, e Entry) []string {
	t.Helper()
	var args []string
	p := ProcessExecutor{runCommand: func(cmd *exec.Cmd) error {
		args = cmd.Args
		return nil
	}}
	if err := p.Execute(context.Background(), Store{GridHome: t.TempDir(), Project: "example"}, e); err != nil {
		t.Fatal(err)
	}
	return args
}

func TestR23RoutineToolsJoinTheAllowList(t *testing.T) {
	e := r23ToolsEntry(t, "tools:\n  - mcp__mail__search\n  - mcp__mail__read\n")
	args := r23RunArgv(t, e)
	wantRules := "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*),mcp__mail__search,mcp__mail__read"
	wantPrompt := "Run the grid:signals skill once. Work only in the Grid data folder. The project is example. Read the source with these tools: mcp__mail__search, mcp__mail__read."
	for i, a := range args {
		if a == "--allowedTools" && args[i+1] != wantRules {
			t.Errorf("--allowedTools %q, want %q", args[i+1], wantRules)
		}
		if a == "-p" && args[i+1] != wantPrompt {
			t.Errorf("prompt %q, want %q", args[i+1], wantPrompt)
		}
	}
}

func TestR23RoutineWithoutToolsKeepsItsArgv(t *testing.T) {
	args := r23RunArgv(t, r23ToolsEntry(t, ""))
	want := []string{"claude", "-p", "Run the grid:signals skill once. Work only in the Grid data folder. The project is example.", "--allowedTools", "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*)", "--max-budget-usd", "1", "--max-turns", "10", "--permission-mode", "dontAsk"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("argv %q, want %q", args, want)
	}
}

// The timer guard covers the tool list: a synced edit that adds a tool cannot
// change what an installed timer runs. An entry with no tools keeps its hash.
func TestR23RoutineToolsChangeTheFingerprint(t *testing.T) {
	plain, err := r23ToolsEntry(t, "").Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := Entry{Run: "signals", Every: "1h"}.Fingerprint()
	if err != nil || plain != legacy {
		t.Fatalf("an entry without tools changed its hash: %v", err)
	}
	with, err := r23ToolsEntry(t, "tools:\n  - mcp__mail__read\n").Fingerprint()
	if err != nil || with == plain {
		t.Fatalf("tools left the fingerprint unchanged: %v", err)
	}
	other, err := r23ToolsEntry(t, "tools:\n  - mcp__mail__search\n").Fingerprint()
	if err != nil || other == with {
		t.Fatalf("a different tool left the fingerprint unchanged: %v", err)
	}
}

// A tool name is one word of the comma list the allow list is: a name that
// holds a comma or a space would become two rules, so it is refused as written.
func TestR23RoutineToolNamesAreOneRuleEach(t *testing.T) {
	for _, bad := range []string{`"mcp__a,Bash"`, `"mcp__a b"`, `""`, `"mcp__a\n"`} {
		data := []byte("---\nname: inbox\nevery: 1h\nrun: signals\nmachine: 01ARZ3NDEKTSV4RRFFQ69G5FAV\ntools: [" + bad + "]\n---\n")
		if _, err := decode("inbox", data); err == nil || err.Error() != "invalid tools" {
			t.Errorf("tool %s: error %v, want %q", bad, err, "invalid tools")
		}
	}
}
