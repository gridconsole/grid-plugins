# C-gg-cli-unknown-flag-walk

Base: b87fc36458332a9e2bd680ff9e3ed5a896638900 (lane branch start, on main 4e736654 plus the C-fu2 lane docs). No test on the base walks contract/commands.json for an unknown flag.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. TestGgUnknownFlagWalkCoversTheContract: walked set = every json:true subcommand read at test time; fails when walked+skipped differs by one | HOLDS | `go test -count=1 -v -run TestGgUnknownFlag ./internal/cli/` exit 0: `walked 137, skipped 7, contract 144`. The 144 is counted by a second, untyped decode (gg_unknown_flag_walk_test.go `ggFlagCount`). Mismatch proof: `ggFlagMembers` returned `out[1:]` locally → `FAIL TestGgUnknownFlagWalkCoversTheContract`: `walked 136 + skipped 7 = 143, contract has 144 json:true subcommands` and `walked set differs from the contract`; restored (`return out`), test passes again. |
| 2. TestGgUnknownFlagWalk: `--gg-no-such-flag` before and after `--json` → exit 2, stdout one JSON object with `error.code` or top-level `code` = invalid_argument | HOLDS for 128 of 137 walked; the other 9 are recorded in `ggFlagKnownOpen` (row 4, FINDING rows) | Same run, exit 0: `--- PASS: TestGgUnknownFlagWalk`. Each of the 137 runs twice (flag first, flag after `--json`). |
| 3. Snapshot of every path and its bytes under the harness root before each run equals the one after | HOLDS | `ggFlagRun` snapshots `h.root` (home, Grid home, tmp, checkout, input file) before and after each of the 274 runs; no member changed a path (no `changed paths` text in any FINDING). `TestGgUnknownFlagSnapshotSeesAWrite` PASS proves the snapshot sees an added, a changed and a removed file. |
| 4. Misbehaving member recorded in `ggFlagKnownOpen`, test green, fails on an unlisted misbehaver, on a listed member that is fixed, and on one gone from the contract; `known open: N of M` | HOLDS | Logged: `known open: 9 of 137`. Map is below. Checks at `TestGgUnknownFlagWalk` (the `len(problems) > 0 && !listed` and `len(problems) == 0 && listed` cases) and `TestGgUnknownFlagWalkCoversTheContract` (`no longer in contract/commands.json`). First run with an empty map failed on exactly these 9 (not shown here to save space). Production code is untouched. |
| 5a. `git diff main --stat` names only the new file | HOLDS for the code; this proof file is the one extra, required by the runbook | `internal/cli/gg_unknown_flag_walk_test.go | 383 +++` plus `docs/lanes/C-gg-cli-unknown-flag-walk.md` (committed separately). |
| 5b. `go test -count=1 -v -run TestGgUnknownFlag ./internal/cli/` PASS with the known open line | HOLDS | exit 0: `--- PASS: TestGgUnknownFlagSnapshotSeesAWrite`, `--- PASS: TestGgUnknownFlagWalkCoversTheContract`, `known open: 9 of 137`, `--- PASS: TestGgUnknownFlagWalk`, `ok .../internal/cli`. |
| 5c. `make lint` exit 0 | HOLDS | `make lint` runs `go vet ./...`; exit 0, no output. `gofmt -l internal/cli` prints nothing. |
| Tests of changed package and importers | HOLDS | `go list` importers of internal/cli: `cmd/grid`, `lib` (and cli itself). `go test -count=1 ./internal/cli/` exit 0 (62s); `go test -count=1 ./cmd/grid/ ./lib/` exit 0 (cmd/grid has no test files; lib ok). |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base | n/a | The lane adds a test file and no product change; it cannot fail on the base. The base failures it finds are the 9 FINDING rows, recorded as known open instead of failing (brief row 4). Before the map was filled, the same run failed on exactly these 9 (`... misbehaves and is not in ggFlagKnownOpen`). |
| No real home touched | HOLDS | `ggFlagFixtureFor` sets HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME under `h.root` (t.TempDir), t.Fatal when HOME is outside it; `h.env.Getenv` returns "" for every key. No child process is started by the test. |
| No guard, gate, cap or hook added; nothing removed; no production edit | HOLDS | diff is one new test file. |
| Public repo: no personal paths, hosts or emails | HOLDS | Only `<ROOT>` and repo-relative paths appear in the file. |
| Test changes: no existing test edited or deleted | HOLDS | guest_parity_test.go and error_codes_test.go are read only. |

