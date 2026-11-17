# P-loop-board

The board interface with its JSONL claim, the older-attempt stale rule, and an exit `done` with a result folding to `submitted`. Branch `lane-P-loop-board`, from main 92e1e9a3, merged with origin/main 414347a3 (clean merge, no conflicts) before the last proof run. V-number V9.

**Result: NOT DONE.** One existing test, `TestOrphanNoExit`, is green on base and red on this lane. Row 3 causes it. Per the brief, it is reported below and not edited.

## What changed

- `internal/board/board.go`, `internal/board/jsonl.go` (new):
  - `Board` interface (`Post`, `Read`, `Claim`), `Key` (task, Job, attempt, plus firing when given) and `ErrClaimed`.
  - `JSONL.Claim` takes `feed/.cache/claim.lock`, an flock on unix and LockFileEx on windows through `internal/fslock`. It holds the lock across a scan of the whole Feed and the append.
  - Only `Claim` takes that lock, and it takes it before any Feed lock.
  - A second claim of a key returns an error that wraps `ErrClaimed` and names the existing post. This holds in the same process or in another one.
- `internal/contract/fold.go`, older attempt:
  - A Job-side post (ack, submit, result, checkpoint, worker-exit) whose attempt is below its Job's final attempt now folds `stale` with reason "older attempt" in two cases. Either no counted assign consumed it (it was `ignored` "no matching counted assign for this tenure"), or the reader marked it stale (it was `stale` "past tenure end offset").
  - An older attempt's post that its own tenure counted stays counted.
  - Current-attempt posts that no assign matched stay `ignored`. A post after its attempt's exit stays `ignored` "attempt already exited". Non-Job-side reader-stale posts keep "past tenure end offset".
- `internal/contract/fold.go`, fence compare: the check `a.Fence <= j.Assign.Fence && p.Gen <= j.Generation` with reason "grant must advance the tenure" is removed. `sameRetry` and the line after it are unchanged.
- `internal/contract/fold.go`, done exit: a `worker-exit` with reason `done` folds the Job to `submitted` with reason "awaiting judgement" when the Job counted a result of that same attempt (`resultOf`). It needs no control post. A result of an earlier attempt does not count, and a later control still applies over it.
- `internal/orchestrator/tick.go`: comment only. The `case "done":` arm stays for a done exit with no result (ruling 2). A done exit with a result is already `submitted` and returns at the early return above it.
- `docs/job-contract.md:132`: the fold moves a done Job to `submitted` when a result of that attempt exists, and the tick does it when none does.
- New tests: `internal/board/pl_bd_claim_test.go`, `internal/contract/pl_bd_attempt_test.go` and `internal/orchestrator/pl_bd_submit_test.go`. All helpers are prefixed `plBd`.
- `contract/schemas/control.json`: not changed, and no change was needed.

## Rows

