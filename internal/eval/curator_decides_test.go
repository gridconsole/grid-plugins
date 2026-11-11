package eval

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// curatorTask loads the shipped agent-curator-decides task and puts a freshly
// built grid first on PATH, because the task's setup and agents run `grid`.
func curatorTask(t *testing.T) Task {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	binary := filepath.Join(t.TempDir(), "grid")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/grid").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	t.Setenv("PATH", filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	tasks, err := LoadSuite("../../evals/tasks")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.ID == "agent-curator-decides" {
			return task
		}
	}
	t.Fatal("agent-curator-decides is not in evals/tasks")
	return Task{}
}

const curatorDecisions = `grid brain decide T-01 csv-semicolons accept --note "The export must keep semicolons for the spreadsheet." &&
grid brain decide T-01 ran-go-test reject --note "A passing test run is session noise, not a lesson."`

func runCurator(t *testing.T, task Task, script string) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	runner := Runner{Agent: []string{"sh", "-c", script}, ResultsDir: t.TempDir(), TempDir: t.TempDir()}
	result, err := runner.Run(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	if result.CleanupError != "" {
		t.Fatal(result.CleanupError)
	}
	return result
}

func failedChecks(result Result) []string {
	var ids []string
	for _, c := range result.Checks {
		if c.Status != "passed" {
			ids = append(ids, c.ID+":"+c.Status)
		}
	}
	return ids
}

func TestCuratorDecides(t *testing.T) {
	task := curatorTask(t)

	t.Run("empty agent", func(t *testing.T) {
		result := runCurator(t, task, "true")
		if result.Setup.Status != "passed" {
			t.Fatalf("setup %s: %+v", result.Setup.Status, result.Setup)
		}
		if result.Summary.GatesPassed != 0 || result.Summary.GatesTotal != 1 {
			t.Fatalf("gate all-decided passed for an agent that did nothing: %+v", result.Summary)
		}
		if result.Status != "failed" {
			t.Fatalf("status %s, want failed: %v", result.Status, failedChecks(result))
		}
	})

	t.Run("fake agent", func(t *testing.T) {
		result := runCurator(t, task, curatorDecisions)
		if result.Setup.Status != "passed" || result.AgentResult.Status != "passed" {
			t.Fatalf("setup %+v, agent %+v", result.Setup, result.AgentResult)
		}
		if result.Summary.Passed != len(task.Checks) || result.Summary.GatesPassed != 1 || result.Summary.GatesTotal != 1 {
			t.Fatalf("not every check passed: %v, %+v", failedChecks(result), result.Summary)
		}
		if result.Status != "passed" {
			t.Fatalf("status %s", result.Status)
		}
	})

	t.Run("brain file written by hand", func(t *testing.T) {
		script := curatorDecisions + ` &&
printf -- '---\nname: by-hand\n---\nWritten without grid.\n' > "$GRID_HOME/projects/notes-app/knowledge/conventions/by-hand.md"`
		result := runCurator(t, task, script)
		if result.Summary.Failed != 1 || result.Summary.GatesPassed != 1 {
			t.Fatalf("want exactly one failed check and the gate passed: %v, %+v", failedChecks(result), result.Summary)
		}
		for _, c := range result.Checks {
			if c.Status == "failed" && c.ID != "no-brain-file-written-by-hand" {
				t.Fatalf("failed check is %s, want no-brain-file-written-by-hand", c.ID)
			}
		}
	})
}
