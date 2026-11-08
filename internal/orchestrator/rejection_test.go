package orchestrator

import (
	"errors"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestWriteInvalidRecord(t *testing.T) {
	for _, name := range []string{"unknown Job", "before ack", "exited", "schema"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			f.tick(t)
			j := f.state(t)
			f.worker.Fence = j.Assign.Fence
			f.worker.Gen = j.Generation
			w := f.worker
			in := feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()}
			want := ""
			switch name {
			case "unknown Job":
				w = f.s.Writer
				in = feed.Input{Type: "control", Body: map[string]any{"target": "J-999", "status": "cancelled"}}
				want = "unknown Job J-999"
			case "before ack":
				want = "Job tenure must start with ack"
			case "exited":
				f.ack(t)
				f.exit(t, "done")
				want = "attempt already exited"
			case "schema":
				f.ack(t)
				in.Body = map[string]any{}
				want = "$.body: oneOf matched 0 branches: $.body: missing required ref"
			}
			_, err := Write(f.s.Feed, w, in)
			if !errors.Is(err, feed.ErrInvalidRecord) || err.Error() != want {
				t.Fatalf("%v, want invalid record: %s", err, want)
			}
		})
	}
}
