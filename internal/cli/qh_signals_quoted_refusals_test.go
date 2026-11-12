package cli

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
)

// qhSkillLine returns the line of the signals skill that holds want.
func qhSkillLine(t *testing.T, want string) string {
	t.Helper()
	text, ok := gridplugins.SkillFor("signals")
	if !ok {
		t.Fatal("the plugin has no signals skill")
	}
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, want) {
			return l
		}
	}
	t.Fatalf("skills/signals/SKILL.md has no line holding %q", want)
	return ""
}

// qhSpans returns the text between each pair of backticks on a line.
func qhSpans(line string) []string {
	var out []string
	for i, p := range strings.Split(line, "`") {
		if i%2 == 1 {
			out = append(out, p)
		}
	}
	return out
}

// qhTokens splits a command the way the skill writes it: spaces separate,
// double quotes group, and an unquoted <placeholder ...> is one word.
func qhTokens(s string) []string {
	var out []string
	var cur strings.Builder
	have, quoted, angle := false, false, false
	for _, r := range s {
		switch {
		case r == '"' && !angle:
			quoted, have = !quoted, true
		case r == '<' && !quoted && !angle:
			angle, have = true, true
			cur.WriteRune(r)
		case r == '>' && angle:
			angle = false
			cur.WriteRune(r)
		case r == ' ' && !quoted && !angle:
			if have {
				out = append(out, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteRune(r)
			have = true
		}
	}
	if have {
		out = append(out, cur.String())
	}
	return out
}

// qhArgs turns the tokens of a `grid signal add` command into argv: each
// <placeholder> becomes a value from fill, keyed by the flag in front of it.
func qhArgs(t *testing.T, command string, fill map[string]string) []string {
	t.Helper()
	_, tail, ok := strings.Cut(command, "grid signal add ")
	if !ok {
		t.Fatalf("no grid signal add in %q", command)
	}
	args := []string{"signal", "add"}
	toks := qhTokens(tail)
	for i, tok := range toks {
		if strings.HasPrefix(tok, "<") && strings.HasSuffix(tok, ">") {
			flag := toks[i-1]
			v, ok := fill[flag]
			if !ok {
				t.Fatalf("no value for the placeholder %s after %s", tok, flag)
			}
			tok = v
		}
		args = append(args, tok)
	}
	return args
}

var qhFill = map[string]string{
	"--project": "example", "--source": "mail", "--source-id": "m-1", "--title": "T",
	"--sender": "Marieke", "--recipient": "the vertex address",
}

func qhFillWith(over map[string]string) map[string]string {
	m := map[string]string{}
	for k, v := range qhFill {
		m[k] = v
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

// Step 4 of the skill names the exit code and the stderr text of a refused add;
// the add line of step 3 with a two-line --sender must give exactly that.
func TestQhSignalsQuotedRefusals(t *testing.T) {
	step := qhSkillLine(t, "4. Read the answer")
	m := regexp.MustCompile("If the command exits ([0-9]+(?: or [0-9]+)*)(?: with `([^`]*)`)?").FindStringSubmatch(step)
	if m == nil {
		t.Fatalf("step 4 names no exit code: %q", step)
	}
	var named []int
	for _, n := range regexp.MustCompile(`[0-9]+`).FindAllString(m[1], -1) {
		c, _ := strconv.Atoi(n)
		named = append(named, c)
	}
	prefix, _, _ := strings.Cut(m[2], "<")
	if prefix == "" {
		t.Errorf("step 4 quotes no stderr text: %q", step)
	}

	add := qhSpans(qhSkillLine(t, "grid signal add --project <project>"))
	if len(add) == 0 {
		t.Fatal("the add line holds no command")
	}
	h := newHarness(t)
	folder := lvSigProject(t, h)
	h.env.Stdin = strings.NewReader("x")
	args := qhArgs(t, add[0], qhFillWith(map[string]string{"--sender": "a\nb"}))
	code := h.run(folder, args...)
	t.Logf("%q -> exit %d, stderr %q", args, code, h.errb.String())
	for _, c := range named {
		if code != c {
			t.Errorf("the skill names exit %d, the add line gave %d", c, code)
		}
	}
	if !strings.HasPrefix(h.errb.String(), prefix) || !strings.Contains(h.errb.String(), "grid: sender must be one line") {
		t.Errorf("stderr %q lacks %q and the sender line", h.errb.String(), prefix)
	}
}

// The personal line gives the reason --project personal fails; it fails from
// every folder, and the form the line recommends works.
func TestQhPersonalFlagRefusedEverywhere(t *testing.T) {
	line := qhSkillLine(t, "--project personal")
	if strings.Contains(line, "from any other folder") {
		t.Errorf("the personal line still limits the refusal to other folders: %q", line)
	}
	exit := regexp.MustCompile(`fails with ` + "`([^`]+)`" + ` \(exit ([0-9]+)\)`).FindStringSubmatch(line)
	if exit == nil {
		t.Fatalf("the personal line names no refusal text and exit: %q", line)
	}
	want, _ := strconv.Atoi(exit[2])
	spans := qhSpans(line)
	if spans[0] != "--project personal" {
		t.Fatalf("first span %q", spans[0])
	}

	h := newHarness(t)
	folder := lvSigProject(t, h)
	for name, cwd := range map[string]string{"grid-home": h.gridHome, "project-folder": folder} {
		h.env.Stdin = strings.NewReader("x")
		args := []string{"signal", "add", "--project", "personal", "--source", "mail", "--source-id", "m-2", "--title", "T", "--body", "-"}
		code := h.run(cwd, args...)
		t.Logf("%s: %q -> exit %d, stderr %q", name, args, code, h.errb.String())
		if code != want || !strings.Contains(h.errb.String(), exit[1]) || code != 1 {
			t.Errorf("%s: exit %d stderr %q, want exit %d (the skill) and 1 with %q", name, code, h.errb.String(), want, exit[1])
		}
		if !strings.Contains(h.errb.String(), "project is not registered") {
			t.Errorf("%s: stderr %q lacks project is not registered", name, h.errb.String())
		}
	}

	var rec string
	for _, s := range spans {
		if strings.HasPrefix(s, `cd "$GRID_HOME" && `) {
			rec = s
		}
	}
	if rec == "" {
		t.Fatalf(`the personal line has no cd "$GRID_HOME" && command: %q`, line)
	}
	if strings.Contains(rec, "--project") {
		t.Errorf("the recommended form still has --project: %q", rec)
	}
	h.env.Stdin = strings.NewReader("x")
	args := qhArgs(t, rec, qhFillWith(map[string]string{"--source-id": "m-3"}))
	code := h.run(h.gridHome, args...)
	t.Logf("recommended: %q from the Grid home -> exit %d, stderr %q", args, code, h.errb.String())
	if code != 0 {
		t.Errorf("the recommended form exit %d: %s", code, h.errb.String())
	}
}
