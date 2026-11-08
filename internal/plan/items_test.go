package plan

import (
	"strings"
	"testing"
)

func TestChecklistBlocks(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"prose then YAML", "- [ ] 5 Composer\n  One prose line.\n  Another prose line.\n  kind: build\n  jobs:\n    - id: J-14\n", ""},
		{"jobs first", "- [ ] 5 Composer\n  jobs: []\n  kind: build\n", ""},
		{"missing jobs", "- [ ] 5 Composer\n  A description.\n", "missing jobs"},
		{"kind only", "- [ ] 5 Composer\n  kind: build\n", "jobs must be a list"},
		{"null jobs", "- [ ] 5 Composer\n  jobs: null\n", "jobs must be a list"},
		{"object jobs", "- [ ] 5 Composer\n  jobs: {id: J-14}\n", "jobs must be a list"},
		{"scalar job", "- [ ] 5 Composer\n  jobs: [J-14]\n", "each jobs entry must be an object"},
		{"prose after YAML", "- [ ] 5 Composer\n  jobs: []\n  Late prose.\n", "plan line"},
		{"blank inside block", "- [ ] 5 Composer\n  Description.\n\n  jobs: []\n", "missing jobs"},
		{"duplicate N", "- [ ] 5 First\n  jobs: []\n- [ ] 05 Second\n  jobs: []\n", "duplicate item 5"},
		{"unknown step field", "- [ ] 5 Composer\n  jobs: []\n  surprise: yes\n", "unknown step field"},
		{"duplicate YAML key", "- [ ] 5 Composer\n  jobs: []\n  jobs: []\n", "already"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Items(tc.text)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
	for _, field := range []string{"write", "after", "review", "risk", "verify"} {
		for _, prefix := range []string{"", "  jobs: []\n", "  jobs: []\n\n"} {
			_, err := Items("- [ ] 5 Composer\n" + prefix + "  " + field + ": []\n")
			if err == nil || !strings.Contains(err.Error(), "step-level "+field+":") || !strings.Contains(err.Error(), "plan line") {
				t.Fatalf("%s: %v", field, err)
			}
		}
	}
}

func TestDerivedJobsAndAnchors(t *testing.T) {
	text := "# Plan, rev 1 · 000000\n- [x] 5 Composer\n  First prose line.\n  Second prose line.\n  kind: build\n  jobs:\n    - id: J-14\n      intent: {owner: intent.md#request}\n    - id: J-15\n      item: 8\n      title: Explicit title\n      intent: {item: plan.md#step-8, goal: intent.md#goal}\n\n## Verify\n\nFirst paragraph.\nContinued paragraph.\n\nSecond paragraph.\n"
	items, err := Items(text)
	if err != nil {
		t.Fatal(err)
	}
	item := items[0]
	j := item.Jobs[0]
	intent := j["intent"].(map[string]any)
	if item.Kind != "build" || j["item"] != 5 || j["title"] != "Composer" || intent["item"] != "plan.md#step-5" || intent["goal"] != "plan.md#verify" || intent["owner"] != "intent.md#request" {
		t.Fatalf("%+v", item)
	}
	if item.Jobs[1]["item"] != 8 || item.Jobs[1]["title"] != "Explicit title" {
		t.Fatal("overwrote explicit values")
	}
	for anchor, want := range map[string]string{"step-5": "- [x] 5 Composer\n  First prose line.\n  Second prose line.", "verify": "First paragraph.\nContinued paragraph."} {
		got, err := Anchor(text, anchor)
		if err != nil || got != want {
			t.Fatalf("%s: %q %v", anchor, got, err)
		}
	}
	if _, err := Anchor(text, "step-99"); err == nil {
		t.Fatal("accepted missing anchor")
	}
	unticked := strings.ReplaceAll(text, "[x]", "[ ]")
	if Hash(text) != Hash(unticked) {
		t.Fatal("tick changed hash")
	}
	other, err := Items(unticked)
	if err != nil || other[0].State != item.State {
		t.Fatal("tick changed run state")
	}
}
