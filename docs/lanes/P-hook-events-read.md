# P-hook-events-read: hook runs leave one line per rule, and `grid hook events` reads it

Base: origin/main (9630011b). `git fetch origin && git merge origin/main` before the last proof run: "Already up to date." (the fetch only added `origin/lane-Pint203`).

## Record format (acceptance row 1)

File: `<GRID_HOME>/.hook-counts/events.jsonl`, one JSON object per line, written by `internal/hookevents`:

    {"rule":"history-log","event":"PostToolUse","at":"2026-11-15T10:00:00Z","outcome":"ok"}

| Field | Value |
| --- | --- |
| `rule` | A name `grid hook levels` lists (`settings.HookNames()`). |
| `event` | The canonical hook event name from the `events` table (`SessionStart`, `PreToolUse`, `PostToolUse`, `Stop`, ...). A route that is not an event (the corpus calls `grid hook history-log`) records the route name. |
| `at` | UTC, RFC 3339, whole seconds. |
| `outcome` | `ok` (consulted, let the event through), `warned` (a warning or an ask, no refusal), `blocked` (a denied tool call, or Stop exit 2), `skipped` (level `off`). The brief names the field but not the values; this set is my choice. |

Nothing else goes in a line: no tool, path, prompt, session or project. The log is Grid-home wide.

Why this file: the brief says "next to the existing hook records". The existing record is `CountHomeAllow` (`internal/hook/count.go`), which writes `<GRID_HOME>/.hook-counts/home-allow.jsonl`. The new file sits in that same folder. It is under the Grid home and not under `CLAUDE_PLUGIN_DATA` on purpose: `grid hook events` is run from a terminal outside Claude Code, where `CLAUDE_PLUGIN_DATA` is not set, so a file kept there would not be found.

Bound: the plugin bounds its app log (`grid open`, `internal/cli/open_start.go`) by size: at the limit the file shifts to `.1`, `.2`, and the oldest is dropped. That code (`shiftAppLog`) is reused, not copied: its body moved unchanged into the leaf package `internal/logshift` (`logshift.go:13`, `Shift(path, limit, files)`), and `shiftAppLog` is now a one-line call (`open_start.go:227`) with its own limit and count. The events log uses 256 KiB and 3 files (`hookevents.go:27-28`), about 7,500 lines. (`home-allow.jsonl` is unbounded; it is not changed.)

## Proof table

