package job

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A job: pointer to a Job the plan names but has not posted says so, rather than
// showing a failed lstat or read with the operating system's path.
func TestFtJobNotPostedSaysTheJobIsNotPosted(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "T-07-demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, pointer := range []string{"job:J-2", "job:J-2#verify", "job:T-07/J-2"} {
		_, err := (&Files{Task: dir}).Resolve(pointer)
		if err == nil {
			t.Fatalf("%s: want an error on a task with no jobs/J-2/brief.md", pointer)
		}
		if !strings.HasPrefix(err.Error(), pointer+":") || !strings.Contains(err.Error(), "has no saved brief yet") {
			t.Errorf("%s: error %q does not say the Job has no saved brief yet", pointer, err)
		}
		for _, bad := range []string{"lstat", "cannot read source", filepath.Dir(dir)} {
			if strings.Contains(err.Error(), bad) {
				t.Errorf("%s: error %q still contains %q", pointer, err, bad)
			}
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: error %q no longer matches fs.ErrNotExist", pointer, err)
		}
	}
}

func TestFtJobNotPostedPostedJobStillResolves(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "T-07-demo")
	if err := os.MkdirAll(filepath.Join(dir, "jobs", "J-2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "jobs", "J-2", "brief.md"), []byte("# Brief\n\nDo it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, pointer := range []string{"job:J-2", "job:T-07/J-2"} {
		if _, err := (&Files{Task: dir}).Resolve(pointer); err != nil {
			t.Errorf("%s: posted Job: %v", pointer, err)
		}
	}
}
