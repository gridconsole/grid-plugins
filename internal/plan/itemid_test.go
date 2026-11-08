package plan

import "testing"

func TestItemID(t *testing.T) {
	for ref, want := range map[string]string{
		"1": "i-1", "i-1": "i-1", "item-1": "i-1", "007": "i-7", "i-12": "i-12", "item-3": "i-3",
		"": "", "0": "", "i-0": "", "i-": "", "item-i-1": "", "i-item-1": "", "-1": "", "+1": "", "1.5": "", "x": "", "_task": "",
		"99999999999999999999999": "",
	} {
		got, ok := ItemID(ref)
		if got != want || ok != (want != "") {
			t.Errorf("ItemID(%q) = %q, %v; want %q", ref, got, ok, want)
		}
	}
}

// proof: is a third step key next to kind: and jobs:, in any order; a proof
// line before them is part of the YAML part, never prose.
func TestItemsProofKeyInAnyOrder(t *testing.T) {
	for name, block := range map[string]string{
		"proof first":  "  proof: screenshot\n  kind: build\n  jobs: []\n",
		"proof middle": "  kind: build\n  proof: screenshot\n  jobs: []\n",
		"proof last":   "  kind: build\n  jobs: []\n  proof: screenshot\n",
	} {
		t.Run(name, func(t *testing.T) {
			items, err := Items("# Plan 1\n- [ ] 1 Capture the result\n  Some prose first.\n" + block)
			if err != nil || len(items) != 1 {
				t.Fatal(items, err)
			}
			if it := items[0]; it.Proof != "screenshot" || it.Kind != "build" || it.Prose != "Some prose first." || len(it.Jobs) != 0 {
				t.Fatalf("%+v", it)
			}
		})
	}
	for name, block := range map[string]string{
		"not a string": "  proof: [check]\n  jobs: []\n",
		"no jobs":      "  proof: check\n",
		"unknown key":  "  proof: check\n  jobs: []\n  other: 1\n",
		"prose after":  "  proof: check\n  jobs: []\n",
	} {
		t.Run("refused "+name, func(t *testing.T) {
			text := "# Plan 1\n- [ ] 1 Capture the result\n" + block
			if name == "prose after" {
				text += "  trailing: prose\n"
			}
			if _, err := Items(text); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
