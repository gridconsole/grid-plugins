# Lane C-kg-writer-drop: the only-writer hook guard goes; the deny rules `grid setup` writes carry its rules

Base: plugin main 35af7d3c. Head: see the report line of the hand-off (commits on lane-C-kg-writer-drop).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. no `EvaluateOnlyWriter` / `evaluateOnlyWriter`, no writer.go | HOLDS | `git grep -n 'EvaluateOnlyWriter\|evaluateOnlyWriter' -- '*.go'` prints nothing; `ls internal/guard/writer.go` says no such file. Base check first: the grep on base listed writer.go, hook.go:334 and the tests, so the guard was still there. |
| 2. `TestKgWriterDenyRulesWritten` | HOLDS | internal/cli/kg_writer_deny_test.go:46; `go test -count=1 -v -run TestKgWriter ./internal/cli/` printed `--- PASS: TestKgWriterDenyRulesWritten` with `default` and `outside_home`. It checks the five exact rule strings of claude.go:50-56 in permissions.deny. It also passes on base (the rules were already written there), as the brief says. |
| 3. `TestKgWriterHookNoLongerRefuses` | HOLDS | internal/cli/kg_writer_deny_test.go:81; `--- PASS: TestKgWriterHookNoLongerRefuses`. On base (writer.go and hook.go restored from origin/main, then put back; `git status --short` clean): `kg_writer_deny_test.go:112: the hook still refuses a task-folder write: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Task folders and the brain are written only by grid. Use the corresponding grid command to make this change."}}` then `--- FAIL: TestKgWriterHookNoLongerRefuses`, exit 1. The test also writes to `<grid>/.host/x` at host-guard block and wants a deny, so it shows the hook ran. |
| 4. tests of the five packages | HOLDS | `go test -count=1 ./internal/guard/ ./internal/hook/ ./internal/cli/ ./internal/settings/ ./internal/golden/ ./internal/account/ ./internal/doctor/ ./internal/packaging/ ./internal/updates/` : every package `ok` (updates has no test files), exit 0 (no FAIL line). The list is the packages `go list` shows importing guard, hook or settings (account, cli, doctor, guard, hook, packaging, updates) plus settings and golden. The two new tests were run with `-v` and print `--- PASS` (rows 2 and 3). |
| 5. `make lint` and `make cross` | HOLDS | `make lint && make cross` exit 0: go vet ./..., then darwin/arm64 and windows/amd64 vet and test compile, all `ok`. |
| Walk tests, plugin | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' && go test -count=1 ./internal/job/lint/` : `ok .../internal/cli`, `ok .../internal/job/lint`, exit 0. The app walk tests (make webui-test and the rest) are not for this repo. |
| New tests in the one new file | HOLDS | internal/cli/kg_writer_deny_test.go only; helpers `kgWriter*`, tests `TestKgWriter*`. |
| No guard, gate, cap or hold added | HOLDS | The lane only removes the guard call. The only-writer name stays in internal/settings/catalog.go:30. stop.go, events.go and internal/guard/helpers.go are not edited. |
| No attribution lines, no personal paths | HOLDS | One commit, one sentence, no trailer. |
| Tests named RealPlugin | n/a | None in the brief. |
| UI rows | n/a | No UI. |

## What changed

- internal/guard/writer.go deleted; the `guard.EvaluateOnlyWriter` line removed from `evaluateGuards` (internal/hook/hook.go:334); the ClaudeBlock comment (internal/settings/claude.go:30-32) no longer names writer.go. The rule list of ClaudeBlock is unchanged.
- internal/settings/testdata/claude/writer.go.golden deleted with its test.
- Tests: see the next section.
- golden/known-differences.txt: four lines removed (host-guard/ordinary-write, plan-guard/brain-curator, plan-guard/recall-present, plan-guard/task-write). With no only-writer deny, those cases match the recording, and `TestGoldenCorpus` fails on a listed case that matches ("remove the line"). This file is outside the Touches list; the test named in row 4 demands it.

