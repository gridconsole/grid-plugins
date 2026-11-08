# Grid Feed and Job contract

Shared file formats use the `fmt-` schema prefix; see [formats.md](formats.md).

The Feed contract: schemas, fixtures and the conformance suite; contract/commands.json (each command, arguments, output schema) comes later.

These JSON Schema 2020-12 documents define the current wire format. Feed core
wins over older formats. All examples use synthetic identities. Sources below
are reference-document names and original line numbers, not machine paths.

| Type | Schema | Source doc:line |
| --- | --- | --- |
| Post envelope | [envelope.json](schemas/envelope.json) | feed-contract.md:36-90 |
| Job post | [job.json](schemas/job.json) | feed-contract.md:65; job-contract.md:5-75 |
| Job record | [job-record.json](schemas/job-record.json) | job-contract.md:5-76 |
| Control, including assign | [control.json](schemas/control.json) | feed-contract.md:175-198; old-plugin/feed/types/control.md:5-23 |
| Ack | [ack.json](schemas/ack.json) | feed-contract.md:184-186 |
| Submit | [submit.json](schemas/submit.json) | feed-contract.md:86; old-plugin/feed/types/submit.md:5-17 |
| Result v2 | [result.json](schemas/result.json) | feed-contract.md:84; job-contract.md:112-120 |
| Checkpoint | [checkpoint.json](schemas/checkpoint.json) | feed-contract.md:82; job-contract.md:141-144 |
| Question and gate | [question.json](schemas/question.json) | feed-contract.md:188-197; old-plugin/feed/types/question.md:5-19 |
| Answer | [answer.json](schemas/answer.json) | feed-contract.md:177; old-plugin/feed/types/answer.md:5-11 |
| Handover | [handover.json](schemas/handover.json) | feed-contract.md:118-122; old-plugin/feed/types/handover.md:5-11 |
| Worker exit | [worker-exit.json](schemas/worker-exit.json) | feed-contract.md:214-216 |
| Loop detected | [loop-detected.json](schemas/loop-detected.json) | feed-contract.md:235-245 |
| Post | [post.json](schemas/post.json) | feed-contract.md:66,90; old-plugin/feed/types/post.md:5-14 |
| Knowledge | [knowledge.json](schemas/knowledge.json) | feed-contract.md:66,90; old-plugin/feed/types/knowledge.md:5-15 |
| Found | [found.json](schemas/found.json) | feed-contract.md:80; job-contract.md:124 |
| Usage notice | [usage-notice.json](schemas/usage-notice.json) | feed-contract.md:113,293 |
| Head | [head.json](schemas/head.json) | feed-contract.md:86; old-plugin/feed/types/head.md:5-13 |
| Host | [host.json](schemas/host.json) | feed-contract.md:86; old-plugin/feed/types/host.md:5-15 |
| Transfer | [transfer.json](schemas/transfer.json) | feed-contract.md:86; old-plugin/feed/types/transfer.md:5-13 |
| Deploy failed | [deploy-failed.json](schemas/deploy-failed.json) | feed-contract.md:86; old-plugin/feed/types/deploy-failed.md:5-17 |
| Task, Job or merge lease | [lease.json](schemas/lease.json) | feed-contract.md:92-124 |
| Project merge lease | [merge-lease.json](schemas/merge-lease.json) | feed-contract.md:115; plugin-design.md:113 |
| Heartbeat file, not a post | [heartbeat.json](schemas/heartbeat.json) | feed-contract.md:200-218 |
| Takeover log entry | [takeover.json](schemas/takeover.json) | feed-contract.md:105-111 |
| Trust tier field | [trust-tier.json](schemas/trust-tier.json) | feed-contract.md:90; job-contract.md:87-91 |

Run `make test` and `make lint` from the repository root. `make test` includes
`go test ./...`. In a sandbox with a read-only default Go cache, prefix both
commands with `GOCACHE="$PWD/.contract-cache"`. To keep temporary test files
inside the checkout as well, first run `mkdir -p .contract-tmp`, then also set
`TMPDIR="$PWD/.contract-tmp"`. The existing module declaration and Go 1.27
version are unchanged. The test helper sets a Git discovery ceiling so fixtures
do not accidentally resolve to an ancestor checkout.

The canonical schemas are embedded by `contract/embed.go` and consumed by
`internal/contract`. Go embed patterns cannot reach parent directories, so the
asset package keeps a single source of truth instead of duplicating schemas.
`SchemaNames`, `Schema` and `Validate` expose these assets. Every schema has at
least two valid and two invalid JSON fixtures; invalid filenames name their
broken rule. The conformance tests walk both schemas and fixture directories.

