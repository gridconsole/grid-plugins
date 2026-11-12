package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// A reviewer or checker whose Job is a pointer to one brief file ("Your job is the
// file <path>.md") is not warned about a target that file names: the fence follows
// that one file, one hop, and reads it as part of the Job. A folder the file names
// with a trailing slash covers the files directly in it. Every failure to read the
// file adds nothing, so the answer is the one without the hop. Each case that adds
// nothing sits beside the same case with a readable file, so it cannot pass by
// accident.

// hopTarget is the file the reviewer edits; it sits outside the projects root.
func hopTarget(f *fixture) string { return filepath.Join(f.repo, "docs", "plan.md") }

// hopBrief is the one file the Job names.
func hopBrief(f *fixture) string { return filepath.Join(f.task, "review-chci.md") }

// hopJob is the Job text: a pointer to the brief file.
func hopJob(brief string) string { return "Your job is the file " + brief }

// hopPad grows text to exactly n bytes, the padding on a line of its own.
func hopPad(text string, n int) string {
	return text + "\n" + strings.Repeat("x", n-len(text)-1)
}

// hopRun builds one Edit by agent of hopTarget: prep makes the files on disk and
// returns the Job text, the first message of the agent's transcript.
func hopRun(agent string, prep func(t *testing.T, f *fixture, target string) string) func(t *testing.T, f *fixture) (hookio.Input, string) {
	return func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := hopTarget(f)
		job := prep(t, f, target)
		in := f.input("Edit", target)
		in.AgentType = agent
		in.TranscriptPath = jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), jpLine(t, "user", job))
		return in, target
	}
}

type hopCase struct {
	name  string
	prep  func(t *testing.T, f *fixture, target string) string
	fires bool
}

// hopTable runs each case as the reviewer and as the checker, at the default level
// (a warning) and at block (the deny).
func hopTable(t *testing.T, cases []hopCase) {
	t.Helper()
	for _, agent := range []string{"grid:grid-reviewer", "grid:grid-checker"} {
		name := agent[strings.Index(agent, ":")+1:]
		for _, tc := range cases {
			jpExpect(t, name+"/"+tc.name, hopRun(agent, tc.prep), name, tc.fires)
		}
	}
}

// hopNames writes a brief that names target and returns the Job that points at it.
func hopNames(f *fixture, target string) string {
	brief := hopBrief(f)
	f.write(brief, "# Review\n\nEdit "+target+" as the review asks.\n")
	return hopJob(brief)
}

// A1: the file the Job names names the target.
func TestHopNamedFileNamesTarget(t *testing.T) {
	hopTable(t, []hopCase{
		{"brief names the target", func(t *testing.T, f *fixture, target string) string {
			return hopNames(f, target)
		}, false},
		{"brief ends the sentence on the Job", func(t *testing.T, f *fixture, target string) string {
			brief := hopBrief(f)
			f.write(brief, "Edit "+target+".\n")
			return "Your job is the file `" + brief + "`."
		}, false},
		{"brief names the target in backticks", func(t *testing.T, f *fixture, target string) string {
			brief := hopBrief(f)
			f.write(brief, "Edit `"+target+"`\n")
			return hopJob(brief)
		}, false},
		{"verdict path first then the brief", func(t *testing.T, f *fixture, target string) string {
			brief := hopBrief(f)
			f.write(brief, "Edit "+target+"\n")
			return "Write your verdict to " + filepath.Join(f.task, "verdict.md") + ". " + hopJob(brief)
		}, false},
		{"brief names another file", func(t *testing.T, f *fixture, target string) string {
			brief := hopBrief(f)
			f.write(brief, "Edit "+filepath.Join(f.repo, "docs", "other.md")+"\n")
			return hopJob(brief)
		}, true},
		{"brief names a longer path", func(t *testing.T, f *fixture, target string) string {
			brief := hopBrief(f)
			f.write(brief, "Edit "+target+".bak\n")
			return hopJob(brief)
		}, true},
		{"brief names the file name in another folder", func(t *testing.T, f *fixture, target string) string {
			brief := hopBrief(f)
			f.write(brief, "Edit "+filepath.Join(f.root, "elsewhere", "docs", "plan.md")+"\n")
			return hopJob(brief)
		}, true},
		{"Job names no file", func(t *testing.T, f *fixture, target string) string {
			hopNames(f, target)
			return "Review the lane and write your verdict."
		}, true},
		{"Job names the file as a relative path", func(t *testing.T, f *fixture, target string) string {
			hopNames(f, target)
			return "Your job is the file review-chci.md"
		}, true},
		{"Job names a file with a longer name", func(t *testing.T, f *fixture, target string) string {
			hopNames(f, target)
			return hopJob(hopBrief(f) + "x")
		}, true},
		{"Job names the file in a longer path", func(t *testing.T, f *fixture, target string) string {
			hopNames(f, target)
			return hopJob("zz" + hopBrief(f))
		}, true},
	})
}

