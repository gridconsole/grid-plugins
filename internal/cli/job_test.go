package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

func TestJobRegistered(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.tmp, "job", "--help"); code != 0 || !strings.Contains(h.out.String(), "grid job render") {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	path := h.tmp + "/brief.md"
	h.write(path, "no frontmatter\n")
	if code := h.run(h.tmp, "job", "lint", path, "--json"); code != 1 {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	var out struct {
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Findings) != 1 || out.Findings[0].Rule != "brief.frontmatter" {
		t.Fatalf("%+v", out)
	}
}

// An agent environment is not refused: the person-by-ancestry gate was removed.
func TestAgentEnvironmentJobRender(t *testing.T) {
	fixture := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	var out, errs bytes.Buffer
	env := Env{Cwd: filepath.Dir(fixture.Task), GridHome: fixture.GridHome, Machine: jobtest.Machine, Stdout: &out, Stderr: &errs,
		Getenv: func(key string) string {
			switch key {
			case "GRID_AGENT", "GRID_WORKER_ID":
				return "grid-builder"
			}
			return ""
		},
	}
	if code := Run([]string{"job", "render", "T-01", "1", "--json"}, env); code != 0 {
		t.Fatalf("exit %d: stdout %s stderr %s", code, &out, &errs)
	}
}

func TestAgentEnvironmentTaskNew(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(key string) string {
		switch key {
		case "GRID_AGENT", "GRID_WORKER_ID":
			return "grid-builder"
		}
		return ""
	}
	if code := h.run(h.home, "task", "new", "Agent made task", "--json"); code != 0 {
		t.Fatalf("exit %d: stdout %s stderr %s", code, h.out, h.errb)
	}
}

func TestJobOwnerContext(t *testing.T) {
	for _, role := range []string{"", "owner"} {
		t.Run("role="+role, func(t *testing.T) {
			fixture := jobtest.New(t, filepath.Join("..", "..", "testdata"))
			var out, errs bytes.Buffer
			env := Env{Cwd: filepath.Dir(fixture.Task), GridHome: fixture.GridHome, Machine: jobtest.Machine, Stdout: &out, Stderr: &errs,
				Getenv: func(key string) string {
					if key == "GRID_ROLE" {
						return role
					}
					return ""
				},
			}
			if code := Run([]string{"job", "render", "T-01", "1", "--json"}, env); code != 0 {
				t.Fatalf("exit %d: stdout %s stderr %s", code, &out, &errs)
			}
			var result struct{ OK bool }
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.OK {
				t.Fatalf("render failed: %s (%v)", &out, err)
			}
		})
	}
}
