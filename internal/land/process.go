package land

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/eval"
	"github.com/gridconsole/grid-plugins/internal/proctree"
)

const outputLimit = 64 * 1024

// windowsTaskkillPath uses Windows separators on every host so it can be tested
// without a Windows runtime. An unset SystemRoot must never yield a relative path.
func windowsTaskkillPath(systemRoot string) string {
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return strings.TrimRight(strings.ReplaceAll(systemRoot, "/", `\`), `\`) + `\System32\taskkill.exe`
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

// runProcess bounds output and delegates tree cancellation to platform-specific code.
func runProcess(ctx context.Context, argv []string, dir string, env []string) eval.ProcessResult {
	started := time.Now()
	result := eval.ProcessResult{Status: "error", ExitCode: -1}
	if err := ctx.Err(); err != nil {
		result.Error = err.Error()
		return result
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	cmd.WaitDelay = 250 * time.Millisecond
	configureProcess(cmd)
	var stdout, stderr boundedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := proctree.Run(cmd)
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
