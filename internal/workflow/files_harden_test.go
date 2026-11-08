package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func hardenHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), rootText(""))
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "workflows.md"), projectText(labelKeys, twoSteps))
	return home
}

func hardenProblem(t *testing.T, home string, got Answer, label, reason string) {
	t.Helper()
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), home) || strings.Contains(string(data), filepath.ToSlash(home)) {
		t.Fatalf("home leaked into JSON: %s", data)
	}
	if len(got.Errors) != 1 || got.Errors[0].File != label || got.Errors[0].Message != reason {
		t.Fatalf("errors = %+v; want %s: %s", got.Errors, label, reason)
	}
}

func TestWorkflowHardenLeafErrors(t *testing.T) {
	for _, kind := range []string{"permission", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			home := hardenHome(t)
			path := filepath.Join(home, "projects", "alpha", "workflows.md")
			reason := "cannot read"
			switch kind {
			case "permission":
				if runtime.GOOS == "windows" || os.Geteuid() == 0 {
					t.Skip("mode 000 needs Unix and a non-root user")
				}
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			case "symlink", "directory":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if kind == "symlink" {
					reason = "symlink refused"
					if err := os.Symlink(filepath.Join(home, "workflows.md"), path); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
				} else {
					reason = "not a regular file"
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := Read(home, "")
			if err != nil {
				t.Fatal(err)
			}
			hardenProblem(t, home, got, "alpha/workflows.md", reason)
			if len(got.Templates) != 1 || got.Files[0].Rev == nil || got.Files[1].Rev != nil {
				t.Fatalf("healthy workspace lost or bad file read: %+v", got)
			}
		})
	}
}

func TestWorkflowHardenBadProjectCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "grid")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "../../cmd/grid")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, kind := range []string{"symlink", "permission"} {
		t.Run(kind, func(t *testing.T) {
			home := hardenHome(t)
			path := filepath.Join(home, "projects", "b")
			reason := "symlink refused"
			if kind == "symlink" {
				outside := t.TempDir()
				putWorkflowFixture(t, filepath.Join(outside, "project.md"), "---\nid: outside-id\n---\n")
				putWorkflowFixture(t, filepath.Join(outside, "workflows.md"), "must never read outside")
				if err := os.Symlink(outside, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			} else {
				if runtime.GOOS == "windows" || os.Geteuid() == 0 {
					t.Skip("mode 000 needs Unix and a non-root user")
				}
				reason = "cannot read"
				putWorkflowFixture(t, filepath.Join(path, "project.md"), "---\nid: id-b\n---\n")
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
			}
			for _, selector := range []string{"", "b", "alpha", "id-alpha"} {
				args := []string{"workflow", "show", "--json"}
				if selector != "" {
					args = append(args, "--project", selector)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, binary, args...)
				cmd.Env = append(os.Environ(), "GRID_HOME="+home)
				data, err := cmd.CombinedOutput()
				if err != nil {
					t.Errorf("selector %q: exit must be 0: %v\n%s", selector, err, data)
					continue
				}
				var got Answer
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatal(err)
				}
				if selector == "" || selector == "b" {
					hardenProblem(t, home, got, "b/workflows.md", reason)
				} else if len(got.Errors) != 0 {
					t.Fatalf("healthy selection: %+v", got)
				}
				wantFiles, wantTemplates := 2, 2
				if selector == "" {
					wantFiles = 3
				} else if selector == "b" {
					wantTemplates = 1
				}
				if len(got.Files) != wantFiles || len(got.Templates) != wantTemplates || got.Files[0].Rev == nil {
					t.Fatalf("selector %q: lost healthy files: %+v", selector, got)
				}
			}
		})
	}
}

// Run FIFO reads in a subprocess: a regression must neither hang the test suite
// nor leave a blocked goroutine behind when the two-second deadline expires.
func TestWorkflowHardenFIFO(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix FIFO test")
	}
	if _, err := exec.LookPath("mkfifo"); err != nil {
		t.Skip("mkfifo unavailable")
	}
	for _, swap := range []string{"before", "after-lstat"} {
		t.Run(swap, func(t *testing.T) {
			home := hardenHome(t)
			path := filepath.Join(home, "projects", "a", "workflows.md")
			putWorkflowFixture(t, filepath.Join(home, "projects", "a", "project.md"), "---\nid: id-a\n---\n")
			if swap == "before" {
				if out, err := exec.Command("mkfifo", path).CombinedOutput(); err != nil {
					t.Fatalf("mkfifo: %v: %s", err, out)
				}
			} else {
				putWorkflowFixture(t, path, "# Workflows\n")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWorkflowHardenOpenChild$")
			cmd.Env = append(os.Environ(), "GRID_WORKFLOW_TEST_HOME="+home, "GRID_WORKFLOW_TEST_SWAP="+swap)
			data, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("FIFO read blocked for two seconds")
			}
			if err != nil {
				t.Fatalf("read: %v: %s", err, data)
			}
			var got Answer
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("JSON: %v: %s", err, data)
			}
			hardenProblem(t, home, got, "a/workflows.md", "not a regular file")
			if len(got.Files) != 3 || len(got.Templates) != 2 {
				t.Fatalf("lost healthy files: %+v", got)
			}
		})
	}
}

func TestWorkflowHardenOpenChild(t *testing.T) {
	home := os.Getenv("GRID_WORKFLOW_TEST_HOME")
	if home == "" {
		return
	}
	if os.Getenv("GRID_WORKFLOW_TEST_SWAP") == "after-lstat" {
		beforeWorkflowOpen = func(relative string) {
			if relative != filepath.Join("projects", "a", "workflows.md") {
				return
			}
			path := filepath.Join(home, relative)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("mkfifo", path).CombinedOutput(); err != nil {
				t.Fatalf("mkfifo: %v: %s", err, out)
			}
		}
	}
	got, err := Read(home, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(data))
	os.Exit(0)
}
