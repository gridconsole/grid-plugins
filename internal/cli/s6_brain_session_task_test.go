package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// s6Note is the round-8 project note brain:project/command-blocks as it was filed.
const s6Note = `---
name: command-blocks
topic: conventions
type: convention
description: How shell commands are written in the handbook.
summary: Shell commands in the handbook are written in ` + "`sh`" + ` code fences, one command per line.
updated: '2026-11-15T01:42:14.040Z'
source: person:owner
superseded_by: ''
links: []
level: project
status: accepted
reason: ''
origin:
  kind: session
  id: d183b075b0c9c1dacc4e8567
---

Shell commands in the handbook are written in ` + "`sh`" + ` code fences, one command per line.
`

// Round 8, T-01 J-1: the Job session ran from a worktree inside the Grid home with
// GRID_TASK set, and searched with no flags. The hook recorded a brain read of the
// project note for those words (history.md:3); the CLI must show the same note.
func TestS6BrainSearchUsesSessionTaskProject(t *testing.T) {
	for _, name := range []string{"GRID_TASK", "GRID_FEED_TASK"} {
		t.Run(name, func(t *testing.T) {
			h, task := feedHarness(t)
			t.Setenv("GRID_TASK", "")
			t.Setenv("GRID_FEED_TASK", "")
			h.write(filepath.Join(h.gridHome, "projects", "example", "project.md"), "---\nid: id-example\n---\n")
			h.write(filepath.Join(h.gridHome, "projects", "example", "knowledge", "conventions", "command-blocks.md"), s6Note)
			worktree := filepath.Join(h.gridHome, "worktrees", "01M7EDXBM4NR2GFFBAFPEXWNM1-J-1-a1")
			h.mkdir(worktree)
			// With the project named, a miss is the fixture's fault, not the scope's.
			code := h.run(worktree, "brain", "search", "--project", "example", "install page handbook links commands")
			if code != 0 || !strings.Contains(h.out.String(), "command-blocks") {
				t.Fatalf("brain search --project example: exit %d out %q err %q", code, h.out, h.errb)
			}
			t.Setenv(name, task)
			code = h.run(worktree, "brain", "search", "install page handbook links commands")
			if code != 0 || !strings.Contains(h.out.String(), "command-blocks") {
				t.Fatalf("brain search from the Job worktree with %s: exit %d out %q err %q", name, code, h.out, h.errb)
			}
			code = h.run(worktree, "brain", "show", "command-blocks")
			if code != 0 || !strings.Contains(h.out.String(), "sh") {
				t.Fatalf("brain show from the Job worktree with %s: exit %d out %q err %q", name, code, h.out, h.errb)
			}
		})
	}
}
