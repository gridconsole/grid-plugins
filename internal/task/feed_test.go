package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFeedIdentity(t *testing.T) {
	dir := t.TempDir()
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, fields := range []string{"id: " + id, "id: T-01\nulid: " + id} {
		if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("---\n"+fields+"\nowner: owner\n---\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, owner, err := FeedIdentity(dir)
		if err != nil || got != id || owner != "owner" {
			t.Fatal(got, owner, err)
		}
	}
}
