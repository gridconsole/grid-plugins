package braincmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/brain"
)

// qfRun runs one brain command through Run and returns its exit code and output.
func qfRun(env Env, args ...string) (int, string, string) {
	var out, errs bytes.Buffer
	env.Stdout, env.Stderr = &out, &errs
	code := Run(args, env)
	return code, out.String(), errs.String()
}

// qfPropose files one proposal in the project (or home) store of the fixture.
func qfPropose(t *testing.T, env Env, project, level, name, topic string) {
	t.Helper()
	s := brain.Store{GridHome: env.GridHome, Project: project}
	_, err := s.Propose(brain.Note{Name: name, Topic: topic, Description: "When checking changes", Summary: "Run the checks", Source: "T-01", Level: level, Body: "Run checks before landing " + name + ".\n"}, brain.Origin{Kind: "session", ID: "example"})
	if err != nil {
		t.Fatal(err)
	}
}

// qfTaskProposal writes one more proposal into the task's own brain-proposals folder.
func qfTaskProposal(t *testing.T, dir, name, topic string) {
	t.Helper()
	n := brain.Note{Name: name, Topic: topic, Type: "reference", Description: "When checking changes", Summary: "Run the checks", Source: "T-01", Level: "project", Status: "proposed", Origin: brain.Origin{Kind: "session", ID: "example"}, Body: "Run checks before landing " + name + ".\n"}
	b, err := brain.Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "brain-proposals", name+".md"), b, 0600); err != nil {
		t.Fatal(err)
	}
}

// qfFixture is decideFixture (task T-01 of project "sample" with task proposal
// lesson) plus a project proposal chk.
func qfFixture(t *testing.T) (Env, string) {
	env, dir := decideFixture(t)
	qfPropose(t, env, "sample", "project", "chk", "conventions")
	return env, dir
}

func TestQfShowTaskProposal(t *testing.T) {
	env, _ := qfFixture(t)
	for _, id := range []string{"lesson", "task:brain-proposals/lesson.md"} {
		code, out, errs := qfRun(env, "show", id, "--proposal", "--task", "T-01", "--json")
		if code != 0 {
			t.Fatalf("show %s --proposal --task T-01: exit %d: %s", id, code, errs)
		}
		if !strings.Contains(out, `"name":"lesson"`) {
			t.Fatalf("show %s: no lesson in %s", id, out)
		}
		var e brain.Entry
		if err := json.Unmarshal([]byte(out), &e); err != nil || e.Note.Name != "lesson" || e.Note.Status != "proposed" || e.Path != "task:brain-proposals/lesson.md" {
			t.Fatalf("show %s: entry %+v: %v", id, e, err)
		}
	}
	// Plain text output reads the same proposal.
	if code, out, errs := qfRun(env, "show", "lesson", "--proposal", "--task", "T-01"); code != 0 || !strings.Contains(out, "lesson") {
		t.Fatalf("plain show: exit %d: %s %s", code, out, errs)
	}
}

// A plain id that is not in the task folder is still read from the level store,
// as on base, and names that are not plain names are refused.
func TestQfShowTaskIdEdges(t *testing.T) {
	env, _ := qfFixture(t)
	code, out, errs := qfRun(env, "show", "chk", "--proposal", "--task", "T-01", "--json")
	if code != 0 || !strings.Contains(out, `"id":"brain:project/chk"`) {
		t.Fatalf("level fallback: exit %d: %s %s", code, out, errs)
	}
	for _, id := range []string{"../lesson", "task:brain-proposals/../lesson.md", "task:other/lesson.md", "task:brain-proposals/missing.md", "missing"} {
		if code, _, _ := qfRun(env, "show", id, "--proposal", "--task", "T-01"); code != 1 {
			t.Fatalf("show %s: exit %d, want 1", id, code)
		}
	}
}

func TestQfShowLevelProposalUnchanged(t *testing.T) {
	env, _ := qfFixture(t)
	for _, extra := range [][]string{nil, {"--json"}} {
		wantCode, wantOut, wantErr := qfRun(env, append([]string{"show", "brain:project/chk", "--proposal"}, extra...)...)
		code, out, errs := qfRun(env, append([]string{"show", "brain:project/chk", "--proposal", "--task", "T-01"}, extra...)...)
		if wantCode != 0 || !strings.Contains(wantOut, "chk") {
			t.Fatalf("show without --task: exit %d: %s %s", wantCode, wantOut, wantErr)
		}
		if code != wantCode || out != wantOut {
			t.Fatalf("--task changed a brain: id: exit %d (want %d)\n%s\nwant\n%s\n%s", code, wantCode, out, wantOut, errs)
		}
	}
}

var qfSpan = regexp.MustCompile("`(grid brain [^`]*)`")

// qfCuratorLines cuts the backticked `grid brain ...` lines out of the curator's
// own text, in order, from the embedded agent file.
func qfCuratorLines(t *testing.T) []string {
	t.Helper()
	text, ok := gridplugins.AgentFor("curator")
	if !ok {
		t.Fatal("no curator agent")
	}
	var lines []string
	for _, m := range qfSpan.FindAllStringSubmatch(text, -1) {
		lines = append(lines, m[1])
	}
	if len(lines) == 0 {
		t.Fatal("curator.md names no grid brain line")
	}
	return lines
}

