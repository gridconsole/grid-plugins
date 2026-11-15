# P-r23-approve-person: grid approve and grid task status: one person check

Plugin main at 0f360eaa. Findings: U-chk-plugin-rev-cli row 1 and U-chk-plugin-rev-orch row 3. Verdict: NOT DONE. Row 1 is fixed (a person approves with no GRID_MEMBER). The brief's second half, "an agent environment must be refused for both verbs", is NOT DONE: it needs a guard on agents, which the lane rules forbid, so I stopped on it and report it (row 2c).

Go code is at 78def420; every run below is on that head. The commits after it change this doc only.

## Proof table

| Row | Result |
|---|---|
| 1a. cli row 1 confirmed on main | HOLDS, confirmed. With main's `approve_record.go` put back (`git checkout origin/main -- internal/cli/approve_record.go`, then restored; `git status --short` clean), `go test -count=1 -run TestR23 ./internal/cli/` exit 1: `--- FAIL: TestR23ApproveNeedsNoGridMember`, `r23_approve_person_test.go:47: "": approve exit 3, want 0; stderr "grid approve: approving is for a person of the team: set GRID_MEMBER\n"`. The test runs `grid task status T-01 waiting` first in the same empty environment; it exits 0 (the test checks that and goes on), then `grid approve T-01 --plan-hash <hash> --json` exits 3. The other two R23 tests pass on main. On the head: `go test -count=1 -v -run TestR23 ./internal/cli/` exit 0, `--- PASS` for `TestR23ApproveNeedsNoGridMember`, `TestR23WorkerShellWithoutMemberWritesNothing` and the 5 subtests of `TestR23ApproveRefusesAgentEnvironment`. |
| 1b. orch row 3 confirmed on main | HOLDS, confirmed. Scratch run (not committed, removed): `grid task status T-01 waiting --json` exit 0 in an unmarked session (`who {"id":"local","role":"owner"}`), with `GRID_AGENT=orchestrator` (`who {"id":"orchestrator","role":"agent"}`) and with `GRID_ROLE=worker` (`who {"id":"grid","role":"worker"}`). No code refuses an agent on that verb. |
| 2a. cli row 1: fix and test | HOLDS. `approvalWriter` (internal/cli/approve_record.go:22): with no lease epoch, no GRID_MEMBER, no GRID_ACTOR and no worker mark, a person's role writes as the local person. An empty or `cli` GRID_ROLE records `{local, owner}`, the person `grid task status` and `grid task close` record in that shell; an explicit `GRID_ROLE=member` records `{local, member}`. Test `TestR23ApproveNeedsNoGridMember` (internal/cli/r23_approve_person_test.go) runs three shells (empty env, `GRID_ROLE=cli`, `GRID_ROLE=owner`) and asserts: `task status` exit 0, `approve` exit 0 with no GRID_MEMBER on stderr, exactly one answer record authored `{local, owner}`, the plan approve Job answered, and after the lease holder's pass (`holderPass`) `task.Approved(path)` is true. Fails on main (row 1a), passes on the head. |
| 2b. agent environment refused for approve | PARTLY. Shells with a worker mark (GRID_WORKER_ID, GRID_AGENT, GRID_WORKER) or an agent role and no GRID_MEMBER stay refused, exit 3 with `approving is for a person of the team: set GRID_MEMBER`, nothing written: `TestR23ApproveRefusesAgentEnvironment` (5 shells) and `TestR23WorkerShellWithoutMemberWritesNothing`. Both pass on main too; they pin refusals that already existed, the lane added none. A shell that carries no GRID_* variable at all is not detected as an agent and now approves (see 2c). |
| 2c. agent environment refused for both verbs (brief Do 2, docs/delivery.md:214-217) | NOT DONE. Two parts, one reason. (1) `grid task status` is not refused from an agent (row 1b), and the unmarked-session `grid approve` now succeeds as the local owner where before only the missing GRID_MEMBER refused it. Closing either needs a new check of the caller as an agent, which is a guard on agents; the lane rules say to stop and report when a brief seems to need one. The code says the same: writeAuthor (internal/cli/task_write.go:46-47) "only records: no command admits or refuses anyone because of it", and docs/lanes/P-person-check.md:18 says `grid person check` "adds no guard, gate or hold on any agent". Nothing in internal/cli detects a Claude Code or Codex session (no parent-process or session-marker read). (2) `agents/orchestrator.md:27,48` tells the orchestrator agent to run `grid task status <task> waiting` and `grid task status <task> running`, so a refusal would break that agent. The row's own fix was the doc: change docs/delivery.md:214-217 to say `grid task status` is agent-callable. I left that paragraph's claim about `grid task status` as it is (the brief chose the other direction) and added only the approve sentence. The owner decides: (a) the guard, with orchestrator.md:27,48 and the unmarked session answered, or (b) the one-sentence doc fix. |
| 3a. go test of the changed package and its importers | HOLDS, see Runs. `go list` importers of internal/cli: cmd/grid (no test files), lib. |
| 3b. make lint | HOLDS: `make lint` (`go vet ./...`) exit 0 on 78def420. |
| 3c. Walk tests | HOLDS on 78def420. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok github.com/gridconsole/grid-plugins/internal/cli 0.177s`. `go test -count=1 ./internal/job/lint/`: `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.536s`. |
| `git fetch origin && git merge origin/main` | HOLDS: `Already up to date.` The app-repo walk tests (webui-test, TestFvArgvCollects, the realplugin guest tables) belong to the app repo: not run, not asked. |
| Windows and macOS builds | HOLDS on 78def420: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| gofmt | HOLDS: `gofmt -l internal/cli` empty. |
| Existing test deleted | `TestApproveRecordCallerWithoutMemberWritesNothing` (internal/cli/approve_record_test.go, 35 lines) is deleted; the file differs from main only by that deletion. It asserted that an empty environment (no GRID_MEMBER, no lease epoch) is refused with exit 3. The owner's direction removed that behaviour: the brief says "A person in their own terminal must be able to approve without exporting GRID_MEMBER", cli row 1 says the same, and the SessionStart text (internal/hook/hook.go:376) tells every session to approve in a terminal. The empty-environment case is covered by `TestR23ApproveNeedsNoGridMember`; the worker-shell case, which it also meant, is covered by `TestR23WorkerShellWithoutMemberWritesNothing`, the same body with `GRID_AGENT` set. No other existing test was edited. |
| No guard, gate, cap or hold added | HOLDS. The change loosens one refusal for a person's terminal; it adds no check of any caller. Every shell refused before is refused now except the one that carries no GRID_* variable (2c). |
| No attribution lines, no personal paths | HOLDS: commit messages are one sentence each with no trailer; this doc and the code hold no home path, host or email. |
| *RealPlugin* tests | None apply. |
| Whole-module test, cross-repo set | not run, not asked (the orchestrator runs them at landing). |
| Budget | About 65 tool calls against the brief's "about 50": the full `internal/cli` run takes about four minutes and was run three times, and a review of my own work added an edit round (the deleted test and the holder's-pass assertion). The WIP bundle was written late, not at 60 percent. |

## Runs (Go code at 78def420)

- `go test -count=1 -run 'TestR23|TestApproveRecord' ./internal/cli/`: `ok` 3.583s.
- `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` exit 0: `ok github.com/gridconsole/grid-plugins/internal/cli 129.322s`, `? github.com/gridconsole/grid-plugins/cmd/grid [no test files]`, `ok github.com/gridconsole/grid-plugins/lib 33.098s`.
- `go test -count=1 ./internal/task/ ./internal/hook/ ./internal/orchestrator/ ./internal/feed/` exit 0 (packages `approvalWriter` reads from; none changed): all `ok`.

## What changed

- internal/cli/approve_record.go: `approvalWriter` resolves a person's own terminal (no GRID_MEMBER, GRID_ACTOR or worker mark) as `local` instead of refusing. The lease path, the GRID_MEMBER path, the refusal line and exit 3 are unchanged; exit 3 is kept (the row's fix said exit 2) because `TestFvGuestApproveRefused`, docs/delivery.md:214 and `grid person check` use 3 for a refused person check. The SessionStart text is true as written and was not changed.
- docs/delivery.md: one sentence after the person-only paragraph says a person in their own terminal approves without GRID_MEMBER.
- internal/cli/r23_approve_person_test.go (new: three tests), internal/cli/approve_record_test.go (one test deleted).

## What this means for agents (read before landing)

After this lane an agent session that carries no GRID_* variable (a Claude Code session started by hand, as the orch row's own check was) can run `grid approve` and is recorded as the local owner; before, only the missing GRID_MEMBER refused it. A shell with a worker mark or an agent role and no GRID_MEMBER is still refused. A shell with a worker mark, GRID_MEMBER set and a person role can approve, as before (`TestFu2ApproveJobWaitingRoles/agent` pins that GRID_AGENT=1 reaches the Job wait). That gap is row 2c.

## NOT VERIFIED

- Nothing here is a UI row. Not run against the real app or a real terminal session: the tests drive `cli.Run` with a stubbed environment and a temp Grid home.
- `ship_land.go:38`, the second `approvalWriter` caller (the ship landing path), was not read; it gets the same writer and the same change, and its tests are in the `internal/cli` run above.
- The `grid task close` claim in cli row 1 (works in the same shell) was not re-run; `grid task status` was.
