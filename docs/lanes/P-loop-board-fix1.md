# P-loop-board-fix1

Rules `TestOrphanNoExit` for the Job state that P-loop-board folds to `submitted`, fixes the stale r23 comment, and answers the two review questions on the claim. Branch `lane-P-loop-board-fix1`, from `lane-P-loop-board` (fd94f7da). `origin/main` (414347a3) is already an ancestor of the head: `git fetch origin` then `git merge origin/main` said "Already up to date", and `git merge-base --is-ancestor origin/main HEAD` exits 0.

**Result: DONE.** The whole module is green on this head, with the one red test of the base lane fixed as the orchestrator ruled (08:03).

## What changed

- `internal/orchestrator/tree_orphan_test.go:170-171`, and only those two lines. The assertion was "a worker-exit is not an orphan" (`r.Orphan != nil`). It is now "a worker-exit is not a `no_exit` orphan" (`r.Orphan != nil && r.Orphan.Kind == OrphanNoExit`), with the reason in the failure message. `git diff fd94f7da HEAD -- internal/orchestrator/tree_orphan_test.go` shows one hunk, `@@ -170,2 +170,2 @@`. The `no_exit` check at `:160`, the result check at `:165-168` and the test comment at `:148-149` are untouched. A Job with a result and exit `done` is `submitted` by the fold, so after its lease `neverJudged` (`write.go:306`) names it `never_judged`, as `TestR23NeverJudgedNamesASubmittedJobNobodyJudges` expects. `neverJudged` is not changed.
- `internal/orchestrator/r23_never_judged_test.go:12-16`: comment only. The fold, not the tick, moves J-1 to `submitted` at the exit `done`, and the tick posts no control for it. `git diff -U0 fd94f7da HEAD -- internal/orchestrator/r23_never_judged_test.go` shows `@@ -13,3 +13,4 @@`, comment lines only.
- This doc. No other file changed. No production code changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `TestOrphanNoExit` passes with the narrowed assertion; its diff shows only lines 170-171 | HOLDS | `go test -count=1 -v -run 'TestOrphanNoExit' ./internal/orchestrator/` gives `--- PASS: TestOrphanNoExit (0.12s)`, exit 0. Before the edit, on this lane, it was `tree_orphan_test.go:171: a worker-exit is not an orphan: &{Kind:never_judged ...}`. Diff: one hunk `@@ -170,2 +170,2 @@` (see above). This is a narrowing, not a new test, so it also passes on base (92e1e9a3), where a done exit stays `running` and has no orphan |
| 1b. r23 comment is current, comment only | HOLDS | `r23_never_judged_test.go:12-16`. `TestR23NeverJudgedNamesASubmittedJobNobodyJudges` and `TestR23JudgedSubmittedJobIsNoOrphan` (4 subtests) still `--- PASS` |
| 2. `go test ./...` whole module green | HOLDS | `go test -count=1 ./...`: exit 0, every package `ok`, including `internal/orchestrator 148.224s`, `internal/board 1.436s`, `internal/contract 2.837s`, `internal/feed 24.075s`, `internal/cli 66.685s`. No FAIL line |
| 2. Importers of the changed packages | HOLDS | The only package changed is `internal/orchestrator` (test files). `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` is the importer list; the whole-module run above covers every package on it |
| 2. Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok .../internal/cli 0.090s`, exit 0 |
| 2. `go test -count=1 ./internal/job/lint/` | HOLDS | `ok .../internal/job/lint 0.412s` in the whole-module run, and `ok 0.030s` in the named run with `TestR23OrchestratorNamesNeverJudged` and `TestR23JobContractSaysTheJudgePostsReviewAndRework` |
| 2. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 2. `make cross` | HOLDS | exit 0. `darwin/arm64` and `windows/amd64` vet and test compile, every package `ok`, including `internal/board` |
| 3. Every P-loop-board row, on this head | HOLDS | See "Base table rerun" below |
| 4. Answers (a) and (b) | HOLDS | See "Answers" below |
| Merge of origin/main before the last proof run | HOLDS | `git fetch origin` exit 0, `git merge origin/main`: Already up to date (414347a3 is an ancestor of HEAD) |
| Doc-parsing tests on the final lane doc | HOLDS | `go test -count=1 ./internal/feed/ ./internal/golden/ ./internal/land/ ./internal/job/lint/`, exit 0: feed `ok`, golden `ok`, land `ok`, lint `ok`. These are the packages whose tests read `docs/lanes`. The cli walk tests again give `ok` |
| Live run, deploy, `claude`, `curl` | not run, the brief forbids it | |
| App repo tests, `make webui-test`, the app guest walk tables | not run, not asked: plugin lane | |

## Base table rerun (row 3)

The rows are quoted from `docs/lanes/P-loop-board.md`. Each was rerun on this head with `go test -count=1 -v -run 'TestPlBd' ./internal/board/ ./internal/contract/ ./internal/orchestrator/` (exit 0), and the standing rows with the commands above.

| Base row | Now | Proof on this head |
|---|---|---|
| 1. (V9) A post with a lower attempt folds stale and changes nothing | HOLDS | `--- PASS: TestPlBdOlderAttemptFoldsStale (0.06s)` with `marked_stale_by_the_reader`, `another_worker`, `another_fence` |
| 1b. What must not be stale | HOLDS | `--- PASS: TestPlBdNotOlderAttempt (0.03s)`, all 4 subtests |
| 2. (V9) `Claim` twice on one key gives `ErrClaimed`, also from two processes | HOLDS | `--- PASS: TestPlBdClaimTwiceInProcess (0.12s)` (`one_after_the_other`, `at_once`), `--- PASS: TestPlBdClaimTwoProcesses (0.36s)` (this process first then a child, a child first then this process, children at once), `--- PASS: TestPlBdKey`, `ok .../internal/board` |
| 3. (V9) Exit done with result folds Submitted with no control post | HOLDS, and the finding is closed | `--- PASS: TestPlBdDoneWithResultFoldsSubmitted (0.06s)` (5 subtests), `--- PASS: TestPlBdDoneWithResultSubmitsWithoutControl`, `--- PASS: TestPlBdDoneWithoutResultKeepsControl`, `--- PASS: TestPlBdDoneWithOlderResultKeepsControl`. `TestOrphanNoExit` is green |
| 4. (V9, F8) A side post from a superseded attempt folds `stale` "older attempt" | HOLDS | Same test as row 1: `another_worker` and `another_fence` |
| Standing: board, contract, feed, orchestrator packages | HOLDS | `ok` for each in the whole-module run (orchestrator `148.224s`). The base lane's FAIL on orchestrator is gone |
| Standing: whole module | HOLDS | exit 0, see row 2 above. The base lane's only failure was `TestOrphanNoExit` |
| Standing: walk tests | HOLDS | see row 2 above |
| Doc-parsing tests on the final lane doc | HOLDS | see the last row below |
| Standing: `make lint`, `make cross` | HOLDS | exit 0, see row 2 above |
| Unedited tests that must pass | HOLDS | `go test -count=1 -v -run 'TestMoves$\|TestUsageLimitReoffer\|TestYaUrWalk\|TestYaUrReoffer\|TestStreamReoffersUsageLimitParkedJob\|TestUsageHoldBlocksHostDemand\|TestR23\|TestAssignClearsTheExitCode\|TestControlTenures\|TestFoldScenarios\|TestAssignmentFold\|TestOrphan' ./internal/contract/ ./internal/feed/ ./internal/orchestrator/ ./internal/cli/ ./internal/routine/ ./internal/job/lint/` exits 0: `TestMoves` (6 rows including `done`), `TestUsageLimitReoffer`, `TestYaUrWalk`, `TestYaUrReoffer` (16 subtests), `TestStreamReoffersUsageLimitParkedJob`, `TestUsageHoldBlocksHostDemand`, the r23 orchestrator tests (`TestR23NeverJudgedNamesASubmittedJobNobodyJudges`, `TestR23JudgedSubmittedJobIsNoOrphan` with 4 subtests, `TestR23LatestResultNeedsItsOwnWake`, 4 `TestR23PlanFault*`), `TestAssignClearsTheExitCode`, `TestControlTenures`, `TestFoldScenarios` (29 scenarios, including `usage-limit-reoffers-same-attempt`, `usage-limit-hold`, `checkpoint-survives-retry`, `stale-old-result-ignored`, `duplicate-assign-ignored`), `TestAssignmentFold`, and all 7 `TestOrphan*` |
| `contract/schemas/control.json` unchanged | HOLDS | This lane changes no file under `contract/` |

## Answers

**(a) The claim key includes the post type: keep.** `Key.holds` (`internal/board/board.go:56-58`) compares `r.Type` with the claim post's type, and `JSONL.Claim` passes `in.Type` (`internal/board/jsonl.go:55`). One attempt writes several post types under the same (task, Job, attempt) header (ack, checkpoint, result, worker-exit), so a key without the type would let any earlier post of that attempt make an `ack` claim fail, and the claim post would then block the attempt's own result. The brief's key names no type, so this one is the only reading under which `Claim` can be called more than once per attempt.

**(b) No caller posts a claimable type through `Post`, so no code and no two-process test.** Evidence:

- `internal/board` has no importer. `grep -rn "internal/board" --include=*.go .` gives no hit, and `go list ./...` shows the package's own entry as the only line that names it. `JSONL.Post` (`internal/board/jsonl.go:18`) has no caller. `Claim` is called only by `internal/board/pl_bd_claim_test.go:93` and `:316`.
- The post type a launcher will claim is `started`, which replaces `ack` (`docs/research/run-jobs-launcher.md:127`, `:171`). No `started` post exists in this module yet.
- `ack` is written today by three paths that go to the Feed, not to the Board, and none takes the claim lock: `Runner.ack` (`internal/runner/runner.go:576-583`) through `orchestrator.Write` (`internal/orchestrator/write.go:61`, post at `:97`), the orchestrator CLI `ack` verb (`internal/cli/orchestrator.go:445` picks `orchestrator.Write`, called at `:456`), and `grid feed post --kind ack` (`internal/cli/feed.go:169`, `feed.Store.Post`). Each is guarded by the Job's lease fence (`write.go:79`) and the Feed lock. They are not `Board.Post` callers, so the brief's rule does not apply to them. Moving them to `Claim` is the launcher lane's work (P-loop-core: "claim under `loop/start.lock`", `docs/research/run-jobs-launcher.md:646`), where an `ack` writer on the Feed path and a `started` claim would have to agree on one lock.

## NOT VERIFIED on a live screen

Nothing here has a screen. The claim between two real launchers on two logins is not run: no launcher exists yet.

## Open items

- For P-loop-core: a launcher that posts `started` must call `Board.Claim`, never `Board.Post`, and the Feed-path `ack` writers above need a decision there (one lock for both paths, or retire them).
- `tree_orphan_test.go:148-149` still says "A result, or a worker-exit, is not an orphan." This is true for the `no_exit` orphan only. The brief limits the diff to lines 170-171, so the comment is left, and the failure message on line 171 carries the reason.

DONE
