package worker

import "testing"

func TestListWithoutGridHome(t *testing.T) {
	entries, err := Store{GridHome: ""}.List()
	if err != nil {
		t.Fatal(err)
	}
	if entries == nil || len(entries) != 0 {
		t.Fatalf("entries: got %#v, want a non-nil empty slice", entries)
	}
}
