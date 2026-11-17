package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// landBrief is the golden brief as a land Job: kind land, role runner, and the
// land fields given. A field with an empty value is left out.
func landBrief(t *testing.T, target, branch, sha string) string {
	t.Helper()
	var extra []string
	for _, f := range []struct{ key, value string }{{"target", target}, {"branch", branch}, {"sha", sha}} {
		if f.value != "" {
			extra = append(extra, f.key+`: "`+f.value+`"`)
		}
	}
	var out []string
	for _, l := range strings.Split(briefWith(t, extra...), "\n") {
		switch {
		case strings.HasPrefix(l, "role:"):
			l = `role: "runner"`
		case strings.HasPrefix(l, "kind:"):
			l = `kind: "land"`
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func landFindings(t *testing.T, text string) []lint.Finding {
	t.Helper()
	var out []lint.Finding
	for _, f := range lint.Check(text, lint.Options{Resolver: testFiles(t)}) {
		if strings.HasPrefix(f.Rule, "job.land-") {
			out = append(out, f)
		}
	}
	return out
}

// A land Job with role runner and all three fields lints clean.
func TestLandKindWithAllFieldsLintsClean(t *testing.T) {
	text := landBrief(t, "main", "grid-T-01-J-3-a1", "0123456789abcdef0123456789abcdef01234567")
	if fs := lint.Check(text, lint.Options{Resolver: testFiles(t)}); len(fs) != 0 {
		t.Fatalf("findings: %+v", fs)
	}
}

// Each missing field is refused with its own rule, and only that one.
func TestLandKindRefusesEachMissingField(t *testing.T) {
	full := map[string]string{"target": "main", "branch": "grid-T-01-J-3-a1", "sha": "0123456789abcdef"}
	for _, tc := range []struct{ missing, rule string }{
		{"target", "job.land-target"},
		{"branch", "job.land-branch"},
		{"sha", "job.land-sha"},
	} {
		t.Run(tc.missing, func(t *testing.T) {
			v := map[string]string{}
			for k, s := range full {
				v[k] = s
			}
			v[tc.missing] = ""
			fs := landFindings(t, landBrief(t, v["target"], v["branch"], v["sha"]))
			if len(fs) != 1 || fs[0].Rule != tc.rule || fs[0].Severity != "error" {
				t.Fatalf("findings = %+v, want one %s error", fs, tc.rule)
			}
		})
	}
	if fs := landFindings(t, landBrief(t, "", "", "")); len(fs) != 3 {
		t.Fatalf("all missing: findings = %+v, want three", fs)
	}
}

// Another kind never needs the land fields.
func TestLandFieldsAreOnlyCheckedForKindLand(t *testing.T) {
	text := strings.Replace(landBrief(t, "", "", ""), `kind: "land"`, `kind: "build"`, 1)
	if fs := landFindings(t, text); len(fs) != 0 {
		t.Fatalf("findings: %+v", fs)
	}
}
