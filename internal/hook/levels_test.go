package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// Exercise public dispatch, including the component aliases, with real files.
// Project cases deliberately oppose the personal value.
func levelCases(t *testing.T, name string, check func(*testing.T, *eventFixture, bool, bool, bool)) {
	t.Helper()
	for _, mode := range []string{"default", "off", "warn", "block", "project-off", "project-warn", "project-block", "bad-personal", "bad-project", "bad-resolution"} {
		t.Run(name+"/"+mode, func(t *testing.T) {
			f := eventSetup(t)
			s := settings.Store{Home: f.home}
			level := strings.TrimPrefix(mode, "project-")
			if strings.HasPrefix(mode, "project-") || mode == "bad-project" {
				if err := project.Write(f.home, project.Record{Name: "example"}); err != nil {
					t.Fatal(err)
				}
				f.env["GRID_PROJECT"] = "example"
				personal := "off"
				if level == "off" {
					personal = "block"
				}
				if err := s.SetHook(name, personal); err != nil {
					t.Fatal(err)
				}
				s.Project = "example"
			}
			if level == "off" || level == "warn" || level == "block" {
				if err := s.SetHook(name, level); err != nil {
					t.Fatal(err)
				}
			}
			broken := strings.HasPrefix(mode, "bad-")
			if broken {
				if mode == "bad-resolution" {
					f.env["GRID_PROJECT"] = "missing-project"
				} else {
					// A directory is unreadable as settings even when tests run privileged.
					path := filepath.Join(f.home, "settings.json")
					if mode == "bad-project" {
						path = filepath.Join(f.home, "projects", "example", "settings.json")
					}
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			check(t, f, level != "off", level == "warn", broken)
		})
	}
}

func TestCatalogContextLevels(t *testing.T) {
	for _, tc := range []struct {
		name, event string
		in          hookio.Input
		text        string
	}{
		{"session-start", "SessionStart", hookio.Input{}, "Grid tracks tasks"},
		{"precompact", "PreCompact", hookio.Input{}, "acceptance ids"},
		{"user-prompt", "UserPromptSubmit", hookio.Input{Prompt: "/grid:plan"}, "Brain recall"},
		{"subagent-start", "SubagentStart", hookio.Input{AgentType: "grid:grid-builder"}, "Brain recall"},
	} {
		levelCases(t, tc.name, func(t *testing.T, f *eventFixture, on, warn, broken bool) {
			out, log, code := f.run(t, tc.event, tc.in)
			if code != 0 || strings.Contains(out, tc.text) != on {
				t.Fatalf("context: %d %q %q", code, out, log)
			}
			if broken && !strings.Contains(log, "hook settings") {
				t.Fatalf("missing settings diagnostic: %q", log)
			}
		})
	}
}

func TestCatalogLogLevels(t *testing.T) {
	for _, component := range []bool{false, true} {
		suffix := "event"
		if component {
			suffix = "component"
		}
		t.Run(suffix, func(t *testing.T) {
			levelCases(t, "history-log", func(t *testing.T, f *eventFixture, on, warn, broken bool) {
				event := "PostToolUse"
				if component {
					event = "history-log"
				}
				_, log, code := f.run(t, event, hookio.Input{HookEventName: "PostToolUse", ToolName: "Write", ToolInput: map[string]any{"file_path": filepath.Join(f.dir, "notes.md")}})
				b, err := os.ReadFile(filepath.Join(f.dir, "history.md"))
				if err != nil {
					t.Fatal(err)
				}
				if code != 0 || strings.Contains(string(b), "file written") != on {
					t.Fatalf("history: %d %q %q", code, b, log)
				}
				if broken && !strings.Contains(log, "hook settings") {
					t.Fatal(log)
				}
			})
			levelCases(t, "usage-log", func(t *testing.T, f *eventFixture, on, warn, broken bool) {
				event := "PostToolUse"
				if component {
					event = "usage-log"
				}
				_, log, code := f.run(t, event, hookio.Input{HookEventName: "PostToolUse", ToolName: "Agent", ToolUseID: "use-1", ToolResponse: map[string]any{"totalTokens": 12}})
				a, err := (session.Store{TaskDir: f.dir}).Activity("session-demo")
				if err != nil {
					t.Fatal(err)
				}
				if code != 0 || (len(a.Usage) > 0) != on {
					t.Fatalf("usage: %d %+v %q", code, a, log)
				}
				if broken && !strings.Contains(log, "hook settings") {
					t.Fatal(log)
				}
			})
			levelCases(t, "session-end", func(t *testing.T, f *eventFixture, on, warn, broken bool) {
				s, key := f.workerSession(t)
				event := "SessionEnd"
				if component {
					event = "worker-exit"
				}
				_, log, code := f.run(t, event, hookio.Input{HookEventName: "SessionEnd", Reason: "done"})
				a, err := s.Activity(key.SessionID)
				if err != nil {
					t.Fatal(err)
				}
				if code != 0 || (a.ExitReason == "done") != on {
					t.Fatalf("exit: %d %+v %q", code, a, log)
				}
				if broken && !strings.Contains(log, "hook settings") {
					t.Fatal(log)
				}
			})
		})
	}
}

func TestCatalogStopLevels(t *testing.T) {
	for _, kind := range []string{"Stop", "SubagentStop", "stop-check", "verify"} {
		t.Run(kind, func(t *testing.T) {
			levelCases(t, "stop-check", func(t *testing.T, f *eventFixture, on, warn, broken bool) {
				dir := filepath.Join(f.dir, "subagents")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				brief := filepath.Join(dir, "01-build.md")
				if err := os.WriteFile(brief, []byte("---\nrole: builder\nstatus: done\n---\n## Outcome\n"), 0600); err != nil {
					t.Fatal(err)
				}
				event := kind
				in := hookio.Input{HookEventName: "Stop"}
				if kind == "SubagentStop" {
					transcript := filepath.Join(f.home, "transcript.jsonl")
					b, _ := json.Marshal(map[string]any{"message": map[string]any{"content": "Use " + brief}})
					if err := os.WriteFile(transcript, b, 0600); err != nil {
						t.Fatal(err)
					}
					in = hookio.Input{HookEventName: kind, AgentType: "grid:grid-builder", AgentTranscriptPath: transcript}
				}
				if kind == "verify" {
					event = "Stop"
					f.env["GRID_JOB"] = "J-1"
					f.env["PATH"] = os.Getenv("PATH")
					dir := filepath.Join(f.dir, "jobs", "J-1")
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
					b, _ := json.Marshal(job.Job{ID: "J-1", Verify: job.Verify{Command: "exit 1"}})
					if err := os.WriteFile(filepath.Join(dir, "job.yaml"), b, 0600); err != nil {
						t.Fatal(err)
					}
				}
				_, log, code := f.run(t, event, in)
				want := 0
				if on && !warn {
					want = 2
				}
				if code != want {
					t.Fatalf("code %d want %d: %s", code, want, log)
				}
				if on && warn && !strings.Contains(log, "Warning: stop-check:") {
					t.Fatalf("missing warning: %s", log)
				}
				if !on && log != "" {
					t.Fatalf("off ran check: %s", log)
				}
				if broken && !strings.Contains(log, "hook settings") {
					t.Fatal(log)
				}
			})
		})
	}
}

func TestHistoryOffCoversEveryWriter(t *testing.T) {
	for _, component := range []bool{false, true} {
		for _, level := range []string{"off", "warn", "block"} {
			t.Run(level+"/component="+map[bool]string{false: "false", true: "true"}[component], func(t *testing.T) {
				f := eventSetup(t)
				if err := (settings.Store{Home: f.home}).SetHook("history-log", level); err != nil {
					t.Fatal(err)
				}
				store := brain.Store{GridHome: f.home}
				origin := brain.Origin{Tier: contract.PersonTeam, Kind: "session", ID: "session-demo"}
				_, err := store.Propose(brain.Note{Name: "cache", Topic: "decisions", Type: "convention", Description: "Cache configuration", Summary: "Keep configuration explicit.", Source: "task:T-01", Level: "home", Body: "Use explicit configuration.", Origin: origin}, origin)
				if err != nil {
					t.Fatal(err)
				}
				entry, err := store.Accept("home", "cache")
				if err != nil {
					t.Fatal(err)
				}
				run := func(event, alias string, in hookio.Input) string {
					t.Helper()
					in.HookEventName = event
					if component {
						event = alias
					}
					out, log, code := f.run(t, event, in)
					if code != 0 || log != "" {
						t.Fatalf("%s: %d %s", event, code, log)
					}
					return out
				}
				shell := hookio.Input{ToolName: "Bash", ToolUseID: "bash-1", ToolInput: map[string]any{"command": "grid task change"}}
				run("PreToolUse", "history-log", shell)
				if err := os.WriteFile(filepath.Join(f.dir, "notes.md"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
				run("PostToolUseFailure", "history-log", shell)
				run("SubagentStop", "history-log", hookio.Input{})
				run("PostToolUse", "brain-read", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, entry.Path)}})
				out := run("UserPromptSubmit", "brain-recall", hookio.Input{Prompt: "/grid:plan"})
				if !strings.Contains(out, "Brain pointers") {
					t.Fatal(out)
				}
				b, err := os.ReadFile(filepath.Join(f.dir, "history.md"))
				if err != nil {
					t.Fatal(err)
				}
				if level == "off" && len(b) != 0 {
					t.Fatalf("off wrote history: %s", b)
				}
				for _, text := range []string{"file written", "subagent finished", "brain read", "brain recall"} {
					if strings.Contains(string(b), text) != (level != "off") {
						t.Fatalf("%s: %s", text, b)
					}
				}
				a, err := (session.Store{TaskDir: f.dir}).Activity("session-demo")
				if err != nil {
					t.Fatal(err)
				}
				if len(a.Snapshots) != 0 {
					t.Fatalf("retained snapshot: %+v", a)
				}
			})
		}
	}
}

func TestUsageOffSuppressesTranscriptRecords(t *testing.T) {
	for _, event := range []string{"SessionStart", "SubagentStop", "usage-log"} {
		t.Run(event, func(t *testing.T) {
			f := eventSetup(t)
			if err := (settings.Store{Home: f.home}).SetHook("usage-log", "off"); err != nil {
				t.Fatal(err)
			}
			out, log, code := f.run(t, event, hookio.Input{HookEventName: "SubagentStop", TranscriptPath: "transcript.jsonl", AgentTranscriptPath: "agent.jsonl", AgentID: "agent-demo"})
			if code != 0 || log != "" {
				t.Fatalf("%d %s", code, log)
			}
			if event == "SessionStart" && !strings.Contains(out, "Grid tracks tasks") {
				t.Fatal(out)
			}
			a, err := (session.Store{TaskDir: f.dir}).Activity("session-demo")
			if err != nil {
				t.Fatal(err)
			}
			if a.Transcript != "" || len(a.Transcripts) > 0 {
				t.Fatalf("usage off recorded transcript: %+v", a)
			}
		})
	}
}

func TestContextComponentLevels(t *testing.T) {
	for _, tc := range []struct {
		name, event string
		in          hookio.Input
	}{
		{"user-prompt", "UserPromptSubmit", hookio.Input{Prompt: "/grid:plan"}},
		{"subagent-start", "SubagentStart", hookio.Input{AgentType: "grid:grid-builder"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := eventSetup(t)
			if err := (settings.Store{Home: f.home}).SetHook(tc.name, "off"); err != nil {
				t.Fatal(err)
			}
			tc.in.HookEventName = tc.event
			out, log, code := f.run(t, "brain-recall", tc.in)
			if out != "" || log != "" || code != 0 {
				t.Fatalf("off: %d %s %s", code, out, log)
			}
			if err := (settings.Store{Home: f.home}).SetHook(tc.name, "warn"); err != nil {
				t.Fatal(err)
			}
			out, log, code = f.run(t, "brain-recall", tc.in)
			if !strings.Contains(out, "Brain recall") || log != "" || code != 0 {
				t.Fatalf("warn: %d %s %s", code, out, log)
			}
		})
	}
}