// The hop reads the Job's file; it does not widen the other fences. A brain file
// the brief names is still a brain file.
func TestHopKeepsOtherFences(t *testing.T) {
	f := setup(t)
	note := filepath.Join(f.grid, "knowledge", "note.md")
	brief := hopBrief(f)
	f.write(brief, "Edit "+note+"\n")
	in := f.input("Edit", note)
	in.AgentType = "grid:grid-reviewer"
	in.TranscriptPath = jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), jpLine(t, "user", hopJob(brief)))
	want := "plan-guard: Only a grid-curator (or a grid-planner) writes the brain; a grid-reviewer writes a proposal in its task's brain-proposals/ folder instead of " + note + "." + off
	if d := EvaluatePlan(in, f.root, f.env); d.Behavior != "" || len(d.Warnings) != 1 || d.Warnings[0] != want {
		t.Fatalf("want the brain warning %q, got %+v", want, d)
	}
}

// A2: a folder the file names with a trailing slash covers the files directly in
// it, not the ones in a subfolder, and not when the slash is missing. The Job's
// own text naming a folder still covers nothing (TestPlanGuardJobNamedPathMatchesWholePaths).
func TestHopNamedFileNamesFolder(t *testing.T) {
	folder := func(f *fixture) string { return filepath.Join(f.repo, "docs") }
	hopTable(t, []hopCase{
		{"folder with a slash and the target directly in it", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "Edit the files in "+folder(f)+"/ as the review asks.\n")
			return hopJob(hopBrief(f))
		}, false},
		{"folder with a slash at the end of a line", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "Files you may edit: "+folder(f)+"/\n")
			return hopJob(hopBrief(f))
		}, false},
		{"folder with a slash in a list", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "- `"+filepath.Join(f.repo, "other")+"/`\n- `"+folder(f)+"/`\n")
			return hopJob(hopBrief(f))
		}, false},
		{"folder without the slash", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "Edit the files in "+folder(f)+" as the review asks.\n")
			return hopJob(hopBrief(f))
		}, true},
		{"parent folder with a slash and the target in a subfolder of it", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "Edit the files in "+f.repo+"/ as the review asks.\n")
			return hopJob(hopBrief(f))
		}, true},
		{"a subfolder and the folder both named", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "Edit the files in "+filepath.Join(f.repo, "docs", "sub")+"/ and "+folder(f)+"/ as the review asks.\n")
			return hopJob(hopBrief(f))
		}, false},
		{"only a subfolder named", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "Edit the files in "+filepath.Join(f.repo, "docs", "sub")+"/ as the review asks.\n")
			return hopJob(hopBrief(f))
		}, true},
		{"a folder with a longer name", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "Edit the files in "+folder(f)+"x/ as the review asks.\n")
			return hopJob(hopBrief(f))
		}, true},
	})
	// A lone slash in prose names the root folder of no one: a file in the root
	// still gets the answer.
	jpExpect(t, "a lone slash names no folder", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(filepath.VolumeName(f.root)+string(filepath.Separator), "hop-plan.md")
		f.write(hopBrief(f), "Approve / reject, and edit the files in / as you like.\n")
		in := f.input("Edit", target)
		in.AgentType = "grid:grid-reviewer"
		in.TranscriptPath = jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), jpLine(t, "user", hopJob(hopBrief(f))))
		return in, target
	}, "grid-reviewer", true)
	// The same folder spelled in the Job's own text, not in the file, covers nothing.
	hopTable(t, []hopCase{
		{"Job names the folder with a slash", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), "No paths here.\n")
			return hopJob(hopBrief(f)) + " and edit the files in " + folder(f) + "/"
		}, true},
	})
	// A target in a subfolder of the named one, one hop deeper, with the subfolder
	// named by nothing: the answer stays.
	jpExpect(t, "target in a subfolder", func(t *testing.T, f *fixture) (hookio.Input, string) {
		target := filepath.Join(f.repo, "docs", "sub", "plan.md")
		f.write(hopBrief(f), "Edit the files in "+folder(f)+"/\n")
		in := f.input("Edit", target)
		in.AgentType = "grid:grid-reviewer"
		in.TranscriptPath = jpWrite(t, filepath.Join(f.root, "tr", "main.jsonl"), jpLine(t, "user", hopJob(hopBrief(f))))
		return in, target
	}, "grid-reviewer", true)
}