## Skip map (7 of 144, printed by the coverage test)

Each would act on the machine when it runs, so it is not run:

- `open`: internal/cli/open.go:67 starts the system browser or app (exec.Command, cmd.Start at :68)
- `land`: internal/land/process.go:50 runs the deploy command
- `routine run`: internal/routine/execute.go:166 runs the routine's command
- `routine schedule`, `routine unschedule`: internal/routine/schedule.go:37 runs the system scheduler program
- `account sign-in`: internal/account/account.go:182 runs the agent program to sign in
- `eval run`: internal/eval/process.go:58 runs the evaluation command

An unknown flag in these is therefore NOT walked; a checker with a throwaway machine could.

## FINDING rows: `ggFlagKnownOpen` (`known open: 9 of 137`)

Request seen with `--gg-no-such-flag` before `--json` and again after it gives the same answer, no file changed:

| Member | Answer on the base |
|---|---|
| job lint | exit 2; stdout `{"ok":false,"findings":[],"error":"unknown flag --gg-no-such-flag"}` (own envelope, no `code`); stderr empty |
| job render | same as job lint |
| job show | same as job lint |
| job verify | same as job lint |
| job trace | exit 2; stdout `{"rows":[],"problems":[{"task":"","job":"","text":"unknown flag --gg-no-such-flag"}]}` (own shape, no `code`); stderr empty |
| deliver | exit 2; stdout `Usage: grid deliver --outcome <text> [files...] ...` (usage on stdout, so no envelope); stderr `flag provided but not defined: -gg-no-such-flag` |
| proof add | exit 2; stdout `Usage: grid proof add <item> <file\|--text text> ...`; stderr `flag provided but not defined: -gg-no-such-flag -by string ...` |
| proof index | exit 2; stdout `Usage: grid proof index [--task <task>] [--json] ...`; stderr `flag provided but not defined: -gg-no-such-flag -json print JSON ...` |
| history add | exit 2; stdout `Usage: grid history add <line> [--task <task>] [--json] ...`; stderr `flag provided but not defined: -gg-no-such-flag -json print JSON ...` |

The four `job` verbs with `{"ok":false,...}` keep their own envelope, which has an `error` string and no `code`; the brief reads that as a miss unless it carries a top-level `code` of invalid_argument. `deliver`, `proof add`, `proof index`, `history add` print usage on stdout, which is what stops `commandFailure.finish` (error_codes.go:76) writing the envelope. The fix belongs to a product lane (not done here).

## What changed

One new file, `internal/cli/gg_unknown_flag_walk_test.go`: `TestGgUnknownFlagWalkCoversTheContract`, `TestGgUnknownFlagWalk`, `TestGgUnknownFlagSnapshotSeesAWrite`, helpers `ggFlag*`, maps `ggFlagSkip` and `ggFlagKnownOpen`. Argv per member: guestParityCommands argv (placeholders {task}→T-01, {file}, {project}→example, {cwd}) when it has the member, else `parity` per `<...>` positional; every required flag not yet in it gets a placeholder value; then `--gg-no-such-flag --json` and `--json --gg-no-such-flag`. One fresh harness per run, project `example` registered.

## NOT VERIFIED

- Nothing needs a live screen; this is a CLI test lane.
- The 7 skipped members are not walked (see skip map).
- The flag is placed after the positionals, as the brief says. A verb that stops parsing at its first positional could read the flag as an operand; a member that passed did so with exit 2 and invalid_argument, so a pass is not hidden, but the flag was not also tried before the positionals.
