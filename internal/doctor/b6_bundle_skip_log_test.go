package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const b6BsLockedMarker = "B6BS-LOCKED-LOG-MARKER"

// b6BsLockedLog writes a log holding b6BsLockedMarker and makes it unreadable for
// this login; the mode is put back when the test ends.
func b6BsLockedLog(t *testing.T, path string) {
	t.Helper()
	writeBundleFixture(t, path, b6BsLockedMarker+"\n", time.Now())
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o600) })
}

func b6BsSkipUnlessUnreadable(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file this login cannot read")
	}
}

// TestB6BsBundleSkipsAnUnreadableHomeLog: a Grid home log that exists but cannot be
// read is left out, the bundle is still written with the readable log as written,
// and skipped.txt holds one line naming the left-out log's archive name and why.
func TestB6BsBundleSkipsAnUnreadableHomeLog(t *testing.T) {
	b6BsSkipUnlessUnreadable(t)
	gh := t.TempDir()
	writeBundleFixture(t, filepath.Join(gh, "logs", "ok.log"), "readable b6 body\n", time.Now())
	b6BsLockedLog(t, filepath.Join(gh, "logs", "locked.log"))

	path, err := Bundle(BundleOptions{GridHome: gh, Out: filepath.Join(t.TempDir(), "bundle.zip")})
	if err != nil {
		t.Fatalf("an unreadable log must not stop the bundle: %v", err)
	}
	if got, want := strings.Join(bundleNames(t, path), ","), "doctor.txt,logs/ok.log,skipped.txt,versions.txt"; got != want {
		t.Fatalf("bundle holds %s, want %s", got, want)
	}
	files := readBundle(t, path)
	lines := strings.Split(strings.TrimSuffix(files["skipped.txt"], "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "logs/locked.log") || !strings.Contains(lines[0], "permission denied") {
		t.Fatalf("skipped.txt = %q, want one line naming logs/locked.log and why", files["skipped.txt"])
	}
	if files["logs/ok.log"] != "readable b6 body\n" {
		t.Fatalf("logs/ok.log = %q, want the log as written", files["logs/ok.log"])
	}
	for name, body := range files {
		if strings.Contains(body, b6BsLockedMarker) {
			t.Fatalf("%s holds the unreadable log's text", name)
		}
	}
}

// TestB6BsBundleSkipsAnUnreadableLauncherLog: the same holds for a launcher log in a
// launcher folder that can be read, and its line goes in the same skipped.txt.
func TestB6BsBundleSkipsAnUnreadableLauncherLog(t *testing.T) {
	b6BsSkipUnlessUnreadable(t)
	gh, machine := t.TempDir(), t.TempDir()
	writeLog(t, filepath.Join(gh, "logs", "grid-open.log"), "home log\n")
	writeLog(t, filepath.Join(machine, "launcher", "alice", "worker.log"), "worker\n")
	b6BsLockedLog(t, filepath.Join(machine, "launcher", "alice", "server.log"))

	path, err := Bundle(BundleOptions{GridHome: gh, Machine: machineAt(machine), Login: "alice", Out: filepath.Join(t.TempDir(), "bundle.zip")})
	if err != nil {
		t.Fatalf("an unreadable launcher log must not stop the bundle: %v", err)
	}
	if got, want := strings.Join(bundleNames(t, path), ","), "doctor.txt,logs/grid-open.log,logs/machine/launcher/worker.log,skipped.txt,versions.txt"; got != want {
		t.Fatalf("bundle holds %s, want %s", got, want)
	}
	line, _ := zipEntry(t, path, "skipped.txt")
	if strings.Count(line, "\n") != 1 || !strings.Contains(line, "logs/machine/launcher/server.log") || !strings.Contains(line, "permission denied") {
		t.Fatalf("skipped.txt = %q, want one line naming logs/machine/launcher/server.log and why", line)
	}
}

// TestB6BsReadBundleTailStillRefusesAHardLink: the single-link refusal stays. A log
// hard-linked after it was written is refused by readBundleTail itself.
func TestB6BsReadBundleTailStillRefusesAHardLink(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("hard-link check is Unix-only")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "logs", "worker.log")
	writeBundleFixture(t, log, "diagnostic\n", time.Now())
	if err := os.Link(log, filepath.Join(dir, "credentials")); err != nil {
		t.Fatal(err)
	}
	if _, err := readBundleTail(log); err == nil || err.Error() != "log is not a single-link regular file" {
		t.Fatalf("readBundleTail on a hard-linked log = %v, want the single-link refusal", err)
	}
}

// TestB6BsSkippedLineIsRedacted: skipped.txt goes through add like every entry, so a
// secret-shaped name and the home path in a skipped line are redacted.
func TestB6BsSkippedLineIsRedacted(t *testing.T) {
	b6BsSkipUnlessUnreadable(t)
	home := t.TempDir()
	gh := filepath.Join(home, ".grid")
	const secret = "sk-b6bsabcdefghijklmnop"
	writeBundleFixture(t, filepath.Join(gh, "logs", "ok.log"), "ok\n", time.Now())
	b6BsLockedLog(t, filepath.Join(gh, "logs", secret+".log"))

	path, err := Bundle(BundleOptions{GridHome: gh, Home: home, Out: filepath.Join(t.TempDir(), "bundle.zip")})
	if err != nil {
		t.Fatalf("an unreadable log must not stop the bundle: %v", err)
	}
	files := readBundle(t, path)
	line, ok := files["skipped.txt"]
	if !ok || !strings.Contains(line, "[REDACTED]") || !strings.Contains(line, "permission denied") {
		t.Fatalf("skipped.txt = %q, want a redacted line", line)
	}
	for name, body := range files {
		if strings.Contains(name, secret) || strings.Contains(body, secret) || strings.Contains(body, home) {
			t.Fatalf("unredacted %s: %q", name, body)
		}
	}
}
