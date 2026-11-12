# C-ch-closed-task-ends-grants: a stopped or closed task ends the grants of the Jobs it cancels

Base: plugin main 23867517 (lane branch `lane-C-ch-closed-task-ends-grants`).

Base check: on 23867517 `git grep -n "taCancelEndGrants(" -- internal/orchestrator` showed only
`ta_cancel_grant.go:27` (definition) and `tick.go:106` (inside Tick). `pgClosedCancel`
(`pg_closed_task.go:126-162`) cancelled each open non-curator Job (`s.control(...)` at :149) and never ended a grant.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 `TestChGrantClosedEndsRunningGrant` red on base, green on lane | HOLDS | Base (pg_closed_task.go stashed): `ch_closed_ends_grants_test.go:112: grant of J-1 still names worker@01ARZ3NDEKTSV4RRFFQ69G5FAW (fence 3), want no holder` / `--- FAIL: TestChGrantClosedEndsRunningGrant`. Lane: `go test -count=1 -v -run 'TestChGrant' ./internal/orchestrator/` exit 0, `--- PASS: TestChGrantClosedEndsRunningGrant`. Holder nil, Fence 3 kept, one Job takeovers line `worker@<task>.3.jsonl`. |
| 2 `TestChGrantClosedLaterPassesAddNoTakeover` | HOLDS | Lane: `--- PASS: TestChGrantClosedLaterPassesAddNoTakeover`. On base it also fails (`:120: grant of J-1 still names worker@... (fence 3)`) because the first closed pass leaves the grant. |
| 3 `TestChGrantClosedNeverGrantedSkipped` | HOLDS | Lane: `--- PASS: TestChGrantClosedNeverGrantedSkipped` (J-1 cancelled, no lease/J-1.json, no Job takeovers line, `s.skipped` empty, no diagnostic). Green on base too: a never-granted Job was never affected. |
| 4 `TestChGrantClosedKeepsCuratorLease` | HOLDS | Lane: `--- PASS: TestChGrantClosedKeepsCuratorLease`. Green on base too, as the brief says: that path reaches Tick, which calls taCancelEndGrants. J-1 holder nil with one Job takeovers line, J-2 assigned, J-2 lease file byte-identical after a second pass. |
| 5 existing closed and cancel tests | HOLDS | `go test -count=1 -v -run 'TestPgClosed\|TestUsClosed\|TestTaCancel' ./internal/orchestrator/` exit 0, 17 PASS lines, no FAIL. |
| 6 `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 6 `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| 6 `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok internal/cli 0.100s`. |
| 6 `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok internal/job/lint 0.175s`. |
| 6 `go test -count=1 ./internal/orchestrator/ ./internal/lease/` | HOLDS | both `ok` (orchestrator 178.240s, lease 0.516s). |
| 6 `git diff main --stat` | HOLDS | only `internal/orchestrator/pg_closed_task.go`, the new test file, and this lane doc. |
| Importers of the changed package | HOLDS | `go list` importers of internal/orchestrator: internal/cli, internal/mcp, internal/migrate (plus orchestrator itself). mcp ok, migrate ok, cli ok on rerun (see note). |
| Walk tests (plugin) | HOLDS | `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` and `internal/job/lint` as above, run after `git fetch origin && git merge origin/main` ("Already up to date"). |
| Windows and macOS builds | HOLDS | `make cross` exit 0. |
| `*RealPlugin*` tests | not run, the brief names none (orchestrator unit tests only). |
| App-repo walk tests (webui, grid-server, guest walk tables) | not applicable, plugin repo. |
| No guard, gate, cap, hold on agents | HOLDS | The change ends a grant the task already cancelled; nothing is refused, held or counted; an error is returned as a control error (SkipTickFailed). |

Note on a flake: the first `go test ./internal/cli/` run, in parallel with the orchestrator package, failed
`TestBwEnvCliAddOnce` ("Cannot read the format version in ~/.grid/VERSION"). It passes alone and in a full rerun
(`go test -count=1 ./internal/cli/` exit 0, 31.849s). The lane touches nothing in cli.

## What changed

- `internal/orchestrator/pg_closed_task.go` `pgClosedCancel`: after the cancel loop and before the curator step,
  inside the same lock and task lease, `Load(s.Feed)` again and `s.taCancelEndGrants(...)`. An error is returned from
  the locked func (pgClosedTask turns it into SkipTickFailed). The curator step keeps the pre-cancel snapshot (open
  curator count is unchanged by cancels).
- File comment rewritten: the closed pass ends each cancelled Job's grant through taCancelEndGrants (EndJob,
  task.go:317); stale task.go line numbers fixed (GrantJob :360, EndJob :317, end :456).
- `internal/orchestrator/ch_closed_ends_grants_test.go`: four tests, helpers `chGrant*`. The takeovers helper
  filters out task-lease lines (the owner's release and the stream's own release also land in takeovers.jsonl), so
  "exactly one line" in the tests means exactly one Job-grant line.

## NOT VERIFIED

- That the app worker stops the child once the lease no longer names it (out of scope; C-ta left it so too).
- Nothing on a live screen; no UI in this lane.
