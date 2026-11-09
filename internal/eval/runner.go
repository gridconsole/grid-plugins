package eval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

//go:embed fake-agent.sh
var fakeAgent string

// Runner executes trusted suite scripts locally. A temporary workspace is not
// an OS sandbox: commands have the permissions of the invoking user.
type Runner struct {
	Agent      []string      // nil selects the embedded fake agent
	ResultsDir string        // required; kept outside the temporary workspace
	TempDir    string        // empty uses the OS temporary directory
	Timeout    time.Duration // whole task, including setup and checks; default 5m
	Judge      Judge
	// TraceParser reads the agent's stdout for trace checks; nil is DefaultTraceParser.
	TraceParser TraceParser
	// TraceLimit is eval_trace_stdout_limit in bytes; zero is DefaultTraceLimit.
	TraceLimit int64
	// AllowEnv names parent variables (login keys such as ANTHROPIC_API_KEY) copied
	// into the task environment when set. Nothing else is inherited; see taskEnvironment.
	AllowEnv []string

	removeAll func(string) error // test hook; nil uses removeTree
}

type Summary struct {
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Errors      int      `json:"errors"`
	Skipped     int      `json:"skipped"`
	GatesPassed int      `json:"gates_passed"`
	GatesTotal  int      `json:"gates_total"`
	CheckScore  *float64 `json:"check_score"` // weighted 0..5; null if not fully measured
}

