package job_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// The brief tells the agent to search the brain, and shows each recalled note as a
// described link: id, summary and why.
func TestBriefPointsAtBrainSearchAndRecall(t *testing.T) {
	res := rrResolverWith(map[string]job.Source{"brain:project/r": rrNote("Retries use backoff.")})
	got := rrRender(t, rrJob(), res, job.Link{To: "brain:project/r", Rel: "uses", Why: "recalled: matches"})
	if !strings.Contains(got.Brief, "`grid brain search <words>`") {
		t.Fatalf("no brain search line:\n%s", got.Brief)
	}
	if !strings.Contains(got.Brief, "- brain:project/r · uses · Retries use backoff. · recalled: matches") {
		t.Fatalf("recall hit is not a described link:\n%s", got.Brief)
	}
	if bare := rrRender(t, rrJob(), rrResolverWith(nil)); !strings.Contains(bare.Brief, "`grid brain search <words>`") {
		t.Fatalf("no brain search line without recall:\n%s", bare.Brief)
	}
}
