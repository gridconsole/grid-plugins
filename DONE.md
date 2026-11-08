# Lane L27

Implemented and committed on the current branch:

- `grid brain decide <task> <proposal-id> accept|reject|merge-into [<note-id>] [--note <reason>] [--json]`. Accept and reject default the destination id to the proposal name; merge requires an existing destination. Fully qualified ids use `brain:home/<name>` or `brain:project/<name>`. Decisions retain the task proposal with terminal status, publish or merge a linked knowledge note, update the brain log and indexes, and record one history line. A durable receipt recovers interrupted publication, refuses conflicting decisions, and prevents duplicate notes, merged text, history lines, and index generations.
- `grid project-md get|set <project> <key> [value] [--json]`. Project names and ids resolve through `internal/project`; the same package validates identity fields. Values are YAML scalars, lists, or mappings; omit the set value for an empty string. Only a top-level front-matter key changes. Other keys, YAML comments, and Markdown body content remain; front-matter line endings normalize to LF, while body line endings remain unchanged. Missing get keys fail. Invalid YAML, non-string mapping keys, non-finite JSON values, and invalid identity field types fail before writing.
- `grid land <task> [--dry-run] [--json]`. Reads and validates exactly one `lands` mapping across the project's `knowledge/deploy/*.md` entries, excluding `TOPIC.md`. Dry-run reads only. Execution requires a team approval for the exact current `plan.hash` value, uses the project's folder on the current machine, invokes only the configured command, bounds execution to ten minutes, and records start and outcome in `history.md`. Failure and timeout return a nonzero exit code. It performs no implicit git operations.

Shared code was reused: task resolution and reads use `internal/task` (and its `gridhome.TasksDirs` lookup); locks use `internal/fslock`; existing brain YAML and atomic file helpers moved into `internal/mdfile` and `internal/safefile`; process execution reuses the existing eval runner, including bounded output and process-group termination. No protected package API changed. The shared CLI file changed only in its command table. No Feed records or replacement task record shapes were introduced; trust values use `contract.TrustTier`.

## Deploy and approval formats

A project deploy entry, for example `knowledge/deploy/lands.md`:

```yaml
---
lands:
  target: staging
  branch: main
  command: make deploy
  needs-approve: true
---
Deployment notes belong here.
```

All four fields are required. `target`, `branch`, and `command` must be nonempty strings on one line; `needs-approve` is a boolean. Unknown lands fields, malformed entries, or multiple lands mappings fail. The command is interpreted by `sh -c` on macOS/Linux. The command receives `GRID_LAND_TASK`, `GRID_LAND_TARGET`, `GRID_LAND_BRANCH`, and `GRID_PLAN_HASH`; it decides how to use the branch and target. The CLI does not interpolate them into command text or check out a branch.

The person/app gate writer supplies `<task>/approvals/land.json`:

```json
{"plan_hash":"opaque-plan-revision","approved":true,"tier":"person-team"}
```

This is a local approval record, not a Feed envelope. `land` never creates or modifies approvals. The plan hash is opaque: only its final line ending is removed; no hashing, decoding, or revision parsing occurs. Agent, guest, missing, denied, and stale approvals fail. `needs-approve: false` does not bypass this task approval requirement. Symlinks in deploy and approval paths are refused. Approval authorization relies on the person/app writer controlling this record, just as other local gate files rely on their writer boundary.

## Validation

Run from the repository root, leaving TMPDIR at its default:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

Focused tests:

```sh
go test ./internal/braincmd ./internal/land ./internal/cli
```

Tests cover table-driven accept/reject/merge, repeated decisions, interrupted-history recovery, unchanged index generations on retries, conflicting decisions, keeper gates, project key edits and preservation, JSON goldens, read-only land dry-run, opaque hashes, approval refusals, symlink refusal, command failure, timeout, environment values, and invalid deploy configurations. All required checks pass on the Linux runner; macOS and Windows were cross-compiled, not executed here.

## Intended differences from the old plugin

References below are relative to the supplied read-only reference tree.

