package doctor

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/version"
)

func writeBundleFixture(t *testing.T, path, body string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}
func readBundle(t *testing.T, path string) map[string]string {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	files := map[string]string{}
	for _, entry := range archive.File {
		if entry.Mode().Perm() != 0600 {
			t.Fatalf("entry mode %s: %v", entry.Name, entry.Mode())
		}
		f, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name] = string(b)
		if strings.HasPrefix(entry.Name, "logs/") && int64(len(b)) > bundleFileLimit {
			t.Fatal("oversized log")
		}
	}
	return files
}
func TestBundleContentsAndMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fslock support is Unix-only")
	}
	home := t.TempDir()
	gh := filepath.Join(home, ".grid")
	now := time.Date(2026, 11, 8, 15, 4, 5, 123, time.FixedZone("fixture", 3600))
	for i := 0; i < 23; i++ {
		writeBundleFixture(t, filepath.Join(gh, "logs", fmt.Sprintf("log-%02d.txt", i)), "Bearer example-token\n"+home+"/project\n", now.Add(time.Duration(i)*time.Second))
	}
	writeBundleFixture(t, filepath.Join(gh, "logs", "nested", "new.log"), "newest\n", now.Add(time.Minute))
	writeBundleFixture(t, filepath.Join(gh, "logs", "crash-old.txt"), "old crash", now.Add(-2*time.Hour))
	writeBundleFixture(t, filepath.Join(gh, "logs", "crash-new.txt"), "new crash sk-abcdefghijklmnop", now.Add(-time.Hour))
	outside := filepath.Join(home, "private.txt")
	writeBundleFixture(t, outside, "private content", now)
	if err := os.Symlink(outside, filepath.Join(gh, "logs", "link.log")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(home, filepath.Join(gh, "logs", "outside")); err != nil {
		t.Fatal(err)
	}
	o := BundleOptions{GridHome: gh, Home: home, Now: func() time.Time { return now }, Report: Report{Checks: []Check{{Name: "example", Detail: home + "/project Bearer sample", Fix: "fix locally"}}}}
	path, err := Bundle(o)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "grid-support-20261108T140405.000000123Z.zip" {
		t.Fatal(path)
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatalf("mode: %v %v", st, err)
	}
	files := readBundle(t, path)
	if len(files) != 23 {
		t.Fatalf("files: %v", files)
	}
	if _, ok := files["logs/log-03.txt"]; ok {
		t.Fatal("old log retained")
	}
	if _, ok := files["logs/log-04.txt"]; !ok {
		t.Fatal("new log omitted")
	}
	if _, ok := files["logs/crash-old.txt"]; ok {
		t.Fatal("old crash retained")
	}
	if files["logs/crash-new.txt"] != "new crash [REDACTED]" {
		t.Fatal(files)
	}
	if !strings.Contains(files["versions.txt"], "grid "+version.Version) || !strings.Contains(files["versions.txt"], "contract "+version.Contract) || !strings.Contains(files["versions.txt"], "app_min "+version.AppMin) {
		t.Fatal(files["versions.txt"])
	}
	for name, body := range files {
		if strings.Contains(body, home) || strings.Contains(body, "example-token") || strings.Contains(body, "private content") {
			t.Fatalf("unredacted %s: %s", name, body)
		}
	}
	if !strings.Contains(files["doctor.txt"], "fix: fix locally") {
		t.Fatal(files["doctor.txt"])
	}
	if _, err = Bundle(o); err == nil {
		t.Fatal("overwrote existing archive")
	}
}

