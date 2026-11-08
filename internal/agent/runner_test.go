package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		case "env":
			_, _ = os.Stdout.WriteString(os.Getenv("PATH") + "\n" + os.Getenv("HOME"))
			os.Exit(0)
		}
		os.Exit(4)
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
	out, code := e.run(ctx, []string{exe, "-test.run=^TestAgentProbeHelper$", "--", "--agent-probe-helper", "env"})
	if code != 0 || out != e.PATH+"\n"+e.Home {
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
