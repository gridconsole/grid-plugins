package jobcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

var seRoleLine = regexp.MustCompile(`(?m)^role:.*$`)

// seLintRole lints the fixture's rendered Job with its role replaced, as
// `lint <file> --task T-01 --json`, and returns the exit code and findings.
func seLintRole(t *testing.T, env Env, file, role string) (int, []string) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !seRoleLine.Match(b) {
		t.Fatal("the rendered Job has no role line")
	}
	edited := seRoleLine.ReplaceAll(b, []byte(`role: "`+role+`"`))
	path := filepath.Join(filepath.Dir(file), "se-role-"+role+".md")
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, errText := refusalRun(env, "lint", path, "--task", "T-01", "--json")
	var r struct {
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("role %s: exit %d stdout %q stderr %q: %v", role, code, stdout, errText, err)
	}
	var rules []string
	for _, f := range r.Findings {
		if f.Rule == "job.role-agent" {
			if f.Severity != "warning" {
				t.Errorf("role %s: job.role-agent severity %q", role, f.Severity)
			}
			rules = append(rules, f.Rule)
		}
	}
	return code, rules
}

// A world with no project has no project brain; knownRole skips it and still reads the home brain.
func TestSeKnownRoleNoProject(t *testing.T) {
	gh := t.TempDir()
	w := &world{store: brain.Store{GridHome: gh}}
	if w.knownRole("nobody") {
		t.Fatal("role nobody known with no project and no brain agent")
	}
	if !w.knownRole("builder") {
		t.Fatal("plugin agent builder unknown with no project")
	}
	dir := filepath.Join(gh, "knowledge", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "grid-auditor.md"), []byte("---\nname: grid-auditor\ndescription: Audits.\n---\nAudits.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !w.knownRole("auditor") {
		t.Fatal("home brain agent grid-auditor unknown with no project")
	}
}

func TestSeLintBrainAgent(t *testing.T) {
	e, taskDir, _ := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the script.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	rendered := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")

	// No agent anywhere: a warning, and the exit code stays 0.
	if code, got := seLintRole(t, e, rendered.Path, "nobody"); code != 0 || len(got) != 1 {
		t.Fatalf("role nobody: exit %d, job.role-agent findings %v; want exit 0 and one", code, got)
	}
	// researcher is a catalog agent (grid-researcher).
	if code, got := seLintRole(t, e, rendered.Path, "researcher"); code != 0 || len(got) != 0 {
		t.Fatalf("role researcher: exit %d, job.role-agent findings %v; want exit 0 and none", code, got)
	}
	// auditor has no plugin or catalog agent until the brain holds grid-auditor.
	if _, got := seLintRole(t, e, rendered.Path, "auditor"); len(got) != 1 {
		t.Fatalf("role auditor before the brain agent: %v; want one finding", got)
	}
	dir := filepath.Join(e.GridHome, "knowledge", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "grid-auditor.md"), []byte("---\nname: grid-auditor\ndescription: Audits.\n---\nAudits.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, got := seLintRole(t, e, rendered.Path, "auditor"); code != 0 || len(got) != 0 {
		t.Fatalf("role auditor with the brain agent: exit %d, job.role-agent findings %v; want exit 0 and none", code, got)
	}
	_ = taskDir
}
