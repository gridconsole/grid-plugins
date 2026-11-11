# C-gt-proctree-cancel-signalled proof

| Row | Result |
|---|---|
| 1. `go test -count=1 -run '^TestRunUnixCancelledContextReturnsPromptly$' -v ./internal/proctree/` | HOLDS, exit 0: `--- PASS: TestRunUnixCancelledContextReturnsPromptly (0.03s)`, `PASS`; `-count=10`: `ok ... 0.252s` |
| 2. `git grep -n -E 'time\.(Since\|Sleep\|AfterFunc)' -- internal/proctree/proctree_unix_test.go` | HOLDS, no output |
| 3. Break: `cmd.Cancel = func() error { return nil }` in newOps | HOLDS, FAIL: `proctree_unix_test.go:87: child was not signalled: exit status 0`, `FAIL ... 30.082s`; restored with `git checkout -- internal/proctree/proctree_other.go`; diff vs main shows only the test file (plus this report) |
| 4. `go test -count=1 ./internal/proctree/` | HOLDS, `ok` |
| gofmt -l internal/proctree | HOLDS, no output |
| go vet (linux), GOOS=windows go vet, GOOS=darwin go vet | HOLDS, exit 0 each |
| Importers of proctree (go list): eval, job, land, routine | HOLDS, all `ok` |
| New test fails on base | The old test had the clock; the break row shows the new test fails when the pinned behaviour is broken. |
| No deletes, renames, skips, guards | HOLDS |

## What changed
Only the body of TestRunUnixCancelledContextReturnsPromptly: runs `sh -c 'echo $$ > "$DIR/pid"; exec sleep 30'`, a goroutine cancels once the pid file is non-empty (10 ms ticker, 30 s bound that only fails with "child never started"), asserts Run error, ProcessState set and Signaled, pid gone (ESRCH). AfterFunc and the 2 s bound removed.

## NOT VERIFIED on a live screen
Nothing UI. RealPlugin rows: none.
