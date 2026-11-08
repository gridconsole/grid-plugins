# Plan commands

Run these commands inside a task folder, or select a task with `--task <folder-or-id>`.
Task lookup uses the same `internal/where` rules as `grid where`. A task path must
belong to the Grid home. `--project <name-or-id>` can disambiguate a task id.

```sh
grid plan hash --task T-07
grid plan items --task T-07 --json
grid plan write draft.md --task T-07
grid plan diff 3 4 --task T-07 --json
```

Flags may precede or follow the file or revision arguments. Use `--` before a
filename that starts with a dash. Relative input filenames resolve from the
working directory. The input may have any filename, including `plan.md` itself.
The source file is read before the destination is replaced.

All four commands support `--json`. Hash returns `{"hash":"abcdef"}`; items
returns an array; write returns `{"revision":4,"hash":"abcdef"}`; diff returns
`from`, `to`, `added`, `removed`, and `changed`. Changed entries contain `before`
and `after` items, one-based positions, and canonical step text for both sides.
Empty result lists are arrays, not null. Exit codes are 0 for success, 1 for a
storage or content refusal, and 2 for invalid command syntax.

## Plan format

The first line is `# Plan, rev <positive integer> · <six hex digits>`.
`grid plan write` generates it. An input without this heading is also accepted;
an existing destination must already have a valid revision heading.

Plan items are top-level numbered checklist lines, in document order. Numbers
are stable ids, so removing an item does not require renumbering the others.
`- [ ] 5 Write the release note` becomes `i-5`, with state `open`. Either `x` or
`X` makes it `built`. Duplicate ids, zero ids and numbers outside uint64 are
refused. Bold markers and backticks are removed from display titles.

Ids are the author's step numbers, so they are stable across revisions only while
steps keep their numbers. Renumbering shows in `grid plan diff` as the old ids
removed and the new ids added.

Optional job fields sit under a step, indented with spaces. Nested YAML values
must be indented further. A blank or unindented line ends the step block.

```markdown
# Plan

## How we approach it

Used: brain:project/conventions/tag-matching

## Steps

- [ ] 5 Add the tag box to the list
  Preserve the selected note while narrowing the list.
  write: [src/list/**, tests/list/**]
  after:
    - J-1
    - {job: J-3, kind: failed, why: "Use the fallback if indexing fails"}
    - gate:Q-7
  review: L2
  risk: [concurrency, cross-platform]
  verify: {command: "go test ./internal/list/...", min_tests: 2}

Verify: filtering preserves the selected note.
```

The reader returns the fields present without inventing defaults. Structured
values must be valid JSON-compatible YAML. Duplicate fields and malformed YAML
are refused. A legacy `verify:` sentence remains a string, including backticks
and colons. This reader does not perform job schema, dependency cycle, review
floor, or risk validation.

When legacy `items.md` exists, its states are retained. A tick promotes `open`
or `working` to `built`. A running `subagents/*.md` brief listing an item promotes
`open` to `working`. Titles and membership always come from the plan. Reads never
rewrite these files; missing optional files are allowed, while unreadable files
are reported. With no steps, items returns an empty array.

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
Historical states come only from each revision's checkboxes. Current `items.md`
and running briefs do not leak into a historical comparison.

Diff reports item changes only. Changes to introductory prose, risks outside
items, or other plan sections can change the plan hash without producing an item
diff. It supplies comparison data for a view such as “Changed since you approved”;
it neither records approvals nor changes any Feed or task record.
