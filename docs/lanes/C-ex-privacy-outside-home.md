# C-ex-privacy-outside-home

Base: `7d516e1c`. No production file changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 level on writes only agent files outside the Grid home | HOLDS | `go test -count=1 -run TestExHome -v ./internal/hook/`: exit 0, `TestExHomeLevelOnWritesOnlyAgents` PASS for HOME/.claude and CLAUDE_CONFIG_DIR; `internal/hook/ex_outside_home_test.go` (walks the rig root, Grid home left out, lists any other new or changed path) |
| 2 env file gets blank line, GRID_HOME, GRID_PROJECTS, then GRID_TASK or `unset GRID_TASK`; earlier bytes kept | HOLDS | `TestExHomeEnvFileGetsItsLinesAtEveryLevel`: 4 subtests (level on and off, task and no task) PASS; compares the file to earlier bytes plus the exact appended text |
| 3 level off: no agents folder, no context, env lines still written, nothing else new | HOLDS | same test, `level-off` subtests PASS (output empty, agents folder absent, only the env file changed) |
| 4 the person's `mine.md` and unmarked `grid-custom.md` unchanged | HOLDS | `TestExHomeKeepsThePersonsOwnAgents` PASS (also an unmarked `grid-scout.md` named like a template, and a plain file); marked `grid-builder.md` still written |
| 5 docs/privacy.md rows and cites | HOLDS | rows for `~/.claude/agents`, `$CLAUDE_ENV_FILE` (every level, off included), the Claude Code settings file, `~/.codex/config.toml`; every cite checked with `git grep -n` on base: hook.go:105, :125; settings_test.go:104; agent_sync.go:54; agentsync.go:47; claude.go:256, :239, :744, :797; settings.go:170; budget.go:611, :939; windows.go:58; account.go:318; claude_user.go:451; timer_systemd.go:21; timer_plist.go:27; timer_schtasks.go:36; land.go:238; worktree.go:118; dist.go:39; runner.go:99; claude.go:619; agentsync.go:60 all found |
| 6 diff stat, tests, lint, Windows vet | HOLDS | `git diff main --stat`: docs/privacy.md and the test file (this lane doc is the third file the runbook asks for); `go test -count=1 ./internal/hook/` ok; `make lint` exit 0 (go vet ./...); `GOOS=windows go vet ./internal/hook/` exit 0; `GOOS=darwin go vet ./internal/hook/` exit 0 |
| New tests fail on base | HOLDS by mutation | no production change is allowed, so the tests pass on base. With `if level != "off" { syncAgents(o) }` made unconditional the tests failed: `level off created [.../home/.claude .../home/.claude/agents .../Explore.md ...] outside the Grid home` and `level off wrote the agents folder`. Reverted with `git checkout` |
| Importers tested | HOLDS | `go list` importers of internal/hook: internal/cli, internal/packaging; `go test -count=1 ./internal/cli/ ./internal/packaging/` ok |
| `*RealPlugin*` tests | not needed | no UI row, no real plugin read |
| No guard, gate or cap added | HOLDS | tests and a doc only |
| No attribution lines, no personal paths | HOLDS | commit message is one sentence |

## Base check

Grep of `os.WriteFile|os.Create|os.OpenFile|os.MkdirAll|AppendFile` (and `atomicfile.Write`, `Rename`, `CreateTemp`) under internal/, not tests, classified: Grid home (feed, brain, session, task, project, signal, question, plan, proof, ports, lease, usage, version cache, migrate stage, localdata, orchestrator, settings store, hook count); Claude config (settings/claude.go, claude_user.go, claude_statusline.go, agentsync.go, hook.go:164 env file); temp (eval, atomicfile, CreateTemp beside targets); OS scheduler files (routine); person's folders (account folder, migrate worktree pointers, doctor `--out`); build output (packaging); test helpers (golden, jobtest, env/testdata). Each non-Grid-home site is in a row of docs/privacy.md or its "Not written" note.

## Differs from the brief

- `~/.codex/config.toml` is read only on base: `readCodex ... writes nothing` (internal/settings/claude_user.go:451) and budget.go:611 only builds a preview. No call site writes it. The brief says the budget rows write it when applied; the page says read only. The budget apply writes the Claude settings file (budget.go:939).
- More paths outside the Grid home than the brief listed: routine timer files (`~/.config/systemd/user`, `~/Library/LaunchAgents`, `~/.grid-routine-timers`), `~/.claude-<name>` account folders, worktree pointer files during a migration, the `--out` support bundle path. They are in the table.

## What changed

- `docs/privacy.md`: the table and two short notes.
- `internal/hook/ex_outside_home_test.go`: `TestExHomeLevelOnWritesOnlyAgents`, `TestExHomeEnvFileGetsItsLinesAtEveryLevel`, `TestExHomeKeepsThePersonsOwnAgents`; helpers start with `exHome`; every path is under `t.TempDir()`.

## NOT VERIFIED on a live screen

Nothing here is a screen. The test does not run a real Claude Code, so what Claude Code does with `CLAUDE_ENV_FILE` is not verified. Tests run with `GOOS=windows`/`darwin` were vetted, not run.
