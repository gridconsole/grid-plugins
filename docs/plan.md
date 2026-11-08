# Plan commands

Run these commands inside a task folder, or select a task with `--task <folder-or-id>`.
Task lookup uses the same `internal/where` rules as `grid where`. A task path must
belong to the Grid home. `--project <name-or-id>` can disambiguate a task id.

```sh
grid plan check T-07
grid plan hash --task T-07
grid plan items --task T-07 --json
grid plan write draft.md --task T-07
grid plan diff 3 4 --task T-07 --json
```

Flags may precede or follow the file or revision arguments. Use `--` before a
filename that starts with a dash. Relative input filenames resolve from the
working directory. The input may have any filename, including `plan.md` itself.
The source file is read before the destination is replaced.

All commands support `--json`. Hash returns `{"hash":"abcdef"}`; items
returns an array; write returns `{"revision":4,"hash":"abcdef"}`; diff returns
`from`, `to`, `added`, `removed`, and `changed`. Changed entries contain `before`
and `after` items, one-based positions, and canonical step text for both sides.
Empty result lists are arrays, not null. Exit codes are 0 for success, 1 for a
storage or content refusal, and 2 for invalid command syntax.

## Plan format

The first line is `# Plan, rev N · <hash>`, where N is the positive revision
number and hash is `plan.Hash` (six hex digits, ticks normalized).
`grid plan write` generates it and also accepts drafts without a revision heading.

A plan item is a top-level line `- [ ] N Title`. N is a stable positive item
number; removing an item does not renumber the others. Duplicate numbers are
refused. The checkbox is display only: neither `x` nor `X` supplies run state.
Bold markers and backticks are removed from display titles.

Under the item, prose lines have two leading spaces. No blank line falls inside
the block. The YAML part starts at the first line beginning with exactly two
spaces and `kind:`, `jobs:` or `proof:` (in any order) and continues to the block end. `kind` is the step
kind; `build` is ordinary code work. `jobs` is always a list, including `[]` when
there are no Jobs yet. Prose must come before YAML. Only `kind`, `jobs` and
`proof` are allowed as step YAML keys. `proof` is a string, the evidence level
(`check`, `log`, `screenshot` or `video`) that `grid proof` and `grid deliver`
require for the step; it defaults to `check`. **Provisional:** the key was added
by the delivery commands and has not been confirmed by the owner, so it can be
renamed or removed, and the plan skill's format should wait for that. Step
references (`1`, `i-1`, `item-1`) are parsed by `plan.ItemID`, the one place item
ids are parsed.

Each Job carries the names in `job.PlannedFields`, even when a list is empty.
When omitted, `item` is N, `title` is the checklist title, `intent.item` is
`plan.md#step-N`, and `intent.goal` is `plan.md#verify`. Explicit values are
preserved. `intent.owner` must still be supplied. The optional per-Job floor
fields are `visible: big|small`, `irreversible: true|false`, and `review_reason`.
A review below its floor requires a nonempty `review_reason`.

```markdown
# Plan, rev 1 · 000000

Used: brain:project/conventions/task-page

- [ ] 5 Composer with Attach on the task page
  The composer sits on a plan item, a question reply and Talk to the planner.
  kind: build
  jobs:
    - id: J-14
      role: builder
      skills: []
      review: L1
      risk: []
      max_rounds: 2
      intent: {owner: intent.md#request}
      links: []
      keep: []
      decisions: []
      read: []
      write: ["app/pages/task/**"]
      out_of_scope: []
      acceptance:
        - {id: A1, text: "The task page has a composer with Attach.", traces: item}
      verify: {command: "node --test tests/app/pages/task*.test.mjs", min_tests: 1}
      budget: {turns_estimate: 20, reads_before_first_edit: 5}
      on_blocked: ask-and-continue
      after: []

## Verify

The task page composer attaches a file and sends the message.
```

The example's hash is a placeholder; install it with `grid plan write`.
Step-level `write:`, `after:`, `review:`, `risk:`, and `verify:` outside `jobs`
are refused with the offending line and line number. Front matter is refused;
the checklist is the only source of plan membership and planned Jobs.
`internal/plan` is the only parser, shared by Job loading and graph validation.

