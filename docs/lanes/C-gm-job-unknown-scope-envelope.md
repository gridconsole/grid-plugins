# C-gm-job-unknown-scope-envelope

`job ack`, `job blocked`, `job cancel`, `job next`, `job order`, `job post`, `job result`, `job tree` and `feed tick` now give the `no_such_project` and `no_such_task` envelopes with `--json`. Stacked on `lane-C-gm-plan-unknown-scope-envelope` (head 837869a9, already merged into this branch). Code head: 70826f0b; the lane report is committed after it.

## What changed

- `internal/cli/orchestrator.go`: new `recordLookupError(env, err)` calls `env.recordError(err)` only when `errorCode(err) != ""`. It runs in the `fail` closure of `runOrchestrator` and at the three print-and-return sites of `runPersonApproval` (the `ReceiptToJob`, `CheckPersonAnswer` and `Load`/`PersonOpen` failures). The lookup errors already carry the sentinels (`where.ErrProjectUnregistered`, `where.ErrTaskNotFound`), so `commandFailure.finish` now writes the envelope. A failure with no sentinel gets no code and keeps its stderr line, exit and empty stdout. `feed tick` reaches `runOrchestrator` through `runOrchestratorFeed`, so it needed no other change; the new test shows it passing.
- `internal/cli/gm_job_scope_envelope_test.go` (new): `TestGmJobUnknownScope` (nine members x `--project absent` / `--task T-99` = eighteen cases) and `TestGmJobFailureWithoutSentinelKeepsOutput`.
- `internal/cli/gg_unknown_project_walk_test.go`, `internal/cli/gg_unknown_task_walk_test.go`: the nine entries removed from each known-open map. Nothing else was removed. The task walk diff also shows gofmt realignment of the remaining values; the blank line after the removed project entries went with them.
- Nothing in `role_policy.go` or `person_check.go`. No guard, gate, cap or hold added.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1. New test FAILs on all eighteen cases on the base | HOLDS | Test committed first (f35fc985) with the base `orchestrator.go`: `go test -count=1 -v -run TestGmJobUnknownScope ./internal/cli/` exit 1. Lines: `--- FAIL: TestGmJobUnknownScope/job_{ack,blocked,cancel,next,order,post,result,tree}_project`, `feed_tick_project`, the same nine `_task`: eighteen `--- FAIL`, then `FAIL github.com/gridconsole/grid-plugins/internal/cli`. Cause on each: `gm_job_scope_envelope_test.go:76: error JSON: unexpected end of JSON input` (exit and stderr were already right; stdout with `--json` empty). |
| 1. After the change it PASSes: same exit and byte-equal stderr plain and with `--json`; stdout exactly the envelope | HOLDS | `go test -count=1 -v -run TestGmJobUnknownScope ./internal/cli/` exit 0, eighteen `--- PASS` lines, `ok internal/cli`. Plain run checks exit 1, exact stderr (`project "absent" is not registered` / `task T-99 not found in project example`), empty stdout; the `--json` run checks exit 1, byte-equal stderr and `assertErrorOutput` (exactly `{"error":{"code","message"}}`, message = stderr without its newline). |
| 2. HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME under the temp folder; Fatal when HOME is not under it; no child process | HOLDS | `gm_job_scope_envelope_test.go`: the `t.Setenv` loop over the four names (folders under `h.home`/`h.gridHome`, which the harness puts under `t.TempDir()` at `cli_test.go:34-38`) and the `strings.HasPrefix(home, h.root+...)` `t.Fatalf`. The test calls `Run` in process and starts no program. |
| 3. `TestGgUnknownProject` PASSes; `ggProjectKnownOpen` holds none of the nine | HOLDS | `go test -count=1 -v -run 'TestGgUnknownProject\|TestGgUnknownTask\|TestGuestProjectParity\|TestStableErrorCodes' ./internal/cli/` exit 0 (`ok`); `TestGgUnknownProjectWalk` PASS, `known open: 21 of 81` (30 on the plan lane's base, minus nine). `grep -n -E '"(job (ack\|blocked\|cancel\|next\|order\|post\|result\|tree)\|feed tick)"'` on the file finds nothing. No count pinned. |
| 4. `TestGgUnknownTask` PASSes; `ggTaskKnownOpen` holds none of the nine | HOLDS | Same run: `TestGgUnknownTaskWalk` PASS, `known open: 6 of 81` (15 minus nine). The nine names now appear in `gg_unknown_task_walk_test.go` only in `ggTaskWalked` (the members the walk runs), not in the known-open map. No count pinned. |
| 5a. Other job failures keep their output | HOLDS | `TestGmJobFailureWithoutSentinelKeepsOutput` (`job tree`, no task selected: exit 1, stderr `no task selected; use --task`, stdout empty plain and with `--json`). It PASSes on the base `orchestrator.go` too (`git checkout 837869a9 -- internal/cli/orchestrator.go`, run, `git checkout HEAD -- internal/cli/orchestrator.go`; status clean afterwards): it pins unchanged behaviour, so it is the one new test that does not fail on the base. |
| 5b. `go test -count=1 ./internal/cli/ ./internal/orchestrator/ ./internal/feed/` exit 0, no existing test edited | HOLDS | At code head 70826f0b: `ok internal/cli 45.041s`, `ok internal/orchestrator 44.232s`, `ok internal/feed 1.542s`. No existing test body was edited; only the nine map entries per walk were removed (and the remaining values realigned by gofmt). |
| 5c. Diff names only the four Touches files | HOLDS | `git diff 837869a9 --stat`: `gg_unknown_project_walk_test.go`, `gg_unknown_task_walk_test.go`, `gm_job_scope_envelope_test.go`, `orchestrator.go` (4 files, before this report). The command as written in the brief, `git diff lane-C-gm-plan-unknown-scope-envelope`, cannot run as typed: there is no local branch of that name (exit 128); 837869a9 is that lane's head. |
| 5d. `make lint` exit 0 | HOLDS | `make lint && echo lint-exit-0` printed `go vet ./...` then `lint-exit-0`. `gofmt -l internal/cli/` lists nothing at the code head. |
| 5e. Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 (printed `windows-ok`). |
| 5f. macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 (printed `darwin-ok`). |
| Guest parity stays green (review item) | HOLDS | `TestGuestProjectParity` PASS (3.91s) in the run named under row 3. A guest's hidden project or task gets the same stderr line as before plus the envelope; the stderr text is unchanged and the code is the one the resolver refusal already carries (`internal/where/where.go:125`, `:303`; `errorCode`, `internal/cli/error_codes.go:30-47`). |

## Rules

| Rule | Result | Evidence |
|---|---|---|
| Edit only the Touches files | HOLDS | 5c. |
| New tests fail on base and pass on the lane, failing lines pasted | HOLDS for `TestGmJobUnknownScope` (row 1); `TestGmJobFailureWithoutSentinelKeepsOutput` is a pin and passes on both (row 5a). |
| No existing test edited to pass; no test deleted | HOLDS | Only the nine known-open entries per walk were removed, as the brief names; the walks fail on exactly those members until they are removed. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change records a code for an error that is already printed; no path is refused that was allowed. |
| Packages that import the changed package | HOLDS | `go list` import scan (imports, test imports and external test imports of `internal/cli`): `cmd/grid`, `contract/formats`, `internal/account`, `internal/golden`, `internal/jobgraph`, `internal/mcp`, `internal/packaging`, `internal/question`, `internal/settings`, `lib`. `go test -count=1` on those plus `internal/cli`, `internal/orchestrator`, `internal/feed`: every package `ok`, `cmd/grid` has no test files. That run was at 4aec85bd plus the walk commit, before the 5a pin was added; after the pin, `internal/cli`, `internal/orchestrator` and `internal/feed` were run again (5b). |
| No pushing; plugin repo public: no personal paths, hosts or emails | HOLDS | Nothing pushed; this report uses relative paths only. |
| Budget of 60 tool calls; WIP bundle at call 35 | NOT HELD (WIP bundle) | About 53 calls. The work was committed by call 31, so no work-in-progress bundle was written at call 35; the one bundle is written after this report. |

## Notes

- `runPersonApproval`'s three sites now call `recordLookupError`, as the brief asks. No test reaches a lookup sentinel there (they are the receipt, answer-cap and person-open failures); the change is inert unless such an error carries a sentinel, and it cannot change a failure without one.
- The comment above `ggTaskKnownOpen` still says "The first group prints its diagnostic on stderr...". That group is now empty. The brief limits the edit to the nine entries, so the comment was left.

## NOT VERIFIED on a live screen

Nothing here is UI. A real guest session reading the envelope for a hidden project or task was not run: the proof is `TestGuestProjectParity` and the new test, which use the same resolver refusal in process. The whole-module test and the cross-repo set are left to the orchestrator at landing.

DONE
