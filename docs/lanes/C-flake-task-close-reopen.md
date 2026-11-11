# C-flake-task-close-reopen

TestConcurrentCloseReopen failed when the reopen child ran all its attempts before the close child's first move landed. Each child now runs until it has made 25 successful moves, so the two overlap by construction.

Base: main e7bea112 (write_test.go:178-249 and write.go:42-63 read there; the child loop was `for range 100`, so the brief applies). Code commits: ce60135f (late-start test on the old 100-attempt child), b633e3f1 (the fix). Rows 2, 3 and 6 ran on b633e3f1; the commit after it adds only this file.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Fails on base | HOLDS | On ce60135f the new test drives a verbatim copy of the base child loop (`for range 100`, refused move counts as an attempt) and base parent checks. `go test -count=3 -run '^TestFlakeCR' ./internal/task/` exit 1, three of three runs: `flake_close_reopen_test.go:109: history [- {"task":"T-01",...,"what":"close","from_state":"queued","to_state":"closed"}]: <nil>` (2.03s, 2.05s, 2.04s). The check that fires is `len(lines) < 2` at flake_close_reopen_test.go:58 on ce60135f; :109 is the call site, because `flakeCRRun` calls `t.Helper()`. The history holds one close receipt. |
| 2. Passes after the change | HOLDS | `go test -race -count=3 -run '^TestFlakeCR' ./internal/task/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/task  12.352s` |
| 3. 200 race runs | HOLDS | `go test -race -count=200 -timeout 30m -run '^TestConcurrentCloseReopen$' ./internal/task/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/task  418.159s`. 0 fail. |
| 4. Under load | NOT RUN | The sandbox refused the command: "Permission to use Bash has been denied because Claude Code is running in don't ask mode" (taskset, sh, timeout and kill are not on the allow list). No other load was substituted. |
| 5. No fixed attempt count | HOLDS | Child ends on `flakeCRMoves` = 25 successes (flake_close_reopen_test.go:18, loop at :118) or the 60 s deadline (:21, checked at :119). Refused "current state" moves are retried with no sleep (:128-129). `grep -n 'for range 100'` finds nothing in internal/task. Parent: each child printed exactly 25 receipts (:66), history holds exactly 2x25 lines (:71), state chain from queued (:86), final state queued (:102). |
| 6a. Diff names the two files | HOLDS | `git diff main --stat`: flake_close_reopen_test.go (new, +141) and write_test.go (+1 -73). With this proof file committed, a third line, docs/lanes/C-flake-task-close-reopen.md, is the runbook's lane doc. write.go, history.go and fslock are untouched. |
| 6b. Package tests | HOLDS | `go test -count=1 ./internal/task/` exit 0 (ok 7.886s). `go test -race -count=1 ./internal/task/` exit 0 (ok 16.453s). Importers (`go list`, direct imports and test imports of internal/task): braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, orchestrator, signal, workflow; all ok in one `go test -count=1` run (jobtest has no test files). |
| 6c. Lint | HOLDS | `make lint` exit 0 (go vet ./..., gofmt -l clean). |
| 6d. Cross builds | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| Rule: new tests fail on base, pass on lane | HOLDS | Rows 1 and 2. |
| Rule: no existing test edited to pass | HOLDS | TestConcurrentCloseReopen is the one test the brief names. Its body moved into `flakeCRRun`/`flakeCRChild` (below); no other test changed. |
| Rule: no deleted test | HOLDS | No test was deleted. The 73 lines removed from write_test.go (net of the one added) were the body of TestConcurrentCloseReopen. They moved, not vanished: child loop to flakeCRChild (:112), receipt match (:78), state chain (:86), lost line (:92), torn task (:97). |
| Rule: no guard, gate, cap or hook on agents | HOLDS | Test files only. The 60 s deadline is a failure bound in a test child, not a limit on any agent. |
| Rule: public repo | HOLDS | No personal path, host or email in the diff or this file. |
| Rule: no attribution lines | HOLDS | Commit messages are one sentence each, with no trailer. |

## What changed

- internal/task/flake_close_reopen_test.go (new): `flakeCRRun` (the shared parent and child entry; the child re-execs the running test by `t.Name()`), `flakeCRChild` (loops to 25 successes), `TestFlakeCRLateClose` (reopen child first, close child 2 s later; the 2 s `time.Sleep` between the Start calls is the stressor, not the fix).
- internal/task/write_test.go: TestConcurrentCloseReopen is one call, `flakeCRRun(t, []string{"close", "reopen"}, 0)`. The `os/exec` and `where` imports it no longer uses are gone.
- Why it holds: close is only allowed from queued and reopen only from closed (write.go:42-55), so the moves strictly alternate. A child that needs 25 successes cannot finish until the other has made its 25, whatever the start order or delay.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Row 4 (load) is NOT RUN, see above.
