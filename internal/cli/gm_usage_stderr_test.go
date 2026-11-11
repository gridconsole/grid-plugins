package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gmIsolate points the home folders at the test's own temp folder and refuses to
// go on when HOME is anywhere else.
func gmIsolate(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("GRID_HOME", filepath.Join(base, "grid"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(base, "claude"))
	t.Setenv("CODEX_HOME", filepath.Join(base, "codex"))
	if !strings.HasPrefix(os.Getenv("HOME"), base) {
		t.Fatal("HOME is not under the test folder: " + os.Getenv("HOME"))
	}
}

var gmMembers = []struct {
	name string
	args []string
}{
	{"deliver", []string{"deliver", "--outcome", "Done"}},
	{"proof add", []string{"proof", "add", "1", "--text", "Pass"}},
	{"proof index", []string{"proof", "index"}},
	{"history add", []string{"history", "add", "Checked the result"}},
}

func TestGmUsageOnBadFlag(t *testing.T) {
	for _, m := range gmMembers {
		t.Run(m.name, func(t *testing.T) {
			gmIsolate(t)
			h, dir := deliveryFixture(t)
			bad := append(append([]string{}, m.args...), "--gg-no-such-flag")
			if code := h.run(dir, bad...); code != 2 {
				t.Fatalf("plain exit %d: %s%s", code, h.out, h.errb)
			}
			plainErr := h.errb.String()
			if h.out.Len() != 0 {
				t.Fatalf("plain stdout not empty: %s", h.out)
			}
			if !strings.Contains(plainErr, "gg-no-such-flag") {
				t.Fatalf("stderr does not name the flag: %s", plainErr)
			}
			if code := h.run(dir, append(bad, "--json")...); code != 2 {
				t.Fatalf("json exit %d: %s%s", code, h.out, h.errb)
			}
			if h.errb.String() != plainErr {
				t.Fatalf("stderr differs with --json:\n%q\n%q", h.errb.String(), plainErr)
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), "invalid_argument")
		})
	}
}

func TestGmUsageOnHelp(t *testing.T) {
	for _, m := range gmMembers {
		t.Run(m.name, func(t *testing.T) {
			gmIsolate(t)
			h, dir := deliveryFixture(t)
			args := append(append([]string{}, strings.Fields(m.name)...), "--help")
			if code := h.run(dir, args...); code != 0 {
				t.Fatalf("exit %d: %s%s", code, h.out, h.errb)
			}
			if !strings.HasPrefix(h.out.String(), "Usage: grid ") {
				t.Fatalf("stdout = %q", h.out)
			}
			if h.errb.Len() != 0 {
				t.Fatalf("stderr = %q", h.errb)
			}
		})
	}
}