// A3: one hop only, and a file that cannot be read adds nothing. Each negative
// sits beside the readable file that is silent.
func TestHopOneHopAndUnreadableFiles(t *testing.T) {
	// body is a brief that names the target, grown to n bytes.
	body := func(target string, n int) string { return hopPad("Edit "+target+" as the review asks.", n) }
	hopTable(t, []hopCase{
		// One hop: the second file names the target, the first names the second.
		{"second file names the target", func(t *testing.T, f *fixture, target string) string {
			second := filepath.Join(f.task, "second.md")
			f.write(second, "Edit "+target+"\n")
			f.write(hopBrief(f), "Read "+second+" next.\n")
			return hopJob(hopBrief(f))
		}, true},
		{"the first file names the target and the second", func(t *testing.T, f *fixture, target string) string {
			second := filepath.Join(f.task, "second.md")
			f.write(second, "No paths.\n")
			f.write(hopBrief(f), "Read "+second+" next, and edit "+target+".\n")
			return hopJob(hopBrief(f))
		}, false},
		// A missing file, then the same file once it exists.
		{"file missing", func(t *testing.T, f *fixture, target string) string {
			return hopJob(hopBrief(f))
		}, true},
		{"file there", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), body(target, 200))
			return hopJob(hopBrief(f))
		}, false},
		// A symlink to a file that names the target, then the file itself.
		{"file is a symlink", func(t *testing.T, f *fixture, target string) string {
			real := filepath.Join(f.root, "real-brief.md")
			f.write(real, body(target, 200))
			f.link(hopBrief(f), real)
			return hopJob(hopBrief(f))
		}, true},
		{"file is the file a symlink points at", func(t *testing.T, f *fixture, target string) string {
			real := filepath.Join(f.root, "real-brief.md")
			f.write(real, body(target, 200))
			return hopJob(real)
		}, false},
		// A folder with the file's name.
		{"file is a folder", func(t *testing.T, f *fixture, target string) string {
			f.write(filepath.Join(hopBrief(f), "inner.md"), body(target, 200))
			return hopJob(hopBrief(f))
		}, true},
		// The size line is 64 KiB: under it the file is read, at it and over it is not.
		{"file one byte under 64 KiB", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), body(target, jobTextLimit-1))
			return hopJob(hopBrief(f))
		}, false},
		{"file of 64 KiB", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), body(target, jobTextLimit))
			return hopJob(hopBrief(f))
		}, true},
		{"file one byte over 64 KiB", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), body(target, jobTextLimit+1))
			return hopJob(hopBrief(f))
		}, true},
		{"file far over 64 KiB", func(t *testing.T, f *fixture, target string) string {
			f.write(hopBrief(f), body(target, 4*jobTextLimit))
			return hopJob(hopBrief(f))
		}, true},
	})
}