## Tests deleted or changed, and what still covers them

- `TestClaudeWriterReservedKindsStayInSync` (settings/claude_test.go) and writer.go.golden, deleted: it pinned writer.go's source, which is gone. The reserved kinds are covered by `TestClaudeDenyRulesEqualWriterGuard` (fixed table) and `TestKgWriterDenyRulesWritten`.
- `TestClaudeDenyRulesEqualWriterGuard` (cli/setup_claude_test.go): the guard call is replaced by a fixed expected-deny table over the same `paths` list, both locations; the rule matching and the "every rule has a hit" check stay.
- `TestSetupWritesDenyRulesForTheHostGuardFiles` (cli/setup_claude_host_test.go): the writer half of `refused` is dropped; the task-folder rows are checked against the written deny rules by `claudeRuleMatches` only. Only host-guard is pinned to block.
- `TestGuardDispatchOnlyWriterDefaultWarns` (hook/tooluse_test.go), deleted: the behaviour (a warning for a task write) was removed with the guard by the owner's direction in the brief. `TestKgWriterHookNoLongerRefuses` covers the write now going through without a deny or the writer text.
- `TestHookFollowsTheOnlyWriterLevel` (hook/settings_test.go), deleted: the only-writer level no longer reaches the dispatcher. Levels through the dispatcher stay covered by `TestHookHomeAllowLevelFollowsTheProject`; `TestKgWriterHookNoLongerRefuses` covers only-writer at block.
- `TestOnlyWriterDefaultWarnsAndDoesNotDeny` (guard/settings_test.go), deleted: same reason; `TestKgWriterHookNoLongerRefuses` covers it. The `only-writer` row of `TestGuardsFollowTheirLevel` is removed; the host-guard and plan-guard rows stay, and `TestConfiguredLevelPreservesDenialOnSettingsError` covers the level mechanism.
- `TestOnlyWriterBlocksTaskInputsAndAttachments` (guard/case_test.go), deleted: `TestClaudeDenyRulesEqualWriterGuard` covers `tasks/T-1/sub/deep/file`, which the `**` rule matches for inputs/ and attachments/.
- Six `f.onlyWriter` rows of `TestCaseAliasesAreJudgedLikeTheirTargets` and the `onlyWriter` helper (guard/case_test.go), removed: the upper-case spellings are covered by `TestClaudeDenyRulesEqualWriterGuard` (`TASKS/t-22-work`, `KNOWLEDGE`, `PROJECT.MD`) through claudeFold. The plan-guard and host-guard rows stay.
- `TestGuardDispatch` (hook/tooluse_test.go): three existing rows changed from `deny` to `allow` (own task, brain, project record), because the dispatcher no longer refuses them; the feed row still wants `deny`, now from host-guard pinned to block. This edits rows of an existing test; the reviewer should look at it.
- `TestEventAtTheLimitIsStillGuarded`: the target moved to `<grid>/.host/x` with host-guard at block; it wants `"deny"`.
- `TestCaseAliasesOfTheGridHomeAreDenied`: only-writer no longer pinned; plan-guard (builder agent) still gives the deny, and the test passes.

## Open items

- golden/known-differences.txt still has other lines whose reason says "the only-writer rule denies ..." (grid-home-allow/permission, plan-guard/brain-builder, forged-acceptance, forged-preview, gate-filename, missing-recall, unmatched-edit). They still differ from the recording, so the test passes, but I did not check whether the stated reason is still true. Left for the cleanup after the four guards.
- internal/guard/settings_test.go has one comment ("All three guards warn by default") that now counts one guard too many. Comment only.
- The whole-module test and the cross-repo set are the orchestrator's at landing.

## NOT VERIFIED on a live screen

Nothing here needs a screen. Claude Code itself enforcing the written deny rules is not run in the sandbox (no claude run, by the brief): the rules are proved by their strings in settings.json and by the pattern matcher of the test helpers.
