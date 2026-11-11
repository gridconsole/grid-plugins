package contract

import (
	"regexp"
	"strings"
	"testing"
)

var kb3QQuoted = regexp.MustCompile(`"([a-z]+)"`)

// kb3QStep2 returns the questions skill text from the line starting "2. Ask:"
// up to the line starting "3. ".
func kb3QStep2(t *testing.T, skill string) string {
	t.Helper()
	var out []string
	in := false
	for _, line := range strings.Split(skill, "\n") {
		if strings.HasPrefix(line, "2. Ask:") {
			in = true
		} else if in && strings.HasPrefix(line, "3. ") {
			break
		}
		if in {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		t.Fatal("step 2 not found in skills/questions/SKILL.md")
	}
	return strings.Join(out, "\n")
}

func TestKb3QWorkerAskText(t *testing.T) {
	step := kb3QStep2(t, readPluginFile(t, "skills/questions/SKILL.md"))
	for _, want := range []string{"may not post person gates", "gate_post_roles"} {
		if !strings.Contains(step, want) {
			t.Errorf("step 2 lacks %q", want)
		}
	}
	if strings.Contains(step, "is refused a person gate") {
		t.Errorf("step 2 still holds the imprecise refusal text")
	}
	if !strings.Contains(readPluginFile(t, "internal/feed/store.go"), "may not post %s gates") {
		t.Errorf("internal/feed/store.go no longer holds the refusal literal the skill quotes")
	}
	var person string
	inDefault := false
	for _, line := range strings.Split(readPluginFile(t, "internal/workflow/gates.go"), "\n") {
		if strings.Contains(line, "func defaultGatePostRoles") {
			inDefault = true
			continue
		}
		if inDefault && strings.Contains(line, `"person":`) {
			person = line
			break
		}
	}
	if person == "" {
		t.Fatal(`no "person": line in defaultGatePostRoles`)
	}
	_, list, _ := strings.Cut(person, ":")
	roles := kb3QQuoted.FindAllStringSubmatch(list, -1)
	if len(roles) == 0 {
		t.Fatalf("no roles on %q", person)
	}
	for _, m := range roles {
		if m[1] == "worker" {
			t.Errorf("default person gate roles now hold worker: update the skill and this test")
		}
		if !strings.Contains(step, m[1]) {
			t.Errorf("default person gate role %q is missing from step 2 of the skill", m[1])
		}
	}
}
