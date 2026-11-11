# C-need-planguard-warn: the plan guard warns instead of blocking, and its two approval-line denies go

Reason: needed.md:47 (fixed gates, text guard, folder checks: remove), simplify.md:40 (history-line protections: remove; keep only that an approval names the plan revision seen) and simplify.md:55 row 4 (the write hook warns, it does not block; E-verdict-4-folderhook did it for only-writer).

Base: main 78b08bc9b65a008a993d4653ceeb9abcd2a22edb. Code commit d2f0eebd; this report is committed on top of it.

## Proof table

| Row | Result |
| --- | --- |
| 1. Every deny in `evaluatePlan` (line numbers on main 78b08bc9, `internal/guard/plan.go`) and what the lane does | **Removed:** :57-59 gate-word file name at the top of a task folder (`gateLike(top)`); :61-65 history.md edit that adds `preview approved`. **Warning by default, deny again at `plan-guard` block, code and text untouched:** :54-56 folder name; :73-75 reviewer/checker outside the projects root; :83-85 non-curator/planner writing the brain; :99-105 curator outside the brain and its task; :106-111 planner outside its folders. **Code untouched (E-verdict-6-recall owns it) but its level moved with the default:** :32 `recallGate` (Bash) and :66-70 plan-ready recall branch. `EvaluatePlan` wraps the whole decision in `configuredLevel`, so these two are warnings by default too and deny at block. This is a side effect the brief did not name; see Open items. |
| 2. `defaultLevel("plan-guard")` is "warn"; host-guard, job-lint, stop-check unchanged | HOLDS. `internal/settings/catalog.go:80` no longer lists plan-guard. `internal/settings/catalog_planguard_test.go` `TestDefaultLevelPlanGuardWarns` asserts plan-guard warn, host-guard/job-lint/stop-check block, only-writer warn. `go test ./internal/settings/...` ok. |
| 3. `internal/guard/planguard_warn_test.go` through `EvaluatePlan`, temp Grid home, no settings file | HOLDS. `TestPlanGuardRemovedApprovalLineDenies` is (a) and (b): no deny and no warning at default and at block. `TestPlanGuardFencesWarnByDefaultAndDenyAtBlock` is (c), (d), (e): for reviewer, folder name, builder-brain, curator, planner, default = `Behavior ""` with exactly one warning equal to `plan-guard: <main's deny text>` and a `systemMessage` output with no `permissionDecision`; block = deny with main's exact text and no warning. Run on main 78b08bc9 (test file copied in, no code change): see "Main run" below. Passes on the lane (`go test ./internal/guard/...` ok). **Brief mismatch on (a):** `<task>/plan accepted.md` does not fail on main, because `gateLike` matches the whole name (`plan accepted`) and never matched `.md`. The test lists `plan accepted`, `plan accepted.md`, `preview approved`, `ship approved`; the `.md` one passes on main and on the lane, the other three fail on main. |
| 4. `git grep -n "previewApproved\|gateLike(top)" -- internal/guard/plan.go` prints nothing; callerless helpers removed | HOLDS. The grep prints nothing (exit 1). `previewApproved` (context.go:144 on main) had plan.go:62 as its only caller: removed; `git grep -n previewApproved -- internal` prints nothing. The local `top` in `evaluatePlan` became unused and is removed. `gateLike` (context.go) stays: `GateLike` (helpers.go:8) calls it and `internal/hook/events.go:406` calls `GateLike`. `addsMark` stays (plan-ready branch :66-70). |
| 5. Every deleted or rewritten test, and what covers what remains | See "Tests" below. |
| 6. Tests, lint, Windows | HOLDS. `go test ./internal/guard/... ./internal/settings/... ./internal/hook/... ./internal/cli/... ./internal/golden/... ./internal/hookio/... ./internal/orchestrator/...`: every package `ok`, 0 FAIL. `make lint` (`go vet ./...`): exit 0. `GOOS=windows go vet ./internal/guard/...`: exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...`: exit 0. |
| Packages importing the changed ones | `go list` of direct importers of `internal/guard` and `internal/settings`: internal/cli, internal/doctor, internal/guard, internal/hook. Also tested: cmd/grid (no test files), internal/packaging, internal/packaging/cmd, internal/doctor: ok. The whole-module run is the orchestrator's. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. The change removes two denies and lowers one default; nothing new blocks. |
| Existing tests not edited to pass; deletions named | HOLDS. Only the two `TestPlanHeaderRules` cases that expected the removed denies are deleted (below); the rest set plan-guard to block first or state the new default. |
| Public repo: no personal paths, hosts or emails | HOLDS. Files changed hold only repo-relative paths. |
| New tests fail on base, pass on lane | HOLDS. Pasted below. |

## Tests

Deleted (row 5: the behaviour was removed by simplify.md:40, history-line protections):
- `TestPlanHeaderRules` case "preview always needs a person" (guard_test.go): the removed `preview approved` deny. What remains is covered by `TestPlanGuardRemovedApprovalLineDenies/history preview approved`, which asserts the edit is not refused at default or block. The history.md edit branch for `plan ready` is still covered by the case "plan ready needs recall".
- `TestPlanHeaderRules` case "gate names forbidden" (guard_test.go): the removed gate-word name deny; covered by `TestPlanGuardRemovedApprovalLineDenies/gate-word name ...`.
- No fence deny test is deleted.

Changed only to state the new default, or to set plan-guard to block first so the deny stays covered:
- `TestPlanHeaderRules` (guard_test.go): `planBlock(f)` after `setup`; every remaining deny row still runs at block.
- `TestBrainWritePluginAgents` (brain_roles_test.go): `planBlock(f)`.
- `TestRecallBash` (plan_edges_test.go): `planBlock(f)`; keeps the recall deny covered (see Open items).
- `TestGuardsFollowTheirLevel` (settings_test.go): plan-guard/none expects no deny, like only-writer.
- `TestGuardLevelsAreSettableAndResolved` (settings/store_test.go) and `TestHookLevelAcceptsEveryLevelForTheFourGuards` (cli/settings_test.go): plan-guard default is warn.
- `golden/plan-guard/bash-recall/setup.json`: adds a `settings.json` that sets plan-guard to block, so the replay still matches the recorded deny.
- Unchanged and passing: task_gates_test.go, legacy_test.go, case_test.go, plan_no_hold_test.go, hookio_test.go, plan_digest_test.go, corpus_test.go.

## Main run of the new tests (main 78b08bc9, code reverted, test file in place)

```
--- FAIL: TestPlanGuardRemovedApprovalLineDenies/gate-word_name_plan_accepted/default
--- PASS: TestPlanGuardRemovedApprovalLineDenies/gate-word_name_plan_accepted.md/default
--- FAIL: TestPlanGuardRemovedApprovalLineDenies/gate-word_name_preview_approved/default
--- FAIL: TestPlanGuardRemovedApprovalLineDenies/gate-word_name_ship_approved/default
--- FAIL: TestPlanGuardRemovedApprovalLineDenies/history_preview_approved/default
--- FAIL: TestPlanGuardRemovedApprovalLineDenies/gate-word_name_plan_accepted/block   (also fails on main: removed at every level)
--- FAIL: TestPlanGuardRemovedApprovalLineDenies/history_preview_approved/block
--- FAIL: TestPlanGuardFencesWarnByDefaultAndDenyAtBlock/reviewer_outside_the_projects_root/default
--- PASS: TestPlanGuardFencesWarnByDefaultAndDenyAtBlock/reviewer_outside_the_projects_root/block
--- FAIL: .../folder_name_beside_tasks/default      --- PASS: .../folder_name_beside_tasks/block
--- FAIL: .../builder_writing_the_brain/default     --- PASS: .../builder_writing_the_brain/block
--- FAIL: .../curator_outside_the_brain/default     --- PASS: .../curator_outside_the_brain/block
--- FAIL: .../planner_outside_its_folders/default   --- PASS: .../planner_outside_its_folders/block
    "default plan-guard refused the write: {... Behavior:deny Reason:A grid-reviewer writes only its Verdict inside a task folder; ... is outside it. ... Warnings:[] ...}"
