# Grid Feed and Job contract

Shared file formats use the `fmt-` schema prefix; see [formats.md](formats.md).

The Feed contract: schemas, fixtures and the conformance suite.

## CLI JSON failures

With `--json`, a failure is written to **stdout** in one of four shapes; the human
diagnostic stays byte-for-byte identical on stderr and exit statuses are unchanged.
Read `code` (or `error.code` on the envelope) to classify a failure; never classify
by message wording. Each example below is read by `TestHfFsShapes`.

1. **Envelope**, for a failure that had no JSON output and has a known code, or exits 2
   (then `invalid_argument`). `message` is stderr without its final newline, including
   any command prefix. The exit status is the command's own (here 1).

<!-- shape:envelope -->
```json
{"error":{"code":"no_such_task","message":"grid task show: task T-99 not found in the Grid home"}}
```

   A guest's refused write (`setup`, `routine`, `project relink`/`remove`, `agent sync`, and the task, session and question writes a guest may not make) prints this envelope with `guest_refused`; its exit stays exit 1, or exit 3 for `question withdraw`, `task delete` and `task restore`.

2. **Signal object**, printed by `signal act`, `signal dismiss` and `signal move`.
   `error` is a category string and `message` is unprefixed and sometimes redacted. The
   category `refused` exits 3 and always carries a `code`: `no_such_project`,
   `no_such_note`, `no_such_task`, `guest_refused`, `note_state` (the note's status
   does not allow the act or move, such as "this note is not snoozed"), `note_exists`
   or `notes_full`. The category `invalid` exits 2 and always carries
   `code: "invalid_argument"`.

<!-- shape:signal-refused -->
```json
{"error":"refused","message":"no such note","code":"no_such_note"}
```

<!-- shape:signal-invalid -->
```json
{"error":"invalid","message":"move requires --to <project|->","code":"invalid_argument"}
```

3. **Person gate object**, printed by every person-gate refusal and by `person check`;
   it exits 3. `reason` is the specific gate refusal reason and `message` matches
   stderr without its final newline; any decision fields of the gate come with them.

<!-- shape:person-gate -->
```json
{"error":"person_required","reason":"agent_role","hint":"approvals need a person; this caller is an agent","code":"person_required","message":"grid person check: approvals need a person; this caller is an agent (reason: agent_role)."}
```

4. **job_waiting object**, printed by `grid approve --json` and by
   `grid job result <job> --json` (with or without `--plan-hash`) on a person Job that
   still waits for a Job in its after list. It exits 3; `waiting_for` is the Job it
   waits on, `reason` is the wait reason unchanged, and stderr keeps its
   `grid approve: <job> cannot be answered yet: <reason>` line.

<!-- shape:job-waiting -->
```json
{"error":"job_waiting","code":"job_waiting","job":"J-04","waiting_for":"J-03","reason":"waiting for J-03"}
```

