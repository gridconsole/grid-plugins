# C-gm-plan-unknown-scope-envelope

The six plan verbs (`plan check`, `plan diff`, `plan hash`, `plan items`, `plan migrate`, `plan write`) now give the `no_such_project` and `no_such_task` envelopes with `--json`. Stacked on `lane-C-gm-question-usage-unknown-task-envelope` (merged at d8937a13). Code head: 6fa1b9fb; the lane report is committed after it.

## What changed

- `internal/cli/plan.go` `planError`: calls `env.recordError(err)` only when `errorCode(err) != ""`, then prints `plan: <err>` and returns 1 as before. The lookup errors already carry the sentinels (`where.ErrProjectUnregistered`, `where.ErrTaskNotFound`), so `commandFailure.finish` now writes the envelope. A plan failure with no sentinel (for example "no current task") gets no code and keeps its stderr line and empty stdout.
- `internal/cli/gm_plan_scope_envelope_test.go` (new): `TestGmPlanUnknownScope` (six verbs x `--project absent` / `--task T-99` = twelve cases) and `TestGmPlanFailureWithoutSentinelKeepsOutput`.
- `internal/cli/gg_unknown_project_walk_test.go`, `internal/cli/gg_unknown_task_walk_test.go`: the six `plan *` entries removed from each known-open map. Nothing else was removed. The task walk diff also shows gofmt realignment of the remaining values; `git diff -w` shows six deleted lines in that file.
- Nothing in `role_policy.go` or `person_check.go`. No guard, gate, cap or hold added.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1. New test FAILs on all twelve cases on the base | HOLDS | Base `plan.go` (`git checkout d8937a13 -- internal/cli/plan.go`), `go test -count=1 -v -run TestGmPlan ./internal/cli/`: exit 1, `--- FAIL: TestGmPlanUnknownScope/plan_{check,diff,hash,items,migrate,write}_{project,task}` (12 lines). Cause on each: `gm_plan_scope_envelope_test.go:81: error JSON: unexpected end of JSON input` (exit and stderr already right; stdout with `--json` empty). `plan.go` was then restored with `git checkout HEAD -- internal/cli/plan.go`. |
| 1. After the change it PASSes: same exit and byte-equal stderr plain and with `--json`; stdout is exactly the envelope | HOLDS | `go test -count=1 -v -run TestGmPlanUnknownScope ./internal/cli/` exit 0, twelve `--- PASS` lines. Plain run checks exit 1, exact stderr, empty stdout; `--json` run checks exit 1, byte-equal stderr, `assertErrorOutput` (exactly `{"error":{"code","message"}}`, message = stderr without its newline). |
| 2. HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME under the temp folder; Fatal when HOME is not under it; no child process | HOLDS | `gm_plan_scope_envelope_test.go` (the `t.Setenv` loop and the `strings.HasPrefix(home, h.root+...)` `t.Fatalf`). The test calls `Run` in process; it starts no program. |
| 3. `TestGgUnknownProject` PASSes; `ggProjectKnownOpen` holds no `plan` entry | HOLDS | `go test -count=1 -v -run TestGgUnknownProject ./internal/cli/` exit 0; `known open: 30 of 81` (was 36 before the six entries went). `grep '"plan ' gg_unknown_project_walk_test.go` finds nothing. |
| 4. `TestGgUnknownTask` PASSes; `ggTaskKnownOpen` holds no `plan` entry | HOLDS | `go test -count=1 -v -run TestGgUnknownTask ./internal/cli/` exit 0; `known open: 15 of 81`. The only `"plan ` text left in `gg_unknown_task_walk_test.go` is in `ggTaskWalked` (the list of members the walk runs), not the known-open map. |
| 5a. A plan failure with no sentinel keeps its output | HOLDS | `TestGmPlanFailureWithoutSentinelKeepsOutput` (`plan hash`, no current task: exit 1, stderr `plan: no current task; use --task <task folder or id>`, stdout empty plain and with `--json`). It PASSes on the base too: it pins unchanged behaviour, so it is the one new test that does not fail on the base. |
| 5b. `go test -count=1 ./internal/cli/ ./internal/plan/` exit 0, no existing test edited | HOLDS | exit 0 (`ok internal/cli 39.855s`, `ok internal/plan 0.183s`), run before the 5a pin was added. After the pin only `-run TestGmPlan` was rerun (exit 0). No existing test body was edited; only the six map entries (and the alignment of the remaining ones) in the two walk files. |
| 5c. Diff names only the four Touches files | HOLDS | `git diff d8937a13 --stat`: `gg_unknown_project_walk_test.go`, `gg_unknown_task_walk_test.go`, `gm_plan_scope_envelope_test.go`, `plan.go` (4 files). The command as written in the brief, `git diff lane-C-gm-question-usage-unknown-task-envelope`, cannot run as typed: there is no local branch of that name. Against `origin/lane-C-gm-question-usage-unknown-task-envelope` it also lists files that main gained after that lane's base; the merge commit d8937a13 is the right base. This report is the only other file after it. |
| 5d. `make lint` exit 0 | HOLDS | `make lint && echo LINT_EXIT_0` printed `go vet ./...` then `LINT_EXIT_0`. `gofmt -l internal/cli/` lists nothing. |
| 5e. Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0, no output. |
| 5f. macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0, no output. |
| Guest parity stays green (review item) | HOLDS | `go test -count=1 -v -run TestGuestProjectParity ./internal/cli/` exit 0, `--- PASS`. A guest's hidden project or task gets the same `plan: ...` stderr line as before plus the envelope; the stderr text is unchanged, and the code is the one the resolver refusal already carries. |

## Rules

| Rule | Result | Evidence |
|---|---|---|
| Edit only the Touches files | HOLDS | 5c. |
| New tests fail on base and pass on the lane, failing lines pasted | HOLDS for `TestGmPlanUnknownScope` (row 1); `TestGmPlanFailureWithoutSentinelKeepsOutput` is a pin and passes on both (row 5a). |
| No existing test edited to pass; no test deleted | HOLDS | Only the six known-open entries per walk were removed, as the brief names; the walks fail on exactly those six members until they are removed (project walk: `plan check ... answers no_such_project now: remove it from ggProjectKnownOpen`, six lines; task walk: `plan ... answers an unknown task as asked: remove it from ggTaskKnownOpen`, six lines). |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change records a code for an error that is already printed; no path is refused that was allowed. |
| Packages that import `internal/cli` | HOLDS | `go list` import scan: `cmd/grid` and `lib`. `go test -count=1 ./cmd/grid/ ./lib/`: `cmd/grid` has no test files; `ok lib 0.891s`. |
| No pushing; plugin repo public: no personal paths, hosts or emails | HOLDS | Nothing pushed; this report uses relative paths only. |
| Budget of 60 tool calls | NOT HELD | The lane used about 62 calls: the extra ones went on the no-sentinel pin and on proving it passes on the base. The work was all done and committed by call 57; only the report and bundle came after the 50-call mark. |

## NOT VERIFIED on a live screen

Nothing here is UI. A real guest session reading the envelope for a hidden project or task was not run: the proof is `TestGuestProjectParity` and the new test, which use the same resolver refusal in process. The whole-module test and the cross-repo set are left to the orchestrator at landing; `go test ./internal/cli/ ./internal/plan/ ./lib/` is what was run here.

DONE
