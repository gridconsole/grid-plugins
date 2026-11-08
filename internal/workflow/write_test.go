package workflow

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

func writeFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLockOpenConcurrent(t *testing.T) {
	dir, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	const name = ".workflows.lock"
	const writers = 32
	for trial := range 32 {
		start := make(chan struct{})
		results := make(chan error, writers)
		for range writers {
			go func() {
				<-start
				lock, err := openLock(dir, name)
				if err == nil {
					opened, statErr := lock.Stat()
					after, lstatErr := dir.Lstat(name)
					switch {
					case statErr != nil:
						err = statErr
					case lstatErr != nil:
						err = lstatErr
					case !after.Mode().IsRegular() || !os.SameFile(opened, after):
						err = errors.New("lock identity changed")
					}
					if closeErr := lock.Close(); err == nil {
						err = closeErr
					}
				}
				results <- err
			}()
		}
		close(start)
		for range writers {
			if err := <-results; err != nil {
				t.Errorf("trial %d: %v", trial, err)
			}
		}
		if err := dir.Remove(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWriteRustValidation(t *testing.T) {
	workspace := writeFixture(t, "root.md")
	project := writeFixture(t, "project.md")
	for _, tc := range []struct{ name, selector, text string }{
		{"empty", "", ""},
		{"no heading", "", strings.Replace(workspace, "# Workflows", "# Other", 1)},
		{"leading blank", "", "\n" + workspace},
		{"NUL", "", workspace + "\x00"},
		{"oversize", "", strings.Repeat("x", 32*1024+1)},
		{"invalid UTF8", "", workspace + "\xff"},
		{"two defaults", "", workspace + "\n" + strings.ReplaceAll(workspace[strings.Index(workspace, "## Accept"):], "## Accept", "## Fallback")},
		{"project default", "helios", workspace},
		{"other project", "northwind", project},
		{"pipe in section", "helios", strings.Replace(project, "## Rate change", "## Rate | change", 1)},
		{"pipe in step", "helios", strings.Replace(project, "Write the change |", "Write | the change |", 1)},
		{"back to later step", "helios", strings.Replace(project, "| 3 | Approve the new rates | approve | sam |  | Write the change |", "| 3 | Approve the new rates | approve | sam |  | Roll out on the first of the month |", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.selector != "" {
				putWorkflowFixture(t, filepath.Join(home, "projects", tc.selector, "project.md"), "---\nid: id-"+tc.selector+"\n---\n")
			}
			_, err := Write(where.Resolver{GridHome: home}, tc.selector, "none", strings.NewReader(tc.text))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("want invalid, got %v", err)
			}
			path := filepath.Join(home, "workflows.md")
			if tc.selector != "" {
				path = filepath.Join(home, "projects", tc.selector, "workflows.md")
			}
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refusal created file: %v", err)
			}
		})
	}
}

func TestWriteInputBoundaryAndRepair(t *testing.T) {
	text := writeFixture(t, "root.md")
	text += strings.Repeat("\n", 32*1024-len(text))
	home := t.TempDir()
	result, err := Write(where.Resolver{GridHome: home}, "", "none", strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(home, "")
	if err != nil || got.Files[0].Rev == nil || *got.Files[0].Rev != result.Rev || got.Files[0].Text != text {
		t.Fatalf("boundary: %+v %v", got, err)
	}
	for _, damaged := range []string{"\xff", strings.Repeat("x", 32*1024+1), ""} {
		putWorkflowFixture(t, filepath.Join(home, "workflows.md"), damaged)
		rev := fmt.Sprintf("%x", sha256.Sum256([]byte(damaged)))[:12]
		if _, err := Write(where.Resolver{GridHome: home}, "", strings.ToUpper(rev), strings.NewReader(text)); err != nil {
			t.Fatalf("repair: %v", err)
		}
	}
}

func TestWriteListsAllParseProblemsAndHidesPaths(t *testing.T) {
	home := t.TempDir()
	text := "# Workflows\n\n## /private/one\nreview rounds: 4\n\n## C:\\private\\two\nreview rounds: 4\n"
	_, err := Write(where.Resolver{GridHome: home}, "", "none", strings.NewReader(text))
	if !errors.Is(err, ErrInvalid) || strings.Count(err.Error(), "review rounds") != 2 || !strings.Contains(err.Error(), "no default workflow") || strings.Contains(err.Error(), "/private") || strings.Contains(err.Error(), `C:\private`) {
		t.Fatalf("diagnostics: %v", err)
	}
}

func TestWriteSelectionAndUnsafeTargets(t *testing.T) {
	text := writeFixture(t, "root.md")
	for _, kind := range []string{"ambiguous", "missing", "absolute", "traversal", "directory", "target link", "lock link", "lock directory", "project link"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			outside := t.TempDir()
			putWorkflowFixture(t, filepath.Join(outside, "workflows.md"), "untouched")
			selector := ""
			switch kind {
			case "ambiguous":
				putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
				putWorkflowFixture(t, filepath.Join(home, "projects", "beta", "project.md"), "---\nid: alpha\n---\n")
				selector = "alpha"
			case "missing":
				selector = "unknown"
			case "absolute":
				selector = outside
			case "traversal":
				selector = "../alpha"
			case "directory":
				if err := os.Mkdir(filepath.Join(home, "workflows.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "lock directory":
				if err := os.Mkdir(filepath.Join(home, ".workflows.lock"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "target link", "lock link":
				name := "workflows.md"
				if kind == "lock link" {
					name = ".workflows.lock"
				}
				if err := os.Symlink(filepath.Join(outside, "workflows.md"), filepath.Join(home, name)); err != nil {
					t.Skip(err)
				}
			case "project link":
				if err := os.Mkdir(filepath.Join(home, "projects"), 0o755); err != nil {
					t.Fatal(err)
				}
				putWorkflowFixture(t, filepath.Join(outside, "project.md"), "---\nid: id-alpha\n---\n")
				if err := os.Symlink(outside, filepath.Join(home, "projects", "alpha")); err != nil {
					t.Skip(err)
				}
				selector = "alpha"
			}
			_, err := Write(where.Resolver{GridHome: home}, selector, "none", strings.NewReader(text))
			if err == nil || strings.Contains(err.Error(), home) || strings.Contains(err.Error(), outside) {
				t.Fatalf("refusal: %v", err)
			}
			if (kind == "directory" || kind == "target link" || kind == "project link") && !errors.Is(err, ErrInvalid) {
				t.Fatalf("want invalid target, got %v", err)
			}
			data, err := os.ReadFile(filepath.Join(outside, "workflows.md"))
			if err != nil || string(data) != "untouched" {
				t.Fatalf("outside changed: %q, %v", data, err)
			}
		})
	}
}

type brokenWorkflowInput struct{}

func (brokenWorkflowInput) Read([]byte) (int, error) {
	return 0, &os.PathError{Op: "read", Path: "/private/input.md", Err: io.ErrUnexpectedEOF}
}

func TestWriteInputErrorsAndAbsentHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	for _, input := range []io.Reader{nil, brokenWorkflowInput{}} {
		_, err := Write(where.Resolver{GridHome: home}, "", "none", input)
		if err == nil || strings.Contains(err.Error(), "/private") {
			t.Fatalf("input error: %v", err)
		}
		if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid input created home: %v", err)
		}
	}
	if _, err := Write(where.Resolver{GridHome: home}, "", "none", strings.NewReader(writeFixture(t, "root.md"))); err != nil {
		t.Fatal(err)
	}
}

func TestWritePersonalCallerPolicy(t *testing.T) {
	home := t.TempDir()
	putWorkflowFixture(t, filepath.Join(home, "projects", "personal", "project.md"), "---\nid: personal-id\n---\n")
	text := strings.ReplaceAll(writeFixture(t, "project.md"), "helios", "personal")
	resolver := where.Resolver{GridHome: home, Caller: where.Caller{Role: "member", Member: "gus"}}
	_, err := Write(resolver, "personal", "none", strings.NewReader(text))
	if err == nil {
		t.Fatal("member wrote a Personal workflow")
	}
	if _, err := os.Stat(filepath.Join(home, "projects", "personal", "workflows.md")); !os.IsNotExist(err) {
		t.Fatal("refused Personal workflow created a file")
	}
	resolver.Caller.Role = "owner"
	if _, err := Write(resolver, "personal", "none", strings.NewReader(text)); err != nil {
		t.Fatalf("owner could not write the same Personal workflow: %v", err)
	}
}
