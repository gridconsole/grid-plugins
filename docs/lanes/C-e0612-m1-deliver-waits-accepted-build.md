# C-e0612-m1-deliver-waits-accepted-build

A `children-done` edge no longer lets the deliver Job start on rejected work. The edge now waits while the build it names is rejected or cancelled. A rejected build meets the edge only through its rework: the Job the Judge posts in its place, once that Job is accepted with its review. End review 0612Z M1.

Verdict: DONE. `make test`, `make lint`, both OS builds and the walk tests ran on e4cb8535. The commit after it touches only `docs/lanes/`. `git fetch origin && git merge origin/main` printed "Already up to date".

## Cause

- C-r11-one-review-per-change has the orchestrator name a reviewed build Job with kind `children-done` (`agents/orchestrator.md:27`, `skills/plan-jobs/SKILL.md:46`).
- On base 8b8959ca, the ChildrenDone case in `internal/jobgraph/graph.go:295-303` read only the children's status. The target's own status was never read.
- When a review rejects, the app's Judge does three things in this order (app `internal/kernel/worker/judge.go`, read only from the app ref, never changed):
  1. It posts the rework as a new top-level Job, with `continues` set to the build and `discovered_from` set to the review.
  2. It closes the build `rejected` ("reworked as J-N").
  3. It closes the review child `accepted`. A review Job is accepted whatever its verdict.
- So J-2 was rejected, J-2.1 accepted, and the deliver Job J-3 `after` J-2 with `children-done` was ready on rejected work.
- The rework's review is numbered under the root of the chain (J-2.2 for the rework J-4).
- Neither `internal/cli/deliver.go` nor `internal/task/deliver.go` checks Job states, so nothing later stopped it.

## Choice: (a), widened to follow the rework

**(a) as worded ("also require the target accepted") breaks an existing test.** `internal/jobgraph/graph_test.go:207-233` `TestReadyChildrenAndGates` expects `children-done` met while the parent J-14 is `parked`, the parent of a split. Making that pass would mean editing an existing test, which the rules forbid.

**(a) as worded also breaks the rework path.** The Judge never accepts the rejected build: it posts a new Job (J-4) instead. "Target accepted" would leave the deliver Job waiting for ever, so brief row 3 could not pass.

**(b) fails row 3.** (b) is the text change: name the build with both `done` and `children-done`.
- A `done` edge on a rejected J-2 never releases, so the same deliver Job cannot become ready after the rework.
- `guReplanRejected` would post a re-plan Job beside the Judge's rework, because done-edge waiters on a rejected Job get one.

**Chosen rule** (`internal/jobgraph/graph.go:256` `childrenDone`, `:275` `reworkDone`, `:309` `childrenAccepted`, called at `:367`):
- The target has at least one child, and every child is `accepted` (unchanged).
- A target closed `cancelled` does not meet the edge.
- A target closed `rejected` meets the edge only through its reworks. A rework is a Job whose `continues` names the target and that is not one of the target's descendants. That excludes a proposed child J-2.N, which also `continues` its parent (`internal/orchestrator/events.go:165`).
- Every rework must be `accepted` with all its own children accepted, or be `rejected` and met in turn through its own reworks. A loop of `continues` ends: each Job is visited once.
- A target that is still open (posted to submitted, or parked) keeps the old rule, so the split case in `TestReadyChildrenAndGates` still holds.

**Why an open target needs nothing more.** Every Judge path closes the builder before its last child is accepted:
- Accept: the build is accepted, then the review.
- Rework: the rework is posted, the build is rejected, then the review is accepted.
- Person path: the person Job is posted under the root, and the plugin's `yvPamControl` (`person.go`) moves the builder before it accepts the person Job.

So an open build with all children accepted is the split parent, not a reviewed build.

**The new wait reasons are distinct text**, for example "waiting for J-4, which reworks J-2, to be accepted". `deadDoneWait` (`internal/orchestrator/write.go:282-291`) matches a wait by its reason text only, with no check of the edge kind. With distinct text, `grid job tree` prints no new `dead_wait` line.

**What was checked:**
- `grep -rn children-done` over the repo, excluding `.git`. The hits are:
  - `graph.go`, `graph_test.go`
  - `internal/job/lint/lint.go`
  - `internal/feed/contract_test.go:500` (a schema case)
  - `internal/orchestrator/ready.go:211` (ReadySince)
  - `contract/schemas/job-record.json`
  - the docs, the agent and the skill
- No `.jsonl` fixture uses `children-done`.
- The only test with a `children-done` edge over a non-accepted parent is `TestReadyChildrenAndGates` (parked). `graph_test.go:234-238` (unsplit parent, no children) still waits.
- No test pins the text "all children of"; it appears only in `graph.go`.
- `contract/README.md:370` and `docs/job-contract.md:116` ("`failed` and `children-done` edges are unchanged", about the done release) are still true.

## Failing lines on base

`go test -count=1 ./internal/jobgraph/ -run TestM1` at 06540774 (8b8959ca plus the new test file only; `graph.go` is unchanged there), exit 1:

