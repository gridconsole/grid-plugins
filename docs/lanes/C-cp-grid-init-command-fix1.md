# C-cp-grid-init-command-fix1

Carries C-cp-grid-init-command (brief of record /srv/gridwork/briefs/C-cp-grid-init-command.md). Orchestrator ruling 17:16: Touches also cover one new file in internal/account (cp_register_kept.go, no change to registry.go) and README.md, contract/README.md and testdata/golden/help.golden where the walks force them.

## Base check

Merged origin/lane-C-cp-grid-init-command (f93d78be) into main with no conflict; merged head d15c2f40. `git fetch origin && git merge origin/main` printed "Already up to date". Nothing else changed on this lane. `git diff main --stat` lists 19 files:
README.md, contract/README.md, contract/commands.json, contract/fixtures/cmd-init/{invalid/missing-field,invalid/unknown-question,invalid/wrong-type,valid/answers,valid/detect-broken,valid/detect}.json, contract/schemas/cmd-init.json, docs/lanes/C-cp-grid-init-command.md, docs/research/first-run-config.md, internal/account/cp_register_kept.go (new), internal/cli/cli.go, internal/cli/contract_inventory_test.go, internal/cli/cp_init_test.go (new), internal/cli/init.go (new), internal/cli/inprocess.go, testdata/golden/help.golden. (This lane's own report is the 20th file.)

## Proof table (all rerun on this head)

| Row | Result |
|---|---|
| 1 Detect | HOLDS. `go test -count=1 -v -run 'CpInitDetect|CpInitAnswers|CpInitHomeWins|...' ./internal/cli/` exit 0: `--- PASS: TestCpInitDetect` with subtests brain-machine, broken, none, old-and-new, owner-v3, v1-elsewhere, v1-login, v1-member, v1-workspace, v2-owner, v3, member_falls_back_to_displayName_then_login. |
| 2 Answers | HOLDS. `--- PASS: TestCpInitAnswers` (v3_kept_keys_and_kept_accounts, no_config_is_an_offer, bad_answers_are_refused). |
| 3 Home wins | HOLDS. `--- PASS: TestCpInitHomeWins`. |
| 4 setup unchanged | HOLDS. `go test -count=1 ./internal/cli/ ./internal/account/ ./internal/gridconfig/ ./internal/job/lint/` exit 0 (cli ok 83.7s). No existing test edited. |
| 5 walks | HOLDS. `--- PASS: TestHsIpWalksEveryCommand` (walked 43 commands, 43 table entries), `--- PASS: TestContractCommandsInventory`, `--- PASS: TestContractCommandsParserInventory`, `--- PASS: TestIlGuestSiteList`; `go test -count=1 ./internal/job/lint/` ok. |
| 6 lint and cross | HOLDS. `make lint` exit 0 (go vet ./...); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile, all ok). |
| Touches (ruling 17:16) | HOLDS. internal/account has one new file, cp_register_kept.go; registry.go is not in the diff. README.md, contract/README.md and testdata/golden/help.golden are the walk-forced files. All other files are the original Touches. |
| make generate | HOLDS. exit 0, `git status --short` empty afterwards. |
| Red on base | HOLDS, from the carried report (test file on base a6c5cb02: FAIL for all three CpInit tests, `unknown command "init"`). Not rerun here, not asked. |
| Doc fix | HOLDS. docs/research/first-run-config.md changed (12 lines) in the carried lane. |
| Importers of changed packages | HOLDS. go list: internal/account is imported by internal/cli and internal/runner; internal/cli by cmd/grid and lib (carried report). `go test -count=1 ./internal/cli/ ./internal/account/ ./internal/gridconfig/ ./internal/job/lint/` ok; `make cross` compiled and ran no-op tests for every package on two platforms. |
| Plugin walk tests | HOLDS. TestHsIpWalksEveryCommand, TestIlGuestSiteList, job/lint pass (above). |
| No guard, gate, cap or hold | HOLDS. None added. |
| No attribution lines | HOLDS. `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0. |
| App-side walk tests, realplugin | not run, not asked (plugin repo; no `*RealPlugin*` test in this brief). |
| UPDATE_CONTRACT_COMMANDS direct form | not run, the brief forbids it (`make generate` used). |

## What changed

Nothing beyond the carried lane: the merge only. No row failed on the merged head.

## NOT VERIFIED on a live screen

Nothing; no UI.

Open items from the carried report stand (claude/codex PATH detection not built; `claudeSettings` recorded in choices only; created workspace gets setup "started"; `--answers` without a member exits 1 when no member can be found).

DONE
