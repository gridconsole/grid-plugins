package land

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
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

// outputPipes are the stdout and stderr pipes of one deploy command. runProcess
// owns both ends of each, so exec starts no copy goroutines and Wait returns
// when the command exits, not when its output has been copied. exec gets no
// wait delay: Cancel ends the whole group (Unix) or tree (Windows), and Wait
// has no copy goroutine left to give up on.
type outputPipes struct {
	outR, outW, errR, errW *os.File
	readers                sync.WaitGroup
}

func newOutputPipes() (*outputPipes, error) {
	p := &outputPipes{}
	var err error
	if p.outR, p.outW, err = os.Pipe(); err != nil {
		return nil, err
	}
	if p.errR, p.errW, err = os.Pipe(); err != nil {
		_ = p.outR.Close()
		_ = p.outW.Close()
		return nil, err
	}
	return p, nil
}

// drain reads r to EOF, or until r is closed, and hands every byte to w. A
// boundedBuffer accepts every write, so a full buffer never stops the reading.
func drain(r io.Reader, w io.Writer) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// start runs the readers. They run for as long as the command does, so a full
// pipe never stops it.
func (p *outputPipes) start(stdout, stderr io.Writer) {
	p.readers.Add(2)
	go func() { defer p.readers.Done(); drain(p.outR, stdout) }()
	go func() { defer p.readers.Done(); drain(p.errR, stderr) }()
}

// finish is called once the command has exited and its group is gone. It
// closes runProcess's copies of the write ends, so the readers see EOF as soon
// as nothing else holds a pipe. A descendant that left the group and still
// holds a pipe is cut off when ctx ends (the land's own timeout or the caller's
// cancel): the read ends are closed then, and what was not yet read is dropped.
// No timer decides anything here.
func (p *outputPipes) finish(ctx context.Context) {
	_ = p.outW.Close()
	_ = p.errW.Close()
	drained := make(chan struct{})
	go func() { p.readers.Wait(); close(drained) }()
	select {
	case <-drained:
	case <-ctx.Done():
		_ = p.outR.Close()
		_ = p.errR.Close()
		<-drained
	}
	_ = p.outR.Close()
	_ = p.errR.Close()
}

// runProcess bounds output and delegates tree cancellation to platform-specific code.
//
// The result follows the command's exit code: Wait returns when the command
// exits, its group is killed (cleanupProcess on Unix, the job proctree.Run ends
// on Windows), and only then are the pipes read to EOF. A background process
// that still held the output is ended with the group, so it can neither turn a
// land that passed into a failed one nor cut the output short.
func runProcess(ctx context.Context, argv []string, dir string, env []string) eval.ProcessResult {
	started := time.Now()
	result := eval.ProcessResult{Status: "error", ExitCode: -1}
	if err := ctx.Err(); err != nil {
		result.Error = err.Error()
		return result
	}
	pipes, err := newOutputPipes()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdout, cmd.Stderr = pipes.outW, pipes.errW
	configureProcess(cmd)
	var stdout, stderr boundedBuffer
	pipes.start(&stdout, &stderr)
	err = proctree.Run(cmd)
	cleanupProcess(cmd)
	pipes.finish(ctx)
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
