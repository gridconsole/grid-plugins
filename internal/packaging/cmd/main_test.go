package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/version"
)

// calls records what the stubbed builders were given.
type calls struct {
	build, write []string
	logWriter    io.Writer
}

func stubs(c *calls, root string) deps {
	return deps{
		getwd: func() (string, error) { return root, nil },
		describe: func() ([]byte, error) {
			return nil, errors.New("describe must not run")
		},
		build: func(r, v string, log io.Writer) error {
			c.build = append(c.build, r, v)
			c.logWriter = log
			return nil
		},
		write: func(r, v string) error {
			c.write = append(c.write, r, v)
			return nil
		},
	}
}

func runArgs(d deps, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, d)
	return code, out.String(), errOut.String()
}

func TestDescribePrintsSemverFromStub(t *testing.T) {
	var c calls
	d := stubs(&c, "/unused")
	d.describe = func() ([]byte, error) { return []byte("v0.2.0-3-gabc1234\n"), nil }
	code, stdout, stderr := runArgs(d, "-describe")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if want := "v0.2.1-dev.3+abc1234\n"; stdout != want {
		t.Fatalf("stdout %q, want %q", stdout, want)
	}
	if len(c.build)+len(c.write) != 0 {
		t.Fatalf("-describe must not build or write: %+v", c)
	}
}

func TestDescribeFallsBackToSourceVersion(t *testing.T) {
	cases := map[string]func() ([]byte, error){
		"stub fails":   func() ([]byte, error) { return nil, errors.New("not a git repository") },
		"junk output":  func() ([]byte, error) { return []byte("not a version at all"), nil },
		"empty output": func() ([]byte, error) { return nil, nil },
	}
	for name, describe := range cases {
		t.Run(name, func(t *testing.T) {
			var c calls
			d := stubs(&c, "/unused")
			d.describe = describe
			code, stdout, stderr := runArgs(d, "-describe")
			if code != 0 || stderr != "" {
				t.Fatalf("code %d stderr %q", code, stderr)
			}
			if want := version.Version + "\n"; stdout != want {
				t.Fatalf("stdout %q, want %q", stdout, want)
			}
		})
	}
}

func TestReleasePrintsCommitCountOfGitRepo(t *testing.T) {
	testgit.Isolate(t)
	root := t.TempDir()
	testgit.Init(t, root, "", true)
	testgit.Run(t, root, "commit", "-q", "--allow-empty", "-m", "second")
	var c calls
	code, stdout, stderr := runArgs(stubs(&c, root), "-release")
	if code != 0 || stderr != "" {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if want := "0.2.2\n"; stdout != want {
		t.Fatalf("stdout %q, want %q", stdout, want)
	}
	if len(c.build)+len(c.write) != 0 {
		t.Fatalf("-release must not build or write: %+v", c)
	}
}

func TestReleaseOutsideGitExitsOneWithError(t *testing.T) {
	testgit.Isolate(t)
	root := t.TempDir()
	// Stop git looking above the folder, in case the machine's temp folder sits in a checkout.
	t.Setenv("GIT_CEILING_DIRECTORIES", root)
	var c calls
	code, stdout, stderr := runArgs(stubs(&c, root), "-release")
	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stdout %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "release version") || !strings.Contains(stderr, root) {
		t.Fatalf("stderr %q does not carry the release error", stderr)
	}
	if len(c.build)+len(c.write) != 0 {
		t.Fatalf("a failed -release must not fall through to build or write: %+v", c)
	}
}

func TestDefaultWritesWithVersionFlag(t *testing.T) {
	var c calls
	code, stdout, stderr := runArgs(stubs(&c, "/root"), "-version", "9.8.7")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if got := strings.Join(c.write, "|"); got != "/root|9.8.7" {
		t.Fatalf("write got %q", got)
	}
	if len(c.build) != 0 {
		t.Fatalf("default path must not build: %v", c.build)
	}
}

func TestDefaultWritesSourceVersionWithoutFlag(t *testing.T) {
	var c calls
	if code, _, stderr := runArgs(stubs(&c, "/root")); code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if got := strings.Join(c.write, "|"); got != "/root|"+version.Version {
		t.Fatalf("write got %q", got)
	}
}

func TestDistBuildsWithVersionFlagAndStdoutLog(t *testing.T) {
	var c calls
	var out, errOut bytes.Buffer
	code := run([]string{"-dist", "-version", "1.2.3"}, &out, &errOut, stubs(&c, "/root"))
	if code != 0 || errOut.Len() != 0 {
		t.Fatalf("code %d stderr %q", code, errOut.String())
	}
	if got := strings.Join(c.build, "|"); got != "/root|1.2.3" {
		t.Fatalf("build got %q", got)
	}
	if c.logWriter != io.Writer(&out) {
		t.Fatal("build must log to stdout")
	}
	if len(c.write) != 0 {
		t.Fatalf("-dist must not write: %v", c.write)
	}
}

func TestBuilderErrorExitsOneOnStderr(t *testing.T) {
	for _, args := range [][]string{{"-dist"}, {}} {
		var c calls
		d := stubs(&c, "/root")
		d.build = func(string, string, io.Writer) error { return errors.New("build broke") }
		d.write = func(string, string) error { return errors.New("write broke") }
		code, stdout, stderr := runArgs(d, args...)
		want := "write broke\n"
		if len(args) > 0 {
			want = "build broke\n"
		}
		if code != 1 || stdout != "" || stderr != want {
			t.Fatalf("args %v: code %d stdout %q stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestGetwdErrorExitsOne(t *testing.T) {
	var c calls
	d := stubs(&c, "")
	d.getwd = func() (string, error) { return "", errors.New("no cwd") }
	code, _, stderr := runArgs(d, "-dist")
	if code != 1 || stderr != "no cwd\n" {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if len(c.build)+len(c.write) != 0 {
		t.Fatalf("no builder may run without a root: %+v", c)
	}
}

func TestBadFlagExitsTwo(t *testing.T) {
	var c calls
	code, _, stderr := runArgs(stubs(&c, "/root"), "-nope")
	if code != 2 || !strings.Contains(stderr, "-nope") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if len(c.build)+len(c.write) != 0 {
		t.Fatalf("a bad flag must not build or write: %+v", c)
	}
}
