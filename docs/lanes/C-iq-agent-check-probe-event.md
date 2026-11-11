# C-iq-agent-check-probe-event

Base: plugin main `ee8a96c1` (the brief names 91fadeff; check.go:36-95 and check_test.go:18-243 read the same on `ee8a96c1`). check.go had no seam for the probe context (`git grep probeContext` found nothing), so the lane adds one. Code commit: see the head sha in the report; the proof file is its own later commit.

## Proof table

| # | Row | Result | Command / place |
|---|-----|--------|-----------------|
| 1 | No `time.Until` / `time.After` in the two tests | HOLDS | `git grep -n 'time.Until\|time.After' -- internal/agent/check_test.go` prints nothing (exit 1). `go test -count=1 -race -run '^(TestAgentCheck\|TestIqAc)' ./internal/agent/`: exit 0, all PASS, TestAgentCheckTimeout 0.00s (was five seconds). |
| 2 | Mutation `Wait/2` in default probeContext fails BothFound with its own line | HOLDS | `check_test.go:31: deadline 2026-11-11 11:49:19.849172219 +0000 UTC m=+2.501307126, true` (exit 1; also failed: TestAgentCheckTimeout `check_test.go:205: wrong probe deadline: ...`, TestIqAcDefaultBudget, TestIqAcLateEntry). Reverted with `git checkout -- internal/agent/check.go`. |
| 3 | Mutation: delete the late-answer check (check.go:92-94 on the lane, :86-88 on base) fails TestAgentCheckTimeout at versions | HOLDS | `check_test.go:223: versions: [{Agent:claude Found:true Path:~/claude Version:3.0.0 Min:2.1.236 SignedIn:<nil> Fix:claude, then /login} {Agent:codex ... Version:0.162.0 ... SignedIn:0xc00011c42f Fix:}]` (exit 1; only that test failed). The line is :223 and not :216-218 because the lane adds lines above it. Reverted. |
| 4 | Mutation: default probeContext = `context.WithCancel(ctx)` fails TestIqAcDefaultBudget | HOLDS | `iq_probe_event_test.go:84: default probe budget: deadline 0001-01-01 00:00:00 +0000 UTC, false` (exit 1). Reverted. |
| 5 | TestIqAcLateEntry: Codex runner held until the Claude probe entered and `iqAcHangCtx(t).Err()` is nil; both reports, no "wrong probe deadline" | HOLDS | `iq_probe_event_test.go:90-142`; PASS in `go test -count=1 -race -v -run '^(TestAgentCheck\|TestIqAc)' ./internal/agent/` (exit 0). No sleep, no elapsed-time assertion. Under the Wait/2 mutation (row 2) it fails with `wrong probe deadline`, so its order check is live. |
| 6a | `git diff main --stat` names only the three files | HOLDS | `check.go 8 +-`, `check_test.go 17 ++--`, `iq_probe_event_test.go 142 +`; `3 files changed, 160 insertions(+), 7 deletions(-)` (at the code commit, no mutation; `git status --short` empty). This proof file is a fourth path, in its own commit, as the runbook asks. |
| 6b | `go test -count=1 -race ./internal/agent/` | HOLDS | `ok ... internal/agent 5.677s`, exit 0. Also `-count=50 -race` of BothFound, Timeout and TestIqAc*: `ok ... 1.371s`. |
| 6c | `make lint` | HOLDS | `make lint && echo "lint exit 0"` printed `lint exit 0`. |
| R1 | Importers of internal/agent tested | HOLDS | `go list` of imports/test imports: `internal/agent`, `internal/cli`. `go test -count=1 -race ./internal/cli/`: `ok ... 217.573s`, exit 0. |
| R2 | Windows and macOS builds | HOLDS | `make cross && echo "cross exit 0"` printed `cross exit 0` (vet and test compile for darwin/arm64 and windows/amd64, every package ok). |
| R3 | New tests fail on base, pass on the lane | HOLDS | Base check.go (`git checkout main -- internal/agent/check.go`): `iq_probe_event_test.go:37:10: undefined: probeContext` ... `FAIL ... [build failed]`, exit 1; restored with `git checkout HEAD -- internal/agent/check.go`. The two new tests then fail on their mutations (rows 2, 4) and pass on the lane. The old wall-clock failure of BothFound/Timeout on a loaded box cannot be replayed on demand; no test was run to claim it. |
| R4 | No existing test edited to make it pass, none deleted | HOLDS | Only the deadline check (:28-31) in TestAgentCheckBothFound and the waits in TestAgentCheckTimeout changed, as the brief names. No test or assertion removed: the Timeout runner keeps its deadline check, now an order check. |
| R5 | No guard, gate, cap or hold on agents; Wait and Check's result unchanged | HOLDS | check.go diff is the `probeContext` var (default `context.WithTimeout(ctx, Wait)`) and its use at :79. `git grep -n probeContext -- '*.go'`: only check.go:54, :79 in non-test code; no non-test file assigns it. |
| R6 | No personal paths, hosts or emails; no attribution lines | HOLDS | Commit messages are one sentence with no trailer. |
| R7 | RealPlugin tests | NOT APPLICABLE | The brief names none; nothing here runs the plugin. |

## What changed

- `internal/agent/check.go`: `var probeContext = func(ctx) (ctx, cancel) { return context.WithTimeout(ctx, Wait) }`, used where the probe made its child context. Same behaviour in production.
- `internal/agent/check_test.go`: TestAgentCheckBothFound takes `before := time.Now()` before Check and requires the probe deadline no earlier than `before.Add(Wait)` and no later than `time.Now().Add(Wait)`. TestAgentCheckTimeout does the same order check in its runner, installs the probe event with `iqAcProbeEnd`, runs Check with `iqAcRunCheck`, calls `endProbes()` once both probes have entered, and waits on `hang.Done()` where it waited on `time.After`.
- `internal/agent/iq_probe_event_test.go` (new): `iqAcHangCtx` (ends at `t.Deadline()` less a tenth of the time left, at most one minute; none without a deadline), `iqAcProbeEnd`, `iqAcRunCheck`, `TestIqAcDefaultBudget`, `TestIqAcLateEntry`.

Design points a reviewer may ask about:
- The brief says the swapped `probeContext` "records its cancel" and the test cancels them. `iqAcProbeEnd` does it with one `end()` call instead of a list of cancels: every probe child ends when `end()` is called, and a probe that starts after `end()` is cancelled before it is returned. A list would leave the fourth probe (Claude's `auth status`, reached only when the late-answer check is deleted, row 3) un-cancelled and the test would hang instead of failing at `versions:`.
- The swapped probeContext wraps the default one, so the probe keeps its real `Wait` deadline and the order check in the Timeout runner stays meaningful. The test never waits on that deadline: it ends the probes by `end()` and every wait selects on the hang context. If a box stalled for five seconds the real budget would end the probes first, with the same result.
- `iqAcRunCheck` cancels the check and waits for it in a cleanup that runs before `iqAcProbeEnd` restores `probeContext`, so a test that fails early cannot leave a probe goroutine reading the variable while it is restored (the `-race` failure path). The tests do not run in parallel (no `t.Parallel` in the package).
- The restore of `probeContext` is in `t.Cleanup` inside `iqAcProbeEnd`, which the test calls, and not spelled out in the test body.

## NOT VERIFIED on a live screen

Nothing here has a screen. NOT VERIFIED: a run on a genuinely loaded box (the old failure needs more than one second of delay between two lines); the new tests are order-only and have no clock to lose, which the mutations and the `-count=50 -race` run support but do not prove.
