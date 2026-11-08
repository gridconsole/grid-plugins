//go:build linux || darwin

package ports

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

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
