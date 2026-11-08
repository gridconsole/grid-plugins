package routine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gridconsole/grid-plugins/internal/brain"
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
	Timeout        time.Duration
}

func (p ProcessExecutor) Execute(ctx context.Context, s Store, e Entry) error {
	argv, err := ParseRun(e.Run)
	if err != nil {
		return err
	}
	timeout := p.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
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
			return fmt.Errorf("grid executable is required")
		}
	} else {
		program = "claude"
		prompt := "Run the grid:" + argv[1] + " skill once. Work only in the Grid data folder."
		if s.Project != "" {
			prompt += " The project is " + s.Project + "."
		}
		args = []string{"-p", prompt, "--allowedTools", "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*)", "--max-budget-usd", "1", "--max-turns", "10", "--permission-mode", "dontAsk"}
	}
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = s.GridHome
	cmd.Stdout, cmd.Stderr = p.Stdout, p.Stderr
	cmd.Env = []string{"HOME=" + p.Home, "GRID_HOME=" + s.GridHome}
	for _, key := range []string{"PATH", "USERPROFILE", "SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL"} {
		if v := get(key); v != "" {
			cmd.Env = append(cmd.Env, key+"="+v)
		}
	}
	cmd.WaitDelay = 2 * time.Second
	configureProcess(cmd)
	defer cleanupProcess(cmd)
	err = cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("routine timed out or was cancelled: %w", ctx.Err())
	}
	return err
}