Other failures print nothing on stdout: with `--json`, an unclassified failure that
exits 1 or 3 and is covered by none of the four shapes prints nothing on stdout, and
only stderr carries it (`grid task show` with no operand outside a task folder is one).
`job lint`, `job render`, `job show`, `job verify` and `job trace` keep their own objects (`ok`, `findings`, `error`; trace: `rows`, `problems`) and gain the top-level `code` on a classified refusal; a success and an unclassified failure print no `code`. Other commands that keep their own older JSON objects on failure carry no code.
A partial success is the exception: when `grid brain revert --json` or
`grid brain supersede --json` has written its change but rebuilding index.md failed,
stdout holds the result object (revert's also gains `code`), stderr holds
`{"error":"<message>","code":"index_rebuild_failed"}`, and the exit is 1.
Plain output, successful JSON and streaming output are unchanged. A code does not
imply an exit status: a missing project during `signal move` exits 3, a missing task
during `task show` exits 1.

| Stable code | Meaning |
| --- | --- |
| `no_such_project` | A requested project is missing or inaccessible. |
| `no_such_task` | A requested task is missing or inaccessible. |
| `no_such_note` | A signal/inbox note does not exist. |
| `invalid_argument` | Invalid arguments or a usage/validation failure. |
| `invalid_policy` | A workflow policy value the command depends on is invalid (signal move to the inbox: `inbox_workspace`); the object also carries `key` and exits 1. |
| `workflow_file_changed` | A workflow file changed since its revision was read (`grid workflow write`); nothing was written and it exits 3. |
| `record_id_used` | A record of another writer tenure on the Feed already has the record id of the post (`grid feed post`, `grid job post` with `--id`); nothing was written and it exits 3. A repeat of the same post by the same writer tenure still returns the stored record. |
| `note_state` | A signal note's status does not allow the act or move (not snoozed, not decided, already decided); nothing was written and it exits 3. |
| `note_exists` | A signal move found the note already in the target project; nothing was written and it exits 3. |
| `notes_full` | A signal move found the target notes folder already holding 4000 files; nothing was written and it exits 3. |
| `guest_refused` | The caller is refused as a guest (role guest, external or an unknown role); nothing was written. |
| `job_exists` | A Job post (`grid job post`, `grid feed post --type job`) named a Job id the Feed already holds; nothing was written. A repeat of the same request (same `--id`, same body) exits 0 instead. Exits 3. |
| `task_state` | The task's state does not allow the move (`task stop`, `task close`, `task reopen`, `task status`, and `deliver`, which the same state table can refuse); nothing was written and it exits 1. |
| `person_required` | The existing person gate refused the request. |
| `job_waiting` | A person Job still waits for a Job in its after list (`grid approve`, `grid job result`); exits 3. |
| `changed` | A settings file or preview changed since the person looked; nothing was written. |
| `invalid_settings` | A settings value, key, preset or file is invalid; nothing was written. |
| `write_failed` | Writing a settings file failed. |
| `unconfirmed_key` | A Codex settings key is read-only until its name is checked on a real install. |
| `nothing_to_undo` | An undo of an agent budget Apply found no Apply to undo. |
| `index_rebuild_failed` | The change was written; rebuilding index.md failed; the stdout object holds the result (`grid brain revert`, `grid brain supersede`); exits 1. |

The first thirteen constants live in `internal/cli/error_codes.go`, the settings
codes in `internal/settings/claude_user.go` and `index_rebuild_failed` in
`internal/braincmd/changes.go`; `person_required` and `job_waiting` are
written as literals in `internal/cli/role_policy.go`, `internal/cli/person_check.go`
and `internal/cli/task_write.go`. `TestHfFsCodeTable` keeps this table equal to that set.

## Command inventory

[commands.json](commands.json) publishes the command table, summaries, build status,
positional arguments, parsed flags and JSON support under the version returned
by `grid version --json`. An empty subcommand name means the command itself;
`<arg>` is required and `[arg]` is optional. Output names resolve under
`schemas/`; an empty output means this lane has not published a schema for
that invocation. App output schemas use `cmd-<command>[-<subcommand>].json`
and have no Feed post registration metadata. When adding a command, update
the inventory from its parser, add or revise its output schema and at least
two captured valid and two invalid fixtures, and extend
`TestContractCommandsApp` in `internal/cli/contract_commands_test.go`.
Validate live stdout before normalizing fixtures; fixed objects reject
unknown fields. See [L47 notes](../docs/lanes/L47.md) for parser conventions,
streaming behavior, fixture capture and current output oddities.

Regenerate the whole inventory after parser changes:

```sh
UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1
```

The generator reads command dispatch expressions and probes the real parsers
with deliberately unknown sentinel flags to discover registered flags and
whether they take values, stopping before command execution.
`TestContractCommandsParserInventory` compares the complete generated document
with the embedded inventory. Operand labels, required-flag semantics and output
schema links are reviewed annotations in `contract_inventory_test.go`; new or
removed parser routes require those annotations to be reconciled. Flags are
listed as parsed, including shared parser flags whose use is restricted later
(for example, Job `--dir` is usable only with `verify`).

Refresh captured output fixtures with:

```sh
UPDATE_CONTRACT_COMMAND_FIXTURES=1 go test ./internal/cli -run '^TestContractCommandsApp$' -count=1
go test ./internal/contract
```

`stream: true` describes an NDJSON line shape. `job next --json`, including
`job next --wait --json`, prints one line and exits; it is not a persistent
subscription. The app must invoke it again for the next result. The `--wait`
form can wait before emitting its single result.

These JSON Schema 2020-12 documents define the current wire format. Feed core
wins over older formats. All examples use synthetic identities. Sources below
are reference-document names and original line numbers, not machine paths.

| Type | Schema | Source doc:line |
| --- | --- | --- |
| Post envelope | [envelope.json](schemas/envelope.json) | feed-contract.md:36-90 |
| Job post | [job.json](schemas/job.json) | feed-contract.md:65; job-contract.md:5-75 |
| Job record | [job-record.json](schemas/job-record.json) | job-contract.md:5-76 |
| Control, including assign; a move may carry optional `send_back` and `sent_back_hash` | [control.json](schemas/control.json) | feed-contract.md:175-198; old-plugin/feed/types/control.md:5-23 |
| Ack | [ack.json](schemas/ack.json) | feed-contract.md:184-186 |
| Submit | [submit.json](schemas/submit.json) | feed-contract.md:86; old-plugin/feed/types/submit.md:5-17 |
| Result v2 | [result.json](schemas/result.json) | feed-contract.md:84; job-contract.md:112-120 |
| Checkpoint | [checkpoint.json](schemas/checkpoint.json) | feed-contract.md:82; job-contract.md:141-144 |
| Question and gate | [question.json](schemas/question.json) | feed-contract.md:188-197; old-plugin/feed/types/question.md:5-19 |
| Task question file | [question-file.json](schemas/question-file.json) | internal/question/question.go:27; internal/question/files.go:145 |
| Answer; may carry optional `send_back` and `sent_back_hash` | [answer.json](schemas/answer.json) | feed-contract.md:177; old-plugin/feed/types/answer.md:5-11 |
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
| Task or Job lease | [lease.json](schemas/lease.json) | feed-contract.md:92-124 |
| Heartbeat file, not a post | [heartbeat.json](schemas/heartbeat.json) | feed-contract.md:200-218 |
| Takeover log entry | [takeover.json](schemas/takeover.json) | feed-contract.md:105-111 |
| Trust tier field | [trust-tier.json](schemas/trust-tier.json) | feed-contract.md:90; job-contract.md:87-91 |
| grid account folder output | [cmd-account-folder.json](schemas/cmd-account-folder.json) | internal/cli/account.go:143 |
| grid account scan output | [cmd-account-scan.json](schemas/cmd-account-scan.json) | internal/cli/account.go:112 |
| grid account status output; also grid account list, the same writer | [cmd-account-status.json](schemas/cmd-account-status.json) | internal/cli/account.go:87 |
| grid account statusline output | [cmd-account-statusline.json](schemas/cmd-account-statusline.json) | internal/cli/account.go:250 |
| grid account windows output | [cmd-account-windows.json](schemas/cmd-account-windows.json) | internal/cli/account.go:165 |
| grid agent budget show output | [cmd-agent-budget-show.json](schemas/cmd-agent-budget-show.json) | internal/cli/budget.go:83 |
| grid approve output | [cmd-approve.json](schemas/cmd-approve.json) | internal/cli/task_write.go:164 |
| grid brain changes output | [cmd-brain-changes.json](schemas/cmd-brain-changes.json) | internal/braincmd/changes.go:188 |
| grid brain drop output | [cmd-brain-drop.json](schemas/cmd-brain-drop.json) | internal/braincmd/hy_drop.go:127 |
| grid brain search output | [cmd-brain-search.json](schemas/cmd-brain-search.json) | internal/braincmd/command.go:345 |
| grid brain write output | [cmd-brain-write.json](schemas/cmd-brain-write.json) | internal/braincmd/ke_write.go:147 |
| grid claude settings set output | [cmd-claude-settings-set.json](schemas/cmd-claude-settings-set.json) | internal/cli/budget.go:143 |
| grid claude settings show output | [cmd-claude-settings-show.json](schemas/cmd-claude-settings-show.json) | internal/cli/budget.go:133 |
| grid feed post output | [cmd-feed-post.json](schemas/cmd-feed-post.json) | internal/cli/feed.go:168 |
| grid feed state output | [cmd-feed-state.json](schemas/cmd-feed-state.json) | internal/cli/feed.go:131 |
| grid intent list output | [cmd-intent-list.json](schemas/cmd-intent-list.json) | internal/cli/intent.go:247 |
| grid job next output | [cmd-job-next.json](schemas/cmd-job-next.json) | internal/cli/orchestrator.go:175 and :228 |
| grid lease status output | [cmd-lease-status.json](schemas/cmd-lease-status.json) | internal/cli/task_lease.go:146 |
| grid plan hash output | [cmd-plan-hash.json](schemas/cmd-plan-hash.json) | internal/cli/plan.go:74 |
| grid project add output | [cmd-project-add.json](schemas/cmd-project-add.json) | internal/cli/projects.go:172 |
| grid project env output | [cmd-project-env.json](schemas/cmd-project-env.json) | internal/cli/ln_project_env.go:184, :268 |
| grid project-md set output | [cmd-project-md-set.json](schemas/cmd-project-md-set.json) | internal/cli/projectmd.go:41 |
| grid project remove output | [cmd-project-remove.json](schemas/cmd-project-remove.json) | internal/cli/projects.go:172 |
| grid project repo output | [cmd-project-repo.json](schemas/cmd-project-repo.json) | internal/cli/project_repo.go:110 |
| grid project scan output | [cmd-project-scan.json](schemas/cmd-project-scan.json) | internal/cli/projects.go:143 |
| grid routine add output | [cmd-routine-add.json](schemas/cmd-routine-add.json) | internal/cli/routine.go:158 |
| grid routine list output | [cmd-routine-list.json](schemas/cmd-routine-list.json) | internal/cli/routine.go:120 |
| grid session handback output | [cmd-session-handback.json](schemas/cmd-session-handback.json) | internal/cli/intent.go:140 |
| grid session list output | [cmd-session-list.json](schemas/cmd-session-list.json) | internal/cli/liveness.go:142 |
| grid session pause output | [cmd-session-pause.json](schemas/cmd-session-pause.json) | internal/cli/intent.go:140 |
| grid session resume output | [cmd-session-resume.json](schemas/cmd-session-resume.json) | internal/cli/intent.go:140 |
| grid session send output | [cmd-session-send.json](schemas/cmd-session-send.json) | internal/cli/intent.go:140 |
| grid session show output | [cmd-session-show.json](schemas/cmd-session-show.json) | internal/cli/liveness.go:162 |
| grid session start output | [cmd-session-start.json](schemas/cmd-session-start.json) | internal/cli/intent.go:140 |
| grid session stop output | [cmd-session-stop.json](schemas/cmd-session-stop.json) | internal/cli/intent.go:140 |
| grid session switch output | [cmd-session-switch.json](schemas/cmd-session-switch.json) | internal/cli/intent.go:140 |
| grid setup output | [cmd-setup.json](schemas/cmd-setup.json) | internal/cli/settings.go:257; internal/settings/claude.go:306 |
| grid skill enable and agent set output | [cmd-settings-receipt.json](schemas/cmd-settings-receipt.json) | internal/cli/settings.go:169 |
| grid skill list output | [cmd-skill-list.json](schemas/cmd-skill-list.json) | internal/cli/settings.go:137 |
| grid task list output | [cmd-task-list.json](schemas/cmd-task-list.json) | internal/cli/task.go:142 |
| grid task move output | [cmd-task-move.json](schemas/cmd-task-move.json) | internal/cli/intent.go:140 |
| grid task new output | [cmd-task-new.json](schemas/cmd-task-new.json) | internal/cli/task.go:189 |
| grid task show output | [cmd-task-show.json](schemas/cmd-task-show.json) | internal/cli/task.go:122 |
| grid task delete and restore output | [cmd-task-trash.json](schemas/cmd-task-trash.json) | internal/cli/task_write.go:438 |
| grid version output | [cmd-version.json](schemas/cmd-version.json) | internal/cli/version.go:54 |
| grid workflow show output | [cmd-workflow-show.json](schemas/cmd-workflow-show.json) | internal/cli/workflow.go:74 |
| grid workflow write output | [cmd-workflow-write.json](schemas/cmd-workflow-write.json) | internal/cli/workflow.go:176 |
| grid where output | [cmd-where.json](schemas/cmd-where.json) | internal/cli/cli.go:407 |
| grid worker list output | [cmd-worker-list.json](schemas/cmd-worker-list.json) | internal/cli/worker.go:57 |

Run `make test` and `make lint` from the repository root. Keep the default
TMPDIR so the test runner can check for leaked temporary folders. Focused
checks are `go test ./internal/cli -run ContractCommands -v` and
`go test ./internal/contract/...`. The existing module declaration is unchanged.
The test helper sets a Git discovery ceiling so fixtures do not resolve to
an ancestor checkout.

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
ready dependencies and cycles, gate timers/CI, usage holds, wake
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
  reviewer, lead, cli, worker and agent may post found; host, cli and lead may post host; only cli posts
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
  neither of two concurrent answers wins by time. A person may also close an open
  person question with `grid question withdraw <Q-id> [--task <folder|id>]
  [--reason <text>] [--json]`. It posts a leaseless control referencing the
  question with `{target, question_event: "withdrawn", text}`; the reason defaults
  to `No longer needed`. Owners and members are allowed by default. The task snapshot's
  `workflow.md` YAML front matter key `gate_post_roles: {withdraw: [owner, member]}`
  replaces this policy (an empty list forbids person withdrawals). A person of
  the team (author role owner, member or lead, never guest or an agent role) may
  also post a leaseless cancel request: lease null, gen 0, fence 0, and exactly
  `{target: <Job id>, status: "cancelled", reason, close_reason}`. The fold
  counts it but never moves the Job with it; the task holder's next pass writes
  the leased cancel with `re` set to the request id, the earliest request per
  Job wins, and every other request gets a message that it was not used
  (docs/decisions/D-r19-person-cancel-holder.md, rule C). A host may
  also supply `question.Store.WithdrawAllowed`. Agent shells still need the task
  lease fence on question, Feed, Job and MCP posting paths. This no-lease refusal
  applies to agent shells only; hosts may write leaseless person records allowed
  by the policy. The CLI treats guest/lead environments as agent shells, so a
  `withdraw: [guest]` or `withdraw: [lead]` policy only takes effect for host-written
  person records. Lease holders retain the existing control path.
  A missing policy file uses the owner/member default without warning; malformed
  policy uses that default with one warning per `question withdraw` command;
  raw `feed post` and MCP withdrawals do not print a fallback warning.
  If the task's policy file cannot be read, every withdrawal path fails with
  exit 1 and a one-line diagnostic before writing, including host and leased
  withdrawals (MCP returns a coded error). `answer`, `question add`, `question
  tick`, and `question sync` also fail before writing: their durable projections
  and decisions require a readable withdrawal policy, even with a host override.
  Read-only question operations and Feed/orchestrator projections instead use
  `withdraw: []` and print one path-free warning per operation: `question list`,
  `feed state`, MCP `feed_state`, `job tree`, orchestrator tick, and non-withdrawal
  `job post` continue, including question defaults and Job progress. This fallback
  never authorizes a question Store write or a withdrawal post.
  Person withdrawal
  follows Feed order and closes only an open question; an applied answer or
  default makes it a refusal. An already withdrawn question is a successful
  retry that prints the record. Refusals return exit 3 (HTTP 409 in the app),
  usage errors 2, other failures 1, and success 0. `accepted` replaces old
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
- A `done` after edge (and a bare Job id) is met by `accepted`, or by `submitted`
  when the Job's latest result has `verified.pass` true or `"pass"`; no verify,
  false and `"unsure"` do not meet it. A later `rejected` puts a dependent that
  has not started back to waiting; a started dependent is neither held nor
  stopped and gets one message post carrying its `job` that names the
  rejection. `failed` and `children-done` edges still wait for their own states.
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
  A Job agent posts found under its own Job with `grid job post --job <its Job> --type found`.
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
- Usage is a number the window shows. Dollars and minutes are nonnegative
  numbers; tokens are nonnegative integers. `grid usage post --tokens N` adds a
  delta; `usage show` includes token totals. The CLI has no threshold or budget
  verbs: `grid usage threshold ...` and `grid usage budget ...` answer the
  unknown-command error with exit 2, a post writes no usage-notice and prints no
  usage-notice line, and the workflow key `usage_notice_thresholds`, a budget in
  a task's `usage/thresholds.json` and `tokens_per_task` in `workspace.md` are
  ignored when present in an old home. The `usage-notice` feed type stays in the
  schema so notices already written to old feeds still read.
- There is no project-wide landing slot; landing safety stays with git and `land.lock`.
  `lease.json` validates the task and Job variants. Task leases have
  pid/pid_start while held; Job leases carry an attempt and null pid fields, with
  liveness in `.live`.
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

- `tasks/<task>/sessions/<machine>/<session>.json`: one session record per
  runtime session id and machine (plugin-design.md has no path entry for it).
- `tasks/<task>/feed/.live/<holder>.<fence>.json`: the Job liveness file
  (feed-contract.md:17 and :27), never shipped off the host.

Session and task allocation use `flock` on Unix and `LockFileEx` on Windows (`internal/fslock`).

## Command output

Plain output of `where` is one `name: value` line per field, with the same names as `--json`; a list gives one line per entry under the field's name, an empty field is left out, and paths under the home folder start with `~`; `--json` paths are absolute. `doctor` prints one `ok|fail <name>: <detail>` line per check. `version` prints `grid <version>`, `contract <n>` and `app_min <version>`. The golden files in `testdata/golden/` hold an example of each.

Every command except `help`, `version`, `doctor` and `migrate` first checks the Grid home's format version (`<GRID_HOME>/VERSION`). A newer format exits 1 with `This data is format <n>; this grid reads up to <m>. Update grid.`, an older one with `This data is format <n>; this grid reads <m>. Run: grid migrate`. A `VERSION` that cannot be read exits 1 with `Cannot read the format version in <GRID_HOME>/VERSION. Run: grid doctor`. A home without `VERSION` passes. `grid hook` prints nothing and exits 0 instead, with one exception: the `SessionStart` event of `grid hook`, while a migration journal is open, prints the `A migration of <GRID_HOME> stopped part way. Run: grid migrate --apply` line as the session's `additionalContext` (the SessionStart JSON answer) and exits 0.

### `grid version --json`

    {"version": string, "contract": string, "app_min": string}

### `grid where --json [--task <path or id>] [--project <name or id>]`

For an explicit `--project`, an exact project folder name match wins over an ID match; without a name match, duplicate IDs are refused as ambiguous.

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

Exit 1 with one line on stderr when the folder cannot be resolved (a task path outside the Grid home, an unknown project or task, a project.md that does not parse). A guest (guest, external or an unknown role) gets `task not found`, exit 1, with no host path, for every task path it cannot use, a path outside the Grid home or a link that leaves it included; owners and members keep `task path <p> is outside the Grid home <home>`.

### `grid job show <file>` and `grid job lint <file> [--task <task>]`

For a guest, `job show <file>` and `job lint <file> [--task <t>]` read the file only when it lies (links resolved) in a task the guest is invited to, and answer `task not found` otherwise, before the file is opened. With `--task`, the task has to be the one the file lies in.

### `grid doctor --json`

    {"ok": bool, "checks": [{"name": string, "ok": bool, "detail": string}]}

Checks, in order: `grid home`, `grid home writable`, `format version`, `migration`, `git`, `grid on PATH`, `plugin hooks`, `allow block`, `deny block`, `sandbox`. The `grid on PATH` detail ends with the version of the grid found (`version <v>`, or `version not read` when it cannot be asked, which never fails the check) and the check fails when that version differs from this grid's, with the path, both versions and a `fix`; two more checks appear only when they apply, after `grid on PATH`: `launcher`, when `<Grid home>/bin/grid` (`bin\grid.exe` on Windows) exists, reports the file it resolves to and its version and fails when that version differs from this grid's; `worker`, when the app's worker holds `worker.lock`, reports `running, version <v>` and fails with the app-version warning when the worker is older than the app version this plugin needs. Writability and format checks are skipped when the Grid home is unavailable. The migration check is never skipped: it is `ok: true` with detail `no interrupted migration`, or it fails with detail `a migration of <label> stopped part way` and the fix `Run grid migrate --apply` when the journal `<home>.migrate-journal` exists beside the Grid home, which includes the moment between migrate's two renames, when the Grid home is absent. The allow block and deny block checks remain in the report with `ok: true`, detail `skipped: Grid home is unavailable`, and no `fix` when the Grid home is unavailable. The deny block check reports whether setup's deny rules are in the Claude settings; it changes nothing. When the settings cannot be read, or hold none of setup's rules, the allow block already fails for that cause, so the deny block check is `ok: true` with a detail beginning `not checked:`; a deny rule missing from settings that do hold setup rules fails with `missing rules: ...`. The sandbox check is always `ok: true` and never fails the report; its detail is one of `on (grid setup --sandbox)`, `off; grid setup --sandbox turns on Claude Code's sandbox` or `unknown: <reason>`. Failing checks also include a `fix` string. The allow block check follows settings symlinks to regular, readable files through the same reader as setup and reports missing setup allow rules or a path-free settings-file failure. Missing rules or a missing file have the hint `Run grid setup`; invalid JSON, unreadable files, dangling symlinks, and nonregular files have `Fix or move the Claude settings file, then run grid setup.` Exit 1 when any check fails.

### `grid workflow show [--project <name-or-id>] [--json] [--policy]`

Reads the workspace `workflows.md` and registered project files, or the workspace and exactly one project selected by name or ID. JSON contains `files`, `templates`, and `errors`; each file has `scope`, `project`, `rev`, and `text`. Revisions are the first 12 hex characters of SHA-256 of the bytes, or null when absent. Plain output remains the file label, template names, and error count. Linked project folders are refused with exit 2 and a diagnostic that does not expose the path.

An exact project folder name wins over an ID; otherwise the ID must be unique. A selector that matches no visible project, or an ID shared by several, exits 1 with empty stdout and `grid: project "<selector>" is missing or ambiguous` on stderr (paths inside the selector print as `[path]`). A guest, or a caller not allowed Personal access (for example a member), gets the same exit and the generic form `grid: project is missing or ambiguous` for every unavailable selector, so a hidden project's name or ID cannot be told apart from an unknown one. Those callers also never see the Personal project in the unfiltered listing.

`--policy` requires `--json` (otherwise exit 2 with usage). It adds exactly one
`policy` array to the existing envelope; without the flag, plain output and the
three-key JSON envelope are unchanged:

```json
{
  "files": [{"scope": "workspace", "project": null, "rev": null, "text": ""}],
  "templates": [],
  "errors": [],
  "policy": [{
    "project": null,
    "keys": {
      "refusal_message": {"value": "plugin", "source": "default"},
      "inbox_act_roles": {
        "value": {
          "turn": ["owner", "member", "local", "agent"],
          "add": ["owner", "member", "local", "agent"],
          "note": ["owner", "member", "local", "agent"],
          "snooze": ["owner", "member", "local", "agent"],
          "dismiss": ["owner", "member", "local", "agent"],
          "wake": ["owner", "member", "local", "agent"],
          "reopen": ["owner", "member", "local", "agent"]
        },
        "source": "default"
      },
      "project_document_write_roles": {"value": ["owner", "member"], "source": "default"},
      "task_undo_roles": {"value": ["owner", "member"], "source": "default"}
    }
  }]
}
```

The example shows an absent workspace file with no registered projects. There is one policy entry
per `files` entry, in the same order, with `project` null for the workspace or the
project name. Every entry contains all four keys below. Each key has `value` and
`source` (`default` or `workflow`); workflow values also have `from` (`workspace`
or `project`). Values come from leading YAML front matter in those same
`workflows.md` files. Each project key uses its own set value, otherwise the
workspace value, otherwise the default. A set value replaces the inherited value
wholly, including empty maps and lists; maps are never merged. Workspace entries
use their own values or defaults.

| Key | Allowed value | Default |
| --- | --- | --- |
| `refusal_message` | `plugin` or `none` | `plugin` |
| `inbox_act_roles` | Map from `turn`, `add`, `note`, `snooze`, `dismiss`, `wake`, `reopen` to lists of `owner`, `member`, `local`, `agent` | Every act: `[owner, member, local, agent]` |
| `project_document_write_roles` | List of `owner` or `member` | `[owner, member]` |
| `task_undo_roles` | List of `owner` or `member` | `[owner, member]` |

An act omitted from a set `inbox_act_roles` map keeps no roles and is refused;
`{}` refuses every act and an empty role list refuses that act. `guest` is invalid
in this key: guests are refused regardless. This command reports policy for app
consumers; it does not change `signal act` enforcement.

Only these four keys are reported and validated. Other keys are ignored, including
`task_file_revision_roles` (its default is a rule, not a role list), `updates_folder`
(future), `gate_post_roles` (gate policy owned by `gates.go`), and `person_verbs`
(task/gate policy and L122 project inheritance). Existing person-gate checks still
apply independently.

A bad type, unknown act, role or value exits 1 with no stdout and exactly
`grid workflow show: invalid <key> in <workspace|project> workflow` on stderr.
Malformed front matter uses `invalid front matter` in place of `invalid <key>`;
unreadable, unsafe, oversized or invalid-UTF-8 policy files fail the same way,
without falling back to permissive defaults. Diagnostics contain no value or
path. Policy uses the existing 32768-byte cap, symlink refusal and guest project
filter; hidden and unknown project requests remain indistinguishable to guests.

### `grid workflow write [--project <name-or-id>] --rev <12-hex|none> --file <path|-> [--json]`

Replaces one workspace file, or `projects/<name>/workflows.md` for exactly one registered project. Input comes from a file (relative to the current folder) or stdin with `--file -`; it must be UTF-8, at most 32 KiB, start with a workflows heading on the first line, and pass workflow parsing. `--rev none` requires absence; otherwise the revision must match under the destination's persistent lock. A mismatch exits 3 with `workflow changed since read`; with `--json` stdout is the envelope `{"error":{"code":"workflow_file_changed","message":"grid workflow write: workflow changed since read"}}`. Invalid input exits 2 and parse diagnostics are listed. Refusals leave existing bytes unchanged.

The project is selected as for `workflow show`. A selector that matches no visible project, or an ID shared by several, exits 1 with `grid workflow write: project "<selector>" is missing or ambiguous`; a caller that passes the role policy but is not allowed Personal access (a member) gets `grid workflow write: project is missing or ambiguous` instead.

Writes use a synced temporary file in the destination folder and atomic rename, with symlinks refused and destination operations confined by `os.Root`. A linked or non-regular target exits 2. JSON is `{ "file": "workflows.md", "rev": "<12-hex>" }`, with `"<project>/workflows.md"` for a project; plain output is `<file> <rev>` on one line. Diagnostics do not expose absolute paths. The replaceable `WriteDefaultAllowed` host policy accepts an unset/empty `GRID_ROLE`, `owner`, or `member`; other roles exit 3 before reading input or writing files, with stderr `grid workflow write: caller role is not allowed to write workflows`. With `--json` that refusal prints the person gate object `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to write workflows","code":"person_required","message":"grid workflow write: caller role is not allowed to write workflows"}`; it is the host policy, not a workflow's role list. Application authorization remains the caller's responsibility.

### `grid task new "<words>" [--project <name-or-id>] [--size <size>] [--workflow <name>] --json`

    {"id": string, "path": string}

The path is absolute. Plain output is two lines: `T-NN created`, then the task folder path, with the home spelled as `~`. The original words are written byte for byte to `intent.md`. Size is `auto` (default), `small`, `normal` or `large`. Workflow is a name, with an empty string meaning selection is left to the workflow rules. State starts as `queued`; creation does not start execution.

Project defaults to the existing folder resolver's answer. `--project` selects a registered name or id; `personal` and `Personal` select the Personal project without requiring a record. Personal tasks live under `<GRID_HOME>/personal/tasks/` and use `project: personal`. Other tasks live under `<GRID_HOME>/projects/<project>/tasks/`. A first write creates the project record, machine identity and format version as needed.

The title comes from the request in four steps. When the request, after leading spaces, tabs and newlines, starts with a front matter block whose `title` is a string that is not blank, the title is that value with each run of whitespace folded to one space. When the block has no such `title`, the title is the first nonblank line of the text after the block, trimmed. A request without a front matter block keeps the first nonblank line of the request, trimmed. A request whose block has no such `title` and nothing but whitespace after it is refused with exit 2 and `task words hold only front matter and no title`, and nothing is written. Every title is limited to 120 Unicode characters. The folder slug uses lowercase ASCII letters and digits, separated by hyphens, limited to 60 characters, with `task` as the fallback. These transformations never change `intent.md`.

### Task operands

`task show`, `approve`, and `task close`,
`stop`, `reopen`, `status`, `history`, `attach`, `rename` and `decline` share
one CLI task resolver. The same resolver handles `deliver --task`,
`proof add/index --task`, `history add --task`, `land <task>`,
`brain show/search/keep --task` and `brain decide <task>`. A task operand can be a display id (`T-01`), an exact
folder name (`T-01-renamed`), or an absolute or current-directory-relative
path to that folder. IDs and folder names retain home-wide lookup; an explicit
folder suffix must match exactly. Omitted `task show` operands select the
enclosing task.

Explicit paths must name an immediate child of that project's tasks directory.
Paths outside an accessible tasks directory, symlinks escaping that directory
(including links into another project's tasks), and any `..` component exit 2
before a write. Missing or ambiguous tasks still exit 1. The existing caller
policy controls Personal enumeration and resolution; folder operands grant no
additional access. For callers without Personal access, a hidden Personal task
folder or ID returns the same `task not found` message and exit 1 as an unknown
ID, even if that folder does not exist. Approval and refusal decisions still
require a person.

`task show --json` reports `id`, absolute `path`, and `folder` (the resolved
folder basename). Task write receipts from `approve`,
`close`, `stop`, `reopen`, `status`, `rename` and `decline` retain their `task`
display id and add `folder`. Persisted history receipts keep their existing
shape. `task history` and `task attach` retain their existing JSON shapes.
ID and folder forms produce identical output for the same state, apart from
write timestamps. For example:

```text
grid task show T-01-renamed --json
grid approve T-01-renamed --plan-hash <hash> --json
grid task close T-01-renamed --json
```

### `grid task show [<task>] --json`

    {
      "id": string,
      "title": string,
      "project": string,
      "size": "auto" | "small" | "normal" | "large",
      "workflow": string,
      "state": string,
      "created": string,
      "path": string,
      "folder": string,
      "intent": string
    }

`created` is a UTC RFC3339 timestamp on creation; `path` is absolute. `intent` is the exact file content. The remaining fields come from YAML front matter in `task.md`. Show folds these files without changing them or inferring execution state. Plain output prints one `name: value` line per field, followed by `intent:` and its text. An omitted id selects the enclosing task; an explicit id is looked up across the Grid home, including Personal. Missing, duplicate or malformed tasks fail instead of returning a partial record.

### `grid task rename <task> --title <text> [--project <p>] [--json]`

Renames an open task. The title is trimmed and whitespace is collapsed, with 1–200 Unicode characters and no line breaks. Titles containing control, bidi override/isolate, or Unicode noncharacter code points are refused with exit 2 before YAML encoding. Only `title` and `updated` change in `task.md`; the folder, intent, summary and other lines remain unchanged. Closed tasks must be reopened first; an unchanged title is rejected. History records `renamed · <title>` in the existing JSON receipt's `note` field, with `what: rename`.

### `grid task decline <task> --reason <text> [--project <p>] [--json]`

Closes an open task with a required reason after whitespace collapse. Reasons containing control, bidi override/isolate, or Unicode noncharacter code points are refused with exit 2 before YAML encoding. Tasks with `kind: planner` and already closed tasks are rejected. Writes `state: closed`, `next: declined by <who>` and `updated`; the host person comes from `GRID_ACTOR`, then `GRID_WHO`, otherwise `you`; an agent shell (no `GRID_ACTOR` and a worker mark, `GRID_WORKER_ID`, `GRID_AGENT` or `GRID_WORKER`) records its worker id instead, so `next` reads `declined by <worker id>`. History records `closed · declined · <reason>` in the existing JSON receipt's `note` field, with `what: decline`.

Both verbs use the same task lock, journal and atomic commit as stop and close, and refuse symlinked `task.md` files. `--project` constrains the task to a registered project name or id, or `personal`; omission uses the home-wide task id. `--json` prints the existing task write receipt; plain output uses `T-NN <verb>: <from-state> -> <to-state>`. Errors print one diagnostic line: exit 2 for invalid title or reason, 3 for a rejected task state or unchanged title, and 1 for scope, resolution or write failures. Success exits 0.

`RenameAllowed` and `DeclineAllowed` are replaceable role policies in `internal/task`, called with the host's trimmed `GRID_ROLE`. Defaults allow the empty local role, owner and member; all other roles, including guest, are refused. A host or workflow may replace the policies during initialization; these commands do not read a workflow file.

`grid task delete <T-NN> [--project <p>] [--json]` moves the whole task folder to `<GRID_HOME>/trash/<project>/<folder>@YYYYMMDDTHHMM/` using the UTC minute; `grid task restore <T-NN> --project <p> [--json]` restores the newest stamped copy. Both hold the existing task lock and serialize folder publication with task creation, append one history receipt after a successful move, print the resulting folder name, and return `{project, id, folder}` with `--json`. Delete accepts any state but refuses the standing Planner and any recorded task lease holder; it never takes or breaks a lease. Same-minute destination collisions, occupied restore ids, absent trash copies, and denied roles exit 3; unsafe symlink paths and invalid input exit 2; unknown delete ids and I/O failures exit 1. Neither move overwrites an existing folder or copies across filesystems. A failed rename leaves no history receipt; a history failure after rename reports that the move happened and does not undo it. Actor and role come from `GRID_ACTOR` and `GRID_ROLE`, with local-person/owner defaults for an ordinary shell; an agent shell (no `GRID_ACTOR` and a worker mark) records its worker id and the role `agent` in the receipt, while the allowed-role policy still reads the host role, so an agent role is refused; and Personal tasks require the same person guard and use `trash/personal/`. Replaceable `DeleteAllowed` and `RestoreAllowed` policies default to owner only; these commands read no workflow file.

### `grid task list [--project <name-or-id>] [--state <state>] --json`

Returns an array of task records (without the show-only `folder` and `plan_hash` fields), sorted by numeric display id, or `[]` when none match. State filtering is exact and case sensitive. Readable entries are full records; an unreadable task folder is listed as `{id, project, unreadable: true}` under any `--state` filter, with a warning on stderr. Plain output is one `T-NN state title` line per task, or no output for an empty list. Both read commands leave files unchanged and ignore hidden creation staging folders.

Task commands exit 0 on success, 2 for invalid flags or argument counts (including invalid creation size and empty words), 3 for the rename/decline rejections described above, and 1 for resolution, data or filesystem errors. Errors go to stderr without JSON on stdout. Flags may precede or follow the positional argument; use `--` before words that start with a hyphen.

### `grid hook <event>`

Reads the event JSON on stdin and always exits 0. `sessionstart` prints

    {"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": string}}

where `additionalContext` names the project or task `grid where` resolved for the event's `cwd`. Other events print nothing yet.

### `grid hook levels [--json]`

Read-only. Prints `<rule> <level>` per line for every hook rule, or with `--json` one object `{"<rule>": "off|warn|block", ...}`, for the current project. The rule names are the ones `grid hook events` answers for.

### `grid hook events [--all] [--json]`

Read-only. Every hook run that consults a rule appends one line to `<GRID_HOME>/.hook-counts/events.jsonl`, next to the `home-allow.jsonl` counter that folder already holds. The file is always under the Grid home, never under `CLAUDE_PLUGIN_DATA`, so this read finds what a hook wrote inside Claude Code. A line is `{"rule": string, "event": string, "at": string, "outcome": string}`: `rule` is a name `grid hook levels` lists; `event` is the Claude Code or Codex hook event the rule was consulted for (`SessionStart`, `PreToolUse`, `PostToolUse`, `Stop`, ...); `at` is UTC, RFC 3339 (`2026-11-15T10:00:00Z`); `outcome` is one of `ok` (consulted and let the event through), `warned` (it spoke, a warning or a question, and did not refuse), `blocked` (it refused: a denied tool call, or `Stop` exit 2) or `skipped` (its level is `off`). A line holds nothing else: no tool, path, prompt, session or project, so the log is Grid-home wide. A rule asked at several places in one run leaves one line. The write is best effort: a failure is ignored and never changes the hook's exit code or output. Only rules a hook run reads leave lines: `session-start`, `session-end`, `history-log`, `usage-log`, `stop-check`, `plan-guard` and `home-allow` (when its fallback allows); `host-guard`, `only-writer`, `needs-you`, `session-check` and `job-lint` have no hook reader and stay null.

The log is bounded like the app log (`grid open`): at 2 MiB the file is shifted to `events.jsonl.1`, then `.2`, and the oldest copy is dropped, so at most three files of 2 MiB stay (6 MiB, about 60,000 lines, over 3 hours at 250 lines a minute). A busy rule can push a quiet rule's last line out of that bound; the rule then answers null.

Default output is one line per rule, `<rule> <lastEvent> <at> <outcome>` with `-` for a rule with no line. `--json` prints an array in the order of `grid hook levels`, one object for every rule it lists:

    [{"rule": string, "lastEvent": string|null, "at": string|null, "outcome": string|null}, ...]

A rule with no line has all three of `lastEvent`, `at` and `outcome` null. `--all` prints the log instead, newest first (file order, across the shifted copies, not sorted by `at`): `<at> <rule> <event> <outcome>` per line, or with `--json` an array of `{rule, event, at, outcome}`, `[]` when the log is empty. A line that does not parse is skipped. Exit 0; exit 2 for an operand or an unknown flag; exit 1 when the folder cannot be read. It takes no `--project` or `--task`.

### `grid brain check [<proposal-id>] [--level home|project] [--project <p>] [--json]`

Read-only gate for what changes what runs. With an id it checks that proposal
(any status); without one it checks every proposal with status `proposed` plus
every entry under `knowledge/skills/` and the `agents` config topic at the
selected levels. It writes nothing. Findings carry `code`, `file`, `line` (when
there is one), `message` and `text`, the quoted source, which is data:

- `config_topic`: a proposal aimed at workflows, environments, machines, deploy,
  people, routines or agents (`config: <step>`, the step to take for that topic).
- `skill_field`: skill front matter key outside `name, description, when_to_use,
  metadata, topics` (`allowed-tools`, `hooks`, `shell`, `context`, `agent`, `model`).
- `skill_shell`: a line starting with `` !` `` or a fence opening with ```` ```! ````.
- `skill_file`: a file other than `SKILL.md` and `reference/*.md`, a symlink, an
  executable bit, `agents/openai.yaml`, `hooks/`, `.mcp.json` or `bin/`.
- `agent_field`: agent front matter key outside `name, description, tools, model,
  effort, maxTurns, metadata`.
- `audit_hash`: without an id only. A published entry whose bytes no longer hash to
  the newest hashed line of `knowledge/log.md` for it (`entry changed outside the
  keeper: v<n> was <hash>, now <hash>`). Entries logged before hashes existed,
  rejections and lines that do not parse give no finding; an entry that is gone
  is not drift. The replaceable `AuditHashCheck` host policy (key
  `brain_audit_hash_levels`, default home and project) turns it off per level.

`--json` prints `{"checked": n, "findings": [...]}`; plain output fences each
quoted text. Exit 0 with no findings, 3 with findings, 1 when the brain cannot
be read, 2 on usage. Prose never changes the result: a knowledge proposal that
asks to become a skill has no finding.

`grid brain accept <id>` publishes what passes the keeper gate (secrets, note
validation, reserved and config topics). A config topic is refused by the gate:
exit 1 and a rejected proposal. The proposal check does not run on accept;
`grid brain check <id>` is a read the keeper may run first. The drift check does
not run on accept either.

### `grid brain changes [--level home|project] [--project <p>] [--since <RFC3339>] [--limit <n>] [--json]`

Lists the audit lines of the level's `knowledge/log.md`, newest first (lines with
the same time keep the later one first). `--since` keeps lines at or after that
RFC 3339 time and `--limit` caps the count (0 or less: all); both filter as
`Store.Changes` does. The level is `--level`, else the resolved project, else
home. `--json` prints `{"changes": [...]}`, each with `at`, `status`, `topic`,
`name`, `version`, `hash`, `origin_kind`, `origin_id` and `reason`; lines written
before versions existed have no topic, version 0 and an empty hash. Plain output
is one line per change, `<at> <status> <topic>/<name> v<n> <hash> <reason>`
(`-` for an empty hash, no reason field when empty, control characters shown as
`?`). A missing log is an empty list: `{"changes":[]}` or no output, exit 0.

