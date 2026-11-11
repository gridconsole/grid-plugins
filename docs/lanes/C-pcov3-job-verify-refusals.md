# C-pcov3-job-verify-refusals

Tests for `grid job verify` refusals, the saved-Job load and the worktree path. Two new test files, no non-test file changed.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 coverage, jobcmd | HOLDS | before `go tool cover -func=/tmp/C-pcov3-job-verify-refusals-jobcmd.out \| tail -1` -> `total: (statements) 84.4%`; after (`...-jobcmd-after.out`) -> `85.4%` (`go test` itself prints 85.5%). Linux/amd64, go test exit 0. |
| 1 coverage, job | HOLDS | before `total: (statements) 90.1%`; after `91.1%`. Both after figures are above before. |
| 2 jobcmd argument refusals | HOLDS | `internal/job/jobcmd/verify_refusals_test.go` TestVerifyArgumentRefusals: no id, two ids, a non-J-N id and `--budget` -> exit 2, stderr exactly `job: verify requires J-N [--task T-NN] [--dir path] [--json]\n`; `--task nosuch` -> exit 1, stderr names `nosuch`; J-9 (no saved Job, no plan step) -> exit 1, stderr holds `plan.md: expected one item "J-9", found 0`. |
| 3 loadVerifyJob | HOLDS | TestLoadVerifyJobRefusals: `id: [broken` (not YAML) and `verify: not-a-map` (does not decode into a Job) are errors, and `verify` prints `job: <that error>` with exit 1; saved id J-2 asked as J-1 -> exactly `saved Job id does not match requested Job`. The plan fallback is not repeated. |
| 4 text output | HOLDS | TestVerifyTextOutput: file-exists pass -> `exit: 0\ntests: unknown\ncommit: \npass: true\n`, no `reason:` line; file-exists missing -> adds `reason: proof files missing or not regular files: proof.txt`. The number and the "unknown against a known min_tests" cases (`tests: 2`; `reason: test totals unknown; at least 3 tests required`) need a shell command, because a file-exists verify never has a total (verify.go:96-117 leaves Tests nil). Those two subtests skip on Windows like the existing verify test. |
| 5 RunVerify refusals | HOLDS | `internal/job/verify_refusals_test.go` TestRunVerifyRefusals, ten subtests: cancelled ctx (`context canceled`), two options, missing folder (not-exist error), folder that is a file, negative min_tests (with a kind and with a command), command with a kind, file-exists with no paths, kind "" and kind "shell". Each checks the error text, that `Reason` is the same text on one line, `ExitCode == -1` and `Pass == false`. |
| 6 verify_paths | HOLDS (Windows branch: compiled, needs Windows run) | TestRunVerifyPathRefusals: relative worktree with a `task.md` folder -> the same error `task.Read` returns; `a/b`, `a\b`, `C:\x` -> `invalid recorded worktree name; specify --dir` (the `C:\x` case is skipped on Windows); two worktrees -> `multiple worktrees; specify --dir`; `task:` Path and Paths with TaskPath "" -> `task: proof path requires a task folder`. On Linux, `WorktreePath(.., "C:\x")` is refused. The Windows branch asserts `C:\x` is returned as an absolute path (verify_paths.go:16-17); compiled, needs Windows run: `GOOS=windows go test -run TestRunVerifyPathRefusals ./internal/job/`. |
| 7 checks | HOLDS | `go test -count=1 ./internal/job/jobcmd/ ./internal/job/` exit 0, 0 fail. `GOOS=windows go vet` and `GOOS=darwin go vet` of both packages exit 0. `GOOS=windows go test -c -o /dev/null` of both packages exit 0. `make lint` exit 0 (`go vet ./...`, no output). |
| Dependent packages | HOLDS | Importers from `go list` (imports and test imports): cli, hook, job/jobcmd, job/lint, jobgraph, lease, orchestrator, ports, plan, golden, contract/formats. `go test -count=1 ./internal/job/... ./internal/cli/ ./internal/hook/ ./internal/jobgraph/ ./internal/lease/ ./internal/orchestrator/ ./internal/ports/ ./internal/plan/ ./internal/golden/ ./contract/formats/` -> all ok, exit 0. |
| New tests fail on base | NOT APPLICABLE | The lane adds tests for behaviour that already exists and changes no non-test file, so on base they pass (`go test` of the new files on an untouched tree is the same code). No failing line can be pasted; the proof is the coverage rise in row 1. No bug was found, so no row was dropped. |
| No existing test edited, none deleted | HOLDS | `git diff origin/main --stat` lists only the two new files. |
| No non-test file, no guard/gate/cap | HOLDS | Same diff. |
| No attribution, no personal paths | HOLDS | One commit, one-sentence message, no trailer. Test files use `t.TempDir()` only. |
| Approval record | HOLDS | The tests never write, remove or assert on approval.json (rebase note for lane-M-plugin-held-f-e). |

## What changed

- `internal/job/jobcmd/verify_refusals_test.go` (new): TestVerifyArgumentRefusals, TestLoadVerifyJobRefusals, TestVerifyTextOutput.
- `internal/job/verify_refusals_test.go` (new): TestRunVerifyRefusals, TestRunVerifyPathRefusals.

Where the code differs from the brief's wording: the task folder for `task.Read` has to be named `T-NN-slug` (`invalid task folder` otherwise), so the path tests use `T-01-example`. A text-form total cannot come from a file-exists verify, so those two subtests use a shell command and skip on Windows.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: any Windows or macOS test execution. The Windows branch of the `C:\x` subtest is compiled only (`GOOS=windows go test -c`), and needs a Windows run.
