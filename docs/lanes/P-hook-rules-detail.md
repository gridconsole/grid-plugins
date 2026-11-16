# P-hook-rules-detail: each hook rule's events and description, and task and reason on a refusal

Brief: owner order 15 Nov 20:50. Split A and B of the app lane's report (U-r26-hooks-rows, "Plugin split").

## Output shape for the app lane

### A: `grid hook levels --json --detail`

An array in the order of `grid hook levels` (alphabetical), one object per rule, four keys each:

```json
[
  {"n":"history-log","level":"warn","events":[{"event":"PreToolUse"},{"event":"PostToolUse"},{"event":"SubagentStop"}],"d":"Writes file edits, Bash commands, finished subagents and brain reads to the task history."},
  {"n":"session-start","level":"warn","events":[{"event":"SessionStart","matcher":"startup|resume|clear|compact|fork"}],"d":"Gives a starting session its Grid context."},
  {"n":"job-lint","level":"block","events":[],"d":"Checks a Job when grid job render builds it; block refuses a Job with lint errors."}
]
```

- `level` is the level of the current project, as `grid hook levels --json` gives it.
- `events` is always an array. Each `event` is a name from hooks/hooks.json; `matcher` appears only where hooks.json gives that event one (SessionStart). A rule no hook event runs has `[]`: host-guard (retired), only-writer (retired), needs-you, session-check (no hook runs them) and job-lint (runs in `grid job render`). The app should draw `[]` as "no hook event", not as an event.
- `d` is one line of plain text.
- The table is `hookInfos` in internal/settings/catalog.go, next to `hooks`. Events per rule (read from the code that consults the rule, internal/hook): history-log PreToolUse, PostToolUse, SubagentStop; home-allow PermissionRequest, PreToolUse; plan-guard PreToolUse, PermissionRequest; session-end SessionEnd, SubagentStop; session-start SessionStart; stop-check Stop, SubagentStop; usage-log SessionStart, SubagentStop, PostToolUse.
- Without `--detail` the output is byte for byte what it was (`{"history-log":"warn",...}`), pinned by TestHookLevelsWithoutDetailIsUnchanged. `--detail` without `--json` prints `<rule> <level> [<event>,...] <d>`.
- The app sorts by event itself; the table order is alphabetical by rule.

### B: `grid hook events --all --json` (and the app's hook_events?all=1)

A line is the old four keys plus two optional ones:

```json
{"rule":"stop-check","event":"Stop","at":"2026-11-16T09:00:00Z","outcome":"blocked","task":"T-10","reason":"Result of Job J-3 not posted on the Feed"}
```

- `task` is the task folder's id (`T-10`, cut from `T-10-rate-limits`), `reason` is one line. Both are set only on a `warned` or `blocked` line of `stop-check` (Stop, SubagentStop) or `job-lint`. Both keys are absent otherwise; an old line reads with both empty.
- job-lint did not write to this log before. `grid job render` now appends a line when lint finds errors: `event` is `"grid job render"` (not a hook event name), `outcome` is `blocked` at block and `warned` at warn, `reason` is `<lint rule>: <first error message>`. At off, or with no lint error, it writes nothing.
- stop-check reason: `Result of Job <id> not posted on the Feed`.
- plan-guard and home-allow lines carry no task or reason (not asked).

## Proof table

