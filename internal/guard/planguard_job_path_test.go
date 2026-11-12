package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// A reviewer or checker is not warned about a file its own Job names: the fence
// reads the first user message of the caller's transcript and stays silent for a
// target that message spells out. Every other target outside the projects root
// answers as before: a warning by default, the deny at block.

// jpLine is one transcript row: a user message whose content is text.
func jpLine(t *testing.T, role, text string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"type": role, "message": map[string]any{"role": role, "content": text}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// jpWrite writes a transcript file and returns its path.
func jpWrite(t *testing.T, path string, lines ...string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// jpFence is the reviewer or checker fence text for target.
func jpFence(agent, target string) string {
	return "A " + agent + " writes only its Verdict inside a task folder; " + target + " is outside it." + off
}

// jpExpect asserts the fence answer for one write at both levels: run builds the
// call and names its target; fires says the fence answers (a warning by default,
// the deny at block) or stays silent.
func jpExpect(t *testing.T, name string, run func(t *testing.T, f *fixture) (hookio.Input, string), agent string, fires bool) {
	t.Helper()
	for _, level := range []string{"default", "block"} {
		t.Run(name+"/"+level, func(t *testing.T) {
			f := setup(t)
			if level == "block" {
				planBlock(f)
			}
			in, target := run(t, f)
			d := EvaluatePlan(in, f.root, f.env)
			if !fires {
				if d.Behavior != "" || len(d.Warnings) != 0 || d.Reason != "" {
					t.Fatalf("%s at %s: want no warning and no deny, got %+v", name, level, d)
				}
				return
			}
			want := jpFence(agent, target)
			if level == "block" {
				if d.Behavior != hookio.Deny("").Behavior || d.Reason != want || len(d.Warnings) != 0 {
					t.Fatalf("%s at block: want deny %q, got %+v", name, want, d)
				}
				return
			}
			if d.Behavior != "" || len(d.Warnings) != 1 || d.Warnings[0] != "plan-guard: "+want {
				t.Fatalf("%s by default: want the warning %q, got %+v", name, "plan-guard: "+want, d)
			}
		})
	}
}

// jpInput builds a Write by agent to path, with the caller's transcript.
func jpInput(f *fixture, agent, path, transcript string) hookio.Input {
	in := f.input("Write", path)
	in.AgentType = agent
	in.TranscriptPath = transcript
	return in
}

func TestPlanGuardJobNamedPath(t *testing.T) {
	for _, agent := range []string{"grid:grid-reviewer", "grid:grid-checker"} {
		name := agent[strings.Index(agent, ":")+1:]
		t.Run(name, func(t *testing.T) {
			// Row 1: the Job names the file; the write is silent at every level.
			jpExpect(t, "named", func(t *testing.T, f *fixture) (hookio.Input, string) {
				verdict := filepath.Join(f.repo, "verdict.md")
				tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"),
					jpLine(t, "user", "Review the lane. Write your verdict to "+verdict+" and nothing else."))
				return jpInput(f, agent, verdict, tr), verdict
			}, name, false)
			// Row 2: another file outside the projects root is fenced as before.
			jpExpect(t, "not named", func(t *testing.T, f *fixture) (hookio.Input, string) {
				verdict, other := filepath.Join(f.repo, "verdict.md"), filepath.Join(f.repo, "other.md")
				tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"),
					jpLine(t, "user", "Write your verdict to "+verdict+"."))
				return jpInput(f, agent, other, tr), other
			}, name, true)
			// Row 3: no transcript, a missing one, or one that is not a file.
			jpExpect(t, "no transcript path", func(t *testing.T, f *fixture) (hookio.Input, string) {
				target := filepath.Join(f.repo, "verdict.md")
				return jpInput(f, agent, target, ""), target
			}, name, true)
			jpExpect(t, "transcript missing", func(t *testing.T, f *fixture) (hookio.Input, string) {
				target := filepath.Join(f.repo, "verdict.md")
				return jpInput(f, agent, target, filepath.Join(f.root, "tr", "gone.jsonl")), target
			}, name, true)
			jpExpect(t, "transcript is a folder", func(t *testing.T, f *fixture) (hookio.Input, string) {
				target := filepath.Join(f.repo, "verdict.md")
				dir := filepath.Join(f.root, "tr")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				return jpInput(f, agent, target, dir), target
			}, name, true)
			jpExpect(t, "transcript is not JSON", func(t *testing.T, f *fixture) (hookio.Input, string) {
				target := filepath.Join(f.repo, "verdict.md")
				tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), "write "+target+" now")
				return jpInput(f, agent, target, tr), target
			}, name, true)
		})
	}
}

