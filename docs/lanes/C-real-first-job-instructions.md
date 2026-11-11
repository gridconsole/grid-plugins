# C-real-first-job-instructions

Base: main cd84f424ad766a589bcc3c5ff1477ae4060d05ff. Lane head: see the report line "head" (the commit that adds this file).

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1. `AgentFor("orchestrator")` has "grid job result" and "plan.md" (fails on main) | HOLDS | `go test ./internal/cli/ -run TestOrchestratorTextTellsTheFirstJobWhatToDo` exit 0 on the lane. On base cd84f424 (agents/orchestrator.md, tests kept) exit 1: `first_job_instructions_test.go:28: AgentFor(orchestrator) lacks "grid job result --task <task> --job <job> --body <json>"` and `lacks "plan.md"`, `lacks "grid plan write"`, `lacks "no result posted"` |
| 2. Example body validates against result.json | HOLDS | `TestOrchestratorExampleResultValidates` cuts the first ```json block out of the embedded text and runs `contract.Validate("result", ...)` (same envelope the contract skill-example test uses): exit 0. On base: `no fenced json block in agents/orchestrator.md` |
| 3. Every acceptance line and the verify of `firstJob()` is named by the text | HOLDS | `TestOrchestratorTextNamesTheFirstJobsAcceptanceAndVerify` reads `firstJob()` and the embed: acceptance A1 text (internal/cli/task_new_first_job.go:36) must be in the text verbatim, verify as `` `file-exists` on `plan.md` ``, and the write path. Exit 0. On base: `agent text does not carry acceptance A1`, `does not name verify file-exists on plan.md` |
| 4. Fixture Feed: assigned, acked, worker-exit done, no result: `NoResult` and text " no result posted" | HOLDS | internal/orchestrator/tree_no_result_test.go `TestTreeFlagsAnEmptyFinish`; internal/cli/first_job_instructions_test.go `TestJobTreeShowsAnEmptyFinish/empty_finish`, line ends " no result posted". On base the orchestrator package does not compile: `tree_no_result_test.go:52:26: a[0].NoResult undefined (type Attempt has no field or method NoResult)`; cli on base prints `attempt 1  fence 2  exited  done` with no text |
| 5. Result before exit, crash, cancelled: no flag; second attempt after an empty first: only first flagged | HOLDS | `TestTreeLeavesOtherEndingsUnflagged` (done with a result, crash, cancelled, no reason), `TestTreeFlagsOnlyTheEmptyAttempt` (second attempt via a `control` to ready; attempt 1 flagged, attempt 2 running and then done with a result not flagged), and the cli subtests result_then_done, crash, cancelled. All exit 0 |
| 6. `grid job tree --json`: `"no_result": true` on row 4, no key on row 5 | HOLDS | `TestJobTreeShowsAnEmptyFinish` checks the key per subtest. `ls contract/schemas/ \| grep tree` finds no schema for `grid job tree` on this base (brief says none), so nothing to update; `cmd-feed-state.json` untouched |
| 7. tick.go unchanged | HOLDS | `git diff --stat main -- internal/orchestrator/tick.go` prints nothing; existing tick tests are in the orchestrator package run in row 8 |
| 8. Tests and lint | HOLDS | `go test ./internal/cli/ ./internal/orchestrator/ ./internal/job/ ./internal/contract/ ./contract/... ./cmd/grid/ . -count=1` exit 0 (cli 24.6s, orchestrator 22.2s, job 3.3s, contract ok). `make lint` (go vet ./...) exit 0. |
| Packages that import the changed ones | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` filtered for internal/orchestrator and internal/cli: importers are `internal/cli` and `cmd/grid`; both covered above (cmd/grid has no test files) |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| gofmt | HOLDS | `gofmt -l internal agents embed.go` prints nothing |
| New tests fail on base, pass on lane | HOLDS | pasted in rows 1 to 4 (stash of the four non-test files, tests kept) |
| No existing test edited or deleted | HOLDS | `git diff --stat main..HEAD` lists only the two new test files as tests |
| No guard, gate, cap or hold on agents | HOLDS | nothing added: the text says nothing retries a no-result Job and gives no repost rule for it (agents/orchestrator.md step 6); tick.go `case "done"` unchanged |
| No attribution lines, no personal data | HOLDS | `git log --format=%B -n 2 \| grep -ci "co-authored\|claude"` prints 0 |

## What changed

- agents/orchestrator.md: tools line gains `Write`. Steps are now eight: 3 writes plan.md first when the Job's `write` names it (draft file `plan.md` in the worktree, stored with `grid plan write plan.md --task <task>`, checked with `grid plan check --task <task>`); 6 also reads attempt lines ending "no result posted" (says what it means, that nothing retries, that the orchestrator looks at the Job's work and decides; no repost rule for it); 7 posts one result with `grid job result --task <task> --job <job> --body <json>` and a complete example body; 8 "Stop." comes after the result. A closing paragraph states A1 and the verify of the first Job. The brief said keep "7. Stop."; the step is now number 8 because the plan step and the result step were added; it is still the last step, after the result.
- internal/cli/task_new_first_job.go:36: acceptance A1 now also names `grid plan write` and `grid job result`. Verify stays file-exists plan.md.
- internal/orchestrator/write.go: `Attempt.NoResult` (`no_result,omitempty`), set in TreeAt at the worker-exit when the reason is done and no result record came for that attempt; a result record clears it.
- internal/cli/orchestrator.go: the attempt line ends " no result posted" when set.
- New tests: internal/cli/first_job_instructions_test.go, internal/orchestrator/tree_no_result_test.go.
- skills/plan-jobs/SKILL.md: read, no contradiction (it only refers to `plan.md#step-N` as intent anchors), left alone.
- `grid plan write` stores plan.md in the task folder (internal/plan/write.go:152), and the draft the agent writes is named plan.md in its worktree, so the Job's file-exists verify finds plan.md either way.

## NOT VERIFIED on a live screen

- That a real claude run follows the new steps and the Job's verify passes in the real worker (needs a live run on the dev UI; I ran no claude).
- That the app's own `grid job tree` consumers (the app view already shows the empty finish) are fine with the extra optional `no_result` key.
