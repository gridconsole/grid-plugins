# C-gq-replan-shows-cause: the re-plan Job names why the cancelled Job stopped

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Cancelled Job, exit_code "launch_failed: no such file", one done-edge waiter: acceptance text holds "Stopped because: launch_failed: no such file" | HOLDS | `TestGqReplanNamesTheStringCause` (internal/orchestrator/gq_replan_cause_test.go), real tick loop on a Feed; `go test ./internal/orchestrator/ ./internal/cli/ -run 'Replan\|replan\|GqCause' -count=1 -v` exit 0 |
| 2. No exit_code: no such sentence; gk tests still pass | HOLDS | `TestGqReplanWithoutCauseHasNoSentence`; all seven `TestGkReplan*` PASS, same run, exit 0 |
| 3. Numeric exit_code 2 shows as "Stopped because: 2" | HOLDS | `TestGqReplanNamesTheNumericCause`, same run |
| 4. postFindings returns no error finding | HOLDS | `TestGqReplanBodyWithCausePassesRenderLint` (internal/cli/gq_replan_cause_render_test.go) with a string, a numeric and a 900-byte cause, PASS, same run |
| 5. `go test ./internal/orchestrator/ ./internal/cli/ -run 'Replan\|replan' -count=1` | HOLDS | exit 0: `ok internal/orchestrator`, `ok internal/cli` |
| Cause cut to 500 bytes (brief item 2) | HOLDS | `TestGqCauseIsCutTo500Bytes`: 800 bytes in, 500 after the label; "€"×300 cut to 498, no split rune |
| Not in intent.goal (brief item 1) | HOLDS | `gqCauseAcceptance` asserts intent.goal stays `intent.md#request`; replan.go builds the sentence into `text` only |
| New tests fail on base, pass on lane | HOLDS | with replan.go stashed to base: `go test ./internal/orchestrator/ -run 'GqReplan\|GqCause' -count=1` exit 1: `gq_replan_cause_test.go:80:44: too many arguments in call to ReplanJobBody` / `:87:12: undefined: gqCause` / `:97:13: undefined: gqCauseText` / `FAIL ... [build failed]`. A compile failure, not an assertion failure: the signature is new. |
| No existing test edited to pass | HOLDS, one call-site edit | Only the new argument `""` was added at `internal/cli/gk_replan_render_test.go:36` (named in Touches) and `internal/orchestrator/gk_replan_after_cancel_test.go:124` (NOT named in Touches; the package would not compile otherwise). No assertion changed. |
| No test deleted | HOLDS | `git diff --stat origin/main..HEAD` shows no deletion |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The 500-byte cut limits the length of text in a Job body, not an agent. It posts a Job and nothing more. |
| Tests of changed packages and importers | HOLDS | `go list` importers: cli, migrate (import orchestrator); cmd/grid, lib (import cli). `go test ./internal/orchestrator/ ./internal/migrate/ ./internal/cli/ ./cmd/grid/ ./lib/ -count=1` exit 0 (orchestrator 53.8s, migrate 6.3s, cli 53.6s, lib 2.3s, cmd/grid no test files) |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/orchestrator internal/cli` empty |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| *RealPlugin* tests | n/a | no row names one; none touched |
| Public repo: no personal paths, hosts, emails | HOLDS | none in the diff |
| Commits: one sentence, no attribution | HOLDS | `git log origin/main..HEAD` |

## What changed

- `internal/orchestrator/replan.go`: `ReplanJobBody(id, cancelled, cause string, waiting []string)`. A non-empty cause adds " Stopped because: <cause>" to the A1 acceptance text, after trimming and cutting to `gqCauseMax` (500) bytes on a rune boundary (`gqCause`). `gkReplan` passes `gqCauseText(j.ExitCode)`: a JSON string unquoted, a number as its digits, anything else empty. intent.goal is unchanged.
- New `internal/orchestrator/gq_replan_cause_test.go`, new `internal/cli/gq_replan_cause_render_test.go`.
- Call sites of the new argument in two gk tests.

## NOT VERIFIED on a live screen

Nothing here is a UI row. The sentence is proven on the body the tick loop posts to a real Feed. Nothing was run against the real plugin binary or an app that writes the cause (C-gq-exit-cause-every-abandon is a separate lane).

## Notes

- The cause is text from the worker-exit and may hold newlines or markdown; it is put in as is after trim. Not a lint error (row 4 passes).
- `ReplanJobBody` is exported and its signature changed; the only callers are `gkReplan` and the two tests.
