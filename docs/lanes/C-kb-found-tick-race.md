# C-kb-found-tick-race

Lane C-kb-found-tick-race, repo plugin, stacked on lane-C-kb-orchestrator-security-found (merged into the branch, 94db86e9). One new test file: `internal/orchestrator/kb_found_race_test.go`. No production file changed, no guard, gate, cap or hold added.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `TestKbFrTwoTicksOneJob` | HOLDS | `go test -count=1 -v -run 'TestKbFr' ./internal/orchestrator/` exit 0, prints `--- PASS: TestKbFrTwoTicksOneJob (0.48s)`. Two Stores on one task folder, both with the fixture's one task tenure (same Writer, lease, fence), each Tick in its own goroutine parked on one channel that is closed once after both are parked (`kbFrTickTogether`, kb_found_race_test.go:39). No sleep, no timeout. The Feed holds exactly `[J-1 J-2]`, one orchestrator Job names the found id, and the job records on the Feed are `[J-1 J-2]`. |
| 2. `TestKbFrTwentyRounds` | HOLDS | Same command, prints `--- PASS: TestKbFrTwentyRounds (11.02s)`; 20 rounds, each on a fresh `t.TempDir()` folder, one Job each. |
| 3. `git diff main --stat` names only the new file | DIFFERS as written | `git diff main --stat` lists 5 files: `docs/lanes/C-kb-orchestrator-security-found.md`, `internal/orchestrator/found_security.go`, `internal/orchestrator/kb_found_race_test.go`, `internal/orchestrator/kb_security_found_test.go`, `internal/orchestrator/tick.go` (514 insertions). Local `main` is 21f02736, before the parent lane landed; the parent lane is merged into this branch as the launcher said. Against the merged parent the diff is the one file: `git diff 94db86e9 HEAD --stat` prints `internal/orchestrator/kb_found_race_test.go | 134 +++, 1 file changed`. `git log --oneline origin/main..HEAD` is 3 commits: the merge 94db86e9 and two test commits. |
| 3. `make lint` exit 0 | HOLDS | `make lint && echo LINT_EXIT=0` printed `LINT_EXIT=0` (go vet ./... and gofmt -l). |
| 3. `make cross` exit 0 | HOLDS | `make lint && echo LINT_EXIT=0 && make cross && echo CROSS_EXIT=0` exit 0: darwin/arm64 and windows/amd64 vet and test compile. |
| New tests fail on base | HOLDS, with a note | See "Base runs". |
| Tests of changed package and importers | HOLDS | Changed package `internal/orchestrator`; `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep -E 'grid-plugins/internal/orchestrator( \|$)'` lists importers `internal/cli`, `internal/mcp`, `internal/migrate`. `go test -count=1 ./internal/orchestrator/ ./internal/cli/ ./internal/mcp/ ./internal/migrate/` exit 0: `ok internal/orchestrator 319.515s`, `ok internal/cli 127.541s`, `ok internal/mcp 1.275s`, `ok internal/migrate 24.292s`. A second run of orchestrator, mcp, migrate also exit 0 (`ok internal/orchestrator 193.626s`). |
| Walk tests (plugin runbook) | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0: `--- PASS: TestHsIpWalksEveryCommand (0.00s)` (walked 39 commands, 39 table entries), `--- PASS: TestIlGuestSiteList (0.09s)`, `ok internal/cli 0.312s`. `go test -count=1 ./internal/job/lint/` exit 0: `ok internal/job/lint 0.452s`. |
| Tests named *RealPlugin* | NOT NEEDED | The brief names none; the test file has none. Nothing skipped. |
| No guard, gate, cap, hold or hook on agents | HOLDS | Only `kb_found_race_test.go` is new; `git diff 94db86e9 HEAD --stat` shows no other file. |
| One-sentence commit messages, no attribution | HOLDS | `git log --format=%B 94db86e9..HEAD` carries no Co-Authored-By or session line; the repo and the new file hold no personal path, host or email. |
| No existing test edited or deleted | HOLDS | The diff above adds one file and touches no other. |

## Base runs

The launcher said the parent lane had not landed. At the time of the run `origin/main` was b561251f and already holds it (`git merge-base --is-ancestor origin/lane-C-kb-orchestrator-security-found origin/main` printed ANCESTOR), so the tests pass on current `origin/main` (`--- PASS` for both). That is right for a test of a behaviour that exists. Two throwaway worktrees (removed afterwards, `git worktree list` shows only the lane, `git status --short` clean) show the tests do fail:

1. Lock removed. On a scratch copy of `origin/main` with `Store.locked` changed to `return s.Feed.WithLease(s.Writer, nil, fn)` (no `.orchestrator.lock`), both fail:

```
--- FAIL: TestKbFrTwoTicksOneJob (0.28s)
    kb_found_race_test.go:127: round 1: Tick 1: Job J-2 already exists (post 01M761BZ13Q6JHFWC7A6YABK82); revisions need a new Job
--- FAIL: TestKbFrTwentyRounds (0.19s)
    kb_found_race_test.go:132: round 1: Tick 1: Job J-2 already exists (post 01M761BZ8W2JPYSCM9NS47XYGP); revisions need a new Job
```

(Both Ticks pick J-2, the Feed refuses the second post, so the test goes red on the race and not on the feature.) The edit was made only in the scratch tree, never in this lane.

2. Before the pass existed. On `7a9a5f02^` (dc301fce, no `found_security.go`) with the new file copied in:

```
--- FAIL: TestKbFrTwoTicksOneJob (0.24s)
    kb_found_race_test.go:127: round 1: orchestrator Jobs naming found 01M761B5G88AQRH2PMBKCSZG2S = [], want [J-2] (Feed holds [J-1])
--- FAIL: TestKbFrTwentyRounds (0.80s)
    kb_found_race_test.go:132: round 1: orchestrator Jobs naming found 01M761B5Q8CA5P7QMVE426G5RN = [], want [J-2] (Feed holds [J-1])
FAIL
```

## Why two goroutines stand for two processes

Each Store opens its own descriptor on `.orchestrator.lock` and flock contends per open file description, so two Stores in one process take the same path two processes take. The tests count the lock calls that found the lock held (`fslock.OnBlockForTest`, kb_found_race_test.go:66) and log the count: every round logged `waited 1 time(s)`, so one Tick really waited on the other. The count is logged, not asserted, because overlap is the scheduler's to say.

## What changed

- `internal/orchestrator/kb_found_race_test.go` (new): `TestKbFrTwoTicksOneJob`, `TestKbFrTwentyRounds`, helpers `kbFrSecondStore`, `kbFrFound`, `kbFrTickTogether`, `kbFrBlocked`, `kbFrJobs`, `kbFrRace`. It uses only the package fixture (`setup`, `f.job`, `f.post`) and `Load`.
- `git fetch origin` in the clone moved `origin/main` to b561251f before I made the bundle; the bundle range `origin/main..lane-C-kb-found-tick-race` is the 3 commits above plus this report.

## NOT VERIFIED on a live screen

None: no UI.

Open items: row 3's literal `git diff main --stat` differs (see the row); the lane is one file against its stacked parent. The build used about 70 tool calls against the budget of 60: the 20-round test, the cross build (over 10 minutes) and the cli tests ran long, and the two base runs and the report checks were extra.

Verdict: DONE