`Fold([]Post) State` is a pure fold over one task. It does not read a clock.
The Feed reader must first check complete lines, writer filename identity and
end offsets, set `Post.Stale` for records beyond a takeover snapshot, and resolve
external bodies. Missing dependencies stay held; stale records still satisfy
presence dependencies. Duplicate IDs count once. Controls apply in generation,
fence and sequence order. Each assignment consumes only its granted Job tenure,
in sequence order, beginning with an ack that cites the assignment. Handover
announces a change but never grants authority. Results and exits are evidence;
explicit control posts make state transitions and new assignments make retries.
The fold also indexes counted posts for the Feed: every question with its answer
candidates and the answer a control applied, `needs_you` (open questions, not
those on cancelled Jobs), `wakes` (security findings on a known Job and every
loop-detected), `by_ref` (post IDs by ref), the newest task-lease `head` and the
newest `host` per worker, both ordered by generation, fence and sequence.
`TierFor`, `ValidWorker` and `PostTypes` give writers the envelope's tier, writer
id pattern and type registrations so no second copy is needed.

The scheduler, lease writer and renderer remain separate responsibilities. They
check pid/start-time liveness, unknown liveness, sleep/wake, crash-loop progress,
ready dependencies and cycles, gate timers/CI, review floors, usage holds, wake
idempotency, and filesystem/process durability. This package does not perform
those operations or claim the multiprocess spike fixtures pass. The fold accepts
already-authorized task tenures; schemas and stamped roles are not authentication.

Disagreements and defaults chosen:

- The request mentions creating a module and preserving it. The checkout already
  has the requested module and Go version; the existing `go.mod` is retained.
- `plugin-design.md:81` assigns schemas and conformance to `contract/`;
  `feed-contract.md:270` puts filesystem goldens in `feed/fixtures/`. This lane
  supplies record conformance fixtures in the requested `contract/fixtures/`.
  Filesystem and concurrency goldens belong to the Feed reader/writer work.
- The old map at `old-plugin-map.md:71-72` lists epoch-based files, claim and 13
  types; the old directory actually also has Job, submit and heartbeat files.
  Current Feed lines 59, 183, 202 and 266 remove epoch, claim and Feed heartbeat.
  No claim schema or heartbeat post is registered. Their fold scenarios show
  unknown legacy records without changing state. Heartbeat validates `.live`
  records instead. Assign is a control body, not a separate post type.
- Feed line 46 calls `author` a string, but line 90 requires `author.role`.
  Choose `{id, role}` and a required envelope `tier`. Roles are owner, member,
  lead, guest, agent, worker, reviewer, curator, host, cli and external. Owner,
  member and lead map to person-team; guest to person-guest; external to external;
  the remaining roles map to agent. These are CLI stamps, never writer choices.
  Role names and the tier field's wire location were otherwise unspecified.
- Each type has `x-class` and `x-roles` annotations plus enforced role constraints.
  The exact role matrix is absent from the current docs: lead, owner, member and
  cli may plan/control; worker, agent and cli may write leased Job evidence;
  reviewers may post found; host, cli and lead may post host; only cli posts
  usage notices. Guest/external authors may answer, post messages or submit
  knowledge marked `external: true`, never Jobs or proposed work. A guest answer
  also requires a question addressed to its author ID, checked by the fold.
  Human authorization and source provenance remain the CLI's responsibility.
- All envelope fields are required, including explicit nulls/empty arrays.
  Optional body fields remain optional; the envelope's all-fields rule does not
  retroactively require every optional field of the old body formats. Unknown
  fields fail known-version writes. Unknown types or versions are displayed with
  an unknown verdict and do not affect state. Result is v2, other types v1.
- ULIDs are uppercase canonical Crockford strings with the overflow bit checked.
  Task IDs are ULIDs, never display numbers. Worker IDs are `<member>@<machine
  ULID>`, split on the last `@`: the member is `git config user.email` until
  sign-in (feed-contract.md:34), so it may hold an `@` but no `/` or whitespace;
  old short host names are not accepted. Dates require UTC
  and exactly three fractional digits. Counters start at the documented bounds;
  body strings are nonempty unless the field is optional or nullable.
- Core line 86 leaves head/host/transfer/deploy-failed classes to their type files,
  whose old files have no class. Choose state, the documented default. Their
  old fields are retained; old time-based host selection, transfer authority and
  handover epoch behavior are not imported. Handover has optional `adopted` for
  `session-gone`. There is no automatic takeover from the age of any post.
