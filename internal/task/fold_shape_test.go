package task

import (
	"path/filepath"
	"strings"
	"testing"
)

// Fold reads the migrated shape (id ULID, display T-NN) that grid migrate
// --apply and grid task new write, and still reads the legacy shape.
func TestFoldTaskShapes(t *testing.T) {
	const stable = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const rest = "title: Example\nproject: personal\nstate: queued\ncreated: 2026-01-02T03:04:05Z\n"
	path := filepath.Join("tasks", "T-07-example")
	for _, tc := range []struct {
		name, front, err string
	}{
		{"migrated", "id: " + stable + "\ndisplay: T-07\ncontract: \"1\"\n", ""},
		{"migrated with same ulid", "id: " + stable + "\ndisplay: T-07\nulid: " + stable + "\n", ""},
		{"legacy", "id: T-07\n", ""},
		{"legacy with ulid", "id: T-07\nulid: " + stable + "\n", ""},
		{"legacy without id", "", ""},
		{"legacy with display", "id: T-07\ndisplay: T-07\n", ""},
		{"migrated without display", "id: " + stable + "\n", "display"},
		{"migrated other display", "id: " + stable + "\ndisplay: T-08\n", "display"},
		{"migrated conflicting ulid", "id: " + stable + "\ndisplay: T-07\nulid: 01ARZ3NDEKTSV4RRFFQ69G5FAW\n", "conflicting"},
		{"legacy other display", "id: T-07\ndisplay: T-08\n", "display"},
		{"legacy other id", "id: T-08\n", "matching id"},
		{"not a ULID", "id: 01ARZ3NDEKTSV4RRFFQ69G5FA\ndisplay: T-07\n", "matching"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Fold(path, "---\n"+tc.front+rest+"---\n", IntentHead+"Words.\n")
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("Fold error %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil || got.ID != "T-07" || got.Title != "Example" || got.Intent != "Words.\n" {
				t.Fatalf("Fold: %+v, %v", got, err)
			}
		})
	}
}
