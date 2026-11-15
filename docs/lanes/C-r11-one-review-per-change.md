# C-r11-one-review-per-change

A change is reviewed once. The orchestrator no longer plans a reviewer Job for a Job whose review the worker posts. A review asked for on top of that one is still possible. Eval rounds 10 and 11, P5.

Verdict: DONE (see the proof table; `make test` row filled from the final run).

## Cause

### 1. Which review is planned and which is automatic (inputs in /srv/gridwork/tmp/C-r11-one-review-per-change-inputs/)

E2, tally T-02 (`tally/tasks/T-02-.../`):
- J-2 is the builder (`jobs/J-2/job.yaml`, role builder). Feed `feed/worker@...2.jsonl:4`.
- J-3 is the planned review. The orchestrator J-1 posted it while it ran: feed line 7 at 05:35:05Z, before J-1's result at 05:35:30Z (`feed/worker@...3.jsonl:2`).
  - `jobs/J-3/job.yaml:5` has role reviewer.
  - `:17` links `job:J-2` as validates.
  - `:58` has `after` J-2.
  - `:65` has `discovered_from: null`.
- J-2.1 is the automatic review, posted after J-2's result: feed line 16 at 05:35:51Z.
  - `jobs/J-2.1/job.yaml:4` has the title "Review J-2: …".
  - `:5` has role reviewer.
  - `:32-33` has verify `reviewer-check` on itself.
  - `:43` has `discovered_from: "J-2"`.
- `worker.log:40` has J-2.1 `judgement_review_started`, and `:44` has `judgement_verdict_posted`.
- J-3 then ran a second time over the same change (`worker.log:45` started). Its verdict came back `judgement_verdict_late` (`:50`).
- The deliver Job J-4 waited on J-3 (feed line 9, `after` … `{"job":"J-3","why":"Delivery waits for the reviewer verdict on the Total fix."}`).

E3, tally-e3 T-03 (`tally-e3/tasks/T-03-.../`):
- J-2 is the builder (feed `worker@...2.jsonl:4`).
- J-3 is the planned review, posted by J-1 (feed line 7, 05:43:53Z).
  - `jobs/J-3/job.yaml:5` has role reviewer.
  - `:17` links `job:J-2`.
  - `:65` has `after` J-2.
  - `:72` has `discovered_from: null`.
- J-2.1 is the automatic review (feed line 16, 05:44:59Z).
  - `jobs/J-2.1/job.yaml:4` has the title "Review J-2: …".
  - `:40-41` has verify `reviewer-check`.
  - `:51` has `discovered_from: "J-2"`.
- `worker.log:66` has J-2.1 `judgement_review_started`, `:73` has the verdict posted, `:74` has J-3 started, and `:78` has J-3 `judgement_verdict_late`.

The feed `author` does not tell the two reviews apart: every job record in `worker@….2.jsonl` is stamped worker/cli. The difference is `discovered_from`, together with when each was posted and which `worker.log` events it has. r11-REPORT.md:33 counts about 0.4 USD per task for the second review.

### 2. Where each review comes from

Plugin, where the orchestrator decides to post a review Job (before this lane):
- `agents/orchestrator.md:18` said "Its `task.steps` are the steps to post Jobs for". Only a `posted_by: deliver` step was left out. The built-in flow lists `Check` (grid-reviewer) right after `Run` with no `posted_by` (`internal/cli/nh_workflow_show_task.go:203-209`), so the orchestrator posted a Job for it.
- `agents/orchestrator.md:23` said "review, after a build, with the build Job in its `after`".
- `agents/orchestrator.md:62` said "a one-line fix is one build and one review".
- `skills/plan-jobs/SKILL.md:43` said "A review Job names the build Job it checks".
- No lint looked at review Jobs (`internal/job/lint/lint.go`, before this lane).

App, where the automatic review comes from. I read its contract only and changed nothing; the source is `/srv/gridwork/ref/app.git` main, `internal/kernel/worker/judge.go`:
- `hand` (`judge.go:658-735`) runs when a Job posts its result. When the workflow step right after that Job's run step (`jFlow.after`, `:621-633`) is kind `check`, it posts the review Job `<id>.N`. That Job has title "Review <id>: <title>", role reviewer (checker for grid-checker), `discovered_from` = the Job, `after: []`, and verify `reviewer-check` on itself (`:713-731`). It logs `judgement_review_started` (`:733`).
- `reviewsOf` (`:451-471`) leaves out a planned review that has not started and waits on the Job. So the planned J-3 neither stops the automatic review nor counts as it, and both run.
- When the next step is `run`, `schedule` or none, the Job is accepted with no review (`:680-696`). When it is a person step, the person point is taken instead (`:697-700`).

## The change

`agents/orchestrator.md`:
- `:18`: "A `check` step right after a `run` step gets no Job from you either: when a Job of the run step posts its result, the worker posts that Job's review itself (`J-2.1` reviews `J-2`), so a review you plan beside it reviews the change twice."
- `:23`: the review kind is now "only one asked for on top of the review the worker posts (a second check step, or the person's words ask for it), with the Job it checks in its `after` and who asked in `review_reason`; a Job whose run step is followed by a `check` step gets no planned review, and `grid job lint` warns `job.review-twice` on one".
- `:27`: the deliver Job's `after` names a Job the worker reviews with kind `children-done`. A `done` edge is met when the Job is submitted with a passing verify (`docs/job-contract.md:109-113`), before the review. `children-done` waits until the `.N` children are accepted (`internal/jobgraph/graph.go:295-303`), and the worker's review is one of those children.
- `:29`: adds an example unposted-kind line, "no review Job: the worker posts J-2.1 for J-2".
- `:62`: now reads "a one-line fix is one build Job, and the worker posts its review".

