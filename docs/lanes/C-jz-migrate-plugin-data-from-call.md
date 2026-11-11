# C-jz-migrate-plugin-data-from-call

`grid migrate` takes CLAUDE_PLUGIN_DATA from the call's environment, and `migrate.Scan` reads no process variable.

Base: `dc301fce757d6374de26d8d1b68de26575341b32` (lane branch `lane-C-jz-migrate-plugin-data-from-call`, from plugin main 35af7d3c).

Head: the commit that adds this file. Every pasted run below was made on `3f4f1593`, which differs from head only by this report file.

`migrate.Scan` has one production caller on this base: `git grep -n 'migrate.Scan(' -- 'internal/**.go' 'lib/**.go' 'cmd/**.go'` gives `internal/cli/migrate.go:68`. The other `Scan(` caller is the test subprocess helper `TestCrashHelper` in `internal/migrate/recover_test.go`; it is threaded and named in the rule row below, as the brief asks.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1. Scan with no WithPluginData takes no plugin data although the process names a folder | HOLDS | `go test -count=1 -v -run 'TestJzMg' ./internal/migrate/` exit 0: `--- PASS: TestJzMgScanReadsNoProcessVariable`, `--- PASS: TestJzMgWithPluginDataIsTheOnlyWayIn`. The first asserts the plan field is empty, the `--dry` report has no `CLAUDE_PLUGIN_DATA/sessions` line and does not name the host folder, the apply manifest has no `plugin_data` and no `plugin-sessions` backup, and the host folder is unchanged. On base it fails: `jz_plugin_data_test.go:45: the plan took the process variable: plugin data ".../host-plugin-data/sessions", 2 files`. |
| 2. lib migrate apply with Environ CLAUDE_PLUGIN_DATA=callDir, test process CLAUDE_PLUGIN_DATA=hostDir | HOLDS | `go test -count=1 -v -run 'TestJzMg' ./lib/` exit 0: `--- PASS: TestJzMgLibMigrateTakesPluginDataFromCallEnviron`. It asserts manifest `plugin_data` is callDir, the call's pointer is rewritten to the new home, and hostDir (tree and bytes) is unchanged. On base it fails: `the manifest records plugin data ".../host-data/sessions" with 2 files, want ".../call-data/sessions"`. The manifest value is `<callDir>/sessions`, not callDir: `pluginSessions` rewrites the field to the sessions subfolder (`internal/migrate/plan.go:794`, not changed). |
| 3. typed `--plugin-data <otherDir>` wins over the Environ value | HOLDS | same run: `--- PASS: TestJzMgLibMigratePluginDataFlagWinsOverEnviron` (manifest names `<otherDir>/sessions`, otherDir pointer rewritten, hostDir and callDir unchanged). This test also passes on base, because the flag already won there; it pins behaviour the lane keeps. |
| extra: Environ without CLAUDE_PLUGIN_DATA migrates no plugin data although the host has one | HOLDS | same run: `--- PASS: TestJzMgLibMigrateEnvironWithoutPluginDataTakesNone`. On base it fails: `the manifest records plugin data ".../host-data/sessions" with 2 files, want none` (the original bug as a lib call). |
| 4. `go test -count=1 ./internal/migrate/ ./lib/` and `-run 'Migrate' ./internal/cli/` | HOLDS | `go test -count=1 ./internal/migrate/ ./lib/ ./internal/cli/ ...` (importer list below) exit 0 for every package; `go test -count=1 -run 'Migrate' ./internal/cli/` exit 0 (`ok ... 1.020s`). |
| 5. `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`, no findings). |
| 5. `make cross` | HOLDS | `make cross` exit 0 (background run, output saved by the harness). Stages seen in the output: `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`; the last line is `ok  github.com/gridconsole/grid-plugins/lib  0.004s`, then `[exited with code 0]`. |
| Design: plan.go drops the Getenv | HOLDS | `internal/migrate/plan.go:88` is `p := &Plan{From: from, Home: home, sums: map[string]string{}}`; `grep -n Getenv internal/migrate/plan.go` has no match. |
| Design: cli takes `env.getenv("CLAUDE_PLUGIN_DATA")` when the flag is empty and passes `WithPluginData(absolute(v, env))` | HOLDS | `internal/cli/migrate.go:65-73`. The binary keeps its behaviour: `OSEnv` leaves `Getenv` nil (`internal/cli/cli.go:113-119`) and `env.getenv` falls back to `os.Getenv` (`cli.go:84-89`). No test runs the built binary with the variable and no flag (read, not run). |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log --oneline`: `bbf33faf`, `aee9e3a3`, `3f4f1593`, then the report commit. No Co-Authored-By or Claude-Session line (the owner's rule wins over the reminder). |
| Rule: public repo, no personal paths, hosts or emails | HOLDS | the new tests use `t.TempDir()` and `/example/work/project` only. |
| Rule: new tests fail on base and pass on the lane | HOLDS | failing lines pasted in rows 1, 2 and the extra row (run with `git stash push internal/migrate/plan.go internal/cli/migrate.go`, then `git stash pop`). `TestJzMgWithPluginDataIsTheOnlyWayIn` and the flag test also pass on base; they pin kept behaviour and the option being the only way in. |
| Rule: no existing test edited to pass; nothing deleted | HOLDS, one helper edit | `internal/migrate/plugin_test.go` is untouched and no test was deleted. `TestCrashHelper` (`internal/migrate/recover_test.go:36-42`) is the subprocess entry of the crash tests and is a `Scan` caller: it called `Scan` with no option while the tests passed the folder as the process variable `CLAUDE_PLUGIN_DATA`. The brief says to thread and name each `Scan` caller. Before the edit `TestPluginSessionsCrashMatrix` failed (`plugin_test.go:51: open .../old-grid/sessions/hook: no such file or directory`; subtests done, displaced, receipt-written, previous-removed, published). The helper now reads the variable and passes `WithPluginData`, as grid migrate does. No assertion changed. The file is outside the Touches list; the reviewer should judge it. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | none added; the change moves where one folder name is read from. |
| Rule: tests of changed packages and of every importer (go list), paste | HOLDS | `go list` over Deps, TestImports and XTestImports for packages that import `internal/migrate`, `internal/cli` or `lib`: cmd/grid, contract/formats, internal/account, internal/braincmd, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/orchestrator, internal/packaging, internal/question, internal/settings, lib (plus internal/migrate itself). `go test -count=1` on all of them, exit 0: migrate 138.3s, lib 161.6s, cli 237.9s, cmd/grid no test files, formats 1.6s, account 10.1s, braincmd 11.8s, golden 5.1s, jobgraph 2.8s, mcp 2.2s, orchestrator 357.4s, packaging 39.3s, question 29.8s, settings 7.7s. |
| Rule: repo-wide walk tests (plugin) | HOLDS | the `-run` pattern TestHsIpWalksEveryCommand, a plain `|`, then TestIlGuestSiteList, is given below the table. Exit 0 (`ok ... 0.127s`). `go test -count=1 ./internal/job/lint/` exit 0 (`ok ... 0.321s`). The app walk tests (`make webui-test` and the others) belong to the app repo and do not apply here. |
| Rule: gofmt | HOLDS | `gofmt -l internal/ lib/` prints nothing. |
| Rule: tests named *RealPlugin* | N/A | no row names one. |
| Note: SCOPES-11nov-1405 question 4 (drop internal/migrate) | HOLDS | `internal/migrate` is still on this base, so the lane applies. |

The walk test command, as run:

`go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'`

