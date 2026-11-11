package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/version"
)

func TestPdHelpExitsZeroWithUsageOnStderr(t *testing.T) {
	var c calls
	code, stdout, stderr := runArgs(stubs(&c, t.TempDir()), "-h")
	if code != 0 {
		t.Fatalf("code %d, want 0", code)
	}
	if stdout != "" {
		t.Fatalf("stdout %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "Usage of packaging:") {
		t.Fatalf("stderr %q does not start with the usage line", stderr)
	}
	for _, flagName := range []string{"-describe", "-dist", "-release", "-version"} {
		if !strings.Contains(stderr, flagName) {
			t.Errorf("stderr %q does not name %s", stderr, flagName)
		}
	}
	if len(c.build) != 0 || len(c.write) != 0 {
		t.Fatalf("-h must not build or write: %+v", c)
	}
}

func TestPdGitDescribeInTaggedCheckout(t *testing.T) {
	testgit.Isolate(t)
	root := t.TempDir()
	testgit.Init(t, root, "", true)
	testgit.Run(t, root, "tag", "v1.2.3")
	t.Chdir(root)

	out, err := gitDescribe()
	if err != nil {
		t.Fatalf("gitDescribe: %v", err)
	}
	if want := "v1.2.3\n"; string(out) != want {
		t.Fatalf("gitDescribe %q, want %q", out, want)
	}
	want, err := version.FromDescribe("v1.2.3")
	if err != nil {
		t.Fatalf("FromDescribe: %v", err)
	}
	if got := checkoutVersion(gitDescribe); got != want {
		t.Fatalf("checkoutVersion %q, want %q", got, want)
	}
}

func TestPdGitDescribeOutsideGitFallsBack(t *testing.T) {
	testgit.Isolate(t)
	dir := t.TempDir()
	// Stop git looking above the folder, in case the machine's temp folder sits in a checkout.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)

	if out, err := gitDescribe(); err == nil {
		t.Fatalf("gitDescribe outside git returned %q and no error", out)
	}
	if got := checkoutVersion(gitDescribe); got != version.Version {
		t.Fatalf("checkoutVersion %q, want %q", got, version.Version)
	}
}
