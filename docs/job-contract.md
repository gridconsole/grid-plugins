# Planned Jobs in a plan

The checklist block in [Plan format](plan.md#plan-format) is the sole planned
Job source. A plan item can contain several Jobs under its `jobs` list. There
is no front-matter Job list and no heading-based item model. `internal/plan`
parses the block; `internal/job` decodes the planned fields; `internal/jobgraph`
uses `job.Job` for dependencies and the plan check.

## Planned fields

Every entry supplies `id`, `item`, `title`, `role`, `skills`, `risk`, `intent`,
`links`, `keep`, `decisions`, `read`, `write`, `out_of_scope`, `acceptance`,
`verify`, `budget`, `on_blocked`, and `after`.
These are `job.PlannedFields`; empty lists remain explicit lists. A plan names
no review level and no round count: whether a review runs is judged per step by
the orchestrator. The record the run keeps does carry `review_rounds` (see
[Job record fields](#job-record-fields)); the plan never sets it. A plan or
record that still carries `review` or `max_rounds` reads without error and the
value is ignored.
The parser derives omitted `item`, `title`, `intent.item`, and `intent.goal`
from the enclosing checklist item. `intent.owner` is required. The step's
`kind` applies to its Jobs and is not repeated inside a Job entry.

Optional Job fields are `visible` (`big` or `small`), `irreversible` (boolean),
`review_reason` (text) and `repo` (the path of one of the project's repos in
project.md; a Job without it works in the first repo). Run fields such as `status`, `attempt`, and
`worktree` do not belong in the plan and are refused there. Rendered brief
metadata retains the step kind and the supplied optional fields for later readers.
A code Job names its repo in the Job record as `"repo": "<path>"`, for example
`{"id": "J1", "title": "Fix the parser", "repo": "plugin"}`.

Job-record contract version 1 also accepts these optional fields: `kind` matches
`^[a-z][a-z-]*$` (1–40 characters), `visible` is `big` or `small`, `irreversible`
is boolean, `review_reason` is a nonempty sentence (at most 500 characters), and
`repo` is a nonempty repo path.
Adding optional fields does not change the contract version.

`grid job render` selects a Job by its id or an item number. An item number is
ambiguous when it has more than one Job; select the Job id in that case.
`plan.md#step-N` quotes the checklist line and prose only. `plan.md#verify`
quotes the first paragraph under `## Verify`.

A `read` path renders as `- Read <path> · <summary> · <why> [tier]`, where `<why>` is the reason of a `links` entry with the same `to` (left out when there is none); a guest or external file shows its summary inside the data fence instead of on the line, a personal note shows the id only, and a file with no summary keeps the bare `- Read <path> [tier]`.

## Job record fields

`contract/schemas/job-record.json` is the shape of a Job record on the Feed. It
has 37 properties; 27 are required. The table has one row per property.
`Written by` is `plan` (the plan is published as Job records: decoded in
`internal/job/planned.go`, encoded by `Record` in `internal/job/record.go`,
posted by `internal/orchestrator/plan.go`), `run` (a later post or control
changes the record, folded in `internal/contract/fold.go`), or `none` (no code
writes it). `grid job render` writes no record field; it prints the record in
the brief metadata (`Metadata` in `internal/job/source.go`) and reads
`continues` (`internal/job/render.go`). A Job record with a field outside this
table is refused. `internal/job/hf_record_doc_test.go` keeps the table equal to
the schema.

| Field | Required | Written by | Meaning |
| --- | --- | --- | --- |
| `id` | required | plan `internal/job/planned.go` | The Job id, `J-N` or `J-N.M`; a proposed child takes `<parent>.<n>` (`internal/orchestrator/events.go`). |
| `item` | required | plan `internal/job/planned.go` | The number of the plan checklist item the Job belongs to. |
| `title` | required | plan `internal/job/planned.go` | One-line title; omitted, it comes from the checklist item. |
| `role` | required | plan `internal/job/planned.go` | The worker role; `Person` marks a step a person answers (`internal/job/job.go`). |
| `skills` | required | plan `internal/job/planned.go` | Skill names the worker loads; may be empty. |
| `risk` | required | plan `internal/job/planned.go` | Risk tags: `concurrency`, `security`, `data-format`, `cross-platform`. |
| `intent` | required | plan `internal/job/planned.go` | `owner`, `item` and `goal`: where the owner's request is quoted. |
| `links` | required | plan `internal/job/planned.go` | Typed links to other files or Jobs, each with a `rel` and a `why`. |
| `keep` | required | plan `internal/job/planned.go` | `brain:` ids of knowledge the Job must keep. |
| `decisions` | required | plan `internal/job/planned.go` | `brain:` ids of decisions that bind the Job. |
| `read` | required | plan `internal/job/planned.go` | Paths the brief shows the worker (`internal/job/render.go`). |
| `write` | required | plan `internal/job/planned.go` | Relative slash paths the Job may change. |
| `out_of_scope` | required | plan `internal/job/planned.go` | What the Job must not do. |
| `acceptance` | required | plan `internal/job/planned.go` | The acceptance rows (`A1`...), each with text and what it `traces` to; at least one. |
| `verify` | required | plan `internal/job/planned.go` | The check: a command with `min_tests`, or `kind` `file-exists` or `reviewer-check`. |
| `budget` | required | plan `internal/job/planned.go` | `turns_estimate` and `reads_before_first_edit`. |
| `on_blocked` | required | plan `internal/job/planned.go` | `ask-and-continue` or `exit`: what the worker does when blocked. |
| `after` | required | plan `internal/job/planned.go` | Dependencies: Job ids, `gate:Q-N` person questions, or `job`/`kind`/`why` objects. |
| `attempt` | required | plan `internal/job/job.go`; run `internal/contract/fold.go` | Starts at 1 (`Defaults`); an assignment post for a later attempt raises it. |
| `continues` | required | plan `internal/orchestrator/events.go`; run `internal/cli/reopen_job.go` | `null`, or the Job this one carries on: a proposed child names its parent, a reopened plan Job names the earlier one. |
| `discovered_from` | required | plan `internal/orchestrator/events.go`, `internal/orchestrator/replan.go` | `null`, or the Job whose result proposed this one, or the cancelled Job a replan replaces. |
| `close_reason` | required | run `internal/contract/fold.go` | `null` until a Job is `rejected` or `cancelled`; those two need a nonempty reason. |
| `checkpoint` | required | run `internal/contract/fold.go` | `null`, or the latest checkpoint post: `job`, `step` and a `commit` hash. |
| `review_rounds` | required | run `internal/contract/fold.go` | Counter, starts at 0; the fold adds 1 when a worker exits with reason `loop`. |
| `level_raised` | required | plan `internal/job/record.go` | `null`, or `from`, `to` and `why`; every writer in the module writes `null`. |
| `worktree` | required | plan `internal/job/record.go`; run `internal/question/job.go` | `null`, a string or a list of strings; a person's question Job writes `none`. |
| `posts_back` | required | plan `internal/job/record.go` | What the worker may post: `result`, `question`, `knowledge`, `checkpoint`; `[]` when unset. |
| `kind` | optional | plan `internal/job/planned.go` | The step's work kind (`^[a-z][a-z-]*$`), taken from the checklist item. |
| `visible` | optional | plan `internal/job/planned.go` | `big` or `small`. |
| `repo` | optional | plan `internal/job/planned.go` | The project repo a code Job works in, by its path in project.md; `.` names the project folder and always passes, whether or not project.md lists it; lint refuses any other path the project does not list (rule `job.repo`); none is the first repo. `grid worker next` puts that repo's folder on the spawn line as `repo`. |
| `target` | optional | plan `internal/job/job.go` | For `kind: land`: the branch the accepted change lands on, for example `main`; lint refuses a land Job without it (rule `job.land-target`). Other kinds leave it out. |
| `branch` | optional | plan `internal/job/job.go` | For `kind: land`: the accepted change's branch; lint refuses a land Job without it (rule `job.land-branch`). Other kinds leave it out. |
| `sha` | optional | plan `internal/job/job.go` | For `kind: land`: the commit of the accepted change, pinned at accept; lint refuses a land Job without it (rule `job.land-sha`). Other kinds leave it out. |
| `irreversible` | optional | plan `internal/job/planned.go` | Boolean; the Job does something that cannot be undone. |
| `review_reason` | optional | plan `internal/job/planned.go` | A nonempty sentence, at most 500 characters, saying why a review is wanted. |
| `order` | optional | run `internal/cli/orchestrator.go`, `internal/contract/fold.go` | Queue position, lowest first; changes only a Job that has not started. |
| `review` | optional | plan `internal/job/planned.go`, run `internal/migrate/receipt.go` | Retired review level `L0` to `L3`; an old plan still carries it and the migrate receipt still writes it; the other builders of a Job record no longer write it; nothing reads it. |
| `max_rounds` | optional | run `internal/migrate/receipt.go` | Retired round count; the migrate receipt still writes it; the other builders of a Job record no longer write it; nothing reads it. |
| `created_at` | optional | none | UTC time with milliseconds; no code writes it. The ready order reads it and falls back to the post time (`internal/orchestrator/ready.go`). |
| `status` | optional | run `internal/contract/fold.go` | The lifecycle status; `Record` drops the brief-only `briefed`, `done` and `blocked` (`internal/job/record.go`). |
| `review_override` | optional | none | `level` and `reason`; the schema allows it, no code writes or reads it. |

## Dependencies and review

`after` is a list of bare Job ids, person-question ids, or objects with `job`, `kind`, and
`why`. A bare Job id means `done`. An object uses `done`, `failed`, or
`children-done` and explains the link in 8 to 160 characters. A person
question id has the form `gate:Q-7` (the token is fixed) and no edge kind. Unknown targets, duplicate Job ids, and
cycles are errors; `children-done` also includes the target's children when
checking cycles. Whether the person has answered is supplied by the Feed, never by a checkbox.

A `done` edge (and a bare Job id) is met when the Job it names is `accepted`,
or is `submitted` and its latest result carries a passing verify: a `verified`
object whose `pass` is `true` or `"pass"`. A submit with no verify, a `false`
or `"unsure"` pass, and a Job that is running again do not meet it, so
dependent work starts in parallel without waiting for review. If that Job is
then `rejected`, a dependent that has not started goes back to waiting; one
that has started is not held or stopped, and a note naming the rejection is
posted to its Feed. `failed` and `children-done` edges are unchanged.

A `children-done` edge is met when the Job it names has at least one child
(`J-2.1` is a child of `J-2`), every child is `accepted`, and the Job itself
was not closed without its work. A review child is accepted whatever its
verdict, so the edge also reads the Job itself: a `cancelled` Job never meets
it, and a `rejected` Job meets it only through its rework: a Job that `continues` it and is not one of its
children, as the app's Judge posts when a review rejects. Every rework must be
`accepted` with its own children accepted, or be `rejected` and met through its
own rework in turn. A Job that is still open, such as a parked parent of a
split, needs only its children (`internal/jobgraph/graph.go`, `childrenDone`).

The review of a submitted Job and the rework after a rejection are posted by the app's Judge, not by the plugin: when a Job's agent exits `done`, the tick moves it to `submitted` ("awaiting judgement", `internal/orchestrator/tick.go`) and the worker stream prints a `wake` line (`internal/orchestrator/stream.go`), and nothing in the plugin reads a review's `ACCEPTED` or `REJECTED` verdict. Without the app's worker a done Job stays `submitted`. `grid job tree` names such a Job as an orphan of kind `never_judged` once its lease has run out and no wake of its result is acknowledged and no review or person Job stands from it (`internal/orchestrator/write.go`, `neverJudged`).

A Job declares no review level: `review` is a retired optional field that
nothing reads, and a Job that still carries one is not refused. The plan check
refuses an unknown `risk` value (`concurrency`, `security`, `data-format` or
`cross-platform`), an unknown work `kind`, a `visible` value other than `big` or
`small`, and a `write` pattern that is not a relative slash-separated path.

Code verification names a test command and a positive `min_tests`. For
`non-code`, `research`, or `scout` work, `verify` may instead name
`kind: file-exists` or `kind: reviewer-check`. A kind and a command cannot be
combined. These checks validate planned instructions, not execution evidence.

`grid plan check [T-NN]` validates the checklist through the shared parser.
`grid job cycles` also accepts a JSON array of `job.Job` objects for graph
inspection; it is not an alternative plan-file parser.
