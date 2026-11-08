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
is `grid sync`, invoked directly or by the worker; intended readers are the app's
sync slice, grid-server, another signed-in computer and the brain curator.
`go-architecture.md:13` makes the plugin the only sync client and says the server
never runs it. Sources: C8 (`findings-concept.md:185`), server-F1
(`findings-runtime.md:193`), `go-architecture.md:13,392`,
`brain-feed.md:285-289`, and `feed-contract.md:7-13`.

All field spellings and policy tokens below are **default, owner may change**:
the docs define the scope and authorities but do not supply a wire object.
`manifest_v` is fixed at 1. The object is a paid-account selection manifest,
with logical identities and classes rather than arbitrary paths or content.
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
| `machines[].id`, `machines[].person` | Logical machine id and its owning person; identifier character restrictions are defaults. | feed-contract.md:34; brain-feed.md:286 |
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
| `projects[].tasks[].machine`, `.gen` | Server binding `(task, machine, gen)`; nonnegative generation, 0 before binding. | feed-contract.md:13,49 |
| `classes` | Seven required data-class keys listed below. | findings-runtime.md:193; go-architecture.md:13 |
| `classes.<class>.authority` | Who originates or authorizes the class. Fixed tokens below. | findings-runtime.md:193; feed-contract.md:13; findings-brain.md:72 |
| `classes.<class>.server_holds` | Server's copy, or `nothing` for excluded classes. | findings-runtime.md:193; go-architecture.md:13 |
| `classes.<class>.readers` | Reader scope, not a self-issued permission. | findings-runtime.md:193; go-architecture.md:392; brain-feed.md:285-286 |
| `classes.<class>.conflict` | Fixed policy token; the three unresolved resolutions are defaults below. | findings-runtime.md:193; findings-concept.md:185; brain-feed.md:18,262,274 |

These are server-F1's five columns. Backticked values are the exact policy tokens
in the schema. Class scope references apply to each cell in that row.

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
normalized remote; the invalid secret fixture changes only the server's secret
holding policy. Both are rejected. Extra path or payload fields are also refused.

The eventual client/server must check account authentication, machine ownership,
unique ids, reference membership, approver roles, the task binding and cut-specific
access. JSON Schema cannot compare those identities or prove the absence of
secret data inside a selected file. The manifest does not grant authority by
asserting a role. Clock ordering and tie-breaking for last-writer-wins remain
with the sync owner; the preservation rule above is frozen as the default.

| Status | Difference or unresolved specification | Evidence |
| --- | --- | --- |
| diverges | A manifest is required by the spec, but there is no writer, reader, file path or sync transport in this checkout. This schema and its example path are proposals. | findings-runtime.md:193; go-architecture.md:13; internal/cli/cli.go:122 (`sync` has a nil handler) |
| diverges | Architecture's cut paragraph says personal notes never sync; its summary and the brain spec permit own-computer sync. This schema uses the latter and never exposes personal notes to project members. No writer resolves the discrepancy yet. | go-architecture.md:392 versus go-architecture.md:13 and brain-feed.md:286; internal/cli/cli.go:122 |

## Saved Job render

Schema: [fmt-job-render.json](schemas/fmt-job-render.json).
Fixtures: [full](fixtures/fmt-job-render/valid/full.json),
[elided](fixtures/fmt-job-render/valid/elided.json).

Files: `<task>/jobs/<job-id>/brief.md` and adjacent `inputs.json`. The same
immutable bundle contains `job.yaml`, which currently contains JSON Job metadata;
that Job record belongs to the Job contract, not this format's metadata object.
The writer is `grid job render`, through `internal/job/jobcmd/command.go:102-118`
and `internal/job/jobcmd/save.go:15-74`. Readers are `grid job show`, reviewers,
the app and replay/audit consumers. Spec: review-F1 (`findings-concept.md:144`),
`brain-feed.md:13,278-282`, and `job-contract.md:80-96`.

