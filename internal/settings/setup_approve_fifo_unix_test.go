//go:build unix

package settings

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Review of fix-agent-setup: approve opened both setup files for writing to tell a person
// from a sandboxed agent, and a FIFO opened for writing waits for a reader, so a FIFO named
// setup-pending.json hung a person's approve. Only a regular file is opened now.
func TestApproveSetupCommandsRefusesAFIFO(t *testing.T) {
	for _, name := range []string{"setup-pending.json", "setup-answers.json"} {
		t.Run(name, func(t *testing.T) {
			_, s := fixture(t)
			got, err := s.RecordAgentSetup([]byte(`{"test":"make test","deploys":"Deploy from this machine","deployCommand":"make deploy"}`))
			must(t, err)
			file := filepath.Join(s.Home, "projects", "example", name)
			must(t, os.Remove(file))
			must(t, syscall.Mkfifo(file, 0o600))
			done := make(chan error, 1)
			go func() { _, _, err := s.ApproveSetupCommands(got.Hash); done <- err }()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), name+" must be a regular file") {
					t.Fatalf("approve with a FIFO named %s = %v, want a regular-file refusal", name, err)
				}
			case <-time.After(5 * time.Second):
				// Release the blocked writer so a failing test leaves no goroutine behind.
				if reader, err := os.OpenFile(file, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
					defer reader.Close()
				}
				select {
				case <-done:
				case <-time.After(5 * time.Second):
				}
				t.Fatalf("approve hung on a FIFO named %s", name)
			}
		})
	}
}
