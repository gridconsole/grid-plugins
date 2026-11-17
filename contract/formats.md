# Shared file formats for g2

The `fmt-` schemas pin shared files, not Feed post types. They have neither
`x-class` nor `x-roles`; `PostTypes()` and `grid feed types --json` are unchanged.
They use only the subset checked by `internal/contract/schema.go:145`.
Reference citations below name the read-only spec documents and their line
numbers. Code citations describe the supplied checkout; no writer is changed.
The format gate is recorded in `while-away.md:63,66,72,75` and
`migration.md:22`; this copy of `while-away.md` has no `fm-formats` anchor.

The [capture suite](formats/formats_test.go) runs the CLI dispatcher in process
on temporary homes. Origin fixtures normalize only the generated identity;
render fixtures contain the complete saved text and decoded source metadata.
`internal/contract/schema_test.go:21` checks every valid and invalid JSON fixture.
A schema checks shape, not the authenticity of an origin or account.

## Sync manifest

Schema: [fmt-sync-manifest.json](schemas/fmt-sync-manifest.json).
Fixtures: [cut 1](fixtures/fmt-sync-manifest/valid/cut-1.json),
[two repos and a guest](fixtures/fmt-sync-manifest/valid/project-two-repos-guest.json).

Status: **spec-only**. Proposed file path: `$GRID_HOME/sync/manifest.json`
(default, owner may change; no production file is created). The intended writer
is a sync verb the plugin does not have today (the empty `grid sync` stub was removed), invoked directly or by the worker; intended readers are the app's
sync slice, grid-server, another signed-in computer and the brain curator.
`go-architecture.md:13` makes the plugin the only sync client and says the server
never runs it. Sources: C8 (`findings-concept.md:185`), server-F1
(`findings-runtime.md:193`), `go-architecture.md:13,392`,
`brain-feed.md:285-289`, and `feed-contract.md:7-13`.

All field spellings and policy tokens below are **default, owner may change**:
the docs define the scope and authorities but do not supply a wire object.
`manifest_v` is fixed at 1. The object is a paid-account selection manifest,
with logical identity and selection entries rather than arbitrary paths or content.
It carries `manifest_v` and those entries; the fixed class policy lives only in
the schema description and this contract. A `classes` property is forbidden.
A local-only installation produces no sync manifest. Cut 1 mirrors a person's
own task folders; other members see status and requests through the server.
Cut 2 permits members' task folders on other members' desktops, subject to their
project role. Raw Feed replication remains deferred in both example cuts.

| Field | Meaning and constraint | Spec doc:line |
| --- | --- | --- |
| `manifest_v` | Integer constant 1; version spelling is a lane default. | findings-runtime.md:193 |
| `cut` | 1 for own computers; 2 for project-member task mirrors. | go-architecture.md:392 |
| `account` | Account whose sync view this manifest describes. | brain-feed.md:286 |
| `account.person` | Opaque person id; no local username or filesystem path. | go-architecture.md:392; brain-feed.md:286 |
| `account.paid` | Constant `true`; project brain sync requires the paid account. | brain-feed.md:286 |
| `machines` | Nonempty list of participating computers. | go-architecture.md:392 |
| `machines[].id` | Machine ULID: 26 uppercase Crockford Base32 characters, using the same pattern as task ids. | feed-contract.md:34 |
| `machines[].person` | Owning person id; identifier character restrictions are defaults. | brain-feed.md:286 |
| `projects` | Nonempty project list, keyed logically by hosted project id. | go-architecture.md:13; brain-feed.md:286 |
| `projects[].id` | Hosted project id, not the readable local folder name. | brain-feed.md:286 |
| `projects[].repos` | Nonempty list identifying the repos associated with the project. It selects no repo files. | go-architecture.md:13 |
| `projects[].repos[].remote` | Normalized lower-case `host/path`, without scheme, credentials, port or `.git`; dotted internet-host form is a default. No repo path field exists. | go-architecture.md:13; brain-feed.md:286 |
| `projects[].members` | Project-specific people; no team-brain scope. | go-architecture.md:392; brain-feed.md:285 |
| `projects[].members[].person`, `.role` | Person id and `member` or `guest`; guest reads are role-filtered and guest brain writes are refused. | brain-feed.md:285; findings-brain.md:72 |
| `projects[].approvers` | Person ids that may approve within the project. | go-architecture.md:392 |
| `projects[].tasks` | Tasks whose task files and folded state are selected. | findings-concept.md:185; feed-contract.md:13 |
| `projects[].tasks[].id` | Task ULID, never the display number `T-NN`. | feed-contract.md:43 |
| `projects[].tasks[].owner` | Person whose computers receive the cut-1 mirror. | go-architecture.md:392 |
| `projects[].tasks[].machine`, `.gen` | Server binding `(task, machine, gen)`; machine ULID and nonnegative generation, 0 before binding. | feed-contract.md:13,34,49 |