The on-disk representation has two files, so a JSON Schema cannot validate the
raw Markdown file and its sibling as one JSON document. Validation uses the
existing `job.Rendered` shape (`internal/job/render.go:30-33`):
`{"brief": <exact brief.md string>, "inputs": <decoded inputs.json>}`.
This wrapper is a lossless validation projection, **not a new saved file**.
The capture test reads the real files, keeps the complete `inputs.json` object
as `json.RawMessage`, and compares the result with the checked-in fixture.
Consumers must keep both files; validating `inputs.json` alone cannot establish
that the saved prompt exists. No hash is substituted for text.

| Field in validation projection | On-disk meaning and constraints | Writer and spec |
| --- | --- | --- |
| `brief` | Complete nonempty UTF-8 prompt, including Job front matter and all rendered sections; exact contents of `brief.md`. | internal/job/jobcmd/save.go:36; findings-concept.md:144 |
| `inputs` | Exact object stored as `inputs.json`; all five fields below are present. | internal/job/render.go:23-29; job-contract.md:80 |
| `inputs.generation` | String containing the project generation map, such as `{"project":1}`, or `content:<sha256>` when no project index generation exists. | internal/job/jobcmd/command.go:113-116,442-458; job-contract.md:80 |
| `inputs.sources` | Array of every resolved source, including candidates omitted from the prompt, sorted by pointer. | internal/job/render.go:41,281-289; findings-concept.md:144 |
| `inputs.not_rendered` | Sorted pointer strings omitted by the brain budget; never null. | internal/job/render.go:204-207; job-contract.md:96 |
| `inputs.brain_tokens` | Integer 0–2000; character-count estimate for the brain section, not a cap on the full prompt. | internal/job/render.go:208,290; job-contract.md:96,98 |
| `inputs.disableSkillShellExecution` | Boolean set when guest/external non-personal sources were resolved, even when a candidate was later omitted. | internal/job/render.go:63-68; job-contract.md:89 |
| `inputs.sources[].pointer` | Resolved source pointer, including any anchor or range. | internal/job/source.go:25; job-contract.md:80 |
| `.summary`, `.text` | Source summary and pinned text, not only a digest. Empty for personal pointer-only sources. | internal/job/source.go:26-27,161-170; findings-concept.md:144 |
| `.tier` | One of the four current trust tiers, taken from the source. | internal/job/source.go:28; job-contract.md:87 |
| `.sha256` | Digest string: full file bytes for files, encoded note for project brain; empty for personal pointers. | internal/job/source.go:29,176-181,237-238; findings-concept.md:144 |
| `.state`, `.topic`, `.updated` | Optional source lifecycle/topic/update strings; omitted when empty. `updated` is copied metadata, not constrained to a timestamp here. | internal/job/source.go:24,30-31,166,216-217; brain-feed.md:278 |
| `.links` | Optional array of source links; each has string `to`, `rel`, `why`. | internal/job/source.go:32,167-168; brain-feed.md:278 |
| `.pointer_only` | Optional boolean, emitted when true for home notes; text, summary, digest and links are not copied. | internal/job/source.go:33,161-164; brain-feed.md:285-286 |

The source fields follow the writer's shape, including empty text/digest for
personal pointers. The schema does not verify digests, pointer resolution or
front-matter semantics inside `brief`; Job lint and the source resolver own
those checks. The saved fixtures exercise real source snapshots and the elision
list. Invalid fixtures remove the text, empty it, or remove the input metadata,
one violation per fixture.

| Status | Difference | Evidence |
| --- | --- | --- |
| diverges | The full text and what-it-read list exist, but storage is keyed by Job id, not attempt. A changed render of the same id is refused; separate per-attempt snapshots are missing. | findings-concept.md:144; internal/job/jobcmd/save.go:26-36,50-54 |
| diverges | The spec asks for a pinned brain index generation; when no index exists the writer pins a digest of the source list instead. The schema preserves that string form. | job-contract.md:80; internal/job/jobcmd/command.go:113-116,442-458 |
| diverges | The complete what-it-read snapshot excludes personal note content and digests; it retains only a pointer, tier and state. This respects personal-note isolation. | findings-concept.md:144; brain-feed.md:285-286; internal/job/source.go:161-164 |

## Brain origin stamp

