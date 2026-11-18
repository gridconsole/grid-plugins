package job

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func TestRunVerify(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixtures")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "proof.txt"), []byte("proof"), 0600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		verify Verify
		pass   bool
		exit   int
		tests  int
		reason string
	}{
		{"pass", Verify{Command: "printf '# tests 3\\n'", MinTests: 3}, true, 0, 3, ""},
		{"fail exit", Verify{Command: "printf '# tests 3\\n'; exit 7", MinTests: 1}, false, 7, 3, "exit 7"},
		{"too few", Verify{Command: "printf '# tests 0\\n'", MinTests: 1}, false, 0, 0, "at least 1"},
		{"unknown", Verify{Command: "printf 'all good\\n'", MinTests: 1}, false, 0, -1, "totals unknown"},
		{"unknown allowed", Verify{Command: "true"}, true, 0, -1, ""},
		{"file", Verify{Kind: "file-exists", Path: "proof.txt"}, true, 0, -1, ""},
		{"files", Verify{Kind: "file-exists", Paths: []string{"proof.txt", "absent.txt"}}, false, 1, -1, "absent.txt"},
		{"directory is not proof", Verify{Kind: "file-exists", Path: "."}, false, 1, -1, "not regular"},
		{"reviewer", Verify{Kind: "reviewer-check"}, false, 3, -1, "reviewer must check"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := RunVerify(context.Background(), Job{Verify: tc.verify}, dir)
			if err != nil {
				t.Fatal(err)
			}
			if r.Pass != tc.pass || r.ExitCode != tc.exit || !strings.Contains(r.Reason, tc.reason) || r.Commit != "" {
				t.Fatalf("%+v", r)
			}
			if tc.tests < 0 {
				if r.Tests != nil {
					t.Fatalf("unexpected tests %d", *r.Tests)
				}
			} else if r.Tests == nil || *r.Tests != tc.tests {
				t.Fatalf("tests: %v", r.Tests)
			}
		})
	}
}

func TestVerifyTotals(t *testing.T) {
	tests := []struct {
		name, output string
		total        int
	}{
		{"go verbose", "=== RUN   TestOne\n    --- PASS: TestOne/sub (0.0s)\n--- PASS: TestOne (0.01s)\nok  \texample.com/demo\t0.1s\n--- FAIL: TestTwo (0.1s)\nFAIL\texample.com/other\t0.1s\n", 2},
		{"go json", `{"Action":"output","Output":"--- PASS: TestOne (0s)\n"}` + "\n" + `{"Action":"pass","Test":"TestOne/sub"}` + "\n" + `{"Action":"pass","Test":"TestOne"}` + "\n" + `{"Action":"pass","Package":"example.com/demo"}`, 1},
		{"go plain", "ok  \texample.com/demo\t0.1s\n", -1},
		{"go mixed missing count", "--- PASS: TestOne (0s)\nok  \texample.com/one\t0.1s\nok  \texample.com/two\t0.1s\n", -1},
		{"go no tests", "?   \texample.com/demo\t[no test files]\n", 0},
		{"node TAP", "# tests 5\n# pass 4\n# fail 1\n", 5},
		{"node spec", "ℹ tests 7\nℹ pass 7\n", 7},
		{"cargo", "test result: ok. 2 passed; 0 failed; 1 ignored; 0 measured; 0 filtered out; finished in 0.00s\ntest result: FAILED. 1 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s\n", 5},
		{"pytest", "================ 3 passed, 1 skipped, 2 deselected in 0.12s ================\n", 4},
		{"pytest quiet", "1 failed, 2 passed, 1 error in 1.25s\n", 4},
		{"pytest warnings", "3 passed, 2 warnings in 0.12s\n", 3},
		{"pytest warning", "3 passed, 1 warning in 0.12s\n", 3},
		{"pytest rerun", "3 passed, 1 rerun in 0.12s\n", 3},
		{"pytest skipped", "2 skipped in 0.12s\n", 2},
		{"pytest xfailed", "2 xfailed, 1 xpassed in 0.12s\n", 3},
		{"pytest errors", "2 errors in 0.12s\n", 2},
		{"pytest combined", "== 3 passed, 1 skipped, 1 xfailed, 2 errors, 1 rerun, 2 warnings in 0.12s ==\n", 7},
		{"unittest plain", ".F\n======\nFAIL: test_b (test_calc.T)\n------\nRan 2 tests in 0.001s\n\nFAILED (failures=1)\n", 2},
		{"unittest ok", "..\n----------------------------------------------------------------------\nRan 2 tests in 0.000s\n\nOK\n", 2},
		{"unittest verbose", "test_add (test_calc.T) ... ok\ntest_sub (test_calc.T) ... ok\ntest_mul (test_calc.T) ... ok\n\n----------------------------------------------------------------------\nRan 3 tests in 0.000s\n\nOK\n", 3},
		{"unittest skipped", "Ran 1 test in 0.000s\n\nOK (skipped=1)\n", 1},
		{"unittest errors", "Ran 5 tests in 0.010s\n\nFAILED (failures=2, errors=1, skipped=1)\n", 5},
		{"go cached", "ok  \texample.com/demo\t(cached)\n", -1},
		{"go build failure", "FAIL\texample.com/demo [build failed]\nFAIL\n", -1},
		{"go binary plain", "PASS\n", -1},
		{"go binary verbose", "--- PASS: TestOne (0.00s)\nPASS\n", 1},
		{"go printed ok", "ok this is test output\n--- PASS: TestOne (0.00s)\nPASS\nok  \texample.com/demo\t0.01s\n", 1},
		{"pytest empty", "no tests ran in 0.01s\n", 0},
		{"color", "\x1b[32m# tests 2\x1b[0m\r\n", 2},
		{"unknown", "everything worked\n", -1},
		{"overflow count", "# tests 99999999999999999999999\n", -1},
		{"overflow sum", "# tests 9223372036854775807\n# tests 1\n", -1},
		{"oversize line", strings.Repeat("x", verifyLineLimit+1) + "\n# tests 5\n", -1},
		{"summary beyond retained output", strings.Repeat("diagnostic\n", 10000) + "# tests 9\n", 9},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &verifyOutput{}
			// Writes need not coincide with lines or UTF-8 boundaries.
			for start := 0; start < len(tc.output); start += 7 {
				_, _ = w.Write([]byte(tc.output[start:min(start+7, len(tc.output))]))
			}
			w.finish()
			n := w.tests()
			if tc.total < 0 {
				if n != nil {
					t.Fatalf("got %d, wanted unknown", *n)
				}
			} else if n == nil || *n != tc.total {
				t.Fatalf("got %v, wanted %d", n, tc.total)
			}
			if w.output.Len() > verifyOutputLimit {
				t.Fatal("unbounded output")
			}
		})
	}
}