These are server-F1's five columns. Backticked values are the exact policy tokens
in the schema description, not fields repeated in each manifest. Class scope
references apply to each cell in that row. The 8 November decision confirms
that personal notes sync only to their owner's computers and secrets never sync.

| Data class | Authority | What the server holds | Who reads | Conflict rule |
| --- | --- | --- | --- | --- |
| Task files (`task_files`; findings-concept.md:185; go-architecture.md:392) | `bound-machine`, checked against the server binding. | `task-files`, selected task files outside repos. | `own-computers-cut-1-project-members-cut-2`; teammates see status in cut 1. | `last-writer-wins-keep-conflict-file`: last writer wins per file; preserve the loser as `<file>.conflict-<machine>`; default, owner may change. |
| Folded Feed state (`folded_feed`; findings-concept.md:185; feed-contract.md:13) | `bound-machine`. | `folded-state`, for status and needs, not raw streams. | `project-status-and-needs`, subject to role. | `rebuild-from-records-never-merge`: rebuild on the authoritative machine from admitted records; never merge folded states; default, owner may change. |
| Project brain (`project_brain`; brain-feed.md:285-286; findings-brain.md:72) | `project-members-no-guest-writes`; CLI stamps provenance, server authenticates the author. | `project-brain-by-project-id`, with repo remotes listed under that project. | `project-members-role-filtered-and-workers`. | `version-with-superseded-by`: entry versions retain `superseded_by`; default, owner may change. |
| Personal notes (`personal_notes`; brain-feed.md:285-286; go-architecture.md:13) | `person`. | `private-personal-notes`, accessible only to the same person's computers. | `own-computers-only`; never another member or shared worker. | `version-with-superseded-by`; default, owner may change. |
| Secrets (`secrets`; go-architecture.md:13) | `local-only`. | `nothing`. | `local-only`. | `never-sync`. |
| Repos (`repos`; findings-concept.md:185; go-architecture.md:13) | `git-remote`; code uses its own Git transport. | `nothing`. | `git-clients`, through Git. | `never-sync`. |
| Raw Feed files (`raw_feed`; feed-contract.md:13; findings-concept.md:185) | `bound-machine`. | `nothing`. | `local-only`. | `deferred-not-in-cut-1`; no replication is enabled by this version. |

Class selectors must exclude local site files, locks, leases and caches
(`brain-feed.md:289`), raw Feed files, secrets and repo files even when nested
below a selected task. The schema does not enumerate physical files or implement
a secret scanner. The invalid repo fixture substitutes a local path for a
normalized remote; the invalid secret fixture attempts to add a forbidden
`classes` block to request server storage of secrets. Both are rejected. Extra
path or payload fields are also refused.

The eventual client/server must check account authentication, machine ownership,
unique ids, reference membership, approver roles, the task binding and cut-specific
access. JSON Schema cannot compare those identities or prove the absence of
secret data inside a selected file. The manifest does not grant authority by
asserting a role. Clock ordering and tie-breaking for last-writer-wins remain
with the sync owner; the preservation rule above is frozen as the default.

| Status | Difference or unresolved specification | Evidence |
| --- | --- | --- |
| diverges | A manifest is required by the spec, but there is no writer, reader, file path or sync transport in this checkout. This schema and its example path are proposals. | findings-runtime.md:193; go-architecture.md:13; internal/cli/cli.go:122 (`sync` has a nil handler) |

## Saved Job render

Schema: [fmt-job-render.json](schemas/fmt-job-render.json).
Fixtures: [full](fixtures/fmt-job-render/valid/full.json),
[elided](fixtures/fmt-job-render/valid/elided.json),
[written before `shown`](fixtures/fmt-job-render/valid/written-by-main.json).

