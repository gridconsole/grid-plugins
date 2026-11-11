# C-jw-feed-post-job-lock: feed post --type job and MCP feed_post take .orchestrator.lock

Code head: 1c1a4294 (the commit after it adds only this report). `git merge main` said "Already up to date."

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. `grid feed post --type job` with the lock held waits for the block event, exits 0 after unlock | HOLDS | `go test -count=1 -v -run TestJwFj ./internal/cli/` exit 0: `--- PASS: TestJwFjFeedPostJobWaitsForPassLock`, `--- PASS: TestJwFjLeasedFeedPostJobWaitsForPassLock` (leased writer, `--lease task`). The test file is internal/cli/jw_feed_post_job_lock_test.go:105-128 (helpers :14-102). On the base (test file only, no production change) the run failed: `jw_feed_post_job_lock_test.go:121: the post finished with exit 0 while .orchestrator.lock was held, and with no block event on it` / `--- FAIL: TestJwFjFeedPostJobWaitsForPassLock` and the same line at :136 for the leased test. |
| 2. The same through MCP feed_post | HOLDS | `--- PASS: TestJwFjMCPFeedPostJobWaitsForPassLock` (same run). Setup is copied from internal/cli/mcp_test.go (`feedHarness`, `GRID_FEED_TASK`, `mcpHello`, `h.run(h.home, "mcp")`). Base: `jw_feed_post_job_lock_test.go:153: the post finished with exit 0 while .orchestrator.lock was held, and with no block event on it` / `--- FAIL: TestJwFjMCPFeedPostJobWaitsForPassLock`. |
| 3. A held seed through feed post still exits 0 and is on the Feed | HOLDS | `--- PASS: TestJwFjHeldSeedStillAccepted` (job after a missing id, exit 0, `Verdict == "held"` on the Feed). It passes on the base too: it guards that PostJobLocked is not PostPerson, so it cannot fail on the base. The existing `TestJoRpHeldIDSkipped` (leased `feed post --type job --after <missing>`) goes through the new path and passes in the full cli run. |
| 4. A non-job post with the lock held finishes with no block event | HOLDS | `--- PASS: TestJwFjFeedPostOtherTypeTakesNoPassLock` (counts block events on .orchestrator.lock: 0). The brief says `--type note`; no `note` type is registered (`grep` of internal/contract finds none), so the test posts `--type post` (message). Passes on the base too (guard). |
| 5a. `go test -count=1 -run 'TestJwFj\|Feed' ./internal/cli/ ./internal/mcp/ ./internal/orchestrator/` | HOLDS | exit 0: `ok internal/cli 1.877s`, `ok internal/mcp 0.095s`, `ok internal/orchestrator 0.742s` |
| 5b. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 5c. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok |
| 5d. `git diff main --stat` names only Touches | HOLDS | feed.go (+3), mcp/tools.go (+3), orchestrator/jw_post_job.go (+32, new), cli/jw_feed_post_job_lock_test.go (+194, new). This report (docs/lanes) is added after that run, as the runbook asks. |
| Rule: tests of changed packages and every importer | HOLDS | Changed: internal/cli, internal/mcp, internal/orchestrator. `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep ...` lists importers: cmd/grid, internal/migrate, lib. `go test -count=1 ./cmd/grid ./internal/cli ./internal/mcp ./internal/migrate ./internal/orchestrator ./lib` exit 0: cmd/grid no test files; cli 155.966s ok; mcp 1.015s ok; migrate 32.091s ok; orchestrator 180.104s ok; lib 48.235s ok. |
| Rule: no sleep, no elapsed-time assertion | HOLDS | Waits are on the `queued` channel from `fslock.OnBlockForTest` and on the command's exit channel. The only timer is `jwFjWait` (30 s), the failure path. Tests do not call `t.Parallel`; the hook is restored by `t.Cleanup`; the hook compares the exact lock path and sends without blocking (:50-66). |
| Rule: no guard, gate, cap or hold on agents | HOLDS | The change adds a lock around `store.Post` only; it refuses nothing new. Nothing removed. |
| Rule: no deleted or edited existing test | HOLDS | `git diff main --stat` shows no existing test touched. |
| Rule: no attribution lines, no personal paths, hosts or emails | HOLDS | Commit message is one sentence, no trailer. |
| Rule: `*RealPlugin*` tests | n/a | No row names one; none touched. |

## What changed

- internal/orchestrator/jw_post_job.go (new): `PostJobLocked(store, in, w)` takes the pass lock and calls `store.Post`. A writer with a lease (`w.Lease != ""`) goes through `Store.locked` (lease file shared, then the pass lock, store.go:112-115). A writer with none goes through `Store.exclusive` (pass lock only), as AnswerPerson picks at person.go:174-177. It is not PostPerson: no `checkAfterTargets`, no id picked, so a held seed is accepted as before.
- Which caller gets which (the Outcome the brief asks for):
  - `grid feed post --type job` (internal/cli/feed.go:167-168): `--lease task` with an epoch (or `GRID_LEASE`) gives `locked`; a member or person with no lease gives `exclusive`. A Job lease never arrives; `refuseJobSide` refuses it first.
  - MCP `feed_post` (internal/mcp/tools.go:225-226): the Session builds the writer from `GRID_LEASE` and `GRID_LEASE_EPOCH`; a leased session gets `locked`, an unleased one `exclusive`. `feed_handover` and every other type are unchanged.
- Test file internal/cli/jw_feed_post_job_lock_test.go: five tests (`TestJwFj...`), helpers `jwFj...`.

## Notes for the reviewer (lock order on the Job post path)

- Order is lease file (task, shared) then `.orchestrator.lock` (exclusive) then `store.Post`, which takes its own wake/lease/tenure locks inside, as the package comment says for a pass. `AnswerPersonWith` and `PostPersonUnless` already nest `store.Post` in the same way.
- `Store.exclusive` creates the task's feed folder (mode 0700) if it is missing, before `Post` validates; a refused job post on a task with no Feed now leaves an empty feed folder. PostPerson does the same today. No existing test broke on it.
- For a leased writer with a bad epoch or a missing lease file, `WithLease` returns `ErrStaleLease` before `Post` runs, so that error comes from the guard and not from `Post`; the exit code is the same path as before in the cli and mcp suites (all pass).

## NOT VERIFIED on a live screen

Nothing here is a UI change. No `*RealPlugin*` run: no row names one. The whole-module test and the cross-repo set are the orchestrator's at landing.

DONE
