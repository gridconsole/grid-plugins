package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func personBody(t *testing.T, id string, after []any) string {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = id
	if after != nil {
		body["after"] = after
	}
	out, _ := json.Marshal(body)
	return string(out)
}

// A person's Job holds only the Jobs behind it and is released by job result.
func TestPersonJobPostNextResult(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-3", "--body", personBody(t, "J-3", []any{"J-2"})); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	h.out.Reset()
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	var rows []struct{ ID, State string }
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatal(err, h.out.String())
	}
	state := map[string]string{}
	for _, r := range rows {
		state[r.ID] = r.State
	}
	if state["J-1"] != "assigned" || state["J-2"] != "posted" || state["J-3"] != "posted" {
		t.Fatalf("%v", state)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.Jobs["J-2"].Role != "Person" {
		t.Fatalf("role %q", st.Jobs["J-2"].Role)
	}
	h.out.Reset()
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Call it Lark."}`); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if st, _ = store.State(); string(st.Jobs["J-2"].Status) != "accepted" {
		t.Fatalf("J-2 is %s", st.Jobs["J-2"].Status)
	}
	h.out.Reset()
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if st, _ = store.State(); string(st.Jobs["J-3"].Status) != "assigned" {
		t.Fatalf("J-3 is %s", st.Jobs["J-3"].Status)
	}
	r, _ := store.Read(feed.Filter{})
	said := false
	for _, e := range r.Records {
		text, _ := e.Body["text"].(string)
		said = said || e.Type == "post" && strings.Contains(text, "member answered J-2 at ") && strings.Contains(text, "Call it Lark.")
	}
	if !said {
		t.Fatal("who, when and the answer are not in the Feed")
	}
}