Files: `<task>/jobs/<job-id>/brief.md` and adjacent `inputs.json`. The same
immutable bundle contains `job.yaml`, which currently contains JSON Job metadata;
that Job record belongs to the Job contract, not this format's metadata object.
The writer is `grid job render`, through `internal/job/jobcmd/command.go:108-124`
and `internal/job/jobcmd/save.go:15-74`. Readers are `grid job show`, reviewers,
the app and replay/audit consumers. Spec: review-F1 (`findings-concept.md:144`),
`brain-feed.md:13,278-282`, and `job-contract.md:80-96`.

The on-disk representation has two files, so a JSON Schema cannot validate the
raw Markdown file and its sibling as one JSON document. Validation uses the
existing `job.Rendered` shape (`internal/job/render.go:51-54`):
`{"brief": <exact brief.md string>, "inputs": <decoded inputs.json>}`.
This wrapper is a lossless validation projection, **not a new saved file**.
The capture test reads the real files, keeps the complete `inputs.json` object
as `json.RawMessage`, and compares the result with the checked-in fixture.
Consumers must keep both files; validating `inputs.json` alone cannot establish
that the saved prompt exists. No hash is substituted for text.

| Field in validation projection | Presence | On-disk meaning and constraints | Writer and spec |
| --- | --- | --- | --- |
| `brief` | required | Complete nonempty UTF-8 prompt, including Job front matter and all rendered sections; exact contents of `brief.md`. | internal/job/jobcmd/save.go:36; findings-concept.md:144 |
| `inputs` | required | Exact object stored as `inputs.json`. The five required fields below are always present; `brief_bytes` and `pasted_chars` are optional. No other field is allowed. | internal/job/render.go:24-35; job-contract.md:80 |
| `inputs.generation` | required | String containing the project generation map, such as `{"project":1}`, or `content:<sha256>` when no project index generation exists. | internal/job/jobcmd/command.go:119-122,451-467; job-contract.md:80 |
| `inputs.sources` | required | Array of every resolved source, including candidates omitted from the prompt, sorted by pointer. | internal/job/render.go:41,281-289; findings-concept.md:144 |
| `inputs.not_rendered` | required | Sorted pointer strings omitted by the brain budget; never null. | internal/job/render.go:204-207; job-contract.md:96 |
| `inputs.brain_tokens` | required | Integer 0–2000; character-count estimate for the brain section, not a cap on the full prompt. | internal/job/render.go:208,290; job-contract.md:96,98 |
| `inputs.disableSkillShellExecution` | required | Boolean set when guest/external non-personal sources were resolved, even when a candidate was later omitted. | internal/job/render.go:63-68; job-contract.md:89 |
| `inputs.sources[].pointer` | required | Resolved source pointer, including any anchor or range. | internal/job/source.go:25; job-contract.md:80 |
| `.summary`, `.text` | required | Source summary and pinned text, not only a digest. Empty for personal pointer-only sources. | internal/job/source.go:26-27,161-170; findings-concept.md:144 |
| `.tier` | required | One of the four current trust tiers, taken from the source. | internal/job/source.go:28; job-contract.md:87 |
| `.sha256` | required | Digest string: full file bytes for files, encoded note for project brain; empty for personal pointers. | internal/job/source.go:29,176-181,237-238; findings-concept.md:144 |
| `.state`, `.topic`, `.updated` | optional | Optional source lifecycle/topic/update strings; omitted when empty. `updated` is copied metadata, not constrained to a timestamp here. | internal/job/source.go:24,30-31,166,216-217; brain-feed.md:278 |
| `.links` | optional | Optional array of source links; each has string `to`, `rel`, `why`. | internal/job/source.go:32,167-168; brain-feed.md:278 |
| `.pointer_only` | optional | Optional boolean, emitted when true for home notes; text, summary, digest and links are not copied. | internal/job/source.go:33,161-164; brain-feed.md:285-286 |
| `.shown` | optional | Optional string, how the brief used the source: `text` (intent sources and the previous acceptance, pasted in full), `excerpt` (a brain note's text after its pointer line), `summary` (a pointer or read line shows the summary), `pointer` (the line names the path or id and nothing of the file). The strongest use wins when a source is used twice (text > excerpt > summary > pointer). Absent when the brief did not use the source, such as a brain note cut by the budget, and in files written before this field. `text` stays in every source whatever `shown` says, so replay rebuilds the same brief. | internal/job/render.go (`Shown*`, `Render`); internal/job/source.go (`Source.Shown`) |
| `inputs.brief_bytes` | optional | Integer, at least 0; absent in files written before it. It is `len(brief)`. | internal/job/render.go (`Inputs`, `Render`) |
| `inputs.pasted_chars` | optional | Integer, at least 0; absent in files written before it. It is the rune count of source text written into the brief (intent sources, excerpts, the previous acceptance), so it is below the summed `sources[].text` whenever a source was only summarised or named. | internal/job/render.go (`Inputs`, `Render`) |

The plugin writes both optional fields on every render (`internal/job/render.go:33-34`
has no `omitempty`); they are optional only so that older files stay valid.
`contract/formats/hf_render_doc_test.go` checks the `inputs.*` rows, their
presence column and the count above against the schema, and the json tags of
`job.Inputs` against the schema's `inputs` properties.

The source fields follow the writer's shape, including empty text/digest for
personal pointers. The schema does not verify digests, pointer resolution or
front-matter semantics inside `brief`; Job lint and the source resolver own
those checks. The saved fixtures exercise real source snapshots and the elision
list. Invalid fixtures remove the text, empty it, or remove the input metadata,
one violation per fixture.

| Status | Difference | Evidence |
| --- | --- | --- |
| diverges | The full text and what-it-read list exist, but storage is keyed by Job id, not attempt. A changed render of the same id is refused; separate per-attempt snapshots are missing. | findings-concept.md:144; internal/job/jobcmd/save.go:26-36,50-54 |
| diverges | The spec asks for a pinned brain index generation; when no index exists the writer pins a digest of the source list instead. The schema preserves that string form. | job-contract.md:80; internal/job/jobcmd/command.go:119-122,451-467 |
| diverges | The complete what-it-read snapshot excludes personal note content and digests; it retains only a pointer, tier and state. This respects personal-note isolation. | findings-concept.md:144; brain-feed.md:285-286; internal/job/source.go:161-164 |

Per-attempt prompt storage (review-F1) belongs to L45F and is outside this follow-up.

## Brain origin stamp

Schema: [fmt-origin.json](schemas/fmt-origin.json).
Fixtures: [session](fixtures/fmt-origin/valid/session.json),
[import](fixtures/fmt-origin/valid/import.json).
Upgrade fixture: [old note](fixtures/fmt-origin/upgrade/legacy-note.md).
Invalid absence fixtures: [null](fixtures/fmt-origin/invalid/absent.json),
[missing fields](fixtures/fmt-origin/invalid/missing.json).

Files: the `origin` block in YAML front matter at
`$GRID_HOME/brain-proposals/<name>.md` or
`$GRID_HOME/projects/<project>/brain-proposals/<name>.md`, retained in accepted
`knowledge/<topic>/<name>.md` at the same level. With the Grid config's `brain` key
set the brain folder is an Obsidian vault: the level's proposals are in its
`Proposals/` folder (not read by the vault walk), and accepted notes are read
as `grid/<topic>/<name>.md` below the level root (the brain folder at home,
`projects/<project>/` in it at project level). A task's own `brain-proposals/`
folder keeps its name. Writer: `grid brain propose`
then `accept`, through `internal/braincmd/command.go:286-304` and
`internal/brain/store.go:227-228`. Readers: brain store/curator, Job source
resolver, app and eventual server sync. The schema validates the decoded block,
not the entire note. Spec: keeper-F1 (`findings-brain.md:62`), keeper-F8
(`findings-brain.md:69`), `brain-feed.md:15`, `job-contract.md:87`.

| Field | Rule and source | Spec doc:line |
| --- | --- | --- |
| `kind` | Required: `session`, `job`, `server` or `import`; never `body`. These are main's accepted spellings. | brain-feed.md:15; internal/brain/note.go:84-88 |
| `id` | Required nonblank single-line string; CR, LF and NUL are forbidden. The CLI makes a random session id. Other callers supply a Job or authenticated source id. | findings-brain.md:62; internal/brain/note.go:81; internal/brain/store.go:150-163 |
| `tier` | Retired (verdicts-11nov v-7). Not a field: the schema forbids it ([tier-key](fixtures/fmt-origin/invalid/tier-key.json)). A note on disk that still has `tier:` in its origin decodes with the key ignored; the next write drops it. | internal/brain/note.go:25-46; internal/brain/origin_test.go |
| Absent legacy block | Invalid: the schema rejects `null`, missing fields and an empty stamp. Upgrade must supply provenance before validation. | brain-feed.md:15; internal/brain/note.go:81-94 |

The origin names who wrote the note and nothing else: `kind` says which sort of
writer and `id` names it. The trust tier of a brain note is dropped
(verdicts-11nov v-7); the Feed record tier (`internal/contract/types.go:12-17`)
is a separate field and stays.

A valid-looking JSON stamp does not prove its provenance. The invalid
`body-tier.json` fixture breaks only the origin-kind rule. A forged block with
`kind: server` is structurally valid, so the capture test also supplies that
block through `--file -` and verifies that the actual writer replaces it with
`session` or `import`. Acceptance preserves the session stamp; imported
acceptance is refused and its body stays out of the index.

The old Markdown fixture has no origin and belongs to the `upgrade` set, never
`valid`. The documented legacy reader path is
`grid brain propose --level home --file <old-note.md> --external --json`:
`brain.Parse` reads the old note, the CLI supplies a fresh `import`
origin (`internal/braincmd/command.go:286-293`), and `Store.Propose` replaces
provenance before calling `Note.Validate` (`internal/brain/store.go:227-238`).
The capture test validates the stored upgraded stamp against the strict schema
and confirms that the body is preserved. No absent stamp passes the schema.
Direct `Note.Validate` and `Store.Show` reject the unstamped original; external
imports remain proposals and cannot be accepted as trusted knowledge.

| Status | Difference | Evidence |
| --- | --- | --- |
| diverges | The CLI's local session stamp is a fresh random id; this path does not derive a team/guest identity or active Job id from authenticated session/server state. The type permits those sources, but this CLI does not implement that integration. | findings-brain.md:62; brain-feed.md:15; internal/brain/store.go:150-163; internal/braincmd/command.go:286-293 |
| diverges | The spec says every entry has an origin; the parser still reads origin-less old notes outside schema validation. Explicit external import stamps them before validation; direct accepted-note reads exclude them. | brain-feed.md:15; internal/brain/markdown.go:24-39; internal/brain/topics.go:223-229 |
| diverges | Imported knowledge (origin kind `import`) is refused by the keeper and excluded by the accepted-note scan, rather than appearing as one labelled index line. | findings-brain.md:69; internal/brain/store.go:461-462; internal/brain/topics.go:227-229 |

## Task summary (summary.md)

Status: **written by the plugin, read by the app**. Path: `summary.md` in the task
folder, beside `task.md` and `history.md`. The task page shows one text per reader
level (owner, developer, customer); this file holds them. The only writer is
`grid task summary <task> --for owner|developer|customer --text <words>`; the
reader is the app's task view (`internal/slices/taskview/view_steps.go`, `summaries`).
There is no schema: the file is Markdown with exactly this shape.

```
## Owner

<owner text>

## Developer

<developer text>

## Customer

<customer text>
```

- One blank line between sections and one trailing newline. A fresh file holds the
  three headings in that order, the sections not yet written with empty bodies.
- A write replaces the body of one section and keeps every other byte of the file,
  including line ends and any line before the first heading.
- Text is trimmed and may span lines. A write is refused (exit 2, nothing written)
  for an unknown reader word (case-insensitive), empty text, text that fails the
  task text check (control, bidi override/isolate and noncharacter code points), and
  text with a line that starts with `## `, which the reader would take as a new section.
- Reader rules (the app): a line `## <Heading>` opens a section; the heading is
  lowercased and trimmed; CRLF is folded; each section is trimmed. `owner` overrides
  the owner text of the task, `developer` falls back to the owner text, and a
  `customer` with no section stays empty.
- Each write appends one history receipt (action `summary`, note `summary · <reader>`).
  The role rule is the one of `grid task rename`: an owner, a member or no role writes;
  any other role is refused with `scope: role cannot write task summaries`.
- The customer text is plain words with no Job ids; the owner text can name agents,
  which is why the reader never copies it into the customer level.
