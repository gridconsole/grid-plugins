package eval

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gridconsole/grid-plugins/internal/proctree"
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

// RunProcess executes a bounded command with process-group cancellation on macOS and Linux.
func RunProcess(ctx context.Context, argv []string, dir string, env []string, stdin string) ProcessResult {
	return runProcess(ctx, argv, dir, env, stdin, nil)
}

// childPipes are the three pipes of one child. The runner owns both ends of
// each, so exec starts no copy goroutines and Wait returns when the child
// exits, not when the output has been copied. There is no WaitDelay: nothing
// else needs it, because Cancel ends the whole group (Unix) or tree (Windows)
// and Wait has no copy goroutine left to give up on.
type childPipes struct {
	inR, inW, outR, outW, errR, errW *os.File
	readers                          sync.WaitGroup
	written                          chan struct{}
}

func newChildPipes() (*childPipes, error) {
	p := &childPipes{written: make(chan struct{})}
	for _, pair := range [][2]**os.File{{&p.inR, &p.inW}, {&p.outR, &p.outW}, {&p.errR, &p.errW}} {
		var err error
		if *pair[0], *pair[1], err = os.Pipe(); err != nil {
			p.closeAll()
			return nil, err
		}
	}
	return p, nil
}

func (p *childPipes) closeAll() {
	for _, f := range []*os.File{p.inR, p.inW, p.outR, p.outW, p.errR, p.errW} {
		if f != nil {
			_ = f.Close()
		}
	}
}

// drain reads r to EOF, or until r is closed, and hands every byte to w.
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

// start runs the readers and the stdin writer. They run for as long as the
// child does, so a full pipe never stops it.
func (p *childPipes) start(stdin string, stdout, stderr io.Writer) {
	p.readers.Add(2)
	go func() { defer p.readers.Done(); drain(p.outR, stdout) }()
	go func() { defer p.readers.Done(); drain(p.errR, stderr) }()
	go func() {
		defer close(p.written)
		_, _ = io.WriteString(p.inW, stdin)
		_ = p.inW.Close()
	}()
}

// finish is called once the child has exited and its group is gone. It closes
// the runner's copies of the child's ends, so the readers see EOF as soon as
// nothing else holds a pipe, and it ends a stdin write that nobody read. A
// descendant that left the group and still holds a pipe is cut off when ctx
// ends: the read ends are closed then, and what was not yet read is dropped.
// No timer decides anything here.
func (p *childPipes) finish(ctx context.Context) {
	for _, f := range []*os.File{p.inR, p.outW, p.errW, p.inW} {
		_ = f.Close()
	}
	<-p.written
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

// runProcess also copies the full stdout to tee when it is set; the stored
// ProcessResult.Stdout keeps its outputLimit cap either way.
//
// The result follows the child's exit: Wait returns, the child's group is
// killed (cleanupProcess on Unix, the job proctree.Run ends on Windows), and
// only then are the pipes read to EOF, so a background child of the command
// cannot hold the output back and the output is complete.
func runProcess(ctx context.Context, argv []string, dir string, env []string, stdin string, tee io.Writer) ProcessResult {
	started := time.Now()
	result := ProcessResult{Status: "error", ExitCode: -1}
	if err := ctx.Err(); err != nil {
		result.Error = err.Error()
		return result
	}
	pipes, err := newChildPipes()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = pipes.inR, pipes.outW, pipes.errW
	configureProcess(cmd)
	var stdout, stderr boundedBuffer
	var stdoutSink io.Writer = &stdout
	if tee != nil {
		stdoutSink = io.MultiWriter(&stdout, tee)
	}
	pipes.start(stdin, stdoutSink, &stderr)
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

// RunCommand exposes the existing bounded, cancellable process runner to hook
// verification. Callers supply argv, cwd and the complete child environment.
func RunCommand(ctx context.Context, argv []string, dir string, env []string) ProcessResult {
	if len(argv) == 0 {
		return ProcessResult{Status: "error", ExitCode: -1, Error: "verification command is empty"}
	}
	return RunProcess(ctx, argv, dir, env, "")
}
