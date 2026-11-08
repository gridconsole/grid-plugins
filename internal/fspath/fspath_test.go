package fspath

import (
	"os"
	"path/filepath"
	"testing"
)

// foldsCase probes the volume instead of trusting the OS: a volume can be
// formatted either way.
func foldsCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "CaseProbe")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Stat(filepath.Join(dir, "caseprobe"))
	return err == nil
}

func TestOnDiskCase(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "Alpha", "Beta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := OnDiskCase(dir); got != dir {
		t.Errorf("exact spelling changed: %s", got)
	}
	missing := filepath.Join(dir, "nope", "Tail")
	if got := OnDiskCase(missing); got != missing {
		t.Errorf("missing tail changed: %s", got)
	}
	if !foldsCase(t, root) {
		t.Skip("case-sensitive filesystem: another spelling names a different path")
	}
	if got := OnDiskCase(filepath.Join(root, "ALPHA", "beta")); got != dir {
		t.Errorf("got %s, want %s", got, dir)
	}
	if got, want := OnDiskCase(filepath.Join(root, "alpha", "BETA", "New")), filepath.Join(dir, "New"); got != want {
		t.Errorf("missing tail: got %s, want %s", got, want)
	}
}
