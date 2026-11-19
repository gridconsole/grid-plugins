# Delivery commands

These commands use the current task, or `--task <task>` to select a task in the
Grid home. `<task>` accepts a T-NN id, an exact task folder name, or a task folder
path through the shared CLI resolver. Folder paths must name a direct child of
an accessible project tasks directory; folder operands do not widen caller
access. All commands support `--json`.

```sh
grid proof add 1 --text 'The acceptance checks passed' --kind check --by grid-checker
grid proof add i-2 screen.png --kind screenshot --note 'Result at the requested size'
grid proof index
grid brain keep
grid history add 'Checked the exported report'
grid deliver --outcome 'The report is ready.' --file report.txt --dry-run
grid deliver --outcome 'The report is ready.' --file report.txt
```

`deliver` also accepts the Outcome as its first positional argument and files
as subsequent arguments. `--file` is repeatable. Relative input paths are
relative to the command's working directory. `--by` defaults to `grid`, without
reading a local username. Outcome text can contain Markdown and newlines; its
first line becomes the frontmatter `outcome` summary.

## Proof

A numbered plan item can declare an evidence kind:

```markdown
- [ ] 1 Check the exported report
  proof: check
  jobs: []
- [ ] 2 Capture the result
  proof: screenshot
  jobs: []
```

`proof:` is one of the three step keys the plan reader allows (`kind:`, `jobs:`,
`proof:`), in any order; every step still needs a `jobs:` list.

The `proof:` key is provisional (see `docs/plan.md`), and no document defines a
proof-level enum. This implementation uses `check`, `log`, `screenshot`, and
`video` as levels. Omitted `proof` means `check`; a log also satisfies a check. Other levels
require the matching kind. Unknown levels fail explicitly. Evidence presence
and integrity do not independently establish that a test passed or that a
reviewer approved the work.

Item arguments accept `1`, `i-1`, and `item-1`, normalized to the existing plan
identifier `i-1` by `plan.ItemID`; `internal/proof` holds no id parsing of its own. Unknown items and `_task` are refused. Text evidence is allowed
for check and log; screenshot and video require files. Screenshots accept PNG,
JPEG and WebP; videos accept MP4 and WebM. Inputs must be nonempty regular files
or text, at most 20 MiB. File bytes are preserved; media duration and content are
not decoded or judged. Sources and destination paths cannot be symlinks below
the resolved task root.

Evidence files have unique names inside `proof/i-N/`. Each item's `proof.json`
is an array of records with `kind`, `by`, `at`, `sha256`, `note`, `file`, and
`plan_hash`. The file path is relative to the task. Each record binds the bytes
to `internal/plan.Hash`; a substantive plan revision requires new evidence.
Checkbox-only changes do not change that hash. Old evidence is retained.

`proof index` writes `proof/index.json` with `plan_hash` and an `items` array.
Each row contains `id`, `title`, `level`, `proven`, and `evidence`. Items follow
plan order; evidence follows timestamp then relative file name. No rebuild
timestamp is added, so unchanged inputs produce identical bytes. The index
checks original files and checksums. A missing record leaves an item unproven;
malformed metadata, missing referenced files, and checksum mismatches are errors.
Evidence for removed plan items is retained on disk but omitted from the index.

## Delivery and the keeper

`brain keep` lists waiting proposals with a scope-qualified path, topic, and
one-line reason. JSON is an array of `{id, path, topic, reason}`. It includes
current-task proposals and the project and home proposal stores. `--project`
and `--level` retain the brain command's existing scope selection. It writes
nothing and does not launch an agent or accept, merge, or decline proposals.
The same command also carries the curator's three acts, each one a call a
subagent can make: `brain keep --name <id> --topic <topic> --description <when>
--summary <what> --source <source> --body <text> --link <JSON>` records a
proposal (state `proposed`; each `--link` carries `to`, `rel`, `summary` and a
`why`, and a link without a reason is refused), `brain keep accept <id>` files
it as a linked note, and `brain keep refuse <id> [--why <reason>]` records the
refusal and writes no note. They run the same code as `brain propose` and
`brain accept [--reject]`.
A missing topic is shown as `unassigned`. Project and home proposals are read
with `brain.Parse`, the reader `accept` uses, so keep lists exactly what accept
will read: an unknown field, an alias, a name that differs from the file name or
a level that differs from the folder is an error. Only the older proposals in a
task folder keep a lenient reader.

A waiting proposal never stops a delivery. Delivery counts the proposals of three
kinds: the task's own folder, the whole project store, and home-level proposals
whose `source` or `origin.id` names this task (`T-NN`, also as `T-NN/J-NN` or a
`T-NN-title` folder name) or its project, as a whole word. It goes ahead and
returns them as `waiting_proposals` (`{id, path, topic, reason}` rows) in the
`--json` result, and prints `N brain proposals wait for the curator (Job J-n)`
otherwise. After the delivery is written, with the task lease held (`GRID_MEMBER`
and `GRID_LEASE_EPOCH`), it posts one curator Job (role `curator`, skill
`brain-keeper`, level L0) that points at the waiting proposals; `--json` names it
in `curator_job` (empty when none is posted). No second curator Job is posted while
one is open, and none when no proposal waits. A Job that cannot be posted never
fails the delivery: one stderr line says why, and the dry run posts nothing and
lists `Post a curator Job for N proposals`. The home store is shared by every
project, so an unrelated proposal there is not counted; `brain keep` still lists
it. A proposal that cannot be read is still an error.

