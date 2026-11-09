package cli

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/doctor"
)

func doctorAllowCheck(t *testing.T, h *harness) doctor.Check {
	t.Helper()
	h.run(h.tmp, "doctor", "--json")
	var report doctor.Report
	if err := json.Unmarshal(h.out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	for _, check := range report.Checks {
		if check.Name == "allow block" {
			return check
		}
	}
	t.Fatal("doctor JSON has no allow block check")
	return doctor.Check{}
}

func TestDoctorAllowBlockCLI(t *testing.T) {
	for _, tc := range []struct {
		name, body, detail string
	}{
		{"complete", `{"permissions":{"allow":["Bash(grid *)","Read(~/.grid/**)","Edit(~/.grid/**)"]}}`, ""},
		{"missing rule", `{"permissions":{"allow":["Bash(grid *)","Read(~/.grid/**)"]}}`, "missing rules: Edit(~/.grid/**)"},
		{"missing file", "", "Claude settings file is missing"},
		{"invalid JSON", "{broken", "Claude settings file is not valid JSON or has an invalid structure"},
		{"symlink", `{}`, "Claude settings file is a symlink"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := claudeHarness(t)
			t.Setenv("CLAUDE_CONFIG_DIR", "")
			// A decoy XDG file must never replace the setup destination.
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(h.root, "xdg"))
			h.write(filepath.Join(h.root, "xdg", "claude", "settings.json"), `{}`)
			file := filepath.Join(h.home, ".claude", "settings.json")
			if tc.body != "" {
				h.write(file, tc.body)
			}
			if tc.name == "symlink" {
				target := filepath.Join(h.root, "settings-target.json")
				if err := os.Rename(file, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, file); err != nil {
					t.Fatal(err)
				}
			}
			check := doctorAllowCheck(t, h)
			if check.OK != (tc.detail == "") || (tc.detail != "" && check.Detail != tc.detail) {
				t.Fatalf("unexpected allow block check: %+v", check)
			}
			if !check.OK && check.Fix != "Run grid setup to add the allow block; grid setup --remove takes it out." {
				t.Fatalf("missing fix hint: %+v", check)
			}
			if strings.Contains(h.out.String(), h.root) {
				t.Fatal("doctor JSON exposed an absolute path")
			}
			h.run(h.tmp, "doctor")
			if !strings.Contains(h.out.String(), "allow block:") || strings.Contains(h.out.String(), h.root) {
				t.Fatal("plain doctor output lacks the check or exposes an absolute path")
			}
			data, err := os.ReadFile(file)
			if tc.body == "" {
				if !os.IsNotExist(err) {
					t.Fatal("doctor created a missing settings file")
				}
			} else if err != nil || string(data) != tc.body {
				t.Fatal("doctor changed settings")
			}
		})
	}
}

func TestDoctorAllowBlockSetupRoundTrip(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "custom"}[custom], func(t *testing.T) {
			h := claudeHarness(t)
			if !custom {
				t.Setenv("CLAUDE_CONFIG_DIR", "")
			}
			h.env.GridHome = filepath.Join(h.home, "data", "grid")
			if code := h.run(h.tmp, "setup", "--yes"); code != 0 {
				t.Fatalf("setup exit %d: %s", code, h.errb)
			}
			if check := doctorAllowCheck(t, h); !check.OK || check.Fix != "" {
				t.Fatalf("setup block was not recognized: %+v", check)
			}
			if code := h.run(h.tmp, "setup", "--remove", "--yes"); code != 0 {
				t.Fatalf("remove exit %d: %s", code, h.errb)
			}
			check := doctorAllowCheck(t, h)
			for _, rule := range []string{"Bash(grid *)", "Read(~/data/grid/**)", "Edit(~/data/grid/**)"} {
				if check.OK || !strings.Contains(check.Detail, rule) {
					t.Fatalf("removed rule not reported: %s", rule)
				}
			}
		})
	}
}

func TestDoctorAllowBlockBundleRedactsSettingsPath(t *testing.T) {
	h := claudeHarness(t)
	// Keep the custom destination outside Home to exercise explicit redaction.
	dir := filepath.Join(h.root, "private-config")
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	file := filepath.Join(dir, "settings.json")
	h.write(filepath.Join(h.gridHome, "logs", "setup.log"), "settings="+file+"\n")
	if code := h.run(h.tmp, "doctor", "--bundle", "--out", "support.zip"); code != 0 {
		t.Fatalf("bundle exit %d: %s", code, h.errb)
	}
	archive, err := zip.OpenReader(strings.TrimSpace(h.out.String()))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, entry := range archive.File {
		f, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), file) {
			t.Fatalf("bundle %s exposed the settings path", entry.Name)
		}
		if entry.Name == "doctor.txt" && !strings.Contains(string(data), "allow block:") {
			t.Error("bundle doctor report has no allow block check")
		}
	}
}
