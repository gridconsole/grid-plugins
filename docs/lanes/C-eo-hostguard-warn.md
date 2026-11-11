# C-eo-hostguard-warn: the host guard warns by default instead of blocking, and its reasons stop saying "gate"

Reason: verdict v-enforce (simplify.md:40, needed.md:47: fixed gates, text guard and folder checks: remove). host-guard was the last file-tool guard that blocked by default; only-writer and plan-guard already warn (E-verdict-4-folderhook, C-need-planguard-warn).

Base: bb3c134081ecb6ec65f02a5ad567337a8e86ec74 (lane-Pint76 merge of main d528178a). Base check: I read host.go:1-152 and catalog.go:75-85 on it. `defaultLevel` listed "host-guard" in the block case (catalog.go:80), so the lane was not already done. The code commit is 89cc9369; this report is committed on top of it.

## Proof table

| Row | Result |
| --- | --- |
| 1. file:line on base: EvaluateHost, the default, every place that states it, every test changed | HOLDS. `EvaluateHost` internal/guard/host.go:94-96 (reads level "host-guard" through `configuredLevel`, settings.go:16-51). Default: internal/settings/catalog.go:78-84, the "host-guard" name in the block case at :80, now removed (job-lint and stop-check stay block). States the default and now say warn: internal/cli/settings.go:26 (usage text), docs/settings.md:113 (table) and :127-129 (paragraph), internal/cli/testdata/settings/hooks.json (`"host-guard":"warn"`), the comment at host.go:89-93. Texts changed: host.go:14 `gateReason`, :16 `switchReason`, :104-106 `deny` (now `hostReason`, new `feedReason` for the Feed branch), comment :53-55. Tests: see "Tests" below. |
| 2. New test, default settings, Write to `<projects>/p/tasks/T-01/task.md`: Behavior not deny, one warning naming task.md; fails on base | HOLDS. `TestHostGuardWarnsByDefault` (internal/guard/dy_hostguard_warn_test.go). Base failure line: `dy_hostguard_warn_test.go:59: default host-guard refused the write: {... Behavior:deny Reason:Gates are released by the person or the app, never by an agent: questions/ and its answers, task.md, ...}`. It also checks the other kinds (questions, approval.json, plan-revisions, project workflows.md, settings, Feed, .host): one warning `host-guard: <the new sentence>`, and a `systemMessage` output with no `permissionDecision`. Passes on the lane. |
| 3. Same input at host-guard block: deny; also `<home>/settings.json` and a file under feed/ | HOLDS. `TestHostGuardDeniesAtBlock` writes host-guard block into the temp home's settings.json (`hostBlock`), then for task record, questions, approval, plan-revisions, project workflows.md, settings.json, feed/ and .host, for Write, Edit, MultiEdit and NotebookEdit: Behavior deny, Reason equal to the new sentence, no warning. (On base the deny already held at default; the new sentences failed, base run: `want the deny "<path> is a task record that grid writes; ..."` for every kind.) |
| 4. `git grep -n -i -E "gate\|held plan\|turn a guard off" internal/guard/host.go`: every hit an identifier, none in a string or comment | HOLDS. Output, 6 lines: `host.go:13:var gateFile = ...` / `host.go:17:const gateReason = "%s is a task record ..."` (the hit is the identifier before the `=`; the string has no such word) / `host.go:64:func gateAt(root, target string) bool {` / `host.go:73: ... gateFile.MatchString(first))` / `host.go:118: person := ... fmt.Sprintf(gateReason, given)` / `host.go:141: if gateAt(root, target) {`. The doc comment of `gateAt` no longer starts with its name so the grep does not hit it. |
| 5. `grid hook levels --json` shows "host-guard":"warn"; docs/settings.md:113 says warn | HOLDS. `TestSettingsJSONGolden` (internal/cli) compares against internal/cli/testdata/settings/hooks.json, now `"host-guard":"warn"`; `TestHookLevelAcceptsEveryLevelForTheFourGuards` checks the `hook levels --json` default is warn for host-guard. docs/settings.md:113 is `| host-guard | warn | Host and Feed guard; warns by default, \`block\` refuses the write |`. `go test ./internal/cli/...` ok. |
| 6. diff stat, tests, lint, Windows | PARTLY OUTSIDE TOUCHES (below), otherwise HOLDS. `go test -count=1 ./internal/cli/... ./internal/guard/... ./internal/settings/... ./internal/hook/... ./internal/golden/...`: all `ok`, 0 FAIL, exit 0. `make lint` (`go vet ./...`): exit 0. `GOOS=windows go vet ./...`: exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...`: exit 0. |
| Packages importing the changed ones | `go list` of direct importers of internal/guard and internal/settings: internal/cli, internal/doctor, internal/guard, internal/hook. Tested: cli, guard, settings, hook, golden, doctor, packaging, packaging/cmd: all `ok`. The whole-module run is the orchestrator's. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. One default lowered from block to warn; no new check; the folders, paths and checks `evaluateHost` makes are unchanged. |
| Existing tests not edited to pass; no deny test deleted | HOLDS. No test deleted. Every changed deny test sets host-guard to block first (`hostSetup`/`hostBlock`), so it still covers the deny; tests that state the default state warn. The old-text checks in host_gates_test.go moved to the new text (below). |
| Public repo: no personal paths, hosts, emails | HOLDS. Only repo-relative paths in the files changed. |
| New tests fail on base, pass on lane | HOLDS. Failing lines are in rows 2 and 3. |

`git diff main --stat`: Touches files plus the new test file, plus these not named in Touches, each needed to keep a test that expects the deny at the default level: `internal/cli/cli_test.go` (TestHookPreToolUse), `internal/hook/events_test.go` (TestInternalErrorsAllowAndGuardStillDenies), `golden/host-guard/host-write/setup.json` and `golden/known-differences.txt` (corpus row). Also `internal/guard/{host_approval,host_preview,host_workflow,host_project_workflow,task_gates}_test.go`, which Touches names only by the list "tests that name host-guard or EvaluateHost".

## Tests

Set host-guard to block first (`hostSetup(t)` = `setup(t)` plus `hostBlock`, in the new test file), so the deny stays covered:
- `TestHostRules`, `TestHostFileToolsGatePaths`, `TestHostFileToolsSettingsPaths`, `TestAgentCannotWriteGuardOffIntoSettings`, `TestHostNeverRefusesBash` (host_test.go, host_gates_test.go); `TestHostApprovalReceipts`, `TestHostProtectsTaskWorkflow` (host_approval_test.go); `TestHostPreviewApprovalReceipt`; `TestHostAgentWorkflowFileTools`; `TestHostAgentProjectWorkflows` (also compares `fmt.Sprintf(gateReason, path)` now that `gateReason` is a format); `TestTaskFileGateProtection`.
- `TestHookPreToolUse` (cli_test.go) and `TestInternalErrorsAllowAndGuardStillDenies` (hook/events_test.go): write `{"hooks":{"host-guard":"block"}}` into the Grid home's settings.json first.
- golden `host-guard/host-write`: `setup.json` gets that settings.json. Its recorded legacy stdout has the old text, so it is a known difference (golden/known-differences.txt; the rule wants an L<n> citation, so it cites L4.md:47, the lane that wrote the host deny text).

Changed because they state the default or the old text:
- `TestGuardsFollowTheirLevel` (guard/settings_test.go): host-guard at "none" expects no deny, like the other two guards; comment says warn.
- `TestDefaultLevelPlanGuardWarns` (settings/catalog_planguard_test.go): host-guard warn. `TestGuardLevelsAreSettableAndResolved` (settings/store_test.go) and `TestHookLevelAcceptsEveryLevelForTheFourGuards` (cli/settings_test.go): default warn for all three guards.
- host_gates_test.go: the two text checks (`"Reading is allowed"`, `"cannot turn a guard off"`) now check the new task-record and settings sentences.
- Not changed, still pass: setup_claude_host_test.go, setup_claude_test.go, case_test.go, legacy_test.go, precompact_records_test.go, corpus_test.go.

## What changed

- `internal/settings/catalog.go`: host-guard leaves the block case.
- `internal/guard/host.go`: four texts, one per kind, each with the given path and an agent's own verb: task record (`gateReason`), settings (`switchReason`), Feed (`feedReason`, new), host folder (`hostReason`, new). The Feed branch used to say `gateReason`; it now has its own sentence. Comments at :53-55 and :89-93 reworded. Identifiers, folders, paths and checks unchanged. The `named` field of the `folders` struct is no longer read (the host sentence names the given path, as the brief wrote it); left in place so the folder list stays as it was.
- `internal/cli/settings.go:26`, `docs/settings.md:113` and :127-129, `internal/cli/testdata/settings/hooks.json`.
- New: `internal/guard/dy_hostguard_warn_test.go`.

## Open items

- `docs/settings.md:28` still says "the four guards default to `block`" (stale already for only-writer and plan-guard). It is outside the lines Touches names and the two sister lanes edit that file, so left. `internal/guard/settings.go:14-16` says "the default of each is block"; outside Touches, left.
- With warn, a file-tool write to a task record, a settings file or the Feed goes through with a warning unless another guard refuses it (only-writer also warns by default; home-allow grants writes inside the Grid home). That is the point of the verdict; the owner should know the settings.json write by an agent is now allowed at default.
- internal/hook/hook.go (input-size deny) untouched, per the brief.
- I made the mechanical test edits with a throwaway Go helper in the clone (removed with `git clean` before the commit, not in it), because a write outside the clone was refused and the budget did not allow one Read and Edit per file. The diff is what counts. The lane used about 60 tool calls.

## NOT VERIFIED on a live screen

- That Claude Code shows the `systemMessage` warning for a host-guard hit and that the write then goes through.
