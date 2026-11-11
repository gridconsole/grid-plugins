# C-kb-orchestrator-security-found: a security `found` on a Job makes the tick post one orchestrator Job for it, once

Repo: plugin. Branch lane-C-kb-orchestrator-security-found, from main.

## Proof table

| Row | Result | Command and output |
| --- | --- | --- |
| 1. `TestKbSfOneJobPerSecurityFound` | HOLDS | `go test -count=1 -v -run 'TestKbSf' ./internal/orchestrator/` exit 0, prints `--- PASS: TestKbSfOneJobPerSecurityFound`: J-1 and one security found, one tick, Jobs [J-1 J-2], role `orchestrator`, A1 holds the found record id. On base (the tick call removed, as the base tick is): `kb_security_found_test.go:74: after one tick: Jobs on the Feed = [J-1], want [J-1 J-2]` / `--- FAIL: TestKbSfOneJobPerSecurityFound` |
| 2. `TestKbSfSecondTickPostsNothing` | HOLDS | same command, `--- PASS: TestKbSfSecondTickPostsNothing`: three more ticks and a new Store on the same folder (`kbSfRestart`) leave [J-1 J-2]; a second security found on J-1 gives J-3 whose A1 holds the second id and not the first. On base: `kb_security_found_test.go:141: after the first tick: Jobs on the Feed = [J-1], want [J-1 J-2]` / `--- FAIL` |
| 3. `TestKbSfOnlySecurity` | HOLDS | same command, `--- PASS: TestKbSfOnlySecurity`: a `bug` found, a security found for J-9 and a found with body `{"ref": ...}` (a hand-built snapshot that lists it as a wake, `f.s.kbSecurityFound(snap)`) post nothing; a security found on J-1 in the same test then posts exactly J-2. On base: `kb_security_found_test.go:194: after a security found on J-1: Jobs on the Feed = [J-1], want [J-1 J-2]` / `--- FAIL` |
| 4. `TestKbSfReplanStillPosts` | HOLDS | same command, `--- PASS: TestKbSfReplanStillPosts`: J-1, J-2 after J-1, a security found, tick posts J-3 (discovered_from null); J-1 closed Rejected (`guRjReject`); the next tick posts J-4, role orchestrator, discovered_from J-1, title `Re-plan after J-1 was rejected: J-2 wait on it`; one more tick posts nothing. On base: `kb_security_found_test.go:206: after the security found: Jobs on the Feed = [J-1 J-2], want [J-1 J-2 J-3]` / `--- FAIL`. `go test -count=1 -run 'TestGu\|TestGk' ./internal/orchestrator/` exit 0 (`ok ... 64.541s`) |
| 5a. `git diff main --stat` | HOLDS for the code | the three commits of code name only `internal/orchestrator/found_security.go` (107 lines), `internal/orchestrator/kb_security_found_test.go` (226), `internal/orchestrator/tick.go` (7). The fourth path, `docs/lanes/C-kb-orchestrator-security-found.md`, is this report, which the builder runbook requires and which is its own last commit. |
| 5b. `make lint` | HOLDS | exit 0 (`go vet ./...`, no gofmt output) |
| 5c. `make cross` | HOLDS | ran as `make cross 2>&1 \| tail -8` (the pipe hides make's own exit code); the last lines are `ok` for the final packages (`internal/workflow`, `lib`) and no `make: *** ... Error` line, which make prints last when a target fails, so the darwin/arm64 and windows/amd64 vet and test compile loop ended cleanly |
| Importers of the changed package | HOLDS | `go list` shows the packages importing `internal/orchestrator`: `internal/cli`, `internal/mcp`, `internal/migrate`. `go test -count=1 ./internal/orchestrator/ ./internal/mcp/ ./internal/migrate/ ./internal/cli/` exit 0: `ok orchestrator 460.066s`, `ok mcp 1.886s`, `ok migrate 79.736s`, `ok cli 257.100s` |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok ... cli 7.368s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok ... lint 6.167s`). The app walk tests (`make webui-test` and the rest) are for the app repo, not this one. |
| `*RealPlugin*` tests | not needed | no row names one; nothing skipped is claimed as passing |
| New tests fail on base, pass on lane | HOLDS | the four failing lines above. `TestKbSfJobBodyIsOneLine` tests only the body builder `SecurityFoundJobBody`, which does not exist on base, so on base it fails to compile; it is not one of the four rows. |
| No existing test edited or deleted | HOLDS | `git diff main --stat` shows no existing test file |
| No guard, gate, cap or hold on agents | HOLDS | the pass posts a Job and nothing more; it refuses, cancels and counts nothing. The 120-rune title cut and the 500-byte text cut (`gqCause`) are text cuts, not limits on an agent. |
| No attribution lines, no personal paths, hosts or emails | HOLDS | commit messages are one sentence with no trailer; the files hold none |

## What changed

- `internal/orchestrator/found_security.go` (new): `Store.kbSecurityFound(snap Snapshot) error`, `kbSfPosted`, and the exported `SecurityFoundJobBody(id, found, job, at, text)`. For each record in `snap.State.Wakes` that is a `found` with `body.kind` `security` and `body.job` a Job of the fold, and no orchestrator Job whose A1 begins `Security found record <id> (`, it posts one Job (role `orchestrator`, title `Security finding on <job>: <text on one line, 120 runes>`, one row A1) and reloads the snapshot so the next found picks the next id.
- `internal/orchestrator/tick.go`: one `Load` and one `s.kbSecurityFound(snap)` after `guReplanPersonCancelled`, in the shape of the passes around it. The pass runs inside `s.locked`, so the check and the post are one step across ticks and processes; the check reads the Feed (`Load`), not memory, so a restart posts nothing twice.
- `internal/orchestrator/kb_security_found_test.go` (new): `TestKbSfOneJobPerSecurityFound`, `TestKbSfJobBodyIsOneLine`, `TestKbSfSecondTickPostsNothing`, `TestKbSfOnlySecurity`, `TestKbSfReplanStillPosts`.

Where the found id is held: in A1, the one acceptance row the job-record requires. No schema changed. The other fields were ruled out: `job-record` has `additionalProperties: false`, `links.to` takes only a brain, job or person target (`internal/job/lint/lint.go:328-340`), `read` needs a line range, and `kind` is lowercase and cannot hold a ULID. `discovered_from` is `null` on the security Job: `gkReplanned` (replan.go:69-76) reads any orchestrator Job with `discovered_from` J as the re-plan of J, so a pointer there would stop the re-plan after J is rejected (proved by `TestKbSfReplanStillPosts`). The check keys on role orchestrator plus the A1 prefix with the found id, never on `discovered_from`.

## Open items

- A security found on a Job already Accepted, Cancelled or Rejected still gets its Job (the brief names only "a Job of the fold"). A closed security Job does not post again: the record on the Feed says it was posted once.
- A found whose body is still a held `ref` (body file not there yet) is not counted by the fold, so it is not a wake; it gets its Job on the tick after the body arrives. The `{"ref": ...}` case in `TestKbSfOnlySecurity` is a hand-built snapshot, because the reader expands a present ref and holds an absent one.
- The Job's `risk` is left empty (as in the re-plan Jobs), so no review gate is added to it.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. The wake that `events.go:217-223` hands to a running lead is unchanged; a live lead and the new Job together were not run (no claude, no server).
