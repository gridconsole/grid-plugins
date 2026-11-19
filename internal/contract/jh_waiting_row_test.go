package contract

import (
	"encoding/json"
	"reflect"
	"testing"
)

// jhResult is a result post of J-1 with the given acceptance rows.
func jhResult(t *testing.T, id int, rows ...any) Post {
	t.Helper()
	r := side(t, "result", id)
	changeBody(t, &r, func(v map[string]any) { v["acceptance"] = rows })
	return r
}

func jhValid(t *testing.T, schema string, v any) error {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return Validate(schema, data)
}

// A result row in the waiting state of docs/decisions/D-job-hold.md is
// accepted with its waits_on, and refused without it or with a bad waits_on.
func TestJhResultWaitingRowSchema(t *testing.T) {
	for _, kind := range []string{WaitJob, WaitLanded, WaitQuestion, WaitPermission} {
		row := map[string]any{"id": "A1", "state": RowWaiting, "waits_on": map[string]any{"kind": kind, "ref": "J-6"}}
		if err := jhValid(t, "result", jhResult(t, 6, row)); err != nil {
			t.Fatalf("waiting row on %s refused: %v", kind, err)
		}
	}
	for name, row := range map[string]map[string]any{
		"no waits_on":   {"id": "A1", "state": "waiting"},
		"no kind":       {"id": "A1", "state": "waiting", "waits_on": map[string]any{"ref": "J-6"}},
		"no ref":        {"id": "A1", "state": "waiting", "waits_on": map[string]any{"kind": "landed"}},
		"empty ref":     {"id": "A1", "state": "waiting", "waits_on": map[string]any{"kind": "landed", "ref": ""}},
		"unknown kind":  {"id": "A1", "state": "waiting", "waits_on": map[string]any{"kind": "merge", "ref": "J-6"}},
		"extra key":     {"id": "A1", "state": "waiting", "waits_on": map[string]any{"kind": "job", "ref": "J-6", "row": "A1"}},
		"waits_on list": {"id": "A1", "state": "waiting", "waits_on": []any{map[string]any{"kind": "job", "ref": "J-6"}}},
	} {
		if jhValid(t, "result", jhResult(t, 6, row)) == nil {
			t.Errorf("%s: waiting row accepted", name)
		}
	}
}

// The checkpoint row mark and the feed state take the waiting state too.
func TestJhCheckpointAndFeedStateTakeWaiting(t *testing.T) {
	if err := jhValid(t, "checkpoint", arMark(t, 4, "A1", RowWaiting, "waits on landed J-6")); err != nil {
		t.Fatalf("waiting row mark refused: %v", err)
	}
	s := Fold(append(arJob(t), arMark(t, 4, "A2", RowWaiting, "")))
	if got := s.Jobs["J-1"].Acceptance[1].State; got != RowWaiting {
		t.Fatalf("row mark folded to %q, want waiting", got)
	}
	j := s.Jobs["J-1"]
	j.Acceptance[1].WaitsOn = &Wait{Kind: WaitLanded, Ref: "J-6"}
	s.Jobs["J-1"] = j
	if err := jhValid(t, "cmd-feed-state", s); err != nil {
		t.Fatalf("cmd-feed-state with a waiting row: %v", err)
	}
	j.Acceptance[1].WaitsOn = &Wait{Kind: "merge", Ref: "J-6"}
	if jhValid(t, "cmd-feed-state", s) == nil {
		t.Fatal("cmd-feed-state accepted an unknown wait kind")
	}
}

// A result with a waiting row validates, folds to RowWaiting and not to not
// met, and the row with its waits_on survives a JSON round trip.
func TestJhWaitingRowRoundTrip(t *testing.T) {
	r := jhResult(t, 6,
		map[string]any{"id": "A1", "state": "met"},
		map[string]any{"id": "A2", "state": "waiting", "waits_on": map[string]any{"kind": "landed", "ref": "J-6"}},
	)
	if err := jhValid(t, "result", r); err != nil {
		t.Fatalf("result refused: %v", err)
	}
	s := Fold(append(arJob(t), r))
	if v := verdict(s, r.ID); v.Kind != "counted" {
		t.Fatalf("result verdict %+v", v)
	}
	if got := s.Jobs["J-1"].Acceptance[1].State; got != RowWaiting {
		t.Fatalf("waiting row folded to %q", got)
	}
	want := AcceptanceRow{ID: "A2", State: RowWaiting, WaitsOn: &Wait{Kind: WaitLanded, Ref: "J-6"}}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"id":"A2","state":"waiting","waits_on":{"kind":"landed","ref":"J-6"}}` {
		t.Fatalf("marshalled %s", data)
	}
	var got AcceptanceRow
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip %+v, want %+v", got, want)
	}
	// A row that does not wait carries no waits_on key.
	if data, _ := json.Marshal(AcceptanceRow{ID: "A1", State: RowMet}); string(data) != `{"id":"A1","state":"met"}` {
		t.Fatalf("met row marshalled %s", data)
	}
}
