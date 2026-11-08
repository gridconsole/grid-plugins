# L39 completion

Built the top-level `grid answer <task> Q-N "text"` command, removed its nested alias and attribution flags, and made question files recoverable projections of the Feed. Add and answer use the caller's fence-0 writer; answers reference the Feed question ULID. List, answer and the new `grid question sync [task]` reconcile missing, corrupt or hand-edited files. The existing question lock and atomicfile helper protect projection writes. No protected Go package or go.mod was changed; cli.go changes only register commands and their descriptions.

The question-file schema now carries optional Job and plan item, required question post ULID, history post ULIDs and caller roles, optional options and a conditional recommendation. The file gate is the posted person gate. The question post also retains recommendation and aging metadata for recovery. Guests can answer only questions addressed to them; team roles may answer any recipient's question. Existing host and question-file guards remain unchanged.

The Feed adapter closes person gates with a single distinct eligible answer and preserves explicit task-control selection for imported conflicts. Withdrawal, aging and defaults now have Feed control events and recover with the files. Those writes require an existing task lease and handover, using GRID_LEASE and GRID_LEASE_EPOCH; fence-0 answers require neither.

## Tests

All required checks passed on the Linux runner:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

`make test` used the default TMPDIR and passed its temporary-file leak check. The cross-builds verify compilation, not native macOS or Windows execution. Focused tests: `go test ./internal/question ./internal/feed ./internal/cli ./internal/guard ./internal/contract`. Coverage includes Feed/file agreement, recovery from deletion and edits, free text, role stamping, allowed/refused guest answers, removed aliases/flags, schema fixtures, person-gate conflicts and selection, stale controls, concurrent ticks/allocation and existing symlink/guard cases.

## Intended differences from the old plugin

Reference paths below are relative to the supplied read-only reference tree.

- One CLI answer call replaces prose-directed file editing and the delayed loop bridge. It appends the answer under the person's writer before updating the file. Sources: `docs/brain-feed.md:215-218`, `docs/plugin-design.md:114`, `old-plugin/skills/questions/SKILL.md:47-52`.
- Task selection uses the existing Grid-home resolver and stable Feed task ULID, rather than selecting the only open task or consulting GRID_PROJECTS. No task is created implicitly. Sources: `docs/feed-contract.md:17`, `old-plugin/skills/questions/SKILL.md:8`, `old-plugin-map.md:99`.
- One Q-N.md projection replaces one editable recipient file containing several blocks. Feed records allocate ids and restore projections; old per-recipient Markdown is warned about and left untouched. Unbacked Q-N files are removed, so file-only data requires migration first. Sources: `old-plugin/skills/questions/SKILL.md:8`, `old-plugin/skills/questions/SKILL.md:15-16`, `docs/brain-feed.md:217`.
- Caller identity and role replace reported-person attribution and the loop's forced Owner attribution. Guests are restricted by the addressed id; team roles may answer guest-addressed questions. Sources: `docs/feed-contract.md:32`, `docs/feed-contract.md:90`, `old-plugin/skills/questions/SKILL.md:45`, `old-plugin/skills/questions/SKILL.md:52`.
- Optional options remain available; recommendations must match an option only when options exist. Free text needs no artificial Other option. Optional Job and plan item are structured fields, and Feed ULIDs link the question and history. Sources: `old-plugin/skills/questions/SKILL.md:31-32`, `old-plugin/skills/questions/SKILL.md:37`, `old-plugin/feed/types/question.md:13`, `docs/brain-feed.md:217`.
- Explicit defaults/deadlines, person gates and UTC millisecond timestamps replace local minute stamps and absent deadline fields. A passed deadline is applied by tick. Sources: `docs/brain-feed.md:33`, `docs/feed-contract.md:188-198`, `old-plugin/skills/questions/SKILL.md:16`.
- Defaults remain provisional and a late answer preserves their history. Identical retries are harmless; a changed answer after a human decision is refused. Imported conflicting candidates need explicit task-control selection. Sources: `docs/go-architecture.md:25`, `docs/plugin-design.md:26`, `old-plugin/skills/questions/SKILL.md:51`, `old-plugin/feed/types/question.md:15`.
- Blocking questions gain bounded aging at one and two configurable intervals, defaulting to eight hours. The due default takes precedence. Sources: `docs/brain-feed.md:219`; the old question states and fields are at `old-plugin/skills/questions/SKILL.md:30-31`.
- Withdrawal is limited to open questions and records a task-lease control; repeats are harmless. Defaulting and aging also retain lease-protected control records, rather than relying on mutable local history. Sources: `docs/feed-contract.md:24-32`, `docs/feed-contract.md:85`, `old-plugin/skills/questions/SKILL.md:56`.
- The caller explicitly addresses a recipient, defaulting to owner. Brain/role routing and planning rules remain the planner's responsibility. This command does not enforce prose filters about money, refusal questions or rework. Sources: `old-plugin/skills/questions/SKILL.md:12`, `old-plugin/skills/questions/SKILL.md:14`, `old-plugin/skills/questions/SKILL.md:34-36`.
- History is reconstructed alongside the question instead of directly appending history.md or changing task.md needs, waiting and next fields. Planner resumption remains a consumer of folded Feed state. Sources: `docs/brain-feed.md:218`, `old-plugin/skills/questions/SKILL.md:38-39`, `old-plugin/skills/questions/SKILL.md:48-50`.
- The lifecycle remains open, answered, defaulted and withdrawn. Setup-only later, disputed and follow-up groups are not introduced. Source: `old-plugin/skills/questions/SKILL.md:30`.

## Limits and reasons

- The protected internal/contract fold still requires explicit answer selection. The person-gate behavior is implemented in internal/feed around that fold; direct contract.Fold callers retain the older semantics. No lease rule was weakened. Withdrawal/tick callers need a task tenure because controls are lease-bound under `docs/feed-contract.md:85`.
- Automatic ticks on every command/hook/worker pass and Job resumption are not wired here: they require dispatcher/Orchestrator integration beyond command registration. This lane updates question and Feed projections without issuing Job status controls. Sources: `docs/go-architecture.md:25`, `docs/brain-feed.md:218`.
- No legacy migration or stable task-id backfill is performed. Existing Feed requirements apply; old file-only L22 and recipient-file data must be migrated before using Feed reconciliation. UI rendering, landing summaries and task/history planner edits remain separate consumers. Sources: `docs/brain-feed.md:33`, `old-plugin/skills/questions/SKILL.md:8`, `old-plugin/skills/questions/SKILL.md:48-50`.
- Native macOS and Windows tests were unavailable in this Linux workspace. Both required cross-builds pass; locking and no-follow handling use the existing platform helpers.

Work is committed in small steps on the supplied current branch; no branch switch or merge into another branch was performed.
