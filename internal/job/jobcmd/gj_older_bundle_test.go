package jobcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The bundle <task>/jobs/<id>/ that an earlier plugin build saved must still be
// read by the current verify and trace. The older bytes live in
// testdata/older-bundle (see its README for the commits they come from).

const (
	gjOldPointer = "brain:project/export-format"
	gjOldDigest  = "2d82e2c5b126282a1cd2a14eba32029b8a7871b79e90dc5072b9fffa9501e27b"
	gjOldTier    = "person-team"
)

// gjOldEnv sets the home keys under one temp root before anything runs, fails
// when HOME is not under that root, and returns an Env that carries the same
// folders. The Env has only Home and GridHome; CLAUDE_CONFIG_DIR and
// CODEX_HOME are covered by the environment.
func gjOldEnv(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	gridHome := filepath.Join(root, "grid")
	for key, dir := range map[string]string{
		"HOME":              home,
		"CLAUDE_CONFIG_DIR": filepath.Join(root, "claude"),
		"CODEX_HOME":        filepath.Join(root, "codex"),
		"GRID_HOME":         gridHome,
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, dir)
	}
	rel, err := filepath.Rel(root, os.Getenv("HOME"))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("HOME is not under the test root (key HOME)")
	}
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	return Env{Resolver: ownerResolver(gridHome), Cwd: root, Home: home, GridHome: gridHome, Stdout: out, Stderr: errs}, out, errs
}

func gjOldRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "older-bundle", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// gjOldBody is the body object of the older Job record, as raw members so that
// the retired keys stay exactly as the older build wrote them.
func gjOldBody(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	var record struct {
		Body map[string]json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(gjOldRead(t, "job-record.json"), &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Body) == 0 {
		t.Fatal("job-record.json has no body")
	}
	return record.Body
}

// gjOldBundle saves the older bundle into a task of the temp Grid home and
// returns the task folder. Only the verify command is replaced, so that no
// shell runs.
func gjOldBundle(t *testing.T, e Env) string {
	t.Helper()
	task := traceTask(t, e.GridHome, "alpha", "T-01")
	body := gjOldBody(t)
	body["verify"] = json.RawMessage(`{"kind":"reviewer-check"}`)
	saved, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte(`"max_rounds"`)) || !bytes.Contains(saved, []byte(`"review"`)) {
		t.Fatal("the saved job.yaml lost a retired key")
	}
	// The older Job names the worktree "example-worktree"; verify looks for that
	// folder under the project before it reads the verify kind.
	if err = os.MkdirAll(filepath.Join(e.GridHome, "projects", "alpha", "worktrees", "example-worktree"), 0o700); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(task, "jobs", "J-1")
	traceWrite(t, filepath.Join(dir, "job.yaml"), saved)
	traceWrite(t, filepath.Join(dir, "inputs.json"), gjOldRead(t, "inputs.json"))
	traceWrite(t, filepath.Join(dir, "brief.md"), []byte("# J-1 Build example feature\n"))
	return task
}

func TestGjOlderBundleVerifyReadsRetiredFields(t *testing.T) {
	e, out, errs := gjOldEnv(t)
	task := gjOldBundle(t, e)

	if code := Run([]string{"verify", "J-1", "--task", task}, e); code != 3 || !strings.Contains(out.String(), "reviewer must check") {
		t.Fatalf("verify of the older bundle: exit %d, want 3 with reviewer must check; stdout=%s stderr=%s", code, out, errs)
	}
	if errs.Len() != 0 {
		t.Fatalf("verify wrote to stderr: %s", errs)
	}
	j, err := loadVerifyJob(task, "J-1")
	if err != nil {
		t.Fatalf("loadVerifyJob: %v", err)
	}
	if j.Review != "L1" || j.Title != "Build example feature" || j.ID != "J-1" {
		t.Fatalf("loaded Job: review %q title %q id %q", j.Review, j.Title, j.ID)
	}
}

func TestGjOlderBundleTraceReadsOldInputs(t *testing.T) {
	e, out, errs := gjOldEnv(t)
	gjOldBundle(t, e)

	if code := Run([]string{"trace", gjOldPointer}, e); code != 0 {
		t.Fatalf("trace of the older bundle: exit %d; stdout=%s stderr=%s", code, out, errs)
	}
	if strings.Contains(out.String(), "invalid inputs.json") || strings.Contains(out.String(), "problem:") {
		t.Fatalf("trace reported a problem: %s", out)
	}
	out.Reset()
	if code := Run([]string{"trace", gjOldPointer, "--json"}, e); code != 0 {
		t.Fatalf("trace --json of the older bundle: exit %d; stdout=%s stderr=%s", code, out, errs)
	}
	var result TraceResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Problems) != 0 {
		t.Fatalf("problems: %+v", result.Problems)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("rows: %+v", result.Rows)
	}
	row := result.Rows[0]
	if row.Task != "T-01" || row.Job != "J-1" || row.Pointer != gjOldPointer || row.SHA256 != gjOldDigest || string(row.Tier) != gjOldTier {
		t.Fatalf("row: %+v", row)
	}
}

func TestGjOlderBundleFixturesKeepTheOldShape(t *testing.T) {
	body := gjOldBody(t)
	for _, key := range []string{"max_rounds", "review"} {
		if _, ok := body[key]; !ok {
			t.Errorf("job body of job-record.json lacks %q", key)
		}
	}

	var inputs map[string]json.RawMessage
	if err := json.Unmarshal(gjOldRead(t, "inputs.json"), &inputs); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"shown", "brief_bytes", "pasted_chars"} {
		if _, ok := inputs[key]; ok {
			t.Errorf("inputs.json has the newer key %q", key)
		}
	}
	var sources []map[string]json.RawMessage
	if err := json.Unmarshal(inputs["sources"], &sources); err != nil {
		t.Fatal(err)
	}
	tiers := map[string]bool{}
	for _, source := range sources {
		if _, ok := source["shown"]; ok {
			t.Errorf("a source of inputs.json has the newer key %q", "shown")
		}
		var tier string
		if err := json.Unmarshal(source["tier"], &tier); err != nil {
			t.Fatal(err)
		}
		tiers[tier] = true
	}
	if !tiers[gjOldTier] || !tiers["external"] {
		t.Errorf("source tiers %v, want person-team and external", tiers)
	}

	readme := string(gjOldRead(t, "README.md"))
	for _, want := range []string{"73025e9e^", "919c2b9a^"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not name %s", want)
		}
	}
}
