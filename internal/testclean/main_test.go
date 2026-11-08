package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	settle = 100 * time.Millisecond
	os.Exit(m.Run())
}

func TestTransientEntryOfAnotherProcessIsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "grid-eval-other")
	var stderr bytes.Buffer
	status := check(dir, func() int {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		go func() {
			time.Sleep(30 * time.Millisecond)
			_ = os.Remove(path)
		}()
		return 0
	}, &stderr)
	if status != 0 {
		t.Fatalf("transient entry reported: %d: %s", status, &stderr)
	}
}

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		name       string
		prefix     string
		remove     bool
		status     int
		wantStatus int
	}{
		{name: "existing entries are ignored"},
		{name: "unrelated entry is ignored", prefix: "unrelated-"},
		{name: "cleaned entry is ignored", prefix: "host-test", remove: true},
		{name: "test failure is preserved", status: 7, wantStatus: 7},
		{name: "leak after test failure", prefix: "host-test", status: 7, wantStatus: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			old := filepath.Join(dir, "host-test-existing")
			if err := os.Mkdir(old, 0o700); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			status := check(dir, func() int {
				if tc.prefix != "" {
					path := filepath.Join(dir, tc.prefix+"new")
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
					if tc.remove {
						if err := os.Remove(path); err != nil {
							t.Fatal(err)
						}
					}
				}
				return tc.status
			}, &stderr)
			if status != tc.wantStatus {
				t.Fatalf("status %d, want %d: %s", status, tc.wantStatus, &stderr)
			}
			if _, err := os.Stat(old); err != nil {
				t.Fatalf("guard modified an existing entry: %v", err)
			}
		})
	}
}

func TestEveryPrefixAndEntryType(t *testing.T) {
	for _, prefix := range prefixes {
		t.Run(prefix, func(t *testing.T) {
			dir := t.TempDir()
			var stderr bytes.Buffer
			status := check(dir, func() int {
				if err := os.Mkdir(filepath.Join(dir, prefix+"-dir"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, prefix+"-file"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				return 0
			}, &stderr)
			if status != 1 || !strings.Contains(stderr.String(), prefix+"-dir") || !strings.Contains(stderr.String(), prefix+"-file") {
				t.Fatalf("missed leak: status %d: %s", status, &stderr)
			}
			for _, suffix := range []string{"-dir", "-file"} {
				if _, err := os.Stat(filepath.Join(dir, prefix+suffix)); err != nil {
					t.Fatalf("guard must report rather than delete leaks: %v", err)
				}
			}
		})
	}
}

func TestSnapshotErrors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	var stderr bytes.Buffer
	if status := check(dir, func() int { t.Fatal("ran without baseline"); return 0 }, &stderr); status != 1 || !strings.Contains(stderr.String(), "before:") {
		t.Fatalf("missing baseline: status %d: %s", status, &stderr)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	status := check(dir, func() int {
		if err := os.Remove(dir); err != nil {
			t.Fatal(err)
		}
		return 0
	}, &stderr)
	if status != 1 || !strings.Contains(stderr.String(), "after:") {
		t.Fatalf("missing final snapshot: status %d: %s", status, &stderr)
	}
}

func TestSymlinkTempRoot(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "temp-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	var stderr bytes.Buffer
	status := check(link, func() int {
		if err := os.Mkdir(filepath.Join(root, "grid-cleanup-test-leak"), 0o700); err != nil {
			t.Fatal(err)
		}
		return 0
	}, &stderr)
	if status != 1 || !strings.Contains(stderr.String(), "grid-cleanup-test-leak") {
		t.Fatalf("missed leak through symlink: status %d: %s", status, &stderr)
	}
}
