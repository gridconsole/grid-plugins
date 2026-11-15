# P-r23-cli-help: grid help, job help and contract/commands.json match the built verbs

Findings: U-chk-plugin-rev-cli.md rows 3, 4, 10, 11, 13. Base: origin/main (`git merge origin/main` said "Already up to date" before the last proof run).

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1. Row 3 (job usage) | CONFIRMED on main | `go run ./cmd/grid job --help` exit 0 printed only render, lint, show, trace, verify, cycles; bare `grid job` exit 0 with the same text; bare `grid feed` exit 2 (`usage: grid feed post\|read\|state\|types\|handover [flags]`). |
| 1. Row 4 (advertised verbs) | CONFIRMED on main | `grid job order` exit 2 `job: unknown job command "order"; use render, lint or show`; `grid feed fold` exit 2 `grid feed: unknown command "fold"`; `grid plan diff` exit 2 `plan: unknown action "diff"; use check, hash, items, write or migrate`; `internal/cli/cli.go` summaries and `contract/commands.json` (plan, job, feed) named all three. |
| 1. Row 10 (hook flags) | CONFIRMED on main | `contract/commands.json:150-166` had `"flags": []` for `hook level` and `hook levels`, `"json": false`; `internal/cli/contract_inventory_test.go:361` skipped the probe for every `hook` route; `settings.go:25` documents `--project` and `--json`. |
| 1. Row 11 (help name and stream) | CONFIRMED on main | `grid feed tick --help` printed `Usage of grid job tick:`; `grid approve --help` printed the usage through `env.Stderr` (`task_write.go:260`); `lease take task --help` too (pinned on stderr by `orchestrator_test.go:296`). |
| 1. Row 13 (brain write doc) | CONFIRMED on main | `docs/decisions/D-ie-brain-write-decision.md:24` `--rev <16-hex\|none>`; `contract/README.md:803` and `contract/commands.json` give `--expect`. |
| 2. Row 3 test | HOLDS | `TestIkJobUsageListsEveryVerb` (internal/cli/ik_cli_help_test.go). On main: `ik_cli_help_test.go:25: job --help lacks "grid job next"` (and post, ack, result, cancel, tree, blocked), `ik_cli_help_test.go:29: bare job: exit 0, want 2 with the usage on stderr`. On head: PASS. |
| 2. Row 4 tests | HOLDS | `TestIkJobUnknownVerbNamesEveryVerb`: main `ik_cli_help_test.go:44: unknown-verb text lacks "next": job: unknown job command "frobnicate"; use render, lint or show` (all ten missing verbs); `TestIkHelpSummariesNameBuiltVerbs`: main `grid plan diff --help: exit 2, but the summary "write, check, items, hash, diff, migrate" advertises it` and `grid feed fold --help: exit 2 ...`. Head: PASS both. |
| 2. Row 10 test | HOLDS | `TestIkHookInventoryHasItsFlags`: main `hook level: commands.json lists no --project flag`, `no --json flag`, `hook levels: commands.json has json false, but --json works`. Head: PASS. `contract/commands.json` regenerated with `make generate`; `TestContractCommandsParserInventory` passes. |
| 2. Row 11 test (named verbs) | HOLDS | `TestIkHelpGoesToStdoutUnderItsOwnName`: main `[feed tick --help]: exit 0, want 0 with "grid feed tick" on stdout and nothing on stderr` and the same for job next, job cancel, approve, plan check, routine add. Head: PASS. |
| 2. Row 11 test (every verb, fix round 1) | HOLDS | `TestIkEveryBuiltVerbHelpGoesToStdout` (internal/cli/ik_cli_help_test.go) walks every built subcommand of `contract/commands.json` with `--help` (skips `hook`, the event handler, and `mcp`) and requires exit 0, usage on stdout, empty stderr. Before the round-1 fix: 74 failing lines, for example `ik_cli_help_test.go:109: grid open --help: exit 0, want 0 with the usage on stdout and nothing on stderr / stdout: "" / stderr: "Usage of grid open:..."`, the same for task status/stop/close/reopen/attach/history/rename/delete/restore/decline/summary, job cycles, question add/ask/list/sync/tick/withdraw, brain show/search/keep/drop/rename/restore/revert/changes/decide/supersede (usage split across stdout and stderr), `person check --help: exit 2`, eval list/run/show/export-suite, signal, artifact, update, and the rest (74 of the built subcommands, run with the lane's round-0 sources stashed; the reviewer counted 136 on main). Head: `go test -count=1 ./internal/cli -run TestIkEveryBuiltVerbHelpGoesToStdout -v` PASS. |
| 2. Row 11 test (bad flag stays on stderr) | HOLDS | `TestIkBadFlagUsageStaysOnStderr`: `task status`, `job cycles`, `question list`, `brain show`, `eval list`, `person check` with `--nope` exit 2, stdout empty, usage on stderr. PASS on head (guards the other half of the rule: only `--help` moves the usage). |
| 2. Row 13 test | HOLDS | `TestIkBrainWriteDecisionNamesExpect`: main (doc change stashed) `D-ie-brain-write-decision.md names --rev; the built verb takes --expect` and `does not give the verb line with --expect`. Head: PASS. Doc diff: every `--rev` in the decision doc is now `--expect` (the doc's `brain write` rows and its verb line). |
| 3. go test of changed packages and their importers | HOLDS | `go test -count=1 ./internal/cli ./internal/braincmd/... ./internal/question/... ./internal/evalcmd/... ./internal/orchestrator/... ./internal/task/... ./internal/job/... ./cmd/grid ./lib` exit 0: `ok internal/cli 178.027s`, `ok internal/braincmd 10.648s`, `ok internal/question 6.385s`, `ok internal/evalcmd 2.087s`, `ok internal/orchestrator 411.344s`, `ok internal/task 18.895s`, `ok internal/job 8.752s`, `ok internal/job/jobcmd 10.246s`, `ok internal/job/lint 0.713s`, `ok lib 57.134s`; cmd/grid has no test files. Importers of braincmd, question, evalcmd from `go list`: internal/cli, internal/orchestrator, internal/task (and cmd/grid, lib through internal/cli); jobgraph (round 0) is unchanged since the last green run. |
| 3. make lint | HOLDS | `make lint` exit 0 (`go vet ./...`, no output), re-run on the round-1 head. `gofmt -l internal cmd lib` prints nothing. |
| 3. Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0, on the round-1 head. |
| 3. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v`: `--- PASS: TestHsIpWalksEveryCommand` (walked 39 commands, 39 table entries), `--- PASS: TestIlGuestSiteList`, `ok`. `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.467s`). Run after the merge below. |
| Merge of origin/main before the last run | HOLDS | `git fetch origin && git merge origin/main`: `Already up to date.` (round 1, before the last proof run). |
| New tests fail on base | HOLDS | Failing lines above. |
| No guard, gate, cap or hold added | HOLDS | Only usage text, summaries, output stream and the inventory probe changed. |
| Make-test cross-repo and whole-module test | not run, not asked | The orchestrator runs them at landing. |
| RealPlugin tests | not run, not asked | No row names them. |

## What changed

- `internal/job/jobcmd/command.go`: usage lists next, post, ack, result, cancel, tree, blocked as well as the six render-side verbs; bare `grid job` writes the usage on stderr and exits 2; `--help` stays stdout, exit 0; the unknown-verb text lists all 13 verbs (`verbList`).
- `internal/cli/cli.go`: summaries drop `order` (job), `fold` (feed) and `diff` (plan). `parse()` sends flag usage to stdout when the arguments ask for help.
- `internal/cli/deliver.go`: `wantsHelp` and `helpToStdout`, built on the existing `usageOut` rule.
- `internal/cli/orchestrator.go`: `feed tick` flag set is named `feed tick`; the cancel usage writes to `fs.Output()`.
- `internal/cli/task_write.go` (approve), `plan.go`, `routine.go`, `task.go` (merge, status verbs), `task_lane.go`: usage writes to `fs.Output()`, which is stdout on `--help` and stderr for any other parse error.
- Fix round 1 (row 11 for every built verb): `internal/cli/cli.go` `Run` sets `env.helpAsked` (new unexported `Env` field) from `wantsHelp(rest)` before it runs a command, and `newFlags` points the flag set at stdout when it is set. That one place covers every verb built on `newFlags` (open, task status/stop/close/reopen/attach/history/rename/delete/restore/decline/summary, job cycles, env, person, worker, agent, claude settings, workflow, skill, project, project-md, update, signal, artifact and the rest) and the helpers that parse for them (`parseMixed`, `taskArgs`, `parsePositionals`). The usage of a bad flag is unchanged: stderr, exit 2.
- `internal/cli/person_check.go`: `person check --help` exits 0 (it returned 2 on `flag.ErrHelp`).
- `internal/question/command.go`, `internal/evalcmd/command.go` and `suiteexport.go`, `internal/braincmd/{command,changes,decide,hy_drop,ke_rename,supersede}.go`: these packages build their own flag sets, so each takes its usage writer from `helpOut(args, env)` (stdout on `--help`, stderr otherwise) and each custom `Usage` writes to `f.Output()`. The brain verbs printed the header on stdout and the flag lines on stderr; both now go to one stream. A bad flag in a brain verb now prints its usage on stderr, not stdout.
- `internal/cli/contract_inventory_test.go`: the probe skips only the `hook` event route; `hook level` and `hook levels` are probed. `contract/commands.json` regenerated (`make generate`).
- `testdata/golden/help.golden`: the three summary changes (generated file the changed table forces).
- `docs/decisions/D-ie-brain-write-decision.md`: `--rev` to `--expect`.

## Existing tests whose pinned expectation changed (the brief's fix changes the pinned behaviour; no assertion was loosened)

- `internal/job/jobcmd/command_refusals_test.go:50` and `internal/cli/he_jp_removed_test.go:58,64`: the unknown-verb text now lists all verbs (row 4 fix: "make the job unknown-verb text list every verb").
- `internal/cli/ik_job_usage_test.go`: bare `grid job` now expects exit 2 with the usage on stderr (row 3 fix: "exit 2 on a bare `grid job` as `grid feed` does"); the exit-line and cycles-line checks are unchanged.
- `internal/cli/orchestrator_test.go:296`: `lease take task --help` is read from stdout, with stderr empty (row 11 fix: usage to stdout).
- `internal/cli/signal_move_test.go` `TestSignalMoveHelp` (round 1): read `signal move --help` from stdout and require stderr empty, because the usage of every verb now goes to stdout (row 11 fix: "send --help usage to stdout everywhere"). The golden text `signal-move-help` is unchanged.
- `testdata/golden/help.golden`: edited by hand for the three summaries because the golden writer needs an env var the shell here refuses; `TestDtGateWordsHelpGolden` passes.

## Not covered

- Nothing from row 11 is left open: `TestIkEveryBuiltVerbHelpGoesToStdout` walks every built subcommand of `contract/commands.json` except `hook` (the event handler, which takes event names and no flags) and `mcp` (a server).
- NOT VERIFIED on a live screen: nothing here is UI.