```
--- FAIL: TestM1DeliverNotReadyOnRejectedBuild (0.00s)
    --- FAIL: TestM1DeliverNotReadyOnRejectedBuild/build_rejected,_review_accepted (0.00s)
        m1_children_done_rework_test.go:97: deliver J-3 is ready, want it waiting: waiting for a Job that reworks J-2, which was rejected
    --- FAIL: TestM1DeliverNotReadyOnRejectedBuild/build_cancelled,_review_accepted (0.00s)
        m1_children_done_rework_test.go:97: deliver J-3 is ready, want it waiting: waiting for J-2 to be accepted, but it was cancelled
--- FAIL: TestM1ReworkAcceptedReleasesDeliver (0.00s)
    --- FAIL: .../rework_posted ... deliver J-3 is ready, want it waiting: waiting for J-4, which reworks J-2, to be accepted
    --- FAIL: .../rework_running ... (same)
    --- FAIL: .../second_rework_posted ... waiting for J-5, which reworks J-4, to be accepted
    --- FAIL: .../rework_accepted,_the_person's_Job_open ... waiting for all children of J-4 to be accepted
--- FAIL: TestM1OnlyAReworkStandsInForTheBuild (0.00s)
    --- FAIL: .../a_proposed_child_is_not_a_rework ... waiting for a Job that reworks J-2, which was rejected
    --- FAIL: .../rework_sent_back_by_the_person ... waiting for a Job that reworks J-4, which was rejected
    --- FAIL: .../rework_cancelled ... waiting for J-4, which reworks J-2, to be accepted, but it was cancelled
    --- FAIL: .../one_of_two_reworks_accepted ... waiting for J-5, which reworks J-2, to be accepted
    --- FAIL: .../continues_loop ... waiting for a Job that reworks J-5, which was rejected
FAIL	github.com/gridconsole/grid-plugins/internal/jobgraph	0.007s
```

The Feed test runs against base `graph.go`, swapped in with `git checkout 8b8959ca -- internal/jobgraph/graph.go`, then restored with `git checkout HEAD -- internal/jobgraph/graph.go`. `git status --short` was clean afterwards. `go test -count=1 ./internal/orchestrator/ -run TestM1`, exit 1:

```
m1_deliver_rework_test.go:89: J-2 rejected, J-2.1 accepted: ready true, waits [], want [{Job:J-3 Target:J-2 Kind:children-done Reason:waiting for J-4, which reworks J-2, to be accepted}]
--- FAIL: TestM1DeliverWaitsForTheReworkOnTheFeed (0.05s)
```

On the lane, all four tests pass: `--- PASS` for each, `ok internal/jobgraph`, `ok internal/orchestrator`.

## Proof table

