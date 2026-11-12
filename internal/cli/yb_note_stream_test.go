package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

const ybStEvidence = "../account/testdata/yb_stream_evidence.log"

// ybStSnapshotPath is the snapshot file of the folder dir (account/windows.go snapshotPath).
func ybStSnapshotPath(h *harness, dir string) string {
	sum := sha256.Sum256([]byte(gridhome.Real(dir)))
	return filepath.Join(h.gridHome, "accounts", "windows", hex.EncodeToString(sum[:4])+".json")
}

func TestYbStAccountNoteStreamKeepsTheWindowsAndPrintsNothing(t *testing.T) {
	h := accountHarness(t, "")
	dir := filepath.Join(h.home, ".claude-yb")
	h.mkdir(dir)
	if code := h.run(h.home, "account", "note", "--dir", dir, "--stream", ybStEvidence); code != 0 || h.out.Len() != 0 || h.errb.Len() != 0 {
		t.Fatalf("note --stream: %d out=%q err=%q", code, h.out.String(), h.errb.String())
	}
	data, err := os.ReadFile(ybStSnapshotPath(h, dir))
	if err != nil || !strings.Contains(string(data), `"source":"stream"`) {
		t.Fatalf("%v: %s", err, data)
	}
	if code := h.run(h.home, "account", "windows", "--json"); code != 0 || !strings.Contains(h.out.String(), `"source":"none"`) {
		t.Fatalf("the default folder has no data: %d %s", code, h.out.String())
	}
}

func TestYbStAccountNoteStreamMissingFileExitsOne(t *testing.T) {
	h := accountHarness(t, "")
	dir := filepath.Join(h.home, ".claude-yb")
	code := h.run(h.home, "account", "note", "--dir", dir, "--stream", filepath.Join(h.tmp, "nope.log"))
	if code != 1 || !strings.HasPrefix(h.errb.String(), "grid account note: ") {
		t.Fatalf("%d %q", code, h.errb.String())
	}
	if _, err := os.Stat(ybStSnapshotPath(h, dir)); !os.IsNotExist(err) {
		t.Fatalf("a snapshot was written: %v", err)
	}
}
