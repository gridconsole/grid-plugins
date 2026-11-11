# Q-fold-exit-code: the fold keeps a worker-exit's `code` and says when the reason was missing

Base: origin/main 3ccec5cb; the lane also holds Q-tree-orphan-jobs (merged, 0c3f4394 is the start). Branch lane-Q-fold-exit-code.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Test that fails on main | HOLDS | Commit a5b1cc5d holds only internal/contract/fold_exit_code_test.go (JSON based, so it compiles on main). Run on main's production code (types.go, fold.go unchanged): `go test -count=1 ./internal/contract/ -run 'TestFoldKeeps|TestFoldSays|TestFoldDrops'` exit 1: `fold_exit_code_test.go:38: worker-exit {reason cancelled, code prepare_failed}: the folded Job holds exit_reason cancelled exit_code <nil> (no field holds prepare_failed): {"id":"J-1",...,"exited":"00000000000000000000000004","exit_reason":"cancelled",...}`; also `:57: fixture body: want exit_reason crash and exit_reason_missing true` and `:86: a whole-number code 2.0 reads as the integer 2`. On the lane the same run exits 0. |
| 2. Reason and code kept; integer round-trips | HOLDS | `TestFoldKeepsWorkerExitCode`: `{"reason":"cancelled","code":"prepare_failed"}` folds to `exit_reason cancelled`, `exit_code "prepare_failed"`; `{"reason":"crash","code":1}` marshals `exit_code` as the bytes `1` (compared as json.RawMessage). Fold: internal/contract/fold.go:366-374 (`applySide` worker-exit branch), `exitCode` helper fold.go:388; fields internal/contract/types.go:162-163. |
| 3. Empty body | HOLDS | `TestFoldSaysWhenTheExitReasonWasMissing`: fixture body, `{}` and `{"code":7}` fold to `exit_reason crash` and `exit_reason_missing true`; `{"reason":"done"}` never sets it. fold.go:370-374. |
| 4. Assign clears; usage_limit rule | HOLDS | `TestAssignClearsTheExitCode` (new attempt after cancelled+code, new attempt after missing reason, usage_limit same attempt with code 3): `exited`, `exit_reason`, `exit_code`, `exit_reason_missing` all cleared. fold.go:192 (`j.ExitCode, j.ExitReasonMissing = nil, false` right after the existing clear of `Exited`, `ExitReason`, `Reason` at :191). Existing `usage-limit-reoffers-same-attempt`, `usage-limit-hold`, `missing-exit-reason-means-crash` in fold_test.go pass unedited. |
| 5. Tree attempt row and cli | HOLDS | internal/orchestrator/write.go `Attempt.Code`, `Attempt.ReasonMissing` (json `code`, `reason_missing`, omitempty) and the worker-exit case in `TreeAt`; internal/cli/orchestrator.go (the attempt line in the `job tree` loop) appends ` code <code>` and ` (no reason given)`. `TestTreeAttemptCarriesTheExitCode`, `TestTreeAttemptSaysWhenTheReasonWasMissing` (orchestrator); `TestJobTreeShowsTheWorkerExitCode` (cli harness): text line is `  attempt 1  fence N  exited  cancelled code prepare_failed`. A Job with no worker-exit: `TestTreeRowWithoutExitHasNoExitCodeKeys` and, in the cli test, `job tree --json` and `feed state --json` before the exit contain none of `exit_code`, `exit_reason_missing`, `code`, `reason_missing`; all four are omitempty, so those bytes are main's. The existing `TestOrphanAbsentRowJSONUnchanged` (old `{*Job, attempts}` shape) passes. |
| 6. feed state validates | HOLDS | `TestJobTreeShowsTheWorkerExitCode` runs `grid feed state --json` after the exit: `contract.Validate("cmd-feed-state", out)` returns nil and the output holds `"exit_code":"prepare_failed"`. Schema: contract/schemas/cmd-feed-state.json, `exit_code` (integer or non-empty string) and `exit_reason_missing` (boolean) as optional Job properties, neither in `required`. New valid fixture contract/fixtures/cmd-feed-state/valid/exit-code.json (string code; integer code with `exit_reason_missing`). A state written by main still validates: valid/basic.json, kind.json, populated.json and `TestFeedStateCarriesJobKind` pass unedited. |
| 7. Named test commands | HOLDS | `go test -count=1 ./internal/contract/ ./internal/golden/` exit 0 (contract 0.758s, golden 0.380s). `go test -count=1 ./internal/contract/ ./internal/orchestrator/ ./internal/cli/ -run 'Exit|Tree|Fold'` exit 0. `go vet ./internal/contract/ ./internal/orchestrator/ ./internal/cli/` exit 0. No golden state file changed (`git diff origin/main --stat` lists no file under internal/golden or any golden directory), so none gained the new fields. The same stat also lists the stacked Q-tree-orphan-jobs files, which are not this lane's. |
| Importers tested | HOLDS | `go list` (Deps containing internal/contract or internal/orchestrator): cmd/grid, artifact, brain, braincmd, cli, doctor, env, feed, guard, hook, intent, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, orchestrator, packaging, packaging/cmd, people, ports, projectmd, projects, question, routine, session, settings, signal, task, usage, where, workflow. `go test -count=1` on all of them (plus job/frontmatter) exit 0: every line `ok`, cmd/grid and job/jobtest have no test files. |
| Lint | HOLDS | `make lint` (go vet ./... and gofmt) exit 0; `gofmt -l internal/` prints nothing. |
| Windows / macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard/gate/cap/hold | HOLDS | The fold and the report only read; nothing blocks, caps or reassigns an agent. |
| No test edited or deleted | HOLDS | Only four new test files; no existing test file in the diff. |
| No personal paths/hosts/emails | HOLDS | Added code, fixture and this report carry none. |
| New tests in new files | HOLDS | internal/contract/fold_exit_code_test.go, internal/orchestrator/tree_exit_code_test.go, internal/cli/tree_exit_code_test.go. |
| No attribution lines | HOLDS | Commit messages are one sentence each, no trailer. |

