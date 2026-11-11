//go:build unix

package routine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/proctree"
)

// jpReScript writes an executable shell script as the grid binary of home.
func jpReScript(t *testing.T, home, body string) string {
	t.Helper()
	program := filepath.Join(home, "grid")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return program
}

// jpReReadToEOF reads f until EOF. The deadline only bounds the wait, and the
// failure names what was waited for. A pollable read ends at the deadline with
// ErrDeadlineExceeded; a read that cannot wait returns EAGAIN and is retried.
func jpReReadToEOF(t *testing.T, f *os.File, what string) {
	t.Helper()
	const bound = 30 * time.Second
	_ = f.SetReadDeadline(time.Now().Add(bound))
	deadline := time.After(bound)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	buf := make([]byte, 512)
	for {
		_, err := f.Read(buf)
		switch {
		case err == nil:
		case errors.Is(err, io.EOF):
			return
		case errors.Is(err, os.ErrDeadlineExceeded):
			t.Fatal(what)
		case errors.Is(err, syscall.EAGAIN):
			select {
			case <-deadline:
				t.Fatal(what)
			case <-tick.C:
			}
		default:
			t.Fatalf("read: %v", err)
		}
	}
}

// jpReWaitPid polls path until it holds a whole line with a pid.
func jpReWaitPid(t *testing.T, path, what string) int {
	t.Helper()
	deadline := time.After(30 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if b, err := os.ReadFile(path); err == nil && bytes.HasSuffix(b, []byte("\n")) {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				return pid
			}
		}
		select {
		case <-deadline:
			t.Fatal(what)
		case <-tick.C:
		}
	}
}

// A background process of the command holds the output pipe open. The run
// follows the command's exit, ends the group, and still has all the output.
func TestJpReBackgroundChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	home := gridhome.Real(t.TempDir())
	fifo := filepath.Join(t.TempDir(), "hold")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	// Open before the run: the script's open for writing waits for a reader.
	reader, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	program := jpReScript(t, home, "exec 3>'"+fifo+"'\nsleep 30 &\necho ok\n")
	var out bytes.Buffer
	p := ProcessExecutor{GridBinary: program, Home: home, Stdout: &out}
	if err := p.Execute(ctx, Store{GridHome: home}, Entry{Run: `grid example`}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := out.String(); got != "ok\n" {
		t.Fatalf("output %q want %q", got, "ok\n")
	}
	// EOF means no writer is left: the background sleep was ended with the group.
	jpReReadToEOF(t, reader, "background sleep still holds the FIFO")
}

// A Go writer gets all the output, more than a pipe holds, from one command
// that exits 0; an *os.File goes to the command as it is.
func TestJpReFullOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	home := gridhome.Real(t.TempDir())
	s := Store{GridHome: home}

	// One writer for both: even lines on stdout, odd lines on stderr.
	const pad = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	var want strings.Builder
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&want, "line %d %s\n", i, pad)
	}
	loop := "i=0\nwhile [ $i -lt 3000 ]; do\n" +
		"  if [ $((i % 2)) -eq 0 ]; then echo \"line $i " + pad + "\"; else echo \"line $i " + pad + "\" >&2; fi\n" +
		"  i=$((i+1))\ndone\n"
	program := jpReScript(t, home, loop)
	var buf bytes.Buffer
	p := ProcessExecutor{GridBinary: program, Home: home, Stdout: &buf, Stderr: &buf}
	if err := p.Execute(ctx, s, Entry{Run: `grid example`}); err != nil {
		t.Fatalf("one writer: %v", err)
	}
	if got := buf.String(); got != want.String() {
		t.Fatalf("one writer: got %d bytes and %d lines, want %d bytes and 3000 lines", len(got), strings.Count(got, "\n"), want.Len())
	}

	// Two writers: each gets its own stream.
	program = jpReScript(t, home, "echo to-out\necho to-err >&2\n")
	var out, errOut bytes.Buffer
	p = ProcessExecutor{GridBinary: program, Home: home, Stdout: &out, Stderr: &errOut}
	if err := p.Execute(ctx, s, Entry{Run: `grid example`}); err != nil {
		t.Fatalf("two writers: %v", err)
	}
	if out.String() != "to-out\n" || errOut.String() != "to-err\n" {
		t.Fatalf("two writers: stdout %q stderr %q", out.String(), errOut.String())
	}

	// An *os.File, and a nil writer, reach the command unchanged.
	program = jpReScript(t, home, "echo to-file\n")
	path := filepath.Join(t.TempDir(), "out.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var gotOut, gotErr io.Writer
	p = ProcessExecutor{GridBinary: program, Home: home, Stdout: f}
	p.runCommand = func(cmd *exec.Cmd) error {
		gotOut, gotErr = cmd.Stdout, cmd.Stderr
		return proctree.Run(cmd)
	}
	if err := p.Execute(ctx, s, Entry{Run: `grid example`}); err != nil {
		t.Fatalf("file: %v", err)
	}
	if gotOut != io.Writer(f) {
		t.Fatalf("cmd.Stdout is %T %v, want the same *os.File", gotOut, gotOut)
	}
	if gotErr != nil {
		t.Fatalf("cmd.Stderr is %T %v, want nil", gotErr, gotErr)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "to-file\n" {
		t.Fatalf("file holds %q, %v", b, err)
	}
}

// A replaced runCommand that never starts the command still returns.
func TestJpReSeamNeverStarts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	home := gridhome.Real(t.TempDir())
	var out bytes.Buffer
	p := ProcessExecutor{GridBinary: filepath.Join(home, "grid"), Home: home, Stdout: &out, Stderr: &out}
	p.runCommand = func(*exec.Cmd) error { return nil }
	if err := p.Execute(ctx, Store{GridHome: home}, Entry{Run: `grid example`}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

// jpReHelperMarker returns the marker path when this test binary was started as
// the helper of TestJpReContextEnds, and "" in a normal run.
func jpReHelperMarker() string {
	for i, a := range os.Args {
		if a == "jp-re-setsid" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// TestJpReHelper is the command of TestJpReContextEnds, not a test of its own.
// It starts a process in a session of its own that shares its stdout, records the
// pid, prints ok and exits 0, so the output pipe stays held outside the group.
func TestJpReHelper(t *testing.T) {
	marker := jpReHelperMarker()
	if marker == "" {
		return
	}
	sleep := exec.Command("sleep", "600")
	sleep.Stdout = os.Stdout
	sleep.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := sleep.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "helper:", err)
		os.Exit(2)
	}
	if err := os.WriteFile(marker, []byte(strconv.Itoa(sleep.Process.Pid)+"\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "helper:", err)
		os.Exit(2)
	}
	fmt.Println("ok")
	os.Exit(0)
}

// A process outside the group holds the output after the command exits. Only the
// context of the run ends the wait for it.
func TestJpReContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	home := gridhome.Real(t.TempDir())
	marker := filepath.Join(t.TempDir(), "marker")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	program := jpReScript(t, home, "exec '"+self+"' -test.run='^TestJpReHelper$' -- jp-re-setsid '"+marker+"'\n")
	var out bytes.Buffer
	p := ProcessExecutor{GridBinary: program, Home: home, Stdout: &out}
	done := make(chan error, 1)
	go func() { done <- p.Execute(ctx, Store{GridHome: home}, Entry{Run: `grid example`}) }()

	pid := jpReWaitPid(t, marker, "helper marker was not written")
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	select {
	case err := <-done:
		t.Fatalf("Execute returned before the context ended: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Execute returned %v, want an error that wraps context.Canceled", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Execute did not return after cancel")
	}
}
