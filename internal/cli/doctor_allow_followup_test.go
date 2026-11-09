package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/doctor"
)

func TestDoctorAllowBlockFollowups(t *testing.T) {
	for _, tc := range []struct {
		name, detail, fix string
		code              int
	}{
		{"symlink", "all setup allow rules are present", "", 0},
		{"dangling symlink", "Claude settings file is a dangling symlink", "Fix or move the Claude settings file, then run grid setup.", 1},
		{"directory target", "Claude settings file is not a regular file", "Fix or move the Claude settings file, then run grid setup.", 1},
		{"unreadable", "cannot read the Claude settings file", "Fix or move the Claude settings file, then run grid setup.", 1},
		{"missing home", "skipped: Grid home is unavailable", "", 1},
		{"invalid JSON", "Claude settings file is not valid JSON or has an invalid structure", "Fix or move the Claude settings file, then run grid setup.", 1},
		{"missing rules", "missing rules: Bash(grid *), Read(~/.grid/**), Edit(~/.grid/**)", "Run grid setup", 1},
		{"missing file", "Claude settings file is missing", "Run grid setup", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := claudeHarness(t)
			healthyDoctorEnvironment(t, h)
			h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
			file := filepath.Join(h.home, ".claude", "settings.json")
			switch tc.name {
			case "symlink", "dangling symlink", "directory target":
				target := filepath.Join(h.home, "dotfiles", "settings.json")
				h.mkdir(filepath.Dir(target))
				if err := os.Rename(file, target); err != nil {
					t.Fatal(err)
				}
				if tc.name != "symlink" {
					if err := os.Remove(target); err != nil {
						t.Fatal(err)
					}
					if tc.name == "directory target" {
						h.mkdir(target)
					}
				}
				if err := os.Symlink(filepath.Join("..", "dotfiles", "settings.json"), file); err != nil {
					t.Fatal(err)
				}
				if tc.name == "symlink" {
					// Force setup to update the target through the same link doctor reads.
					h.write(target, `{}`)
					if code := h.run(h.tmp, "setup", "--yes"); code != 0 {
						t.Fatalf("setup exit %d: %s", code, h.errb)
					}
					if st, err := os.Lstat(file); err != nil || st.Mode()&os.ModeSymlink == 0 {
						t.Fatal("setup replaced the settings symlink")
					}
				}
			case "unreadable":
				if os.Geteuid() == 0 {
					t.Skip("root can read files without read permission")
				}
				if err := os.Chmod(file, 0); err != nil {
					t.Fatal(err)
				}
			case "missing home":
				h.env.GridHome = filepath.Join(h.home, "absent")
			case "invalid JSON":
				h.write(file, `{broken`)
			case "missing rules":
				h.write(file, `{}`)
			case "missing file":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			if code := h.run(h.tmp, "doctor", "--json"); code != tc.code {
				t.Errorf("doctor exit %d, want %d", code, tc.code)
			}
			var report doctor.Report
			if err := json.Unmarshal(h.out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			failures, found := 0, false
			for _, check := range report.Checks {
				if !check.OK {
					failures++
				}
				if check.Name == "allow block" {
					found = true
					if check.Detail != tc.detail || check.Fix != tc.fix || check.OK != (tc.fix == "") {
						t.Errorf("unexpected allow check: %+v", check)
					}
				}
			}
			if !found || failures != tc.code || report.OK != (tc.code == 0) {
				t.Errorf("found=%v failures=%d report.OK=%v", found, failures, report.OK)
			}
			if code := h.run(h.tmp, "doctor"); code != tc.code || !strings.Contains(h.out.String(), "allow block: "+tc.detail) {
				t.Errorf("plain doctor exit %d: %s", code, h.out)
			}
		})
	}
}
