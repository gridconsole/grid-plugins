package job_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// leRwOwnerHeld is the one writer of the retired fields that stays: the owner
// holds migrate.
const leRwOwnerHeld = "internal/migrate/receipt.go"

// TestLeRwNoRetiredWriters fails when a non-test Go file under internal/ other
// than the migrate receipt writes the retired review level or max_rounds into a
// Job record.
func TestLeRwNoRetiredWriters(t *testing.T) {
	hfJrTempHome(t)
	retired := []*regexp.Regexp{
		regexp.MustCompile(`"review":\s*"L[0-3]"`),
		regexp.MustCompile(`"max_rounds":\s*[0-9]`),
	}
	err := filepath.WalkDir(filepath.Join(hfJrRoot(), "internal"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(hfJrRoot(), p)
		rel = filepath.ToSlash(rel)
		if rel == leRwOwnerHeld {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, re := range retired {
			if re.Match(b) {
				t.Errorf("%s writes a retired Job field (%s)", rel, re)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
