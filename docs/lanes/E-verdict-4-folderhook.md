# E-verdict-4-folderhook: the folder hook warns and does not block

Owner verdict verdicts-11nov v-4: only-writer warns by default and does not block.

## Proof table

| Row | Result |
| --- | --- |
| 1. only-writer defaults to warn; catalog and golden files updated | HOLDS. `internal/settings/catalog.go:76` no longer lists only-writer. `internal/cli/testdata/settings/hooks.json` reads `"only-writer":"warn"` (`TestSettingsJSONGolden/hooks` passes). `internal/settings/testdata/claude/writer.go.golden` refreshed from writer.go (comment only; no predicate changed). `docs/settings.md` and the usage line at `internal/cli/settings.go:26` say warn. |
| 2. Default Write into a task folder: no deny, warning reaches the session | HOLDS. `TestOnlyWriterDefaultWarnsAndDoesNotDeny` (`internal/guard/settings_test.go`): Behavior is empty, one warning `only-writer: Task folders and the brain are written only by grid...`, output is a `systemMessage` with no `permissionDecision`. `TestGuardDispatchOnlyWriterDefaultWarns` (`internal/hook/tooluse_test.go`) shows the same through the dispatcher for PreToolUse and PermissionRequest. Both fail on base (pasted below). |
| 3. only-writer set to block still denies | HOLDS. `TestFourGuardsFollowTheirLevel/only-writer/block` and `/malformed` (unchanged, deny); `TestGuardDispatch` (every deny row, now with block set explicitly); `TestOnlyWriterBlocksTaskInputsAndAttachments` (block set explicitly); `TestClaudeDenyRulesEqualWriterGuard` (block set explicitly). |
| 4. go test of guard, settings, cli, and make lint | HOLDS. `go test ./cmd/grid ./internal/cli ./internal/doctor ./internal/guard ./internal/hook ./internal/packaging/... ./internal/settings`: exit 0, all ok. `make lint`: exit 0. |
| New tests fail on base, pass on lane | HOLDS. With catalog.go reverted: `--- FAIL: TestOnlyWriterDefaultWarnsAndDoesNotDeny ... default only-writer refused the write: {... Behavior:deny ...}`; `--- FAIL: TestGuardDispatchOnlyWriterDefaultWarns/pretooluse ... "permissionDecision":"deny"`; `/permissionrequest ... "behavior":"deny"`. |
| Packages importing the changed ones (go list) | cmd/grid, internal/cli, internal/doctor, internal/guard, internal/hook, internal/packaging, internal/packaging/cmd, internal/settings. All tested above (two have no test files). |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard, gate, cap, hold or hook added | HOLDS. The change lowers a level; nothing new blocks. |
| Existing tests not edited to pass; deletions | HOLDS. No test deleted. Existing tests that asserted the old default were changed only to state the new default (`internal/settings/store_test.go`, `internal/cli/settings_test.go`, `internal/guard/settings_test.go` for only-writer/none) or to set only-writer to block first so they keep covering the deny (`internal/guard/case_test.go`, `internal/hook/tooluse_test.go`, `internal/cli/setup_claude_test.go`). The brief names "tests that expect the deny". |
| Public repo: no personal paths, hosts or emails | HOLDS. |

## What changed

- `internal/settings/catalog.go`: only-writer leaves the block-default list.
- `internal/guard/writer.go`: comment only. The `testdata/claude/writer.go.golden` snapshot is refreshed with it.
- `internal/cli/settings.go`: usage text names the new default.
- `internal/cli/testdata/settings/hooks.json`, `docs/settings.md`: new default.
- Tests as listed above, plus two new ones.

## Open items for the orchestrator

- `grid setup` still writes Claude `permissions.deny` rules (`Edit(...)` on task folders, the brain and project.md). Those are a static refusal inside Claude and do not read the hook level, so on a machine that ran setup, a default Write into a task folder may still be refused by Claude itself. `TestClaudeDenyRulesEqualWriterGuard` ties those rules to the guard at block. Removing or conditioning them is in `internal/settings` Claude block code, outside this brief's Touches list; it needs a decision.
- With warn, the decision is empty, so the later home-allow guard grants the write inside the Grid home ("Write stays inside the Grid home") and the warning goes out as `systemMessage` (shown by Claude Code to the person; not verified that it also lands in the agent's context).

## NOT VERIFIED on a live screen

- That Claude Code shows the systemMessage warning on a task-folder write and that the write then succeeds on a machine without the setup deny rules.
