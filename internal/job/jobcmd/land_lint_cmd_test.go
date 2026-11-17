package jobcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

var landKindLine = regexp.MustCompile(`(?m)^kind:.*\n`)

// landLintCmd lints the rendered Job as a land Job (role runner, kind land and
// the given land fields; an empty value is left out) through
// `lint <file> --task T-01 --json`, and returns the exit code, ok and findings.
func landLintCmd(t *testing.T, env Env, file, name string, fields map[string]string) (int, bool, []struct{ Rule, Severity string }) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !seRoleLine.Match(b) {
		t.Fatal("the rendered Job has no role line")
	}
	lines := []string{`role: "runner"`, `kind: "land"`}
	for _, key := range []string{"target", "branch", "sha"} {
		if v := fields[key]; v != "" {
			lines = append(lines, key+`: "`+v+`"`)
		}
	}
	edited := seRoleLine.ReplaceAll(landKindLine.ReplaceAll(b, nil), []byte(strings.Join(lines, "\n")))
	path := filepath.Join(filepath.Dir(file), "land-"+name+".md")
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, errText := refusalRun(env, "lint", path, "--task", "T-01", "--json")
	var r struct {
		OK       bool `json:"ok"`
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("%s: exit %d stdout %q stderr %q: %v", name, code, stdout, errText, err)
	}
	var out []struct{ Rule, Severity string }
	for _, f := range r.Findings {
		out = append(out, struct{ Rule, Severity string }{f.Rule, f.Severity})
	}
	return code, r.OK, out
}

// grid job lint passes a land Job with role runner and target, branch and sha,
// and refuses each missing field with its own rule and exit 1.
func TestLandKindJobLintCommand(t *testing.T) {
	e, _, _ := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the script.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	rendered := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")

	full := map[string]string{"target": "main", "branch": "grid-T-01-J-3-a1", "sha": "0123456789abcdef0123456789abcdef01234567"}
	code, ok, fs := landLintCmd(t, e, rendered.Path, "full", full)
	if code != 0 || !ok {
		t.Fatalf("full land Job: exit %d ok %v findings %+v; want exit 0 and ok", code, ok, fs)
	}
	for _, f := range fs {
		if f.Severity == "error" || strings.HasPrefix(f.Rule, "job.land-") || f.Rule == "job.role-agent" {
			t.Fatalf("full land Job: finding %+v", f)
		}
	}
	for _, missing := range []string{"target", "branch", "sha"} {
		v := map[string]string{}
		for k, s := range full {
			v[k] = s
		}
		v[missing] = ""
		code, ok, fs := landLintCmd(t, e, rendered.Path, "no-"+missing, v)
		var land []string
		for _, f := range fs {
			if strings.HasPrefix(f.Rule, "job.land-") {
				land = append(land, f.Rule+"/"+f.Severity)
			}
		}
		if code != 1 || ok || len(land) != 1 || land[0] != "job.land-"+missing+"/error" {
			t.Fatalf("no %s: exit %d ok %v land findings %v; want exit 1 and one job.land-%s error", missing, code, ok, land, missing)
		}
	}
}
