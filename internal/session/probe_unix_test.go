//go:build linux || darwin

package session

import (
	"context"
	"github.com/gridconsole/grid-plugins/internal/contract"
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

func TestProcessStartRoundTrip(t *testing.T) {
	start, err := ProcessStart(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if got := ProbeLocal(Live{PID: os.Getpid(), PIDStart: contract.FormatTime(start)}); got != Alive {
		t.Fatalf("got %v, want Alive", got)
	}
	for _, pid := range []int{0, -1} {
		if _, err := ProcessStart(pid); err == nil {
			t.Fatalf("accepted pid %d", pid)
		}
	}
}