| Row | State | Command / where |
|---|---|---|
| merge origin/main first | HOLDS | `git fetch origin && git merge origin/main`: Already up to date (before the work and before the last runs) |
| A: --detail from one table in catalog.go | HOLDS | internal/settings/catalog.go `hookInfos`, `HookRules`; internal/cli/settings.go `--detail` |
| A: table and hooks.json agree, test fails when not | HOLDS | internal/settings/hook_rules_test.go TestHookRuleEventsAreWired reads hooks/hooks.json |
| A: without --detail byte for byte | HOLDS | TestHookLevelsWithoutDetailIsUnchanged (exact string) |
| B: Record task and reason optional | HOLDS | internal/hookevents/hookevents.go Record; TestRecordTaskAndReasonAreOptional (old line reads; a record without them writes the four old keys) |
| B: set by stop-check | HOLDS | internal/hook/stop.go (`o.runs.why`), ruleruns.go; TestRuleRunsStopCheckRecordsTaskAndReason |
| B: set by job-lint | HOLDS | internal/job/jobcmd/command.go `recordJobLint`; TestRenderRecordsTheJobLintRefusal |
| New tests FAIL on base | HOLDS | see below |
| No rule's behaviour, level or block decision changed | HOLDS | diff: hook outcomes, exit codes and stdout untouched; the new lines are appended after the decision (flushRuns) and `recordJobLint` ignores its failure like every hook record. Existing hook, job and cli tests pass unedited |
| go test of the changed packages and their importers | HOLDS | `go test -count=1 ./cmd/grid ./internal/account ./internal/brain ./internal/cli ./internal/doctor ./internal/guard ./internal/hook ./internal/hookevents ./internal/job/jobcmd ./internal/packaging ./internal/settings ./internal/updates ./lib` exit 0, all ok (cmd/grid and updates have no test files) |
| make lint | HOLDS | exit 0 (`go vet ./...`) |
| GOOS=windows go build ./... | HOLDS | exit 0 |
| GOOS=darwin go build ./... | HOLDS | exit 0 |
| existing hook tests unedited | HOLDS, one forced addition | no existing test changed behaviour. internal/cli/settings_choke_test.go gets one map entry, `"HookRules": "catalog.go"`: its reviewed-exports table fails ("unreviewed settings export HookRules") for any new exported function in internal/settings (runbook 13: a table the changed code forces). contract/commands.json gets the `--detail` flag the same way (TestContractCommandsParserInventory) |
| Walk: TestHsIpWalksEveryCommand, TestIlGuestSiteList | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok |
| Walk: internal/job/lint | HOLDS | `go test -count=1 ./internal/job/lint/` ok |
| Windows and macOS builds after merge | HOLDS | run after the last merge (Already up to date) |
| No guard, gate, cap or hold added | HOLDS | only records and a read-only listing |
| No attribution lines | HOLDS | commits carry none |

### New tests on base

With the source changes stashed and the new test files in place:

```
internal/hookevents/hookevents_why_test.go:22:85: unknown field Task in struct literal of type Record
internal/settings/hook_rules_test.go:37:11: undefined: HookRules
--- FAIL: TestRuleRunsStopCheckRecordsTaskAndReason (0.01s)
    ruleruns_why_test.go:52: stop-check block: line {Rule:stop-check Outcome:blocked Task: Reason:}, want task ... and a reason naming Job J1
--- FAIL: TestRenderRecordsTheJobLintRefusal/block (0.03s)
    job_lint_record_test.go:33: open .../.hook-counts/events.jsonl: no such file or directory
--- FAIL: TestHookLevelsDetailShape (0.03s)
    hook_levels_detail_test.go:39: exit 2: flag provided but not defined: -detail
```

(The first run found the task folder is `T-01-cache-configuration`, so the id is cut to `T-01` by `hookevents.TaskOf`; the test now pins that.)

## What changed

- internal/settings/catalog.go: `HookEvent`, `HookRule`, `hookInfos`, `HookRules`.
- internal/cli/settings.go: `grid hook levels --detail`; contract/commands.json and contract/README.md follow.
- internal/hookevents/hookevents.go: `Record.Task`, `Record.Reason`, `TaskOf`.
- internal/hook/ruleruns.go, stop.go: stop-check keeps its task and reason on the line it writes.
- internal/job/jobcmd/command.go: `recordJobLint`.
- New tests: internal/settings/hook_rules_test.go, internal/cli/hook_levels_detail_test.go, internal/hookevents/hookevents_why_test.go, internal/hook/ruleruns_why_test.go, internal/job/jobcmd/job_lint_record_test.go.

## NOT VERIFIED on a live screen

- The Settings > Hooks page: this lane has no UI. The app lane reads the shapes above.
- Not run, not asked: the whole-module `go test ./...` and the cross-repo set (the orchestrator runs them at landing).

## Open for the owner

- The event column for host-guard, only-writer, needs-you and session-check is `[]`: no hook runs them. The design draws an event for every rule; the app lane decides how to show a rule with none.
- job-lint's line has `event: "grid job render"`, which is not a hooks.json event. A reader that joins log events to `events[].event` will not match it.

DONE
