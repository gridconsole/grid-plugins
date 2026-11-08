# L41: usage logging and notices

Built `internal/usage` and the `grid usage` CLI adapter. The command table has one registration change. The L20 status line now marks the usage half merged; its artifact text is unchanged. No dependencies, contract changes, hook wiring, task/history writes, migration, or desktop UI changes were added.

## Commands

```sh
grid usage post --task T-01 --job J-1 --dollars 2.50 --minutes 8 --session session-1 --model example-model --json
grid usage post --task T-01 --job lead --minutes 2
grid usage show --task T-01 --json
grid usage budget set --task T-01 --dollars 200 --minutes 120
grid usage budget show --task T-01 --json
grid usage threshold set --task T-01 --dollars 50 --dollars 80 --dollars 100 --minutes 50
grid usage threshold show --task T-01 --json
grid usage threshold clear --task T-01 --dollars
grid usage budget clear --task T-01 --minutes
```

All commands accept `--task` and `--json`; task selection uses the existing `where` resolver. Inside a task folder, `--task` is optional. Each post is a delta, including repeated calls with the same session identifier. At least one amount is required, and an explicitly supplied zero is valid. No stdin payload is read. Missing or invalid flags and positional arguments are rejected. Dollar and minute amounts must be finite and non-negative; aggregate overflow is also rejected before appending. Job IDs have positive numeric components, or are exactly `lead`.

The log is `<task>/usage/usage.jsonl`. Each line contains `v:1`, a UTC timestamp with millisecond precision, `job`, `session`, `model`, `dollars`, and `minutes`. Omitted amounts become zero. Omitted session/model labels become empty strings. `show` folds the complete log into lexically ordered per-Job rows and task totals, including post counts; every Lead wake contributes to the single `lead` row. No log trimming loses accumulated usage. An unterminated tail is ignored on read and truncated by the next post; every invalid complete line fails with its line number, without rewriting the log.

## Notice rule

The correction in the lane request takes precedence over the original absolute-dollar examples: a threshold is a percentage of the task's budget. For example, with a dollar budget of 200, the 50% notice fires at a dollar total of 100, and its body is `{task: <stable task ULID>, threshold: 50, dollars: 100}`. A budget may exceed 100; a percentage may not. The schema's 0–100 cap is correct and unchanged, not a bug or a proposed schema change.

One `internal/fslock` lock at `usage/.lock` covers each append, totals fold, threshold/budget update, duplicate check, Feed post, and fired-state update. Settings use the existing atomic writer. The task ULID comes from `feed.TaskID`; every notice goes through `feed.Store.Post` with `author.role: cli`, no lease, and exactly one amount. The CLI reuses the existing Feed identity helper, resolving it only if a notice needs writing. An agent invoking `usage post` does not become the author of a raw Feed record; the CLI writes the notice. Direct agent-role `usage-notice` posts remain rejected by the Feed contract.

For each configured unit, the amount at percentage P is `budget * (P / 100)`. A notice is eligible when usage has crossed that amount since configuration and the task/unit/percentage has not fired. Ordinary posts therefore implement `previous_total < amount <= new_total`. Percentages are evaluated ascending within dollars, then ascending within minutes. Crossing both units produces separate records. Each record's amount is the new total, not the delta or the budget. A 0% threshold is accepted by the schema but cannot be crossed from non-negative usage.

Configuration records a baseline total for each changed unit. Setting a budget or lowering/adding a threshold below the current total never fires retroactively. Keeping that baseline until configuration changes also allows a later post to retry a crossing whose notice failed, including a crash immediately after appending usage. A zero delta can retry notices without adding usage. Retrying a nonzero delta adds it again; usage posts themselves have no deduplication key.

Before posting, the implementation checks admitted, validated `usage-notice` entries from `feed.Store.Read` for the same task, unit, and percentage. The contract's task-state fold deliberately discards notice bodies, so its `State` projection cannot supply this check. The order is Feed post, then atomic settings write marking `fired`. If the second step fails, the next post finds the existing record and repairs `fired` without posting again. New notices are returned separately from recovered existing notices. Fired identity survives threshold removal, re-addition, and budget changes.

A Feed, identity, or notice-settings failure after a durable log append yields a warning on stderr and in JSON; command exit remains 0 when output succeeds. A log validation/write failure is an error. Notices do not change task files, Job state, or the contract's Needs-you projection. The desktop app consumes Feed notices for its own display and notifications.

## Defaults and decisions

- **Default, owner may change:** percentages are 50, 80, and 100 for each unit; neither budget is initially set, so no notices fire. This follows the corrected request, replacing the earlier absolute-amount interpretation.
- **Default, owner may change:** settings live at `<task>/usage/thresholds.json`, version 1, with `dollars` and `minutes` percentage arrays, `budget` and `baseline` maps, and a `fired` map from `unit:percentage` to Feed record ULID. There are at most eight distinct percentages per unit, sorted and deduplicated.
- **Default, owner may change:** `usage budget set/show/clear` supplies the otherwise unspecified budget interface. Budgets must be positive finite amounts; use `clear` to disable one. Budget amounts have no 100-unit cap.
- **Default, owner may change:** `set` replaces only supplied units; omitted units stay unchanged. `clear` without selectors clears both units. Threshold clearing retains budgets, and budget clearing retains thresholds. Neither clears `fired`. Changing a unit snapshots its current total as its baseline and replaces that unit's previous eligibility window.
- **Default, owner may change:** notice failures after a successful log append warn and leave exit 0; later posts retry eligible notices. Checks and notices are synchronous under the usage lock.
- **Default, owner may change:** session/model labels are optional, bounded to 160 characters, and restricted to identifier/name characters. Session labels match the existing session key character convention. They are not paths to open, and no session records are modified or required to exist. Prompt text and transcript-path fields are excluded.
- **Default, owner may change:** this lane builds explicit CLI commands only. Nothing chooses between a future hook or agent caller, measures minutes, estimates prices, or wires runtime reporting. Minutes and dollars come directly from each post.

