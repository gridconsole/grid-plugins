package agent

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentProbeHelper(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--agent-probe-helper" {
			continue
		}
		switch os.Args[i+1] {
		case "block":
			if err := os.WriteFile(os.Args[i+2], []byte("started"), 0o600); err != nil {
				os.Exit(3)
			}
			for {
				time.Sleep(time.Hour)
			}
		case "output":
			_, _ = os.Stdout.WriteString(strings.Repeat("x", 128*1024))
			os.Exit(0)
		case "cwd":
			cwd, err := os.Getwd()
			if err != nil {
				os.Exit(3)
			}
			_, _ = os.Stdout.WriteString(cwd)
			os.Exit(0)
		case "env":
			_, _ = os.Stdout.WriteString(os.Getenv("PATH") + "\n" + os.Getenv("HOME"))
			os.Exit(0)
		case "stdin":
			if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
				os.Exit(3)
			}
			os.Exit(0)
		}
		os.Exit(4)
	}
}

func TestRunnerNullStdin(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(inputPath, []byte("parent stdin must not reach the probe"), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	// Keep this test nonparallel: a nonempty parent stdin distinguishes null
	// input from os.Stdin even when the test suite itself runs with null stdin.
	previous := os.Stdin
	os.Stdin = input
	t.Cleanup(func() {
		os.Stdin = previous
		_ = input.Close()
	})
	e := Env{Home: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), Wait)
	defer cancel()
	out, code := e.run(ctx, "claude", []string{exe, "-test.run=^TestAgentProbeHelper$", "--", "--agent-probe-helper", "stdin"})
	if code != 0 || out != "" {
		t.Fatalf("expected null stdin (immediate EOF): output %q, exit %d", out, code)
	}
}

func TestRunnerEnvironment(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	e := Env{PATH: t.TempDir(), Home: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, code := e.run(ctx, "claude", []string{exe, "-test.run=^TestAgentProbeHelper$", "--", "--agent-probe-helper", "env"})
	if code != 0 || out != wantProbePath(exe)+"\n"+e.Home {
		t.Fatalf("output %q, exit %d", out, code)
	}
}

func TestProbeOutputTimeout(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestAgentProbeHelper$", "--", "--agent-probe-helper", "block", marker)
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	start := time.Now()
	out, code := probeOutput(cmd)
	if code == 0 || ctx.Err() != context.DeadlineExceeded || time.Since(start) > 4*time.Second {
		t.Fatalf("output %q, exit %d, context %v, elapsed %s", out, code, ctx.Err(), time.Since(start))
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("helper did not start: %v", err)
	}
	if cmd.ProcessState == nil {
		t.Fatal("child was not waited on")
	}
	if state, err := cmd.Process.Wait(); err == nil || state != nil {
		t.Fatalf("child was not already reaped: %v, %v", state, err)
	}
}

func TestChildEnvironment(t *testing.T) {
	e := Env{Home: t.TempDir(), Base: []string{"HOME=ignored", "PATH=ignored", "ANTHROPIC_API_KEY=secret", "OPENAI_API_KEY=secret", "LC_ALL=C", "CLAUDE_CONFIG_DIR=claude-home", "CODEX_HOME=codex-home", "UNRELATED=no"}}
	binary := filepath.Join(t.TempDir(), "claude")
	for _, name := range []string{"claude", "codex"} {
		got := strings.Join(e.childEnv(name, binary), "\n")
		own, other := "CLAUDE_CONFIG_DIR=claude-home", "CODEX_HOME=codex-home"
		if name == "codex" {
			own, other = other, own
		}
		if !strings.Contains(got, own) || !strings.Contains(got, "LC_ALL=C") || strings.Contains(got, other) || strings.Contains(got, "secret") || strings.Contains(got, "ignored") || strings.Contains(got, "UNRELATED") {
			t.Fatalf("environment: %s", got)
		}
	}
}

func TestProbeOutputBound(t *testing.T) {
	var b probeBuffer
	input := strings.Repeat("x", 100*1024)
	for i := 0; i < 2; i++ {
		if n, err := b.Write([]byte(input)); err != nil || n != len(input) {
			t.Fatalf("write: %d %v", n, err)
		}
	}
	if b.Len() != 64*1024 {
		t.Fatalf("kept %d bytes", b.Len())
	}
}

func TestRunnerBoundedOutputAndWorkingDirectory(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := Env{Home: home}
	for _, mode := range []string{"output", "cwd"} {
		ctx, cancel := context.WithTimeout(context.Background(), Wait)
		out, code := e.run(ctx, "claude", []string{exe, "-test.run=^TestAgentProbeHelper$", "--", "--agent-probe-helper", mode})
		cancel()
		want := strings.Repeat("x", 64*1024)
		if mode == "cwd" {
			want = home
		}
		if code != 0 || out != want {
			t.Fatalf("%s: code %d, output length %d", mode, code, len(out))
		}
	}
}