func TestBundleTailRedaction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fslock support is Unix-only")
	}
	tests := []struct{ name, body, contains, absent string }{
		{"tail", "old line\n" + strings.Repeat("diagnostic\n", int(bundleFileLimit/11)+10) + "newest line\n", "newest line", "old line"},
		{"cut token", "Bearer " + strings.Repeat("secret", int(bundleFileLimit/6)+20) + "\nlast line\n", "last line", "secret"},
		{"cut PEM", "-----BEGIN PRIVATE KEY-----\n" + strings.Repeat("secret\n", int(bundleFileLimit/7)+20) + "-----END PRIVATE KEY-----\nlast line\n", "last line", "secret"},
		{"cut unterminated PEM", "-----BEGIN PRIVATE KEY-----\n" + strings.Repeat("secret\n", int(bundleFileLimit/7)+20), "[REDACTED]", "secret"},
		{"large single line", strings.Repeat("x", int(bundleFileLimit)+20), "", "x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gh := t.TempDir()
			writeBundleFixture(t, filepath.Join(gh, "logs", "worker.log"), tc.body, time.Now())
			path, err := Bundle(BundleOptions{GridHome: gh})
			if err != nil {
				t.Fatal(err)
			}
			body := readBundle(t, path)["logs/worker.log"]
			if !strings.Contains(body, tc.contains) || strings.Contains(body, tc.absent) {
				t.Fatalf("incorrect retained tail (%d bytes)", len(body))
			}
		})
	}
}

func TestBundleMissingLogsAndOutputRefusal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fslock support is Unix-only")
	}
	gh := t.TempDir()
	out := filepath.Join(gh, "custom", "report.zip")
	path, err := Bundle(BundleOptions{GridHome: gh, Out: out})
	if err != nil {
		t.Fatal(err)
	}
	if path != out || len(readBundle(t, path)) != 2 {
		t.Fatal(path)
	}
	original, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(out, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = Bundle(BundleOptions{GridHome: gh, Out: out}); err == nil {
		t.Fatal("existing output accepted")
	}
	after, err := os.ReadFile(out)
	if err != nil || string(after) != string(original) {
		t.Fatal("existing file changed")
	}
	if err = os.Symlink(out, filepath.Join(gh, "link.zip")); err != nil {
		t.Fatal(err)
	}
	if _, err = Bundle(BundleOptions{GridHome: gh, Out: filepath.Join(gh, "link.zip")}); err == nil {
		t.Fatal("output symlink accepted")
	}
}

func TestBundleSkipsHardLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fslock support is Unix-only")
	}
	root := t.TempDir()
	gh := filepath.Join(root, "grid")
	outside := filepath.Join(root, "credentials")
	writeBundleFixture(t, outside, "aws_secret_access_key = example-private-value", time.Now())
	writeBundleFixture(t, filepath.Join(gh, "logs", "ordinary.log"), "ordinary diagnostic", time.Now())
	linked := filepath.Join(gh, "logs", "hard.log")
	if err := os.Link(outside, linked); err != nil {
		t.Fatal(err)
	}
	outsideInfo, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	linkedInfo, err := os.Stat(linked)
	if err != nil || !os.SameFile(outsideInfo, linkedInfo) {
		t.Fatalf("fixture is not a hard link: %v", err)
	}
	path, err := Bundle(BundleOptions{GridHome: gh})
	if err != nil {
		t.Fatal(err)
	}
	files := readBundle(t, path)
	if len(files) != 3 || files["logs/ordinary.log"] != "ordinary diagnostic" {
		t.Fatalf("unexpected archive contents: %v", files)
	}
	for name, body := range files {
		if name == "logs/hard.log" || strings.Contains(body, "example-private-value") {
			t.Fatal("collected outside file through hard link")
		}
	}
	// Recheck the opened descriptor too, covering a link added after selection.
	if _, err := readBundleTail(linked); err == nil {
		t.Fatal("read a hard link after selection")
	}
}

func TestBundleCustomOutputLeavesOnlyArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fslock support is Unix-only")
	}
	outDir := t.TempDir()
	gh := t.TempDir()
	out := filepath.Join(outDir, "report.zip")
	if _, err := Bundle(BundleOptions{GridHome: gh, Out: out}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "report.zip" {
		t.Fatalf("unexpected output folder entries: %v, %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(gh, "support", ".grid-support.lock")); err != nil {
		t.Fatalf("support lock: %v", err)
	}
}
