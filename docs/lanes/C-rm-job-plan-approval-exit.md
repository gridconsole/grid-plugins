# C-rm-job-plan-approval-exit

`grid job render` drops the dead exit-code-2 loop for a `job.plan-approval` finding. A lint error now exits 1 straight away, with the same report as before.

Cloned base: `d0400870cc95f90b11e59a87af6cf970de097a0c` (the brief traced it on c9ca10ce; `internal/job/jobcmd/command.go` is the same there).

## Proof table

| Row | Result |
|---|---|
| 1. Before any edit, `git grep -n "job\.plan-approval" -- . ':!docs/lanes'` at d0400870 | HOLDS: one hit, `internal/job/jobcmd/command.go:132:	if finding.Rule == "job.plan-approval" {`. No rule emits it, nothing else reads it. |
| 2. `git grep -n "job.plan-approval" -- '*.go'` after the change | HOLDS: prints nothing (exit 1 = no match). The new test names only `job.on-blocked`. |
| 3. New test `TestRenderLintErrorExitsOne` (`internal/job/jobcmd/rm_plan_approval_exit_test.go`): the `command_test.go:86-101` fixture, `render T-01 1 --json`, asserts exit 1, at least one finding, every rule == `job.on-blocked`, no `jobs/` folder | HOLDS. Lane: `--- PASS: TestRenderLintErrorExitsOne (0.02s)`, `ok jobcmd 0.043s`. Main d0400870 (command.go stashed back, test kept): `--- PASS: TestRenderLintErrorExitsOne (0.01s)`, `ok jobcmd 0.193s`. It passes on both, as the brief expects (behaviour unchanged). |
| 4. Deleted tests | HOLDS: none deleted. No test asserted exit 2 for a plan-approval finding. `TestJobsRenderWithoutApprovedPlan` (`internal/job/no_plan_rule_test.go`) still covers render without an approved plan. |
| 5a. `go test ./internal/job/...` | HOLDS: exit 0 (job, frontmatter, jobcmd, lint ok). |
| 5b. Importers of jobcmd (`go list` import scan): `internal/cli`, `internal/job/jobcmd` | HOLDS: `go test ./internal/job/... ./internal/cli/... -count=1` exit 0. |
| 5c. `go test ./... -count=1` | HOLDS: exit 0, every package ok. |
| 5d. `make lint` (go vet) and `gofmt -l internal/job` | HOLDS: both exit 0, gofmt prints nothing. |
| Windows and macOS builds: `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS: both exit 0. |
| 6. `git diff --stat main` touches only the Touches files plus this report | HOLDS: `internal/job/jobcmd/command.go`, `internal/job/jobcmd/rm_plan_approval_exit_test.go`, `docs/lanes/C-rm-job-plan-approval-exit.md`. |
| No guard, gate, cap or hold added on agents | HOLDS: a code path was removed, nothing added. |
| No attribution lines in commits, no personal paths, hosts or emails | HOLDS. |

## What changed

- `internal/job/jobcmd/command.go`: the loop that set `code = 2` for a `job.plan-approval` finding is gone. The lint-error branch returns `output(e, f, report{ID: j.ID, Findings: findings}, 1)`. The other exit codes of `grid job` are untouched.
- `internal/job/jobcmd/rm_plan_approval_exit_test.go`: new test pinning exit 1, the `job.on-blocked` rule and no `jobs/` folder for a render lint error.

## NOT VERIFIED on a live screen

Nothing here has a screen. This is a CLI exit code, and the test covers it.
