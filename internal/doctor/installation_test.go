package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const goodHooks = `{"hooks":{"SessionStart":[{"matcher":"startup|compact","hooks":[{"type":"command","command":"grid hook sessionstart","timeout":10}]}]}}`

func healthyHooks(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(goodHooks), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func gridOK(string) (string, error) { return "grid", nil }

func TestValidateHooks(t *testing.T) {
	for _, bad := range []string{`no json`, `null`, `{}`, `{"hooks":[]}`, `{"hooks":{"SessionStart":[]}}`,
		`{"hooks":{"SessionStart":[{"hooks":[]}]}}`,
		`{"hooks":{"SessionStart":[{"matcher":"[","hooks":[{"type":"command","command":"grid"}]}]}}`,
		`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":""}]}]}}`,
		`{"hooks":{"SessionStart":[{"hooks":[{"type":"prompt","command":"grid"}]}]}}`,
		`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"grid","timeout":-1}]}]}}`,
	} {
		if err := ValidateHooks([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	if err := ValidateHooks([]byte(goodHooks)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateHooks(b); err != nil {
		t.Fatalf("shipped hooks: %v", err)
	}
}

func TestInstallationChecksAndHints(t *testing.T) {
	home := t.TempDir()
	writeVersion(t, home, "1")
	path := healthyHooks(t)
	o := Options{GridHome: home, GitVersion: gitOK, HooksFile: path, LookPath: gridOK}
	rep := Run(o)
	if !rep.OK {
		t.Fatal(rep)
	}
	for _, c := range rep.Checks {
		if c.Fix != "" {
			t.Errorf("passing check has a fix: %+v", c)
		}
	}
	o.LookPath = func(string) (string, error) { return "", errors.New("missing") }
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertChecks(t, Run(o), map[string]string{"grid on PATH": "!not found", "plugin hooks": "!not valid JSON"}, false)
	for _, c := range Run(o).Checks {
		if c.OK == (c.Fix != "") {
			t.Errorf("fix must be set exactly on failing checks: %+v", c)
		}
	}
	o.HooksFile = filepath.Join(home, "missing")
	assertChecks(t, Run(o), map[string]string{"plugin hooks": "!cannot read"}, false)
}
