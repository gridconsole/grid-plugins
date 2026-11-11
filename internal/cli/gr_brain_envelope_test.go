package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// grBrainHarness is a harness whose four homes are folders of its temp root, with
// one registered project, "example", whose checkout is the returned cwd.
func grBrainHarness(t *testing.T, getenv map[string]string) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	for key, dir := range map[string]string{"HOME": h.home, "GRID_HOME": h.gridHome,
		"CLAUDE_CONFIG_DIR": filepath.Join(h.root, "claude"), "CODEX_HOME": filepath.Join(h.root, "codex")} {
		h.mkdir(dir)
		t.Setenv(key, dir)
	}
	h.env.Getenv = func(key string) string { return getenv[key] }
	cwd := filepath.Join(h.root, "checkout")
	h.mkdir(cwd)
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
	return h, cwd
}

// grBrainRun runs one grid call and returns its exit, stdout and stderr.
func grBrainRun(h *harness, cwd string, args ...string) (int, string, string) {
	h.env.Stdin = strings.NewReader("")
	exit := h.run(cwd, args...)
	return exit, h.out.String(), h.errb.String()
}

// grBrainEnvelope runs args plain and with --json and requires the same exit and
// stderr line, the line `want`, and the standard envelope on stdout only with
// --json.
func grBrainEnvelope(t *testing.T, h *harness, cwd string, args []string, want string) {
	t.Helper()
	exit, out, stderr := grBrainRun(h, cwd, args...)
	if exit != 1 || out != "" || stderr != want {
		t.Fatalf("plain %v: exit=%d stdout=%q stderr=%q, want exit 1, no stdout, stderr %q", args, exit, out, stderr, want)
	}
	jsonExit, jsonOut, jsonErr := grBrainRun(h, cwd, append(append([]string{}, args...), "--json")...)
	if jsonExit != 1 || jsonErr != want {
		t.Fatalf("--json %v: exit=%d stderr=%q, want exit 1, stderr %q", args, jsonExit, jsonErr, want)
	}
	assertErrorOutput(t, jsonOut, jsonErr, CodeNoSuchProject)
}

// grBrainMembers holds one valid call of each brain verb that takes --project;
// every one reaches the project lookup before anything else can fail.
var grBrainMembers = map[string][]string{
	"show":    {"brain", "show", "note"},
	"revert":  {"brain", "revert", "conventions/note", "--to", "1"},
	"propose": {"brain", "propose", "--name", "note", "--topic", "conventions", "--description", "d", "--summary", "s", "--source", "src", "--body", "b"},
	"search":  {"brain", "search", "words"},
	"topics":  {"brain", "topics"},
	"changes": {"brain", "changes"},
}

func TestGrBrainEnvelopeUnknownProject(t *testing.T) {
	for name, argv := range grBrainMembers {
		t.Run(name, func(t *testing.T) {
			h, cwd := grBrainHarness(t, nil)
			grBrainEnvelope(t, h, cwd, append(append([]string{}, argv...), "--project", "absent"),
				"brain: project \"absent\" is not registered\n")
		})
	}
}

// A guest sees only the projects its project-roles.json names. A registered
// project it is not granted answers like one that does not exist: the same
// envelope, and the same plain line apart from the name it was asked for.
// revert and changes refuse a guest's role before they look at a project, so
// they answer a hidden and an absent project alike with that refusal.
func TestGrBrainEnvelopeGuestHiddenProject(t *testing.T) {
	for name, argv := range grBrainMembers {
		t.Run(name, func(t *testing.T) {
			h, cwd := grBrainHarness(t, map[string]string{"GRID_ROLE": "guest", "GRID_MEMBER": "gus"})
			h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
			h.record(project.Record{Name: "beta", ID: "01BETA"})
			h.write(filepath.Join(h.gridHome, "projects", "alpha", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
			for _, asked := range []string{"beta", "absent"} {
				args := append(append([]string{}, argv...), "--project", asked)
				if name == "revert" || name == "changes" {
					exit, out, stderr := grBrainRun(h, cwd, append(args, "--json")...)
					if exit != 3 || out != `{"error":"person_required","reason":"role_not_allowed","hint":"`+strings.TrimPrefix(strings.TrimSuffix(stderr, "\n"), "grid brain "+name+": ")+`","code":"person_required","message":"`+strings.TrimSuffix(stderr, "\n")+"\"}\n" || !strings.Contains(stderr, "caller role is not allowed") {
						t.Fatalf("%s --project %s: exit=%d stdout=%q stderr=%q, want the role refusal, exit 3", name, asked, exit, out, stderr)
					}
					continue
				}
				grBrainEnvelope(t, h, cwd, args, "brain: project \""+asked+"\" is not registered\n")
			}
		})
	}
}

// A brain failure that wraps no where sentinel keeps today's answer: the JSON
// object on stderr, nothing on stdout. (A malformed revert id is a usage error,
// exit 2 with a plain line, with or without --json; it never reached this path.)
func TestGrBrainEnvelopeOtherFailureKeepsJSONObject(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"revert-no-such-version", []string{"brain", "revert", "conventions/missing", "--to", "9", "--level", "home"}},
		{"show-no-such-note", []string{"brain", "show", "missing", "--level", "home"}},
		{"accept-no-such-proposal", []string{"brain", "accept", "missing", "--level", "home"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cwd := grBrainHarness(t, nil)
			plainExit, plainOut, plainErr := grBrainRun(h, cwd, tc.args...)
			if plainExit != 1 || plainOut != "" || !strings.HasPrefix(plainErr, "brain: ") {
				t.Fatalf("plain: exit=%d stdout=%q stderr=%q", plainExit, plainOut, plainErr)
			}
			exit, out, stderr := grBrainRun(h, cwd, append(append([]string{}, tc.args...), "--json")...)
			if exit != 1 || out != "" {
				t.Fatalf("--json: exit=%d stdout=%q stderr=%q, want exit 1 and no stdout", exit, out, stderr)
			}
			var object map[string]string
			if err := json.Unmarshal([]byte(stderr), &object); err != nil || len(object) != 1 || object["error"] != strings.TrimSuffix(strings.TrimPrefix(plainErr, "brain: "), "\n") {
				t.Fatalf("--json stderr = %q, want {\"error\":%q}", stderr, strings.TrimSuffix(strings.TrimPrefix(plainErr, "brain: "), "\n"))
			}
		})
	}
}
