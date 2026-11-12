package routine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/proctree"
)

func identity(home, project, name string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(home) + "\x00" + project + "\x00" + name))
	return hex.EncodeToString(sum[:16])
}

// ParseRun accepts a grid argv (optional grid prefix), or one skill name.
// Quoting groups arguments; no shell, expansion, pipeline or redirection runs.
func ParseRun(text string) ([]string, error) {
	if len(text) > 8192 || strings.ContainsAny(text, "\r\n\x00") {
		return nil, fmt.Errorf("run must be one line of at most 8192 bytes")
	}
	var args []string
	var word strings.Builder
	var quote rune
	escaped, started := false, false
	for _, r := range text {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		if strings.ContainsRune(";|&<>`", r) {
			return nil, fmt.Errorf("run accepts grid arguments or a skill, not shell operators")
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("run has unfinished quoting")
	}
	if started {
		args = append(args, word.String())
	}
	if len(args) == 0 || args[0] == "" {
		return nil, fmt.Errorf("run is required")
	}
	if args[0] == "grid" {
		args = args[1:]
		if len(args) == 0 {
			return nil, fmt.Errorf("grid needs a verb")
		}
		args = append([]string{"grid"}, args...)
	} else if len(args) > 1 {
		args = append([]string{"grid"}, args...)
	} else {
		skill := strings.TrimPrefix(args[0], "grid:")
		if !brain.ValidName(skill) {
			return nil, fmt.Errorf("skill must be a plain name")
		}
		return []string{"skill", skill}, nil
	}
	if !brain.ValidName(args[1]) || args[1] == "routine" {
		return nil, fmt.Errorf("routine commands cannot recursively invoke routines")
	}
	return args, nil
}

// ProcessExecutor bounds unattended work and passes only an explicit environment.
// Claude reads its normal user login; arbitrary parent API keys and task lease
// credentials are not inherited. Tests inject an Executor and never start agents.
type ProcessExecutor struct {
	GridBinary     string
	Home           string
	Getenv         func(string) string
	Stdout, Stderr io.Writer
	// Zero fields use the entry policy, including its defaults.
	Timeout  time.Duration
	MaxTurns int
	Budget   float64
	// Tests replace process execution and the timeout clock without sleeping.
	runCommand  func(*exec.Cmd) error
	withTimeout func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

func (p ProcessExecutor) Execute(ctx context.Context, s Store, e Entry) error {
	_, err := p.execute(ctx, s, e)
	return err
}

// ExecuteTask runs a routine and, for a workflow routine, returns the id of the
// task it started. That task posts its own first Job and carries on by itself.
func (p ProcessExecutor) ExecuteTask(ctx context.Context, s Store, e Entry) (string, error) {
	return p.execute(ctx, s, e)
}

// workflowArgv is the grid command a workflow routine runs: one new task named
// after the routine, in the routine's project, or Personal for a home routine.
func workflowArgv(s Store, e Entry) []string {
	project := s.Project
	if project == "" {
		project = gridhome.Personal
	}
	return []string{"grid", "task", "new", e.Name, "--workflow", e.Workflow, "--project", project, "--json"}
}

func (p ProcessExecutor) execute(ctx context.Context, s Store, e Entry) (string, error) {
	var argv []string
	var err error
	if e.Workflow != "" {
		argv = workflowArgv(s, e)
	} else if argv, err = ParseRun(e.Run); err != nil {
		return "", err
	}
	limits, err := e.EffectiveLimits()
	if err != nil {
		return "", err
	}
	if p.Timeout == 0 {
		p.Timeout, _ = time.ParseDuration(limits.Timeout)
	}
	if p.MaxTurns == 0 {
		p.MaxTurns = limits.MaxTurns
	}
	if p.Budget == 0 {
		p.Budget = limits.Budget
	}
	withTimeout := p.withTimeout
	if withTimeout == nil {
		withTimeout = context.WithTimeout
	}
	ctx, cancel := withTimeout(ctx, p.Timeout)
	defer cancel()
	get := p.Getenv
	if get == nil {
		get = os.Getenv
	}
	var program string
	var args []string
	if argv[0] == "grid" {
		program = p.GridBinary
		args = argv[1:]
		if program == "" {
			return "", fmt.Errorf("grid executable is required")
		}
	} else {
		program = "claude"
		prompt := "Run the grid:" + argv[1] + " skill once. Work only in the Grid data folder."
		if s.Project != "" {
			prompt += " The project is " + s.Project + "."
		}
		args = []string{"-p", prompt, "--allowedTools", "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*)", "--max-budget-usd", strconv.FormatFloat(p.Budget, 'f', -1, 64), "--max-turns", strconv.Itoa(p.MaxTurns), "--permission-mode", "dontAsk"}
	}
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = s.GridHome
	cmd.Env = []string{"HOME=" + p.Home, "GRID_HOME=" + s.GridHome}
	for _, key := range []string{"PATH", "USERPROFILE", "SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL", "APPDATA", "LOCALAPPDATA", "USERNAME", "PATHEXT", "ComSpec"} {
		if v := get(key); v != "" {
			cmd.Env = append(cmd.Env, key+"="+v)
		}
	}
	configureProcess(cmd)
	stdout, stderr := p.Stdout, p.Stderr
	var printed *bytes.Buffer
	if e.Workflow != "" {
		printed = &bytes.Buffer{}
		stdout, stderr = keepStdout(printed, stdout, stderr)
	}
	output, err := startRoutineOutput(cmd, stdout, stderr)
	if err != nil {
		return "", err
	}
	runCommand := p.runCommand
	if runCommand == nil {
		runCommand = proctree.Run
	}
	err = runCommand(cmd)
	// The result follows the child's exit. Its group is ended first, so a
	// background process that still holds the output cannot hold the result back,
	// and only then is the output read to its end.
	cleanupProcess(cmd)
	output.finish(ctx)
	if ctx.Err() != nil {
		return "", fmt.Errorf("routine timed out or was cancelled: %w", ctx.Err())
	}
	if err != nil || printed == nil {
		return "", err
	}
	id := printedTaskID(printed.Bytes())
	if id == "" {
		return "", fmt.Errorf("grid task new printed no task id")
	}
	return id, nil
}

// keepStdout copies the child's stdout into kept as well as to stdout. When one
// writer takes both streams, a shared lock keeps its Write to one goroutine at a
// time, as exec does for one writer given twice.
func keepStdout(kept *bytes.Buffer, stdout, stderr io.Writer) (io.Writer, io.Writer) {
	if stdout == nil {
		return kept, stderr
	}
	both := io.MultiWriter(kept, stdout)
	if stderr != nil && routineSameWriter(stdout, stderr) {
		mu := &sync.Mutex{}
		return lockedWriter{mu, both}, lockedWriter{mu, stderr}
	}
	return both, stderr
}

type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l lockedWriter) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(b)
}