- Unspecified old body details use arrays for changed/evidence/knowledge, question
  options and knowledge concepts. `post` messages require text, bounded at 4000
  characters; attachment paths are relative and reject parent traversal. Old
  priority, brief, next, epoch and heartbeat tail fields are not in the new wire.
- Control assign is exactly the current `{assign: {job, attempt, to, fence,
  until}}`. Other moves use `{target, status, reason}`, with optional attempt
  and close_reason. Target may be a Job ID or a Job-side post ID. A hold has
  target/reason only. Closing a question selects
  `{target: question-post-id, answer: answer-post-id, status: answered|closed}`;
  neither of two concurrent answers wins by time. `accepted` replaces old
  `done`, because new dependency rules name accepted. Cancelled/rejected require
  close_reason; a close never silently supplies one from a generic reason.
- Job-side evidence requires a matching Job lease, attempt, generation, worker
  and fence; ack must precede the evidence and name its assign. The fold consumes
  each assigned tenure before the next task control, retaining historical result
  IDs but preventing old attempts from changing the current assignment. Only
  the reader's snapshot decides which late lines are stale. Assign attempts must
  grow, except usage_limit may re-offer the same number with a fresh tenure.
  Exit alone does not manufacture a retry/control; a missing reason means crash.
  Loop increments review_rounds and requests a lead wake; usage_limit sets a hold
  reason. The scheduler decides retry limits and posts the resulting controls.
- Result arrival alone is not acceptance. Control posts decide acceptance,
  rejection and stale status. Proposed Jobs remain typed proposals with their
  discovered_from edge; the fold never turns a worker proposal into a Job.
  decisions_needed requests a lead wake. L1 verification requires the validating
  Job ID. `pass` accepts a boolean or pass/unsure because the docs use both pass
  as a field and pass/unsure as output. Dropped acceptance results require a
  decision. Proposal fields use title, role, why, gap and discovered_from; the
  repeated why in the prose is one field. Must-fix rows name an acceptance ID,
  write, verify or classify. Optional fields for detailed Verdict coverage and
  soft_fail are not specified; no new Verdict record type is invented here.
- Found security raises review immediately. The target level is unspecified;
  choose L3 and request a lead wake. Other findings change no Job state. Unknown
  found kinds fail current-version validation and remain visible as bad records.
- The Job record requires the planned and run fields shown in the example;
  created_at and status are optional because the example omits them. New posts
  start posted with no granted attempt; the plan's attempt does not grant a lease.
  Revisions use new Jobs. Worktree is a string, a nonempty array for multiple
  repos, or null for non-git work. `continues` follows the shown nullable Job ID,
  despite prose also describing a prior attempt. Review overrides have an
  optional `{level, reason}` field; authorization/floor computation is elsewhere.
- Job links allow the seven explicitly named writable relations; derived blocks
  is refused. The full 15-value brain vocabulary is outside these sources.
  `why` is 8-160 characters; target-copy and noun checks need semantic lint and
  are not guessed here. Keep/decisions are brain IDs; traces accepts item, owner,
  decisions paths or brain IDs. Command verify requires positive min_tests;
  file-exists uses path and reviewer-check uses job. Those two payload names are
  defaults because the docs specify the kinds but not their object fields.
- Gate metadata is optional on a question: id/kind plus at for timer, check for
  CI, job/state for Job gates, and optional default/at for person gates. Exact
  payload names were unspecified. The fold exposes unanswered questions and
  applies selected answers, but scheduling gate readiness needs external facts.
- Usage threshold has no documented units/range. Choose a numeric percentage
  from 0 through 100; dollars and minutes are nonnegative, exactly one is present,
  and the body task must equal the envelope task. Usage is chatter, never a cap.
- The task-local lease path table conflicts with the per-project merge rule.
  Merge leases explicitly require project and are stored by their caller at
  project scope. The merge wire format is otherwise unspecified: choose lease,
  project, holder, fence, gen, since and until, plus optional renewed_at. The
  docs say "without renewal" but give no renewal payload: choose renewed_at for
  an explicit renewal, never earlier than since. Until is exactly 15 minutes
  after the most recent renewal, or since if never renewed. There is no automatic
  renewal in this package; a released or expired valid lease is breakable by the
  next holder. A new grant needs a new fence. `lease.json`
  also validates the merge variant. Task leases have pid/pid_start while held;
  Job leases carry an attempt and null pid fields, with liveness in `.live`.
