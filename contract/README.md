# Grid Feed and Job contract

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
  Task IDs are ULIDs, never display numbers. Worker IDs use a placeholder member
  and a machine ULID; old short host names are not accepted. Dates require UTC
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
  next lander. A new grant needs a new fence. `lease.json`
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

## Command output

Plain output of `where` is one `name: value` line per field, with the same names as `--json`; a list gives one line per entry under the field's name, an empty field is left out, and paths under the home folder start with `~`; `--json` paths are absolute. `doctor` prints one `ok|fail <name>: <detail>` line per check. `version` prints `grid <version>`, `contract <n>` and `app_min <version>`. The golden files in `testdata/golden/` hold an example of each.

Every command except `help`, `version`, `doctor` and `migrate` first checks the Grid home's format version (`<GRID_HOME>/VERSION`). A newer format exits 1 with `This data is format <n>; this grid reads up to <m>. Update grid.`, an older one with `This data is format <n>; this grid reads <m>. Run: grid migrate`. A `VERSION` that cannot be read exits 1 with `Cannot read the format version in <GRID_HOME>/VERSION. Run: grid doctor`. A home without `VERSION` passes. `grid hook` prints nothing and exits 0 instead.

### `grid version --json`

    {"version": string, "contract": string, "app_min": string}

### `grid where --json [--task <path or id>] [--project <name or id>]`

    {
      "project":   string,    // project folder name under <GRID_HOME>/projects, or "Personal"
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

### `grid hook <event>`

Reads the event JSON on stdin and always exits 0. `sessionstart` prints

    {"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": string}}

where `additionalContext` names the project or task `grid where` resolved for the event's `cwd`. Other events print nothing yet.
