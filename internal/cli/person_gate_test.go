package cli

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fileBytes(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.Type().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out[rel] = sha256.Sum256(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPersonAnswerAuthorship(t *testing.T) {
	h, dir := feedHarness(t)
	q := questionAdd(t, h, dir, "member")
	t.Setenv("GRID_ACTOR", "reviewer")
	t.Setenv("GRID_ROLE", "owner")
	if code := h.run(dir, "answer", dir, q.ID, "Yes", "--json"); code != 0 {
		t.Fatalf("%d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), `"who":"reviewer"`) || !strings.Contains(h.out.String(), `"role":"owner"`) {
		t.Fatal(h.out)
	}
}