`plan.md#step-N` resolves to the item line plus its prose lines, excluding YAML.
`plan.md#verify` resolves to the first paragraph under exactly `## Verify`,
excluding the heading and later paragraphs. Old `Verify:` sentences and
heading-based item anchors do not substitute for these forms.

`grid plan check [T-NN]` checks planned-field presence after derivation, unknown
fields (including nested fields), review floors and exceptions, positive
`verify.min_tests` with a named test command, duplicate item numbers and Job ids,
dependency targets, and cycles. Non-code Jobs can use `file-exists` or
`reviewer-check`. It prints one line per reported problem and exits 1 when any
problem exists; a valid plan exits 0 without output. `--json` returns `ok` and
`problems`. A malformed block stops structural parsing; fix it and rerun to
check subsequent blocks. Check does not execute verification commands.

When legacy `items.md` exists, `grid plan items` retains its states. A running
`subagents/*.md` brief can promote `open` to `working`. Checkboxes never promote
state. Titles and membership always come from the checklist. Reads do not
rewrite these files; missing optional files are allowed. An empty plan returns
an empty item list. Revision comparisons contain no run-state inference.

## Hash compatibility

Hash uses the original six-digit SHA-256 calculation, including ECMAScript's
Unicode whitespace set. It always drops the first line. It normalizes CRLF,
trailing whitespace, blank lines at the ends, and checkbox ticks. It omits the
`## Where it stands` section except unindented steps and their indented blocks.
Indented `evidence:` and `needs:` lines are omitted only inside step blocks.
Everything retained is joined with a final newline.

Leading indentation, spaces within sentences, internal blank lines, item order,
step prose, `verify:` and the new job fields still change the hash. In particular,
`after:` changes count even though legacy `needs:` changes do not. Whitespace
normalization is deliberately limited to the old algorithm. The short hash is
an approval compatibility value, not a cryptographic integrity guarantee.

## Writes and history

Write requires a nonempty `Used:` citation or exactly `Recall: nothing found`.
Legacy bullet and bold forms and case-insensitive labels are accepted. Fenced
examples, HTML comments, blockquotes and indented code do not satisfy the gate.
This is a citation-presence check; it does not resolve or validate the note.

Every successful write increments the installed revision, even if the hash
stays the same, except that bytes identical to the current plan are a no-op and
return the current revision. A new plan starts at 1; the input's revision number is ignored.
The previous file's exact bytes are kept in `plan-revisions/<number>.md`.
Archives are never pruned. A conflicting archive refuses the write; an identical
archive left by an interrupted write is reused.
New plan files, archives, and locks have mode 0600; the archive directory has
mode 0700. Replacing a plan does not preserve its former permission bits.

Writers hold the persistent `.plan.lock` through `internal/fslock`. Each file
is written to a temporary file in its destination directory, synced, closed,
and renamed. The archive is installed before the current plan. A process failure
between those operations leaves the old current plan and its complete archive;
a retry can continue. This is not a multi-file transaction or a promise of
directory-entry durability across power loss. Read commands create no lock file
and see complete plan files. Readers do not receive a transactional snapshot of
the independently maintained legacy state files.

Storage files must be regular files; symlinks at the plan, lock, archive, or legacy
state paths are refused. The task root is resolved first, so symlinked ancestor
directories, including macOS temporary directories, work normally. Storage
operations inherit fslock's unsupported-platform error on native Windows. The
package and CLI cross-compile for Windows and macOS.

## Revision comparison

Diff takes two positive integer revision numbers. Each is loaded from the
current plan or its archive; a missing revision or an archive with a mismatched
heading is an error. Reverse and same-revision comparisons are supported.

Items match by id. Content or absolute position changes are reported as changed;
inserting an item can therefore also move later items. Comparison uses the full
canonical step text rather than the truncated hash. Changes to descriptions and
job fields count. Tick, evidence, and legacy `needs:` changes alone do not count.
Checkboxes supply no historical run state. Current `items.md` and running
briefs do not leak into a historical comparison.

Diff reports item changes only. Changes to introductory prose, risks outside
items, or other plan sections can change the plan hash without producing an item
diff. It supplies comparison data for a view such as “Changed since you approved”;
it neither records approvals nor changes any Feed or task record.