Schema: [fmt-origin.json](schemas/fmt-origin.json).
Fixtures: [session](fixtures/fmt-origin/valid/session.json),
[import](fixtures/fmt-origin/valid/import.json),
[old note](fixtures/fmt-origin/valid/legacy-note.md),
[absent old stamp](fixtures/fmt-origin/valid/legacy-absent.json).

Files: the `origin` block in YAML front matter at
`$GRID_HOME/brain-proposals/<name>.md` or
`$GRID_HOME/projects/<project>/brain-proposals/<name>.md`, retained in accepted
`knowledge/<topic>/<name>.md` at the same level. Writer: `grid brain propose`
then `accept`, through `internal/braincmd/command.go:286-304` and
`internal/brain/store.go:227-228`. Readers: brain store/curator, Job source
resolver, app and eventual server sync. The schema validates the decoded block,
not the entire note. Spec: keeper-F1 (`findings-brain.md:62`), keeper-F8
(`findings-brain.md:69`), `brain-feed.md:15`, `job-contract.md:87`.

| Field | Rule and source | Spec doc:line |
| --- | --- | --- |
| `kind` | Required: `session`, `job`, `server` or `import`; never `body`. These are main's accepted spellings. | brain-feed.md:15; internal/brain/note.go:84-88 |
| `id` | Required nonblank single-line string; CR, LF and NUL are forbidden. The CLI makes a random session id. Other callers supply a Job or authenticated source id. | findings-brain.md:62; internal/brain/note.go:81; internal/brain/store.go:150-163 |
| `tier` | Required: `person-team`, `person-guest`, `agent`, `external`. It is supplied independently of note text. | brain-feed.md:15; job-contract.md:87; internal/brain/note.go:89-94 |
| Absent legacy block | Represented by JSON `null` in this projection only; not a new stamp and never permission to admit unstamped knowledge. | brain-feed.md:15; internal/brain/markdown.go:24-39; internal/brain/topics.go:223-229 |

Main's constants win (`internal/contract/types.go:12-17`). The current
`brain-feed.md:15` and `job-contract.md:87` names map identically to them.
The older keeper finding uses different words:

| Older word in findings-brain.md:62 | Current tier | Mapping status |
| --- | --- | --- |
| member | `person-team` | default, owner may change |
| guest | `person-guest` | default, owner may change |
| external | `external` | exact |
| unknown | `external` for unknown imports | default, owner may change; never promote to member |
| No separate agent word | `agent` for local CLI sessions | main's constant; default, owner may change |

A valid-looking JSON stamp does not prove its provenance. The invalid
`body-tier.json` fixture breaks only the origin-kind rule. A forged block with
`kind: server` and `tier: person-team` is structurally valid, so the capture test
also supplies that block through `--file -` and verifies that the actual writer
replaces it with `session/agent` or `import/external`. Acceptance preserves the
session stamp; external acceptance is refused and its body stays out of the index.

The old Markdown fixture has no origin. `brain.Parse` and the CLI import path
still read it. `Note.Validate` rejects it and `Store.Show` excludes it from
accepted knowledge until provenance is stamped. The capture test checks all
three boundaries rather than treating successful parsing as acceptance.

| Status | Difference | Evidence |
| --- | --- | --- |
| diverges | The keeper finding's member/guest/external/unknown names are not the stored enum; the four current constants above win. | findings-brain.md:62 versus brain-feed.md:15; internal/contract/types.go:12-17 |
| diverges | The CLI's local session stamp is a fresh random id and always agent; this path does not derive a team/guest identity or active Job id from authenticated session/server state. The type permits those sources, but this CLI does not implement that integration. | findings-brain.md:62; brain-feed.md:15; internal/brain/store.go:150-163; internal/braincmd/command.go:286-293 |
| diverges | The spec says every entry has an origin; the parser still reads origin-less old notes. They are excluded from accepted knowledge, not silently upgraded. | brain-feed.md:15; internal/brain/markdown.go:24-39; internal/brain/topics.go:223-229 |
| diverges | Guest/external knowledge is refused by the keeper and excluded by the accepted-note scan, rather than appearing as one labelled index line. | findings-brain.md:69; internal/brain/store.go:461-462; internal/brain/topics.go:227-229 |
