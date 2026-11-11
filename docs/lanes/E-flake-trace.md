# E-flake-trace: TestOverwrittenTraceFileIsNotRead failed under load

Base: main. Branch lane-E-flake-trace. One code commit (internal/eval/trace_integrity_test.go only) plus this table.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. Name the cause: what depends on wall-clock time | HOLDS | Base `internal/eval/trace_integrity_test.go:28`: the fake agent ran `until grep -q Skill "$HOME/../trace.jsonl" \|\| [ $n -ge 40 ]; do sleep 0.05; done`. Since 4e25e955 the runner keeps stdout in memory (`internal/eval/runner.go:214-222`) and writes no trace.jsonl, so the `grep` could never succeed and the loop always ran all 40 rounds. Each round forks `grep` and `sleep`, so the wait was a floor of 2s that grew with load (my estimate: 80 forks in 5.2s is about 65ms per fork at the brief's load 110; not measured). The test set a 5s whole-task timeout (`trace_integrity_test.go:27`) that covers that wait. At 5.2s the context ended the agent, `AgentResult.Status` was not "passed", and `runner.go:190-196` marked every check "skipped: setup or agent did not succeed". |
| 2. Remove the timing dependence, do not just raise the timeout | HOLDS | `trace_integrity_test.go:25-41`. The agent is now `printf '%s\n' "$1"; printf '%s\n' "$2" > "$HOME/../trace.jsonl"`: it waits on nothing, uses no sleep and no poll loop, and the 5s override is gone (the shared 3s guard from `testRunner` stays, and it only catches a hang). A third check, `command_passes grep -q Forged ../trace.jsonl`, runs after the agent exits and proves the forged line is on disk, so the two trace checks passing means the file was not read rather than never written. The two original assertions are unchanged. The test takes 0.01s, against at least 2s before. |
| 3. Proof under load: `go test -count=50 -cpu 1,4 -run TestOverwrittenTraceFileIsNotRead ./internal/eval/` while `go test -count=3 ./...` runs in a second shell | HOLDS | **Base, same two commands:** `--- FAIL: TestOverwrittenTraceFileIsNotRead (5.26s)`, `trace_integrity_test.go:34: overwritten file was read: [... Status:skipped ... Reason:setup or agent did not succeed ...]` for both checks, `FAIL github.com/gridconsole/grid-plugins/internal/eval 262.930s`, exit 1. **Lane, same two commands:** load shell `go test -count=3 ./...` exit 0; stress shell `go test -count=50 -cpu 1,4 -v` printed 100 `--- PASS`, 0 `--- FAIL`, `ok ... internal/eval 3.278s`, exit 0. Because the lane stress run is only a few seconds long, I repeated it while the load shell was still running tests: `-count=50 -cpu 1,4` gave `ok ... 3.018s`, and `-count=500 -cpu 1,4 -run 'TestOverwrittenTraceFileIsNotRead\|TestAgentCannotForgeTheTrace'` (1000 runs of the two trace-integrity tests) gave `ok ... 29.942s`. Exit 0 for all. |
| 4a. go test ./... | HOLDS | `go test -count=3 ./...` (the load shell) exit 0, 57 `ok` lines, no FAIL line (internal/eval 25.274s for three runs). The orchestrator runs the whole-module test again at landing. |
| 4b. make lint | HOLDS | `make lint` (`go vet ./...`, `gofmt -l .`) exit 0, no output after the command echo. |
| Packages changed and their importers | HOLDS | Changed: internal/eval. Importers from `go list -f '{{range .Deps}}...'` plus a grep of imports: cmd/grid, internal/cli, internal/evalcmd, internal/hook, internal/land, internal/packaging, internal/packaging/cmd. `go test -count=1 ./internal/eval/... ./internal/evalcmd/... ./internal/cli/... ./internal/hook/... ./internal/land/... ./internal/packaging/... ./cmd/grid/...` exit 0 (eval 3.232s, evalcmd 0.164s, cli 30.445s, hook 2.886s, land 3.405s, packaging 0.578s; cmd/grid and packaging/cmd have no test files). |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base and pass on the lane | HOLDS, with a note | No test was added: the flaky test itself is the thing the brief orders fixed, so it is changed in place. Its failure on base is load-dependent, because the base agent always waits at least 2s; the failing lines are pasted under row 3. On the lane it passed in all 1200 loaded runs above. |
| No existing test edited to make it pass; none deleted | HOLDS | The edit is to the one test the brief names. Its two assertions (`absent` and `real` both "passed") are unchanged and a third is added, so it is stricter. No test was deleted. `git diff origin/main..HEAD --stat` shows internal/eval/trace_integrity_test.go and this file. |
| Touch only internal/eval/ | HOLDS | The code change is only `internal/eval/trace_integrity_test.go`. The only other file is this table, which the runbook asks for under docs/lanes. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Test-only change; nothing runs on an agent. |
| No attribution lines, no personal paths, hosts or emails | HOLDS | The one commit message is one sentence with no trailers; this file holds no home path, host or email. |
| Nothing pushed | HOLDS | Bundle only. |

## What changed

- `internal/eval/trace_integrity_test.go`: `TestOverwrittenTraceFileIsNotRead` no longer polls up to 2s for a trace file the runner stopped writing, no longer sets its own 5s timeout, and gains a `forged-on-disk` check. The unused `time` import is removed. The comment now describes the in-memory trace and says why no wait is needed.
- No runner code changed: `runner.go` was already correct, and the flake was only in the test's fake agent.

## Known trade-off

The old wait existed so that a reintroduced runner-written trace.jsonl would be copied before the agent overwrote it. The runner no longer writes that file. If someone reintroduces it, a runner that reads `../trace.jsonl` from disk without copying stdout into it is caught, since the forged line is read and `absent` fails. A runner that copies stdout into that file while the agent runs would not be caught reliably without a wait, and a wait on a file that is correctly never written is what caused the flake. That design was removed by 4e25e955.

## NOT VERIFIED on a live screen

Nothing here has a UI. The load was the brief's own: `go test -count=3 ./...` in a second shell. I did not measure the load average during my runs, so I cannot say it reached 110. The base failure was reproduced once in a 100-run stress shell (one FAIL of the 100); the fix does not depend on the load level, because the agent no longer waits at all.
