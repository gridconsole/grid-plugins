package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

const seRule = "job.role-agent"

// seKnowsBuilder is a KnownRole that knows only the builder.
func seKnowsBuilder(role string) bool { return role == "builder" }

// seWithRole is the golden brief with its role replaced.
func seWithRole(t *testing.T, role string) string {
	t.Helper()
	text := brief(t)
	if !strings.Contains(text, "\nrole: \"writer\"\n") {
		t.Fatal("the golden brief no longer has role: \"writer\"")
	}
	return strings.Replace(text, "\nrole: \"writer\"\n", "\nrole: \""+role+"\"\n", 1)
}

func seFindings(fs []lint.Finding, rule string) []lint.Finding {
	var out []lint.Finding
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func TestSeRoleWithoutAgentWarns(t *testing.T) {
	text := seWithRole(t, "nobody")
	// KnownRole nil: the base behaviour, no role finding and no finding at all.
	base := lint.Check(text, lint.Options{Resolver: testFiles(t)})
	t.Logf("findings with KnownRole nil, role nobody: %+v", base)
	if len(seFindings(base, seRule)) != 0 {
		t.Fatalf("a nil KnownRole added %s: %+v", seRule, base)
	}
	fs := lint.Check(text, lint.Options{Resolver: testFiles(t), KnownRole: seKnowsBuilder})
	if len(fs) != 1 {
		t.Fatalf("want one finding, got %+v", fs)
	}
	f := fs[0]
	if f.Rule != seRule || f.Severity != "warning" || f.Line != 6 {
		t.Fatalf("finding %+v; want rule %s, severity warning, line 6 (the role line)", f, seRule)
	}
	if f.Message != "No agent runs role nobody: none in the plugin, the catalog or the brain" {
		t.Fatalf("message %q", f.Message)
	}
	if f.Fix != "Use the step's who without grid- (for example builder, designer), or add knowledge/agents/grid-nobody.md to the brain." {
		t.Fatalf("fix %q", f.Fix)
	}
	if lint.HasErrors(fs) {
		t.Fatalf("a warning failed the check: %+v", fs)
	}
}

func TestSeRolesKnown(t *testing.T) {
	for _, role := range []string{"builder", "Builder", "grid-builder", "person", "Person"} {
		fs := lint.Check(seWithRole(t, role), lint.Options{Resolver: testFiles(t), KnownRole: seKnowsBuilder})
		if got := seFindings(fs, seRule); len(got) != 0 {
			t.Errorf("role %q: %+v", role, got)
		}
	}
}
