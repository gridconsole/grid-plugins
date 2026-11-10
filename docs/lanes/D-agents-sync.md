# D-agents-sync: agent templates and `grid agent sync`

Repo: plugin. Branch lane-D-agents-sync, base f9602370 (plugin main). The decision is `docs/lanes/D-agents-decide.md` in the app repo, section 4 "D-agents-sync", with section 1 for the rules.

## What changed

**Step 1: `agent-templates/`.** Twelve files: `grid-builder`, `grid-checker`, `grid-curator`, `grid-designer`, `grid-operator`, `grid-planner`, `grid-researcher`, `grid-reviewer`, `grid-runner`, `grid-scout` (ported from the old plugin's `agents/grid-*.md`, read from the old-plugin reference copy) and the two new ones, `grid-library` (brain Q&A, read-only) and `Explore` (read-only search, Haiku). Each has plain front matter (name, description, model, effort, maxTurns, tools, skills) and a body in plain words. No `permissionMode`, `hooks`, `mcpServers` or `metadata` key. The old node scripts, `grid-brain` calls, the feed MCP tool and the old app's Rust, Prototype and premerge rules are not carried. They name `grid` verbs instead. The templates are not in `agents/`, so Claude does not load them.

**Step 2: sync.**
- `internal/settings/agentsync.go`: `SyncAgents` renders every template into `~/.claude/agents/` (or beside `CLAUDE_CONFIG_DIR`'s settings.json) with `model` and `effort` from `Store.Effective()` and a marker block, `metadata.grid.owner: grid` and `metadata.grid.template: <sha256 of the template>`.
  - A file is written only when its rendered bytes differ, so a second run writes nothing.
  - A file that exists without the marker, or is not a regular file, is the person's: never touched, reported `yours`.
  - A broken overlay stops the sync with an error and writes nothing, so a defaults-only render never undoes the person's values.
  - `Explore` is not in the catalog, so it keeps the template's Haiku/low unless an overlay entry names it.
- `internal/cli/agent_sync.go` and one dispatch line in `internal/cli/agent.go`: `grid agent sync [--json] [--templates DIR] [--dir DIR]`. It prints `<agent> written|unchanged|yours <path>`. `pluginRoot` was lifted out of `doctor.go` so both use it (same behaviour).
- `internal/hook/hook.go`: SessionStart calls the sync (only when `CLAUDE_PLUGIN_ROOT` is set and the `session-start` hook level is not `off`; a failure is logged and never reaches the session). The session context now names the bare `grid-scout` ... `grid-library`.
- `internal/packaging/dist.go`: `agent-templates` is copied into dist with `skills` and `agents`, so it is covered by `SHA256SUMS`.

## New tests, each fails on base

Base is f9602370. I copied only the two new test files into a detached worktree of it and ran them there. (That run was on the test files as first committed; the two `Sync...` constants were made unexported afterwards, so `undefined: SyncYours` reads `syncYours` in the current file.)

```
$ cd basecheck && go test -count=1 ./internal/hook/ ./internal/settings/ -run 'TestSessionStartSyncIdempotent|TestSessionStartNamesBareAgents|TestAgentSyncLeavesPersonsOwnFile|TestAgentSyncNewTemplateKeepsOverlay'
# github.com/gridconsole/grid-plugins/internal/settings [github.com/gridconsole/grid-plugins/internal/settings.test]
internal/settings/agentsync_test.go:23:32: undefined: SyncOptions
internal/settings/agentsync_test.go:53:18: undefined: SyncAgents
internal/settings/agentsync_test.go:64:49: undefined: SyncYours
internal/settings/agentsync_test.go:67:44: undefined: SyncWritten
--- FAIL: TestSessionStartSyncIdempotent (0.00s)
    agentsync_test.go:48: the first SessionStart wrote no agent file: open /tmp/TestSessionStartSyncIdempotent2894273612/001/.claude/agents/grid-builder.md: no such file or directory
--- FAIL: TestSessionStartNamesBareAgents (0.00s)
    agentsync_test.go:75: context names plugin-qualified agents, which sync does not write: Grid tracks tasks, ... grid:grid-scout maps code; grid:grid-researcher researches facts; ...
FAIL	github.com/gridconsole/grid-plugins/internal/hook	0.008s
FAIL	github.com/gridconsole/grid-plugins/internal/settings [build failed]
```

The two hook tests fail on an assertion. The two settings tests fail to build on base, because `SyncAgents` does not exist there. Their exact names, in `internal/settings/agentsync_test.go`: `TestAgentSyncLeavesPersonsOwnFile` (a `grid-builder.md` without the marker is byte-identical and reported `yours`) and `TestAgentSyncNewTemplateKeepsOverlay` (templates replaced with a new body and sha, sync: the file has the new body and the overlay's `opus` / `high`, and the sha changed). In `internal/hook/agentsync_test.go`: `TestSessionStartSyncIdempotent` (second SessionStart leaves the mtime of the files, set to 48 hours ago between runs, unchanged) and `TestSessionStartNamesBareAgents`.

One more test, `TestShippedTemplatesRender`, renders the real `agent-templates/` and checks the twelve names, the marker and that no forbidden key or old-script text is in any file. It is a check on the templates, not one of the four.

All four pass on the lane head (`go test -count=1 ./internal/settings/ ./internal/hook/ -run 'AgentSync|SessionStart' -v`, exit 0).

## Existing tests and fixtures I changed (owner's direction: decision section 1, "`hook.go:158` changes ... to the bare names")

The session context string is the thing the decision changes, so the lines that record it changed with it. No assertion was weakened.
- `internal/hook/hook_test.go:76`: the wanted substring `grid:grid-builder` became `grid-builder changes files`. `TestSessionStartNamesBareAgents` now also asserts that no `grid:grid-` is left.
- `testdata/golden/hook-sessionstart-git.golden`, `-personal.golden`, `-empty-input-uses-cwd.golden`: the one agent sentence uses the bare names. Nothing else in them changed.
- `internal/cli/settings_choke_test.go` (the settings-package choke point): the reviewed-export list gained `AgentTemplatesPath`, `ClaudeAgentsDir` and `SyncAgents`, and the one place that may build a read-only `Store{...}.Effective()` was widened from `ReadHookLevels` to `ReadHookLevels` and `SyncAgents`. The owner should confirm that review; `SyncAgents` only reads the overlay and writes only the agents folder. `TestSettingsPackageChokeMutations` still passes, so the bypass probes are still caught.

## Checks

| Check | Exit |
|---|---|
| `go test -count=1 ./internal/settings/... ./internal/hook/... ./internal/cli/... ./internal/packaging/...` | 0 |
| Importers (`go list` of packages importing settings, hook, packaging or cli): `cmd/grid`, `internal/cli`, `internal/doctor`, `internal/guard`, `internal/hook`, `internal/packaging`, `internal/packaging/cmd`. `go test -count=1 ./internal/doctor/... ./internal/guard/... ./internal/golden/... ./cmd/...` | 0 (`cmd/grid` and `packaging/cmd` have no test files) |
| `make lint` (`go vet ./...`, `gofmt -l .`) | 0 |
| `GOOS=windows CGO_ENABLED=0 go build ./...` | 0 |
| `GOOS=darwin CGO_ENABLED=0 go build ./...` | 0 |

Before the fixes above, the first run of the cli package failed (`TestHookSessionStartGolden`, `TestSettingsStoreChokePoint`); both pass after them. The whole-module test, `make dist` and the cross-repo set are for the orchestrator at landing.

## Overlap with D-agents-server

I touched `internal/cli/agent.go` for one dispatch line (`sync`) and the usage line, and `internal/cli/doctor.go` for the `pluginRoot` lift. I did not edit `store.go`, `claude.go`, `budget.go` or `topics.go`. A merge of the two lanes may need a hand merge in `agent.go`'s `runAgentCommand`.

## NOT VERIFIED

- The twelve bodies were written by a forked sub-agent from the old bodies. No test proves a prompt good: the owner or a reviewer reads them before landing. The `grid` verbs they name were checked against `grid <verb> --help` by the writer, not by a test.
- The writer added `Bash` to `grid-curator`, `grid-designer` and `grid-researcher` (the old files had none) so they can run `grid brain` and `grid job result`. The limit to `grid` verbs is wording only. It is a decision for the owner.
- `grid-checker` ships as sonnet/low (the catalog value), where the old file said haiku.
- A real Claude Code session loading `~/.claude/agents/grid-*.md`, and a user agent beating a plugin agent of the same name, are from the docs (decision open item 4), not seen. All runs used temp folders; nothing was written to a real `~/.claude`.
- `grid agent sync` has no test of its own. It is a thin wrapper over `SyncAgents`, which the four tests and `TestShippedTemplatesRender` cover; the verb builds on all three platforms but was not run by a test.
- Windows: the agent files use `atomicfile` and no lock, so they are not blocked by the missing Windows lock. CRLF templates are normalised before the sha is taken, but that path has no test and nothing ran on Windows.
- The Codex host is not synced: the hook syncs only when `CLAUDE_PLUGIN_ROOT` is set.
- No UI involved.

## Open items

1. Agents can run `grid agent sync` themselves. The host guard is unchanged (decision open item 1); adding the verb to it is a guard change, left to the owner.
2. `grid agent set Explore` is not possible (not in the catalog), so Explore's model and effort come from its template.
3. If `session-start` is set to `off`, the sync does not run at SessionStart; `grid agent sync` still does.

## Budget

About 75 tool calls against the 60 given (the brief said about 90). The step-35 work-in-progress bundle was written at about call 50, late. Step 1 was done by a forked sub-agent in parallel, which is why the count stays near that.
