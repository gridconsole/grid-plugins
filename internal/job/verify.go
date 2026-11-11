package job

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/proctree"
)

// VerifyResult is the evidence for the L0 check. Tests is nil when output did
// not report test counts. ExitCode is the command's exit code, not the verdict:
// a command can exit zero yet fail MinTests. Reviewer checks use exit code 3.
type VerifyResult struct {
	Pass            bool   `json:"pass"`
	ExitCode        int    `json:"exit_code"`
	Tests           *int   `json:"tests"`
	Commit          string `json:"commit"`
	Reason          string `json:"reason,omitempty"`
	Output          string `json:"output,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
}

// VerifyOptions supplies the Grid and task context shared by CLI and L0 callers.
// Cwd is the fallback directory and the base for a relative dir override.
// TaskPath is an already resolved task folder, read through internal/task.
type VerifyOptions struct {
	GridHome string
	TaskPath string
	Cwd      string
}

// RunVerify executes the Job's verification and returns evidence even on failure.
// dir overrides the single recorded worktree, then options.Cwd (or process cwd).
// Named worktrees and task: proof paths are resolved here for all callers.
// Failed checks are results; invalid configuration, launch and cancellation errors
// also return an error. No Job, task or Feed record is changed.
func RunVerify(ctx context.Context, j Job, dir string, options ...VerifyOptions) (VerifyResult, error) {
	r := VerifyResult{ExitCode: -1}
	fail := func(err error) (VerifyResult, error) {
		r.Reason = strings.Join(strings.Fields(err.Error()), " ")
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(options) > 1 {
		return fail(errors.New("verify accepts at most one options value"))
	}
	var o VerifyOptions
	if len(options) == 1 {
		o = options[0]
	}
	var err error
	j, dir, err = resolveVerifyPaths(j, dir, o)
	if err != nil {
		return fail(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fail(err)
	}
	if !info.IsDir() {
		return fail(errors.New("verification directory is not a folder"))
	}
	git := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD")
	git.Dir = dir
	if b, err := git.Output(); err == nil {
		r.Commit = strings.TrimSpace(string(b))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	v := j.Verify
	if v.MinTests < 0 || (v.Command != "" && v.Kind != "") {
		return fail(errors.New("verify requires a command or kind and nonnegative min_tests"))
	}
	if v.Command == "" {
		switch v.Kind {
		case "reviewer-check":
			r.ExitCode, r.Reason = 3, "a reviewer must check this Job"
			return r, nil
		case "file-exists":
			paths := append([]string(nil), v.Paths...)
			if v.Path != "" {
				paths = append(paths, v.Path)
			}
			if len(paths) == 0 {
				return fail(errors.New("file-exists requires at least one path"))
			}
			r.ExitCode = 0
			var missing []string
			for _, path := range paths {
				if err := ctx.Err(); err != nil {
					return fail(err)
				}
				p := path
				if !filepath.IsAbs(p) {
					p = filepath.Join(dir, p)
				}
				st, err := os.Stat(p)
				if path == "" || err != nil || !st.Mode().IsRegular() {
					missing = append(missing, path)
				}
			}
			if len(missing) > 0 {
				r.ExitCode = 1
				r.Reason = "proof files missing or not regular files: " + strings.Join(strings.Fields(strings.Join(missing, ", ")), " ")
			} else {
				r.Pass = true
			}
			return r, nil
		default:
			return fail(errors.New("verify requires a command, file-exists or reviewer-check"))
		}
	}
	cmd := verifyShell(ctx, v.Command)
	cmd.Dir = dir
	output := &verifyOutput{}
	// Stdout and stderr share one pipe RunVerify owns, so exec starts no copy of
	// its own and Run returns when the shell exits, even while a process it left
	// behind still holds the pipe. That process is ended with the tree below.
	pr, pw, err := os.Pipe()
	if err != nil {
		return fail(err)
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		_, _ = io.Copy(output, pr)
	}()
	err = proctree.Run(cmd)
	cleanupVerifyProcess(cmd)
	// With the tree ended, the pipe reaches EOF once its last writer is gone. A
	// process outside the tree may hold it open until ctx ends; closing the read
	// end then ends the copy.
	_ = pw.Close()
	select {
	case <-copied:
	case <-ctx.Done():
		_ = pr.Close()
		<-copied
	}
	_ = pr.Close()
	output.finish()
	r.Tests, r.Output, r.OutputTruncated = output.tests(), output.output.String(), output.truncated
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	if err != nil {
		r.Reason = fmt.Sprintf("verification command failed (exit %d)", r.ExitCode)
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return r, nil
		}
		return fail(err)
	}
	switch {
	case r.Tests == nil && v.MinTests > 0:
		r.Reason = fmt.Sprintf("test totals unknown; at least %d tests required", v.MinTests)
	case r.Tests != nil && *r.Tests < v.MinTests:
		r.Reason = fmt.Sprintf("only %d tests; at least %d required", *r.Tests, v.MinTests)
	default:
		r.Pass = true
	}
	return r, nil
}
