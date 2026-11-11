# O-hook-brain-show-read: proof

Base: main at 330256ce. Lane head: see the report (the sha of the commit that adds this file).

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. A test through the hook entry fails on main | HOLDS | On 330256ce (brain.go reverted, test file kept): `go test -count=1 -run 'TestBrainShow\|TestBrainReadNamesTheJob' ./internal/hook/` exit 1. Failing lines: `brain_show_read_test.go:76: history: ` (empty, no `brain read` line after a Bash `grid brain show deploy`), `brain_show_read_test.go:91: home id: `, `:117: "grid brain show deploy" not recorded: `, `:156: via: - ... · brain read · brain:project/deploy · by claude` (no `via J-3`), `:165: worktree cwd did not log to GRID_TASK: `. Test: `TestBrainShowLogsOneReadLine`, internal/hook/brain_show_read_test.go. |
| 2. One line `brain read · brain:project/deploy`, resolved id, dedupe shared with Read | HOLDS | `TestBrainShowLogsOneReadLine` (line asserted `· brain read · brain:project/deploy · by claude`, no `via`; repeat show, `--json` show and a Read add nothing, with a line for another note between so history's own repeat suppression cannot hide the key); `TestBrainReadThenShowAddsNothing`; `TestBrainShowResolvesAsTheCLIDoes` (`brain:home/x`, `--level home deploy` for a project-only note logs nothing). Code: internal/hook/brain.go:82-96 `recordBrainRead` uses key `read:<id>` for both paths; id from `Store.Show` at brain.go:114. `go test -count=1 -race ./internal/hook/` exit 0. |
| 3. Recognised and ignored forms | HOLDS | `TestBrainShowRecognisedForms` (plain, absolute path to grid, flags before and after the id, `--task`, `&& b`, `\| jq`, quoted id, redirects `> f` and `2>&1`) and `TestBrainShowIgnoredForms` (search, `echo grid brain show`, `ls && grid brain show`, no id, only flags, unknown note, `--proposal` before and after, two ids, unknown flag, unterminated quote, `mygrid`, empty command); each asserts no stdout, no log output, exit 0, and an empty history. Parser: brain.go:125-225. |
| 4. `via <Job id>` when GRID_JOB is set, on both paths | HOLDS | `TestBrainReadNamesTheJob` (GRID_JOB=J-3: a show and a Read both give `· by claude · via J-3`); without GRID_JOB the line is as on main (`TestBrainShowLogsOneReadLine` asserts no `via`; existing `TestRecallPointersReadDedupeAndPlanChange` passes). brain.go:94. |
| 5. Worktree cwd logs to GRID_TASK; history-log off writes nothing | HOLDS | `TestBrainShowFromWorktreeAndHookOff`: cwd is an unrelated temp dir, line lands in the GRID_TASK task; `settings.Store.SetHook("history-log","off")` then show writes nothing. brain.go:102-120 uses `taskDir` (events.go:155) and `hookEnabled("history-log")`. |
| 6. Existing hook tests unchanged and green; vet | HOLDS | `go test -count=1 -race ./internal/hook/` exit 0 (`ok ... 6.4s`); `go vet ./internal/hook/` exit 0; `GOOS=windows CGO_ENABLED=0 go vet ./internal/hook/` exit 0. No existing test file edited (`git diff --stat 330256ce..HEAD`: brain.go and the new test only). |
| Importers of the changed package, tested | HOLDS | `go list` importers of internal/hook: internal/cli, internal/packaging. `go test -count=1 ./internal/cli/ ./internal/packaging/` exit 0 (both `ok`). |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard, gate, cap or hold on agents | HOLDS | The hook still only records: no deny, no stdout (tests assert empty stdout and log), unparseable command returns silently (brain.go:103-105). Called inside `o.record`, so a panic is logged and exits 0. |
| Packages touched | HOLDS | Only internal/hook/brain.go and the new internal/hook/brain_show_read_test.go (plus this file). events.go unchanged: PostToolUse already reaches `brainRead` for every tool (events.go:112, :124). braincmd and task untouched. |
| Public repo hygiene | HOLDS | No personal paths, hosts or emails in the diff; commit message has no attribution line. |
| Deleted tests | HOLDS | None deleted. |

## What changed

- `brainRead` (internal/hook/brain.go:29) now also takes `Bash` on PostToolUse and hands it to `brainShow`.
- `brainShow` parses the command with `parseBrainShow`, resolves the id with `Store.Show` (project first, `brain:<level>/<name>` explicit, `--level` folded in as the CLI does), and appends through `recordBrainRead`.
- `recordBrainRead` is the old Read tail, shared by both paths: mark `read:<id>` under the session lock, then append under `.task.lock` after the session lock is released (order kept: session lock, then `.task.lock`). It passes `GRID_JOB` as `via`.
- `parseBrainShow` and `shellWords` read only the first simple command up to an unquoted `&&`, `||`, `;`, `|`, `&` or newline; nothing is run or expanded. `--proposal`, an unknown flag, a second id or a missing id gives no line.
- Behaviour note: a Read now also carries `via <Job>` when GRID_JOB is set (row 4 asks for it).
- Behaviour note: the first command of a line is the only one read; `ls && grid brain show x` is not recognised (the brief says first command).

## NOT VERIFIED on a live screen

- A real Claude Code Bash PostToolUse payload: the test input uses `tool_input.command` as in the hook's other Bash handling (internal/hook/history.go bashHistory reads the same hookio.Input). The scorecard rows "Notes read" and "Notes looked up" agreeing on a live Job were not checked.
- `grid brain show` run for real by an agent whose cwd is a worktree.
