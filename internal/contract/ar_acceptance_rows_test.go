package contract

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// arJob is the job fixture with five acceptance rows and no states.
func arJob(t *testing.T) []Post {
	t.Helper()
	posts := base(t)
	changeBody(t, &posts[0], func(v map[string]any) {
		rows := []any{}
		for i := 1; i <= 5; i++ {
			rows = append(rows, map[string]any{"id": fmt.Sprintf("A%d", i), "text": fmt.Sprintf("Row %d holds", i), "traces": "item"})
		}
		v["acceptance"] = rows
	})
	return posts
}

// arMark is a checkpoint of the running tenure that marks one acceptance row.
func arMark(t *testing.T, id int, row, state, note string) Post {
	t.Helper()
	p := side(t, "checkpoint", id)
	mark := map[string]any{"id": row, "state": state}
	if note != "" {
		mark["note"] = note
	}
	body(t, &p, map[string]any{"job": "J-1", "acceptance": mark})
	return p
}

func arMet(rows []AcceptanceRow) (met, total int) {
	for _, r := range rows {
		if r.State == RowDropped {
			continue
		}
		total++
		if r.State == RowMet {
			met++
		}
	}
	return met, total
}

// A Job with five rows, three marked met while it runs, reads 3 of 5 from the
// fold; a later mark of a row wins, not-met reads not met, and the step
// checkpoint a later attempt continues from is left as it was.
func TestArFiveRowsThreeMetReadThreeOfFive(t *testing.T) {
	step := side(t, "checkpoint", 4)
	posts := append(arJob(t), step,
		arMark(t, 5, "A1", "met", "go test ./x exit 0"),
		arMark(t, 6, "A2", "met", ""),
		arMark(t, 7, "A4", "not-met", "fails on Windows"),
		arMark(t, 8, "A4", "met", "fixed in abc1234"),
		arMark(t, 9, "A5", "not-met", ""),
	)
	s := Fold(posts)
	j := s.Jobs["J-1"]
	if j.Status != Running {
		t.Fatalf("status %s, want running", j.Status)
	}
	want := []AcceptanceRow{
		{ID: "A1", State: "met", Note: "go test ./x exit 0"},
		{ID: "A2", State: "met"},
		{ID: "A3", State: "pending"},
		{ID: "A4", State: "met", Note: "fixed in abc1234"},
		{ID: "A5", State: "not met"},
	}
	if !reflect.DeepEqual(j.Acceptance, want) {
		t.Fatalf("acceptance\n got %+v\nwant %+v", j.Acceptance, want)
	}
	if met, total := arMet(j.Acceptance); met != 3 || total != 5 {
		t.Fatalf("read %d of %d, want 3 of 5", met, total)
	}
	if j.Checkpoint == nil || j.Checkpoint.Step != "Checks complete" || j.Checkpoint.Commit == "" {
		t.Fatalf("a row mark replaced the step checkpoint: %+v", j.Checkpoint)
	}
	for id := 5; id <= 9; id++ {
		if v := verdict(s, fmt.Sprintf("%026d", id)); v.Kind != "counted" {
			t.Fatalf("mark %d: verdict %+v, want counted", id, v)
		}
		if !contains(j.History, fmt.Sprintf("%026d", id)) {
			t.Fatalf("mark %d not in history %v", id, j.History)
		}
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate("cmd-feed-state", data); err != nil {
		t.Fatalf("cmd-feed-state: %v", err)
	}
}

// An old Job record has rows without states and an old checkpoint has a step:
// the rows read pending and the step checkpoint folds as before.
func TestArOldRecordReadsPending(t *testing.T) {
	posts := append(base(t), side(t, "checkpoint", 4))
	j := Fold(posts).Jobs["J-1"]
	if want := []AcceptanceRow{{ID: "A1", State: "pending"}}; !reflect.DeepEqual(j.Acceptance, want) {
		t.Fatalf("acceptance %+v, want %+v", j.Acceptance, want)
	}
	if j.Checkpoint == nil || j.Checkpoint.Step != "Checks complete" {
		t.Fatalf("checkpoint %+v", j.Checkpoint)
	}
	// A record row the planner dropped reads dropped and does not count.
	posts = base(t)
	changeBody(t, &posts[0], func(v map[string]any) {
		v["acceptance"] = []any{
			map[string]any{"id": "A1", "text": "Kept", "traces": "item"},
			map[string]any{"id": "A2", "text": "Dropped", "traces": "item", "state": "dropped"},
		}
	})
	j = Fold(posts).Jobs["J-1"]
	if met, total := arMet(j.Acceptance); met != 0 || total != 1 || j.Acceptance[1].State != RowDropped {
		t.Fatalf("acceptance %+v: %d of %d", j.Acceptance, met, total)
	}
}

// A mark of a row the Job record does not have is ignored with its reason and
// changes nothing.
func TestArUnknownRowIgnored(t *testing.T) {
	posts := append(arJob(t), arMark(t, 4, "A9", "met", ""))
	s := Fold(posts)
	if v := verdict(s, fmt.Sprintf("%026d", 4)); v.Kind != "ignored" || !strings.Contains(v.Reason, "A9") {
		t.Fatalf("verdict %+v, want ignored naming A9", v)
	}
	j := s.Jobs["J-1"]
	if met, total := arMet(j.Acceptance); met != 0 || total != 5 {
		t.Fatalf("read %d of %d, want 0 of 5", met, total)
	}
	if contains(j.History, fmt.Sprintf("%026d", 4)) {
		t.Fatal("an ignored mark is in the history")
	}
}

// A mark outside a tenure (no ack) does not count, like every Job-side post.
func TestArMarkNeedsTheTenure(t *testing.T) {
	posts := append(arJob(t)[:2], arMark(t, 4, "A1", "met", ""))
	s := Fold(posts)
	if s.Jobs["J-1"].Acceptance[0].State != RowPending {
		t.Fatalf("a mark before ack counted: %+v", s.Jobs["J-1"].Acceptance)
	}
}

// The result's row states are the latest: they overwrite the marks.
func TestArResultStatesAreLatest(t *testing.T) {
	r := side(t, "result", 6)
	changeBody(t, &r, func(v map[string]any) {
		v["acceptance"] = []any{
			map[string]any{"id": "A1", "state": "met"},
			map[string]any{"id": "A2", "state": "dropped", "decision": "decisions/d1"},
			map[string]any{"id": "A7", "state": "met"},
		}
	})
	posts := append(arJob(t), arMark(t, 4, "A1", "not met", ""), arMark(t, 5, "A2", "met", ""), r)
	s := Fold(posts)
	if v := verdict(s, r.ID); v.Kind != "counted" {
		t.Fatalf("result verdict %+v", v)
	}
	j := s.Jobs["J-1"]
	if j.Acceptance[0].State != RowMet || j.Acceptance[1].State != RowDropped || j.Acceptance[1].Note != "decisions/d1" || len(j.Acceptance) != 5 {
		t.Fatalf("acceptance %+v", j.Acceptance)
	}
}

// The row-mark body is a checkpoint branch of its own: a dropped state, an
// unknown key or a missing state is refused, and the step branch is unchanged.
func TestArCheckpointRowSchema(t *testing.T) {
	good := arMark(t, 4, "A1", "met", "proved")
	data, _ := json.Marshal(good)
	if err := Validate("checkpoint", data); err != nil {
		t.Fatalf("row mark refused: %v", err)
	}
	for name, mark := range map[string]map[string]any{
		"dropped":   {"id": "A1", "state": "dropped"},
		"no state":  {"id": "A1"},
		"bad id":    {"id": "R1", "state": "met"},
		"extra key": {"id": "A1", "state": "met", "by": "me"},
	} {
		p := side(t, "checkpoint", 4)
		body(t, &p, map[string]any{"job": "J-1", "acceptance": mark})
		data, _ := json.Marshal(p)
		if Validate("checkpoint", data) == nil {
			t.Errorf("%s: row mark accepted", name)
		}
	}
	p := side(t, "checkpoint", 4)
	body(t, &p, map[string]any{"job": "J-1", "step": "tests", "commit": "aaaaaaa", "acceptance": map[string]any{"id": "A1", "state": "met"}})
	data, _ = json.Marshal(p)
	if Validate("checkpoint", data) == nil {
		t.Error("a step checkpoint with a row mark was accepted")
	}
}
