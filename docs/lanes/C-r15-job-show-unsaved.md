# C-r15-job-show-unsaved: `grid job show` prints a Job that has no saved jobs folder (round 15 F6)

Repo: plugin. Branch lane-C-r15-job-show-unsaved.

## Failing on base

`grid job show <task> J-2` on a `grid question ask` Job (new test, base, command.go stashed):

```
--- FAIL: TestJobShowWithoutSavedFolder (0.06s)
    r15_job_show_unsaved_test.go:45: job show J-2: exit 1:  job: lstat .../tasks/T-07-example/jobs: no such file or directory
```

## Proof table

| Row | Result |
|---|---|
| 1. show of a Job with no `jobs/<id>/brief.md` prints it from its Feed job record (id, role, kind, state from `feed.FoldRead`, title, after, and the person Job's question from intent.goal); `--json` gives `job`, `state`, `state_reason`, `record` as for a saved Job | HOLDS, internal/job/jobcmd/command.go `showFromFeed`, `feedBrief`, and the fallback in the `show` case |
| 1. unknown id (neither Feed nor disk) is still an error | HOLDS, `go test ./internal/cli -run TestJobShowWithoutSavedFolder` exit 0 (J-99 exits 1, text and `--json`) |
| 1. a guest keeps the access `resolveTask` gives it | HOLDS, `resolveTask` still runs first in `showPath`; the fallback runs only after it returned a task dir (command.go `showPath`, unchanged) |
| 1. no folder written for a person Job | HOLDS, the test stats `jobs/<id>` before and after show: absent both times |
| 1. one place covers question, ship and posted-not-started Jobs | HOLDS, one fallback in `show`; no brief writer added. Question Job and posted Job tested; the ship Job takes the same path (a Feed job record with no folder) |
| 1. reword "is not posted yet" | HOLDS, internal/job/source.go:126 now "Job J-n has no saved brief yet (it is rendered when it starts)". The brief names jobcmd/source.go; the line is in internal/job/source.go |
| 2. tests: question Job, posted-not-started Job, unknown id, text and `--json` | HOLDS, internal/cli/r15_job_show_unsaved_test.go `TestJobShowWithoutSavedFolder`, exit 0; fails on base (above) |
| 3. `go test ./internal/job/... ./internal/cli/...` | HOLDS, exit 0 (job 1.6s, jobcmd 2.0s, lint 0.4s, cli 41.8s) |
| Importers of the changed packages | HOLDS, `go test -count=1 ./internal/job/... ./internal/cli/... ./internal/hook/... ./internal/lease/... ./internal/migrate/... ./internal/orchestrator/... ./internal/ports/... ./internal/jobgraph/... ./internal/mcp/... ./cmd/grid/... ./lib/...` all ok (list from `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: importers of internal/job are cli, hook, lease, migrate, orchestrator, ports, jobgraph, jobcmd, lint, jobtest; of jobcmd: cli, cmd/grid via cli, lib via cli). The run before the last test edit failed only TestQeOldShowLineFailsOnCancelledJob; see below, then cli passed |
| `make lint` | HOLDS, exit 0 (`go vet ./...`); `gofmt -l internal` prints nothing |
| Windows build | HOLDS, `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS, `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Walk tests | HOLDS, `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| `git fetch origin && git merge origin/main` | HOLDS, "Already up to date" before the final run |
| Real-plugin tests | not run, not asked (plugin repo; the show path needs no built plugin) |
| Whole-module test, cross-repo set | not run, the orchestrator runs them at landing |

## Existing tests I changed (the brief's direction)

- internal/job/ft_job_not_posted_test.go:24-25: the expected text is now "has no saved brief yet" instead of "is not posted yet". The brief orders the reword (a posted Job renders at start); every other assertion in the test (prefix, no lstat, fs.ErrNotExist) is unchanged.
- internal/cli/qe_job_read_lines_test.go `TestQeOldShowLineFailsOnCancelledJob`: it pinned `grid job show` exiting 1 on a cancelled Job that was never rendered. That is the failure the brief removes, so it now asserts exit 0 and `state: Done (cancelled)`. The test name is kept so the plan-jobs skill walk keeps its reference.

## What changed

- internal/job/jobcmd/command.go: `show` no longer stops at a missing `jobs/<id>/brief.md`. When showPath or the read fails with not-exist and a task resolved, `showFromFeed` folds the task Feed and takes the newest non-stale job record for the id (`pickJob`, as render does). Text shows id, role, kind, title, state (with its reason), after, and `question:` (person Job) or `goal:`, then a line saying no brief is saved yet. `--json` gives the same `job`, `state`, `state_reason`, `record` as a saved Job; `path` and `brief` are empty. No Feed record keeps the original error.
- internal/job/source.go:126: reworded error.

## Not verified on a live screen

Nothing here is UI. Not run against a real Grid home with a live worker: the tests use the harness Feed, which is the same store.
