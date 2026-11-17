# P-b1-landed-after-ready: a `landed` after edge lints and becomes ready in the live pass

Repo plugin, branch lane-P-b1-landed-after-ready, from main 5d7c2242 (a6c5cb02 at the lane start). Map 1205b T-1205b-1.

## Proof table

| Row | Result |
|---|---|
| 1. `go test -count=1 -v ./internal/job/lint/ -run TestB1LandedAfter` | HOLDS, exit 0: `--- PASS: TestB1LandedAfterLintsClean`, `--- PASS: TestB1LandedAfterUnknownKindRefused`. The unknown-kind case asserts rule `job.after-kind` and that its message or fix names `landed` (fix: "Use done, failed, children-done or landed."). The existing pin lint_test.go:148 (kind "unknown") is untouched and passes. |
| 2. `go test -count=1 -v ./internal/orchestrator/ -run 'TestB1ReadyAfterLanded\|TestB1NotReadyBeforeLanded'` | HOLDS, exit 0: `--- PASS: TestB1NotReadyBeforeLanded`, `--- PASS: TestB1ReadyAfterLanded`. J-2 waits on `landed` of J-1; with J-1 Accepted and no landed post `readyIDs` has no J-2; after a landed post (body `main_sha`, author role `cli`, no lease) `readyIDs` is `[J-2]`. Built on `setup`, `f.job`, `f.post`, `readyIDs`. |
| 3. `go test -count=1 ./internal/jobgraph/ ./internal/contract/ ./internal/feed/ ./internal/orchestrator/ ./internal/job/lint/` | HOLDS, exit 0 (all five `ok`). |
| 3b. Importers of the changed packages (go list: approvefix, board, cli, hook, job/jobcmd, lease, mcp, migrate, question, runner, task, usage, cmd/grid, lib) | HOLDS: `go test -count=1` on each, all `ok` (no test files in approvefix, runner, cmd/grid); `./internal/cli/` whole, `ok` 133.8s. |
| 4. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0, run after `git fetch origin && git merge origin/main` ("Already up to date"). |
| 5. `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| 5. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0. |
| No attribution lines: `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` | HOLDS, prints 0. |
| New tests fail on base | HOLDS, see the mutations below (the base state of both production lines). |
| `make realplugin` guest walk tables | not run, not asked (plugin repo; those are app-repo checks). |
| Whole-module test, cross-repo set | not run, the orchestrator runs them at landing. |

## Mutations (one per Go row)

1. Drop `landed` from the word set at lint.go:254 (`wordSet("done failed children-done")`):
   `b1_landed_after_test.go:32: a landed after edge has findings: [{Rule:job.after-kind Line:21 Severity:error Message:Invalid dependency kind Fix:...}]` and `--- FAIL: TestB1LandedAfterLintsClean`. Reverted.
2. Drop `Landed: s.Landed` at ready.go:145 (`CreatedAt: s.Created}`):
   `b1_ready_after_landed_test.go:44: ready after the landed post = [], want [J-2]` and `--- FAIL: TestB1ReadyAfterLanded`. `TestB1NotReadyBeforeLanded` still passes, as it should. Reverted. `git status --short` clean before commit.

## What changed

- internal/job/lint/lint.go:254-255: `landed` is in the kind word set; the fix reads "Use done, failed, children-done or landed." An unknown kind is still refused.
- Feed read, option chosen: a new exported helper `feed.FoldReadLoopAt` in internal/feed/questions.go, returning `(State, contract.Loop, decisions)` with the same withdraw rule. `FoldReadAt` keeps its signature and now calls it, so its other callers see no change (`contract.Fold` was already `FoldLoop` with the Loop dropped).
- internal/orchestrator/ready.go: `Snapshot.Landed map[string]bool`, filled in `Load` from `Loop.Landed` keys; `readyAt` passes `Landed: s.Landed` in the jobgraph input.
- ReadySince for a landed edge is unchanged: the landed time is not used. A landed edge dates from the target's settle time (else the Job's Created time), as before.
- New tests: internal/job/lint/b1_landed_after_test.go, internal/orchestrator/b1_ready_after_landed_test.go. Package-level names start with `b1`.
- Not touched: skills/plan-jobs/SKILL.md:44 (noted in INDEX.md). No guard, gate, cap or hold added.

## NOT VERIFIED

- On a live launcher: none exists yet. The lane is proved on the Feed fixture only (a real Feed on disk with real posts), not in a running launcher or a live pass.
