package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestClaudeAllowStatusUsesSetupRulesReadOnly(t *testing.T) {
	f := newClaudeFixture(t)
	f.o.GridHome = filepath.Join(f.o.Home, "data", "grid")
	f.o.Yes = true
	f.wantCode(0)
	rules, err := ClaudeBlock(f.o.Home, f.o.GridHome)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, rule := range rules {
		if rule.Section == "permissions" && rule.Key == "allow" {
			want = append(want, rule.Value)
		}
	}
	before := f.read(f.path)
	info, err := os.Stat(f.path)
	if err != nil {
		t.Fatal(err)
	}
	status := ReadClaudeAllowStatus(f.path, f.o.Home, f.o.GridHome)
	if status.Reason != "" || len(status.Missing) != 0 || !slices.Equal(status.Present, want) {
		t.Fatalf("setup rules not recognized: %+v", status)
	}
	after, err := os.Stat(f.path)
	if err != nil || !os.SameFile(info, after) || !info.ModTime().Equal(after.ModTime()) || info.Mode() != after.Mode() || !bytes.Equal(before, f.read(f.path)) {
		t.Fatal("status changed settings")
	}
	// Exact rule membership matters; a broad or similar permission is not the block.
	for _, missing := range want {
		doc, err := parseClaudeObject(before)
		if err != nil {
			t.Fatal(err)
		}
		if err := changeClaudeRule(&doc, ClaudeRule{"permissions", "allow", missing}, true); err != nil {
			t.Fatal(err)
		}
		if err := changeClaudeRule(&doc, ClaudeRule{"permissions", "allow", missing + "extra"}, false); err != nil {
			t.Fatal(err)
		}
		f.write(f.path, string(doc.bytes()))
		status := ReadClaudeAllowStatus(f.path, f.o.Home, f.o.GridHome)
		if status.Reason != "" || !slices.Equal(status.Missing, []string{missing}) || len(status.Present) != len(want)-1 {
			t.Fatalf("missing %s: %+v", missing, status)
		}
	}
}

func TestClaudeAllowStatusInvalidStructure(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `{`, `{} {}`, `{"permissions":null}`,
		`{"permissions":{"allow":null}}`, `{"permissions":{"allow":"Bash(grid *)"}}`,
		`{"permissions":{"allow":[null]}}`, `{"permissions":{"allow":[1]}}`,
		`{"permissions":{},"permissions":{}}`, `{"permissions":{"allow":[],"allow":[]}}`,
	} {
		t.Run(body, func(t *testing.T) {
			f := newClaudeFixture(t)
			f.write(f.path, body)
			status := ReadClaudeAllowStatus(f.path, f.o.Home, f.o.GridHome)
			if status.Reason != "Claude settings file is not valid JSON or has an invalid structure" {
				t.Fatalf("invalid settings: %+v", status)
			}
			if string(f.read(f.path)) != body {
				t.Fatal("invalid settings changed")
			}
		})
	}
}

func TestClaudeAllowStatusFileFailures(t *testing.T) {
	for _, kind := range []string{"missing", "symlink", "dangling symlink", "directory", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			f := newClaudeFixture(t)
			want := "Claude settings file is missing"
			switch kind {
			case "symlink", "dangling symlink":
				f.write(f.path, `{}`)
				target := filepath.Join(f.o.Home, "target.json")
				if kind == "symlink" {
					f.write(target, `{}`)
				}
				if err := os.Remove(f.path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, f.path); err != nil {
					t.Fatal(err)
				}
				want = "Claude settings file is a symlink"
			case "directory":
				if err := os.MkdirAll(f.path, 0700); err != nil {
					t.Fatal(err)
				}
				want = "Claude settings file is not a regular file"
			case "unreadable":
				if os.Geteuid() == 0 {
					t.Skip("root can read files without read permission")
				}
				f.write(f.path, `{}`)
				if err := os.Chmod(f.path, 0); err != nil {
					t.Fatal(err)
				}
				want = "cannot read the Claude settings file"
			}
			status := ReadClaudeAllowStatus(f.path, f.o.Home, f.o.GridHome)
			if status.Reason != want || strings.Contains(status.Reason, f.o.Home) {
				t.Fatalf("unexpected reason: %+v", status)
			}
			if kind == "missing" {
				if _, err := os.Lstat(f.path); !os.IsNotExist(err) {
					t.Fatal("status created missing settings")
				}
			}
		})
	}
}