| Row | Result | Proof |
|---|---|---|
| 1. (V9) A post with a lower attempt folds stale and changes nothing | HOLDS | `TestPlBdOlderAttemptFoldsStale` (`internal/contract/pl_bd_attempt_test.go:53`). The attempt 1 result, checkpoint and `worker-exit done` posts arrive after attempt 2's assign, marked stale by the reader, from another worker, or with another fence. Each folds `stale` "older attempt". The Job deep-equals the fold without those posts, in order and shuffled. Lane: `--- PASS: TestPlBdOlderAttemptFoldsStale` with `marked_stale_by_the_reader`, `another_worker` and `another_fence`. Base fails: `pl_bd_attempt_test.go:76: result of attempt 1 folds stale "past tenure end offset", want stale "older attempt"` and `... folds ignored "no matching counted assign for this tenure", want stale "older attempt"` |
| 1b. What must not be stale | HOLDS | `TestPlBdNotOlderAttempt` (`pl_bd_attempt_test.go:88`), which passes on base by design: these are the cases the rule must leave alone. An attempt 1 checkpoint counted under its own tenure survives the retry. A current-attempt post with fence 99 stays `ignored` "no matching counted assign for this tenure". A post after its attempt's exit stays `ignored` "attempt already exited". A reader-stale control keeps "past tenure end offset". Lane: PASS with all 4 subtests |
| 2. (V9) `Claim` twice on one key gives `ErrClaimed`, also from two processes | HOLDS | `TestPlBdClaimTwiceInProcess` (`internal/board/pl_bd_claim_test.go:129`) claims an `ack` one after the other and with 8 goroutines at once. `TestPlBdClaimTwoProcesses` (`:229`) re-runs the test binary as a child (`TestPlBdClaimChild`, `:172`) and claims this process first then a child, a child first then this process, and 4 children at once. Every case has exactly one winner, every other caller gets `errors.Is(err, ErrClaimed)`, the Feed has exactly one ack, and J-1 folds `running`. `TestPlBdKey` (`:275`) covers key match and refusal, including firing. Lane: `--- PASS: TestPlBdClaimTwiceInProcess (0.05s)`, `--- PASS: TestPlBdClaimTwoProcesses (0.14s)`, `--- PASS: TestPlBdKey`, `ok .../internal/board`. Base: `internal/board` does not exist on main, so these tests cannot build there |
| 3. (V9) Exit done with result folds Submitted with no control post | HOLDS, but see the finding | Fold: `TestPlBdDoneWithResultFoldsSubmitted` (`pl_bd_attempt_test.go:131`). A result then done folds `submitted` "awaiting judgement". A done exit with no result stays `running`. Attempt 2 done with only attempt 1's result stays `running`. Attempt 2 result then done folds `submitted`. A later accept still wins. Base fails: `--- FAIL: TestPlBdDoneWithResultFoldsSubmitted/result_then_done`, `pl_bd_attempt_test.go:166: Job = &{... Status:running ...}, want submitted "awaiting judgement"`. Tick: `TestPlBdDoneWithResultSubmitsWithoutControl` (`internal/orchestrator/pl_bd_submit_test.go:49`) checks `submitted` before the tick, no new control after it, and the wake still names the result. Base fails: `pl_bd_submit_test.go:54: before the tick: want submitted "awaiting judgement", got running ""`. Ruling 2: `TestPlBdDoneWithoutResultKeepsControl` (`:70`) and `TestPlBdDoneWithOlderResultKeepsControl` (`:76`) pass on base by design. The tick still posts exactly one control, `submitted` "awaiting judgement" re the exit, for a done exit with no result of that attempt |
| 4. (V9, F8) A side post from an attempt that a newer assign superseded folds `stale` "older attempt" | HOLDS | Same test as row 1. The `another_worker` and `another_fence` cases are attempt 1 side posts that the attempt 2 assign superseded. Before, they folded `ignored` "no matching counted assign for this tenure". Now they fold `stale` "older attempt" |
| Standing: `go test -count=1 ./internal/board/... ./internal/contract/... ./internal/feed/... ./internal/orchestrator/...` | **FAIL** | After the merge: board `ok 0.258s`, contract `ok 0.900s`, feed `ok 7.737s`, orchestrator `FAIL 98.953s`, only on `--- FAIL: TestOrphanNoExit`. See the finding below |
| Standing: whole module and importers | **FAIL**, only `TestOrphanNoExit` | `go test -count=1 ./...`: every package `ok` except `internal/orchestrator`, whose only failing test is `TestOrphanNoExit`. Before the merge: same result, `FAIL .../internal/orchestrator 159.550s`. After the merge: the same, `--- FAIL: TestOrphanNoExit (0.08s)` and `FAIL .../internal/orchestrator 111.269s`, with no other FAIL line. The whole module covers every importer of `internal/contract`, `internal/board` and `internal/orchestrator` (from `go list`) |
| Standing: walk tests | HOLDS | After the merge: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` gives `ok .../internal/cli 0.039s`. `go test -count=1 ./internal/job/lint/` gives `ok 0.151s`, which includes `TestR23JobContractSaysTheJudgePostsReviewAndRework` on the edited `docs/job-contract.md` |
| Doc-parsing tests on the final lane doc | HOLDS | After the last edit of this doc: `go test -count=1 ./internal/feed/ ./internal/golden/ ./internal/land/ ./internal/job/lint/` gives feed `ok 8.258s`, golden `ok 0.271s`, land `ok 0.531s`, lint `ok 0.212s`. These are the packages whose tests name `docs/lanes`. The cli walk tests again give `ok 0.047s` |
| Standing: `make lint` | HOLDS | After the merge: exit 0 (`go vet ./...`, no output) |
| Standing: `make cross` | HOLDS | After the merge: exit 0. darwin/arm64 and windows/amd64 vet and test compile, every package `ok`, including `internal/board` (the fslock claim lock builds on windows) |
| Unedited tests that must pass | HOLDS | Run by name after the merge, all `--- PASS`: <br>• `TestMoves`, all 6 rows including `done` (`orchestrator_test.go:127`)<br>• `TestUsageLimitReoffer`, `TestYaUrWalk`, `TestYaUrReoffer` (16 subtests)<br>• `TestStreamReoffersUsageLimitParkedJob`, `TestUsageHoldBlocksHostDemand`<br>• the r23 orchestrator tests: `TestR23NeverJudgedNamesASubmittedJobNobodyJudges`, `TestR23JudgedSubmittedJobIsNoOrphan` (4), `TestR23LatestResultNeedsItsOwnWake`, and the 4 `TestR23PlanFault*`<br>• `TestAssignClearsTheExitCode`, `TestControlTenures`<br>• `TestFoldScenarios`: `usage-limit-reoffers-same-attempt`, `usage-limit-hold`, `checkpoint-survives-retry`, `stale-old-result-ignored`, `duplicate-assign-ignored`<br>• `TestAssignmentFold` (feed)<br>The r23 tests in cli, routine, lint and contract pass in the whole-module run |
| `contract/schemas/control.json` unchanged | HOLDS | `git diff 92e1e9a3 HEAD -- contract/schemas/control.json` is empty. The fold rule needs no new control field |
| Merge of origin/main | HOLDS | `git fetch origin` then `git merge origin/main` (414347a3), clean, before the last proof run. The first fetch failed while the shared disk was full (`couldn't write .../refs/remotes/origin/main.lock`). It succeeded once space came back |
| Live run, deploy, `claude`, `curl` | not run, the brief forbids it | |
| App repo tests | not run, not asked | plugin lane |

