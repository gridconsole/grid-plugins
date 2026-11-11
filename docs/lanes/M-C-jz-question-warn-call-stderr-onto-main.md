# M-C-jz-question-warn-call-stderr-onto-main

Lane C-jz-question-warn-call-stderr (the withdrawal-policy warning of a Feed fold goes to the call's Stderr) merged onto plugin main `b561251f` (the brief names 21f02736; origin/main had moved on to Pint118 and held the trio when this lane started). The lane's own report is `docs/lanes/C-jz-question-warn-call-stderr.md`.

## The merge

`git merge origin/main` merged with no textual conflict. It did not build: main's `sendBackPlan` (internal/cli/task_write.go:312) called `planPersonJob(dir)`, and this lane's signature is `planPersonJob(env Env, dir string)`. Build error as seen: `internal/cli/task_write.go:312:30: not enough arguments in call to planPersonJob  have (string)  want (Env, string)`. Fixed by passing the caller's `env`, as the lane's other call site at task_write.go:274 does (commit 967cbac1). `grep -rn planPersonJob internal lib cmd`: the definition (orchestrator.go:522) and two call sites, both with `env`. No other main-added call or `feed.Store{...}` literal needed a change: `TestJzQwEveryStoreLiteralSetsWarn` walks all of internal/cli and passes.

## Proof table

| Row / rule | Result | Command, exit code, or file:line |
|---|---|---|
| 0. `go build ./... && go vet ./internal/cli/` (before the lane's rows) | HOLDS | Failed after the merge (error above); after the fix `go build ./... && go vet ./internal/cli/` exit 0. |
| 1. origin/main is an ancestor; diff names only lane files | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0. `git diff origin/main --stat`: 17 files: docs/lanes/C-jz-question-warn-call-stderr.md, internal/cli/{approve_record,deliver,deliver_keeper_job,feed,orchestrator,reopen_job,task_new_first_job,task_write}.go, internal/cli/jz_store_warn_test.go, internal/feed/{fold,store}.go, internal/orchestrator/{ready,tick}.go, internal/question/feed.go, internal/task/gates.go, lib/t2_jz_warn_test.go: all files the lane C-jz touched (task_write.go includes the one-line fix above). Plus this doc once committed. |
| 2.1 (brief row 1) walk of `feed.Store` literals in internal/cli | HOLDS | `go test -count=1 -v -run 'TestJzQw' ./internal/cli/` exit 0: `--- PASS: TestJzQwEveryStoreLiteralSetsWarn`, `--- PASS: TestJzQwWalkTellsLiterals`. Base failure (nine files:lines) is in the lane's own report; the merge changed no production literal. |
| 2.2 (brief row 2) lib.Run `feed state --json`, warning in call Stderr, none on the pipe | HOLDS | `go test -count=1 -v -run 'TestJzQw' ./lib/` exit 0: `--- PASS: TestJzQwPolicyWarningGoesToCallStderr`. |
| 2.3 (brief row 3) two concurrent calls, own Stderr, start barrier | HOLDS | Same run: `--- PASS: TestJzQwTwoCallsEachKeepTheirWarning`. |
| 2.4a (brief row 4) `go test -count=1 ./internal/feed/ ./internal/orchestrator/ ./internal/task/ ./internal/question/ ./lib/` | HOLDS | task, question, lib `ok` (25s, 9.6s, 42s) in the combined run. In that run, while three other runs loaded the machine, `internal/orchestrator` hit go test's 10m default timeout (panic in orchestrator_test.go:706, not a failure of an assertion) and the feed line was cut off. Rerun alone: `go test -count=1 -timeout 40m ./internal/orchestrator/ ./internal/feed/` exit 0: `ok orchestrator 433.662s`, `ok feed 232.538s`. |
| 2.4b `go test -count=1 -run 'Feed\|Orchestrator\|Job' ./internal/cli/` | HOLDS | exit 0: `ok internal/cli 37.638s`. |
| 2.5 (brief row 5) `make lint` | HOLDS | exit 0. |
| 4. `make cross` | HOLDS | exit 0: `cross: darwin/arm64 vet`, `darwin/arm64 test compile`, `windows/amd64 vet`, `windows/amd64 test compile`, every package `ok`. (A first run was stopped by the 10m background limit under load, not a failure; rerun with a 1h timeout exit 0.) |
| Importers of the changed packages | HOLDS | `go list` over Imports of feed, orchestrator, task, question gives: approvefix, braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, lease, mcp, migrate, orchestrator, question, signal, task, updates, usage. `go test -count=1` over all but cli/orchestrator/task/question (covered above): exit 0; braincmd, guard, hook, job, job/frontmatter, job/jobcmd, job/lint, land, lease, mcp, migrate, signal, usage `ok`; approvefix, job/jobtest, updates have no test files. |
| 3. Walk test `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.515s`). |
| 3. Walk test `internal/job/lint` | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.686s`). |
| 3. App-repo walk tests (webui-test, TestFvArgvCollects, cmd/grid-server, slices Imports) | n/a | App-only; this is the plugin repo. |
| No test dropped, edited or deleted | HOLDS | The merge was clean; this lane's only new edit is the one call in task_write.go:312. Both sides' tests are present. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Nothing added that blocks or limits an agent. |
| Pushing | HOLDS | Nothing pushed. |

## What changed

- Merge of origin/main into the lane (clean).
- `internal/cli/task_write.go:312`: `planPersonJob(dir)` became `planPersonJob(env, dir)`.
- This doc.

## Left on the fallback (os.Stderr)

Unchanged from the lane's report: the feed.Store literals outside internal/cli that hold no writer (approvefix, hook/events.go, job/jobcmd/feedjob.go, lease/job.go, migrate/receipt.go, orchestrator/stream.go, task/gates.go:265).

## NOT VERIFIED on a live screen

None of this is UI. Nothing needs a live screen.

DONE
