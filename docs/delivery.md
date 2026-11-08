# Delivery commands

These commands use the current task, or `--task T-NN` to select a task anywhere
in the Grid home. Task lookup uses `internal/task` and its shared task-directory
resolver. All commands support `--json`.

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
- [ ] 2 Capture the result
  proof: screenshot
```

The lane brief requires proof levels but the referenced documents do not define
an enum. This implementation uses `check`, `log`, `screenshot`, and `video` as
levels. Omitted `proof` means `check`; a log also satisfies a check. Other levels
require the matching kind. Unknown levels fail explicitly. Evidence presence
and integrity do not independently establish that a test passed or that a
reviewer approved the work.

Item arguments accept `1`, `i-1`, and `item-1`, normalized to the existing plan
identifier `i-1`. Unknown items and `_task` are refused. Text evidence is allowed
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
A missing topic is shown as `unassigned`; malformed proposal records are errors.

Delivery refuses closed, stopped, or cancelled tasks, waiting proposals,
missing or invalid proof, empty plans, and unreadable deliverables. Refusals are
one stderr line and a nonzero exit. Preflight refusals and `--dry-run` write no
files, including locks. A dry run checks the same inputs and prints each action.

On success, delivery replaces `## Outcome` while retaining other body sections
and task metadata, imports deliverables using `internal/artifact`, rebuilds the
proof index, and appends a dated delivery line through `task.AppendHistory`.
It then sets `state: waiting`, `step: ship`, `gate: ship`, and a `next` instruction
to review the Outcome. An existing legacy `status` field is also set to waiting.
It never writes an approval or closes the task. The existing task API continues
to own the task record; no new Feed or task-record schema is introduced.

The task, plan, and brain writer locks protect delivery's checks and gate update.
Proof and history writes use their respective persistent kernel locks. File
replacements use `internal/atomicfile`. A late I/O failure can leave an Outcome,
copied artifacts, index, or history line; delivery does not newly raise the gate
unless all preceding writes succeed. This is not a transaction across files.
Repeated delivery replaces Outcome, appends another history line, and imports
listed files again under new artifact IDs.

`history add` takes exactly one nonempty line and rejects newline and NUL input.
It appends `- <UTC RFC3339 time> · <line>` using the single history writer and
`.history.lock`. Existing history is preserved, including a missing final
newline. The lock is shared across processes and is released by the kernel on
process exit. Runtime locking is supported on macOS and Linux; other platforms
inherit `internal/fslock`'s existing unsupported-platform error.