## Intended differences from the old plugin

Reference paths below identify the supplied read-only material, not copied files. Some line references in the brief predate that material: the supplied `prototype/audit/decisions.md` has 49 lines, so its cited line 75 is unavailable. The current task prototype's usage rows begin at line 5068, rather than the brief's line 5150. The lane request and its percentage correction govern those differences.

| Difference | Reference and reason |
| --- | --- |
| Explicit Go commands accept deltas per Job and one combined `lead` row, instead of hook-event snapshots, Agent token totals, and transcript-derived costs. No token/pricing or elapsed-time inference is added. | `old-plugin/hooks/usage-log.mjs:1`, `:28`, `:59`; `docs/old-plugin-map.md:33`; `docs/plugin-design.md:216`; `prototype/audit/ui-task.md:88`; `prototype/task.html:5068`. |
| One task log replaces per-session `.usage` destinations for new data. Existing `.usage` files remain untouched and are not migrated. Session and model labels remain optional metadata. | `old-plugin/hooks/lib.mjs:594`, `:599`; `docs/old-plugin-map.md:76`. The lane request chooses `usage/usage.jsonl`. |
| Keep the full usage history; serialize reads, tail repair, append, and notices under a persistent lock, sync the usage append, and reject corrupt complete lines. The old writer used one append call, dropped lines above 4,000 characters, and trimmed over 2,000 lines down to 1,500. | `old-plugin/hooks/lib.mjs:590`, `:595`, `:599`, `:612`. Lifetime totals must not lose older deltas. |
| No transcript paths, prompt content, brief references, hook-event metadata, response keys, token snapshots, or runtime response payloads are collected. Invalid labels or numbers are rejected instead of silently dropping unsupported values. | `old-plugin/hooks/usage-log.mjs:8`, `:26`, `:32`, `:39`, `:48`, `:59`; `docs/old-plugin-map.md:33`. |
| No SessionStart, PostToolUse, or SubagentStop wiring, trimming hook, network notification, or desktop callback is added. The caller must explicitly report usage. | `old-plugin/hooks/usage-log.mjs:1`, `:17`, `:22`; `docs/old-plugin-map.md:8`, `:17`, `:19`, `:47`. |
| Invalid input or a failed usage write returns an error; a failed notice after a successful append warns and returns success. The old hook swallowed recording errors so the session continued. | `old-plugin/hooks/usage-log.mjs:12`; `docs/lanes/L5.md:34`. |
| New CLI-authored notices are contract-validated, with exactly one amount per record and persistent duplicate suppression. The usage log stays separate from the Feed; neither per-post usage nor task-state changes are posted. Desktop notification and Needs-you presentation are left to the app. | `docs/feed-contract.md:66`, `:113`; `docs/lanes/L5.md:84`; `prototype/audit/ui-task.md:18`, `:60`; `prototype/task.html:5078`. |
| Budgets and percentage thresholds replace the absolute-dollar/minute examples in the older design and prototype. They never enforce spending holds or stop a Job. | `docs/plugin-design.md:216`; `prototype/audit/ui-task.md:60`; the lane request's explicit percentage correction overrides these examples. Agent-account usage-limit holds remain separate. |

## Validation and limits

Passed on Linux with the caller's default TMPDIR:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
go test -race ./internal/usage ./internal/cli
```

`make test` includes the temporary-file leak check and the existing Feed role-gate tests. New tests cover invalid and finite amounts, omitted amounts, Job IDs and Lead aggregation, metadata restrictions, stdin/flag errors, unknown tasks, golden JSON output, torn tails, corrupt line numbers, percentage configuration, positive budgets over 100, no retroactive firing, exact and multiple crossings, both units, contract validation, CLI author and task identity, failure recovery, and 32 concurrent posters with no lost lines or duplicate notices. The injected failure occurs after the real Feed post and before the settings write, followed by a real retry. Race tests passed for usage and CLI.

Native macOS execution was not available; Darwin compilation passed, and the implementation uses the existing portable lock/atomic helpers without assuming temporary-directory paths are canonical. Windows compilation passed; usage operations that need a lock inherit `fslock.ErrUnsupported` on native Windows, as the existing repository's Windows Feed locking implementation is intentionally unavailable. No new platform-specific locking implementation was added.

Power-loss durability of chatter notices remains the Feed store's existing policy: it appends chatter without the synchronous flush used for state records (`internal/feed/store.go:316`). The required process-crash recovery is tested; this lane does not alter Feed durability. Desktop display/notifications, hooks, old-log migration, automatic time measurement, and account holds are excluded by the lane's scope. All requested CLI and local storage work is complete.
