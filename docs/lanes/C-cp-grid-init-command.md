# C-cp-grid-init-command

Base: plugin main a6c5cb02 (the brief was planned on 414347a3; this clone started at a6c5cb02). On base, `git grep -n '"init"' -- internal/cli contract/commands.json` shows only internal/cli/ship_land_test.go:37 and internal/cli/ve_task_merge_test.go:35, both git calls in tests, so there was no `grid init`. Before the last proof run, `git fetch origin && git merge origin/main` merged a9f80b38. `make generate` after the merge changed nothing.

## Proof table

| Row | Result |
|---|---|
| 1 `go test -count=1 -run 'CpInit' ./internal/cli/` (Detect) | HOLDS, exit 0. `--- PASS: TestCpInitDetect` with one subtest per contract/gridconfig case: brain-machine, broken, none, old-and-new, owner-v3, v1-elsewhere, v1-login, v1-member, v1-workspace, v2-owner, v3. Each checks data and source against the case's expect.json, the member, the asked flags (sandbox, testDefault and packs not asked) and the output against cmd-init. The subtest `member_falls_back_to_displayName_then_login` shows the config's member, then the account's displayName (`sam-example`, source account), then the login (`ann-login`, source login). |
| 2 `TestCpInitAnswers` | HOLDS, exit 0. The `v3` case writes at `gridconfig.Path` with v 3, keeps the unknown keys `laterTop` and `laterKey`, registers the kept `~/.claude-bob` and not the dropped `~/.claude-cara`, and a second run reports `written:false`. A byte snapshot of every file under the temp home is identical after the second run (no sleep). `none` writes a new config that `gridhome.Resolve` reads back, and is idempotent too. Bad answers (not a plain name, an unknown key, a relative path) exit non-zero and write nothing. |
| 3 `TestCpInitHomeWins` | HOLDS, exit 0. `--workspace acme` on v3 gives acme's data `~/.grid/acme` from the config; `GRID_HOME` gives source env, not asked; `--grid-home` gives source flag, not asked, and `--answers` with it writes that folder. `--workspace acme --answers -` sets current to acme, and Resolve follows it. |
| Red on base | HOLDS. The test file was placed on a base worktree at a6c5cb02: `--- FAIL: TestCpInitDetect`, `--- FAIL: TestCpInitAnswers`, `--- FAIL: TestCpInitHomeWins`; for example `cp_init_test.go:181: grid init --detect: exit 2: {"error":{"code":"invalid_argument","message":"grid: unknown command \"init\". Run: grid help"}}`. |
| 4 `go test -count=1 ./internal/cli/` | HOLDS, exit 0 (`ok ... 48.921s`) on the merged head. No existing test edited; the setup tests pass unchanged. |
| 5 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0 (`ok ... 0.134s`) on the merged head. `init` is in hsIpTable as binary-only, because it scans account folders (internal/cli/inprocess.go). |
| 5 `go test -count=1 -run 'TestContractCommandsInventory\|TestContractCommandsParserInventory' ./internal/cli/` | HOLDS, exit 0 (`ok ... 3.889s`). |
| 5 `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0 (`ok ... 0.267s`). |
| 6 `make lint && make cross && echo LINT_CROSS_EXIT_0` | HOLDS: `LINT_CROSS_EXIT_0` printed on the merged head (go vet; darwin/arm64 and windows/amd64 vet and test compile). |
| Doc fix | HOLDS. docs/research/first-run-config.md sections 5 and 6 now name `~/.config/grid/config.json` (Windows `%APPDATA%\Grid\config.json`) and the launcher at `bin/grid` in the machine folder (`~/Library/Application Support/Grid`, `~/.local/state/grid`, `%LOCALAPPDATA%\Grid`; see internal/gridconfig/gridconfig.go:12 and internal/machinefolder/machinefolder.go:62-63). Sections 1 to 4 are left as they were. |
| Changed packages and their importers | HOLDS. `go list` gives cmd/grid, internal/cli, internal/runner and lib as importers of internal/cli or internal/account. `go test -count=1 ./internal/cli/ ./internal/account/ ./internal/runner/ ./cmd/grid/ ./lib/ ./internal/contract/... ./internal/gridconfig/`: all ok; runner and cmd/grid have no test files. |
| `make generate` | HOLDS. contract/commands.json gains the `init` entry (flags --answers, --detect, --json, --workspace; output cmd-init.json). |
| Contract schema and fixtures | HOLDS. contract/schemas/cmd-init.json is a oneOf of the detect shape and the answers shape; contract/fixtures/cmd-init has 3 valid and 3 invalid fixtures. The tests validate the real output against it. |
| Touches | NOT DONE. internal/account/cp_register_kept.go (new, `account.RegisterKept`) is outside the brief's Touches. "registers the kept accounts" needs a subset registration, and no exported call does one: `Scan` (internal/account/account.go:159) lists every folder, and `Status(env, true)` (internal/account/registry.go:330) registers every scanned folder. A workaround in internal/cli would bypass the registry lock or record the wrong folders. Fix: add internal/account to Touches. |
| Touches: files the walks force | README.md (`TestLtReadmeNamesEveryCommand`: `## Commands` must name `grid init`), contract/README.md (`TestIySchemaRows`: one row per schema) and testdata/golden/help.golden (`TestDtGateWordsHelpGolden`; edited by hand because the `UPDATE_GOLDEN=1 go test` form is refused). Each is one added line. |
| Shared test table | HOLDS. The only edit is the `"init": {output: "cmd-init.json"}` row in internal/cli/contract_inventory_test.go, which the walk requires. New tests are in the new file internal/cli/cp_init_test.go, with helpers prefixed `cpInit`. |
| No existing test edited to pass | HOLDS. |
| A missing config is an offer | HOLDS. `--detect` on `none` exits 0 and offers defaults; `--answers` on `none` writes a new config. A broken config is reported by `--detect` (exit 0, `problem` set). `--answers` refuses to overwrite it and keeps the file. |
| Remove nothing | HOLDS. No file or code removed. |
| No guard, gate, cap, hold or blocking hook | HOLDS. None added. |
| No sudo, no claude, no curl, no server, no push | HOLDS. |
| No private paths, hosts or emails in committed files | HOLDS. Fixtures use `/home/sam/...` and `@example.com` placeholders. |
| No attribution lines | HOLDS. `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0. |
| App-side walk tests | not run, not asked (plugin repo). |
| `UPDATE_CONTRACT_COMMANDS=1 go test` direct form | not run, the brief forbids it (`make generate` used). |

## Open items

- Section 5 also lists `claude` and `codex` on PATH and in the usual folders (`~/.local/bin`, `/opt/homebrew/bin`, ...). That detection is not built; accounts come from the account folders and the registry.
- The `claudeSettings` answer is recorded in the config's choices only. `grid setup` still writes the Claude Code entries.
- A workspace entry that init creates gets `setup: "started"`.
- When no member can be found (no config member, no account displayName, no login), `--answers` without a `member` exits 1 with "member \"\" is not a plain name". The app always sends the member it asked.
