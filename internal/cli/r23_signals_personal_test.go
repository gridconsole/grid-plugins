package cli

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The personal line of the signals skill gives the reason --project personal
// fails; it fails from every folder. The form it recommends has no cd and no
// --project (a routine already runs in the Grid home) and works from there.
// This replaces TestQhPersonalFlagRefusedEverywhere, whose last half required a
// span that starts with cd "$GRID_HOME" &&, the form a routine's allow list
// refuses.
func TestR23PersonalSignalAddFromGridHome(t *testing.T) {
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
		if strings.HasPrefix(s, "grid signal add ") {
			rec = s
		}
	}
	if rec == "" {
		t.Fatalf("the personal line has no grid signal add command: %q", line)
	}
	if strings.Contains(rec, "--project") || strings.Contains(line, "cd \"$GRID_HOME\"") {
		t.Errorf("the recommended form still has --project or a cd: %q", line)
	}
	h.env.Stdin = strings.NewReader("x")
	args := qhArgs(t, rec, qhFillWith(map[string]string{"--source-id": "m-3"}))
	code := h.run(h.gridHome, args...)
	t.Logf("recommended: %q from the Grid home -> exit %d, stderr %q", args, code, h.errb.String())
	if code != 0 {
		t.Errorf("the recommended form exit %d: %s", code, h.errb.String())
	}
}
