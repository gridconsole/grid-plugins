package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// postResult files the contract's own result fixture on the fixture task's Feed
// for Job J-1.
func (f *eventFixture) postResult(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "result", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Task, Worker string
		Fence        int
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	id, err := feed.TaskID(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte(fx.Task), []byte(id))
	feedDir := filepath.Join(f.dir, "feed")
	if err := os.MkdirAll(feedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var one bytes.Buffer
	if err := json.Compact(&one, raw); err != nil {
		t.Fatal(err)
	}
	one.WriteByte('\n')
	if err := os.WriteFile(filepath.Join(feedDir, fmt.Sprintf("%s.%d.jsonl", fx.Worker, fx.Fence)), one.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func stopInput(event string) hookio.Input {
	in := hookio.Input{HookEventName: event}
	if event == "SubagentStop" {
		in.AgentType = "grid:grid-builder"
	}
	return in
}

func TestStopJobWithResultStops(t *testing.T) {
	for _, event := range []string{"Stop", "SubagentStop"} {
		t.Run(event, func(t *testing.T) {
			f := eventSetup(t)
			f.env["GRID_JOB"] = "J-1"
			f.postResult(t)
			out, log, code := f.run(t, event, stopInput(event))
			if code != 0 || out != "" || log != "" {
				t.Fatalf("result posted: code %d out %q log %q", code, out, log)
			}
		})
	}
}

func TestStopJobWithoutResultBlocksOnce(t *testing.T) {
	for _, event := range []string{"Stop", "SubagentStop"} {
		t.Run(event, func(t *testing.T) {
			f := eventSetup(t)
			f.env["GRID_JOB"] = "J-1"
			in := stopInput(event)
			_, log, code := f.run(t, event, in)
			if code != 2 {
				t.Fatalf("no result: code %d, want 2: %s", code, log)
			}
			if strings.Count(strings.TrimSpace(log), "\n") != 0 || !strings.Contains(log, "J-1") {
				t.Fatalf("want one plain line naming the Job: %q", log)
			}
			in.StopHookActive = true
			_, log, code = f.run(t, event, in)
			if code != 0 || log != "" {
				t.Fatalf("stop_hook_active: code %d log %q", code, log)
			}
		})
	}
}

func TestStopWithNoJobStopsSilently(t *testing.T) {
	f := eventSetup(t)
	// The old checks would have blocked here: an empty brief Outcome and a done
	// brief with no Outcome section.
	sub := filepath.Join(f.dir, "subagents")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "01-build.md"), []byte("---\nrole: builder\nstatus: done\n---\n## Outcome\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, in := range []hookio.Input{
		{HookEventName: "Stop"},
		{HookEventName: "SubagentStop", AgentType: "grid:grid-builder"},
		{HookEventName: "SubagentStop", AgentType: "grid:grid-scout"},
	} {
		out, log, code := f.run(t, in.HookEventName, in)
		if code != 0 || out != "" || log != "" {
			t.Fatalf("%s %s with no Job: code %d out %q log %q", in.HookEventName, in.AgentType, code, out, log)
		}
	}
}

func TestSubagentStopScoutWithJobEnvStopsSilently(t *testing.T) {
	f := eventSetup(t)
	f.env["GRID_JOB"] = "J-1"
	out, log, code := f.run(t, "SubagentStop", hookio.Input{AgentType: "grid:grid-scout"})
	if code != 0 || out != "" || log != "" {
		t.Fatalf("scout: code %d out %q log %q", code, out, log)
	}
}