| Row | Result |
| --- | --- |
| 1. Diff with file:line, record format here | HOLDS. Record format above. Writer: `internal/hookevents/hookevents.go:57` `Append` (recover, home must exist, one `os.Mkdir`, `logshift.Shift`, one `O_APPEND` write, 0600), `:88` `Read` (oldest copy first, reversed), `:139` `Last`. Collection: `internal/hook/ruleruns.go` (`ruleRuns`, nil-safe `note`, `levelOutcome` :38, `decisionOutcome` :47, `flushRuns` :60), `hook.go:62-65` (`Run` sets `o.runs`, calls the handler, then `o.flushRuns()`). Call sites: `settings.go:23` (every `hookLevel` read), `hook.go:502` home-allow, `hook.go:524,531,534,543` plan-guard, `stop.go:101,111` stop-check. Reader: `internal/cli/hook_events.go:22` `runHookEvents`, routed at `internal/cli/hook.go:15`, usage line `internal/cli/settings.go:25`, help summary `internal/cli/cli.go:143`. Contract: `contract/README.md:659` (levels), `:663-673` (events), `contract/commands.json:151`. `git diff --stat origin/main..HEAD`: 17 files outside the corpus, 770 insertions, 29 deletions, plus 52 corpus `files.diff` files (below). |
| 2a. A hook run writes one record | HOLDS. `TestRuleRunsOneLinePerRulePerRun` (`ruleruns_test.go:60`): SessionStart leaves `session-start` and `usage-log` once each, with event, UTC time and `ok`; PostToolUse adds one `history-log` line (history-log is consulted at four places and still leaves one). `TestRuleRunsOutcomes` (:92): `skipped` at off, `blocked` (exit 2), `warned` and `skipped` for stop-check. |
| 2b. A failed write does not change the hook's exit or output | HOLDS. `TestRuleRunsFailedWriteChangesNothing` (:134): a regular file stands where `.hook-counts` would be, so no line can be written. Exit code, stdout and stderr are compared with a run that can write, and match. The test also checks the second run did write a line, so the comparison is not empty. `TestRuleRunsNoGridHomeWritesNothing` (:157): no Grid home, no folder made. The write is also behind `recover` in `Append`, and happens after the handler returned, so it cannot change the handler's result. |
| 2c. `grid hook events --json` shape, a rule with and without runs | HOLDS. `TestHookEventsJSONShape` (`hook_events_test.go:34`): one row per `settings.HookNames()` entry in `levels` order; a rule with a line has `lastEvent`, `at`, `outcome` from its newest line; a rule with none has all three `null`. `TestHookEventsNoLog` (:92): no file, every rule null, exit 0. `TestHookEventsAllNewestFirst` (:113): `--all` is the log newest first across the shifted copies, `[]` when empty. `TestHookEventsRefusesOperands` (:140): exit 2. |
| 2d. The bound | HOLDS. `TestRuleRunsLogIsBounded` (`ruleruns_test.go:168`): `events.jsonl` seeded at 256 KiB plus `.1` and `.2`; one hook run shifts them (`.1` holds the old active file, `.2` the old `.1`), the old `.2` is gone, no `.3`, and the active file is a fresh one-line file. |
| 3. `contract/README.md` lists both, where the other reads are | HOLDS. New sections `grid hook levels [--json]` (:659) and `grid hook events [--all] [--json]` (:663-673) follow `grid hook <event>`. They state the file, the record format, the outcome values, the bound, the null rules and the exit codes. `grid hook levels` had no README section; it is added because the events rule names point at it. |
| 4. New tests fail on main, pass on the lane | HOLDS. See Tests. |
| 5. `go test ./internal/...` of the changed packages and their importers | HOLDS. Changed: `internal/hook`, `internal/hookevents`, `internal/logshift`, `internal/cli`, `internal/golden` (recordings only). `go list` importers (output below). `go test -count=1 ./...`: every package `ok` (exit 0), including `internal/hook` 10.5s, `internal/cli` 71.2s, `internal/golden` 0.67s, `internal/packaging` 8.8s, `lib` 14.8s. `hookevents` and `logshift` have no test files of their own; they are covered by the `internal/hook` and `internal/cli` tests above. |
| 6. Walk tests | HOLDS. `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `walked 39 commands, 39 table entries`, both tests and all five subtests PASS, `ok internal/cli 0.077s`. `go test -count=1 -v ./internal/job/lint/`: PASS, `ok internal/job/lint 0.246s`. |
| 7. `make lint` | HOLDS. Exit 0. Output is `go vet ./...` and no finding. |
| 8. `make cross` | HOLDS. Exit 0. darwin/arm64 and windows/amd64: vet, then test compile of every package, all `ok`. |
| 9. Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin CGO_ENABLED=0 go build ./...` both exit 0, no output. |
| 10. `git fetch origin && git merge origin/main` before the last run | HOLDS. "Already up to date." |
| 11. App guest walk tables (`make realplugin ...`) | Not run, not asked. They are app-repo checks and this lane is in the plugin repo. |
| 12. No guard, gate, cap, hold or blocking hook added | HOLDS. The write is best effort, runs after the handler, and never changes a decision, exit code or output (2b). The size bound is a log rotation, not a limit on any agent. No level, deny or wait is added. |
| 13. Never: push, /srv/gmf, the app, /root/.grid, `pkill -f` | HOLDS. Nothing pushed (the bundle is local). Work stayed in this clone. Tests use temp homes. |
| 14. Files outside the brief's Touches | Three kinds, each forced by the change and named here. (a) `internal/cli/contract_inventory_test.go`, an existing test: its walk table lists the verbs of `grid hook`, so the new verb is added (`:272` `paths["hook"]` gains `"events"`, `:432` `"hook events": {}`). Nothing else in it changed and no assertion was loosened. (b) `testdata/golden/help.golden:10` and the `hook` summary in `cli.go:143` and `commands.json:138`: `TestExHelpSummariesNameEveryVerb` requires the `grid help` line to name every verb in commands.json, so the line now ends "; read hook events". (c) The golden corpus, 52 `golden/<case>/files.diff` files (or `go-expected/files.diff` for cases with a legacy recording): every hook case records the files a run wrote, and each now also writes `.hook-counts/events.jsonl`. Re-recorded with the corpus's own flag (`go test ./internal/golden -run TestGoldenCorpus -args -xs-rerecord=<cases>`, `xs_rerecord_test.go:19`). `git diff --numstat -- golden`: every file is additions only (0 deletions), each only the `events.jsonl` hunk. No stdout, stderr or exit recording changed. Every stop-check case exits 0 (`grep -rLx 0 golden/stop-check --include=exit` lists none), and every outcome recorded in the corpus is `ok`. `internal/logshift` and `internal/hookevents` are new packages; `internal/cli/open_start.go` loses the body of `shiftAppLog` to `logshift`. |