func TestVerifyDirectoryAndCommit(t *testing.T) {
	testgit.Isolate(t)
	dir := t.TempDir()
	testgit.Init(t, dir, "", true)
	j := Job{Worktree: Worktree{"": dir}, Verify: Verify{Kind: "reviewer-check"}}
	r, err := RunVerify(context.Background(), j, "")
	if err != nil || len(r.Commit) != 40 {
		t.Fatalf("%+v %v", r, err)
	}
	override := t.TempDir()
	r, err = RunVerify(context.Background(), j, override)
	if err != nil || r.Commit != "" {
		t.Fatalf("%+v %v", r, err)
	}
	j.Worktree["second"] = override
	if _, err = RunVerify(context.Background(), j, ""); err == nil {
		t.Fatal("ambiguous worktree accepted")
	}
}

// gtVerifyReadPid returns the pid the fixture shell wrote to bg.pid, or 0 when
// the file is absent or does not yet hold a whole line.
func gtVerifyReadPid(dir string) int {
	b, err := os.ReadFile(filepath.Join(dir, "bg.pid"))
	if err != nil || !strings.HasSuffix(string(b), "\n") {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// gtVerifyWaitPidGone waits for the pid in bg.pid to disappear; the bound only
// names what never happened.
func gtVerifyWaitPidGone(t *testing.T, dir string) {
	t.Helper()
	pid := gtVerifyReadPid(dir)
	if pid == 0 {
		t.Fatal("bg.pid missing or incomplete")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for p.Signal(syscall.Signal(0)) == nil {
		select {
		case <-deadline:
			t.Fatalf("descendant still alive: pid %d", pid)
		case <-ticker.C:
		}
	}
}

func TestVerifyCancelKillsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixture")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := RunVerify(ctx, Job{Verify: Verify{Command: "(echo ready > ready; sleep 30; echo survived > survived) & echo $! > bg.pid; wait"}}, dir)
		done <- err
	}()
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil && gtVerifyReadPid(dir) != 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("exited before readiness: %v", err)
		case <-deadline:
			t.Fatal("child did not start")
		case <-ticker.C:
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("cancel did not return")
	}
	gtVerifyWaitPidGone(t, dir)
	if _, err := os.Stat(filepath.Join(dir, "survived")); !os.IsNotExist(err) {
		t.Fatal("child survived cancellation")
	}
}

func TestVerifyExitedShellKillsDescendants(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process group fixture")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := RunVerify(ctx, Job{Verify: Verify{Command: "(sleep 30; echo survived > survived) & echo $! > bg.pid; exit 0"}}, dir)
	if err != nil || !r.Pass {
		t.Fatalf("a shell that exits 0 passes after its group is ended: %+v %v", r, err)
	}
	gtVerifyWaitPidGone(t, dir)
	if _, err := os.Stat(filepath.Join(dir, "survived")); !os.IsNotExist(err) {
		t.Fatal("descendant survived its shell")
	}
}

func TestVerifyRealTAP(t *testing.T) {
	// Captured from node --test --test-reporter=tap testdata/verify/node-tap.mjs;
	// only the absolute source path is replaced with a public placeholder.
	b, err := os.ReadFile("testdata/verify/node-tap.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"", "--- PASS: TestOne (0.00s)\nPASS\nok  \texample.com/demo\t0.01s\n"} {
		w := &verifyOutput{}
		_, _ = w.Write(append([]byte(prefix), b...))
		w.finish()
		want := 4 // # tests includes the nested child; # pass is 3.
		if prefix != "" {
			want++
		}
		if n := w.tests(); n == nil || *n != want {
			t.Fatalf("mixed=%t: got %v, want %d", prefix != "", n, want)
		}
	}
}

func TestVerifyGoPrintedOK(t *testing.T) {
	r, err := RunVerify(context.Background(), Job{Verify: Verify{
		Command: "go test -v testdata/verify/go_ok_test.go", MinTests: 1,
	}}, ".")
	if err != nil || !r.Pass || r.Tests == nil || *r.Tests != 1 {
		t.Fatalf("result: %+v, error: %v", r, err)
	}
}
