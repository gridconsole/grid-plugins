# C-ft-job-pointer-not-posted

Base: 9dba6dc6a3c42b88f1526c01981b126ae83bd2eb

Base check: before the fix the new test failed:
`job:J-2: lstat <tmp>/T-07-demo/jobs: no such file or directory does not say the Job is not posted yet` (same for `job:J-2#verify`, `job:T-07/J-2`).

| Row | Result |
|---|---|
| 1. `go test ./internal/job/ -run TestFtJobNotPosted -count=1` (three pointer forms, prefix, "is not posted yet", ErrNotExist, no lstat/cannot read source/temp path) | HOLDS, exit 0; internal/job/ft_job_not_posted_test.go |
| 2. Posted Job resolves with no error | HOLDS, TestFtJobNotPostedPostedJobStillResolves PASS |
| 3. TestPlanPointerOnATaskWithNoPlanSaysTheFileIsMissing unchanged | HOLDS, PASS, exit 0, test file untouched |
| 4. `go test ./internal/job/... -count=1` | HOLDS, exit 0 |
| Importers tested (go list): hook, jobgraph, lease, migrate, orchestrator, ports, cli, job/jobcmd, job/lint | HOLDS, all ok, exit 0 |
| Lint: gofmt -l internal/job (empty), go vet ./internal/job/... | HOLDS, exit 0 |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS, exit 0 |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS, exit 0 |
| New test fails on base, passes on lane | HOLDS (see base check) |
| No guard/gate/cap; nothing removed; no existing test edited | HOLDS |
| *RealPlugin* tests | Not applicable: none relevant |

## What changed
- internal/job/source.go (`job:` case): a missing brief returns
  `<pointer>: Job <id> is not posted yet: jobs/<id>/brief.md does not exist: %w` wrapping fs.ErrNotExist.
- internal/job/ft_job_not_posted_test.go: new tests.

## NOT VERIFIED on a live screen
Lint's `job.pointer` text display (internal/job/lint/lint.go:347) was not viewed live; its tests pass.
