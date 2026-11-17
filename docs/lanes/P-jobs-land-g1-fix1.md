# P-jobs-land-g1-fix1: fix round for P-jobs-land-g1

Cause. P-jobs-land-g1 carries lane-P-run-jobs-a7, which adds the command
`run` to `contract/commands.json` (commit 43d26ad5, "grid run"). The README
`## Commands` section did not name `grid run`, so
`TestLtReadmeNamesEveryCommand` failed once main was merged (main alone
passes because main has no `run` command; the lane alone failed after the merge
because the test reads the README the lane did not update).

Decision. The test is right and the lane's change is right: `grid run` is a
real, built command (`"built": true`), and the test guards that the README names
every command. The pinned file is README.md, a hand-written list with no
generator, so it is updated. The test is unchanged.

Change. One line in README.md `## Commands`: `grid run` added to the
`grid task`, `grid plan`, `grid job`, `grid lease` group. Nothing else.

Failure on the merged head before the fix:

    lt_readme_groups_test.go:47: README.md ## Commands does not name `grid run`
    --- FAIL: TestLtReadmeNamesEveryCommand (0.00s)
    FAIL	github.com/gridconsole/grid-plugins/internal/cli	0.080s

## Proof table

| Row | Result |
|---|---|
| 1. `go test ./internal/cli/ -run 'TestLtReadmeNamesEveryCommand' -count=1 -v` | HOLDS, exit 0: `--- PASS: TestLtReadmeNamesEveryCommand (0.00s)` / `ok  github.com/gridconsole/grid-plugins/internal/cli  0.009s` |
| 2. Every acceptance row of /srv/gridwork/briefs/P-jobs-land-g1.md on the merged head | HOLDS. The brief's rows are doc rows (V26/V27, V23, lane 13, V11.e, V12.f, V12.c, commits); the fix touches only README.md, so the closing lines in docs/research/run-jobs-launcher.md and docs/research/judge-land.md are unchanged by this round. Per-row file:line proof is in docs/lanes/P-jobs-land-g1.md (that table stands, line numbers unchanged: `git diff origin/main..HEAD` for the two docs is not touched by this lane). Last line of both docs is still `DONE`. |
| 3. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestLtReadmeNamesEveryCommand\|TestGbDocs'` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli  0.521s` |
| 3. `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0: `ok  github.com/gridconsole/grid-plugins/internal/job/lint  0.405s` |
| 3. `make webui-test`, `TestFvArgvCollects`, `TestEveryWiredMount`, slices Imports | not run, not asked (app-repo walk tests; this is the plugin repo) |
| 4. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output) |
| 4. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64: vet and test compile, no FAIL) |
| Tests of changed packages and importers | the only changed file is README.md, read by `internal/cli` tests; no Go package changed, so no importer set; `internal/cli` run above |
| New test | none added: the existing test already fails on the merged head without the fix and passes with it (failing lines above) |
| `git fetch origin && git merge origin/main` | done at start of the round; Merge made by the ort strategy, no conflict |

## Notes

- docs/lanes/P-jobs-land-g1.md says "Docs only. No code changed". The lane
  does carry the code of `grid run` through lane-P-run-jobs-a7 (43d26ad5);
  that is what the failing test caught. Line 3 of that doc is corrected in this
  round to say so.
- NOT VERIFIED on a live screen: nothing here is UI.