// The Job's text is the first user message only, read from the start of the file
// and no further than 64 KiB; a path spelled anywhere else is not named by the Job.
func TestPlanGuardJobNamedPathReadsFirstMessageOnly(t *testing.T) {
	const agent = "grid:grid-reviewer"
	jpExpect(t, "named by a later message", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"),
			jpLine(t, "user", "Review the lane."),
			jpLine(t, "assistant", "I will write "+target),
			jpLine(t, "user", "Write "+target))
		return jpInput(f, agent, target, tr), target
	}, "grid-reviewer", true)
	jpExpect(t, "named after 64 KiB", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"),
			jpLine(t, "user", strings.Repeat("word ", 14000)+target))
		return jpInput(f, agent, target, tr), target
	}, "grid-reviewer", true)
	jpExpect(t, "named in content blocks", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		b, err := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": "Review the lane."},
			map[string]any{"type": "text", "text": "Verdict file: `" + target + "`."},
		}}})
		if err != nil {
			t.Fatal(err)
		}
		tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), string(b))
		return jpInput(f, agent, target, tr), target
	}, "grid-reviewer", false)
	jpExpect(t, "named after a row that is no message", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"),
			`{"type":"summary","summary":"x"}`,
			jpLine(t, "user", "Write "+target))
		return jpInput(f, agent, target, tr), target
	}, "grid-reviewer", false)
}

// A path counts as named when the Job spells it as a whole path, the way the call
// gives it or as the absolute path it resolves to; a longer path that holds it is
// not the same file.
func TestPlanGuardJobNamedPathMatchesWholePaths(t *testing.T) {
	const agent = "grid:grid-reviewer"
	for _, tc := range []struct {
		name  string
		job   func(f *fixture) string
		given func(f *fixture) string
		fires bool
	}{
		{"absolute, sentence end", func(f *fixture) string { return "Verdict: " + filepath.Join(f.repo, "v.md") + "." },
			func(f *fixture) string { return filepath.Join(f.repo, "v.md") }, false},
		{"absolute, in parentheses", func(f *fixture) string { return "(" + filepath.Join(f.repo, "v.md") + ") is yours" },
			func(f *fixture) string { return filepath.Join(f.repo, "v.md") }, false},
		{"relative call, absolute Job", func(f *fixture) string { return "Write " + filepath.Join(f.repo, "v.md") },
			func(f *fixture) string { return "v.md" }, false},
		{"relative as given", func(f *fixture) string { return "Write v.md next to the code" },
			func(f *fixture) string { return "v.md" }, false},
		{"Job names a longer path", func(f *fixture) string { return "Write " + filepath.Join(f.repo, "v.md.bak") },
			func(f *fixture) string { return filepath.Join(f.repo, "v.md") }, true},
		{"Job names a longer file name", func(f *fixture) string { return "Write " + filepath.Join(f.repo, "v.mdx") },
			func(f *fixture) string { return filepath.Join(f.repo, "v.md") }, true},
		{"Job names a folder", func(f *fixture) string { return "Write inside " + f.repo + "/" },
			func(f *fixture) string { return filepath.Join(f.repo, "v.md") }, true},
		{"Job names the same name in another folder", func(f *fixture) string { return "Write " + filepath.Join(f.root, "elsewhere", "v.md") },
			func(f *fixture) string { return "v.md" }, true},
		{"Job names a longer path that ends in it", func(f *fixture) string { return "Write zz" + filepath.Join(f.repo, "v.md") },
			func(f *fixture) string { return filepath.Join(f.repo, "v.md") }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			given := tc.given(f)
			tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), jpLine(t, "user", tc.job(f)))
			in := jpInput(f, agent, given, tr)
			in.Cwd = f.repo
			d := EvaluatePlan(in, f.root, f.env)
			if d.Behavior != "" {
				t.Fatalf("default level refused the write: %+v", d)
			}
			if fired := len(d.Warnings) == 1; fired != tc.fires {
				t.Fatalf("fires=%v, want %v: %+v", fired, tc.fires, d)
			}
		})
	}
}

