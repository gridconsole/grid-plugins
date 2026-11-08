package eval

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const outputLimit = 64 * 1024

// ProcessResult retains bounded output even when the command fails or times out.
type ProcessResult struct {
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
	Error      string `json:"error,omitempty"`
}

type boundedBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := outputLimit - b.buffer.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

func runProcess(ctx context.Context, argv []string, dir string, env []string, stdin string) ProcessResult {
	started := time.Now()
	result := ProcessResult{Status: "error", ExitCode: -1}
	if err := ctx.Err(); err != nil {
		result.Error = err.Error()
		return result
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdin = dir, env, strings.NewReader(stdin)
	cmd.WaitDelay = 250 * time.Millisecond
	configureProcess(cmd)
	var stdout, stderr boundedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	cleanupProcess(cmd)
	result.DurationMS = time.Since(started).Milliseconds()
	result.Stdout, result.Stderr = stdout.buffer.String(), stderr.buffer.String()
	result.Truncated = stdout.truncated || stderr.truncated
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	switch {
	case ctx.Err() != nil:
		result.Error = ctx.Err().Error()
	case err != nil:
		result.Error = fmt.Sprint(err)
		if _, ok := err.(*exec.ExitError); ok {
			result.Status = "failed"
		}
	default:
		result.Status = "passed"
	}
	return result
}
