package plan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestNqNeedsItemsReadTheLine(t *testing.T) {
	text := "# Plan\n" +
		"- [ ] 1 One\n  needs: none\n  jobs: []\n" +
		"- [ ] 2 Two\n  needs: 1\n  jobs: []\n" +
		"- [ ] 3 Three\n  needs: 1, 3\n  jobs: []\n" +
		"- [ ] 4 Four\n  needs: 1,3\n  jobs: []\n" +
		"- [ ] 5 Five\n  Just prose.\n  jobs: []\n" +
		"- [ ] 6 Six\n  needs: original\n  jobs: []\n"
	items, err := Items(text)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{}, {1}, {1, 3}, {1, 3}, nil, nil}
	if len(items) != len(want) {
		t.Fatalf("got %d items", len(items))
	}
	for i, w := range want {
		if !reflect.DeepEqual(items[i].Needs, w) {
			t.Fatalf("item %d: Needs %#v, want %#v", i+1, items[i].Needs, w)
		}
	}
	if items[0].Needs == nil || items[4].Needs != nil || items[5].Needs != nil {
		t.Fatalf("none must be empty non-nil, a missing or malformed line nil")
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{`"needs":[]`, `"needs":[1]`, `"needs":[1,3]`, `"needs":null`} {
		if !strings.Contains(string(b), frag) {
			t.Fatalf("json lacks %s: %s", frag, b)
		}
	}
	if !strings.Contains(items[1].Prose, "needs: 1") {
		t.Fatalf("Prose lost the needs line: %q", items[1].Prose)
	}
	anchor, err := Anchor(text, "step-2")
	if err != nil || !strings.Contains(anchor, "needs: 1") {
		t.Fatalf("anchor %q, %v", anchor, err)
	}
}

func TestNqNeedsYamlPartStillRefusesNeeds(t *testing.T) {
	_, err := Items("- [ ] 1 One\n  jobs: []\n  needs: 1\n")
	if err == nil || !strings.Contains(err.Error(), "unknown step field") {
		t.Fatalf("got %v", err)
	}
}
