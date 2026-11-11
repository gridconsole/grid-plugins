package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// Minimum versions and Wait mirror setup/agents.rs:22-29.
const (
	ClaudeMin = "2.1.236"
	CodexMin  = "0.162.0"
	Wait      = 5 * time.Second
)

// Report describes readiness; an empty Fix means the agent is ready.
// SignedIn is absent when the status was not asked or could not be determined.
type Report struct {
	Agent    string `json:"agent"`
	Found    bool   `json:"found"`
	Path     string `json:"path,omitempty"`
	Version  string `json:"version,omitempty"`
	Min      string `json:"min"`
	SignedIn *bool  `json:"signedIn,omitempty"`
	Fix      string `json:"fix,omitempty"`
}

// Check probes both agents concurrently and returns Claude then Codex, as Rust
// does. Every version/status invocation gets its own Wait deadline. Runners
// must support concurrent calls and honor their context.
func Check(ctx context.Context, env Env) []Report {
	reports := make([]Report, 2)
	var wg sync.WaitGroup
	for i, name := range []string{"claude", "codex"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reports[i] = check(ctx, env, name)
		}()
	}
	wg.Wait()
	return reports
}

// probeContext gives each probe its Wait budget. Only tests assign it, so a
// test can end a probe by an event instead of waiting out the budget.
var probeContext = func(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, Wait)
}

func check(ctx context.Context, env Env, name string) Report {
	min := ClaudeMin
	if name == "codex" {
		min = CodexMin
	}
	r := Report{Agent: name, Min: min}
	lookup := env.Lookup
	if lookup == nil {
		lookup = env.resolve
	}
	binary, err := lookup(name)
	if err != nil || binary == "" {
		r.Fix = fixLine(name, "install", false, runtime.GOOS)
		return r
	}
	r.Found, r.Path = true, gridhome.Tilde(binary, env.Home)
	if isResolvedBatchLauncher(binary) {
		r.Fix = fixLine(name, "install", false, runtime.GOOS)
		return r
	}
	probe := func(args ...string) (string, int) {
		child, cancel := probeContext(ctx)
		defer cancel()
		if child.Err() != nil {
			return "", -1
		}
		argv := append([]string{binary}, args...)
		var out string
		var code int
		if env.Runner != nil {
			out, code = env.Runner(child, argv)
		} else {
			out, code = env.run(child, name, argv)
		}
		if child.Err() != nil {
			return "", -1
		}
		return out, code
	}
	out, code := probe("--version")
	if code == 0 {
		r.Version = parseVersion(out)
	}
	if r.Version == "" {
		r.Fix = fixLine(name, "install", false, runtime.GOOS)
		return r
	}
	if !atLeast(r.Version, min) {
		r.Fix = fixLine(name, "update", fromBrew(binary), runtime.GOOS)
		return r
	}
	args := []string{"auth", "status"}
	if name == "codex" {
		args[0] = "login"
	}
	out, code = probe(args...)
	r.SignedIn = signedIn(name, out, code)
	if r.SignedIn == nil || !*r.SignedIn {
		r.Fix = fixLine(name, "signin", false, runtime.GOOS)
	}
	return r
}

func parseVersion(out string) string {
	for _, word := range strings.Fields(out) {
		word = strings.TrimPrefix(word, "v")
		if len(word) == 0 || word[0] < '0' || word[0] > '9' {
			continue
		}
		version := strings.FieldsFunc(word, func(c rune) bool { return c == '-' || c == '+' })[0]
		if _, ok := versionParts(version); ok {
			return version
		}
		return "" // Rust stops at the first digit-leading word, even if malformed.
	}
	return ""
}

func versionParts(version string) ([]uint64, bool) {
	parts := strings.Split(version, ".")
	nums := make([]uint64, len(parts))
	for i, part := range parts {
		// Rust's unsigned integer parser also accepts a leading plus.
		n, err := strconv.ParseUint(strings.TrimPrefix(part, "+"), 10, 64)
		if err != nil {
			return nil, false
		}
		nums[i] = n
	}
	return nums, true
}

func atLeast(version, min string) bool {
	have, ok := versionParts(version)
	need, valid := versionParts(min)
	if !ok || !valid {
		return false
	}
	for i := 0; i < max(len(have), len(need)); i++ {
		var h, n uint64
		if i < len(have) {
			h = have[i]
		}
		if i < len(need) {
			n = need[i]
		}
		if h != n {
			return h > n
		}
	}
	return true
}

func signedIn(name, out string, code int) *bool {
	if code < 0 {
		return nil
	}
	value := code == 0
	if name == "claude" && value {
		var status map[string]json.RawMessage
		if json.Unmarshal([]byte(out), &status) != nil {
			return nil
		}
		var method *string
		if json.Unmarshal(status["authMethod"], &method) != nil || method == nil {
			return nil
		}
		value = *method != "none"
	}
	return &value
}

func fromBrew(binary string) bool {
	real, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(real, string(filepath.Separator)) {
		if part == "Caskroom" {
			return true
		}
	}
	return false
}

func fixLine(name, fix string, brew bool, platform string) string {
	if fix == "signin" {
		if name == "claude" {
			return "claude, then /login"
		}
		return "codex login"
	}
	cask := name
	if name == "claude" {
		cask = "claude-code"
	}
	if fix == "update" {
		if brew {
			return "brew upgrade --cask " + cask
		}
		if name == "claude" {
			return "claude update"
		}
	}
	if fix == "install" && platform == "darwin" {
		return "brew install --cask " + cask
	}
	if name == "claude" {
		if platform == "windows" {
			return "irm https://claude.ai/install.ps1 | iex"
		}
		return "curl -fsSL https://claude.ai/install.sh | bash"
	}
	if platform == "windows" {
		return `powershell -ExecutionPolicy ByPass -c "irm https://chatgpt.com/codex/install.ps1 | iex"`
	}
	return "curl -fsSL https://chatgpt.com/codex/install.sh | sh"
}