// qfFill puts the row's values into the placeholders of one line.
func qfFill(line string, vals map[string]string) string {
	for k, v := range vals {
		line = strings.ReplaceAll(line, k, v)
	}
	return line
}

// qfArgv splits a filled line into the arguments after "grid brain", keeping a
// double-quoted run as one argument.
func qfArgv(t *testing.T, line string) []string {
	t.Helper()
	var args []string
	var cur strings.Builder
	quoted, has := false, false
	for _, r := range strings.TrimPrefix(line, "grid brain ") {
		switch {
		case r == '"':
			quoted, has = !quoted, true
		case r == ' ' && !quoted:
			if has {
				args = append(args, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if quoted {
		t.Fatalf("unbalanced quote in %q", line)
	}
	if has {
		args = append(args, cur.String())
	}
	return args
}

// qfAlternatives expands the one `accept|merge-into <note>|reject` choice of a
// decide line into its three commands.
var qfChoice = regexp.MustCompile(`accept\|merge-into <note>\|reject`)

func qfVariants(line string) []string {
	if !qfChoice.MatchString(line) {
		return []string{line}
	}
	return []string{qfChoice.ReplaceAllString(line, "accept"), qfChoice.ReplaceAllString(line, "merge-into <note>"), qfChoice.ReplaceAllString(line, "reject")}
}

const (
	qfAccept = iota
	qfMerge
	qfReject
)

// qfCurator holds the curator's lines by what each one does.
type qfCurator struct {
	list, index                        string
	taskRead, taskFile                 []string
	levelRead, levelCheck, levelAccept []string
	levelReject                        []string
}

func qfClassify(t *testing.T) qfCurator {
	t.Helper()
	var c qfCurator
	for _, l := range qfCuratorLines(t) {
		switch {
		case strings.HasPrefix(l, "grid brain keep "):
			c.list = l
		case strings.HasPrefix(l, "grid brain index"):
			c.index = l
		case strings.HasPrefix(l, "grid brain show ") && strings.Contains(l, "--task <task>"):
			c.taskRead = append(c.taskRead, l)
		case strings.HasPrefix(l, "grid brain decide "):
			c.taskFile = append(c.taskFile, l)
		case strings.HasPrefix(l, "grid brain show "):
			c.levelRead = append(c.levelRead, l)
		case strings.HasPrefix(l, "grid brain check "):
			c.levelCheck = append(c.levelCheck, l)
		case strings.HasPrefix(l, "grid brain accept ") && strings.Contains(l, "--reject"):
			c.levelReject = append(c.levelReject, l)
		case strings.HasPrefix(l, "grid brain accept "):
			c.levelAccept = append(c.levelAccept, l)
		default:
			t.Fatalf("curator.md line %q is not a line the walk knows", l)
		}
	}
	return c
}

// require fails when the curator's text lacks a kind of line. The walk calls it
// after the list line has run, so a wrong line is named before a missing one.
func (c qfCurator) require(t *testing.T) {
	t.Helper()
	for what, n := range map[string]int{"task read": len(c.taskRead), "task decide": len(c.taskFile), "level read": len(c.levelRead), "level check": len(c.levelCheck), "level accept": len(c.levelAccept), "level reject": len(c.levelReject), "index": len(c.index)} {
		if n == 0 {
			t.Fatalf("curator.md has no %s line", what)
		}
	}
	if len(c.taskFile) != 1 {
		t.Fatalf("curator.md has %d decide lines, want one", len(c.taskFile))
	}
	if len(qfVariants(c.taskFile[0])) != 3 {
		t.Fatalf("the decide line %q does not offer accept|merge-into <note>|reject", c.taskFile[0])
	}
}

// qfRows runs the curator's list line and returns the rows it prints.
func qfRows(t *testing.T, env Env, c qfCurator) []brain.WaitingProposal {
	t.Helper()
	line := qfFill(c.list, map[string]string{"<task>": "T-01"})
	code, out, errs := qfRun(env, qfArgv(t, line)...)
	if code != 0 {
		t.Fatalf("line %q: exit %d: %s", line, code, errs)
	}
	var rows []brain.WaitingProposal
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("line %q: output is not the JSON row list: %q", line, out)
	}
	c.require(t)
	return rows
}

// qfRunLine fills and runs one line, and returns what Run said.
func qfRunLine(t *testing.T, env Env, line string, vals map[string]string) (int, string, string, string) {
	t.Helper()
	filled := qfFill(line, vals)
	code, out, errs := qfRun(env, qfArgv(t, filled)...)
	return code, out, errs, filled
}

func qfVals(row brain.WaitingProposal, reason string) map[string]string {
	level := ""
	if rest, ok := strings.CutPrefix(row.Path, "brain:"); ok {
		level, _, _ = strings.Cut(rest, "/")
	}
	return map[string]string{"<task>": "T-01", "<id>": row.ID, "<level>": level, "<project>": "sample", "<note>": "brain:project/target", "<reason>": reason, "<why>": reason}
}

func TestQfCuratorWalk(t *testing.T) {
	env, dir := qfFixture(t)
	c := qfClassify(t)
	// Rows of every kind: three task rows, two project rows and a home row.
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
	want := map[string]int{"lesson": qfAccept, "dup": qfMerge, "weak": qfReject, "chk": qfAccept, "chk-no": qfReject, "hm": qfAccept}
	rows := qfRows(t, env, c)
	if len(rows) != len(want) {
		t.Fatalf("keep --task T-01 --json lists %d rows, want %d: %+v", len(rows), len(want), rows)
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
			code, out, errs, filled := qfRunLine(t, env, step, qfVals(row, "checked by the walk"))
			if code != 0 {
				t.Fatalf("row %s: line %q: exit %d: %s%s", row.Path, filled, code, out, errs)
			}
		}
	}
	if code, out, errs, filled := qfRunLine(t, env, c.index, qfVals(brain.WaitingProposal{}, "")); code != 0 {
		t.Fatalf("line %q: exit %d: %s%s", filled, code, out, errs)
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
	if e, err := (brain.Store{GridHome: env.GridHome}).Show("brain:home/hm"); err != nil || e.Note.Status != "accepted" {
		t.Fatalf("home row: %+v %v", e.Note.Status, err)
	}
	for _, name := range []string{"weak", "chk-no"} {
		if _, err := s.Show("brain:project/" + name); err == nil {
			t.Fatalf("rejected %s was published", name)
		}
	}
	for _, name := range []string{"weak", "chk-no"} {
		_, out, errs := qfRun(env, "show", "brain:project/"+name, "--proposal", "--task", "T-01", "--json")
		if name == "weak" {
			_, out, errs = qfRun(env, "show", "weak", "--proposal", "--task", "T-01", "--json")
		}
		if !strings.Contains(out, `"status":"rejected"`) {
			t.Fatalf("%s is not rejected: %s%s", name, out, errs)
		}
	}
}

// A task proposal in a config topic is refused by decide's own check; the walk
// records it as a reject with the finding, and nothing is published.
func TestQfDecideRefusesAFlaggedTaskRow(t *testing.T) {
	env, dir := qfFixture(t)
	c := qfClassify(t)
	path := filepath.Join(dir, "brain-proposals", "lesson.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n, err := brain.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	n.Topic = "deploy"
	if b, err = brain.Encode(n); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	var row brain.WaitingProposal
	for _, r := range qfRows(t, env, c) {
		if r.ID == "lesson" {
			row = r
		}
	}
	if !strings.HasPrefix(row.Path, "task:") {
		t.Fatalf("lesson is not a task row: %+v", row)
	}
	variants := qfVariants(c.taskFile[0])
	accept, reject := variants[qfAccept], variants[qfReject]
	code, _, errs, filled := qfRunLine(t, env, accept, qfVals(row, "checked by the walk"))
	if code != 1 || !strings.Contains(errs, "proposal refused: ") {
		t.Fatalf("line %q: exit %d, want 1 with %q: %s", filled, code, "proposal refused: ", errs)
	}
	finding := strings.TrimSpace(errs[strings.Index(errs, "proposal refused: ")+len("proposal refused: "):])
	if code, out, errs, filled := qfRunLine(t, env, reject, qfVals(row, finding)); code != 0 {
		t.Fatalf("reject line %q: exit %d: %s%s", filled, code, out, errs)
	}
	got, err := brain.Parse(mustRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "rejected" || got.Reason != finding {
		t.Fatalf("proposal ends %q with reason %q, want rejected with %q", got.Status, got.Reason, finding)
	}
	if _, err := os.Stat(filepath.Join(env.GridHome, "projects", "sample", "knowledge", "deploy", "lesson.md")); err == nil {
		t.Fatal("the flagged proposal was published")
	}
	if _, err := (brain.Store{GridHome: env.GridHome, Project: "sample"}).Show("brain:project/lesson"); err == nil {
		t.Fatal("the flagged proposal is a note")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A home row is filed with its full brain: id: the plain id names a project
// proposal, which is why the curator's accept line carries brain:<level>/<id>.
// decide stays a task-folder verb and does not find a level proposal.
func TestQfLevelRowNeedsTheFullId(t *testing.T) {
	env, _ := qfFixture(t)
	qfPropose(t, env, "", "home", "hm", "decisions")
	if code, _, errs := qfRun(env, "accept", "--project", "sample", "hm"); code != 1 {
		t.Fatalf("accept --project sample hm: exit %d, want 1: %s", code, errs)
	}
	if code, _, errs := qfRun(env, "accept", "--project", "sample", "brain:home/hm"); code != 0 {
		t.Fatalf("accept --project sample brain:home/hm: exit %d: %s", code, errs)
	}
	if code, _, errs := qfRun(env, "decide", "T-01", "chk", "accept", "--note", "x"); code != 1 || !strings.Contains(errs, "no such file") {
		t.Fatalf("decide of a project proposal: exit %d: %s", code, errs)
	}
}
