# P-loop-board-2

Folds the new loop posts: landed, blocked, approval and allow (V22, V26, V27). Branch `lane-P-loop-board-2`, from 81ba523f, with `origin/main` (bb4df62f) merged in before the last proof run. The proof runs ran on 070cc8d1. The commits after it change only this doc.

**Result: DONE.** Every acceptance row holds at the fold and the graph, and the whole module is green. Row 2 holds at the fold only: the tick does not know Blocked yet (open item 1, `tick.go` is outside Touches).

## What changed

- `internal/contract/fold.go`
  - `Fold` is now `FoldLoop` (`:58`), with `Fold` a wrapper that returns its State, so every caller is unchanged.
  - `Loop` (`:52`) holds `Allow` (Job to the allowed rules) and `Landed` (Job to its landed post). Nothing in `types.go` changes: it is outside Touches.
  - `Blocked` (`:45`) is a new Status, declared in fold.go for the same reason.
  - `moved` (`:239`, `:291`) records a status control after a Job's latest assign.
  - `foldAttempts` (`:449`) folds `started`, `exit`, `blocked`, `waiting` and `cancel`. `foldGrants` and `approves` (`:528`, `:547`) fold `allow`. The landed loop is at `:298-310`.
  - The P-loop-board rules are unchanged: the older-attempt stale rule, the dropped fence compare, `sameRetry`, and the done-with-result Submitted in the tenure loop. `isJobSide` and `olderAttempt` are untouched.
- `internal/jobgraph/graph.go`
  - adds the `Landed = "landed"` kind (`:25`);
  - accepts it in `New` (`:76`);
  - adds `Snapshot.Landed` (`:190`);
  - in `Ready` (`:382-384`), the edge is met only when `Snapshot.Landed[target]` is true, otherwise it waits with "waiting for J to land".
- New tests, in new files only, with helpers prefixed `plBd2`:
  - `internal/contract/pl_bd2_loop_fold_test.go`
  - `internal/jobgraph/pl_bd2_landed_test.go`
- No existing test is edited. No file outside Touches changes except this doc: `git diff --stat origin/main HEAD` lists those four files and this doc.

### What the fold does per post type

The new launcher and agent posts carry no lease and fence 0, so no tenure consumes them. Each one matches on its envelope `job` and `attempt` only (RJ :165, :171).

**Common to `started`, `exit`, `blocked`, `waiting` and `cancel`**

| Case | Result |
|---|---|
| Attempt below the Job's | `stale` "older attempt" |
| Attempt the Job was not assigned | `ignored` "no counted assign for this attempt" |
| Unknown Job | `ignored` "unknown Job target" |
| Otherwise | `counted`, and appended to the Job's History |

The decision is taken per Job, after the controls loop. It therefore does not depend on the order the writers sort in: `cli@` sorts before `worker@`, so the launcher's exit is walked before the agent's blocked.

**Per type**

- `started`
  - Moves Assigned to Running.
- `exit`
  - The first exit of the attempt sets Exited, ExitReason and ExitCode, unless a worker-exit of the attempt already set them. A second exit is `ignored` "attempt already exited".
  - An exit counts with no `started` before it (pinned: "blocked and a failed exit, no started and no result").
- `blocked` plus an `exit` of the same attempt
  - Folds Blocked with reason "blocked on <rule>" (V26), in either order. This also holds over the fold's own Submitted from a worker-exit done with a result.
  - Otherwise, an exit `done` with a counted result of the attempt folds Submitted "awaiting judgement".
  - A status control after the assign (accept, park, and so on) stands, and none of these posts undo it.
- `blocked` with no exit, `waiting`, `cancel`
  - Count and move nothing.
  - A cancel is a request. The holder's leased control cancels, as with the unleased cancel control.
- `wake`
  - No change against base: it counts and moves nothing.
  - The RJ gives it no fold rule (RJ :320-321). The orchestrator acknowledges wake events through the `wake` field of its own posts (`events.go:233-238`), not through the fold.
