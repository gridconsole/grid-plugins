package orchestrator

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// The incident note a park keeps is evidence for later, and the brain refuses its topic in an
// install (grid brain propose: topic incidents is not in the first slice). A park whose incident
// note fails must still ask the person: the tick must not fail every pass and leave the Job
// as it was.
func TestParkAsksThePersonWhenTheIncidentNoteFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(t *testing.T, f *fixture)
	}{
		{"crash loop", func(t *testing.T, f *fixture) {
			for range 3 {
				f.ack(t)
				f.exit(t, "crash")
				f.tick(t)
			}
		}},
		{"empty finish", func(t *testing.T, f *fixture) { ewEmptyRounds(t, f, 3, false) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			var diag bytes.Buffer
			f.s.Diagnostics = &diag
			f.s.Incident = func(string, []string) error { return errors.New("topic incidents is not in the first slice") }
			f.job(t, "J-1", nil)
			f.tick(t)
			tc.run(t, f)
			if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" {
				t.Fatalf("status %s reason %q, want parked needs_person", j.Status, j.Reason)
			}
			qtPkQuestion(t, f)
			if !strings.Contains(diag.String(), "incident note for J-1 was not kept") {
				t.Fatalf("diagnostics %q name no failed incident note", diag.String())
			}
		})
	}
}
