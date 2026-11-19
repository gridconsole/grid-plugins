package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// dwBaseCheck lints the golden brief with its base and after edges replaced.
func dwBaseCheck(t *testing.T, base string, after []job.Dependency) []lint.Finding {
	t.Helper()
	src := brief(t)
	j, _, err := job.DecodeBrief(src)
	if err != nil {
		t.Fatal(err)
	}
	j.After = after
	m, err := job.Metadata(j)
	if err != nil {
		t.Fatal(err)
	}
	if base != "" {
		// Metadata leaves base out, so the brief text carries it as a field of its own.
		m = strings.Replace(m, "---\n", "---\nbase: \""+base+"\"\n", 1)
	}
	return lint.Check(m+src[strings.Index(src, "\n## Intent"):], lint.Options{})
}

func dwBaseFinding(fs []lint.Finding) *lint.Finding {
	for i := range fs {
		if fs[i].Rule == "job.base-head" {
			return &fs[i]
		}
	}
	return nil
}

func TestDwHeadBaseRefusedWithoutAfter(t *testing.T) {
	f := dwBaseFinding(dwBaseCheck(t, "head:J-2", nil))
	if f == nil || !strings.Contains(f.Fix, "Add J-2 to after") {
		t.Fatalf("head:J-2 with no after: finding %+v", f)
	}
}

func TestDwHeadBaseAcceptedWithAfter(t *testing.T) {
	fs := dwBaseCheck(t, "head:J-2", []job.Dependency{{Job: "J-2"}})
	if len(fs) != 0 {
		t.Fatalf("head:J-2 after J-2 has findings: %+v", fs)
	}
}

func TestDwPlainBaseLintsAsToday(t *testing.T) {
	fs := dwBaseCheck(t, "main", nil)
	if dwBaseFinding(fs) != nil || len(fs) != 0 {
		t.Fatalf("a plain base has findings: %+v", fs)
	}
}