### `grid brain revert <topic>/<name> --to <n> [--reason <why>] [--level home|project] [--project <p>] [--json]`

Publishes stored version `n` of a knowledge entry again, as a new version, so the
history only grows: after versions 1 to 3, `--to 1` makes version 4 with version
1's bytes. Under the level lock it stores the copy, publishes the entry
atomically, appends an audit line with status `reverted`, reason `revert to v<n>`
(`revert to v<n>: <why>` when `--reason` is given; the audit line folds whitespace
and writes `|` as `/`) and the new version and hash, then rebuilds that level's
index. `--json` adds `reason` when given. When the entry
already holds those bytes (and the newest stored copy does too) nothing is written
and the current version is reported (`noop` means nothing was written). A stale
or missing entry is written again, so it is not a no-op even when the newest
stored copy already held the bytes and no version is added. Config topics are
refused (they are changed through their own commands), as is a copy that is not
a valid entry for that topic, name and level, and a name another topic already
holds. A project without a Grid record is refused as for `propose`; use
`--level home`.

Plain output is `v<n>` for a write, or `already at v<current>` for the no-op.
`--json` prints `{"version": <n>, "noop": <bool>}`, `noop` true only when nothing
was written. `--to` is required and must be 1 or more, and `<topic>/<name>` must
be two plain names (exit 2). A missing version or entry, or any store error,
exits 1.