- Heartbeat until must equal beat_at plus ten minutes. The schema does not permit
  renewal through Feed posts. `Fold(posts)` has no heartbeat input and therefore
  cannot copy beat_at for display; a caller can overlay display-only liveness.
  Expired until, unknown pid checks and sleeping machines cannot make this fold
  infer stale. Only the holder's explicit control does that.
- JSON Schema cannot compare envelope/body identifiers or add durations. Go
  Validate additionally enforces these relationships and takeover filename
  identity. Standalone schema consumers must enforce these same semantic checks.
  The minimal validator supports only the used subset: type, enum, const,
  properties, required, additionalProperties, items, array/string/number bounds,
  uniqueItems, pattern, date-time, local file refs, allOf/anyOf/oneOf/not/if/then.
  It rejects unsupported schema keywords, never fetches refs, preserves integer
  precision and counts Unicode code points. It is not a general schema engine.

## Local state paths

Under the Grid home, `projects/<project>/`:

- `lease/merge.json` and `lease/seq`: the project merge lease and its fence
  counter. The lease is one per project (feed-contract.md:113), so its fence
  counter is per project too, not the task Feed's `lease/seq`. The lease
  package writes `{lease, holder, fence, since, until}` and does not yet carry
  `project`, `gen` or `renewed_at`, so it does not validate against
  [merge-lease.json](schemas/merge-lease.json); settle that shape before task
  and Job leases share the reader.
- `tasks/<task>/sessions/<machine>/<session>.json`: one session record per
  runtime session id and machine (plugin-design.md has no path entry for it).
- `tasks/<task>/feed/.live/<holder>.<fence>.json`: the Job liveness file
  (feed-contract.md:17 and :27), never shipped off the host.

Merge lease, session and task allocation code uses `flock`; on Windows it returns
"not supported until the Windows Feed is decided" (feed-contract.md:12).

## Command output

Plain output of `where` is one `name: value` line per field, with the same names as `--json`; a list gives one line per entry under the field's name, an empty field is left out, and paths under the home folder start with `~`; `--json` paths are absolute. `doctor` prints one `ok|fail <name>: <detail>` line per check. `version` prints `grid <version>`, `contract <n>` and `app_min <version>`. The golden files in `testdata/golden/` hold an example of each.

Every command except `help`, `version`, `doctor` and `migrate` first checks the Grid home's format version (`<GRID_HOME>/VERSION`). A newer format exits 1 with `This data is format <n>; this grid reads up to <m>. Update grid.`, an older one with `This data is format <n>; this grid reads <m>. Run: grid migrate`. A `VERSION` that cannot be read exits 1 with `Cannot read the format version in <GRID_HOME>/VERSION. Run: grid doctor`. A home without `VERSION` passes. `grid hook` prints nothing and exits 0 instead.

### `grid version --json`

    {"version": string, "contract": string, "app_min": string}

### `grid where --json [--task <path or id>] [--project <name or id>]`

    {
      "project":   string,    // project folder name under <GRID_HOME>/projects, or "personal" (tasks in <GRID_HOME>/personal/tasks)
      "id":        string,    // the record's id; "" without a record
      "rule":      "explicit" | "task" | "record" | "multi-repo" | "git" | "folder" | "personal",
      "folder":    string,    // the project folder on this machine, the current folder for Personal; "" when unknown
      "record":    bool,      // whether <GRID_HOME>/projects/<project>/project.md exists
      "repos":     [string],  // multi-repo: the repo folders found; else []
      "keys":      [string],  // match keys: common:<git dir>, remote:<normalised url>, root:<commit>
      "task":      string,    // the task folder name; "" outside a task
      "task_path": string     // the task folder path; "" outside a task
    }

Exit 1 with one line on stderr when the folder cannot be resolved (a task path outside the Grid home, an unknown project or task, a project.md that does not parse).

### `grid doctor --json`

    {"ok": bool, "checks": [{"name": string, "ok": bool, "detail": string}]}

Checks, in order: `grid home`, `grid home writable`, `format version`, `git`. Exit 1 when any check fails.

### `grid task new "<words>" [--project <name-or-id>] [--size <size>] [--workflow <name>] --json`

    {"id": string, "path": string}

The path is absolute. Plain output is two lines: `T-NN created`, then the task folder path, with the home spelled as `~`. The original words are written byte for byte to `intent.md`. Size is `auto` (default), `small`, `normal` or `large`. Workflow is a name, with an empty string meaning selection is left to the workflow rules. State starts as `queued`; creation does not start execution.

