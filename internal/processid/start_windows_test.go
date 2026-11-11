//go:build windows

package processid

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// absentPID is far above any pid Windows hands out, so OpenProcess answers
// ERROR_INVALID_PARAMETER for it.
const absentPID = 0x7FFFFFF0

// TestHelperHoldWindows is the child the tests below start: it holds until its
// stdin closes. Run on its own it does nothing.
func TestHelperHoldWindows(t *testing.T) {
	if os.Getenv("GRID_PROCESSID_HELPER") != "1" {
		return
	}
	io.Copy(io.Discard, os.Stdin)
}

// holdChild starts a child that lives until the returned release is called.
func holdChild(t *testing.T) (*exec.Cmd, func()) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldWindows$")
	cmd.Env = append(os.Environ(), "GRID_PROCESSID_HELPER=1")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			in.Close()
			cmd.Wait()
		}
	}
	t.Cleanup(release)
	return cmd, release
}

func TestStartCurrentProcessWindows(t *testing.T) {
	got := Start(os.Getpid())
	if got == "" {
		t.Fatal("no start for the current process")
	}
	// The same text format ps prints on macOS, in UTC.
	parsed, err := ParseLstart(got, time.UTC)
	if err != nil {
		t.Fatalf("start %q does not parse as an lstart line: %v", got, err)
	}
	if parsed.After(time.Now()) {
		t.Fatalf("start %q is in the future", got)
	}
	if again := Start(os.Getpid()); again != got {
		t.Fatalf("start changed between reads: %q then %q", got, again)
	}
	created, err := CreationTime(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if want := created.UTC().Truncate(time.Second).Format(LstartLayout); got != want {
		t.Fatalf("Start %q, CreationTime says %q", got, want)
	}
}

func TestStartRejectsNonPositivePIDWindows(t *testing.T) {
	for _, pid := range []int{0, -1} {
		if got := Start(pid); got != "" {
			t.Errorf("pid %d: %q", pid, got)
		}
		if _, err := CreationTime(pid); !errors.Is(err, ErrGone) {
			t.Errorf("pid %d: CreationTime error %v", pid, err)
		}
	}
}

func TestStartAbsentPIDWindows(t *testing.T) {
	if got := Start(absentPID); got != "" {
		t.Fatalf("absent pid has start %q", got)
	}
	if _, err := CreationTime(absentPID); !errors.Is(err, ErrGone) {
		t.Fatalf("absent pid: %v, want ErrGone", err)
	}
}

func TestStartChildLivesAndExitsWindows(t *testing.T) {
	before := time.Now()
	cmd, release := holdChild(t)
	pid := cmd.Process.Pid

	created, err := CreationTime(pid)
	if err != nil {
		t.Fatal(err)
	}
	if created.Before(before.Add(-2*time.Second)) || created.After(time.Now().Add(2*time.Second)) {
		t.Fatalf("child created %v, started between %v and %v", created, before, time.Now())
	}
	if Start(pid) == "" {
		t.Fatal("live child has no start")
	}

	// Hold a second handle so the pid cannot be reused after the child exits:
	// an exited process whose handle is still open must read as gone.
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	release()
	if _, err := CreationTime(pid); !errors.Is(err, ErrGone) {
		t.Fatalf("exited child with an open handle: %v, want ErrGone", err)
	}
	if got := Start(pid); got != "" {
		t.Fatalf("exited child has start %q", got)
	}
}
