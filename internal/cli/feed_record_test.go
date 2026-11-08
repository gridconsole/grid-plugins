package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

func TestFeedPlanJobRecordRoundTrip(t *testing.T) {
	for _, name := range []string{"defaults", "optional planned fields"} {
		t.Run(name, func(t *testing.T) {
			h, task := feedHarness(t)
			plan, err := os.ReadFile("../../testdata/job/task/plan.md")
			if err != nil {
				t.Fatal(err)
			}
			h.write(filepath.Join(task, "plan.md"), string(plan))
			j, err := job.LoadPlan(task, "J-1")
			if err != nil {
				t.Fatal(err)
			}
			if name == "optional planned fields" {
				j.Kind, j.Visible, j.Irreversible, j.ReviewReason = "build", "big", true, "The owner approved the review exception."
			}
			body, err := j.Record()
			if err != nil {
				t.Fatal(err)
			}
			if err := contract.Validate("job-record", body); err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal(body, &want); err != nil {
				t.Fatal(err)
			}
			if code := h.run(task, "feed", "post", "--type", "job", "--job", j.ID, "--body", string(body), "--json"); code != 0 {
				t.Fatalf("post: %d %s", code, h.errb)
			}
			var posted struct {
				Post feed.Record `json:"post"`
			}
			if err := json.Unmarshal(h.out.Bytes(), &posted); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(posted.Post.Body, want) {
				t.Fatalf("posted body = %#v, want %#v", posted.Post.Body, want)
			}
			if code := h.run(task, "feed", "read", "--type", "job", "--json"); code != 0 {
				t.Fatalf("read: %d %s", code, h.errb)
			}
			var read feed.ReadResult
			if err := json.Unmarshal(h.out.Bytes(), &read); err != nil {
				t.Fatal(err)
			}
			if len(read.Bad) != 0 || len(read.Records) != 1 || read.Records[0].Verdict != "counted" || !reflect.DeepEqual(read.Records[0].Body, want) {
				t.Fatalf("persisted record = %+v", read)
			}
			if code := h.run(task, "feed", "state", "--json"); code != 0 {
				t.Fatalf("state: %d %s", code, h.errb)
			}
			var state contract.State
			if err := json.Unmarshal(h.out.Bytes(), &state); err != nil {
				t.Fatal(err)
			}
			folded := state.Jobs[j.ID]
			if folded == nil || folded.ID != "J-1" || folded.Title != j.Title || folded.Role != j.Role || folded.Status != "posted" || len(folded.History) != 1 || folded.History[0] != posted.Post.ID {
				t.Fatalf("folded Job = %+v", folded)
			}
			if len(state.Verdicts) != 1 || state.Verdicts[0].Kind != "counted" {
				t.Fatalf("fold verdicts = %+v", state.Verdicts)
			}
		})
	}
}
