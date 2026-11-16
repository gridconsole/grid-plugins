# P-bw2-hook-rules-gate-flag

`grid hook levels --json --detail` rows gain `"gate": bool`, true for home-allow, host-guard, job-lint, only-writer, plan-guard, stop-check. It reports which rules are gates; it adds no gate and changes no level or rule behaviour.

## Proof table

| Row | Check | Result |
|---|---|---|
| Base check | `git grep -n 'json:"gate"' main -- internal/settings` empty | HOLDS (no gate field on main) |
| 1 | `go test -count=1 -v -run TestBw2 ./internal/cli` shows `--- PASS: TestBw2DetailMarksTheSixGates`, exit 0. On base (main's catalog.go put back) it fails: `host-guard: gate = <nil>, want a bool` ... `--- FAIL: TestBw2DetailMarksTheSixGates` | HOLDS |
| 2 | `TestBw2GateFlagDoesNotFollowTheLevel` passes; on base fails: `plan-guard = map[... level:off ...], want level off and gate true` | HOLDS |
| 3 | `go test -count=1 ./internal/cli ./internal/settings` exit 0 (TestHookLevelsDetailShape now 5 keys, TestHookLevelsWithoutDetailIsUnchanged, TestHookRulesCoverEveryHook, TestHookRuleEventsAreWired inside) | HOLDS |
| 4 | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok | HOLDS |
| 5 | `make lint` exit 0; `make cross` exit 0 (darwin/arm64, windows/amd64 vet and test compile) | HOLDS |
| 6 | `git diff main --stat`: catalog.go, hook_levels_detail_test.go, bw2_hook_gate_flag_test.go, contract/README.md, this doc | HOLDS |
| Importers of settings | account, brain, doctor, guard, hook, job/jobcmd, updates, packaging, lib, cmd/grid, cli: `go test -count=1` ok (updates, cmd/grid have no tests) | HOLDS |
| Walk tests | TestHsIpWalksEveryCommand, TestIlGuestSiteList, internal/job/lint ok | HOLDS |
| Merge main | `git fetch origin && git merge origin/main`: already up to date | HOLDS |
| App-only walks (webui-test, realplugin guest walks) | not run, not asked (plugin repo) | not run, not asked |
| Whole-module test, cross-repo set | not run, not asked (orchestrator at landing) | not run, not asked |

## What changed

- internal/settings/catalog.go: `Gate` on HookRule after Level; `gate` on hookInfo, true on the six rows; HookRules copies it.
- internal/cli/hook_levels_detail_test.go: key count 5, message names `gate`.
- contract/README.md: row shape gains `"gate": bool` with one sentence.
- internal/cli/bw2_hook_gate_flag_test.go: two new tests.
- Text form of `--detail` and plain `--json` unchanged.

## NOT VERIFIED on a live screen

Nothing here is UI. The app reader (hooks_read.go) was not run against the new shape.

DONE
