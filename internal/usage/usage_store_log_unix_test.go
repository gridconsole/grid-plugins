//go:build unix

package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsageStoreShowUsageIsFile(t *testing.T) {
	s := testStore(t)
	if err := os.WriteFile(filepath.Join(s.TaskPath, "usage"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.Show()
	usWantPrefix(t, err, "open ")
}

func TestUsageStoreLogIsFolder(t *testing.T) {
	s := testStore(t)
	if err := os.MkdirAll(usLogPath(s), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := s.Show()
	usWantPrefix(t, err, "read ")
}