When the new version is written and logged but the index rebuild then fails, the
verb still reports the version before the error and exits 1: plain output prints
`v<n>` on stdout and the error on stderr; `--json` prints
`{"version": <n>, "noop": false, "error": "<message>"}` on stdout (`error` only in
this case) and the usual `{"error": "<message>"}` on stderr.

Both verbs check `GRID_ROLE` after the flags parse and before any resolution,
read or write. `changes` checks the replaceable `brain.ChangesReadAllowed` (host
key `brain_changes_read_roles`) and `revert` checks the replaceable
`brain.CurateAllowed`; both default to the empty local role, `owner`, `member` and
`worker` (a Job agent, which brain-keeper tells to read the log and to supersede
and revert). Any other role, including `guest`, exits 3 with
`grid brain <changes|revert>: caller role is not allowed to <read brain changes|revert brain entries>`.
`grid brain supersede` checks `brain.CurateAllowed` at the same point and exits 3
with `grid brain supersede: caller role is not allowed to supersede brain entries`.
`brain.RevertAllowed` (host key `brain_revert_roles`) keeps its default, the empty
local role, `owner` and `member`, and gates `write`, `rename`, `drop` and
`restore`, which still refuse a `worker`.
With `--json` each of the seven refusals (changes, revert, supersede, drop,
restore, write, rename) keeps that stderr line and exit 3 and
also prints one stdout object, the person-gate refusal shape:
`{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to <read brain changes|revert brain entries|supersede brain entries|drop brain entries|restore brain entries|write brain entries|rename brain entries>","code":"person_required","message":"<the stderr line without its newline>"}`.
Without `--json` stdout stays empty.
The CLI does not read a workflow file to configure them.

