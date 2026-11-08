package job

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		{"go verbose", "=== RUN   TestOne\n    --- PASS: TestOne/sub (0.0s)\n--- PASS: TestOne (0.01s)\nok  example.com/demo 0.1s\n--- FAIL: TestTwo (0.1s)\nFAIL example.com/other 0.1s\n", 2},
		{"go json", `{"Action":"output","Output":"--- PASS: TestOne (0s)\n"}` + "\n" + `{"Action":"pass","Test":"TestOne/sub"}` + "\n" + `{"Action":"pass","Test":"TestOne"}` + "\n" + `{"Action":"pass","Package":"example.com/demo"}`, 1},
		{"go plain", "ok  example.com/demo 0.1s\n", -1},
		{"go mixed missing count", "--- PASS: TestOne (0s)\nok example.com/one 0.1s\nok example.com/two 0.1s\n", -1},
		{"go no tests", "? example.com/demo [no test files]\n", 0},
		{"node TAP", "# tests 5\n# pass 4\n# fail 1\n", 5},
		{"node spec", "ℹ tests 7\nℹ pass 7\n", 7},
		{"cargo", "test result: ok. 2 passed; 0 failed; 1 ignored; 0 measured; 0 filtered out; finished in 0.00s\ntest result: FAILED. 1 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s\n", 5},
		{"pytest", "================ 3 passed, 1 skipped, 2 deselected in 0.12s ================\n", 4},
		{"pytest quiet", "1 failed, 2 passed, 1 error in 1.25s\n", 4},
		{"pytest empty", "no tests ran in 0.01s\n", 0},
		{"color", "\x1b[32m# tests 2\x1b[0m\r\n", 2},
		{"unknown", "everything worked\n", -1},
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

func TestVerifyCancelKillsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixture")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := RunVerify(ctx, Job{Verify: Verify{Command: "(echo ready > ready; sleep 1; echo survived > survived) & wait"}}, dir)
		done <- err
	}()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
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
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not return")
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "survived")); !os.IsNotExist(err) {
		t.Fatal("child survived cancellation")
	}
}
