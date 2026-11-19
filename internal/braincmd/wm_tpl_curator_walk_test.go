package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// The grid-curator template is synced into the user's agents. This walk runs its
// command lines on the fixture TestQfCuratorWalk uses, so a line that stops
// working fails here.

var wmTplSpan = regexp.MustCompile("`(grid brain [^`\n]+)`")

// wmTplCurator holds the template's lines by what each one does. A line that
// only searches, lists topics, reads the index or proposes is not run.
type wmTplCurator struct {
	list                               string
	taskRead, taskFile                 []string
	levelRead, levelCheck, levelAccept []string
	levelReject                        []string
}

func wmTplClassify(t *testing.T) wmTplCurator {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "agent-templates", "grid-curator.md"))
	if err != nil {
		t.Fatal(err)
	}
	var c wmTplCurator
	for _, m := range wmTplSpan.FindAllStringSubmatch(string(b), -1) {
		l := m[1]
		switch {
		case strings.HasPrefix(l, "grid brain keep "):
			c.list = l
		case strings.HasPrefix(l, "grid brain show ") && strings.Contains(l, "--task <task>"):
			c.taskRead = append(c.taskRead, l)
		case strings.HasPrefix(l, "grid brain decide "):
			c.taskFile = append(c.taskFile, l)
		case strings.HasPrefix(l, "grid brain show ") && strings.Contains(l, "brain:<level>/<id>"):
			c.levelRead = append(c.levelRead, l)
		case strings.HasPrefix(l, "grid brain check "):
			c.levelCheck = append(c.levelCheck, l)
		case strings.HasPrefix(l, "grid brain accept ") && strings.Contains(l, "--reject"):
			c.levelReject = append(c.levelReject, l)
		case strings.HasPrefix(l, "grid brain accept "):
			c.levelAccept = append(c.levelAccept, l)
		}
	}
	return c
}

func (c wmTplCurator) require(t *testing.T) {
	t.Helper()
	for _, r := range []struct {
		what string
		n    int
	}{{"list", len(c.list)}, {"level accept (`grid brain accept --project <project> brain:<level>/<id>`)", len(c.levelAccept)}, {"level reject", len(c.levelReject)}, {"level read", len(c.levelRead)}, {"level check", len(c.levelCheck)}, {"task read (`grid brain show <id> --proposal --task <task>`)", len(c.taskRead)}, {"task decide", len(c.taskFile)}} {
		if r.n == 0 {
			t.Fatalf("grid-curator.md has no %s line", r.what)
		}
	}
	if len(c.taskFile) != 1 || len(qfVariants(c.taskFile[0])) != 3 {
		t.Fatalf("grid-curator.md must have one decide line offering accept|merge-into <note>|reject, has %q", c.taskFile)
	}
}

// wmTplRows reads the JSON row list the template's list line prints.
func wmTplRows(t *testing.T, line, out string) []brain.WaitingProposal {
	t.Helper()
	var rows []brain.WaitingProposal
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("line %q: output is not the JSON row list: %q", line, out)
	}
	return rows
}

func wmTplVals(row brain.WaitingProposal, reason string) map[string]string {
	v := qfVals(row, reason)
	v["<proposal-id>"] = row.ID
	v["<note-id>"] = "brain:project/target"
	return v
}

func TestWmTplCuratorWalk(t *testing.T) {
	env, dir := qfFixture(t)
	c := wmTplClassify(t)
	// The rows TestQfCuratorWalk builds: three task rows, two project rows, a home row.
	qfTaskProposal(t, dir, "dup", "conventions")
	qfTaskProposal(t, dir, "weak", "conventions")
	qfPropose(t, env, "sample", "project", "chk-no", "conventions")
	qfPropose(t, env, "", "home", "hm", "decisions")
	s := brain.Store{GridHome: env.GridHome, Project: "sample"}
	if _, err := s.Propose(brain.Note{Name: "target", Topic: "conventions", Description: "When testing", Summary: "Original", Source: "T-00", Level: "project", Body: "Original body."}, brain.Origin{Kind: "session", ID: "example"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept("project", "target"); err != nil {
		t.Fatal(err)
	}
	c.require(t)
	want := map[string]int{"lesson": qfAccept, "dup": qfMerge, "weak": qfReject, "chk": qfAccept, "chk-no": qfReject, "hm": qfAccept}
	code, out, errs, filled := qfRunLine(t, env, c.list, wmTplVals(brain.WaitingProposal{}, ""))
	if code != 0 {
		t.Fatalf("line %q: exit %d: %s", filled, code, errs)
	}
	rows := wmTplRows(t, filled, out)
	if len(rows) != len(want) {
		t.Fatalf("%q lists %d rows, want %d: %+v", filled, len(rows), len(want), rows)
	}
	for _, row := range rows {
		d, ok := want[row.ID]
		if !ok {
			t.Fatalf("unexpected row %+v", row)
		}
		var steps []string
		switch {
		case strings.HasPrefix(row.Path, "task:"):
			steps = append(steps, c.taskRead...)
			steps = append(steps, qfVariants(c.taskFile[0])[d])
		case strings.HasPrefix(row.Path, "brain:"):
			steps = append(steps, c.levelRead...)
			steps = append(steps, c.levelCheck...)
			if d == qfReject {
				steps = append(steps, c.levelReject...)
			} else {
				steps = append(steps, c.levelAccept...)
			}
		default:
			t.Fatalf("row %+v has neither a task: nor a brain: path", row)
		}
		for _, step := range steps {
			code, out, errs, filled := qfRunLine(t, env, step, wmTplVals(row, "checked by the walk"))
			if code != 0 {
				t.Fatalf("row %s: line %q: exit %d: %s%s", row.Path, filled, code, out, errs)
			}
		}
	}
	if _, out, _ := qfRun(env, "keep", "--task", "T-01", "--json"); out != "[]\n" {
		t.Fatalf("keep --task T-01 --json after the walk prints %q, want []", out)
	}
	for _, name := range []string{"lesson", "chk"} {
		if e, err := s.Show("brain:project/" + name); err != nil || e.Note.Status != "accepted" {
			t.Fatalf("%s: %+v %v", name, e.Note.Status, err)
		}
	}
	if _, err := s.Show("brain:project/dup"); err == nil {
		t.Fatal("dup was published as its own note instead of merged")
	}
	if e, err := s.Show("brain:project/target"); err != nil || !strings.Contains(e.Note.Body, "Run checks before landing dup.") {
		t.Fatalf("dup was not merged into target: %v", err)
	}
	// the walk names the project, so the home row is filed in it
	if e, err := s.Show("brain:project/hm"); err != nil || e.Note.Status != "accepted" {
		t.Fatalf("home row: %+v %v", e.Note.Status, err)
	}
	for _, name := range []string{"weak", "chk-no"} {
		if _, err := s.Show("brain:project/" + name); err == nil {
			t.Fatalf("rejected %s was published", name)
		}
	}
}
