package cli

import (
	"bytes"
	"fmt"
	"io/fs"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/intent"
)

// qwEnPathErr is the error a failed task.md write returns: the wrapped errno under
// a *fs.PathError, as os.WriteFile and os.Rename give it.
func qwEnPathErr(errno error) error {
	return fmt.Errorf("write task.md: %w", &fs.PathError{Op: "write", Path: "/x/task.md", Err: errno})
}

// A full disk on any write is exit 4, as the Feed append's feed.ErrDiskFull is; a
// permission or read-only failure keeps exit 1, and the other codes do not move.
func TestQwEnExitCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"enospc under a path error", qwEnPathErr(syscall.ENOSPC), 4},
		{"bare enospc", syscall.ENOSPC, 4},
		{"feed disk full", fmt.Errorf("append: %w", feed.ErrDiskFull), 4},
		{"permission", qwEnPathErr(fs.ErrPermission), 1},
		{"eacces", qwEnPathErr(syscall.EACCES), 1},
		{"erofs", qwEnPathErr(syscall.EROFS), 1},
		{"other write error", qwEnPathErr(syscall.EIO), 1},
		{"plain disk text", fmt.Errorf("write task.md: no space left on device"), 1},
		{"claimed", fmt.Errorf("claim: %w", intent.ErrClaimed), 5},
		{"stale lease", fmt.Errorf("post: %w", feed.ErrStaleLease), 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCode(tc.err); got != tc.want {
				t.Errorf("exitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// The exit a --json call leaves with: only an exit 1 whose recorded error is a full
// disk becomes 4. A read-only or permission error stays 1; usage, rejected and done
// exits, and a call that recorded nothing, do not move.
func TestQwEnFinishExit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exit     int
		recorded error
		want     int
	}{
		{"1 enospc", 1, qwEnPathErr(syscall.ENOSPC), 4},
		{"1 feed disk full", 1, fmt.Errorf("append: %w", feed.ErrDiskFull), 4},
		{"1 erofs", 1, qwEnPathErr(syscall.EROFS), 1},
		{"1 permission", 1, qwEnPathErr(fs.ErrPermission), 1},
		{"1 other", 1, qwEnPathErr(syscall.EIO), 1},
		{"1 nothing recorded", 1, nil, 1},
		{"2 enospc", 2, qwEnPathErr(syscall.ENOSPC), 2},
		{"2 other", 2, fmt.Errorf("usage"), 2},
		{"3 enospc", 3, qwEnPathErr(syscall.ENOSPC), 3},
		{"0 enospc", 0, qwEnPathErr(syscall.ENOSPC), 0},
		{"0 nothing", 0, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := qwEnFinishExit(tc.exit, tc.recorded); got != tc.want {
				t.Errorf("qwEnFinishExit(%d, %v) = %d, want %d", tc.exit, tc.recorded, got, tc.want)
			}
		})
	}
}

// recordError keeps the error for the finish and prints the home line once, however
// many times a command records; a call without a recorder prints and keeps nothing.
func TestQwEnRecordOnce(t *testing.T) {
	var buf bytes.Buffer
	f := &commandFailure{}
	env := Env{Stderr: &buf, failure: f}
	first, second := qwEnPathErr(syscall.EROFS), qwEnPathErr(syscall.ENOSPC)
	env.recordError(first)
	env.recordError(first)
	if got := strings.Count(buf.String(), qwEnHomeLine); got != 1 {
		t.Errorf("home line printed %d times, want 1: %q", got, buf.String())
	}
	env.recordError(second)
	if f.err != second || qwEnFinishExit(1, f.err) != 4 {
		t.Errorf("recorder keeps %v; want the last error, which is a full disk", f.err)
	}
	none := Env{Stderr: &buf}
	buf.Reset()
	none.recordError(first)
	if buf.Len() != 0 {
		t.Errorf("no recorder, stderr = %q, want nothing", buf.String())
	}
}

// qwDiskFull knows the errno of this platform's full disk and no other. Errno 112 is
// ERROR_DISK_FULL on Windows and EHOSTDOWN on Linux, where it is not a full disk.
func TestQwEnDiskFullPlatform(t *testing.T) {
	if !qwDiskFull(qwEnPathErr(syscall.ENOSPC)) || qwDiskFull(nil) || qwDiskFull(qwEnPathErr(syscall.EIO)) {
		t.Error("ENOSPC under a path error must be a full disk; nil and EIO must not")
	}
	windows := runtime.GOOS == "windows"
	for _, errno := range []syscall.Errno{112, 39} {
		if got := qwDiskFull(qwEnPathErr(errno)); got != windows {
			t.Errorf("qwDiskFull(Errno(%d)) = %t on %s, want %t", errno, got, runtime.GOOS, windows)
		}
	}
}

// qwEnHome records err on an Env that has a failure recorder, then prints the
// command's own line as task.go does, and returns what reached stderr.
func qwEnHome(err error) string {
	var buf bytes.Buffer
	env := Env{Stderr: &buf, failure: &commandFailure{}}
	env.recordError(err)
	fmt.Fprintf(env.Stderr, "grid task x: %v\n", err)
	return buf.String()
}

// An unwritable Grid home says so in a first line with no path; the command's own
// line stays under it. A full disk or any other error adds no line.
func TestQwEnHomeLine(t *testing.T) {
	const first = "grid: the Grid home cannot be written"
	for _, tc := range []struct {
		name string
		err  error
		line bool
	}{
		{"erofs", qwEnPathErr(syscall.EROFS), true},
		{"fs permission", qwEnPathErr(fs.ErrPermission), true},
		{"eacces", qwEnPathErr(syscall.EACCES), true},
		{"enospc", qwEnPathErr(syscall.ENOSPC), false},
		{"io error", qwEnPathErr(syscall.EIO), false},
		{"plain", fmt.Errorf("storage failure"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := qwEnHome(tc.err)
			lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
			own := fmt.Sprintf("grid task x: %v", tc.err)
			if !tc.line {
				if len(lines) != 1 || lines[0] != own {
					t.Fatalf("stderr = %q, want only the command's line %q", got, own)
				}
				return
			}
			if len(lines) != 2 || lines[0] != first || lines[1] != own {
				t.Fatalf("stderr = %q, want %q then %q", got, first, own)
			}
			if strings.ContainsAny(lines[0], `/\`) {
				t.Errorf("first line %q holds a path separator", lines[0])
			}
		})
	}
}
