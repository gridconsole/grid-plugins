# C-jw-keeper-held-recheck: the keeper Job's check under the lock counts a held id

Repo: plugin. Branch lane-C-jw-keeper-held-recheck, merged with main ("Already up to date").

## Base check

On the base, `internal/cli/deliver_keeper_job.go:137` read
`if err == nil && (openCuratorJob(s) != "" || s.State.Jobs[id] != nil) {`, with no
`JobIDs`, so the brief applies (not stopped).

## What changed

- `internal/cli/deliver_keeper_job.go:141`: the check run under `.orchestrator.lock` in
  `WriteUnless` also returns true when `slices.Contains(s.JobIDs, id)`, so the
  `for try` loop picks the id again (`nextJobID` already counts `JobIDs`). Import of
  `slices` added.
- `internal/cli/jw_keeper_held_recheck_test.go` (new): helpers `jwKh*`, tests
  `TestJwKhHeldIDPlantedBeforeTheLockIsSkipped` and
  `TestJwKhNothingPlantedTakesTheFirstID`. The setup is copied from
  `iu_keeper_held_id_test.go`; that file is not edited.

Which event the plant waits on: deliver takes no lock before the pick (`Load` takes
none; the delivery write is finished and holds none), so the first
`fslock.OnBlockForTest` event on `<task>/feed/.orchestrator.lock` is the wait inside
`WriteUnless` -> `Store.exclusive` (`internal/orchestrator/store.go:141`), which follows
the pick at `deliver_keeper_job.go:125`. The base run below proves it: the curator
record took the planted id, which it could only do if the plant came after the pick.

## Base failure (the fix removed, test file kept)

`go test -count=1 -run 'TestJwKh' -v ./internal/cli/` on the base, exit 1:

```
--- FAIL: TestJwKhHeldIDPlantedBeforeTheLockIsSkipped (0.17s)
 held record still held: want one curator Job J-2 in State, got ["J-1"]; job records (id role verdict): ["J-1 builder held" "J-1 curator counted"]; stdout "T-01 delivered; waiting for the person\n2 brain proposals wait for the curator (Job J-1)\n"
 held record still held: job records = ["J-1 builder held" "J-1 curator counted"], want ["J-1 builder held" "J-2 curator counted"]
 held record released: want one curator Job J-2 in State, got []; job records (id role verdict): ["J-1 builder counted" "J-1 curator ignored"]
 held record released: State holds J-2 = <nil>, want the curator Job
 held record released: job records = ["J-1 builder counted" "J-1 curator ignored"], want ["J-1 builder counted" "J-2 curator counted"]
--- PASS: TestJwKhNothingPlantedTakesTheFirstID (0.02s)
```

The brief says the curator record "is ignored". That holds once the held record is
released (its after target arrives): the fold reads the planted record first, counts
it, and ignores the curator record (`J-1 curator ignored`, no curator Job in State, the
keeper never runs). While the planted record is still held the fold counts the curator
record on the same id (`J-1 curator counted`); the test checks both moments, and
releases the held record by posting a record whose id is the planted record's after
target.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. Test holds `.orchestrator.lock` on its own fd, runs deliver in a goroutine, waits on the `fslock.OnBlockForTest` event, plants a held J-n through `feed.Store.Post`, unlocks; deliver exits 0, curator is J-(n+1) and in State; base lands on J-n and is ignored | HOLDS | `go test -count=1 -v -run 'TestJwKh' ./internal/cli/` exit 0: `--- PASS: TestJwKhHeldIDPlantedBeforeTheLockIsSkipped (0.17s)`; test at `jw_keeper_held_recheck_test.go:139`, helper `jwKhDeliverHolding` at :85; base failure pasted above (exit 1) |
| 2. Nothing planted: curator takes J-n as before | HOLDS | `--- PASS: TestJwKhNothingPlantedTakesTheFirstID (0.04s)`, `jw_keeper_held_recheck_test.go:219`, same lock-held route, no plant, curator is J-1 and the only job record. It also passes on the base (it is a control, not a failing test); `TestDeliverPostsOneCuratorJobForWaitingProposals` covers J-1 without the lock route |
| 3a. `go test -count=1 -run 'TestJwKh\|TestIuRk\|Keeper\|Curator' ./internal/cli/` 0 fail with `--- PASS: TestJwKh...` | HOLDS | exit 0, `ok github.com/gridconsole/grid-plugins/internal/cli 0.940s` with -v: `--- PASS: TestIuRkKeeperSkipsHeldID (0.03s)`, `--- PASS: TestJwKhHeldIDPlantedBeforeTheLockIsSkipped (0.11s)`, `--- PASS: TestJwKhNothingPlantedTakesTheFirstID (0.05s)` |
| 3b. `make lint` exit 0 | HOLDS | `make lint && echo LINT_EXIT_0` printed `LINT_EXIT_0` |
| 3c. `make cross` exit 0 | HOLDS | `make cross && echo CROSS_EXIT_0` printed `CROSS_EXIT_0` (darwin/arm64 and windows/amd64 vet and test compile, every package ok) |
| 3d. `git diff main --stat` names only the Touches files | HOLDS, with the runbook report | `internal/cli/deliver_keeper_job.go` (6 lines) and `internal/cli/jw_keeper_held_recheck_test.go` (239 lines, new); the lane report `docs/lanes/C-jw-keeper-held-recheck.md` the runbook asks for is the only other file |
| R1. Small commits, one-sentence messages, no attribution lines | HOLDS | `git log --oneline -4`: c6f1a593 (test), d8942b09 (fix), then the report commit; no Co-Authored-By or session line |
| R2. New tests fail on base, pass on the lane | HOLDS | base failure pasted above (exit 1); lane pass at rows 1 and 3a. Row 2's test is a control and passes on both |
| R3. No existing test edited or deleted | HOLDS | `git diff main --stat` shows no other test file; `iu_keeper_held_id_test.go` untouched |
| R4. No guard, gate, cap, hold or blocking hook on agents | HOLDS | the change adds one id check under a lock that already exists; it only makes the post pick another id |
| R5. Tests of changed package and of every importer | HOLDS | importers from `go list` (imports, test imports, external test imports): cmd/grid, contract/formats, internal/account, internal/braincmd, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1 ./internal/cli/ <those>` exit 0: cli 137.173s ok, contract/formats ok, account ok, braincmd ok, golden ok, jobgraph ok, mcp ok, packaging ok, question ok, settings ok, lib ok, cmd/grid no test files |
| R6. Lint, Windows and macOS builds | HOLDS | rows 3b and 3c |
| R7. UI rows | not applicable | no UI change |
| R8. *RealPlugin* tests | not applicable | no row names one; none run |
| R9. No sleep, no elapsed-time assertion; wait on a channel fed by an event; 30 s timer only on the failure path; hook test has no `t.Parallel`, restores with `t.Cleanup` | HOLDS | `jw_keeper_held_recheck_test.go:107` `t.Cleanup(fslock.OnBlockForTest(...))`, the hook compares the path and sends without blocking; the two `time.After(jwKhDeadline)` (:122, :130) are failure paths; no `t.Parallel`, no `Sleep` |
| R10. `git merge main` before the last proof run | HOLDS | `git merge main`: "Already up to date." |
| R11. `git status --short` clean before commit | HOLDS | clean after the last commit |

## NOT VERIFIED on a live screen

Nothing here is a screen. Not run: the Windows and macOS test binaries (they compile and
vet through `make cross`, but `-exec /bin/true` does not run them), so the
lock-contention behaviour of `fslock` on those systems is not exercised by this test.
Whole-module test and the cross-repo set are the orchestrator's at landing.
