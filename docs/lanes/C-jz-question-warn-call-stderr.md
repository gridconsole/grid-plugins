# C-jz-question-warn-call-stderr

The withdrawal-policy warning of a Feed fold goes to the call's Stderr, not the host's.

Base: plugin main `dc301fce757d6374de26d8d1b68de26575341b32` (the brief names 35af7d3c; main is dc301fce, which holds C-jw-feed-lock-context-tests). Read errors versus missing file (questions.go:252-263): `workflow.GatePostRoles` returns an `*os.PathError` for an unreadable workflow.md, which fails closed and warns `cannot read task withdrawal policy (workflow.md); using withdraw: []`; a missing file takes the defaults with no warning.

## Proof table

| Row / rule | Result | Command, exit code, or file:line |
|---|---|---|
| 1. walk: every keyed `feed.Store` literal in internal/cli has a `Warn` key | HOLDS | `go test -count=1 -v -run 'TestJzQw' ./internal/cli/` exit 0: `--- PASS: TestJzQwEveryStoreLiteralSetsWarn`, `--- PASS: TestJzQwWalkTellsLiterals`. On base (production files checked out from dc301fce, tests kept) exit 1, naming all nine: `approve_record.go:94`, `deliver_keeper_job.go:108`, `deliver_keeper_job.go:163`, `feed.go:216`, `orchestrator.go:156`, `orchestrator.go:172`, `orchestrator.go:527`, `reopen_job.go:28`, `task_new_first_job.go:82`: "feed.Store literal has no Warn key; set Warn: env.Stderr". Recount on base: nine keyed, as the brief says. The walk fails if it sees fewer than nine. A zero-value `feed.Store{}` is skipped (`TestJzQwWalkTellsLiterals` proves keyed/no Warn, Warn, `&feed.Store{}`, zero value, another package's `Store`). |
| 2. lib.Run `feed state --json` with unreadable policy: line in the call's Stderr, none on the pipe | HOLDS | `go test -count=1 -v -run 'TestJzQw' ./lib/` exit 0: `--- PASS: TestJzQwPolicyWarningGoesToCallStderr`. workflow.md is a directory; os.Stderr is a pipe during the call. On base it fails: `t2_jz_warn_test.go:76: call Stderr holds the warning 0 times, want once: ""` and `t2_jz_warn_test.go:79: the host's os.Stderr got the warning: "warning: cannot read task withdrawal policy (workflow.md); using withdraw: []\n"`. |
| 3. two calls behind a start barrier, own Stderr and own Grid home: line once in each | HOLDS | Same run: `--- PASS: TestJzQwTwoCallsEachKeepTheirWarning` (start barrier is a closed channel after a ready WaitGroup; no sleep). On base: `call 0: Stderr holds the warning 0 times`, `call 1: ... 0 times`, and the host pipe holds the line twice. |
| 4a. `go test -count=1 ./internal/feed/ ./internal/orchestrator/ ./internal/task/ ./internal/question/ ./lib/` | HOLDS | exit 0: ok feed 142s, orchestrator 335s, task 59s, question 25s, lib 78s. |
| 4b. `go test -count=1 -run 'Feed|Orchestrator|Job' ./internal/cli/` | HOLDS | exit 0: `ok internal/cli 31.493s`. |
| Importers of the changed packages (feed, orchestrator, task, question, cli) | HOLDS | List from `go list -f` over Imports, TestImports, XTestImports: cmd/grid, contract/formats, internal/account, approvefix, braincmd, cli, feed, golden, guard, hook, job, job/jobcmd, job/jobtest, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, question, settings, signal, task, updates, usage, workflow, lib. All ok except: `internal/land` failed once under load (`TestFailureTimeoutAndEnvironment/environment`: land timed-out: context deadline exceeded, while two other test runs and make lint ran) and passed on its own: `go test -count=1 ./internal/land` exit 0 (31s). `internal/cli` hit the 10m default go test timeout in the same loaded run (panic while `TestGuestProjectParity` was still running, not hung); rerun alone: `go test -count=1 -timeout 40m ./internal/cli` exit 0 (`ok internal/cli 130.404s`). |
| 5a. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output). |
| 5b. `make cross` | HOLDS | exit 0: `cross: darwin/arm64 vet`, `darwin/arm64 test compile`, `windows/amd64 vet`, `windows/amd64 test compile`. |
| Walk tests: `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (0.509s). |
| Walk tests: `internal/job/lint` | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0 (3.152s). |
| Walk tests of the app repo (webui-test, TestFvArgvCollects, cmd/grid-server, slices Imports) | n/a | App-only; this repo is the plugin. |
| New tests fail on base, pass on lane | HOLDS | Rows 1-3 above; base failures pasted. Base was reproduced with `git checkout dc301fce -- internal/feed internal/orchestrator internal/task internal/question internal/cli`, then `git checkout HEAD -- <same>`; `git status --short` showed only the two new test files before they were committed. |
| No existing test edited or deleted | HOLDS | `git diff --stat dc301fce..HEAD -- '*_test.go'` lists only the two new files. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change adds one optional writer; nothing blocks or limits any agent. |

## What changed

- `internal/feed/store.go`: `Store.Warn io.Writer`; nil is the process stderr, as before.
- `internal/feed/fold.go:22`, `internal/orchestrator/ready.go:45`, `internal/orchestrator/tick.go:30`, `internal/task/gates.go:272`: pass the store's `Warn` as the writer of `QuestionWithdrawReadPolicy`. `questions.go` is untouched: its `policyWarningWriter` already falls back to `os.Stderr` for a nil writer.
- `internal/question/feed.go:29`: `question.Store.Warn` already existed; the feed store it builds now carries it.
- `internal/cli`: `Warn: env.Stderr` on all nine keyed literals (feed.go:216, approve_record.go:94, deliver_keeper_job.go:108 and :163, orchestrator.go:156, :172, :527, reopen_job.go:28, task_new_first_job.go:82). Three helpers had no `env` in reach, so they take it as a first parameter: `curatorJobOpen(env, dir)` (deliver.go:82 is its only caller), `planPersonJob(env, dir)` (task_write.go:250 only), `recordApproval(env, dir, ...)` (task_write.go:270 and orchestrator.go:507). The zero-value `feed.Store{}` error returns got no key.
- New tests: `internal/cli/jz_store_warn_test.go`, `lib/t2_jz_warn_test.go`.

## Left on the fallback (os.Stderr)

Literals outside internal/cli that hold no writer of their own, so they still write the warning to the process stderr when their fold meets an unreadable workflow.md: `internal/approvefix/approvefix.go:39`, `internal/hook/events.go:373`, `internal/job/jobcmd/feedjob.go:45`, `internal/lease/job.go:50-51`, `internal/migrate/receipt.go:108,137`, `internal/orchestrator/stream.go:192` (its struct has `Diag io.Writer`, but that is "one readable line per skip; nil discards", a different channel from a nil-means-stderr warning, so I did not wire it), `internal/task/gates.go:265` (`planJobResult(dir)` takes only a dir; the store's `Warn` is passed at :272 but is nil there). `internal/question/command.go:204` already carries `Warn: env.Stderr` on its own struct. Whether `job next` and the other lib calls that go through jobcmd/lease/hook paths should also get a writer is a follow-up: those structs need a writer added first.

## NOT VERIFIED on a live screen

None of this is UI. Nothing here needs a live screen.
