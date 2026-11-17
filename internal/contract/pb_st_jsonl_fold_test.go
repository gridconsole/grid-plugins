package contract

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var pbStJSONLUpdate = flag.Bool("pbst.jsonl.update", false, "rewrite the JSONL fold golden")

// V9 regression: a JSONL home folds exactly as on base. The scenarios use only the
// API base has; the golden was written by base's FoldLoop and is read here.
func TestPbStJSONLFoldsAsOnBase(t *testing.T) {
	scenarios := map[string][]Post{}
	scenarios["base"] = base(t)
	running := append(base(t), side(t, "checkpoint", 4))
	scenarios["running"] = running
	unleased := base(t)
	unleased[1].Lease = nil
	scenarios["unleased control"] = unleased
	scenarios["accepted then cancelled"] = append(base(t), move(t, 4, Accepted), move(t, 5, Cancelled))
	reversed := append(base(t), move(t, 4, Accepted), move(t, 5, Rejected))
	reversed[3].Seq, reversed[4].Seq = 9, 8
	scenarios["seq out of order"] = reversed
	mv := side(t, "task-move", 6)
	scenarios["task-move"] = append(base(t), mv, side(t, "result", 7))
	dup := side(t, "ack", 8)
	dup.Job, dup.Attempt, dup.After = pointer("J-1"), pointer(1), base(t)[2].After
	scenarios["second ack"] = append(base(t), dup)

	got := map[string]any{}
	for name, posts := range scenarios {
		s, loop := FoldLoop(posts)
		got[name] = map[string]any{"state": s, "loop": loop}
	}
	data, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	golden := filepath.Join("testdata", "pb_st_jsonl_fold.golden.json")
	if *pbStJSONLUpdate {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("the JSONL fold differs from base's:\n%s", data)
	}
}