Project defaults to the existing folder resolver's answer. `--project` selects a registered name or id; `personal` and `Personal` select the Personal project without requiring a record. Personal tasks live under `<GRID_HOME>/personal/tasks/` and use `project: personal`. Other tasks live under `<GRID_HOME>/projects/<project>/tasks/`. A first write creates the project record, machine identity and format version as needed.

The title is the first nonblank line of the request, trimmed and limited to 120 Unicode characters. The folder slug uses lowercase ASCII letters and digits, separated by hyphens, limited to 60 characters, with `task` as the fallback. These transformations never change `intent.md`.

### `grid task show [T-NN] --json`

    {
      "id": string,
      "title": string,
      "project": string,
      "size": "auto" | "small" | "normal" | "large",
      "workflow": string,
      "state": string,
      "created": string,
      "path": string,
      "intent": string
    }

`created` is a UTC RFC3339 timestamp on creation; `path` is absolute. `intent` is the exact file content. The remaining fields come from YAML front matter in `task.md`. Show folds these files without changing them or inferring execution state. Plain output prints one `name: value` line per field, followed by `intent:` and its text. An omitted id selects the enclosing task; an explicit id is looked up across the Grid home, including Personal. Missing, duplicate or malformed tasks fail instead of returning a partial record.

### `grid task list [--project <name-or-id>] [--state <state>] --json`

Returns an array of the same records as `show`, sorted by numeric display id, or `[]` when none match. State filtering is exact and case sensitive. Plain output is one `T-NN state title` line per task, or no output for an empty list. Both read commands leave files unchanged and ignore hidden creation staging folders.

Task commands exit 0 on success, 2 for invalid flags or argument counts (including invalid creation size and empty words), and 1 for resolution, data or filesystem errors. Errors go to stderr without JSON on stdout. Flags may precede or follow the positional argument; use `--` before words that start with a hyphen.

### `grid hook <event>`

Reads the event JSON on stdin and always exits 0. `sessionstart` prints

    {"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": string}}

where `additionalContext` names the project or task `grid where` resolved for the event's `cwd`. Other events print nothing yet.

## Task intent files

Sources: go-architecture.md:178-199,273,372-384,596. Session actions and moves
record requests for a worker. The CLI never launches, stops, signals or moves an
agent, changes a task/session record, or writes a Feed post. With no worker,
requests remain pending. These types live in `internal/intent`, not in the Feed
record package.

Each request is one immutable `intents/<id>.json` under an existing task folder:

```json
{"id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","verb":"send","target":"session-example","args":{"text":"Continue the task"},"who":"member","created":"2016-07-30T23:54:10.259Z","idempotency_key":"<sha256-hex>"}
```

- `id`: canonical uppercase ULID (48-bit Unix milliseconds, 80 random bits).
- `verb`: `start`, `stop`, `send`, `pause`, `resume`, `handback`, `switch`, `move`.
- `target`: session identifier or task folder name for session actions; destination
  worker identifier for `move`. The containing task folder always scopes the request.
- `args`: string-valued object; `start` allows `agent`, `send` requires `text`,
  `switch` requires `agent`; all other verbs use `{}`. Unknown arguments fail.
- `who`: nonempty caller label, default `cli`; an audit label, not authentication.
- `created`: UTC RFC3339 time at millisecond precision (zero fractions may be omitted).
- `idempotency_key`: only on sends, SHA-256 of canonical JSON `[who,target,args]`,
  or `[who,target,explicit-key]` when supplied. The task folder scopes the key.
  The caller is part of both forms, so one caller's send never swallows another's.

A matching send in the five seconds after the original creation returns that
original request, including if already completed. Different text with the same
explicit key during that window fails. Different targets or callers never
coalesce. After five seconds a new request is written. Other verbs never coalesce.

Without a key, the same caller sending the same text to the same target twice
within five seconds gets one request, so a deliberate repeat ("yes", "go") is
dropped. The desktop therefore passes `--who <person>` and a fresh
`--idempotency-key <per-click id>` on every send click. A retried click reuses
its key and coalesces; a second click has a new key and is always written.

Worker lifecycle:

- `intents/.lock` is a permanent fslock lock inode. All readers and transitions
  lock it; never unlink or replace it. The shared filesystem must support
  distributed advisory flock and atomic exclusive creation. Requests and
  completions are synced before atomic no-replace publication by hard link.
  Where the filesystem refuses hard links (FAT, exFAT, some network mounts; the
  link fails with ENOTSUP, EOPNOTSUPP, ENOSYS or EPERM), publication falls back
  to an `O_EXCL` create, write and sync under the same lock. That path is not
  atomic: a crash mid-write can leave a short file, which a failed attempt
  removes but a crash cannot.