## Finding: `TestOrphanNoExit` goes red (not edited)

`internal/orchestrator/tree_orphan_test.go:150-173`. The test acks J-1, posts a result, then posts `worker-exit done` with no tick. It then asks for the tree row at lease + 1s and expects no orphan (`:169-172`):

```
--- FAIL: TestOrphanNoExit (0.08s)
    tree_orphan_test.go:171: a worker-exit is not an orphan: &{Kind:never_judged Since:2026-11-15T08:03:27.845Z Detail:submitted, and no wake of its result acknowledged after its lease ran out; the review and any rework are posted by the app's Judge}
FAIL	github.com/gridconsole/grid-plugins/internal/orchestrator	98.953s
```

On base, with `internal/contract/fold.go` from 92e1e9a3 and the rest unchanged: `--- PASS: TestOrphanNoExit (0.09s)`.

Cause: row 3 makes a result plus exit `done` fold `submitted` without a tick. `neverJudged` (`internal/orchestrator/write.go:306`) reads only fold state, so it now names that Job `never_judged` at lease + 1s. Before, the Job stayed `running` with an exit until the tick posted the control. `TestR23NeverJudgedNamesASubmittedJobNobodyJudges` expects `never_judged` for the same fold state after a tick. The two tests now differ only in whether a tick ran, and the brief makes the fold, not the tick, decide.

Options, for a ruling. Neither is done here, because the brief says not to edit a red test and `write.go` is outside Touches.

1. **Recommended:** narrow `tree_orphan_test.go:170-171` to "a worker-exit is not a `no_exit` orphan" (`r.Orphan == nil || r.Orphan.Kind != OrphanNoExit`). Its comment says the test pins Row 4, `no_exit`, and a done Job with a result that nobody judges is `never_judged` by design (the r23 rule).
2. Change `neverJudged` to skip a Job whose `submitted` came from the fold rather than a control. This would hide a real never-judged Job and contradict the r23 test, so it is not recommended.

## Base-grep hits and verdicts

The brief's exact grep command is not quoted here. This is a reconstruction on base 92e1e9a3 that returns the recorded hit list line for line: `git grep -n -e "awaiting judgement" -e "past tenure end offset" -e "grant must advance" -e "no matching counted assign" -e "sameRetry" -- internal docs`.

