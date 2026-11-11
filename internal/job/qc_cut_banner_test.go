package job_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// TestQcPcCutBannerNamesShow renders a Job whose brain block is cut and holds
// the banner to a command a dispatcher has: `grid brain show <id>`, with the
// cut ids left in inputs.json not_rendered.
func TestQcPcCutBannerNamesShow(t *testing.T) {
	j, f, _ := sample(t)
	base, e := job.Render(j, job.RenderOptions{Resolver: f})
	if e != nil {
		t.Fatal(e)
	}
	sources := base.Inputs.Resolver()
	j.Keep = nil
	j.Links = nil
	for _, id := range []string{"09", "08", "07", "06", "05", "04", "03", "02", "01"} {
		p := "brain:project/topic-" + id
		j.Links = append(j.Links, job.Link{To: p, Rel: "uses", Why: "Explains this requirement in the plan"})
		sources[p] = job.Source{Pointer: p, Summary: "Summary " + id, Text: "Exact source " + id, Tier: contract.PersonTeam, Topic: "topic"}
	}
	got, e := job.Render(j, job.RenderOptions{Resolver: sources})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Inputs.NotRendered) == 0 {
		t.Fatal("the brain block was not cut")
	}
	want := fmt.Sprintf("%d entries cut (inputs.json not_rendered); `grid brain show <id>` reads one", len(got.Inputs.NotRendered))
	if !strings.Contains(got.Brief, want) {
		t.Errorf("brief lacks %q", want)
	}
	if strings.Contains(got.Brief, "grid brain topic") {
		t.Error("brief names `grid brain topic`, which no dispatcher has")
	}
	for _, id := range got.Inputs.NotRendered {
		if !strings.HasPrefix(id, "brain:") {
			t.Errorf("not_rendered id %q does not start with brain:", id)
		}
	}
}