| Difference | Reference and reason |
| --- | --- |
| Mechanical decision writes are a Go CLI operation instead of manual curator file edits; no person prompt is added. | `old-plugin/agents/grid-curator.md:16`, `:20`, `:23`; `docs/brain-feed.md:303`, `:304`, `:313`; `docs/migration.md:67`. |
| Rejection is `status: rejected`, using the existing Go brain schema, rather than the old curator's `declined`; accept and reject retain the proposal for audit. | `old-plugin/agents/grid-curator.md:17`, `:23`; `docs/brain-feed.md:365`; existing `internal/brain.Note` lifecycle. |
| Accepted notes use topic directories, canonical level ids, structured existing links, and a Markdown source-task link; there is no legacy decision-date or skills/SKILL.md routing. | `old-plugin/agents/grid-curator.md:20`, `:23`; `docs/brain-feed.md:244`, `:263`, `:274`, `:285`. |
| Merge deterministically appends the curated proposal text and links to the destination rather than asking a model to rewrite that destination; keeper validation runs again on the combined note. | `old-plugin/agents/grid-curator.md:17`; `docs/brain-feed.md:313` assigns the mechanical writes to the CLI. The curator supplies the finished proposal text. |
| Trust, secret, size, and config-topic gates follow the existing brain store; unknown-origin, guest, and external proposals cannot be accepted without the separate provenance workflow. | `docs/brain-feed.md:15`, `:16`, `:263`, `:266`; `docs/feed-contract.md:90`. |
| Durable receipts, locked history writes, and atomic publication make retries safe; timestamps are generated UTC values instead of copied hook timestamps. Brain history events are `brain decided` with the explicit action and destination. | `old-plugin/agents/grid-curator.md:23`; `docs/brain-feed.md:279`, `:365`; `old-plugin-map.md:69`. |
| project-md edits arbitrary top-level front-matter keys and preserves the body; it does not parse the legacy Environments body section or translate “Commit and push”. | `old-plugin/skills/deliver/scripts/project-md.mjs:31`, `:109`; `docs/plugin.md:285`; `docs/migration.md:67`. The requested key interface replaces the old landing/environments reader. |
| Landing configuration comes from the deploy topic, not project.md's legacy landing labels; missing or invalid configuration fails instead of asking or choosing a local fallback when no remote exists. | `old-plugin/skills/deliver/scripts/land-plan.mjs:17`; `old-plugin/skills/deliver/SKILL.md:55`, `:61`; `docs/brain-feed.md:251`, `:292`. |
| Landing executes a configured shell command, not a hard-coded merge/push action or the old deploy Job argv array. The ten-minute limit and bounded output remain; branch and target are command inputs. | `old-plugin/skills/deliver/SKILL.md:62`; `old-plugin/feed/deploy.mjs:3`, `:15`, `:54`; `docs/brain-feed.md:251`, `:292`. The requested lands command is the sole authority for git or deployment actions. |
| Every actual land needs an approval matching the current opaque plan hash, even if the topic says needs-approve false; legacy history text cannot approve it. | `old-plugin/skills/deliver/SKILL.md:12`, `:54`; `docs/plugin-design.md:92`, `:166`; this lane's explicit approval requirement. The local record shape above is an integration seam, not a claimed spec schema. |
| Land records command start/result and propagates failures; it does not reopen task.md, emit deploy-failed Feed posts, revert production, fix forward, or generate a task concept note. | `old-plugin/feed/deploy.mjs:4`, `:75`; `old-plugin/skills/deliver/scripts/land-note.mjs:1`, `:2`; `docs/migration.md:67`. Those orchestrator/deliver operations are outside this command slice. |

## Limitations and unavailable work

- No Rust `writes_brain.rs` or `write/decide.rs` was present in the reference tree, so no Rust recording parity could be run. The available JS, curator instructions, and new design supplied the references.
- This checkout has no implemented approval command or gate reader and no canonical land approval schema. The documented `approvals/land.json` reader is ready for that writer to integrate; this lane does not grant approvals or invent Feed posts. The deploy config writer is also outside this slice.
- `internal/project.Write` serializes only identity fields and discards unknown fields and the body. With its API frozen, project-md uses `project.Load` and `project.Parse`, then the shared atomic writer to preserve the full document. It does not introduce another front-matter parser or change the project API.
- Windows compiles, but mutating commands retain `internal/fslock`'s explicit unsupported-platform error. macOS/Linux locking and process-group behavior use the existing build-tagged implementations. Runtime macOS validation still needs a macOS runner.
- Automatic curator batching, proposal production changes, provenance promotion, Feed notifications, deployment rollback, and land-note generation remain with their owning commands. A killed land process may leave a start line without a result; land cannot guarantee exactly-once execution of an arbitrary external command, and execution retries are deliberate user actions.
