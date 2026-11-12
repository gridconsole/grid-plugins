# C-bw-project-env-edit-one-lock

`grid project env add|edit` now reads the stored entry, decides add or edit, merges the flags and writes, all inside one `EditConfigBy` call (one brain lock).

## Base check

At base 952f3413: `ln_project_env.go:204` `old, err := store.ReadConfig("project", "environments", *id)`; `:207` `"environment %s already exists; use edit"`; `:249` `projectenv.SaveBy(store, next, origin)`; `projectenv.go:344` `func([]byte) ([]byte, error) { return data, nil }`. The merge did not run inside the edit function, so the fix was needed.

## Proof table

| Row | Result |
|---|---|
| 1 `TestBwEnvEditByMergesUnderTheLock` | HOLDS: `go test -count=1 -v ./internal/projectenv/ -run TestBwEnv` exit 0, `--- PASS`. Red on base (EditBy stashed): `internal/projectenv/bw_edit_by_test.go:38:17: undefined: EditBy` (also :70, :81, :94), `FAIL ... [build failed]`. |
| 2 `TestBwEnvCliHeldLockKeepsBoth` | HOLDS on the lane (`-count=20` pass 20 of 20). Red on base (ln_project_env.go stashed): `bw_project_env_race_test.go:159: stored entry = {ID:dev Name:Builds Kind: ... How: ...}, want Name Builds and How Run in CI` (the same test also failed with `Name:Seed ... How:Run in CI`). Both edits reach the lock (two block events) before release, on the lane and on base. |
| 2 `TestBwEnvCliEditsKeepEveryField`, `TestBwEnvCliAddOnce` | HOLD on the lane (20 of 20). On base `go test -count=20 -run TestBwEnvCli ./internal/cli` failed every round for all three tests, e.g. `TestBwEnvCliEditsKeepEveryField: stored entry = {Name:Seed ... From:template} want all seven values`, `TestBwEnvCliAddOnce: 4 adds exited 0, want exactly one`. They did not pass by chance on base. |
| 3 One writer unchanged | HOLDS: `go test -count=1 -v ./internal/projectenv/ ./internal/cli -run 'TestBwEnv\|TestLnEnvCli'` exit 0, `--- PASS:` for RoundTrip, EditKeepsWhatHasNoFlag, RefusalsWriteNothing (all 34 subtests), GuestIsRefused, UnknownProjectCode, SessionAuthorWithoutAPerson. `go test -count=1 ./internal/projectenv/ ./internal/brain/` exit 0. No existing test file edited (`git diff origin/main --stat` below). |
| 4 TestBwEnv pass | HOLDS: every TestBwEnv test shows `--- PASS:`; the cli ones: `go test -count=20 ./internal/cli -run TestBwEnvCli` exit 0 (60 of 60 PASS lines). |
| 5 `make lint` | HOLDS: exit 0 (`go vet ./...`). gofmt -l on both packages is empty. |
| 5 `make cross` | HOLDS: exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| 5 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS: exit 0. |
| 5 `go test -count=1 ./internal/job/lint/` | HOLDS: exit 0. |
| 5 `git diff origin/main --stat` | HOLDS: only `internal/cli/bw_project_env_race_test.go`, `internal/cli/ln_project_env.go`, `internal/projectenv/bw_edit_by_test.go`, `internal/projectenv/projectenv.go`, plus this lane doc. |
| Importers of changed packages | HOLDS: `go list` shows `internal/projectenv` is imported by `internal/cli`, which is imported by `cmd/grid` and `lib`. `go test -count=1 ./internal/projectenv/ ./internal/brain/ ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (cmd/grid has no test files). |
| Merge of main | HOLDS: `git fetch origin && git merge origin/main` answered Already up to date. |
| Repo walk tests (plugin) | HOLD: the two walk tests and `internal/job/lint` are the rows 5 lines above (exit 0). App-only walks (webui, guest tables) do not apply to this plugin lane. |
| No guard, gate, cap or hook added | HOLDS: no such code in the diff. |
| `*RealPlugin*` tests | not applicable; the lane has none. |

## What changed

- `internal/projectenv/projectenv.go`: new `EditBy(s, id, origin, change func(old Entry, exists bool) (Entry, error)) (Entry, error)`. Its closure runs inside one `EditConfigBy`: parses the old bytes (nil means no entry, `exists` false), calls `change`, refuses a changed id, renders, and returns the entry as `Parse` reads it back. An unparsable stored entry answers `environment <id>: <error>` as before. Save, SaveBy, Remove, RemoveBy are unchanged.
- `internal/cli/ln_project_env.go`: the add and edit block uses `EditBy`. "environment %s already exists; use edit" and the edit ErrNotExist wrap (`environment %s: %w`) are made inside the closure on the old bytes, with the same texts. The `--json` `item` still comes from the parsed stored entry.
- Two new test files with `bwEnv` helpers and `TestBwEnv` tests; the held-lock test counts block events with `fslock.OnBlockForTest` on alpha's `.brain.lock` and bounds each wait with `bwEnvHangCtx`.

## Notes

- The brief's `TestBwEnvCliAddOnce` command line `add alpha --id dev --name N<i> --json` is refused by the command (`--kind is required`, ln_project_env.go:134); the test adds `--kind local`.
- A refused `edit` of a missing id now goes through `EditConfigBy`, so it takes the lock and may create the project's brain folder and `.brain.lock` file before it refuses; it writes no entry, no version and no audit line (TestLnEnvCliRefusalsWriteNothing still passes).
- Two edits of an entry that end in the same bytes write no second version, as before (EditConfigBy).

## NOT VERIFIED on a live screen

Nothing here is a screen. All rows are go test and make runs.