- `landed`
  - A landed post whose envelope `job` names a known Job goes into `Loop.Landed`. If there are several, the lowest id is kept. A landed post for an unknown Job is `ignored` "unknown Job target".
  - There is no attempt rule (RJ :165 does not list landed). The match follows RJ :217: the kind is "true when a `landed` post exists for that Job".
- `approval` and the assign's `allow`
  - The `allow` of a Job's current assign is in `Loop.Allow` only when the assign's `grant` names an approval that meets all of these:
    - it is counted;
    - it is by a person (`TierFor(role) == PersonTeam`: owner, member or lead);
    - its decision is `approve`;
    - its envelope `job` is the Job;
    - its `rules` list every allow rule. The allow may be a subset of the approval's rules.
  - The approval's attempt is not compared: it names the blocked attempt, and the assign is the next one.
  - Anything else folds the allow absent, and the assign itself still counts and is granted. That covers an agent's or the CLI's approval, a decline, another rule, another Job, a grant naming a non-approval or a held approval, and no grant.
  - A grant note (P-loop-grants) is not read.

## Proof table

| Row | Result | Proof |
|---|---|---|
| Base check before the first edit: `git grep -n -e Submitted -e Blocked main -- internal/contract internal/jobgraph '*_test.go'` | HOLDS | 108 lines. The pathspecs are a union, so the grep covers every `_test.go`. No test names a `Blocked` Job status: main has none, and the `Blocked` hits are lock-test names and result text (`r23_result_shape_test.go:99`). The `Submitted` hits are existing scenarios with no blocked post. No existing test went red: see the whole-module row |
| P-loop-board fold rules re-read on main and not changed | HOLDS | `isJobSide`, `olderAttempt` with its stale relabel, `sameRetry` and the done-with-result Submitted in the tenure loop have no changed lines. The only lines changed inside `Fold`'s old body are the `moved` assignments at `:239` and `:291` and the new calls at `:296-310`. `TestPlBdOlderAttemptFoldsStale`, `TestPlBdNotOlderAttempt` and `TestPlBdDoneWithResultFoldsSubmitted` pass in the whole-module run |
| 1. (V27) (4) `after` kind `landed` is Ready only after a `landed` post | HOLDS | `go test -count=1 -v ./internal/jobgraph/ -run TestPlBd2`, exit 0. `TestPlBd2LandedEdgeWaitsForLanded` has 6 subtests: an accepted target with no landing waits with "waiting for J-1 to land"; a submitted target waits; another Job's landing does not count; accepted and landed is Ready; a done edge needs no landing; `Blocks` lists the kind. `TestPlBd2LandedPostMakesEdgeReady` folds a real landed post with `contract.FoldLoop` and feeds `Loop.Landed` to `Ready`. `TestPlBd2LandedFolds` (contract) covers the fold side and the unknown-Job case |
| 2. (6, V26) `blocked` then `exit` folds Blocked, not Submitted | HOLDS at the fold | `go test -count=1 -v ./internal/contract/ -run TestPlBd2`, exit 0. `TestPlBd2BlockedThenExitFoldsBlocked` has 10 subtests, each run in three input orders (as given, reversed, shuffled). Started, result, blocked, exit done gives Blocked "blocked on Bash(make deploy-prod)"; the same posts with no blocked give Submitted "awaiting judgement". It also covers a failed exit with no started, Blocked over a worker-exit done with a result, the blocked writer sorting before the launcher, blocked with no exit (Running), and a later accept standing. `TestPlBd2BlockedIsPerAttempt`: a blocked of attempt 1 folds stale "older attempt" after attempt 2's assign, the assign after Blocked is counted, and attempt 2 folds Submitted. The tick is open item 1 |
| 3. (7, V26, V22) `allow` with a person's `approval` folds; with an agent-authored approval and no grant, or an approval naming another rule or Job, it folds as absent | HOLDS | `TestPlBd2AllowNeedsPersonApproval` has 12 subtests in three orders. Present: owner, member, an approval listing more rules than the allow. Absent: agent-authored, cli-authored, another rule, an allow rule the approval lacks, another Job, decline, a grant naming the blocked post, a grant naming a held approval, no grant. In every case the approval is counted (held in its own case), the assign of attempt 2 is counted with the Job Running, and `FoldLoop`'s State equals `Fold`'s |
| New tests fail on base | HOLDS | With the production change stashed (`git stash push -- internal/contract/fold.go internal/jobgraph/graph.go`), `go test -count=1 ./internal/contract/ ./internal/jobgraph/ -run TestPlBd2` fails to build: `pl_bd2_landed_test.go:33:90: undefined: Landed`, `:37:85: unknown field Landed in struct literal of type Snapshot`, `:99:28: undefined: contract.FoldLoop`, `pl_bd2_loop_fold_test.go:55:6: undefined: Blocked`, `:87:9: undefined: isAttemptSide`, `:179:17: undefined: FoldLoop`, then `FAIL .../internal/contract [build failed]` and `FAIL .../internal/jobgraph [build failed]`. The "fails on base" is a compile failure. They pass on the lane (rows 1-3) |
| Standing: `go test -count=1 ./internal/contract/... ./internal/jobgraph/... ./internal/feed/... ./internal/orchestrator/...` exits 0 | HOLDS | Covered by the whole-module run on 070cc8d1 (next row), exit 0. Its lines for these packages: `ok internal/contract 2.579s`, `ok internal/feed 21.601s`, `ok internal/jobgraph 0.323s`, `ok internal/orchestrator 248.469s`. The command itself was not run apart, because the whole-module run is a superset |
| Standing: whole module `go test -count=1 ./...` | HOLDS | `go test -count=1 ./...`, not piped, on 070cc8d1 after the merge: exit 0, 76 `ok` lines, no FAIL line, no existing test red |
| Importers of the changed packages | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...`, filtered to lines that import the package. `internal/jobgraph`: `internal/cli`, `internal/orchestrator`, and `internal/jobgraph` itself. `internal/contract`: 33 lines, namely `contract`, `contract/formats`, `lib`, and under `internal/`: `artifact`, `board`, `brain`, `braincmd`, `cli`, `contract`, `feed`, `guard`, `hook`, `intent`, `job`, `job/jobcmd`, `job/lint`, `jobgraph`, `land`, `lease`, `mcp`, `migrate`, `orchestrator`, `ports`, `question`, `runner`, `session`, `settings`, `signal`, `task`, `updates`, `usage`, `where`, `workflow`. All of them are in the whole-module run |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok .../internal/cli 0.190s`, exit 0, on 070cc8d1 after the merge |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | `ok .../internal/job/lint 0.522s`, exit 0, on 070cc8d1 after the merge |
| `make lint` | HOLDS | `make -C . lint`: `go vet ./...` with no output, exit 0 |
| `make cross` | HOLDS | `make -C . cross`, not piped, on 070cc8d1: exit 0, no FAIL or panic line |
| Merge of origin/main before the last proof run | HOLDS | `git fetch origin` brought `81ba523f..bb4df62f main`. `git merge --no-edit origin/main` was a clean ort merge (P-land-kind: `types.go` comment, `lint.go`, `job.go`, schemas, tests; no overlap with this lane's files). The walk tests and all proof runs above ran after it |
| No attribution lines: `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` | HOLDS | prints `0` |
| Touches only | HOLDS | `git diff --stat origin/main HEAD`: `internal/contract/fold.go`, `internal/contract/pl_bd2_loop_fold_test.go`, `internal/jobgraph/graph.go`, `internal/jobgraph/pl_bd2_landed_test.go`, and this doc (which the runbook requires) |
| New tests in new files only, helpers prefixed `plBd2`, no existing test edited | HOLDS | Two new files. Their helpers are `plBd2Loop`, `plBd2Exit`, `plBd2Orders`, `plBd2Counted`, `plBd2Post`, `plBd2Graph` and `plBd2Snapshot`. They reuse the existing helpers (`base`, `side`, `retry`, `move`, `verdict`, `plBdShuffle`, `plBdSide`, `mustGraph`) without changing them |
| No guard, gate, cap, hold or blocking hook on agents; nothing removed; lease and fence kept | HOLDS | A non-matching `allow` folds absent and never refuses the assign: the assign after Blocked is counted in every row-3 case. Blocked refuses nothing: a new assign is granted (`TestPlBd2BlockedIsPerAttempt`). Only two lines are replaced: `return out` became `return out, loop`, and `New`'s `case "", Done, Failed, ChildrenDone:` gained `Landed`. `grid lease`, `--lease-epoch`, the assign `fence` and the envelope `lease`/`fence` are untouched |
| fold.go merge order: P-loop-board, P-loop-board-2, then P-board-store, then P-loop-grants | HOLDS | Built on P-loop-board (in main). The new code is in new functions below `Fold`'s body (`foldAttempts`, `foldGrants`, `approves`), plus three call lines, to keep the later merges small. No grant-note reader |
| Doc-parsing tests on the final lane doc | HOLDS | With this doc in the tree: `go test -count=1 ./internal/feed/ ./internal/golden/ ./internal/land/ ./internal/job/lint/ ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|.'` (the `.` runs every test), exit 0 (`ok` for all five; `internal/cli` 45.678s) |
| Live run, deploy, `claude`, `curl`, background server | not run, the brief forbids it | |
| App repo tests, `make webui-test`, the app walk tables | not run, not asked: plugin lane | |

## NOT VERIFIED

- Nothing here has a screen.
- No launcher writes `started`, `exit`, `blocked`, `approval` or `landed` yet, so these folds run only on fixture posts, never on a live Feed.

## Open items

1. **Tick (outside Touches).** `exit` now sets Exited and ExitReason.
   - For a Blocked Job whose exit says `done`, `tick.go:288` does not stop on Blocked, and `tick.go:329-332` posts a submitted control "awaiting judgement". `moved` then lets that control win, so in the live loop the Job reads Submitted.
   - Other exit reasons go into the tick's failure path.
   - `tick.go:288` needs Blocked in its stop list. Until then, row 2 holds at the fold only.
2. **Same-attempt retry.** The launcher's posts are folded after the controls loop, so `sameRetry` (`fold.go:242`) never sees a launcher `exit` with `usage_limit`. A usage-limit re-assign at the same attempt still needs the worker-exit. A second run of one attempt would also re-match the first run's exit, because the match is on job and attempt only.
3. **Controls and unleased posts have no order.** A status control after the assign (stale, parked) keeps Blocked from applying, even when the control came before the blocked and exit posts.
4. **`StateOf` (in `types.go`) reads `blocked` as "Not started".** There is no Blocked in the Status constants or in the job-record and control status enums. `jobgraph.Ready` does not offer a Blocked Job again, and `started()` does not count it.
5. **`internal/orchestrator/ready.go:145` builds `jobgraph.Snapshot` without `Landed`.** A landed edge therefore waits until ready.go calls `contract.FoldLoop` and fills it. Before this lane, `jobgraph.New` refused the kind.
6. **`internal/job/lint/lint.go:253` still refuses the after kind `landed`** ("Use done, failed or children-done").
7. **State does not carry `allow`.** `Loop.Allow` is returned only by `FoldLoop`. The launcher (`started`'s `allow_dropped`) must read it there.
8. **`contract/docs/feed-posts.md` is stale.** It still says "The fold has no case for it yet" for approval (`:80`), blocked (`:118`), cancel (`:156`), exit (`:306`), landed (`:660`), started (`:860`) and waiting (`:1075`). No test pins that text.
9. **Only the launcher's `exit` gets the Blocked rule.** A `blocked` with only a worker-exit of the attempt does not fold Blocked, so the P-loop-board rule is not changed.
10. **The landed post names its own Job in the envelope.** The match is RJ :217's reading: the edge target is that Job. Mapping a land Job's landed post to the build Jobs it landed is judge-land's, as are cross-task `after` targets (RJ :454). Neither is built here.

DONE