Delivery refuses closed, stopped, or cancelled tasks, tasks whose state cannot
move to `waiting`, missing or invalid proof, empty plans, and
unreadable deliverables. Refusals are one stderr line, prefixed `deliver:`, and a nonzero exit.
Preflight refusals and `--dry-run` write no files, including locks. A dry run
checks the same inputs and prints each action.

### The write

Delivery writes the task record only through the task writer (`internal/task`,
`withWrite` and `commitWrite`), under `.task.lock`. After the checks pass again
under the locks, it imports the deliverables, rebuilds the proof index, then
commits `task.md` and one history receipt together in a single journal commit. In
`task.md` it replaces `## Outcome` (other sections stay), sets `state: waiting`,
`step: ship`, a `next` instruction, `outcome` (the Outcome's first
line) and `updated`; it writes no `gate`. It sets an existing legacy `status` field to waiting. The
header is edited as a YAML document, as `task status` does, so key order,
comments and unknown keys survive; new keys go at the end.

The state moves through `task.NextState`: delivery is refused from `paused` and
`closed`, and from a task that is already `waiting`, as `grid task status` would
refuse the same move. One waiting task is delivered: a task at its plan gate
whose plan approval is current. `grid approve` keeps the task waiting, so when
`approval.json` holds the plan hash that `plan.md` has now, delivery moves the
task to waiting at step `ship` as it does from `running`. A waiting task with no
approval, with an approval of an older plan, or already at step `ship` is
refused. To deliver again after the ship person Job began waiting for the person, send the
task back first (`grid task status T-NN running`); delivery then replaces the
Outcome and appends another receipt. A journal left by a crashed writer is
replayed before delivery reads the task, so it cannot overwrite the delivery
later. Delivery never writes an approval and never closes the task.

The history receipt is a task event like the others (`what: deliver`, the
Outcome's first line as `note`, `--by` as the author). Artifacts and the proof
index are written before the commit and are not part of it: a failure after
the import and before the commit can leave copied artifacts or a rebuilt index,
and the ship person Job is not posted. Repeated delivery imports listed files again under new
artifact IDs.

Lock order: `.task.lock`, then `.plan.lock`, then the brain levels' locks
(project, home, task), then `.proof.lock`. `grid approve` takes `.task.lock` and
then `.plan.lock`; nothing takes them the other way round.

### Approval after delivery

There is one approval: the plan Job's accepted result (contract/README.md, "Plan
approval receipts"). Delivery writes no approval and no `gate`. After the write it
posts one person Job for the ship step (`Post person Job J-n for the ship step`;
`person_job` in the `--json` result), which the person answers in the Grid app or
with `grid approve T-NN`, as any plan approve Job. No second person Job is posted
while one is open. A Job that cannot be posted never fails the delivery. There is no
preview command, and no ship or preview receipt: `approval-ship.json` and
`approval-preview.json` are no longer written or read.

The plan hold that `grid deliver` and the guard share also compares the
task's workflow, as its template defines it at every size, with the
`workflow_digest:` that `grid task new` wrote in `task.md`, also for a task made
with size `auto`: a different definition (a project folder renamed or rebuilt
around the task, or a person's change to `workflows.md`) holds the task with
code `workflow_changed`, and `grid deliver` refuses it with `plan not approved`
(with `--json`, the `workflow_changed` error envelope). Tasks without the field
keep the folder-name check alone (contract/README.md).

`grid approve` has no gate flag: it takes `--plan-hash`, `--note`,
`--send-back` and `--json`. Approving again while the plan Job's result still
matches the plan's hash and digest writes nothing new.

### History

`history add` takes exactly one nonempty line and rejects newline and NUL input.
It appends a task event (`what: note`, the line as `note`) through `Store.Note`,
which uses the same `.task.lock` and journal as `task status`, so notes and state
moves never lose each other's lines. A note changes no state and works on a
closed task. Lines follow the writer's format, `- {json event}`, and existing
history is preserved, including a missing final newline. Runtime locking is
supported on macOS and Linux; other platforms inherit `internal/fslock`'s
existing unsupported-platform error.

`grid approve` is for a person of the team or an agent (the owner, 19 Nov; no
signing). An agent's shell (an agent `GRID_ROLE`, or a worker mark such as
`GRID_AGENT` with no `GRID_ACTOR`) answers the task's open approve Job unleased:
the answer's author is its member in role `agent`, its reason `approved by agent
<GRID_JOB>` (else the agent's worker id), and the holder's next pass accepts it.
With no open approve Job it exits **3** and writes nothing. A person's shell exits
**3** and writes nothing when `GRID_MEMBER` is not set, and a guest always does.
It records a person's approval as `GRID_MEMBER`, and it does not look at the
process ancestry. `grid task status` is not person-only: it records its caller (a
person's shell as the local owner, a worker shell as the worker), and
`agents/orchestrator.md:27,48` has the orchestrator agent run it. A person runs
`grid approve` from the Grid app or a terminal that exports `GRID_MEMBER`; an
agent Grid did not launch runs `GRID_AGENT=<name> grid approve`. `grid deliver`
still moves the task to waiting at step `ship` in-process and does not require a
person.
