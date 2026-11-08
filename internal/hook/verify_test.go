package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/session"
)

func TestStopRunsSavedJobVerificationAndCapsBlocks(t *testing.T) {
	f := eventSetup(t)
	f.env["GRID_JOB"] = "J-1"
	f.env["PATH"] = os.Getenv("PATH")
	dir := filepath.Join(f.dir, "jobs", "J-1")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	save := func(command string, min int) {
		t.Helper()
		b, _ := json.Marshal(job.Job{ID: "J-1", Verify: job.Verify{Command: command, MinTests: min}})
		if err := os.WriteFile(filepath.Join(dir, "job.yaml"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save("printf '1 passed\\n'", 2)
	for i := 0; i < 9; i++ {
		_, _, code := f.run(t, "Stop", hookio.Input{StopHookActive: i > 0})
		want := 2
		if i == 8 {
			want = 0
		}
		if code != want {
			t.Fatalf("stop %d: code %d, want %d", i+1, code, want)
		}
	}
	a, err := (session.Store{TaskDir: f.dir}).Activity("session-demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.VerifyFailed) != 1 {
		t.Fatal("failed verification not recorded at cap")
	}
	save("printf '2 passed\\n'", 2)
	_, log, code := f.run(t, "Stop", hookio.Input{StopHookActive: true})
	if code != 0 || log != "" {
		t.Fatalf("passing verify: %d %s", code, log)
	}
	save("printf '2 passed\\n'; exit 1", 2)
	_, _, code = f.run(t, "Stop", hookio.Input{})
	if code != 2 {
		t.Fatal("failing command ended turn")
	}
}
func TestVerificationCountFormats(t *testing.T) {
	cases := map[string]int{"{\"Action\":\"pass\",\"Package\":\"example\",\"Test\":\"TestOne\"}\n{\"Action\":\"pass\",\"Package\":\"example\"}": 1, "--- PASS: TestOne (0.00s)\n--- PASS: TestTwo (0.00s)": 2, "ok 1 first\nok 2 second": 2, "Ran 3 tests in 0.01s": 3, "Tests: 4 passed, 4 total": 4, "# pass 5": 5, "ok example (cached)": 0}
	for text, want := range cases {
		if got := testCount(text); got != want {
			t.Fatalf("%q: got %d want %d", text, got, want)
		}
	}
}
