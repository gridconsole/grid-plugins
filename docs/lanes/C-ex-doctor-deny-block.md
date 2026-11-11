# C-ex-doctor-deny-block

Base: main bb3c1340 was not what the clone held; the clone's base is `7d516e1c` (origin/main at lane start). On it doctor had only `allow block` (installation.go checkAllowBlock, doctor.go:89); no deny or sandbox check existed, so the lane went ahead.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. After `grid setup --yes`: `ok deny block`, `ok sandbox: off; ...`, exit 0; after `--sandbox`: `ok sandbox: on ...` | HOLDS | `go test -count=1 -v -run TestExDeny ./internal/cli/` PASS `TestExDenyDoctorAfterSetup` (internal/cli/ex_deny_block_cli_test.go:61) |
| 2. One deny rule removed: `deny block` not ok, "missing rules: Edit(~/.grid-person/**)", fix "Run grid setup", exit non-zero, allow line unchanged | HOLDS | PASS `TestExDenyDoctorMissingDenyRule` (ex_deny_block_cli_test.go:75): plain and `--json` output |
| 3. Missing file, unreadable file, invalid JSON: deny reasons and fix equal allow's; Grid home unavailable: "skipped: Grid home is unavailable" | HOLDS | PASS `TestExDenyBlockSameReasonsAsAllow` (3 sub-tests), `TestExDenyBlockSkipsUnavailableHome` (internal/doctor/ex_deny_block_test.go) |
| 4. Doctor writes nothing (bytes and mtime unchanged after every case of rows 1 to 3) | HOLDS | `exDenyUnchanged` / `exDenyDoctor` are called in every case of both new files; mtime is aged 48h first in the doctor tests |
| 5. TestAllowBlockWithInjectedSettingsPath, doctor_test.go table, TestRunMissingHome, installation_test.go, TestDoctorGolden pass; diff of allow_test.go and doctor_test.go is only the deny rules and the :60 count | HOLDS | `go test -count=1 ./internal/doctor/` ok; `TestDoctorGolden` ok in `go test ./internal/cli/` (goldens regenerated with `make generate`, exit 0, which touched only the 5 doctor goldens). `git diff main --stat`: allow_test.go 7 lines (deny rules), doctor_test.go 6 lines (the count) |
| 6a. `git diff main --stat` names only Touches files | NOT DONE, one file added | `internal/cli/settings_choke_test.go` (one line): see open item 2 |
| 6b. `go test -count=1 ./internal/doctor/ ./internal/settings/ ./internal/cli/`: 0 fail | NOT DONE | doctor ok, settings ok; cli has 1 failing existing test, `TestDoctorAllowBlockFollowups` (6 sub-tests): see open item 1 |
| 6c. `make lint` | HOLDS | exit 0 (`go vet ./...`, gofmt clean) |
| 6d. `GOOS=windows go vet ./internal/doctor/ ./internal/settings/` | HOLDS | exit 0 (also ./internal/cli/); `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Rule: new tests fail on base | HOLDS | With main's doctor.go, installation.go, claude_status.go, cli/doctor.go: `TestExDenyDoctorAfterSetup` FAIL (`ex_deny_block_cli_test.go:66: exit 0: ok grid home ... ok allow block: ...` with no deny or sandbox line), `TestExDenyDoctorMissingDenyRule` FAIL (`:103: exit 0 ...` same output); doctor package: `vet: ex_deny_block_test.go:201:4: o.Projects undefined (type Options has no field or method Projects)` |
| Rule: packages that import the changed ones | HOLDS except open item 1 | Direct importers of internal/doctor or internal/settings (`go list`): account, cli, doctor, guard, hook, orchestrator, settings. Run: account ok, doctor ok, guard ok, hook ok, settings ok, orchestrator ok (first run), cli: only the item-1 failure |
| Rule: no guard, gate, cap or hold on agents | HOLDS | doctor only reads and reports; `sandbox` is always ok and never changes the exit code (`TestExDenySandboxNeverFailsReport`) |
| Rule: no deleted tests, no existing test edited to pass | HOLDS with one note | Only `settings_choke_test.go:104` got two names in its reviewed-exports list (item 2); `TestDoctorAllowBlockFollowups` was left unedited |
| Rule: `*RealPlugin*` tests | n/a | no row needs the built plugin |
| Rule: no personal paths, hosts, emails | HOLDS | tests use t.TempDir() and the harness home only |

## What changed

- `internal/settings/claude_status.go`: `ReadClaudeDenyStatus(path, home, gridHome, projects)` (the `permissions.deny` rules of `claudeBlock`, same path-free reasons as the allow reader) and `ReadClaudeSandbox(path)` answering `on`, `off` or `unknown`. A missing file or absent key is `off`; no path, unreadable, invalid JSON or a non-boolean `enabled` is `unknown` with a reason. The file read and parse is now one shared helper, so the allow reader's reasons are unchanged.
- `internal/doctor`: `checkDenyBlock` ("all setup deny rules are present" / "missing rules: ..."), hint case shared with `allow block`, `checkSandbox` (always ok), `Options.Projects`. The deny check is skipped like the allow one when the Grid home is unavailable; the sandbox line is always printed.
- `internal/cli/doctor.go`: passes GRID_PROJECTS, made absolute against the cwd the way `grid setup` does (settings.go:191-194).
- Tests: `healthyAllowSettings` also writes the deny rules; `TestRunMissingHome` checks by name; two new files; 5 goldens updated.

## Open items

1. **Existing test fails: `internal/cli/doctor_allow_followup_test.go:98`** (`TestDoctorAllowBlockFollowups`). It counts failures and requires `failures == exit code` (1). With a bad allow block, the deny block now fails with the same reason (that is brief row 3), so there are 2 failures. The brief did not list this file; the rule forbids editing an existing test to make it pass, so I left it unedited. Suggested fix, for the owner or orchestrator: in that loop, do not count `deny block` (`if !check.OK && check.Name != "deny block" { failures++ }`), or expect 2 failures where the allow block fails.
2. **`internal/cli/settings_choke_test.go:104`** is not in Touches. `TestSettingsStoreChokePoint` lists every exported function of the settings package; the two new readers had to be added to that list (`ReadClaudeDenyStatus`, `ReadClaudeSandbox`, both read-only, `claude_status.go`). Without this, internal/cli fails.
3. The brief's "plugin main bb3c1340" did not match the clone's base (`7d516e1c`); the code the brief cites was as described.

## NOT VERIFIED on a live screen

Nothing in this lane is a screen. Plain and JSON doctor output were proven through the CLI harness only; no real `~/.claude` was read.
