# D-plan-hold: the plan hold and the fixed block levels are removed

Status: PARTLY DONE. The production change and the guard tests are done; three
test files of other packages still pin the old fixed levels and fail, and lint, the
Windows and macOS builds and the importing packages were not run (60-call budget).

## Proof table

| Row | State | Evidence |
| --- | --- | --- |
| 1. List of every guard, hold, fixed level | HOLDS | table below |
| 2. Plan hold gone: an unaccepted plan no longer blocks repo writes | HOLDS | `go test ./internal/guard/` exit 0; `TestUnacceptedPlanDoesNotBlockRepoWrites` (internal/guard/plan_no_hold_test.go); the deny that was internal/guard/plan.go:125-138 is gone |
| 3. No guard level fixed at block, documented | PARTLY | Code: `neverOff`/`AlwaysBlock` deleted (internal/settings/catalog.go), `ResolveHookLevel` no longer short-circuits (store.go:171-200), the four `Evaluate*` read their level through `configuredLevel` (guard/plan.go:22, host.go:94, writer.go:16, brief.go:90); `TestFourGuardsFollowTheirLevel` passes. Docs: docs/settings.md:28, :103-106, :119-133. NOT DONE: the tests below that still pin the old rule |
| 4. Tests updated; go test ./... and make lint 0 fail | NOT DONE | only `go build ./...` (exit 0) and `go test ./internal/guard/` (exit 0) were run |
| New tests fail on base | NOT DONE | not run against base; the old held-write deny failed the new test's cases by construction (25 `TestPlanHeaderRules/*/deny` cases denied before the edit and passed after the production edit only once the hold was gone) |
| Tests of changed packages and their importers | NOT DONE | guard only. Importers not listed |
| lint, Windows, macOS builds | NOT DONE | not run |
| No new guard, gate, cap or hold | HOLDS | the diff only deletes rules and routes the four guards through the existing `configuredLevel`; the one added helper is a wrapper |
| Small commits, no attribution lines, no personal paths | HOLDS | `git log origin/main..` |

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

## Deleted tests

Each assertion was that the removed hold or fixed level exists. The behaviour was
removed by the owner's direction of 9 Nov (no Grid-made guards, approvals from the
workflow with a default), as the brief D-plan-hold.md and S6c-map.md (S6c5 row,
"behaviour removed by the owner's direction") write it. Nothing else covers them.

- internal/guard: write_hold_test.go, plan_hold_named_test.go, plan_stage_steps_test.go, deliver_hold_test.go, approval_test.go (whole files).
- internal/guard/plan_edges_test.go: `TestPlanStages`, `TestEdits`, `TestExplicitFilesystemBase` (`TestRecallBash`, `TestRecallLineForms` kept as they were).
- internal/guard/guard_test.go `TestPlanHeaderRules`: cases "held task keeps writes in projects", "acceptance is the history event", "inactive task releases hold", "missing task remains held", "cannot forge acceptance", "cannot move step past gate", "cannot remove approval workflow", "cannot close task", "cannot corrupt readable task", "unreadable task can stay unreadable", "unknown edit is refused", "nontext writes are refused", "notebooks cannot edit task controls", "top names printable ASCII", "nested names no alternate streams", "parts no trailing dot", "parts no trailing space", "switch releases plan guard", "main agent remembered task", "child inherits main hold", "child owns hold without siblings", "legacy session list fallback", "symlink write follows target", "dangling symlink follows target". The planner, curator, brain, tasks-folder, preview, plan-ready, gate-name, reviewer and checker cases stay and pass.
- internal/guard/settings_test.go: `TestAlwaysBlockGuardsIgnoreHookLevels` replaced by `TestFourGuardsFollowTheirLevel`.

## Open items (NOT DONE, to finish next)

These fail now because they assert the fixed levels:

- internal/settings/store_test.go: lines 86-88 (`plan-guard warn` listed as an invalid change), :140 and :189 (`AlwaysBlock` no longer exists, compile error), :154-193 `TestAlwaysBlockHooksRefuseOffAndWarn` (uses `neverOff`).
- internal/cli/settings_test.go:98-123 `TestHookLevelRefusesToLowerAlwaysBlockGuards`.
- internal/hook/settings_test.go:17-40 `TestHookKeepsAlwaysBlockGuardsWhateverTheSettingsSay`.
- internal/cli/settings_choke_test.go:101 lists `AlwaysBlock` among the catalog.go functions; check it.

Each is to be replaced by a test that the level is accepted and followed (the model is `TestFourGuardsFollowTheirLevel`). Then run `go test` for guard, hook, settings, cli, task and every importer (`go list -f '{{.ImportPath}} {{.Imports}}' ./...`), `make lint`, `GOOS=windows go build ./...`, `GOOS=darwin go build ./...`.

Not touched, to settle: `GRID_PLAN_GUARD=off` still suppresses plan checks (docs/settings.md:148); internal/task/hold.go keeps `PlanHeldWith` for the two readers above.

## NOT VERIFIED on a live screen

Nothing here is a UI change. A hook run in a live Claude Code session was not tried: the guard decisions were checked through `EvaluatePlan` and the adapters only.