## What changed

- `internal/migrate/plan.go`: `Scan` no longer seeds `pluginData` from `os.Getenv("CLAUDE_PLUGIN_DATA")`; only `WithPluginData` sets it. The `WithPluginData` comment says so.
- `internal/cli/migrate.go`: with no `--plugin-data`, the folder is `env.getenv("CLAUDE_PLUGIN_DATA")` of the call, made absolute against `env.Cwd` like the flag. A relative variable is now resolved against the working folder as the flag is; before it reached `Scan` as typed.
- `lib/lib.go` (doc comment only, outside the Touches list): the host-read bullet for the migrate plan is gone, and the doc says `migrate` takes the folder from Environ or a typed `--plugin-data`.
- `internal/migrate/recover_test.go`: the crash helper passes the variable as the option (see the rule row above).
- New: `internal/migrate/jz_plugin_data_test.go` (helpers `jzMg*`, tests `TestJzMg*`) and `lib/t2_jz_migrate_test.go`.

## Open items

- The `--rollback` conflict check (`internal/cli/migrate.go:21`) still looks at the typed `--plugin-data` only; a variable in Environ does not refuse a rollback, as before.
- No test runs the built `grid` binary with CLAUDE_PLUGIN_DATA and no flag; the lib tests cover the `env.Getenv` path and the binary path is read from `cli.go:84-89`.

## NOT VERIFIED on a live screen

Nothing here is a UI change. Not run: the built binary with the variable set in the shell and no flag.

DONE
