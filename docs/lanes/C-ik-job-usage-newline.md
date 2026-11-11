# C-ik-job-usage-newline

Base sha: ee8a96c126781462cbe0067771db40d01911cd76 (command.go:65 had no `\nExit status:`).

## Proof table

| Row / rule | Result |
|---|---|
| 1. TestIkJuJobHelpExitLine `job --help`: exit 0, exactly one line starting "Exit status:" equal to exitStatusHelp | HOLDS: `go test -count=1 ./internal/cli -run TestIkJu` exit 0. Test committed before the fix and failed on base ee8a96c1: `ik_job_usage_test.go:29: lines starting with Exit status: [], want exactly "Exit status: 0 done, ..."` (out shows `grid job cycles <items.json> [--json]Exit status: 0 done`) |
| 2. Same for `grid job` with no args; line before is `       grid job cycles <items.json> [--json]` | HOLDS: subtest `job` of the same test, same failing base line, passes on lane (ik_job_usage_test.go:30-33 checks the previous line) |
| 3a. `go test -count=1 ./internal/cli -run 'TestIkJu\|TestRecordExitHelp\|TestExitRefusalHelpGolden'` | HOLDS: exit 0, PASS for all three |
| 3b. `go test -count=1 ./internal/job/jobcmd/` | HOLDS: ok, exit 0 |
| 3c. `make lint` | HOLDS: exit 0 (go vet ./...) |
| 3d. `git diff main --stat` | HOLDS: only internal/cli/ik_job_usage_test.go and internal/job/jobcmd/command.go (plus this report under docs/lanes) |
| Importers of jobcmd (go list) | internal/cli, internal/job/jobcmd; `go test -count=1 ./internal/cli ./internal/job/jobcmd/` ok, exit 0 |
| Windows / macOS builds | HOLDS: `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0 |
| No guard/gate/cap added; no existing test edited or deleted | HOLDS |
| Commits: one sentence, no attribution | HOLDS |
| RealPlugin tests | Not named by the brief; none run |

## What changed
- command.go:65: inserted `\n` before "Exit status" in the usage literal; no other word changed.
- New internal/cli/ik_job_usage_test.go (TestIkJuJobHelpExitLine, covers `job --help` and `job`).

## NOT VERIFIED on a live screen
Nothing needs a live screen.

The usage still hand-copies exitStatusHelp (drift risk, noted in the brief); not changed because Touches limits the edit to the literal. The test now guards drift.
