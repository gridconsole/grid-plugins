# C-pcov-cli-account-agent-sync

Tests for `grid account list|status|scan|sign-in` and `grid agent sync`. One new file, `internal/cli/account_verbs_test.go`. No non-test file changed.

## Proof table

| Row | Result |
|---|---|
| 1 signedInWord, three cases | HOLDS: `TestAccountSignedInWord`; `go test ./internal/cli/ -run 'Account|AgentSync'` exit 0 (20 tests pass); signedInWord 100.0% |
| 2 account list: none registered; two lines, default tagged; --json | HOLDS: `TestAccountListLines` (exact "account: none registered", exact lines built as `account <id>[ (default)]: <email>, <word>, <dir>`, one `(default)`); the --json decode is via `h.accounts("list")` with ids claude-ann, claude-bob. `status` prints two lines too. accountStatus 94.1% |
| 3 account scan: two lines, second "no email"; --json folders; unknown flag exit 2 + usage | HOLDS: `TestAccountScanLinesAndUsage`; accountScan 100.0% |
| 4 sign-in: --dir/--kind runs the fake once, argv `auth login`, CLAUDE_CONFIG_DIR=D, stdout; --json to stderr and `{"dir","kind","exit":0}` | HOLDS: `TestAccountSignInRunsTheLoginOnTheFolder` |
| 4 home folder: nothing added for CLAUDE_CONFIG_DIR | HOLDS: `TestAccountSignInHomeFolderGetsNoFolderVariable` pins `CLAUDE_CONFIG_DIR=[]` even with a variable set in the caller's shell (account.go:174-181 strips it, child is "" for the home folder) |
| 4 unknown folder exit 1 + `grid account sign-in: ` + fake not run; `--kind bogus` exit 1 "is not claude or codex"; unknown --id exit 1 `grid account sign-in: no account`; no target exit 2 "give --id, or --dir and --kind" | HOLDS: `TestAccountSignInRefusals` (also `--dir` without `--kind` exit 2; login count 0) |
| 4 --id of a registered account resolves folder and kind | HOLDS: `TestAccountSignInByID` (account registered by `status` from the folder's own login email; it ran, not skipped) |
| 4 fake exits 3: exit 1, --json "exit": 3; no fake on PATH: exit 1 "is not installed or cannot start" | HOLDS: `TestAccountSignInFailureAndMissingProvider` |
| 4 Windows skip | HOLDS: `signInHarness` calls `t.Skip` on windows, naming the sh-script reason |
| 4 emptyReader.Read | HOLDS: `TestAccountEmptyReader` (Read gives 0, EOF; `account note` with nil stdin gives exit 1 "grid account note: " on the empty input); Read 100.0% |
| 5 no Grid home: non-zero + "set GRID_HOME"; guest: non-zero + guestSettingsRefusal | HOLDS: `TestAgentSyncRefusals` (`h.env.GridHome = ""` via the harness; guest from `guestLeakFixture(t, "guest")`; the agents folder is not created) |
| 5 --templates/--dir: written, second run all unchanged, person's file left byte-identical ("yours"), --json {"agents":[...]} | HOLDS: `TestAgentSyncWritesThenReportsUnchanged`, `TestAgentSyncLeavesThePersonsFileAlone` |
| 5 default paths: CLAUDE_PLUGIN_ROOT/agent-templates; no --dir lands in C/agents only | HOLDS: `TestAgentSyncDefaultPaths` (env through `h.env.Getenv`, plus `t.Setenv` of both variables, so the builder's own never reaches the run) |
| 5 missing --templates folder: non-zero | HOLDS: `TestAgentSyncMissingTemplatesFolder` (and no templates + no plugin root: non-zero) |
| 6 `go test ./internal/cli/ -count=1 -run 'Account|AgentSync'` | HOLDS: exit 0, `ok ... 0.053s` |
| 6 coverage | HOLDS: before 86.2% (Linux, untouched clone) -> after 88.0%. accountStatus 94.1%, accountScan 100.0%, accountSignIn 94.1%, runAgentSync 89.7% (all above 75%); also accountWindows still 73.7%, not in the list of four |
| 6 full `go test -cover ./internal/cli/` | HOLDS: `ok ... 27.225s coverage: 88.0%`, exit 0 |
| 6 `GOOS=windows go test -c -o /dev/null ./internal/cli/` / `GOOS=darwin ...` | HOLDS: exit 0 each. They compile only; the sign-in tests are compiled, needs Windows run (skipped there by design) and `GOOS=darwin go test ./internal/cli/ -run 'Account|AgentSync'` on a Mac |
| 6 `make lint` | HOLDS: exit 0 (`go vet ./...`); `gofmt -l internal/cli/` empty |
| Rule: importers of internal/cli | `go list` gives `cmd/grid`; `go test ./cmd/grid/` exit 0, no test files |
| Rule: tests fail on base | NOT APPLICABLE as written: tests only, no code change, so they pass on base too. They cover code that had no test; the first run of the draft failed 3 assertions of my own that I fixed in the test (probe runs counted as logins, nil stdin exit) |
| Rule: no non-test file, no existing test edited, no guard added | HOLDS: `git diff --stat origin/main..HEAD` shows the new test file and this report only |
| No test exposed a bug | HOLDS: no NOT DONE row |

## What changed

- `internal/cli/account_verbs_test.go` (new): 14 tests across account verbs and agent sync, with a fake `claude` sh script (builtins only, PATH set to its folder) and an agent-template fixture.
- `docs/lanes/C-pcov-cli-account-agent-sync.md`: this report.

Note on `account sign-in --id`: the `--id` path calls `account.Status(core, false)`, which probes `claude auth status` for each folder, so the fake runs for a status probe as well as for the login. The tests count only `auth login` lines.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run: the real `claude` or `codex`, a Windows host, a macOS host.
