package cli

import (
	"bytes"
	"encoding/json"
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

func TestExDenyDoctorMissingDenyRule(t *testing.T) {
	const rule = "Edit(~/.grid-person/**)"
	h, path := exDenyHarness(t)
	var doc map[string]map[string]any
	if err := json.Unmarshal(exDenyRead(t, path).data, &doc); err != nil {
		t.Fatal(err)
	}
	kept := []any{}
	for _, v := range doc["permissions"]["deny"].([]any) {
		if v != rule {
			kept = append(kept, v)
		}
	}
	if len(kept) != len(doc["permissions"]["deny"].([]any))-1 {
		t.Fatalf("setup did not write %s", rule)
	}
	doc["permissions"]["deny"] = kept
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	code, out := exDenyDoctor(t, h, path)
	if code == 0 || !strings.Contains(out, "fail deny block: missing rules: "+rule+"\nfix: Run grid setup\n") {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !exDenyHasLine(out, "ok allow block: all setup allow rules are present") {
		t.Fatalf("exit %d: %s", code, out)
	}
	// The JSON report names the check and its fix.
	code, out = exDenyDoctor(t, h, path, "--json")
	var rep struct {
		OK     bool `json:"ok"`
		Checks []struct {
			Name, Detail, Fix string
			OK                bool
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range rep.Checks {
		switch c.Name {
		case "deny block":
			found = !c.OK && c.Detail == "missing rules: "+rule && c.Fix == "Run grid setup"
		case "allow block":
			if !c.OK {
				t.Fatalf("allow block changed: %+v", c)
			}
		}
	}
	if code == 0 || rep.OK || !found {
		t.Fatalf("exit %d: %s", code, out)
	}
}
