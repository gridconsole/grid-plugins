package eval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
}

// Run persists failures as well as successes before deleting the workspace.
// A non-nil error means validation, workspace creation or persistence failed;
// task failures are reported by Result.Status, allowing a suite to continue.
func (r Runner) Run(ctx context.Context, task Task) (Result, error) {
	var result Result
	if err := task.Validate(); err != nil {
		return result, err
	}
	if r.ResultsDir == "" || r.Timeout < 0 || (len(r.Agent) > 0 && strings.TrimSpace(r.Agent[0]) == "") {
		return result, fmt.Errorf("results directory, nonnegative timeout and a valid agent command are required")
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
	defer os.RemoveAll(root)
	root, err = filepath.Abs(root)
	if err != nil {
		return result, err
	}
	workspace := filepath.Join(root, "workspace")
	home := filepath.Join(root, "home")
	for _, dir := range []string{workspace, home, filepath.Join(home, ".grid"), filepath.Join(home, ".config")} {
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
	env := taskEnvironment(home, workspace, task.ID)
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	result.Setup = runProcess(ctx, []string{"sh", "-eu", "-c", task.Setup}, workspace, env, "")
	if result.Setup.Status == "passed" {
		result.AgentResult = runProcess(ctx, agent, workspace, env, task.Prompt)
	}
	for _, check := range task.Checks {
		if result.Setup.Status != "passed" || result.AgentResult.Status != "passed" {
			scorer := "code"
			if check.Type == "llm_judge" {
				scorer = "judge"
			}
			result.Checks = append(result.Checks, CheckResult{Check: check, Status: "skipped", Scorer: scorer, Reason: "setup or agent did not succeed"})
			continue
		}
		result.Checks = append(result.Checks, evaluate(ctx, task, check, workspace, env, r.Judge))
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
	return result, saveResult(r.ResultsDir, result)
}

func taskEnvironment(home, workspace, id string) []string {
	overrides := map[string]string{
		"HOME": home, "USERPROFILE": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"GRID_HOME": filepath.Join(home, ".grid"), "GRID_EVAL_TASK": id, "PWD": workspace,
	}
	env := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			env = append(env, entry)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
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