## Importers (`go list`)

    cmd/grid imports internal/cli
    internal/cli imports internal/hook, internal/hookevents, internal/logshift
    internal/hook imports internal/hookevents
    internal/hookevents imports internal/logshift
    internal/packaging imports internal/hook
    lib imports internal/cli

All of these, and `internal/packaging/cmd` (imports `internal/packaging`), pass in `go test -count=1 ./...`.

## Tests

New tests were run on the base tree (origin/main plus only the two new test files, in a temporary worktree, removed since) and on the lane.

- `internal/hook/ruleruns_test.go`. On main:
  - `ruleruns_test.go:68: SessionStart wrote [], want session-start and usage-log once each` (TestRuleRunsOneLinePerRulePerRun)
  - `ruleruns_test.go:100: no line for session-start in []` (TestRuleRunsOutcomes)
  - `ruleruns_test.go:149: the run without the obstacle wrote no line; the comparison proves nothing` (TestRuleRunsFailedWriteChangesNothing)
  - `ruleruns_test.go:199: copies not shifted: .1 ` and `:205: the oldest copy was not dropped` and `:208: the active log after the shift: ...` (size 262197, not a fresh file) (TestRuleRunsLogIsBounded)

  `TestRuleRunsNoGridHomeWritesNothing` passes on main too: it pins an absence (no folder made with no Grid home), which main also satisfies. On the lane all five PASS.
- `internal/cli/hook_events_test.go`. On main:
  - `hook_events_test.go:56: not a JSON array: unexpected end of JSON input:` (TestHookEventsJSONShape)
  - `hook_events_test.go:99: rows []: unexpected end of JSON input` (TestHookEventsNoLog)
  - `hook_events_test.go:122: unexpected end of JSON input` (TestHookEventsAllNewestFirst)
  - `hook_events_test.go:143: an operand: exit 0, want 2` and `:146: an unknown flag: exit 0, want 2` (TestHookEventsRefusesOperands)

  On main `grid hook events` was an unknown hook event: no output, exit 0. On the lane all four PASS.
- The last commit changes one failure message in `TestRuleRunsLogIsBounded` (`%d lines` instead of printing the lines); the assertion is the same.
- No existing test was edited to make it pass, and none was deleted. `contract_inventory_test.go` gained the new verb (row 14a).

## What changed

- Every hook run (`hook.Run`) collects, per rule, the last outcome noted during the run, and appends one line per rule after the handler returns. The rules that note: every `hookLevel` read (session-start, session-end, history-log, usage-log, stop-check, plan-guard, ...), plan-guard decisions, home-allow when its fallback allows, and stop-check at block and warn.
- `grid hook events [--all] [--json]` reads the log. Default: one row per rule in `grid hook levels` order, from each rule's newest line. `--all`: the log newest first.
- The shift code behind the app log moved to `internal/logshift`, and the events log uses it.
- `contract/README.md`, `contract/commands.json`, the usage line and the help line name the new verb.

## Open items

- `host-guard`, `only-writer`, `needs-you` and `session-check` have no runtime reader (the same finding as P-r23-hook-levels row 1c), so they list in `grid hook events --json` and always read null. `job-lint` runs in `grid job render` (jobcmd), not under `grid hook`, so it is not recorded and reads null too. Recording it would mean a second writer in `jobcmd`; not asked.
- The log is Grid-home wide: no project, task or session field, so `grid hook events` answers for the whole home, not for the current project (it takes no `--project`). A project field would have to come from the brief.
- A noisy rule (history-log fires on every tool call) can push a quiet rule's last line out of the 3-file bound. That rule then reads null although it ran. If that matters, the fix is a per-rule "last" file or a larger bound; the brief asked for the bound only.
- `outcome` values are my set of four; the brief did not list them.
- No `cmd-hook-events.json` schema file was added (not asked); the README documents the output in prose and a one-line shape.
- `home-allow.jsonl` next to it is still unbounded. Not asked.
- The root `README.md:31` still says "set and list hook levels" for `grid hook`. Not in the brief's Touches; left.

## NOT VERIFIED on a live screen

Nothing in this lane has a UI row. Not run in a real Claude Code session: a hook that Claude Code itself fires, followed by `grid hook events --json` from a terminal. Tests drive `hook.Run` and the CLI in process with temp homes; the corpus drives the CLI the same way.
