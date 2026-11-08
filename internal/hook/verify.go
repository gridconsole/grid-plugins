package hook

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/eval"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/session"
	"go.yaml.in/yaml/v4"
)

// A saved Job is immutable input to verification. No command is taken from the
// hook's tool input, transcript, or a legacy free-form verify sentence.
func verifyStop(o Options, in hookio.Input, dir string) (int, bool) {
	id := o.Getenv("GRID_JOB")
	if id == "" || in.HookEventName != "Stop" {
		return 0, false
	}
	if !job.ValidID(id) || in.SessionID == "" {
		o.log("verify", fmt.Errorf("verification needs a valid Job and session id"))
		return 0, true
	}
	path, err := job.Within(dir, filepath.Join("jobs", id, "job.yaml"))
	if err != nil {
		o.log("verify", err)
		return 0, true
	}
	text, err := readBounded(path)
	if err != nil {
		o.log("verify", err)
		return 0, true
	}
	var raw map[string]any
	var j job.Job
	if err = yaml.Load([]byte(text), &raw); err == nil {
		err = frontmatter.Into(raw, &j)
	}
	if err != nil || j.ID != id {
		o.log("verify", fmt.Errorf("invalid saved Job metadata"))
		return 0, true
	}
	if j.Verify.Command == "" {
		return 0, false
	}
	store := session.Store{TaskDir: dir}
	a, err := store.Activity(in.SessionID)
	if err != nil {
		o.log("verify", err)
		return 0, true
	}
	key := fmt.Sprintf("%s:%s:%s:%x", id, o.Getenv("GRID_FENCE"), o.Getenv("GRID_ATTEMPT"), sha256.Sum256([]byte(fmt.Sprintf("%s:%d", j.Verify.Command, j.Verify.MinTests))))
	cap := 8
	if o.Getenv("GRID_RUNTIME") == "codex" {
		cap = 2
	}
	if a.VerifyBlocks[key] >= cap {
		return 0, true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	argv := []string{"sh", "-c", j.Verify.Command}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/C", j.Verify.Command}
	}
	var env []string
	for _, k := range []string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "TEMP", "TMP", "TMPDIR", "GOCACHE", "GOPATH", "GOROOT", "GRID_HOME", "GRID_TASK", "GRID_JOB"} {
		if v := o.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	result := eval.RunCommand(ctx, argv, in.Cwd, env)
	if ctx.Err() != nil {
		result.Status = "failed"
	}
	if result.Status == "error" {
		o.log("verify", fmt.Errorf("cannot run verification: %s", result.Error))
		return 0, true
	}
	count := testCount(result.Stdout + "\n" + result.Stderr)
	passed := result.Status == "passed" && count >= j.Verify.MinTests
	err = store.UpdateActivity(in.SessionID, func(a *session.Activity) error {
		if a.VerifyBlocks == nil {
			a.VerifyBlocks = map[string]int{}
		}
		if a.VerifyFailed == nil {
			a.VerifyFailed = map[string]bool{}
		}
		if passed {
			delete(a.VerifyBlocks, key)
			delete(a.VerifyFailed, key)
		} else {
			a.VerifyBlocks[key]++
			a.VerifyFailed[key] = true
		}
		return nil
	})
	if err != nil {
		o.log("verify", err)
		return 0, true
	}
	if passed {
		return 0, true
	}
	output := strings.TrimSpace(result.Stdout + "\n" + result.Stderr)
	runes := []rune(output)
	if len(runes) > 2000 {
		output = string(runes[len(runes)-2000:])
	}
	if o.Stderr != nil {
		fmt.Fprintf(o.Stderr, "Job %s verification failed (exit %d; %d tests observed, %d required). Fix it, then stop again.\n%s\n", id, result.ExitCode, count, j.Verify.MinTests, output)
	}
	return 2, true
}

var countSummary = regexp.MustCompile(`(?im)(?:\b([0-9]+) passed\b|\bRan ([0-9]+) tests?\b|^# pass ([0-9]+)\b)`)
var goPass = regexp.MustCompile(`(?m)^\s*--- PASS: ([^\s]+)`)
var tapPass = regexp.MustCompile(`(?m)^ok [0-9]+(?:\s|$)`)

func testCount(text string) int {
	count := 0
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		var row struct {
			Action  string
			Test    string
			Package string
		}
		if json.Unmarshal([]byte(line), &row) == nil && row.Action == "pass" && row.Test != "" {
			seen[row.Package+"/"+row.Test] = true
		}
	}
	count = len(seen)
	count = max(count, len(goPass.FindAllString(text, -1)), len(tapPass.FindAllString(text, -1)))
	for _, m := range countSummary.FindAllStringSubmatch(text, -1) {
		for _, v := range m[1:] {
			n, _ := strconv.Atoi(v)
			count = max(count, n)
		}
	}
	return count
}
