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
`--json` result, and prints `N brain proposals wait for the curator` otherwise,
so the orchestrator can start `grid-curator`. The home store is shared by every
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
`step: ship`, `gate: ship`, a `next` instruction, `outcome` (the Outcome's first
line) and `updated`, and sets an existing legacy `status` field to waiting. The
header is edited as a YAML document, as `task status` does, so key order,
comments and unknown keys survive; new keys go at the end.

The state moves through `task.NextState`: delivery is refused from `paused` and
`closed`, and from a task that is already `waiting`, as `grid task status` would
refuse the same move. To deliver again after the ship gate was raised, send the
task back first (`grid task status T-NN running`); delivery then replaces the
Outcome and appends another receipt. A journal left by a crashed writer is
replayed before delivery reads the task, so it cannot overwrite the delivery
later. Delivery never writes an approval and never closes the task.

The history receipt is a task event like the others (`what: deliver`, the
Outcome's first line as `note`, `--by` as the author). Artifacts and the proof
index are written before the commit and are not part of it: a failure after
the import and before the commit can leave copied artifacts or a rebuilt index,
and the gate is not raised. Repeated delivery imports listed files again under new
artifact IDs.

Lock order: `.task.lock`, then `.plan.lock`, then the brain levels' locks
(project, home, task), then `.proof.lock`. `grid approve` takes `.task.lock` and
then `.plan.lock`; nothing takes them the other way round.

### Ship approval

Once delivery has set `gate: ship`, `grid approve T-NN` records a ship approval in
`approval-ship.json`, not a plan approval. It is the same event with
`gate: ship` and the plan hash the person saw, under the same stale-hash rule as
the plan approval. `approval.json` is not touched, so the plan approval stays
valid. Reopening a task invalidates all three receipt files (plan, ship and
preview). `gate` stays in `task.md` until a later writer replaces it, including
after the task is sent back.

### Preview approval

A task whose `task.md` has `gate: preview` records the same way in
`approval-preview.json` (`gate: preview`) and also gets the history line
`- <time> · preview approved · <plan_hash> · by <who>`. `approval.json` and
`approval-ship.json` are not touched.

`grid preview [--task <task>] [--spec spec.md] [--by agent] [--dry-run] [--json]`
raises that gate. It is agent-callable like `deliver`; approving the preview stays
person-only. Under `.task.lock`, then `.plan.lock`, it sets `state: waiting` (the
legacy `status` only where present), `step: preview`, `gate: preview`, `next` and
`updated`, and appends one `what: preview` event whose note is the spec's first
heading. It writes no approval. The task's steps come from the inline `workflow:`
list in `task.md`, else the named template in the project's (then the workspace's)
`workflows.md`, filtered by the task's size; a workflow that cannot be resolved is
refused. It also refuses (exit 1, `Preview blocked: ...`) when the steps have no
`preview` approve step or no plan approval before it, when the plan receipt does
not match `plan.md`, when the spec (a regular file inside the task folder, no
symlink) is missing or blank, and from paused, closed or already waiting tasks.
Usage errors exit 2. `--dry-run` lists the actions and writes nothing.

The plan hold that `grid deliver` and the guard share also compares the
task's workflow, as its template defines it at every size, with the
`workflow_digest:` that `grid task new` wrote in `task.md`, also for a task made
with size `auto`: a different definition (a project folder renamed or rebuilt
around the task, or a person's change to `workflows.md`) holds the task with
code `workflow_changed`, and `grid deliver` refuses it with `plan not approved`
(with `--json`, the `workflow_changed` error envelope). Tasks without the field
keep the folder-name check alone (contract/README.md).

### The gate a person saw

`grid approve --gate plan|preview|ship` names the gate on the card the person
pressed. Under `.task.lock` it is compared with the task's gate (`ship`, `preview`,
and `plan` for any other `gate` value); when the task has moved on, for example
delivery raised `ship` after the person pressed Approve on the plan, the approval
is refused with exit 1 and `Changed since you approved: the task is at gate <g>,
not <want>`, and nothing is written. Another value exits 2 before any lock. Without
`--gate` the task's gate decides. Approving again while the receipt for that gate
still matches the plan's hash and digest (and the deploy rule) writes nothing: the
receipt is not rewritten and no second `preview approved` line or approve event is
added; the stored receipt is printed.

### History

`history add` takes exactly one nonempty line and rejects newline and NUL input.
It appends a task event (`what: note`, the line as `note`) through `Store.Note`,
which uses the same `.task.lock` and journal as `task status`, so notes and state
moves never lose each other's lines. A note changes no state and works on a
closed task. Lines follow the writer's format, `- {json event}`, and existing
history is preserved, including a missing final newline. Runtime locking is
supported on macOS and Linux; other platforms inherit `internal/fslock`'s
existing unsupported-platform error.

Person-only commands, including `grid approve` and `grid task status`, refuse
with exit **3** when called from an agent environment or process ancestry. Run
them from the Grid app or a separate person terminal. `grid person check --json`
reports whether that entry point is available. `grid deliver` still raises the
ship gate in-process and does not require the person gate.