```

## What changed

- `internal/guard/plan.go`: the two approval-line denies and the unused `top` removed; the default comment says warn.
- `internal/guard/context.go`: `previewApproved` removed.
- `internal/settings/catalog.go`: plan-guard leaves the block-default list.
- `internal/cli/settings.go` usage text, `docs/settings.md` (table row and the paragraph under it), `internal/cli/testdata/settings/hooks.json`: plan-guard is warn.
- New: `internal/guard/planguard_warn_test.go`, `internal/settings/catalog_planguard_test.go`. Tests changed as listed.

## Open items

- The recall denies (:32 `recallGate` and :66-70) share the plan-guard level, so a person who has not set a level no longer gets the plan-ready recall deny, only a warning. The brief leaves their code to E-verdict-6-recall; the orchestrator should confirm the warning is the wanted default for them, or hand E-verdict-6-recall a note that its tests now need plan-guard at block (as `TestRecallBash` and golden bash-recall have here).
- `recordings/cases.py` still builds `plan-guard/bash-recall` without the settings file; regenerating the golden row from it would drop the `settings.json` line in `setup.json`.
- `internal/guard/settings.go:14-16` says "the default of each is block"; already stale for only-writer, now for plan-guard. It is outside the Touches list, so left.
- A malformed settings file still keeps the deny (`configuredLevel` keeps a deny when settings cannot be read); `TestGuardsFollowTheirLevel/plan-guard/malformed` passes as before.

## NOT VERIFIED on a live screen

- That Claude Code shows the `systemMessage` warning from a plan-guard fence, and that the write then goes through (the empty decision lets the later home-allow guard grant writes inside the Grid home).
