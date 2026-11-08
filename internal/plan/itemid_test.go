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
