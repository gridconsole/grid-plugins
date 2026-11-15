package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// grid routine add --tool names the tools of the source a skill routine reads;
// list shows them back, and a name that would split into two rules is refused.
func TestR23RoutineAddToolFlag(t *testing.T) {
	h := newHarness(t)
	args := []string{"routine", "add", "inbox", "--every", "1h", "--run", "signals", "--tool", "mcp__mail__search", "--tool", "mcp__mail__read", "--json"}
	if code := h.run(h.root, args...); code != 0 {
		t.Fatalf("add: exit %d: %s", code, h.errb)
	}
	var added struct {
		Tools []string `json:"tools"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &added); err != nil {
		t.Fatal(err)
	}
	want := []string{"mcp__mail__search", "mcp__mail__read"}
	if !reflect.DeepEqual(added.Tools, want) {
		t.Fatalf("add printed tools %q, want %q", added.Tools, want)
	}
	h.out.Reset()
	if code := h.run(h.root, "routine", "list", "--json"); code != 0 {
		t.Fatalf("list: exit %d: %s", code, h.errb)
	}
	var listed []struct {
		Tools []string `json:"tools"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &listed); err != nil || len(listed) != 1 || !reflect.DeepEqual(listed[0].Tools, want) {
		t.Fatalf("list tools %+v (%v), want %q", listed, err, want)
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(h.root, "routine", "add", "bad", "--every", "1h", "--run", "signals", "--tool", "mcp__a,Bash"); code == 0 || !strings.Contains(h.errb.String(), "invalid tools") {
		t.Fatalf("a tool with a comma: exit %d, stderr %q", code, h.errb.String())
	}
}
