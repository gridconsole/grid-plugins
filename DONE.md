# L39 rework completion

Implemented the review fixes in internal/feed and internal/question without changing protected packages or schemas. Person gates accept only owner, member, lead or the addressed guest; raw agent and CLI answer posts cannot close them. Contract verdicts remain unchanged. Passed defaults release gates from folded records without tick, while lease-protected tick controls remain available for audit history.

Unbacked Q-N.md files remain byte-identical across list, add, answer, tick and sync, are listed with a "not in the Feed" warning and reserve their ids. They cannot be answered through the Feed until migrated. List performs no writes, including locks and Feed caches; sync and mutations repair backed projections. Timestamp parsing errors are handled, lease requirements appear in withdraw/tick help, and the guard names grid answer.

The prior completion notes now live in [docs/lanes/L39.md](docs/lanes/L39.md), including a rework section. [L22 command documentation](docs/lanes/L22.md) reflects the corrected behavior.

## Verification

All acceptance checks passed on the Linux runner:

```sh
make test
make lint
go test -race ./internal/question/... ./internal/feed/... ./internal/cli ./internal/guard
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

`make test` used the default TMPDIR and its temporary-file leak check passed. Focused regression suites cover person roles and raw Feed bypasses, unchanged contract verdicts, defaults without controls, CLI tick controls, file preservation with or without feed/, reserved ids, and read-only listing of damaged or missing projections.

## Intended differences from the old plugin

Reference paths below are relative to the supplied read-only reference tree.

- One CLI answer call replaces prose-directed file editing and the delayed loop bridge. It appends the answer under the person's writer before updating the file. Sources: `docs/brain-feed.md:215-218`, `docs/plugin-design.md:114`, `old-plugin/skills/questions/SKILL.md:47-52`.
- Task selection uses the existing Grid-home resolver and stable Feed task ULID, rather than selecting the only open task or consulting GRID_PROJECTS. No task is created implicitly. Sources: `docs/feed-contract.md:17`, `old-plugin/skills/questions/SKILL.md:8`, `old-plugin-map.md:99`.
- One Q-N.md projection replaces one editable recipient file containing several blocks. Feed records allocate ids and restore projections; old per-recipient Markdown is warned about and left untouched. Unbacked Q-N files remain byte-identical, are listed with a "not in the Feed" warning, and reserve their ids. Changing their decisions requires migration first. Sources: `old-plugin/skills/questions/SKILL.md:8`, `old-plugin/skills/questions/SKILL.md:15-16`, `docs/brain-feed.md:217`.
- Caller identity and role replace reported-person attribution and the loop's forced Owner attribution. Guests are restricted by the addressed id; team roles may answer guest-addressed questions. Sources: `docs/feed-contract.md:32`, `docs/feed-contract.md:90`, `old-plugin/skills/questions/SKILL.md:45`, `old-plugin/skills/questions/SKILL.md:52`.
- Optional options remain available; recommendations must match an option only when options exist. Free text needs no artificial Other option. Optional Job and plan item are structured fields, and Feed ULIDs link the question and history. Sources: `old-plugin/skills/questions/SKILL.md:31-32`, `old-plugin/skills/questions/SKILL.md:37`, `old-plugin/feed/types/question.md:13`, `docs/brain-feed.md:217`.
- Explicit defaults/deadlines, person gates and UTC millisecond timestamps replace local minute stamps and absent deadline fields. A passed deadline applies from folded records at read time, without a new Feed event; tick may record its audit event. Sources: `docs/brain-feed.md:33`, `docs/feed-contract.md:188-198`, `old-plugin/skills/questions/SKILL.md:16`.
- Defaults remain provisional and a late answer preserves their history. Identical retries are harmless; a changed answer after a human decision is refused. Imported conflicting candidates need explicit task-control selection. Sources: `docs/go-architecture.md:25`, `docs/plugin-design.md:26`, `old-plugin/skills/questions/SKILL.md:51`, `old-plugin/feed/types/question.md:15`.
- Blocking questions gain bounded aging at one and two configurable intervals, defaulting to eight hours. The due default takes precedence. Sources: `docs/brain-feed.md:219`; the old question states and fields are at `old-plugin/skills/questions/SKILL.md:30-31`.
- Withdrawal is limited to open questions and records a task-lease control; repeats are harmless. Aging and optional default audit events also use lease-protected control records, rather than mutable local history. Sources: `docs/feed-contract.md:24-32`, `docs/feed-contract.md:85`, `old-plugin/skills/questions/SKILL.md:56`.
- The caller explicitly addresses a recipient, defaulting to owner. Brain/role routing and planning rules remain the planner's responsibility. This command does not enforce prose filters about money, refusal questions or rework. Sources: `old-plugin/skills/questions/SKILL.md:12`, `old-plugin/skills/questions/SKILL.md:14`, `old-plugin/skills/questions/SKILL.md:34-36`.
- History is reconstructed alongside the question instead of directly appending history.md or changing task.md needs, waiting and next fields. Planner resumption remains a consumer of folded Feed state. Sources: `docs/brain-feed.md:218`, `old-plugin/skills/questions/SKILL.md:38-39`, `old-plugin/skills/questions/SKILL.md:48-50`.
- The supported lifecycle is open, answered, defaulted and withdrawn. Setup-only later, disputed and follow-up groups are not introduced. Source: `old-plugin/skills/questions/SKILL.md:30`.

## What remains outside this lane

- No legacy migration or stable task-id backfill was added. Existing Feed identity requirements still apply. File-only questions are preserved and visible, but need migration before their decisions can be changed. Sources: docs/feed-contract.md:17, old-plugin/skills/questions/SKILL.md:8.
- Automatic command/hook/worker ticks, Job resumption, task.md/history.md planner edits and UI rendering require separate integration. Default gate release itself no longer depends on that integration. Sources: docs/go-architecture.md:25, docs/brain-feed.md:218, old-plugin/skills/questions/SKILL.md:48-50.
- Direct contract.Fold callers retain the protected package's explicit-selection semantics. The separate person-gate layer in internal/feed preserves its verdicts. Sources: docs/feed-contract.md:177, docs/feed-contract.md:188-197.
- Native macOS and Windows execution is unavailable in this Linux workspace; the required cross-builds check compilation. Existing fslock and atomicfile helpers provide platform handling.