`skills/plan-jobs/SKILL.md`:
- `:11`: the same rule on the workflow line, next to the `posted_by: deliver` sentence.
- `:43`: a review Job you post is one asked for on top, and it carries `review_reason`.
- `:46`: the deliver Job uses `children-done` for a reviewed Job.

`internal/job/lint/lint.go:262,350-367`: new warning `job.review-twice`. It fires on a Job that has:
- role reviewer or checker;
- an `after` that names a Job;
- `discovered_from` null;
- no `review_reason`.

It is a warning only. `grid job render` and `grid job lint` refuse only on errors (`internal/job/jobcmd/command.go:163`), so no Job is held. The worker's own review (`discovered_from` set, `after: []`) and a review asked for on top (`review_reason` set) are not flagged.

Tests:
- `internal/job/lint/r11_review_twice_test.go` covers three cases:
  - the E2 J-3 shape is flagged, for reviewer, grid-reviewer and checker, with a bare or object `after`;
  - the J-2.1 shape, a review with `review_reason`, a builder, and a reviewer on a gate are not flagged;
  - each case lints clean of errors.
- `internal/job/lint/r11_review_twice_text_test.go` pins both sentences on the workflow line. It fails if "one build and one review" or the old review bullet comes back, and it checks that removing the sentence makes it fail.

## Proof table

| Row | State | Proof |
|---|---|---|
| Brief 1: E2 and E3 planned versus automatic review, with file:line | HOLDS | "Cause" section 1 above |
| Brief 2: where the orchestrator posts a review, and where the automatic one comes from, with file:line | HOLDS | "Cause" section 2 above |
| Brief 3: the plugin plans no review for a Job the worker reviews; a review on top stays possible; change kept to the orchestrator text, the plan-jobs skill and the lint; a test where the lint changes | HOLDS | `agents/orchestrator.md:18,23`, `skills/plan-jobs/SKILL.md:11,43`, `internal/job/lint/lint.go:350-367`, `r11_review_twice_test.go` |
| Acceptance: a lint case flags a plan with a builder Job and a planned review of it | HOLDS | `go test -count=1 -run TestR11 -v ./internal/job/lint/`, exit 0 (PASS lines below) |
| Acceptance: the orchestrator text that forbids it, quoted | HOLDS | `agents/orchestrator.md:18`, quoted under "The change" |
| Acceptance: `make test` passes | see the run below | `make test` |
| Brief 4: lane doc with the cause, the change and the rows | HOLDS | this file |
| New tests fail on base | HOLDS | failing lines below |
| No existing test edited | HOLDS | `git diff origin/main --stat` only adds test files |
| No guard, gate, cap or hold on agents | HOLDS | the lint finding is a warning; refusal is on errors only (`jobcmd/command.go:163`) |
| App not changed | HOLDS | the diff is plugin-only; `judge.go` was read with `git show` from the ref repo |
| Tests of changed packages and their importers | HOLDS | `go list` importers: internal/cli, internal/job, internal/job/jobcmd (plus the root embed readers); all run by `make test` |
| `make lint` | HOLDS | exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./... && GOOS=darwin go build ./...` exit 0 |
| Walk tests (plugin) | HOLDS | listed below |
| Merge origin/main before the last proof | HOLDS | listed below |
| Whole-module test and cross-repo set | not run, not asked | the orchestrator runs them at landing (beyond `make test`) |
| *RealPlugin* tests | not run, not asked | the brief names none |

### New tests on base (lint rule before c0fb0901, text before 9fc9c717)

```
--- FAIL: TestR11PlannedReviewOfABuildIsFlagged (0.00s)
    r11_review_twice_test.go:42: role reviewer after ["J-2"]: want one job.review-twice finding, got []
--- FAIL: TestR11OrchestratorPlansNoReviewTheWorkerPosts (0.00s)
    r11_review_twice_text_test.go:42: ../../../agents/orchestrator.md: the text still plans a review per build: one build and one review
    r11_review_twice_text_test.go:42: ../../../skills/plan-jobs/SKILL.md: the workflow line does not hold: A `check` step right after a `run` step is not a Job you post either: ...
```

TestR11ReviewNotFlagged passes on base too. It checks for false positives and is not a red-to-green test.

## Open items

- `docs/job-contract.md:15-16` still says "whether a review runs is judged per step by the orchestrator". It is outside this brief's places. The judgement is now "only on top of the worker's review".
- A follow-up in `internal/cli` (outside this brief) could mark the check step in `grid workflow show --task --json`, as `posted_by: deliver` is marked, so the orchestrator reads the rule from data and not only from text.
- The deliver Job's `children-done` edge waits for every `.N` child of the build Job to be accepted. A child that is cancelled leaves delivery waiting. The tick's `dead_wait` line covers done edges only. NOT VERIFIED on a live round.

## NOT VERIFIED on a live run

That the orchestrator in an eval round posts no J-3 beside J-2.1, and that delivery then waits on J-2.1 through `children-done`. A checker sees this in the next eval round.