| Hit | Verdict |
|---|---|
| `internal/contract/fold.go:83` "past tenure end offset" | Changed as asked. Still set for every reader-stale post, then relabelled "older attempt" for a Job-side post of an older attempt (now `:85`, relabel at `:279`) |
| `internal/contract/fold.go:202` `sameRetry :=` | Unchanged, as the brief says (now `:205`) |
| `internal/contract/fold.go:203` attempt check that uses `sameRetry` | Unchanged (now `:206`) |
| `internal/contract/fold.go:208` "grant must advance the tenure" | Removed with its fence compare (`:207-208`), as asked. `duplicate-assign-ignored` still passes through the `:203` check |
| `internal/contract/fold.go:269` "no matching counted assign for this tenure" | Changed as asked. Kept for current-attempt posts, while older-attempt posts fold `stale` "older attempt" (now `:285-287`) |
| `internal/orchestrator/tick.go:330` done control | Kept (ruling 2). Comment added at `:330-331`, and the call is now `:332` |
| `internal/orchestrator/r23_never_judged_test.go:13` comment "a tick that folds the exit to submitted" | Left: a test file this lane may not edit. The comment is now out of date. Its helper `r23Submitted` (`:16-33`) posts a result before the exit `done`, so the fold makes J-1 `submitted` at the exit and the tick posts no control. The test still passes because it checks `submitted` after the tick. Listed under open items |
| `docs/job-contract.md:132` | Changed as asked: the fold acts when a result exists, and the tick when none does |
| `docs/lanes/P-brain-board-decide.md:18` (`sameRetry` via `legacy`) | Left: an old lane's record. `sameRetry` is unchanged |
| `docs/lanes/P-r23-judge-review.md:14` (`tick.go:329-330` done control) | Left: an old lane's record. It is still true for a done exit with no result |
| `docs/research/brain-board.md:83`, `:104` (`fold.go:199-209` assign must advance, or `sameRetry`) | Left: research doc. The attempt rule and `sameRetry` still hold. Only the fence compare inside that range is gone |
| `docs/research/brain-board.md:317`, `:319`, `:477` (`sameRetry` in the migration plan) | Left: research for B3. `sameRetry` is unchanged |
| `docs/research/dogfood-gaps.md:23` (`tick.go:330`) | Left: research snapshot. The done arm still exists |
| `docs/research/judge-land.md:37`, `:265`, `:285`, `:327` (launcher control "awaiting judgement") | Left: research for the judge lane. A done exit with a result no longer gets this control. That is the plan of `run-jobs-launcher.md:151`, which this lane carries out |
| `docs/research/land-job-type.md:24` (`tick.go:329-330`) | Left: research snapshot. It is still true for a done exit with no result |
| `docs/research/run-jobs-launcher.md:151`, `:310`, `:444` (done with a result folds Submitted, no control) | Matches. This lane carries it out |
| App: `internal/kernel/worker/judge.go:20-21`, `judge_notyet_test.go:11` (comments) | Left: app repo, outside this lane. Not checked here, app repo |
| App: `replay_first_task_realplugin_unix_test.go:580-581` (done with no result) | Kept by ruling 2. The tick still posts that control. Not run here (app repo) |

## Outside Touches

None. Only the listed files are changed: `internal/board/*`, `internal/contract/fold.go`, the `tick.go` comment, `docs/job-contract.md`, the three new `pl_bd` test files and this doc.

## Open items

- **`TestOrphanNoExit` is red** (the finding above). It needs a ruling.
- **An out-of-date test comment.** `internal/orchestrator/r23_never_judged_test.go:13` says the tick folds the exit to `submitted`. With a result, the fold now does it. The test passes, but the comment needs an edit this lane may not make.
- **The claim key also matches the post type.** The brief's key is (task, Job, attempt, plus firing). One attempt writes several post types under the same header (ack, checkpoint, result, worker-exit). Without the type, any earlier post of that attempt would make a claim fail, and the claim post would block the attempt's own result. `Claim(key, ack)` therefore means "no ack of this attempt yet". The key also rejects a claim post whose own header or firing does not match the key, because a later claim could never find such a post.
- **The claim lock is a file in `feed/.cache/`.** Only `Claim` takes it. A plain `Post` of the same type does not, so a writer that bypasses `Claim` can still add a second post. Callers that need create-only must use `Claim`.
- **The shared disk filled during the run.** `/` reached 0 bytes free from other work on the host. A git checkout failed for a moment, and the first fetch could not write its ref. Both were redone once space came back. The working tree and the commits are intact.
