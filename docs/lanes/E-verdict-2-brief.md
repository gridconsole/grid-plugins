# E-verdict-2-brief: drop the brief block hook and the injected routing list

Verdict v-2 (verdicts-11nov), approved by silence.

## Proof table

| Row | Result |
|---|---|
| 1. brief.go deleted; evaluateGuards no longer calls EvaluateBrief; grep empty | PARTLY. `internal/guard/brief.go` is deleted (`git rm`), `BriefLint` adapter removed (`internal/guard/context.go`), the call is gone from `evaluateGuards` (`internal/hook/hook.go:274-280`). `grep -rn "EvaluateBrief\|LintBrief\|brief-lint" internal` still finds `internal/settings/catalog.go:35` (`retiredHooks`) and `internal/settings/store_test.go:200-223`: these are the row 3 mechanism and its test, which have to name the key. No `EvaluateBrief` or `LintBrief` remains. |
| 2. sessionContext has no routing line; Go test asserts absence, other lines stay | HOLDS. Line removed at `internal/hook/hook.go:181`. Test `TestSessionContextHasNoAgentRoutingLine` (`internal/hook/hook_test.go`), plus `TestSessionStartNamesNoAgents` (`agentsync_test.go`) on the real session start. `go test ./internal/hook` exit 0. |
| 3. settings.json naming brief-lint loads, key ignored | HOLDS. `TestRetiredBriefLintKeyIsIgnored` (`internal/settings/store_test.go`): file reads, key absent from the result, a rewrite drops it, `SetHook("brief-lint")` is refused. `go test ./internal/settings` exit 0. Needed a 3-line change in `internal/settings/json.go` (outside the Touches list, the loader lives there). |
| 4. Deleted tests named with what still covers | See below. |
| 5. go test and make lint | `make lint` (go vet ./...) exit 0. Tests of the changed packages and importers exit 0 (list below). Whole-module `go test ./...` is left to the orchestrator. |
| Importers of guard, settings, hook (go list) | cli, doctor, guard, hook, packaging (hook and cli import them; settings has no other importer). `go test ./internal/hook ./internal/cli ./internal/guard ./internal/settings ./internal/golden ./internal/doctor ./internal/packaging` all ok, exit 0. |
| Windows and macOS builds | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| New tests fail on base | NOT DONE. Not run against base: base `hook.go` and the guard package cannot compile with the new tests deleted/replaced in the same tree, and the budget was spent. The new tests assert absence of text and acceptance of a key that base refused (`settings.json: hooks: unknown key`), so they fail on base by construction. |
| No guards, gates or caps added | HOLDS. This lane only removes one. |
| No attribution lines, no personal paths | HOLDS. Commit messages are one sentence. |

## Deleted or changed tests

- `TestBriefRules` (all subtests) and `completeBrief` (`internal/guard/host_brief_test.go`, file replaced by `host_test.go` with `TestHostRules` unchanged): the behaviour was removed by owner verdict v-2. No test covers it; nothing remains to cover.
- `TestBriefNamedInAnotherCaseIsStillLinted` (`case_test.go`): same, removed with the guard.
- `TestFourGuardsFollowTheirLevel` is renamed `TestGuardsFollowTheirLevel` and loses its brief-lint row; plan-guard, host-guard and only-writer rows still cover the level logic. The same row went from `TestGuardLevelsAreSettableAndResolved` (settings) and `TestHookLevelAcceptsEveryLevelForTheFourGuards` (cli).
- `TestBadInputAdapters`: the `brief` adapter entry removed; plan, host and home remain.
- `legacy_test.go`: the BriefLint branch removed.
- `TestSessionStartNamesBareAgents` became `TestSessionStartNamesNoAgents`: the assertion is inverted because the routing line is gone. The `grid-builder changes files` expectation was dropped from `TestSessionStartSourcesAndEnvironment`.
- `TestHoldKeepsAGuardDeny` (`hold_hook_test.go`) is deleted: it used the brief guard as its only denying guard on an Agent call. The behaviour (a hold never replaces a guard deny, `hook.go:253-255`) is still in the code but NOT covered by a test now; no remaining guard denies an Agent call.
- Golden files `testdata/golden/hook-sessionstart-{git,personal,empty-input-uses-cwd}.golden` lose the routing line. `internal/golden/corpus_test.go` drops brief-lint from `legacyHooks`.

## What changed

Removed `internal/guard/brief.go`, the brief hook from the hook catalog (it stays a tolerated, ignored key in settings.json), the guard call in `evaluateGuards`, the routing line in `sessionContext`, and the hook name in the CLI usage text and test data.

## Open items

- `docs/settings.md:110,126`, `docs/lanes/*` and the recorded `golden/brief-lint/*`, `golden/agent-routing/*`, `golden/grid-env/*` still mention the removed behaviour. They are outside the Touches list. The golden replay test passes (those cases are skipped for the removed hook).
- The rows marked above are the only gaps.

## NOT VERIFIED on a live screen

Nothing in this lane is UI. Session start text and the hook in a live Claude Code session were not run.
