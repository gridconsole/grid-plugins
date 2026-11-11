package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

const (
	thirdTaskID  = "01ARZ3NDEKTSV4RRFFQ69G5FAY"
	regProjectID = "01ARZ3NDEKTSV4RRFFQ69G5FB1"
)

// workerNextSpawnLines runs the real `worker next --json` in a child process, as
// TestWorkerNextStreamsSpawnLinesUntilSIGINT does, until it has printed one spawn
// line for each of want tasks. It returns the raw lines by task id and stderr.
func workerNextSpawnLines(t *testing.T, h *harness, want int) (map[string]string, string) {
	t.Helper()
	pid, start := taskLeaseIdentity(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkerNextProcess$")
	cmd.Env = append(os.Environ(), "GRID_TEST_WORKER_NEXT=1", "GRID_TEST_HOME="+h.gridHome,
		"GRID_TEST_PID="+strconv.Itoa(pid), "GRID_TEST_PID_START="+start)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	stop := func() string {
		_ = cmd.Process.Kill()
		go func() {
			for range lines {
			}
		}()
		_ = cmd.Wait()
		return stderr.String()
	}
	got := map[string]string{}
	deadline := time.After(30 * time.Second)
	for len(got) < want {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("stream ended before %d spawn lines: %v\nstderr: %s", want, got, stop())
			}
			var line struct {
				Spawn struct {
					Task string `json:"task"`
				} `json:"spawn"`
			}
			if err := json.Unmarshal([]byte(l), &line); err != nil || line.Spawn.Task == "" {
				t.Fatalf("want one spawn object per line, got %q (%v)\nstderr: %s", l, err, stop())
			}
			got[line.Spawn.Task] = l
		case <-deadline:
			t.Fatalf("no spawn lines within 30s: %v\nstderr: %s", got, stop())
		}
	}
	return got, stop()
}

// workerReads is the pair of keys the app worker decodes from a spawn line
// (kernel/worker/job.go: Model and Effort, json "model" and "effort").
type workerReads struct {
	Spawn struct {
		Model  string `json:"model"`
		Effort string `json:"effort"`
	} `json:"spawn"`
}

func spawnModel(t *testing.T, raw string) (model, effort string) {
	t.Helper()
	var r workerReads
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	return r.Spawn.Model, r.Spawn.Effort
}

// Rows 1 and 6: a builder Job (the fixture's role) is spawned on the catalog's
// sonnet and medium, under the two keys the app worker reads.
func TestWorkerNextSpawnCarriesTheCatalogModel(t *testing.T) {
	h, first := feedHarness(t)
	ownerPost(t, first, feedTaskID, "J-1")
	got, stderr := workerNextSpawnLines(t, h, 1)
	if m, e := spawnModel(t, got[feedTaskID]); m != "sonnet" || e != "medium" {
		t.Errorf("builder spawn model/effort = %q/%q, want sonnet/medium\nline: %s\nstderr: %s", m, e, got[feedTaskID], stderr)
	}
}

// Row 3: a person's `grid agent set` is what the Job runs on; a project's value wins in its own tasks only;
// a project folder with no record has the personal value and no settings error.
func TestWorkerNextSpawnFollowsPersonalAndProjectSettings(t *testing.T) {
	h, unregistered := feedHarness(t)
	if err := (settings.Store{Home: h.gridHome}).SetAgent("grid-builder", settings.Agent{Model: "opus"}); err != nil {
		t.Fatal(err)
	}
	reg := filepath.Join(h.gridHome, "projects", "reg", "tasks", "T-08-reg")
	other := filepath.Join(h.gridHome, "projects", "other", "tasks", "T-09-other")
	h.write(filepath.Join(reg, "task.md"), "---\nid: "+secondTaskID+"\n---\n")
	h.write(filepath.Join(other, "task.md"), "---\nid: "+thirdTaskID+"\n---\n")
	for name, id := range map[string]string{"reg": regProjectID, "other": "01ARZ3NDEKTSV4RRFFQ69G5FB2"} {
		if err := project.Write(h.gridHome, project.Record{Name: name, ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := (settings.Store{Home: h.gridHome, Project: "reg"}).SetAgent("grid-builder", settings.Agent{Model: "haiku"}); err != nil {
		t.Fatal(err)
	}
	ownerPost(t, unregistered, feedTaskID, "J-1")
	ownerPost(t, reg, secondTaskID, "J-1")
	ownerPost(t, other, thirdTaskID, "J-1")
	got, stderr := workerNextSpawnLines(t, h, 3)
	for name, c := range map[string]struct{ task, model string }{
		"unregistered project folder": {feedTaskID, "opus"},
		"project with its own model":  {secondTaskID, "haiku"},
		"another registered project":  {thirdTaskID, "opus"},
	} {
		if m, e := spawnModel(t, got[c.task]); m != c.model || e != "medium" {
			t.Errorf("%s: model/effort = %q/%q, want %s/medium\nline: %s", name, m, e, c.model, got[c.task])
		}
	}
	if strings.Contains(stderr, "settings") || strings.Contains(stderr, "unknown project") {
		t.Errorf("stderr names a settings error:\n%s", stderr)
	}
}
