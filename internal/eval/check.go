package eval

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// Judge is the only model-scoring hook. Implementations choose their runtime,
// credentials and budget, respect ctx, and return evidence from the workspace.
// No judge is installed by default; a requested judge without one is an error.
type Judge interface {
	Evaluate(ctx context.Context, request JudgeRequest) (Judgement, error)
}

type JudgeRequest struct {
	Task      Task
	Check     Check
	Workspace string
}

type Judgement struct {
	Passed   bool
	Score    float64 // 0 through 5
	Reason   string
	Evidence string // workspace-relative path:line, when available
}

type CheckResult struct {
	Check
	Status   string         `json:"status"`
	Scorer   string         `json:"scorer"`
	Score    *float64       `json:"score,omitempty"`
	Reason   string         `json:"reason"`
	Evidence string         `json:"evidence,omitempty"`
	Process  *ProcessResult `json:"process,omitempty"`
}

func evaluate(ctx context.Context, task Task, c Check, dir string, env []string, judge Judge) CheckResult {
	r := CheckResult{Check: c, Scorer: "code", Status: "error"}
	if c.Type == "llm_judge" {
		r.Scorer = "judge"
	}
	if err := ctx.Err(); err != nil {
		r.Reason = err.Error()
		return r
	}
	switch c.Type {
	case "file_exists", "text_contains":
		// os.Root confines filesystem checks even if the agent plants symlinks.
		root, err := os.OpenRoot(dir)
		if err != nil {
			r.Reason = err.Error()
			return r
		}
		defer root.Close()
		info, err := root.Stat(c.Path)
		if err != nil {
			r.Reason = err.Error()
			if os.IsNotExist(err) {
				mark(&r, false)
			}
			return r
		}
		if !info.Mode().IsRegular() {
			r.Reason = "path is not a regular file"
			mark(&r, false)
			return r
		}
		if c.Type == "file_exists" {
			r.Reason, r.Evidence = "file exists", c.Path
			mark(&r, true)
			return r
		}
		f, err := root.Open(c.Path)
		if err != nil {
			r.Reason = err.Error()
			return r
		}
		defer f.Close()
		const maxText = 1024 * 1024
		data, err := io.ReadAll(io.LimitReader(f, maxText+1))
		if err != nil || len(data) > maxText {
			r.Reason = "cannot read text or file exceeds 1 MiB"
			return r
		}
		index := strings.Index(string(data), c.Text)
		mark(&r, index >= 0)
		r.Reason = "text not found"
		if index >= 0 {
			r.Reason = "text found"
			r.Evidence = fmt.Sprintf("%s:%d", c.Path, 1+strings.Count(string(data[:index]), "\n"))
		}
	case "command_passes":
		p := runProcess(ctx, c.Command, dir, env, "")
		r.Process, r.Status, r.Reason = &p, p.Status, p.Error
		if p.Status != "error" {
			mark(&r, p.Status == "passed")
		}
		if p.Status == "passed" {
			r.Reason = "command exited zero"
		}
	case "llm_judge":
		if judge == nil {
			r.Reason = "LLM judge is not configured"
			return r
		}
		judgement, err := judge.Evaluate(ctx, JudgeRequest{Task: task, Check: c, Workspace: dir})
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			r.Reason = err.Error()
			return r
		}
		if math.IsNaN(judgement.Score) || math.IsInf(judgement.Score, 0) || judgement.Score < 0 || judgement.Score > 5 {
			r.Reason = "judge score must be between 0 and 5"
			return r
		}
		mark(&r, judgement.Passed)
		r.Score, r.Reason, r.Evidence = &judgement.Score, judgement.Reason, judgement.Evidence
	}
	return r
}

func mark(r *CheckResult, passed bool) {
	score := 0.0
	r.Status = "failed"
	if passed {
		r.Status, score = "passed", 5
	}
	r.Score = &score
}
