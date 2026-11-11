//go:build unix

package golden_test

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

// grFifoDriver makes a named pipe under the box when hook(argv[0]) is true.
func grFifoDriver(t *testing.T, hook func(first string) bool) golden.Driver {
	return grDriver(golden.Output{}, func(in golden.Invocation) golden.Output {
		if hook(in.Argv[0]) {
			if err := syscall.Mkfifo(filepath.Join(in.Root, "demo", "grpipe"), 0o600); err != nil {
				t.Error(err)
			}
		}
		return golden.Output{}
	})
}

// Row 12: replay.go:315 (the snapshot before the main run, after the prelude).
func TestGRSnapshotBeforeUnsupported(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "setup.json"), `{"argv":["main"],"pre":[{"argv":["pre"],"input":""}]}`)
	driver := grFifoDriver(t, func(first string) bool { return first == "pre" })
	_, err := golden.Replay("testdata/fixtures", dir, t.TempDir(), driver)
	grWantPrefix(t, err, "unsupported fixture file: ")
}

// Row 12: replay.go:320 (the snapshot after the main run).
func TestGRSnapshotAfterUnsupported(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "setup.json"), `{"argv":["main"],"pre":[{"argv":["pre"],"input":""}]}`)
	driver := grFifoDriver(t, func(first string) bool { return first == "main" })
	_, err := golden.Replay("testdata/fixtures", dir, t.TempDir(), driver)
	grWantPrefix(t, err, "unsupported fixture file: ")
}
