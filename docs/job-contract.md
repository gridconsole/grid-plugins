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
These are `job.PlannedFields`; empty lists remain explicit lists. A Job names
no review level and no round count: whether a review runs is judged per step by
the orchestrator. A plan or record that still carries `review` or `max_rounds`
reads without error and the value is ignored.
The parser derives omitted `item`, `title`, `intent.item`, and `intent.goal`
from the enclosing checklist item. `intent.owner` is required. The step's
`kind` applies to its Jobs and is not repeated inside a Job entry.

Optional Job fields are `visible` (`big` or `small`), `irreversible` (boolean),
and `review_reason` (text). Run fields such as `status`, `attempt`, and
`worktree` do not belong in the plan and are refused there. Rendered brief
metadata retains the step kind and the supplied optional fields for later readers.

Job-record contract version 1 also accepts these optional fields: `kind` matches
`^[a-z][a-z-]*$` (1–40 characters), `visible` is `big` or `small`, `irreversible`
is boolean, and `review_reason` is a nonempty sentence (at most 500 characters).
Adding optional fields does not change the contract version.

`grid job render` selects a Job by its id or an item number. An item number is
ambiguous when it has more than one Job; select the Job id in that case.
`plan.md#step-N` quotes the checklist line and prose only. `plan.md#verify`
quotes the first paragraph under `## Verify`.

A `read` path renders as `- Read <path> · <summary> · <why> [tier]`, where `<why>` is the reason of a `links` entry with the same `to` (left out when there is none); a guest or external file shows its summary inside the data fence instead of on the line, a personal note shows the id only, and a file with no summary keeps the bare `- Read <path> [tier]`.

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