// printedTaskID reads the id of the JSON object `grid task new --json` prints.
func printedTaskID(b []byte) string {
	dec := json.NewDecoder(bytes.NewReader(b))
	for {
		var v struct {
			ID string `json:"id"`
		}
		if dec.Decode(&v) != nil {
			return ""
		}
		if v.ID != "" {
			return v.ID
		}
	}
}

// routineOutput owns the pipes that carry a command's output to a Go writer.
// When a writer is not an *os.File, exec would copy the output itself, in a
// goroutine that Wait waits for, so a descendant holding the pipe would keep Wait
// from returning. Here the command gets the write end of a pipe this file owns,
// so exec starts no copy and Wait returns when the child exits; the copies run
// from the read ends, and finish reads them to the end after the group is gone.
// There is no timer: only the routine's own context cuts a drain short.
type routineOutput struct {
	copies  sync.WaitGroup
	readers []*os.File
	writers []*os.File
}

// startRoutineOutput sets cmd.Stdout and cmd.Stderr. A writer that is nil or an
// *os.File goes to the command unchanged, as exec takes it: nothing is copied,
// so nothing is waited for. Any other writer gets a pipe and one copy goroutine.
// The same writer for both gets one pipe and one goroutine, as exec does, so its
// Write is never called from two goroutines.
func startRoutineOutput(cmd *exec.Cmd, stdout, stderr io.Writer) (*routineOutput, error) {
	o := &routineOutput{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	owned := func(w io.Writer) bool {
		if w == nil {
			return false
		}
		_, isFile := w.(*os.File)
		return !isFile
	}
	var outWrite *os.File
	if owned(stdout) {
		w, err := o.pipe(stdout)
		if err != nil {
			o.finish(nil)
			return nil, fmt.Errorf("routine output: %w", err)
		}
		cmd.Stdout, outWrite = w, w
	}
	if owned(stderr) {
		if outWrite != nil && routineSameWriter(stdout, stderr) {
			cmd.Stderr = outWrite
		} else {
			w, err := o.pipe(stderr)
			if err != nil {
				o.finish(nil)
				return nil, fmt.Errorf("routine output: %w", err)
			}
			cmd.Stderr = w
		}
	}
	return o, nil
}

// routineSameWriter is exec's own test for one writer given twice. A writer
// whose type cannot be compared counts as different.
func routineSameWriter(a, b io.Writer) (same bool) {
	defer func() {
		if recover() != nil {
			same = false
		}
	}()
	return a == b
}

// pipe starts the copy of a new pipe into w and returns the end for the command.
func (o *routineOutput) pipe(w io.Writer) (*os.File, error) {
	r, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	o.readers = append(o.readers, r)
	o.writers = append(o.writers, pw)
	o.copies.Add(1)
	go func() {
		defer o.copies.Done()
		_, _ = io.Copy(w, r)
	}()
	return pw, nil
}

// finish is called once the command is over and its group is gone. It closes the
// write ends this side holds, so the copies see EOF as soon as nothing else holds
// a pipe, and waits for them. A process that left the group and still holds a
// pipe is cut off when ctx ends: the read ends are closed then, and what was not
// read yet is dropped. A nil ctx never ends. finish also serves a command that
// never started: the copies then end at once.
func (o *routineOutput) finish(ctx context.Context) {
	for _, f := range o.writers {
		_ = f.Close()
	}
	drained := make(chan struct{})
	go func() {
		o.copies.Wait()
		close(drained)
	}()
	var ended <-chan struct{}
	if ctx != nil {
		ended = ctx.Done()
	}
	select {
	case <-drained:
	case <-ended:
		for _, f := range o.readers {
			_ = f.Close()
		}
		<-drained
	}
	for _, f := range o.readers {
		_ = f.Close()
	}
}
