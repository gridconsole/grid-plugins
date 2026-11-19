package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exDenyHarness is the doctor harness after `grid setup --yes`, with the Grid
// home versioned so that doctor has nothing else to report.
func exDenyHarness(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
	healthyDoctorEnvironment(t, h)
	return h, filepath.Join(h.home, ".claude", "settings.json")
}

type exDenyFile struct {
	data  []byte
	mtime int64
}

func exDenyRead(t *testing.T, path string) exDenyFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return exDenyFile{data, info.ModTime().UnixNano()}
}

// exDenyDoctor runs doctor and fails the test when it changed the settings file.
func exDenyDoctor(t *testing.T, h *harness, path string, args ...string) (int, string) {
	t.Helper()
	old := exDenyRead(t, path)
	code := h.run(h.tmp, append([]string{"doctor"}, args...)...)
	now := exDenyRead(t, path)
	if !bytes.Equal(old.data, now.data) || old.mtime != now.mtime {
		t.Fatal("doctor changed the Claude settings file")
	}
	return code, h.out.String()
}

func exDenyHasLine(out, prefix string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func TestExDenyDoctorAfterSetup(t *testing.T) {
	h, path := exDenyHarness(t)
	code, out := exDenyDoctor(t, h, path)
	if code != 0 || !exDenyHasLine(out, "ok deny block: all setup deny rules are present") ||
		!exDenyHasLine(out, "ok sandbox: off; grid setup --sandbox turns on Claude Code's sandbox") {
		t.Fatalf("exit %d: %s", code, out)
	}
	if code := h.run(h.tmp, "setup", "--yes", "--sandbox"); code != 0 {
		t.Fatalf("setup --sandbox exit %d: %s", code, h.errb)
	}
	code, out = exDenyDoctor(t, h, path)
	if code != 0 || !exDenyHasLine(out, "ok deny block:") || !exDenyHasLine(out, "ok sandbox: on (grid setup --sandbox)") {
		t.Fatalf("exit %d: %s", code, out)
	}
}