// A subagent's Job is the first message of its own transcript. The hook names it
// (agent_transcript_path), or it sits next to the session's transcript as
// subagents/agent-<id>.jsonl; the session's first message is a person's prompt,
// never the subagent's Job.
func TestPlanGuardJobNamedPathSubagent(t *testing.T) {
	const agent = "grid:grid-reviewer"
	session := func(f *fixture) string { return filepath.Join(f.root, "tr", "session.jsonl") }
	sub := func(f *fixture, id string) string {
		return filepath.Join(f.root, "tr", "session", "subagents", "agent-"+id+".jsonl")
	}
	jpExpect(t, "subagent transcript beside the session names it", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		jpWrite(t, session(f), jpLine(t, "user", "Do the work."))
		jpWrite(t, sub(f, "a1"), jpLine(t, "user", "Write your verdict to "+target))
		in := jpInput(f, agent, target, session(f))
		in.AgentID = "a1"
		return in, target
	}, "grid-reviewer", false)
	jpExpect(t, "only the session's first message names it", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		jpWrite(t, session(f), jpLine(t, "user", "Edit "+target))
		jpWrite(t, sub(f, "a1"), jpLine(t, "user", "Review the lane."))
		in := jpInput(f, agent, target, session(f))
		in.AgentID = "a1"
		return in, target
	}, "grid-reviewer", true)
	jpExpect(t, "subagent transcript missing, session names it", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		jpWrite(t, session(f), jpLine(t, "user", "Edit "+target))
		in := jpInput(f, agent, target, session(f))
		in.AgentID = "a1"
		return in, target
	}, "grid-reviewer", true)
	jpExpect(t, "another subagent's transcript names it", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		jpWrite(t, session(f), jpLine(t, "user", "Do the work."))
		jpWrite(t, sub(f, "a2"), jpLine(t, "user", "Write "+target))
		in := jpInput(f, agent, target, session(f))
		in.AgentID = "a1"
		return in, target
	}, "grid-reviewer", true)
	jpExpect(t, "agent id with a separator", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		jpWrite(t, session(f), jpLine(t, "user", "Do the work."))
		// Joined and cleaned, this id would read tr/session/x.jsonl.
		jpWrite(t, filepath.Join(f.root, "tr", "session", "x.jsonl"), jpLine(t, "user", "Write "+target))
		in := jpInput(f, agent, target, session(f))
		in.AgentID = "x/../../x"
		return in, target
	}, "grid-reviewer", true)
	jpExpect(t, "hook names the agent transcript", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		jpWrite(t, session(f), jpLine(t, "user", "Do the work."))
		in := jpInput(f, agent, target, session(f))
		in.AgentID = "a1"
		in.AgentTranscriptPath = jpWrite(t, filepath.Join(f.root, "tr", "own.jsonl"), jpLine(t, "user", "Write "+target))
		return in, target
	}, "grid-reviewer", false)
	jpExpect(t, "transcript path is the subagent's own", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "verdict.md")
		in := jpInput(f, agent, target, jpWrite(t, sub(f, "a1"), jpLine(t, "user", "Write "+target)))
		in.AgentID = "a1"
		return in, target
	}, "grid-reviewer", false)
}

// The Job silences the reviewer and checker fence only. A brain file the Job names
// is still a brain file: the brain fence answers for a reviewer as before.
func TestPlanGuardJobNamedPathKeepsOtherFences(t *testing.T) {
	f := setup(t)
	note := filepath.Join(f.grid, "knowledge", "note.md")
	tr := jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), jpLine(t, "user", "Write "+note))
	d := EvaluatePlan(jpInput(f, "grid:grid-reviewer", note, tr), f.root, f.env)
	want := "plan-guard: Only a grid-curator (or a grid-planner) writes the brain; a grid-reviewer writes a proposal in its task's brain-proposals/ folder instead of " + note + "." + off
	if d.Behavior != "" || len(d.Warnings) != 1 || d.Warnings[0] != want {
		t.Fatalf("want the brain warning %q, got %+v", want, d)
	}
	// A builder is not fenced by this rule at all, named or not.
	other := filepath.Join(f.repo, "file.go")
	in := jpInput(f, "grid:grid-builder", other, tr)
	if d := EvaluatePlan(in, f.root, f.env); d.Behavior != "" || len(d.Warnings) != 0 {
		t.Fatalf("builder write: want no answer, got %+v", d)
	}
}
