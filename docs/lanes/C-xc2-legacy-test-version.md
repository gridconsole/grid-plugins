# C-xc2-legacy-test-version: the legacy case of the task shape test on the new rule

Repair of C-xc-legacy-home-no-stamp, which ended NOT DONE on row N1. Verdict: DONE.

## What changed

- `git merge origin/main` merged cleanly: no conflicts, no file resolved by hand.
- `internal/cli/task_shape_test.go`: in `TestTaskCommandsMigratedAndLegacyShape`, the legacy case (no VERSION) writes VERSION `"1\n"` just before `grid task new`. Every check on the legacy-shape task (show, rename keeps `id: T-01` / `ulid:`, new task in migrated shape, list) is unchanged, and show and rename still run on the unversioned home.
- `docs/lanes/C-xc-legacy-home-no-stamp.md`: the NOT DONE rows (N1, the verdict and the last line) are rewritten as CLOSED BY C-xc2-legacy-test-version.

## The failure, before the edit (after the merge)

`go test -count=1 ./internal/cli -run TestTaskCommandsMigratedAndLegacyShape`, exit 1:

```
--- FAIL: TestTaskCommandsMigratedAndLegacyShape (0.05s)
    --- FAIL: TestTaskCommandsMigratedAndLegacyShape/legacy (0.05s)
        task_shape_test.go:53: new: 1 grid task new: This data is format 0; this grid reads 1. Run: grid migrate
```

## Proof table

| Row | Result | Proof |
|---|---|---|
| This brief: `go test -count=1 ./internal/cli -run TestTaskCommandsMigratedAndLegacyShape` | HOLDS | Failing before, passing after the edit, as part of the next row. |
| This brief: `go test -count=1 ./internal/cli/ ./internal/gridhome/` | HOLDS | `ok internal/cli 102.195s`, `ok internal/gridhome`, exit 0. |
| Keep every check of the legacy case; no weakened row, no deleted test, no skip | HOLDS | The diff of `task_shape_test.go` is 8 added lines and nothing removed. |
| Old rows 1-4 (TestXcFreshHomeStamped, TestXcLegacyHomeNotStamped, TestXcAppFilesStillFresh, TestXcVersionedHomesUnchanged, TestXcLegacyRule) | HOLDS | `go test -count=1 -v -run TestXc ./internal/gridhome/ ./internal/localdata/ ./internal/settings/ ./internal/task/`: every `--- PASS`, all four `ok`. |
| Old row 5a: five packages | HOLDS | gridhome, localdata, settings, task `ok` in that run; migrate `ok 21.973s` in the importer run. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok internal/cli 0.264s`; `go test -count=1 ./internal/job/lint/`: `ok internal/job/lint 0.139s`. |
| Tests of importers | HOLDS | `go list` importers of localdata, settings, task or gridhome: account, agent, brain, braincmd, cli, doctor, gitx, guard, hook, job, job/jobcmd, job/jobtest, land, localdata, migrate, ports, project, projectmd, projects, proof, routine, settings, signal, task, updates, usage, where, worker, workflow, lib, plus gridhome; golden, mcp and orchestrator were run too. `go test -count=1` on all: every package `ok` (jobtest and updates have no test files), exit 0. |
| make lint | HOLDS | `make lint` runs `go vet ./...`, no output, exit 0. |
| Windows and macOS builds | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `darwin/arm64 test compile`, `windows/amd64 vet`, `windows/amd64 test compile`. |
| Touches | HOLDS | `git diff origin/main --stat`: only the Touches of C-xc-legacy-home-no-stamp, `internal/cli/task_shape_test.go`, the `setup` fixture in `internal/guard/guard_test.go` (3 lines, named in the old report) and the two lane reports. |
| Commits | HOLDS | One sentence each, no attribution lines, no personal paths or hosts. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Only a test file and reports changed here. |
| RealPlugin tests, UI rows, mismatch proof | N/A | No row names them. |
| Whole-module test, cross-repo set | Left to the orchestrator | Run at landing. |

## NOT VERIFIED on a live screen

Nothing here is UI. No real `grid` binary was run against a real old home.

DONE
