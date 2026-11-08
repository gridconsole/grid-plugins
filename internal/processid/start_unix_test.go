//go:build unix

package processid

import (
	"os"
	"testing"
)

func TestStartIdentifiesCurrentProcess(t *testing.T) {
	token := Start(os.Getpid())
	if token == "" || Start(os.Getpid()) != token {
		t.Fatalf("unstable process token: %q", token)
	}
	for _, pid := range []int{0, -1, 2147483647} {
		if got := Start(pid); got != "" {
			t.Errorf("nonexistent pid %d: %q", pid, got)
		}
	}
}
