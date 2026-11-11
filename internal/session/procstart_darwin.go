//go:build darwin

package session

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// procStart reads the process start time in UTC at ps's second precision.
func procStart(pid int) (time.Time, error) {
	observed, err := processStartText(pid)
	if err != nil {
		return time.Time{}, err
	}
	return processid.ParseLstart(observed, time.UTC)
}

// processStartText runs ps in UTC so the line parses without a zone guess.
func processStartText(pid int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	cmd.Env = append(os.Environ(), "LC_ALL=C", "TZ=UTC")
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}

// The darwin pair for localHost: ps in UTC, and ps as processid.Start runs it.
var (
	osLstartUTC   = processStartText
	osLstartLocal = processid.PSLstart
)
