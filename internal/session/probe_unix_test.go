//go:build linux || darwin

package session

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProbeLocal(t *testing.T) {
	if ProbeLocal(Live{}) != Unknown {
		t.Fatal("missing identity is not unknown")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(os.Getpid()))
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	start := strings.TrimSpace(string(b))
	if ProbeLocal(Live{PID: os.Getpid(), PIDStart: start}) != Alive {
		t.Fatal("current process not alive")
	}
	if ProbeLocal(Live{PID: os.Getpid(), PIDStart: "2000-01-01T00:00:00.000Z"}) != Dead {
		t.Fatal("different start not detected")
	}
}
