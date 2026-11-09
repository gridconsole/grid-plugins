package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCheckCLI(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		fmt.Fprint(w, `{"version":"99.0.0","contract":"2","app_min":"2.0.0"}`)
	}))
	defer s.Close()
	t.Setenv("GRID_RELEASE_MANIFEST_URL", s.URL)
	t.Setenv("GRID_APP_VERSION", "0.0.1")
	t.Setenv("GRID_SESSION_ID", "example-session")
	h := newHarness(t)
	if code := h.run(h.tmp, "version", "--check", "--json"); code != 0 {
		t.Fatalf("%d: %s", code, h.errb)
	}
	var r struct {
		Check struct {
			Status string `json:"status"`
			Cached bool   `json:"cached"`
		} `json:"check"`
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Check.Status != "update available" || r.Warning == "" || hits != 1 {
		t.Fatalf("%s hits=%d", h.out, hits)
	}
	if code := h.run(h.tmp, "version", "--check"); code != 0 || hits != 1 || strings.Contains(h.out.String(), "Update the app.") {
		t.Fatalf("%d: %s hits=%d", code, h.out, hits)
	}
	if code := h.run(h.tmp, "version", "--check", "--manifest-url", "file:///manifest"); code != 0 || !strings.Contains(h.out.String(), "unavailable") || hits != 1 {
		t.Fatalf("%d: %s", code, h.out)
	}
	if code := h.run(h.tmp, "version", "--check", "--timeout", "0s"); code != 2 {
		t.Fatalf("%d: %s", code, h.errb)
	}
}

func healthyDoctorEnvironment(t *testing.T, h *harness) {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("GRID_PROJECTS", "")
	if code := h.run(h.tmp, "setup", "--yes"); code != 0 {
		t.Fatalf("setup exit %d: %s", code, h.errb)
	}
	root := filepath.Join(h.root, "plugin")
	h.write(filepath.Join(root, "hooks", "hooks.json"), `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"grid hook sessionstart"}]}]}}`)
	bin := filepath.Join(h.root, "bin")
	h.write(filepath.Join(bin, "grid"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(bin, "grid"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", root)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestHookInvocationErrorsAreSilent(t *testing.T) {
	for _, args := range [][]string{{"hook", "sessionstart", "--bad"}, {"hook", "--bad"}, {"hook", "sessionstart"}} {
		h := newHarness(t)
		h.env.Stdin = strings.NewReader("invalid json")
		if code := h.run(h.tmp, args...); code != 0 || h.out.Len() != 0 || h.errb.Len() != 0 {
			t.Fatalf("%v exit=%d stdout=%s stderr=%s", args, code, h.out, h.errb)
		}
	}
}

func TestDoctorFindsHooksFromThePluginRoot(t *testing.T) {
	const hooks = `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"grid hook sessionstart"}]}]}}`
	for _, layout := range []string{"bin", "dist"} {
		t.Run(layout, func(t *testing.T) {
			h := newHarness(t)
			h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
			healthyDoctorEnvironment(t, h)
			t.Setenv("CLAUDE_PLUGIN_ROOT", "")
			root := filepath.Join(h.root, "checkout")
			h.write(filepath.Join(root, "hooks", "hooks.json"), hooks)
			h.write(filepath.Join(root, layout, "grid"), "")
			h.env.Executable = func() (string, error) { return filepath.Join(root, layout, "grid"), nil }
			// h.tmp holds no hooks folder: the current folder must not matter.
			if code := h.run(h.tmp, "doctor"); code != 0 {
				t.Fatalf("exit %d: %s%s", code, h.out, h.errb)
			}
		})
	}
	t.Run("override", func(t *testing.T) {
		h := newHarness(t)
		h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
		healthyDoctorEnvironment(t, h)
		t.Setenv("CLAUDE_PLUGIN_ROOT", "")
		h.env.Executable = func() (string, error) { return filepath.Join(h.root, "nowhere", "grid"), nil }
		if code := h.run(h.tmp, "doctor"); code != 1 {
			t.Fatalf("a missing hooks file must fail: exit %d: %s", code, h.out)
		}
		file := filepath.Join(h.root, "plugin", "hooks", "hooks.json")
		if code := h.run(h.tmp, "doctor", "--hooks-file", file); code != 0 {
			t.Fatalf("exit %d: %s%s", code, h.out, h.errb)
		}
	})
}
