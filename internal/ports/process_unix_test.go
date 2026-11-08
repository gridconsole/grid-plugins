//go:build linux || darwin

package ports

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

func TestStopSignalLadder(t *testing.T) {
	for _, exitAfter := range []int{1, 2, 3, 0} {
		t.Run(fmt.Sprint(exitAfter), func(t *testing.T) {
			r := Record{PID: 123, Group: true, PIDStart: "original"}
			const grace = 100 * time.Millisecond
			var events []string
			var elapsed time.Duration
			waits := 0
			err := stopRecord(r, grace, func(got Record) (bool, error) {
				if got != r {
					t.Fatalf("identity changed: %+v", got)
				}
				events = append(events, "identity")
				return true, nil
			}, func(pid int, sig syscall.Signal) error {
				if pid != -r.PID {
					t.Fatalf("did not signal the whole group: %d", pid)
				}
				events = append(events, sig.String())
				return nil
			}, func(pid int, duration time.Duration) bool {
				if pid != r.PID || duration != grace {
					t.Fatalf("wait(%d, %s)", pid, duration)
				}
				elapsed += duration // Advance a controlled clock, without sleeping.
				waits++
				events = append(events, "wait")
				return waits == exitAfter
			})
			if err != nil {
				t.Fatal(err)
			}
			stages := exitAfter
			if stages == 0 {
				stages = 3 // A killed zombie need not have been reaped yet.
			}
			want := []string{"identity", "interrupt", "wait", "identity", "terminated", "wait", "identity", "killed", "wait"}[:3*stages]
			if !reflect.DeepEqual(events, want) || elapsed != time.Duration(stages)*grace {
				t.Fatalf("events %v after %s, want %v", events, elapsed, want)
			}
		})
	}
}

func TestStopRechecksIdentityBeforeEscalation(t *testing.T) {
	for _, identityErr := range []error{nil, errors.New("identity unavailable")} {
		t.Run(fmt.Sprint(identityErr), func(t *testing.T) {
			checks, signals := 0, 0
			err := stopRecord(Record{PID: 123}, time.Second, func(Record) (bool, error) {
				checks++
				if checks > 1 {
					return false, identityErr
				}
				return true, nil
			}, func(int, syscall.Signal) error {
				signals++
				return nil
			}, func(int, time.Duration) bool { return false })
			if !errors.Is(err, identityErr) || checks != 2 || signals != 1 {
				t.Fatalf("checks=%d signals=%d err=%v", checks, signals, err)
			}
		})
	}
}

func TestStopAfterGroupLeaderExits(t *testing.T) {
	for _, recorded := range []bool{true, false} {
		name := "Stop"
		if recorded {
			name = "StopRecord"
		}
		t.Run(name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			cmd := exec.Command("/bin/sh", "-c", "sleep 30 & sleep 0.3")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Stdout = writer
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() }()
			writer.Close()
			r := Record{PID: cmd.Process.Pid, Group: true, PIDStart: processid.Start(cmd.Process.Pid)}
			if r.PIDStart == "" {
				t.Fatal("missing live leader start token")
			}
			if err := cmd.Wait(); err != nil {
				t.Fatal(err)
			}
			if Alive(r.PID, false) || !Alive(r.PID, true) {
				t.Fatal("expected a live group with its leader gone")
			}
			// EOF proves the surviving child has stopped, even if an orphaned
			// zombie still makes kill(0) report that the group exists.
			exited := make(chan error, 1)
			go func() { _, err := io.Copy(io.Discard, reader); exited <- err }()
			if recorded {
				err = StopRecord(r, 20*time.Millisecond)
			} else {
				err = Stop(r.PID, 20*time.Millisecond)
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-exited:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("group child survived shutdown")
			}
		})
	}
}
