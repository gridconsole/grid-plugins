package account

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/proctree"
)

const (
	maxLine   = 4 << 20         // a longer line of the provider's output is skipped, not kept
	exitGrace = time.Second     // after the answer, the child may leave by itself this long
	killWait  = 2 * time.Second // after the kill, wait this long for the output pipes
)

// lineSink is the child's stdout: it cuts the bytes into lines and hands each
// whole line to on, which must not keep it. Nothing is queued, so a child that
// writes without end cannot grow this process's memory.
type lineSink struct {
	on   func(line []byte)
	buf  []byte
	skip bool // inside a line longer than maxLine
}

func (s *lineSink) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			if !s.skip {
				s.buf = append(s.buf, p...)
				if len(s.buf) > maxLine {
					s.buf, s.skip = nil, true
				}
			}
			return n, nil
		}
		if !s.skip {
			s.buf = append(s.buf, p[:i]...)
			if len(s.buf) <= maxLine {
				s.on(bytes.TrimSuffix(s.buf, []byte{'\r'}))
			}
		}
		s.buf, s.skip, p = s.buf[:0], false, p[i+1:]
	}
	return n, nil
}

// probeChild is one provider command running for one probe. Its output is read
// by on; its stderr goes nowhere, so nothing the provider says about a sign-in
// can reach a note or a file.
type probeChild struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	cancel context.CancelFunc
	done   chan error // the command's end, once
	exited bool
}

// dropEnv removes entries named by names from environ (case-folded on Windows).
func dropEnv(environ []string, goos string, names ...string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		name, _, found := strings.Cut(kv, "=")
		drop := false
		for _, n := range names {
			if found && (name == n || (goos == "windows" && strings.EqualFold(name, n))) {
				drop = true
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

// startProbe starts bin in its own process group, in the empty folder cwd, with
// the provider's folder variable set to dir (or unset for dir ""). ctx ends the
// whole group.
func startProbe(ctx context.Context, env Env, bin string, args []string, dirVar, dir, cwd string, drop []string, on func(line []byte)) (*probeChild, error) {
	path, err := exec.LookPath(bin)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cctx, path, args...)
	cmd.Dir = cwd
	cmd.Env = providerEnv(dropEnv(env.environ(), env.goos(), drop...), dirVar, dir, env.goos())
	cmd.Stdout = &lineSink{on: on}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	configureProbe(cmd)
	cmd.WaitDelay = killWait
	p := &probeChild{cmd: cmd, stdin: stdin, cancel: cancel, done: make(chan error, 1)}
	go func() { p.done <- proctree.Run(cmd) }()
	return p, nil
}

// send writes one line to the child's stdin.
func (p *probeChild) send(line string) error {
	_, err := io.WriteString(p.stdin, line+"\n")
	return err
}

func (p *probeChild) closeStdin() { _ = p.stdin.Close() }

// stop ends the probe: the child may leave by itself for a short grace, then the
// whole group is killed, and what the child left behind goes with it.
func (p *probeChild) stop(grace time.Duration) {
	p.closeStdin()
	if !p.exited && grace > 0 {
		select {
		case <-p.done:
			p.exited = true
		case <-time.After(grace):
		}
	}
	p.cancel()
	if !p.exited {
		<-p.done
		p.exited = true
	}
	cleanupProbe(p.cmd)
}

// scratchDir is the fresh empty folder a probe runs in, so the provider finds
// no project, settings or history of the person's there. The caller removes it.
func scratchDir() (string, error) {
	return os.MkdirTemp("", "grid-usage-")
}
