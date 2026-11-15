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
| 2. Row 11 test | HOLDS | `TestIkHelpGoesToStdoutUnderItsOwnName`: main `[feed tick --help]: exit 0, want 0 with "grid feed tick" on stdout and nothing on stderr` and the same for job next, job cancel, approve, plan check, routine add. Head: PASS. |
| 2. Row 13 test | HOLDS | `TestIkBrainWriteDecisionNamesExpect`: main (doc change stashed) `D-ie-brain-write-decision.md names --rev; the built verb takes --expect` and `does not give the verb line with --expect`. Head: PASS. Doc diff: every `--rev` in the decision doc is now `--expect` (the doc's `brain write` rows and its verb line). |
| 3. go test of changed packages and their importers | HOLDS | `go test -count=1 ./internal/cli ./internal/job/... ./internal/jobgraph/... ./cmd/grid ./lib` exit 0: `ok internal/cli 122.595s`, `ok internal/job 4.380s`, `ok internal/job/jobcmd 4.691s`, `ok internal/job/lint 0.628s`, `ok internal/jobgraph 1.080s`, `ok lib 20.145s`; cmd/grid has no test files. Importers from `go list`: cmd/grid, internal/cli, lib. |
| 3. make lint | HOLDS | `make lint` exit 0 (`go vet ./...`, no output). |
| 3. Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| 3. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v`: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok`. `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.589s`). |
| Merge of origin/main before the last run | HOLDS | `git fetch origin && git merge origin/main`: `Already up to date.` |
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
- `internal/cli/contract_inventory_test.go`: the probe skips only the `hook` event route; `hook level` and `hook levels` are probed. `contract/commands.json` regenerated (`make generate`).
- `testdata/golden/help.golden`: the three summary changes (generated file the changed table forces).
- `docs/decisions/D-ie-brain-write-decision.md`: `--rev` to `--expect`.

## Existing tests whose pinned expectation changed (the brief's fix changes the pinned behaviour; no assertion was loosened)

- `internal/job/jobcmd/command_refusals_test.go:50` and `internal/cli/he_jp_removed_test.go:58,64`: the unknown-verb text now lists all verbs (row 4 fix: "make the job unknown-verb text list every verb").
- `internal/cli/ik_job_usage_test.go`: bare `grid job` now expects exit 2 with the usage on stderr (row 3 fix: "exit 2 on a bare `grid job` as `grid feed` does"); the exit-line and cycles-line checks are unchanged.
- `internal/cli/orchestrator_test.go:296`: `lease take task --help` is read from stdout, with stderr empty (row 11 fix: usage to stdout).
- `testdata/golden/help.golden`: edited by hand for the three summaries because the golden writer needs an env var the shell here refuses; `TestDtGateWordsHelpGolden` passes.

## Not covered

- Row 11 says "send --help usage to stdout everywhere". Done for every flag set that goes through `parse()` and for approve, plan, routine and the task verbs listed above. Not walked: other hand-rolled flag sets that still print Go's default usage on stderr (for example those that call `parseMixed` with no env). Open item for a follow-up lane.
- NOT VERIFIED on a live screen: nothing here is UI.
