package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

const gyJwBody = `{"error":"job_waiting","code":"job_waiting","job":"J-2","waiting_for":"J-1","reason":"waiting for J-1 to be accepted"}`

// gyJwEpoch is the task holder's lease epoch, read before the run turns into a person.
var gyJwEpoch string

const gyJwStderr = "J-2 cannot be answered yet: waiting for J-1 to be accepted\n"

// gyJwHarness posts person Job J-2 of the given kind after the agent Job J-1
// (not accepted), then runs the next commands as an unleased owner.
func gyJwHarness(t *testing.T, kind string) (*harness, string, string) {
	t.Helper()
	h, path, _ := orchestratorHarness(t)
	gyJwEpoch = os.Getenv("GRID_LEASE_EPOCH")
	t.Setenv("HOME", h.home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(h.home, ".codex"))
	t.Setenv("GRID_HOME", h.gridHome)
	text := "# Plan\n\nShip the parser.\n"
	for name, content := range map[string]string{
		"plan.md":   text,
		"intent.md": "# Intent\n\nShip the parser.\n",
	} {
		if err := os.WriteFile(filepath.Join(path, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	asPerson(t, "member")
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, "J-2", []any{"J-1"})), &body); err != nil {
		t.Fatal(err)
	}
	body["kind"] = kind
	out, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", string(out)); code != 0 {
		t.Fatalf("post J-2: %d %s", code, h.errb.String())
	}
	asPerson(t, "owner")
	h.out.Reset()
	h.errb.Reset()
	return h, path, text
}

// gyJwFeedBytes is the total size of the task's Feed record files (*.jsonl); the
// read cache under feed/.cache grows on any read and holds no record.
func gyJwFeedBytes(t *testing.T, path string) int64 {
	t.Helper()
	var n int64
	err := filepath.Walk(filepath.Join(path, "feed"), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".cache" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			n += info.Size()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// gyJwRun runs the command and returns exit, stdout and stderr.
func gyJwRun(h *harness, path string, args ...string) (int, string, string) {
	h.out.Reset()
	h.errb.Reset()
	code := h.run(path, args...)
	return code, h.out.String(), h.errb.String()
}

// gyJwRefusal runs args with --json and without, and requires the same exit and
// stderr with nothing on stdout in both. It returns exit and stderr.
func gyJwRefusal(t *testing.T, h *harness, path string, args ...string) (int, string) {
	t.Helper()
	code, out, errs := gyJwRun(h, path, append(append([]string{}, args...), "--json")...)
	if code == 0 || out != "" {
		t.Fatalf("--json: exit %d, stdout %q, stderr %q", code, out, errs)
	}
	pcode, pout, perrs := gyJwRun(h, path, args...)
	if pcode != code || perrs != errs || pout != "" {
		t.Fatalf("plain: exit %d stdout %q stderr %q; --json: exit %d stderr %q", pcode, pout, perrs, code, errs)
	}
	return code, errs
}

func TestGyJwJobResultJSON(t *testing.T) {
	h, path, _ := gyJwHarness(t, "ask")
	before := gyJwFeedBytes(t, path)
	code, out, errs := gyJwRun(h, path, "job", "result", "J-2", "--body", `{"answer":"yes"}`, "--json")
	if code != 3 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if out != gyJwBody+"\n" {
		t.Fatalf("stdout %q, want %q", out, gyJwBody+"\n")
	}
	if !strings.Contains(errs, gyJwStderr) {
		t.Fatalf("stderr %q", errs)
	}
	jsonErr := errs
	code, out, errs = gyJwRun(h, path, "job", "result", "J-2", "--body", `{"answer":"yes"}`)
	if code != 3 || out != "" || errs != jsonErr {
		t.Fatalf("plain: exit %d, stdout %q, stderr %q, want stderr %q", code, out, errs, jsonErr)
	}
	if after := gyJwFeedBytes(t, path); after != before {
		t.Fatalf("Feed grew from %d to %d bytes", before, after)
	}
}

func TestGyJwPlanHashJSON(t *testing.T) {
	h, path, text := gyJwHarness(t, "approve")
	before := gyJwFeedBytes(t, path)
	args := []string{"job", "result", "J-2", "--body", `{"answer":"approved"}`, "--plan-hash", plan.Hash(text)}
	code, out, errs := gyJwRun(h, path, append(args, "--json")...)
	if code != 3 || out != gyJwBody+"\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	jsonErr := errs
	code, out, errs = gyJwRun(h, path, args...)
	if code != 3 || out != "" || errs != jsonErr || !strings.Contains(errs, gyJwStderr) {
		t.Fatalf("plain: exit %d, stdout %q, stderr %q, want %q", code, out, errs, jsonErr)
	}
	if after := gyJwFeedBytes(t, path); after != before {
		t.Fatalf("Feed grew from %d to %d bytes", before, after)
	}
}

// A Job that is not a person's, an accepted person Job and an answer over the cap
// keep their exit and stderr and print no body.
func TestGyJwOtherRefusalsPrintNoBody(t *testing.T) {
	t.Run("not a person Job", func(t *testing.T) {
		h, path, _ := gyJwHarness(t, "ask")
		code, errs := gyJwRefusal(t, h, path, "job", "result", "J-1", "--body", `{"answer":"yes"}`)
		t.Logf("not a person Job: exit %d stderr %q", code, errs)
	})
	t.Run("answer over the cap", func(t *testing.T) {
		h, path, _ := gyJwHarness(t, "ask")
		long := `{"answer":"` + strings.Repeat("x", 3501) + `"}`
		code, errs := gyJwRefusal(t, h, path, "job", "result", "J-2", "--body", long)
		t.Logf("over the cap: exit %d stderr %q", code, errs)
		if strings.Contains(errs, "waiting") {
			t.Fatalf("stderr %q", errs)
		}
	})
	t.Run("accepted person Job", func(t *testing.T) {
		h, path, _ := gyJwHarness(t, "ask")
		var body map[string]any
		if err := json.Unmarshal([]byte(personBody(t, "J-3", nil)), &body); err != nil {
			t.Fatal(err)
		}
		body["kind"] = "ask"
		out, _ := json.Marshal(body)
		if code, _, errs := gyJwRun(h, path, "job", "post", "--person", "dana", "--job", "J-3", "--body", string(out)); code != 0 {
			t.Fatalf("post J-3: %d %s", code, errs)
		}
		if code, _, errs := gyJwRun(h, path, "job", "result", "J-3", "--body", `{"answer":"yes"}`); code != 0 {
			t.Fatalf("answer J-3: %d %s", code, errs)
		}
		asHolder(t, gyJwEpoch)
		if code, _, errs := gyJwRun(h, path, "job", "next", "--json"); code != 0 {
			t.Fatalf("next: %d %s", code, errs)
		}
		asPerson(t, "owner")
		code, errs := gyJwRefusal(t, h, path, "job", "result", "J-3", "--body", `{"answer":"again"}`)
		t.Logf("accepted person Job: exit %d stderr %q", code, errs)
	})
}
