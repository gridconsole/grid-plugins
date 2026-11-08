// Package agent checks locally installed agents without starting an agent turn.
package agent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner runs argv without a shell and returns stdout and the exit status.
// Implementations must honor ctx, detach standard input from any terminal, and
// kill and reap their child before returning. A start failure has status -1.
type Runner func(ctx context.Context, argv []string) (stdout string, exit int)

// Env supplies the launch PATH, user home, and optional process probe.
// A nil Runner uses exec.CommandContext; an empty PATH searches no directories.
type Env struct {
	PATH   string
	Home   string
	Runner Runner
}

func (e Env) run(ctx context.Context, argv []string) (string, int) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// Preserve launch credentials and other settings, overriding only the inputs
	// supplied by Env. Cmd's nil stdin and stderr connect to the null device.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PATH") || strings.EqualFold(key, "HOME") || strings.EqualFold(key, "USERPROFILE") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "PATH="+e.PATH, "HOME="+e.Home, "USERPROFILE="+e.Home)
	return probeOutput(cmd)
}

func probeOutput(cmd *exec.Cmd) (string, int) {
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Also bound pipe draining if a descendant holds stdout open after the
	// direct child has exited. Run always waits for and reaps the direct child.
	cmd.WaitDelay = 100 * time.Millisecond
	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != 0 {
			return stdout.String(), cmd.ProcessState.ExitCode()
		}
		return stdout.String(), -1
	}
	return stdout.String(), 0
}
