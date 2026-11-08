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

func putWorkflowFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func workflowHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		putWorkflowFixture(t, filepath.Join(home, "projects", name, "project.md"), "---\nid: id-"+name+"\n---\n")
	}
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), "# Workflows\n")
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "workflows.md"), "# Alpha\n")
	return home
}

func TestWorkflowFilesAllProjects(t *testing.T) {
	files, problems, err := readFiles(where.Resolver{GridHome: workflowHome(t)}, "")
	if err != nil || len(problems) != 0 || len(files) != 3 {
		t.Fatalf("files=%+v problems=%+v err=%v", files, problems, err)
	}
	if files[0].Scope != "workspace" || files[0].Project != nil || files[0].Text != "# Workflows\n" || files[0].Rev == nil {
		t.Fatalf("workspace: %+v", files[0])
	}
	if files[1].Scope != "project" || *files[1].Project != "alpha" || files[1].Rev == nil || files[1].Text != "# Alpha\n" {
		t.Fatalf("alpha: %+v", files[1])
	}
	if *files[2].Project != "beta" || files[2].Rev != nil || files[2].Text != "" {
		t.Fatalf("absent beta: %+v", files[2])
	}
}

func TestWorkflowFilesOneProject(t *testing.T) {
	home := workflowHome(t)
	for _, selector := range []string{"alpha", "id-alpha"} {
		t.Run(selector, func(t *testing.T) {
			files, problems, err := readFiles(where.Resolver{GridHome: home}, selector)
			if err != nil || len(problems) != 0 || len(files) != 2 || *files[1].Project != "alpha" {
				t.Fatalf("files=%+v problems=%+v err=%v", files, problems, err)
			}
		})
	}
	for _, selector := range []string{"unknown", "../alpha"} {
		if _, _, err := readFiles(where.Resolver{GridHome: home}, selector); err == nil {
			t.Fatalf("accepted project %q", selector)
		}
	}
}

func TestWorkflowFilesAbsentHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	files, problems, err := readFiles(where.Resolver{GridHome: home}, "")
	if err != nil || len(problems) != 0 || len(files) != 1 || files[0].Rev != nil || files[0].Text != "" {
		t.Fatalf("files=%+v problems=%+v err=%v", files, problems, err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("reader created home: %v", err)
	}
	if _, _, err := readFiles(where.Resolver{GridHome: ""}, ""); err == nil {
		t.Fatal("accepted unknown home")
	}
}

func TestWorkflowRevIsContentHash(t *testing.T) {
	home := t.TempDir()
	var previous string
	for _, text := range []string{"# Workflows\n", "# Workflows\n", "# Workflows\r\n"} {
		putWorkflowFixture(t, filepath.Join(home, "workflows.md"), text)
		files, problems, err := readFiles(where.Resolver{GridHome: home}, "")
		if err != nil || len(problems) != 0 {
			t.Fatalf("problems=%+v err=%v", problems, err)
		}
		want := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))[:12]
		if files[0].Rev == nil || *files[0].Rev != want || files[0].Text != text {
			t.Fatalf("file=%+v want rev=%s", files[0], want)
		}
		if strings.Contains(text, "\r") && previous == *files[0].Rev {
			t.Fatal("changed bytes kept the same revision")
		}
		previous = *files[0].Rev
	}
}

func TestWorkflowFilesReadErrorsKeepOtherFiles(t *testing.T) {
	for _, kind := range []string{"directory", "invalid-utf8"} {
		t.Run(kind, func(t *testing.T) {
			home := workflowHome(t)
			path := filepath.Join(home, "projects", "beta", "workflows.md")
			if kind == "directory" {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				putWorkflowFixture(t, path, "\xff")
			}
			files, problems, err := readFiles(where.Resolver{GridHome: home}, "")
			if err != nil || len(files) != 3 || len(problems) != 1 || problems[0].index != 2 {
				t.Fatalf("files=%+v problems=%+v err=%v", files, problems, err)
			}
			if files[0].Rev == nil || files[1].Rev == nil || files[2].Text != "" {
				t.Fatalf("files=%+v", files)
			}
			if (files[2].Rev != nil) != (kind == "invalid-utf8") {
				t.Fatalf("revision: %+v", files[2])
			}
		})
	}
}

func TestWorkflowFilesEmptyPresentFile(t *testing.T) {
	home := t.TempDir()
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), "")
	files, problems, err := readFiles(where.Resolver{GridHome: home}, "")
	if err != nil || len(problems) != 0 || len(files) != 1 || files[0].Rev == nil || *files[0].Rev != "e3b0c44298fc" {
		t.Fatalf("files=%+v problems=%+v err=%v", files, problems, err)
	}
}

func TestWorkflowFilesAmbiguousProject(t *testing.T) {
	home := workflowHome(t)
	putWorkflowFixture(t, filepath.Join(home, "projects", "beta", "project.md"), "---\nid: alpha\n---\n")
	if _, _, err := readFiles(where.Resolver{GridHome: home}, "alpha"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous selection: %v", err)
	}
}

func TestWorkflowSymlinkRefused(t *testing.T) {
	home := workflowHome(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	putWorkflowFixture(t, outside, "must not be read")
	path := filepath.Join(home, "projects", "beta", "workflows.md")
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	files, problems, err := readFiles(where.Resolver{GridHome: home}, "")
	if err != nil || len(problems) != 1 || len(files) != 3 {
		t.Fatalf("files=%+v problems=%+v err=%v", files, problems, err)
	}
	if problems[0].index != 2 || !strings.Contains(problems[0].err.Error(), "symlink") || files[2].Rev != nil || files[2].Text != "" {
		t.Fatalf("file=%+v problems=%+v", files[2], problems)
	}
	// A symlink spelling of the home itself is allowed (including macOS temp roots).
	alias := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(home, alias); err != nil {
		t.Fatal(err)
	}
	if files, problems, err := readFiles(where.Resolver{GridHome: alias}, "alpha"); err != nil || len(problems) != 0 || len(files) != 2 {
		t.Fatalf("aliased home: files=%+v problems=%+v err=%v", files, problems, err)
	}
}
