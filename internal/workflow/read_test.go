package workflow

import (
	"crypto/sha256"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/where"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowShowAllProjects(t *testing.T) {
	home := workflowHome(t)
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), rootText(""))
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "workflows.md"), projectText(labelKeys, twoSteps))
	got, err := Read(where.Resolver{GridHome: home}, "")
	if err != nil || len(got.Files) != 3 || len(got.Templates) != 2 || len(got.Errors) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	if got.Files[0].Project != nil || *got.Files[1].Project != "alpha" || *got.Files[2].Project != "beta" || got.Files[2].Rev != nil || got.Files[2].Text != "" {
		t.Fatalf("%+v", got.Files)
	}
	if got.Templates[0].File != "workflows.md" || got.Templates[1].File != "alpha/workflows.md" {
		t.Fatalf("%+v", got.Templates)
	}
}
func TestWorkflowShowOneProject(t *testing.T) {
	home := workflowHome(t)
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), rootText(""))
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "workflows.md"), projectText(labelKeys, twoSteps))
	for _, selector := range []string{"alpha", "id-alpha"} {
		got, err := Read(where.Resolver{GridHome: home}, selector)
		if err != nil || len(got.Files) != 2 || len(got.Templates) != 2 || len(got.Errors) != 0 || *got.Files[1].Project != "alpha" {
			t.Fatalf("%+v %v", got, err)
		}
	}
	if _, err := Read(where.Resolver{GridHome: home}, "unknown"); err == nil {
		t.Fatal("accepted unknown project")
	}
}

// grid-core/tests/workflows.rs:405-434; sample root replaces the unavailable original fixture.
func TestWorkflowBadSectionKeepsRest(t *testing.T) {
	good, err := os.ReadFile("testdata/root.md")
	if err != nil {
		t.Fatal(err)
	}
	broken := "## Broken\napplies when: label broken\nreview rounds: 2\n\n| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n| 1 | Wait | sleep | grid-builder |  |\n\n"
	text := strings.Replace(string(good), "## Web page", broken+"## Web page", 1)
	home := t.TempDir()
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), text)
	got, err := Read(where.Resolver{GridHome: home}, "")
	if err != nil || len(got.Templates) != 7 || len(got.Errors) != 1 || len(got.Files) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	line := strings.Count(text[:strings.Index(text, "| 1 | Wait")], "\n") + 1
	p := got.Errors[0]
	if p.File != "workflows.md" || p.Section == nil || *p.Section != "Broken" || p.Line == nil || *p.Line != line || p.Message != "\"sleep\" is not a step kind" {
		t.Fatalf("%+v", p)
	}
	if !strings.HasPrefix(p.String(), "workflows.md, section \"Broken\", line ") {
		t.Fatal(p.String())
	}
	if got.Files[0].Text != text || got.Files[0].Rev == nil || *got.Files[0].Rev != fmt.Sprintf("%x", sha256.Sum256([]byte(text)))[:12] {
		t.Fatalf("%+v", got.Files)
	}
}

func TestWorkflowReadLimitsAndErrors(t *testing.T) {
	for _, tc := range []struct{ name, text, message string }{{"limit", strings.Repeat(" ", 32768), "the file must start with \"# Workflows\""}, {"over", strings.Repeat(" ", 32769), "larger than 32768 bytes"}, {"large", strings.Repeat("x", 100000), "larger than 32768 bytes"}, {"UTF8", "\xff", "not UTF-8"}, {"NUL", "# Workflows\n\x00", "the file holds a NUL byte"}, {"empty", "", "the file must start with \"# Workflows\""}} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			putWorkflowFixture(t, filepath.Join(home, "workflows.md"), tc.text)
			got, err := Read(where.Resolver{GridHome: home}, "")
			if err != nil || len(got.Files) != 1 || len(got.Errors) != 1 || got.Errors[0].Message != tc.message {
				t.Fatalf("%+v %v", got, err)
			}
			if got.Files[0].Rev == nil || *got.Files[0].Rev != fmt.Sprintf("%x", sha256.Sum256([]byte(tc.text)))[:12] {
				t.Fatalf("%+v", got.Files)
			}
			if (len(tc.text) > 32768 || tc.name == "UTF8") && got.Files[0].Text != "" {
				t.Fatal("unreadable text retained")
			}
		})
	}
}

func TestWorkflowReadAbsentAndSymlink(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	got, err := Read(where.Resolver{GridHome: home}, "")
	if err != nil || len(got.Files) != 1 || got.Files[0].Rev != nil || len(got.Templates) != 0 || len(got.Errors) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("reader created home: %v", err)
	}
	home = workflowHome(t)
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), rootText(""))
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "workflows.md"), projectText(labelKeys, twoSteps))
	if err := os.Symlink(filepath.Join(home, "workflows.md"), filepath.Join(home, "projects", "beta", "workflows.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got, err = Read(where.Resolver{GridHome: home}, "")
	if err != nil || len(got.Files) != 3 || len(got.Templates) != 2 || len(got.Errors) != 1 || got.Errors[0].File != "beta/workflows.md" || got.Errors[0].Section != nil || got.Errors[0].Line != nil || got.Files[2].Rev != nil {
		t.Fatalf("%+v %v", got, err)
	}
}