// Result is one task attempt, not an entire corpus round. ConfigHash identifies
// the task metadata, prompt, setup and checks used for reproducible comparisons.
type Result struct {
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id"`
	Task          string        `json:"task"`
	Title         string        `json:"title"`
	Tier          string        `json:"tier"`
	Size          string        `json:"size"`
	ConfigHash    string        `json:"config_hash"`
	Agent         []string      `json:"agent"`
	Fake          bool          `json:"fake"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	DurationMS    int64         `json:"duration_ms"`
	TimeoutMS     int64         `json:"timeout_ms"`
	Status        string        `json:"status"`
	Setup         ProcessResult `json:"setup"`
	AgentResult   ProcessResult `json:"agent_result"`
	Checks        []CheckResult `json:"checks"`
	Summary       Summary       `json:"summary"`
	CleanupError  string        `json:"cleanup_error,omitempty"` // temporary folder left behind
}

// Run persists failures as well as successes after deleting the workspace.
// A non-nil error means validation, workspace creation or persistence failed;
// task failures are reported by Result.Status, allowing a suite to continue.
// A temporary folder that cannot be removed is recorded in Result.CleanupError.
func (r Runner) Run(ctx context.Context, task Task) (Result, error) {
	var result Result
	if err := task.Validate(); err != nil {
		return result, err
	}
	if r.ResultsDir == "" || r.Timeout < 0 || (len(r.Agent) > 0 && strings.TrimSpace(r.Agent[0]) == "") {
		return result, fmt.Errorf("results directory, nonnegative timeout and a valid agent command are required")
	}
	if err := ValidateAllowEnv(r.AllowEnv); err != nil {
		return result, err
	}
	if r.Timeout == 0 {
		r.Timeout = 5 * time.Minute
	}
	if err := os.MkdirAll(r.ResultsDir, 0o700); err != nil {
		return result, err
	}
	root, err := os.MkdirTemp(r.TempDir, "grid-eval-")
	if err != nil {
		return result, err
	}
	return r.runInWorkspace(ctx, task, root)
}

// runInWorkspace owns root from entry, even if attempt returns early or panics.
func (r Runner) runInWorkspace(ctx context.Context, task Task, root string) (result Result, err error) {
	remove := r.removeAll
	if remove == nil {
		remove = removeTree
	}
	completed := false
	defer func() {
		if rerr := remove(root); rerr != nil {
			rerr = fmt.Errorf("remove temporary folder %s: %w", root, rerr)
			if err != nil {
				err = errors.Join(err, rerr)
			} else {
				result.CleanupError = rerr.Error()
			}
		}
		// A panic propagates after cleanup without persisting an incomplete result.
		if completed && err == nil {
			err = saveResult(r.ResultsDir, result)
		}
	}()
	result, err = r.attempt(ctx, task, root)
	completed = true
	return result, err
}

// attempt runs one task inside tempRoot. Run removes tempRoot afterwards, so the
// saved result can say whether that worked.
func (r Runner) attempt(ctx context.Context, task Task, tempRoot string) (Result, error) {
	var result Result
	root, err := filepath.Abs(tempRoot)
	if err != nil {
		return result, err
	}
	workspace := filepath.Join(root, "workspace")
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	gridHome := filepath.Join(home, ".grid")
	for _, dir := range []string{workspace, home, tmp, gridHome, filepath.Join(gridHome, "projects"), filepath.Join(home, ".config")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return result, err
		}
	}
	started := time.Now().UTC()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return result, err
	}
	config, err := json.Marshal(struct {
		Task   Task   `json:"task"`
		Prompt string `json:"prompt"`
		Setup  string `json:"setup"`
	}{task, task.Prompt, task.Setup})
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256(config)
	result = Result{
		SchemaVersion: SchemaVersion, ID: started.Format("20060102T150405Z") + "-" + hex.EncodeToString(nonce),
		Task: task.ID, Title: task.Title, Tier: task.Tier, Size: task.Size,
		ConfigHash: hex.EncodeToString(digest[:]), Agent: append([]string{}, r.Agent...), Fake: len(r.Agent) == 0,
		StartedAt: started, TimeoutMS: r.Timeout.Milliseconds(), Status: "passed",
		AgentResult: ProcessResult{Status: "skipped", ExitCode: -1}, Checks: []CheckResult{},
	}
	agent := append([]string{}, r.Agent...)
	if result.Fake {
		result.Agent = []string{"fake"}
		agent = []string{"sh", "-c", fakeAgent}
	}
	for i := range agent {
		if agent[i] == "{prompt}" {
			agent[i] = task.Prompt
		}
	}
	env := taskEnvironment(home, workspace, tmp, task.ID, r.AllowEnv)
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	result.Setup = RunProcess(ctx, []string{"sh", "-eu", "-c", task.Setup}, workspace, env, "")
	var trace traceInput
	if result.Setup.Status == "passed" {
		result.AgentResult, trace = r.runAgent(ctx, agent, workspace, env, task.Prompt)
	}
	trace.parser = r.TraceParser
	for _, check := range task.Checks {
		if result.Setup.Status != "passed" || result.AgentResult.Status != "passed" {
			scorer := "code"
			if check.Type == "llm_judge" {
				scorer = "judge"
			}
			result.Checks = append(result.Checks, CheckResult{Check: check, Status: "skipped", Scorer: scorer, Reason: "setup or agent did not succeed"})
			continue
		}
		result.Checks = append(result.Checks, evaluate(ctx, task, check, workspace, env, r.Judge, trace))
	}
	result.Summary = summarize(result.Checks)
	if result.Setup.Status == "failed" || result.AgentResult.Status == "failed" || result.Summary.Failed > 0 {
		result.Status = "failed"
	}
	if result.Setup.Status == "error" || result.AgentResult.Status == "error" || result.Summary.Errors > 0 {
		result.Status = "error"
	}
	result.FinishedAt = time.Now().UTC()
	result.DurationMS = result.FinishedAt.Sub(started).Milliseconds()
	return result, nil
}

// runAgent runs the agent and keeps its full stdout, up to the trace limit, in
// memory. The trace checks read those bytes, never a file the agent could reach.
func (r Runner) runAgent(ctx context.Context, agent []string, workspace string, env []string, prompt string) (ProcessResult, traceInput) {
	limit := r.TraceLimit
	if limit <= 0 {
		limit = DefaultTraceLimit
	}
	sink := &cappedBuffer{limit: limit}
	process := runProcess(ctx, agent, workspace, env, prompt, sink)
	return process, traceInput{data: sink.bytes(), truncated: sink.truncated}
}

// removeTree deletes root, first making directories the agent left read-only
// writable again so they cannot leak the folder.
func removeTree(root string) error {
	if err := os.RemoveAll(root); err == nil {
		return nil
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if d != nil && d.IsDir() {
			_ = os.Chmod(path, 0o700)
		}
		return nil
	})
	return os.RemoveAll(root)
}

// taskEnvironment builds the child environment from nothing. Only PATH, LANG,
// TERM, LC_* and the variables named in allow come from the parent; HOME, TMPDIR
// and the Grid folders point into the attempt, so no GRID_*, CLAUDE_CODE_*,
// CLAUDE_ENV_FILE or messaging variable of the invoking session reaches the agent.
func taskEnvironment(home, workspace, tmp, id string, allow []string) []string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if key == "PATH" || key == "LANG" || key == "TERM" || strings.HasPrefix(key, "LC_") {
			env[key] = value
		}
	}
	for _, key := range allow {
		if value, ok := os.LookupEnv(key); ok {
			env[key] = value
		}
	}
	gridHome := filepath.Join(home, ".grid")
	for key, value := range map[string]string{
		"HOME": home, "USERPROFILE": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"), "TMPDIR": tmp,
		"GRID_HOME": gridHome, "GRID_PROJECTS": filepath.Join(gridHome, "projects"),
		"GRID_EVAL_TASK": id, "PWD": workspace,
	} {
		env[key] = value
	}
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	sort.Strings(out)
	return out
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateAllowEnv rejects names that would carry session state or undo the
// attempt's own HOME, TMPDIR and Grid folders into the agent.
func ValidateAllowEnv(names []string) error {
	for _, name := range names {
		switch {
		case !envName.MatchString(name):
			return fmt.Errorf("invalid environment variable name %q", name)
		case strings.HasPrefix(name, "GRID_"), strings.HasPrefix(name, "CLAUDE_CODE_"),
			name == "CLAUDE_ENV_FILE", name == "CLAUDE_PLUGIN_ROOT", strings.Contains(name, "MESSAGING"):
			return fmt.Errorf("environment variable %s carries session state and cannot be passed to an agent", name)
		case slices.Contains([]string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME", "TMPDIR", "PWD"}, name):
			return fmt.Errorf("environment variable %s is set per attempt and cannot be passed", name)
		}
	}
	return nil
}

func summarize(checks []CheckResult) Summary {
	var s Summary
	var points, weights float64
	complete := true
	for _, c := range checks {
		switch c.Status {
		case "passed":
			s.Passed++
		case "failed":
			s.Failed++
		case "error":
			s.Errors++
		case "skipped":
			s.Skipped++
		}
		if c.Gate {
			s.GatesTotal++
			if c.Status == "passed" {
				s.GatesPassed++
			}
			continue
		}
		if c.Score == nil {
			complete = false
			continue
		}
		weight := c.Weight
		if weight == 0 {
			weight = 1
		}
		weights += weight
		points += *c.Score * weight
	}
	if complete && weights > 0 {
		score := points / weights
		s.CheckScore = &score
	}
	return s
}