### `grid brain drop <topic>/<name> --expect <16hex> [--reason <why>] [--level home|project] [--project <p>] [--json]`

Hides a knowledge entry from `brain search`, the SessionStart index (and its
entry count) and job sources. Nothing is deleted: the note file and its stored
versions are never written, and `brain show` still returns the entry, now with
`"dropped": true`. Under the level's brain lock it checks `--expect` against the
note's current rev, appends one audit line with status `dropped` (the current
version and hash, the origin and the reason, whitespace folded), then rebuilds
that level's index after the lock. The rev is the first 8 bytes of the SHA-256 of
the note file as 16 lower-case hex digits (the app's `vaultRev`). The newest
`accepted`, `dropped` or `restored` line of the entry is its state: a drop needs
an accepted note that is not superseded and not already dropped. Two drops with
one rev at once write one line; the second is refused (exit 9).

The origin of the line is the person from the session: `person:<id>` where `<id>`
is `GRID_WHO`, else `GRID_ACTOR`, else `GRID_MEMBER` (a plain name), else a
random `session` origin as for `revert`. No flag names the person; `--by` is an
unknown flag (exit 2).

`--json` prints `{"status":"dropped","version":<n>,"rev":"<rev>"}`; plain output
is `dropped <topic>/<name> v<n> rev <rev>`. When the line is written but the index
rebuild fails, the result is printed first, then the error, and the exit is 1.

Exit codes of drop and restore: 0 done, 1 a store error (a full
disk included), 2 usage (a missing operand, a topic or name that is not a plain
name, a missing or non-hex `--expect`, an unknown flag), 3 role refused, 6 stale
`--expect`, 8 no such note, 9 refused by the note's state. Never 4, 5 or 7. A
failure with 6, 8 or 9 prints one JSON line on stderr, with or without `--json`:
`{"error":"conflict","rev":"<current rev>"}`, `{"error":"no_such_note"}` or
`{"error":"refused","reason":"<why>"}`. Refused (9): a topic that holds settings
(`machines`, `skills`, `agents` and the like), the `TOPIC` file, a note that is
not accepted or is superseded, an entry already dropped. The role check
(`brain.RevertAllowed`, the empty local role, `owner` and `member`) runs after the
usage check and before any resolution, read or write: any other role, including
`guest`, exits 3 with
`grid brain drop: caller role is not allowed to drop brain entries`. A project
without a Grid record is refused as for `propose` (exit 1); use `--level home`.

### `grid brain restore <topic>/<name> --expect <16hex> [--reason <why>] [--level home|project] [--project <p>] [--json]`

Brings a dropped entry back to search, the index and job sources. Same flags,
rev check, origin, output and exit codes as `brain drop`, with the audit status
`restored` and the stderr line
`grid brain restore: caller role is not allowed to restore brain entries` for exit
3. A restore needs an entry whose newest state line is `dropped`; any other entry
is refused (exit 9). The note file and its versions are not written.

### `grid brain write <topic>/<name> --expect <16hex|none> --file <path|-> [--reason <why>] [--level home|project] [--project <p>] [--json]`

Publishes the caller's Markdown as the knowledge note `<topic>/<name>` of one
level: a new note with `--expect none`, or a replace at the note's current rev
(the rev of `brain drop`). `--file -` reads stdin. The bytes must be an accepted
note that names the same topic, name and level and passes the keeper gate, at
most 1 MiB. Under the level's brain lock it checks the rev, checks that the note
on disk has a stored copy of its exact bytes, stores the new bytes as the next
version, publishes the note atomically and appends one audit line with status
`created` or `edited` (the version, the hash, the origin and the reason,
whitespace folded), then rebuilds that level's index after the lock. The origin
is the person from the session, as for `brain drop`; no flag names the person.
A refusal writes nothing.

`--json` prints `{"status":"created|edited","version":<n>,"rev":"<rev>"}`; plain
output is `<created|edited> <topic>/<name> v<n> rev <rev>`. When the note and its
line are written but the index rebuild fails, the result is printed first, then
the error, and the exit is 1.

Exit codes: 0 done, 1 a store error (a project without a Grid record included;
use `--level home`), 2 usage (a missing operand or `--file`, a topic or name that
is not a plain name, an `--expect` that is neither 16 hex digits nor `none`, an
unknown flag, input that is not such a note, a linked topic folder or note file),
3 role refused, 6 stale `--expect` (`{"error":"conflict","rev":"<current rev or none>"}`),
7 not versioned (`{"error":"not_versioned"}`: the topic holds settings or has no
definition, or the note on disk has no stored copy of its bytes, a note placed by
hand), 9 refused (`{"error":"refused","reason":"<why>"}`: the `TOPIC` file, a name
another topic of the level holds). Never 4, 5 or 8. The JSON line of 6, 7 and 9
goes to stderr with or without `--json`. The role check (`brain.RevertAllowed`,
the empty local role, `owner` and `member`) runs after the usage check and before
any resolution, read or write: any other role, including `guest`, exits 3 with
`grid brain write: caller role is not allowed to write brain entries`. A note of
`type: workflow`, in the input or on disk, is written only by the empty local
role and `owner`; `member` exits 3 with
`grid brain write: caller role is not allowed to write workflow notes`.

### `grid brain rename <topic>/<name> <new-name> --expect <16hex> [--reason <why>] [--level home|project] [--project <p>] [--json]`

Renames one accepted, versioned knowledge note within its topic and level, if
`--expect` is its current rev, and relinks the notes that name it. Under the
level's brain lock the note's bytes with the new name are stored as a version of
the new name, written as the new file, the old file is removed and one audit
line with status `renamed` is appended; a failure before that line puts the old
file back. Then every other accepted note of the same level whose links or
`superseded_by` name `brain:<level>/<name>` gets `brain:<level>/<new-name>` as a
new version with a `linked` audit line. A dropped note that links to the old name
is skipped and keeps its link. A note whose relink fails is left as it was and
listed as `kept`; the exit stays 0. Notes of the other level are not relinked.
The old name's stored versions stay where they are. The index is rebuilt after
the lock. The origin is the person from the session, as for `brain drop`.

`--json` prints `{"status":"renamed","from":"<topic>/<name>","to":"<topic>/<new-name>","version":<n>,"rev":"<rev>","linked":[...],"kept":[...],"skipped":[...]}`;
plain output is `renamed <topic>/<name> -> <topic>/<new-name> v<n> rev <rev>`,
then one `linked <topic>/<name>`, `kept <topic>/<name>` or
`skipped <topic>/<name>` line per note. When the rename is applied but a later
step fails (listing the notes to relink, or the index rebuild), the result is
printed first, then the error, and the exit is 1.

Exit codes: 0 done, 1 a store error (a project without a Grid record included;
use `--level home`), 2 usage (a missing operand, a topic, name or new name that is
not a plain name, a missing or non-hex `--expect`, an unknown flag), 3 role
refused, 6 stale `--expect` (`{"error":"conflict","rev":"<current rev>"}`), 7 not
versioned (`{"error":"not_versioned","reason":"<why>"}`: the topic holds
settings, the note has no stored version, or the newest stored version is not the
note on disk), 8 no such note or topic (`{"error":"no_such_note"}`), 9 refused
(`{"error":"refused","reason":"<why>"}`: the `TOPIC` file or `TOPIC` as the new
name, a note that is not accepted, superseded or dropped, a workflow note, the
same name, a new name the topic or another topic of the level already holds).
Never 4 or 5. The JSON line of 6 to 9 goes to stderr with or without `--json`.
The role check is that of `brain write`: any role but the empty local role,
`owner` and `member`, including `guest`, exits 3 with
`grid brain rename: caller role is not allowed to rename brain entries`.

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

Windows builds and its locks are implemented (`LockFileEx`), compiled by
`make cross`; a run on a real Windows machine is still to do.

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
the reason. Exit codes are 0 for success, 1 for storage/scope errors (may retry),
2 for invalid command usage, 3 for no current claim token (do not retry),
and 5 taken for an already claimed or done intent (skip the intent quietly;
ownership may change). These refusal codes apply to claim, renew, release and
done. Existing session list/show behavior is preserved. These new commands define
no task or Feed record shapes.

`grid feed` and `grid job post|ack|result|cancel` exit with 0 for success, 1 for storage/scope/resolution or other errors, 2 for usage, 3 for final rejection (stale lease, invalid record, or Job-side kind/Job lease refused by `grid feed post`; do not retry), and 4 for disk full. Status 5 taken applies to the intent refusals above.

## Plan approval receipts

The plan Job's accepted result is the only approval of a plan: `grid approve`
answers the task's plan Job, and the approval counts only while the `plan_hash`
and `plan_digest` in that result match `plan.md`. `grid approve` also writes
`approval.json` in the task folder, a copy for readers of the task folder (the
app reads it). The copy is not a fallback: a task with no plan Job has no
approval, and nothing reads `approval.json` to approve a plan. It is read only
by a repeat `grid approve` while the plan Job is still open, and by the
migration that moves an old plan receipt into the plan Job's result and removes
the file. Preview and ship receipts (`approval-preview.json`,
`approval-ship.json`) are no longer written or read: an old journal replay drops
them. Their names are still in the host guard's file list, which warns by
default and denies only at level `block`.

`grid approve <task> --plan-hash <hash> --send-back <words> [--json]` sends the
open plan approve Job back: the holder rejects it with close reason `sent back
by <who>` and the words; the plan is not approved. The answer and the control
carry the optional `send_back` (always `true`) and `sent_back_hash` (the 6-hex
plan hash the person read); `type_v` stays 1. History records `send_back`, the
plan hash and who, never the words; `--note` is refused with `--send-back`; a
guest cannot send back. The `--json` output has `what` `send_back`.

The result and the copy retain `plan_hash`, the six lower-case hex digits used in the plan heading.
New approvals also write `plan_digest`: all 64 lower-case hex digits of SHA-256
of canonical plan bytes, including step `evidence:` and `needs:` lines that the
legacy short hash omits. The short hash is therefore not necessarily a prefix
of the digest. The snake_case name follows Rust's
`plan_hash` naming; it is an additive plugin receipt field, not a field claimed
to exist in the Rust history format. Both values are computed from one snapshot
under the plan lock and recorded together. Publication checks the hash and
digest computed from its exact publication snapshot without rereading the plan.
Receipts made before step metadata was included may need explicit reapproval.
The heading and history keep the short hash. The CLI `--plan-hash`
compare-and-set token accepts either the six lower-case hex digits or the
64 lower-case hex digest printed alongside `hash` by `grid plan hash --json`.
`grid task show --json` exposes these as `plan_hash` and `plan_digest`.
Both token forms are checked under the same plan lock as the receipt write.
Whichever token is supplied, the receipt stores the current six-hex hash in
`plan_hash` and the current full digest in `plan_digest`. Plain `grid plan hash`
output remains the short hash.

The task's `workflow.md` front matter key `plan_hash_token` accepts `any`
(the default when the file or key is missing) or `digest`, which requires the
64-hex form of a supplied token. An invalid value, unreadable file or malformed
front matter refuses CLI approval with exit 3; it never falls back to `any`.

Publication, the guard, and locked approval readers require both values to
match. A receipt without `plan_digest` does not count as approved; a present
empty, null, non-string or incorrect digest is also refused. Explicitly
reapproving with either token writes a digest receipt (subject to the workflow
token policy). An implicit reapproval also
checks the previous digest, so a short-hash collision requires an explicit
review and `--plan-hash` to replace the approval. `approval.json` carries no
`gate` field.

`grid approve` takes `--plan-hash`, `--note` and `--json`, and no flag names a
step: the approval is the plan Job's result whatever the step is called.
Without `--plan-hash` it approves the current plan, and refuses with `Changed
since you approved` when the result it finds no longer matches. The person
check still comes first: an agent gets exit 3. A repeat approve whose result
already matches the current hash, digest and lands hash (and the `--plan-hash`
token, when one is given) writes nothing: no rewrite of `approval.json` and no
second history line. It exits 0 and prints the stored approval. While the plan
Job is still open, the repeat finds the approval in `approval.json` instead.
Reopening a task writes a reopen event over `approval.json`, so the copy no
longer matches, and the task's new plan Job needs its own approval.

`grid plan migrate` reads no receipt file: it asks for the approved plan hash,
which is the plan Job's result. If that hash is the plan being migrated, the
migration changes an approved plan and needs `--yes`.

Rust's existing history receipt remains readable by the legacy guard path:
`- <time> · plan accepted · rev <n> · <hash> · by <person>`.
It counts only for a task with no plan Job; a task with a plan Job is answered by
the Job's result, which takes precedence over that prose receipt, including when
its digest has become stale. Legacy approvals remain weak until reapproved;
this extension does not retroactively strengthen approvals or change Rust readers.

The host guard protects `task.md` under the same write rule as the approval
file, including when the plan guard is disabled, because its `workflow` selects
the approval steps. The guard warns by default and denies only at level
`block`. Reads remain allowed. An `approve` step whose name
contains `plan` is the plan approval, as in Rust; an `approve` step before the
first `run` or `work` also requires plan approval, regardless of its name (for
example, `sign-off`). A later approval without `plan` in its name remains a
delivery approval.

A task is bound to its workflow by its project: `task.md` must name the project
whose `tasks/` folder it sits in. A task whose `task.md` names another project
than its folder is held, and no approval releases it. An inline `workflow:` list
needs no project and is read as it is. Grid does not write a digest of the
workflow into `task.md`, and nothing holds a task because a `workflows.md`
changed after the task was made.

The task's `workflow.md` snapshot and workspace/project `workflows.md` files
are protected by the host guard against the Write, Edit, MultiEdit and
NotebookEdit file tools, including symlink aliases for workspace/project policy,
even with the plan guard disabled. The host guard reads no shell text: a Bash
command is never answered by it. Reads remain allowed. The generated Claude permission
rules already protect every file in task folders. Task snapshots supply
additional `person_verbs` and gate posting roles; signal verbs also apply the
project/workspace person policy described below.
Invalid person policy is ignored with one warning that omits paths and values:
`grid: warning: invalid person_verbs; ignoring the key and keeping the person-only floor`;
the person-only floor still applies. Hook commands bypass this policy so their
handlers can return normal decisions, except the floor verb `hook level`.

## Signal acts

`grid signal act <id> [--project <p>] [--act turn|add|note|snooze|dismiss|wake|reopen]
[--task T-<n>] [--until <time>] [--later] [--json]` decides an inbox note.
Omitting `--act` means `note`. `grid signal dismiss <id> [--project <p>] [--json]`
is an alias for `--act dismiss`. The project must be registered. Guests are
refused before opening a note, including guests invited to that project.

`grid signal move <id> [--project <from>] --to <project|-> [--json]` moves
an open note between registered projects. The source resolves like the other
signal verbs. `--to -` means the workspace inbox: key `inbox_workspace` in the
workspace `workflows.md` YAML front matter defaults to `personal` when unset, and
`personal` is the only supported value. Any other value, malformed front matter,
or an unsafe workflow file refuses an inbox move with exit 1 and leaves the signal
unchanged; with `--json` the policy error is
`{"error":"invalid_policy","key":"inbox_workspace","message":"<msg>","code":"invalid_policy"}`. Named destinations do not
consult this inbox policy. Both projects
must pass registration and Personal access checks; guests receive the same
`guests cannot act on signals` refusal, even in an invited project. Moving
`new` into a named project changes it to `placed`; moving `placed` into Personal
changes it to `new`. Snoozed notes retain their status and `until`. Other note
content, including timestamps, is preserved. JSON adds optional `from`, sets
`project` to the destination and `became` to the resulting status, and uses
`result: updated`.

Move returns exit 0 on success, 2 for invalid operands or a missing/empty
`--to`, 1 for I/O or lock errors or an invalid inbox policy, and 3 for these refusals: `no such project`,
`no such note`, `this note is already decided (<status>)`,
`the note is already there`, `a note with this id is already there`,
`notes already holds 4000 files`, or the guest refusal above. Its typed errors
use the JSON envelopes described below. Move holds Add's workspace signal lock
through checks, the atomic source update and the rename, never overwrites an
existing target, and restores the source content if rename fails. Both folders
are under one home on the same filesystem; there is no copy-and-delete fallback.
The person-gate floor does not yet include `signal move`; local signed-out
callers resolve as they do for the existing signal verbs. The existing
`person_verbs` policy also applies to moves.

Readable statuses are `new`, `placed`, `snoozed`, `kept`, `dismissed`, and `done`.
Legacy `acted` reads as `done`; writers never emit `acted`. Open notes are
`new`, `placed`, and `snoozed`; the other statuses are decided.

| Act | Effect | Required option |
| --- | --- | --- |
| `turn` | Create a task in the note's project, set `done` and `became` to its id | None |
| `add` | Set `done` and `became` without creating a task | `--task` matching `^T-[0-9]+$` |
| `note` | Set `kept` | None |
| `snooze` | Set `snoozed` and `until` | `--until` in RFC3339 or local `YYYY-MM-DDTHH:MM` |
| `dismiss` | Set `dismissed` | None |
| `wake` | Set `new`, only from `snoozed` | None |
| `reopen` | Set `new` and clear `became`, only from a decided status | None |

Every applied act updates `updated`. Every applied act except snooze clears
`until`; snooze stores it in local `YYYY-MM-DDTHH:MM`, converting RFC3339 offsets
to local time. Other than turn, add and reopen, acts preserve `became`.
Repeating note on kept or dismiss on dismissed succeeds with `unchanged` and
leaves the file byte-identical. Other state refusals also leave it untouched.

Turn checks state and calls the same task creation API as `grid task new`, all
under the signal lock. The task intent is the note title, a blank line, then
`Signal: projects/<p>/notes/<id>.md`. No note body is copied. Task creation
failure leaves the note byte-identical. Concurrent turns create one task and
refuse the losing caller. Task and note publication are separate atomic writes;
a crash or note-write failure after task publication can leave an unlinked task.

JSON retains `result`, `id`, `project`, `note`, and `status`, adding optional
`became` and `until`. Successful changes use `result: updated`; idempotent
repeats use `result: unchanged`.

With `--json`, exit 2 also writes exactly one JSON object line to stdout:
`{"error":"invalid","message":"<msg>"}`. Exit 3 writes
`{"error":"refused","message":"<msg>"}`. The message contains no absolute path;
unknown act values containing path separators are omitted from the JSON message,
and flag parsing errors use `invalid signal arguments`. Existing stderr
diagnostics remain unchanged, including `grid: <msg>` for typed input errors
and refusals. These errors never use `person_required`. A move refused for an
invalid inbox policy exits 1 and writes
`{"error":"invalid_policy","key":"inbox_workspace","message":"<msg>","code":"invalid_policy"}`;
the message is the unprefixed stderr text and contains no path or value. Without `--json`, errors leave stdout empty.

| Exit | Meaning |
| --- | --- |
| 0 | Applied or unchanged |
| 1 | I/O, lock, task creation, unregistered project, or (move) invalid inbox policy failure |
| 2 | Invalid signal id, unknown act, missing/invalid required option, `--task` or `--until` on another act, or any `--later` flag |
| 3 | State refusal or guest caller |

State refusals print exactly one of these single stderr lines:

```text
grid: no such note
grid: this note is not snoozed
grid: this note is not decided
grid: this note is already decided (<status>)
```

Guest refusal prints `grid: guests cannot act on signals`. `--later` is reserved
until task creation supports `start: later`; even `--later=false` is rejected.
The replaceable policy defaults are `inbox_act_roles` (existing owner/member,
local and agent roles, guests refused), `inbox_acts` (the seven acts), and
`inbox_turn_body` (`link`; a future `copy` policy may replace it).
Use `grid workflow show --json --policy` to read effective `inbox_act_roles`;
`inbox_acts` and `inbox_turn_body` are not included in that allowlist.

Project-level `person_verbs` defaults to `[signal act turn, signal act dismiss,
signal act snooze]`: default, owner may change. Set it in leading YAML front
matter in `projects/<name>/workflows.md`, or in workspace `workflows.md` for
inheritance. A project's set value wholly replaces the workspace value, and a
set value replaces the default; `[]` requires no additional person verbs.
A list holds verb names; a map such as `{signal act turn: [owner]}` additionally
restricts person roles, using the same role names as task policy. `signal act`
covers every act, omitted `--act` means `note`, and `signal dismiss` maps to
`signal act dismiss`. Both `--act turn` and `--act=turn` are recognized.
Signal verbs always apply the project policy (falling back to workspace policy,
then defaults), including from a task folder or with `--task`. Any task-local
`workflow.md` policy adds person verbs to that policy; it cannot remove project
requirements. Both policies' matching role restrictions must be satisfied.
Other task-scoped commands continue to use task-local policy.

Policy cannot remove the person-gate floor. Invalid project policy or unreadable
project/workspace files emit a path-free warning and retain the floor, rather
than making every command exit 1. Hook event execution bypasses the gate;
`hook level` remains a person-only settings mutation. The gate runs before
signal dispatch: a person refusal exits 3 and `--json` emits exactly one
`person_required` object; callers passing the gate still face signal role,
guest, input, and state checks. Project policy files retain the workflow reader's
32 KiB limit and symlink refusal. Task person and plan-token policy reads use
the bounded 1 MiB reader and refuse symlinked `workflow.md` files.
