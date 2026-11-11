//go:build windows

package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bundleLogNames(t *testing.T, root string) []string {
	t.Helper()
	files, err := bundleFiles(root, "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.name)
	}
	return names
}

func TestBundleWindowsCollectsSingleLinkLog(t *testing.T) {
	logs := filepath.Join(t.TempDir(), "logs")
	log := filepath.Join(logs, "ordinary.log")
	writeBundleFixture(t, log, "ordinary diagnostic", time.Now())
	if names := bundleLogNames(t, logs); len(names) != 1 || names[0] != "ordinary.log" {
		t.Fatalf("bundleFiles on a one-link log: %v", names)
	}
	got, err := readBundleTail(log)
	if err != nil || !strings.Contains(got, "ordinary diagnostic") {
		t.Fatalf("readBundleTail on a one-link log: %q, %v", got, err)
	}
}

func TestBundleWindowsDropsHardLinkedLog(t *testing.T) {
	logs := filepath.Join(t.TempDir(), "logs")
	first := filepath.Join(logs, "first.log")
	second := filepath.Join(logs, "second.log")
	writeBundleFixture(t, first, "aws_secret_access_key = example-private-value", time.Now())
	if err := os.Link(first, second); err != nil {
		t.Skip("this volume cannot make a hard link: " + err.Error())
	}
	if names := bundleLogNames(t, logs); len(names) != 0 {
		t.Fatalf("bundleFiles listed a hard-linked log under %v", names)
	}
	for _, path := range []string{first, second} {
		// The opened handle is rechecked too, covering a link added after the walk.
		if _, err := readBundleTail(path); err == nil || err.Error() != "log is not a single-link regular file" {
			t.Fatalf("readBundleTail(%s) on a hard-linked log: %v", filepath.Base(path), err)
		}
	}
}

func TestBundleWindowsOpenRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "credentials")
	writeBundleFixture(t, outside, "aws_secret_access_key = example-private-value", time.Now())
	link := filepath.Join(root, "logs", "link.log")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("this account cannot create a symlink (needs the symlink privilege or developer mode): " + err.Error())
	}
	f, err := openBundleLog(link)
	if err == nil {
		f.Close()
		t.Fatal("openBundleLog followed a symlink to a file outside logs")
	}
	if _, err := readBundleTail(link); err == nil {
		t.Fatal("readBundleTail followed a symlink to a file outside logs")
	}
}