| Row | State | Proof |
| --- | --- | --- |
| Brief 1: failing unit test first (build rejected, review accepted, deliver `children-done` not ready), shown failing on main | HOLDS | Test commit 06540774 comes before the fix 02e9e238. `internal/jobgraph/m1_children_done_rework_test.go:109` TestM1DeliverNotReadyOnRejectedBuild. The failing lines above, exit 1 at 06540774. |
| Brief 2: fix with (a) or (b), say why, list what was checked | HOLDS | (a), widened to follow the rework: `internal/jobgraph/graph.go:247-316`. The why and the list are in "Choice" above. |
| Brief 3: rework accepted with its new review accepted makes deliver ready; tested | HOLDS | `m1_children_done_rework_test.go:120` TestM1ReworkAcceptedReleasesDeliver (also a second round and a person child). `internal/orchestrator/m1_deliver_rework_test.go:75` TestM1DeliverWaitsForTheReworkOnTheFeed walks the Judge's post order through Load and Ready. `go test -count=1 ./internal/jobgraph/ ./internal/orchestrator/` exit 0. |
| Brief 4: `agents/orchestrator.md:27` and `skills/plan-jobs/SKILL.md:46` match; one-review lint warning kept | HOLDS | Both sentences now say delivery waits for the Job and its review, or the rework and the rework's review (e4cb8535). Every phrase `internal/cli/er_deliver_job_text_test.go` pins is kept, and none of approve, gate or "ask the person" is used. `job.review-twice` is unchanged at `internal/job/lint/lint.go:364`, `SKILL.md:11` and `orchestrator.md:23`. `go test -count=1 ./internal/job/lint/ ./internal/cli/ ./internal/mcp/ ./internal/job/ ./internal/plan/` exit 0. |
| Brief 5: lane doc with the cause, the choice and the rows | HOLDS | This file. |
| Acceptance: the new test fails on 8b8959ca and passes on the lane | HOLDS | Base: exit 1, lines above. Lane: `go test -count=1 ./internal/jobgraph/ ./internal/orchestrator/` exit 0. |
| Acceptance: the rework test passes | HOLDS | Same run, exit 0 (TestM1ReworkAcceptedReleasesDeliver, TestM1OnlyAReworkStandsInForTheBuild, TestM1DeliverWaitsForTheReworkOnTheFeed). |
| Acceptance: `make test` passes | HOLDS | `make test` on e4cb8535, exit 0. Every package `ok`; "temp leak check: no new matching entries". |
| Acceptance: `make lint` passes | HOLDS | `make lint` on e4cb8535, exit 0 (`go vet ./...`, gofmt clean). |
| Acceptance: the diff | HOLDS | `git diff --stat origin/main..HEAD`, before this doc: 6 files, 341 insertions, 10 deletions. See "What changed". |
| Rule: tests of the changed packages and of every package that imports them | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep internal/jobgraph` gives cmd/grid, internal/cli, internal/doctor, internal/jobgraph, internal/mcp, internal/migrate, internal/orchestrator, lib. All ran in `make test`, exit 0. The text files are read by internal/cli, internal/job, internal/job/lint, internal/mcp and internal/plan, which ran in the same run. |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0. `GOOS=darwin go build ./...` exit 0. |
| Rule: walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0, with `--- PASS` for both under `-v`. `go test -count=1 ./internal/job/lint/` exit 0. |
| Rule: fetch and merge main before the last proof run, then rerun the walk tests | HOLDS | `git fetch origin && git merge origin/main` printed "Already up to date". The walk tests and `make test` ran after it. |
| Rule: new tests fail on base and pass on the lane; no existing test edited | HOLDS | `git diff --stat origin/main..HEAD` lists only two new `_test.go` files: `m1_children_done_rework_test.go` and `m1_deliver_rework_test.go`. No test was deleted. |
| Rule: mismatch-proof edit of a production file only for its row, status clean before committing | HOLDS | Base `graph.go` was swapped in only for the Feed test's base run, then restored. `git status --short` was empty before each commit. |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log --format='%h %s%n%b' origin/main..HEAD`: 4 commits plus this doc, one sentence each, empty bodies. |
| Rule: public repo, no personal paths, hosts or emails | HOLDS | `grep -rn "/Users\|/srv/\|@gmail"` over the six changed files, exit 1 (no match). The review is cited as "end review 0612Z M1". |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | The change is the readiness rule of an existing edge kind in `graph.go`. No hook, cap or limit. A Job that has started is never stopped, and the texts name no new step for agents. |
| Rule: do not change the app | HOLDS | The app was read only, through `git show` on the app ref; nothing in it was written. |
| Rule: never push, curl, sudo | HOLDS | None was run. The bundle is the hand-off. |
| Runbook: `/srv/gridwork/tmp/hand/runbook-reviewer.txt` read and the lane checked against it | HOLDS | Every brief row has a row with a command and exit code or a file:line, and the walk tests are pasted. The brief has no Touches list. One changed file, `docs/job-contract.md`, is not named in the brief: it gets a paragraph saying when `children-done` is met, because that file is where the repo defines the edge kinds. It is a doc only and changes no behaviour. |
| `make ops-check` | not run, not asked | No route changed. |

## What changed

- `internal/jobgraph/graph.go`:
  - The ChildrenDone case calls `childrenDone`.
  - `childrenDone` reads the target's status: a cancelled target waits, and a rejected one is met through `reworkDone`.
  - `reworkDone` follows `continues` to the reworks, excluding descendants, and recurses on a rejected rework.
  - `childrenAccepted` holds the old children loop.
- `internal/jobgraph/m1_children_done_rework_test.go` (new): three table tests, 19 cases, on the graph.
- `internal/orchestrator/m1_deliver_rework_test.go` (new): the Judge's post order as Feed records, read through Load and Ready.
- `agents/orchestrator.md:27` and `skills/plan-jobs/SKILL.md:46`: the deliver sentence says delivery waits for the Job and its review, or, when the review sends the Job back, for the rework and the rework's review. The deliver Job itself does not change.
- `docs/job-contract.md:118-126`: when a `children-done` edge is met.

## Open items

- `Cycle` does not follow reworks. A rework that names the deliver Job in its own `after` would not be found as a loop; it would wait as any blocked Job does. What the Judge puts in a rework's `after` was not checked.
- `internal/orchestrator/ready.go:211` ReadySince for `children-done` takes only the settle times of the `target.` children. It does not count the rework or the rework's own children, so the "ready since" time of a deliver Job after a rework can be early. This is ordering metadata only; readiness is right.
- `dead_wait` still covers done edges only. A `children-done` wait on a rejected build that never gets a rework, or on a cancelled build, is not flagged in `grid job tree`. The orchestrator text for `dead_wait` was left as it is.
- Done edges do not follow reworks. A Job `after` the build with a done edge still gets a re-plan Job from `guReplanRejected` even when the Judge reworks.
- A `children-done` edge named directly on a rework Job (J-4) would wait for ever: its review is numbered J-2.N under the root, not J-4.N. Planned Jobs can only name Jobs that exist when they are posted, so the orchestrator text does not lead there.

## NOT VERIFIED

- A live round, where a review really rejects, the app's Judge posts the rework, and the deliver Job starts only after the rework's review. The Judge's post order is taken from reading app `internal/kernel/worker/judge.go` and replayed as Feed records in `TestM1DeliverWaitsForTheReworkOnTheFeed`, not run end to end.
- The person path through the app (rounds used up, person Job under the root) is covered by graph cases only (`m1_children_done_rework_test.go`, "rework sent back by the person"), not by Feed records.

DONE
