# D-plan-hold: the plan hold and the fixed block levels are removed

Status: DONE after fix round 1, except one row: the new tests were not run against
base (see the table). Code head before this report commit: 9f27e4cc.

## Proof table

| Row | State | Evidence |
| --- | --- | --- |
| 1. List of every guard, hold, fixed level | HOLDS | table "Row 1" below |
| 2. Plan hold gone: an unaccepted plan no longer blocks repo writes | HOLDS | `go test ./internal/guard/` exit 0; `TestUnacceptedPlanDoesNotBlockRepoWrites` (internal/guard/plan_no_hold_test.go); `TestForgedPlanDigest` now asserts `guard.EvaluatePlan` gives no deny for a forged plan while `task.Approved` is false and `task.PlanHeldWith` is true (internal/orchestrator/plan_digest_test.go:108-130), `go test ./internal/orchestrator/` exit 0. The deny that was internal/guard/plan.go:125-138 is gone. Whether a plan needs approval is an `approve` step of the workflow: docs/settings.md:127-131 |
| 3. No guard level fixed at block, documented | HOLDS | Code: `neverOff`/`AlwaysBlock` deleted from internal/settings/catalog.go (on main they are at catalog.go:35-38, and `checkLevel` refuses off/warn at :61-67); `checkLevel` is now `allowed("hook level", level, levels)` (catalog.go:54-57); `ResolveHookLevel` returns the stored level (store.go:171-199); the four `Evaluate*` read it through `configuredLevel` (guard/plan.go:22, host.go:94, writer.go:16, brief.go:90). Tests, all exit 0: `TestFourGuardsFollowTheirLevel` (guard), `TestGuardLevelsAreSettableAndResolved` (settings/store_test.go), `TestHookLevelAcceptsEveryLevelForTheFourGuards` (cli/settings_test.go), `TestHookFollowsTheOnlyWriterLevel` (hook/settings_test.go). Docs: docs/settings.md:101-106, :119-133 |
| 4. Tests updated; nothing outside the list removed; go test and make lint 0 fail | HOLDS for the changed packages and their importers | `go test ./internal/guard/ ./internal/hook/ ./internal/settings/ ./internal/task/ ./internal/cli/ ./internal/golden/ ./internal/orchestrator/ ./internal/braincmd/ ./internal/doctor/ ./internal/job/... ./internal/land/ ./internal/signal/` exit 0, every package `ok`. `make lint` (`go vet ./...`) exit 0. The whole-module `go test ./...` is the orchestrator's at landing |
| New tests fail on base and pass on the lane | NOT DONE (not run on base) | Not run against `origin/main`. They fail there by construction: main's `checkLevel` refuses `off` and `warn` for the four guards (`git show origin/main:internal/settings/catalog.go`, lines 61-67: "%s always blocks and cannot be set to %s; allowed: block"), and the new tests call `SetHook(name, "off")` and `SetHook(name, "warn")` and require success. Before fix round 1 the lane head did not compile its settings tests: `internal/settings/store_test.go:140:7: undefined: AlwaysBlock`, `:156:23: undefined: neverOff`, `:189:6: undefined: AlwaysBlock` |
| Tests of changed packages and their importers | HOLDS | list below; command in row 4 |
| lint, Windows, macOS builds | HOLDS | `make lint` exit 0; `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| No new guard, gate, cap or hold | HOLDS | the diff deletes rules and routes the four guards through the existing `configuredLevel`; the one added function is the `evaluateOnlyWriter` wrapper split in writer.go; the rest of the fix round is tests |
| Small commits, no attribution lines, no personal paths | HOLDS | `git log origin/main..` |

Importers of the changed packages (direct, from `go list` over Imports and TestImports of guard, settings and task): braincmd, cli, doctor, guard, hook, job, job/jobcmd, job/jobtest, land, orchestrator, signal; golden runs them through cli. Each was tested (job/jobtest has no test files).

## Row 1: every guard, hold and fixed level in internal/guard and internal/hook

| Item | Fate |
| --- | --- |
| Held-write deny (plan.go, was :125-138): repo writes refused while the plan is not accepted | REMOVED |
| Odd-name deny inside an unaccepted task folder (was :67-69) | REMOVED (exists only for the hold) |
| Forged-acceptance denies, `leavesPlan` (was :70-84, :176-212) | REMOVED |
| `isHeld`, `heldStage`, `planStageWith`, `planAcceptedMark` (guard/context.go) | REMOVED (unused) |
| Fixed block level of brief-lint, host-guard, only-writer, plan-guard (`neverOff`, `AlwaysBlock`, the `checkLevel` refusal, the CLI text) | REMOVED. Default stays block |
| Gate-name deny at the top of a task folder, `history.md` alias and preview-approved line (plan.go :49-60) | KEPT: not the plan hold; the gate-file rows are a separate lane (S6c-map.md host.go/host_targets.go rows) |
| plan.md brain-recall check on Bash (`recallGate`, recall.go) | KEPT: S6c-map.md:14-16, "Leave them". It still asks `held()` to know the plan is being written |
| Plan-ready needs recall line (plan.go :61-65) | KEPT, same line |
| Planner/curator/reviewer/checker role fences and brain-write roles (plan.go) | KEPT: roles are product, S6c-decisions.md Q3. The planner fence still asks `held()`, so it never narrows |
| Host guard: gate files, settings-file and setup-verb denies (host.go) | KEPT: S6c-decisions.md, "the Edit deny rules that setup writes stay". Level now follows the setting |
| Only-writer, brief-lint (writer.go, brief.go) | KEPT, level now follows the setting |
| Home-allow (home.go) | already configurable |
| Oversize tool-call deny (hook.go:238-247) | KEPT: input the guards cannot read |
| Usage budget hold on agent start (`usageHold`, hook.go:284-313) | KEPT, and flagged. It acts only when a person has set a budget (no budget, no hold, hook.go:286-287). I found no decision line that keeps or removes it; the brief names only the plan hold and the fixed levels. The orchestrator should rule |
| Stop-check (hook/stop.go) | KEPT: already a configurable level |

## What fix round 1 changed

1. internal/settings/store_test.go: deleted `TestAlwaysBlockHooksRefuseOffAndWarn` and the `AlwaysBlock` asserts in `TestAllowedValues`; dropped the two cases and the two hand-written documents that expected `plan-guard warn` / `host-guard off` / `only-writer warn` to be refused (`TestInvalidChangesLeaveBytes`, `TestInvalidDocumentsWriteNothing`; `plan-guard never` is still refused). Added `TestGuardLevelsAreSettableAndResolved`: off, warn and block are accepted for the four guards, read back, read from a settings file, the project level wins, and `ResolveHookLevel` returns the set level.
2. internal/settings/testdata/claude/writer.go.golden: refreshed to the new writer.go. `TestClaudeWriterReservedKindsStayInSync` compares this snapshot byte for byte; the writer.go diff is the level wrapper and a comment only (git diff origin/main.. -- internal/guard/writer.go), the reserved-kind predicates are unchanged, and `TestClaudeDenyRulesEqualWriterGuard` (internal/cli/setup_claude_test.go:159) passes.
3. internal/cli/settings_test.go: dropped `host-guard off` and `only-writer warn --project` from the refused commands in `TestSettingsCLIOverridesAndFailures`; replaced `TestHookLevelRefusesToLowerAlwaysBlockGuards` with `TestHookLevelAcceptsEveryLevelForTheFourGuards` (the command exits 0 for off, warn, block, personal and per project, and `hook levels --json` shows the level). internal/cli/settings_choke_test.go:101: the `AlwaysBlock` entry removed from the exports map, since the function no longer exists.
4. internal/hook/settings_test.go: replaced `TestHookKeepsAlwaysBlockGuardsWhateverTheSettingsSay` with `TestHookFollowsTheOnlyWriterLevel` (pretooluse and permissionrequest; none/off/warn/block/malformed: block and an unreadable file deny, warn lets the write through with the denial as `systemMessage`, off does not mention the guard).
5. Golden: dropped the case `plan-guard/held-repo` (golden/plan-guard/held-repo, its line in golden/INDEX.md, its `add(...)` in recordings/cases.py).
6. internal/orchestrator/plan_digest_test.go `TestForgedPlanDigest`: not deleted. The two `guard.EvaluatePlan ... "deny"` assertions became task-level ones: for the forged plan, `task.Approved` is false and `task.PlanHeldWith` is true, also with a legacy history line naming the original hash; the original approved plan reads as approved and not held; the guard gives no deny. The digest check in task/hold.go and gates.go is untouched.

## Deleted tests

Each assertion was that the removed hold or fixed level exists. The behaviour was
removed by the owner's direction of 9 Nov (no Grid-made guards, approvals from the
workflow with a default), as the brief D-plan-hold.md and S6c-map.md (S6c5 row,
"behaviour removed by the owner's direction") write it. Nothing else covers them.

- internal/guard: write_hold_test.go, plan_hold_named_test.go, plan_stage_steps_test.go, deliver_hold_test.go, approval_test.go (whole files).
- internal/guard/plan_edges_test.go: `TestPlanStages`, `TestEdits`, `TestExplicitFilesystemBase` (`TestRecallBash`, `TestRecallLineForms` kept as they were).
- internal/guard/guard_test.go `TestPlanHeaderRules`: cases "held task keeps writes in projects", "acceptance is the history event", "inactive task releases hold", "missing task remains held", "cannot forge acceptance", "cannot move step past gate", "cannot remove approval workflow", "cannot close task", "cannot corrupt readable task", "unreadable task can stay unreadable", "unknown edit is refused", "nontext writes are refused", "notebooks cannot edit task controls", "top names printable ASCII", "nested names no alternate streams", "parts no trailing dot", "parts no trailing space", "switch releases plan guard", "main agent remembered task", "child inherits main hold", "child owns hold without siblings", "legacy session list fallback", "symlink write follows target", "dangling symlink follows target". The planner, curator, brain, tasks-folder, preview, plan-ready, gate-name, reviewer and checker cases stay and pass.
- internal/guard/settings_test.go: `TestAlwaysBlockGuardsIgnoreHookLevels` replaced by `TestFourGuardsFollowTheirLevel`.
- internal/settings/store_test.go: `TestAlwaysBlockHooksRefuseOffAndWarn` replaced by `TestGuardLevelsAreSettableAndResolved`.
- internal/cli/settings_test.go: `TestHookLevelRefusesToLowerAlwaysBlockGuards` replaced by `TestHookLevelAcceptsEveryLevelForTheFourGuards`.
- internal/hook/settings_test.go: `TestHookKeepsAlwaysBlockGuardsWhateverTheSettingsSay` replaced by `TestHookFollowsTheOnlyWriterLevel`.
- golden case `plan-guard/held-repo` ("A held task cannot write into the repository."): the behaviour was removed by the owner's direction of 9 Nov, as above. The corpus records what the legacy script did, so the recording is dropped, not edited to an allow. Still covering the neighbours: `plan-guard/accepted-repo`, `plan-guard/guard-off`, `plan-guard/task-write`; for a plan that is not accepted writing into the repo, `TestUnacceptedPlanDoesNotBlockRepoWrites`.

## Open items

- `GRID_PLAN_GUARD=off` still suppresses plan checks (docs/settings.md:148); not touched.
- The usage budget hold (row 1) needs the orchestrator's ruling.
- The new tests were not run on base (row above).

## NOT VERIFIED on a live screen

Nothing here is a UI change. A hook run in a live Claude Code session was not tried: the guard decisions were checked through `EvaluatePlan`, the adapters and `hook.Run` only.
