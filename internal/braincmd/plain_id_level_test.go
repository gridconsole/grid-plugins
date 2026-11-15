package braincmd

import (
	"os"
	"strings"
	"testing"
)

func plainIDBrain(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.put(t, "projects/example/project.md", "---\nid: 01EXAMPLE\n---\n")
	for _, p := range []struct{ level, name string }{{"project", "plain-p"}, {"home", "plain-h"}} {
		args := []string{"propose", "--level", p.level, "--name", p.name, "--topic", "decisions",
			"--description", "Before changing checks.", "--summary", "Run the checks.", "--source", "job:T-01/J-01", "--body", "# " + p.name + "\nRun make test.\n"}
		if p.level == "project" {
			args = append(args, "--project", "example")
		}
		if code := h.run(args...); code != 0 {
			t.Fatalf("propose %s: %d %s", p.name, code, &h.err)
		}
	}
	return h
}

// A plain id with --project finds a project proposal and not a home one; the
// full brain:<level>/<id> finds both. skills/brain-keeper says so.
func TestPlainIDFindsOnlyAProjectProposalWithProject(t *testing.T) {
	h := plainIDBrain(t)
	if code := h.run("check", "plain-p", "--project", "example"); code != 0 {
		t.Fatalf("check plain project id: %d %s", code, &h.err)
	}
	if code := h.run("show", "plain-p", "--proposal", "--project", "example"); code != 0 {
		t.Fatalf("show plain project id: %d %s", code, &h.err)
	}
	if code := h.run("show", "plain-h", "--proposal", "--project", "example"); code == 0 {
		t.Fatalf("a plain id found a home proposal with --project:\n%s", &h.out)
	}
	if code := h.run("accept", "plain-h", "--project", "example"); code == 0 {
		t.Fatal("accept with a plain id filed a home proposal under --project")
	}
	if code := h.run("accept", "brain:home/plain-h", "--project", "example"); code != 0 {
		t.Fatalf("accept brain:home/plain-h: %d %s", code, &h.err)
	}
	if code := h.run("accept", "plain-p", "--project", "example"); code != 0 {
		t.Fatalf("accept plain project id: %d %s", code, &h.err)
	}
}

func TestBrainKeeperSaysWhenThePlainIDIsEnough(t *testing.T) {
	data, err := os.ReadFile("../../skills/brain-keeper/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "grid brain accept --project <project> <id>") {
			if !strings.Contains(line, "brain:home/<id>") {
				t.Errorf("the plain-id accept line does not say a home proposal needs brain:home/<id>:\n%s", line)
			}
			return
		}
	}
	t.Fatal("no line shows grid brain accept --project <project> <id>")
}
