# P-run-jobs-a7-fix1: proof table

Fix round for P-run-jobs-a7, ejected from landing batch pint225 on `TestLtReadmeNamesEveryCommand`.

## Cause
The failure after `git merge origin/main`:

    lt_readme_groups_test.go:47: README.md ## Commands does not name `grid run`
    --- FAIL: TestLtReadmeNamesEveryCommand
    FAIL	github.com/gridconsole/grid-plugins/internal/cli

The lane's chain (P-run-jobs, commit 43d26ad5) adds `grid run` to `contract/commands.json`, `internal/cli/cli.go:168` and `internal/cli/run.go`. The test requires every command of `contract/commands.json` to appear as `grid <name>` in README's `## Commands` section. The README did not name `grid run`. The new command is the brief's intent, so the pinned doc was the stale side. The test is unchanged.

## Fix
`README.md`: one bullet in `## Commands` naming `grid run`, with the one-line description from `internal/cli/cli.go:168`. Commit 4e2c382c.

| Row | Status | Proof |
|---|---|---|
| 1. `go test ./internal/cli/ -run 'TestLtReadmeNamesEveryCommand' -count=1 -v` | HOLDS | Failed before the README change (lines above); after it `--- PASS: TestLtReadmeNamesEveryCommand`, `ok ... internal/cli`, exit 0 |
| 2. Every acceptance row of the P-run-jobs-a7 brief, on the merged head | HOLDS | The brief is a docs lane; its rows are in `docs/lanes/P-run-jobs-a7.md`. Re-checked on the merged head: section "Addendum 7" at `docs/research/run-jobs-launcher.md:458`; `tail -n 2` of the doc ends `DONE`; `grep -n -E '/srv/\|/home/\|@gmail\|prod-eu'` on the doc: no match; this lane touches no file the research doc depends on. The remaining rows of that table are unchanged and not affected by a README bullet |
| 3. Walk tests: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | ok, exit 0 |
| 3. Walk tests: `go test -count=1 ./internal/job/lint/` | HOLDS | ok, exit 0 |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output) |
| 4. `make cross` | HOLDS | exit 0: vet and test compile for darwin/arm64 and windows/amd64, all packages ok |
| Tests of the changed packages and importers | HOLDS | Importer list from `go list`: `cmd/grid`, `internal/cli`, `internal/runner`, `lib`. `go test -count=1 ./internal/cli/... ./internal/runner/... ./cmd/grid/... ./lib/...`: cli ok (83s), lib ok, runner and cmd/grid have no test files; exit 0 |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main`: merge made, exit 0; all runs above are on the merged head |
| New tests fail on base and pass on the lane | not run, not asked | no test added; the existing test is the proof (failure quoted above, pass after) |
| Test weakened, skipped or deleted | HOLDS | none; `git diff origin/main -- internal/cli/lt_readme_groups_test.go` is empty |
| Guard, gate, cap, hold or blocking hook on agents | HOLDS | none added; one README line |
| Windows and macOS builds | HOLDS | `make cross`, above |

## What changed
- `README.md`: one bullet naming `grid run`.
- `docs/lanes/P-run-jobs-a7-fix1.md`: this table.

## NOT VERIFIED on a live screen
Nothing here has a screen.

## Open items
`make webui-test`, the app-repo walk tests and the guest walk tables are app-repo checks and do not apply to the plugin. The whole-module test and the cross-repo set run at landing.

DONE
