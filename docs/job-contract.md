# Planned Jobs in a plan

The checklist block in [Plan format](plan.md#plan-format) is the sole planned
Job source. A plan item can contain several Jobs under its `jobs` list. There
is no front-matter Job list and no heading-based item model. `internal/plan`
parses the block; `internal/job` decodes the planned fields; `internal/jobgraph`
uses `job.Job` for dependencies and floor evaluation.

## Planned fields

Every entry supplies `id`, `item`, `title`, `role`, `skills`, `review`, `risk`,
`max_rounds`, `intent`, `links`, `keep`, `decisions`, `read`, `write`,
`out_of_scope`, `acceptance`, `verify`, `budget`, `on_blocked`, and `after`.
These are `job.PlannedFields`; empty lists remain explicit lists.
The parser derives omitted `item`, `title`, `intent.item`, and `intent.goal`
from the enclosing checklist item. `intent.owner` is required. The step's
`kind` applies to its Jobs and is not repeated inside a Job entry.

Optional Job fields are `visible` (`big` or `small`), `irreversible` (boolean),
and `review_reason` (text). Run fields such as `status`, `attempt`, and
`worktree` do not belong in the plan and are refused there. Rendered brief
metadata retains the step kind and supplied floor fields for later readers.

Job-record contract version 1 also accepts these optional fields: `kind` matches
`^[a-z][a-z-]*$` (1–40 characters), `visible` is `big` or `small`, `irreversible`
is boolean, and `review_reason` is a nonempty sentence (at most 500 characters).
Adding optional fields does not change the contract version.

`grid job render` selects a Job by its id or an item number. An item number is
ambiguous when it has more than one Job; select the Job id in that case.
`plan.md#step-N` quotes the checklist line and prose only. `plan.md#verify`
quotes the first paragraph under `## Verify`.

## Dependencies and review

`after` is a list of bare Job ids, gate ids, or objects with `job`, `kind`, and
`why`. A bare Job id means `done`. An object uses `done`, `failed`, or
`children-done` and explains the link in 8 to 160 characters. Gate ids have
the form `gate:Q-7` and no edge kind. Unknown targets, duplicate Job ids, and
cycles are errors; `children-done` also includes the target's children when
checking cycles. Gate state is supplied by the Feed, never by a checkbox.

The first applicable review rule sets the floor: irreversible or environment
work is L2; declared risk or a tagged risk path is L2; a big visible change or
UI path is L1; tests, docs, renames, generated files, and task-only research
are L0; other work is L1. The declared review must be L0 through L3. A lower
level needs a nonempty `review_reason`; this records the exception without
changing the floor or removing classify/capture requirements. `visible: small`
does not lower a floor established by scope or risk.

Code verification names a test command and a positive `min_tests`. For
`non-code`, `research`, or `scout` work, `verify` may instead name
`kind: file-exists` or `kind: reviewer-check`. A kind and a command cannot be
combined. These checks validate planned instructions, not execution evidence.

`grid plan check [T-NN]` validates the checklist through the shared parser.
`grid job cycles` and `grid job floor` also accept JSON arrays of `job.Job`
objects for graph inspection; they are not alternative plan-file parsers.
Project path tags can be passed to `grid job floor --policy <policy.json>`.
The plan check currently applies built-in path rules and explicit Job fields;
it does not discover project path tags from brain notes.
