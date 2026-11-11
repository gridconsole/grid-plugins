// Package agent checks locally installed agents without starting an agent turn.
package agent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"
)

// Runner runs argv without a shell and returns stdout and the exit status.
// Implementations must honor ctx, detach standard input from any terminal, and
// kill and reap their child before returning. A start failure has status -1.
type Runner func(ctx context.Context, argv []string) (stdout string, exit int)

// Env supplies the launch PATH, user home, and optional process probe.
// A nil Runner uses exec.CommandContext. Lookup and Base are optional test seams;
// nil uses the standard resolver and process environment respectively.
type Env struct {
	PATH   string
	Home   string
	Runner Runner
	Lookup func(name string) (string, error)
	Base   []string
}

func (e Env) run(ctx context.Context, name string, argv []string) (string, int) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = e.Home
	cmd.Env = e.childEnv(name, argv[0])
	// Nil stdin/stderr connect to the null device; stdout is captured below.
	return probeOutput(ctx, cmd)
}

var systemPath = []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/opt/homebrew/bin", "/usr/local/bin"}

func (e Env) childEnv(name, binary string) []string {
	base := e.Base
	if base == nil {
		base = os.Environ()
	}
	return childEnvFor(runtime.GOOS, string(os.PathListSeparator), base, e.Home, name, binary)
}

// childEnvFor builds the probe child's environment for goos from base. It reads
// no host state, so every goos branch runs under any test host.
//
// On windows the PATH is the binary's folder, then <SYSTEMROOT>\System32 and
// <SYSTEMROOT> from base (SYSTEMROOT matched case-insensitively; none when it is
// absent or empty), so the child gets no Unix folder and keeps the system folder.
// The folder names are plain string joins on a backslash, and the binary's folder
// is cut at the last backslash or slash, because path/filepath on a non-Windows
// host does not split on a backslash. Every other goos gets the binary's folder
// followed by systemPath.
func childEnvFor(goos, listSep string, base []string, home, name, binary string) []string {
	windows := goos == "windows"
	config := "CLAUDE_CONFIG_DIR"
	if name == "codex" {
		config = "CODEX_HOME"
	}
	env := []string{}
	systemRoot := ""
	for _, entry := range base {
		key, value, _ := strings.Cut(entry, "=")
		if key == "HOME" || (windows && strings.EqualFold(key, "USERPROFILE")) {
			continue
		}
		pass := key == "USER" || key == "LOGNAME" || key == "LANG" || key == "TMPDIR" || key == "SHELL" || strings.HasPrefix(key, "LC_") || key == config
		if windows {
			switch strings.ToUpper(key) {
			case "USERNAME", "APPDATA", "LOCALAPPDATA", "SYSTEMROOT", "TEMP", "TMP", "PATHEXT", "COMSPEC":
				pass = true
			}
			if systemRoot == "" && strings.EqualFold(key, "SYSTEMROOT") {
				systemRoot = value
			}
		}
		if pass {
			env = append(env, entry)
		}
	}
	var dirs []string
	if windows {
		dirs = []string{windowsDir(binary)}
		if systemRoot != "" {
			dirs = append(dirs, strings.TrimSuffix(systemRoot, `\`)+`\System32`, systemRoot)
		}
	} else {
		dirs = append([]string{path.Dir(binary)}, systemPath...)
	}
	env = append(env, "HOME="+home, "PATH="+strings.Join(dirs, listSep))
	if windows {
		env = append(env, "USERPROFILE="+home)
	}
	return env
}

// windowsDir is the folder of a Windows path: everything before the last
// backslash or slash, the root kept whole (`C:\` and `\`), and "." when the path
// has no separator.
func windowsDir(binary string) string {
	i := strings.LastIndexAny(binary, `\/`)
	switch {
	case i < 0:
		return "."
	case i == 0 || (i == 2 && binary[1] == ':'):
		return binary[:i+1]
	}
	return binary[:i]
}

// Keep the first 64 KiB and drain the rest, matching Rust's MAX_OUTPUT.
type probeBuffer struct{ buffer bytes.Buffer }

func (b *probeBuffer) Len() int       { return b.buffer.Len() }
func (b *probeBuffer) String() string { return b.buffer.String() }

func (b *probeBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if left := 64*1024 - b.Len(); left > 0 {
		_, _ = b.buffer.Write(p[:min(left, n)])
	}
	return n, nil
}

// probeOutput runs cmd and returns its stdout and exit status. Stdout is a pipe
// this function owns, so os/exec starts no copy goroutine and Wait returns as
// soon as the direct child has exited, however long a descendant keeps the pipe.
// The child runs in its own group (iuApRun); once it has exited the group is
// ended, the write end is closed, and the read end is read to EOF. If ctx ends
// before that, the read end is closed and what was read so far returns with -1.
// No timer is involved: only the child's exit, the EOF and ctx end the wait.
func probeOutput(ctx context.Context, cmd *exec.Cmd) (string, int) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", -1
	}
	cmd.Stdout = w
	var stdout iuApSink
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		chunk := make([]byte, 32*1024)
		for {
			n, err := r.Read(chunk)
			_, _ = stdout.Write(chunk[:n])
			if err != nil {
				return
			}
		}
	}()
	runErr := iuApRun(cmd)
	// The child holds its own copy of the write end; ours must go for EOF.
	_ = w.Close()
	select {
	case <-copied:
	case <-ctx.Done():
		// A process that left the group still holds the pipe. Closing the read
		// end ends the copy; it runs in its own goroutine because on some systems
		// Close waits for a Read that is blocked.
		go func() { _ = r.Close() }()
		return stdout.String(), -1
	}
	_ = r.Close()
	if runErr != nil {
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != 0 {
			code := cmd.ProcessState.ExitCode()
			if code < 0 {
				// A signalled child exited unsuccessfully, unlike a start failure.
				// Check separately recognizes context cancellation as unknown.
				code = 1
			}
			return stdout.String(), code
		}
		return stdout.String(), -1
	}
	return stdout.String(), 0
}

// iuApSink is probeBuffer made safe to read while the copy goroutine still
// writes, which can happen when ctx ends before the pipe reaches EOF.
type iuApSink struct {
	mu     sync.Mutex
	buffer probeBuffer
}

func (s *iuApSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.Write(p)
}

func (s *iuApSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.String()
}
