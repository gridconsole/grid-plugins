# D-job-hold: a Job that waits on something else holds; it is not rejected and gets no rework

Base: plugin `3a3fd239`, app `2301df188`. Plugin paths are repo-relative; app paths are prefixed `app:`.

Decision only. It changes no code, schema or skill; steps 2 to 8 of T-14 build from it.

## Owner rule (cited, not reopened)

From the task input `inputs/jobs-fail-less.md:16`:

> A Job that cannot finish because it is waiting on something else (another Job, a merge or install, a person's answer or decision, a permission) is not rejected and gets no rework. It holds: it stays open in a waiting state that names what it waits on, and continues or is re-checked by itself once that thing lands. Only a real defect in the work leads to rejection and rework.

And the standing owner decisions: no Grid-made guards, caps or limits; everything is a Job; carry on by yourself until a person point. So a hold has **no timer, no expiry and no cap** on how often a Job holds or is released. None of the choices below adds one.

## Today

- A result row is `met`, `not met` or `dropped` (`contract/schemas/result.json:61-66`; fold words `internal/contract/types.go:200-203`). There is no way to say "this row cannot hold yet because X has not landed".
- The reviewer rejects on any `NOT DONE` row (`skills/review-job/SKILL.md:12`), first line `ACCEPTED` or `REJECTED` (`:35`).
- The app Judge reads only those two words (`app:internal/kernel/verdict/verdict.go:21-26`). Any other first word parses to `""` and fails as `verdict_unreadable` (`app:internal/kernel/worker/judge.go:1079-1084`); `REJECTED` goes to rework while rounds remain, then to a "Decide how" person Job (`judge.go:1123-1136`).
- `Blocked` (`internal/contract/fold.go:53-55`) is an attempt that posted `blocked` and exited; the fold makes it `Blocked`, not `Submitted`, even with a result (`fold.go:740-743`), so it never reaches a review. It waits for a person's approval and a new assign (V26).
- `Parked` (`internal/contract/types.go:97`) is an open Job a person or its children must move on: a split parent, or a Job parked after repeated failures with a question file.

## Decision 1: the waiting row state and what it waits on (A1)

A result acceptance row gets a fourth state, **`waiting`**, distinct from `not met`. `not met` means the work is short; `waiting` means the work cannot hold until something outside the Job lands.

A `waiting` row must carry **`waits_on`**, one object:

```json
{"id": "A3", "state": "waiting", "waits_on": {"kind": "landed", "ref": "J-6"}}
```

| `kind` | Waits on | `ref` |
| --- | --- | --- |
| `job` | another Job's change being accepted | the Job id, `J-n` or `J-n.m` |
| `landed` | another Job's change merged and installed | the Job id a `landed` post names in its envelope, `J-n` (`fold.go:519-531` keys `Loop.Landed` by it). The landed post covers merge and install; there is no separate install record |
| `question` | a person's answer or decision | the question's gate id, `Q-n`, as the `after` edge `gate:Q-n` names it (`internal/jobgraph/graph.go` `gateID`) |
| `permission` | a permission rule the Job lacks | the id of the `blocked` post that names the rule (`contract/schemas/blocked.json`) |

Defaults:

- One `waits_on` per row. A row that needs two things names the one that is not there yet; after release, the re-check may hold the row again on the second. No cap applies.
- In the builder's proof table the row's line reads `WAITING on <kind> <ref>`, next to `HOLDS` and `NOT DONE`.
- A `permission` wait keeps today's path: the builder posts `blocked` naming the rule, then its result with the row `waiting` on that post, and exits. The fold makes that `Blocked` (`fold.go:740-743`), so it never reaches a review or a verdict; it is already a hold with no rework. The `waiting` row is what lets the feed state and the task page say what it waits on. The other three kinds go through review to `HELD` (Decision 2).
- The fold reads `waiting` as its own row word (`RowWaiting`), never as `not met` or `pending`.

## Decision 2: the reviewer verdict word (A2)

The reviewer's first line gets a third word: **`HELD`**, next to `ACCEPTED` and `REJECTED`.

The reviewer writes `HELD` when every row of the Job is `HOLDS` (met), `dropped` with its decision, or `WAITING` with a `waits_on` that names an existing record of its kind, at least one row is `WAITING`, and nothing is must-fix. The second line lists the waits: `Waits: A3 on landed J-6`.

A real defect wins: any `NOT DONE` row, a change outside the write list, a failed verify or a `WAITING` row whose wait is not why the row cannot hold (the ref does not exist, or the row does not depend on it) makes the verdict `REJECTED`, as today. That last case is written as must-fix on the row.

`HELD` **never leads to a rework Job and never to a "Decide how J-x continues" person Job.** The Judge's `HELD` branch posts only the hold (Decision 4) and closes the review Job; a `HELD` review is not counted in the Job's review rounds (`roundsOf`), so a hold never pushes a later rejection towards the person Job either.

## Decision 3: how each wait is released (A3)

One component sees every release: **the plugin orchestrator**. It decides in `Ready` (`internal/orchestrator/ready.go:136-140`), "the only Orchestrator ready predicate", and writes in its tick (`internal/orchestrator/tick.go`), whose `assign` already posts the `ready` control before a new assign (`tick.go:256-259`). Its `Snapshot` (`ready.go:28-43`) already holds Job statuses, `Landed` and the questions behind `gate:Q-n`; the counted approvals the fold reads in `foldGrants` (`fold.go:752-771`) are not in it yet, and the permission step adds them. The app Judge only posts the hold; it does not release it, so there is one place that decides and the app and plugin cannot disagree.

| Kind | Record that releases it | Seen by |
| --- | --- | --- |
| `job` | a control post moving `ref` to `accepted` (the same rule as a `done` edge) | plugin orchestrator, `Ready` |
| `landed` | a counted `landed` post for `ref` (`Snapshot.Landed`), per `internal/jobgraph/graph.go:23-25`: an accept is not a landing | plugin orchestrator, `Ready` |
| `question` | the question answered or its person default applied: `gate:Q-n` reads open in `Ready`'s gates (`ready.go:178-184`) | plugin orchestrator, `Ready` |
| `permission` | a counted approval by a person that answers the `blocked` post `ref` and names its rules (`contract/schemas/approval.json`, V22) | plugin orchestrator, `Ready` and tick, with the approvals added to `Snapshot` |

When every wait of a held Job is met, `Ready` lists it as ready and the tick's `assign` posts the control moving it from `held` to `ready`, reason `hold released`, then the new assign. A `Blocked` permission wait is released the same way once its approval is counted: the tick assigns its next attempt with the approved rules, so no person makes the new assign. From there it runs as any ready Job: a new attempt of the **same Job id** (no new Job, no `continues`), whose builder finishes or re-checks only the rows that waited and posts a result, and a review follows as usual. **No person takes part in the release or the re-check.** The only person in any of the four kinds is the one who answers the question or grants the permission, which is the thing being waited on, not a decision about the hold.

A `job` or `landed` wait follows its target through reworks (a Job such as `J-9` with `continues: J-6`; a child such as `J-6.1` is not a rework) and is judged on the final state of that chain only. A `landed` wait is met by a counted `landed` post for any Job of the chain, whatever that Job's status, so a target that landed and was then cancelled releases it. Once the target and every Job that continues it were rejected or cancelled, by a worker or by a control, and for a `landed` wait none of them landed, the wait can no longer be met: the hold stays, `grid job blocked` shows the wait ("waiting for J-6, which was cancelled" for a `job` wait, "waiting for J-6 to land" for a `landed` wait), `grid job tree` shows the Job as `dead_wait`, and the tick posts one orchestrator re-plan Job for that held Job, wait and last Job of the chain, the newest by Job number, titled "Re-plan J-1: its wait on J-6 can no longer be met (dead at J-9)" (`internal/orchestrator/replan_held.go`). The title is the record: a chain that stays dead gets one re-plan however many rows name it or ticks pass, and a chain that a later Job continues and that dies again at a new last Job gets one more. Two more waits can never be met and take the same path, with `dead_wait` in the tree: a `job` wait on a Job that is not on the Feed ("(J-99 is not on the Feed)"; a new Job never takes that id), and a `job` or `landed` wait in a wait cycle, where the Job waited on waits on the held Job in turn through holds and the after edges of Posted Jobs ("(in the wait cycle J-1, J-2)"), which gets one re-plan for the whole cycle. A `question` wait whose question is withdrawn, answered or has its own Job cancelled is met, not dead: the question closes, its gate reads open and the hold is released. When that re-plan Job ends cancelled or rejected while the Job is still held on the same dead wait, the task's owner gets one person Job to decide, as for any dead re-plan; when it ends accepted, the owner is asked from the tick after the one that first saw it accepted, so the re-plan's own posts can end the hold first. A Posted or Ready Job behind a Job cancelled by a control alone still gets nothing (`gkReplan`), and a dead Job with both a Posted and a held waiter gets two re-plans, one from each rule. No timer turns a hold into anything else.

## Decision 4: how a held Job is stored and read (A4)

**A new Job status, `held`, for waits on a Job, a landing or a question.** A permission wait stays the existing `Blocked`, gaining only its `waits_on`.

Reason in one sentence: `Blocked` is the fold's own outcome of a `blocked` post plus an exit and so fits a permission wait, but it is made before any review and cannot carry a reviewer's `HELD`, and `Parked` is the status a person or its children must move on, so a hold the reviewer confirms needs its own status that reads as no person point.

Stored:

- The app Judge, on a `HELD` verdict, posts one control for the reviewed Job: `{"status": "held", "reason": "held by <review Job>", "waits_on": [{"row": "A3", "kind": "landed", "ref": "J-6"}]}`, copied from the `WAITING` rows of the review's result. Then it closes the review Job, `verdict held`.
- `held` is added to the control and job-record status enums (`contract/schemas/control.json:219`, `contract/schemas/job-record.json:549`) and to `contract.Status` (`internal/contract/types.go:89-100`).
- The fold copies `waits_on` from that control onto the Job as `WaitsOn []Wait` (`row`, `kind`, `ref`), and clears it when a later control moves the Job out of `held`. For a `Blocked` Job it copies `waits_on` from the `waiting` rows of the blocked attempt's result.
- `StateOf(held)` reads **Started** with the reason `held`, as `Blocked` reads Started with `blocked` (`types.go:177-187`). The Job is open and in progress; it is not Needs review and not Done.

Read:

- `grid feed state` shows the Job with `"status": "held"`, `"state": "Started"`, `"state_reason": "held"` and `"waits_on": [{"row", "kind", "ref", "met"}]`, `met` filled by the same `Ready` pass that releases it.
- `grid job blocked` lists a held Job with its waits as the why (`internal/cli/orchestrator.go:261-275`).
- The app task page reads `grid feed state` through `ViewFeedJob` (`app:internal/slices/taskview/view.go:437-460`); it gains `WaitsOn` and shows the Job as "Held: waits on landed J-6 (A3)", one line per wait. It is not counted in Needs you, because no person decision is open on it.

## What the following steps build

| Step | Builds |
| --- | --- |
| Contract | `waiting` row state and `waits_on` in `result.json`; `held` in control and job-record; `RowWaiting`, `Held`, `WaitsOn` in `internal/contract` |
| Skills | `run-job`: the `WAITING on` line; `review-job`: the `HELD` verdict and its rule |
| Plugin orchestrator | `Ready` releases held Jobs; the release control; `grid job blocked` and `feed state` show waits |
| App Judge | `HELD` in `internal/kernel/verdict`; the Judge's hold branch, no rework, no person Job, not counted in rounds; a test where a waiting row gives a hold and no rework Job |
| App task page | `WaitsOn` on `ViewFeedJob` and the Held line |

Nothing in this decision is left for a person: each choice above has its default, and the owner rule fixes the rest.