- Claim creates `<id>.claim` with `O_EXCL`, containing
  `{token,who,created,until}`. `token` is a fresh ULID; `until` is five minutes
  after claim creation. An existing unexpired marker rejects every other claimant.
- Renew extends a live claim: the holder passes its claim token and `until`
  becomes five minutes after the renew, keeping `token`, `who` and `created`.
  The marker is replaced atomically under the lock. A token that is not the
  current unexpired claim (expired, replaced by a second claimer, wrong intent)
  is refused with the same error as a stale done, and a completed request cannot
  be renewed. A worker that needs longer than five minutes renews before `until`;
  a worker that misses it loses the claim like any other.
- Expiry makes the request available again. Under the same lock, a new claimant
  removes the expired marker and exclusively creates its replacement. A crash
  after exclusive creation but before complete JSON leaves a marker that expires
  five minutes after its filesystem modification time. Workers need synchronized
  clocks and must stop acting when their claim expires.
- Done requires the current unexpired claim token. It publishes
  `<id>.done.json` with `{token,created,error?}`. `error` records a worker failure;
  a failed completion is terminal, not automatically retried. Repeating done with
  the same token and error succeeds; a stale token cannot complete a newer claim.
  The marker is retained as history. Claiming a completed request fails.
- Delivery is at least once after a crash. Workers must make external effects
  idempotent using the intent ID: a crash after an effect but before done can
  cause another delivery. Claim tokens fence completion, not external processes.
- Abandoned `.intent-*` temporary files are ignored. Symlinks at `intents`, lock,
  request, claim and completion paths are refused. Ancestors of the task path
  are resolved, so macOS temporary-directory aliases work.

`grid intent list --json` is an array sorted by ULID. Each entry contains the
request fields plus `state` (`pending`, `claimed`, `done`), `claim` for an active
complete claim, and `done` for a completion. A partial unexpired claim has state
`claimed` without a `claim` object. `--pending` returns only requests currently
available to claim, including expired claims. Empty output is `[]`.

The worker protocol runs on Linux and macOS. Windows builds, but the existing
fslock implementation refuses operations requiring locks there.

CLI syntax (positional arguments precede flags):

```text
grid session start [--agent <agent>]
grid session stop
grid session send <text> [--idempotency-key <key>]
grid session pause
grid session resume
grid session handback
grid session switch <agent>
grid task move <machine>
grid intent list [--json] [--pending]
grid intent claim <id> [--who <worker>] [--json]
grid intent renew <id> [--token <token>] [--json]
grid intent done <id> [--token <token>] [--error[=message]] [--json]
```

All commands accept `--task <path-or-id>` and `--project <name-or-id>`; by default
scope is the current task. There is no implicit new task. Session writes also
accept `--target <session>` (default: task folder name), `--who <caller>` (default:
`cli`) and `--json`. `send --text <text>` handles text beginning with a dash;
`switch --agent <agent>` is equivalent to the positional form. Move accepts
`--who` and `--json`, plus the architecture's alternate spelling
`grid task move <task> --to <machine>` or `grid task move --to <machine>`.
Move verifies every configured repository in the project's local folders has a
Git remote, refusing without writing anything if code cannot reach the worker.
It does not contact that remote or validate destination availability.

A successful write prints the request with `--json`, otherwise one line naming
its ID, verb and target. Claim prints `{token,who,created,until}` with `--json`,
otherwise one line naming the intent, token and expiry. Done prints its completion
object with `--json`, otherwise one line naming the intent and done state. Renew
prints the renewed claim like claim does. Done and renew read the token from
`--token`, its alias `--claim`, or `GRID_INTENT_CLAIM`; a token is required even
though the environment permits the short `grid intent done <id>` spelling.
A bare `--error` records `worker reported an error`; `--error=message` supplies
the reason. Exit codes are 0 for success, 1 for storage/scope errors or a claim
refusal, and 2 for invalid command usage. Existing session list/show behavior is
preserved. These new commands define no task or Feed record shapes.

`grid feed` and `grid job post|ack|result|cancel` exit with 0 for success, 1 for storage/scope/resolution or other errors, 2 for usage, 3 for final rejection (stale lease or invalid record; do not retry), and 4 for disk full.