## What changed

- internal/contract/types.go: `Job` gains `ExitCode json.RawMessage` (`exit_code,omitempty`) and `ExitReasonMissing bool` (`exit_reason_missing,omitempty`), as a separate block at the end of the struct so no existing line is realigned (F-review-levels-drop edits the same struct).
- internal/contract/fold.go: the worker-exit branch reads `code`, sets `ExitReasonMissing` when `reason` is empty and keeps `crash` as the reason; the assign branch clears both new fields; new `exitCode` keeps only an integer or a non-empty string (`null`, `""`, bool, fraction, object and array are dropped, `2.0` reads as `2`) so state.json always validates against cmd-feed-state. The fold stays pure.
- internal/orchestrator/write.go: `Attempt.Code` (text) and `Attempt.ReasonMissing`.
- internal/cli/orchestrator.go: the attempt line appends ` code <code>` and ` (no reason given)`.
- contract/schemas/cmd-feed-state.json and the new valid fixture.
- Three new test files.

## Notes for the reviewer

- Text format: the brief writes `exited cancelled code prepare_failed`; the existing line keeps its two-space column separator, so the line reads `exited  cancelled code prepare_failed`. I appended as the brief says and did not change the existing separator. The test asserts the two-space form.
- Ref body (`{"ref":"bodies/<ulid>.json"}`, the schema's second oneOf), out of scope, how the fold sees it today (from reading fold.go, not covered by a test): `applySide` unmarshals only `Reason` and `Code` from the post body, so a ref body has neither; the Job folds to `exit_reason crash`, and with this lane also `exit_reason_missing true` and no `exit_code`. The fold never opens `bodies/`, so a code held there is not seen. The Tree reads `r.Body` the same way.
- The fold key order: the two new keys sit at the end of the marshalled Job; a Job with no worker-exit marshals exactly as on main.
- F-review-levels-drop (65c27583, not on main) edits types.go, fold.go and cmd-feed-state.json in other hunks; mine are the end of the `Job` struct, the worker-exit and assign branches, and the `exit_reason` property neighbourhood, so a rebase should be a textual merge only.

## NOT VERIFIED on a live screen

- The dev UI showing `exit_code` / `exit_reason_missing` from `grid feed state --json`, and the `grid job tree` line on a real task; run only through the cli test harness.
