# P-r23-approve-person: grid approve and grid task status: one person check

Plugin main at 0f360eaa. Findings: U-chk-plugin-rev-cli row 1 and U-chk-plugin-rev-orch row 3. Verdict: row 1 fixed; orch row 3 confirmed and NOT DONE (it needs a guard on agents, which the lane rules forbid; see below).

## Proof table

| Row | Result |
|---|---|
| 1a. cli row 1 confirmed on main | HOLDS, confirmed. `go test -count=1 -run TestR23ApproveNeedsNoGridMember ./internal/cli/` on main (fix stashed): `--- FAIL: TestR23ApproveNeedsNoGridMember`, `r23_approve_person_test.go:45: "": approve exit 3, want 0; stderr "grid approve: approving is for a person of the team: set GRID_MEMBER\n"`. The test runs `grid task status T-01 waiting` first in the same empty environment and it exits 0 (the test checks that first and goes on), then `grid approve T-01 --plan-hash <hash> --json` exits 3. |
| 1b. orch row 3 confirmed on main | HOLDS, confirmed. Scratch run (not committed, removed): `grid task status T-01 waiting --json` exit 0 in an unmarked session (`who {"id":"local","role":"owner"}`), with `GRID_AGENT=orchestrator` (`who {"id":"orchestrator","role":"agent"}`) and with `GRID_ROLE=worker` (`who {"id":"grid","role":"worker"}`). No code refuses an agent there. |
| 2a. cli row 1: fix and test | HOLDS. `approvalWriter` (internal/cli/approve_record.go:22): with no lease epoch, no GRID_MEMBER, no GRID_ACTOR and no worker mark, a person's role (empty, cli, owner, member) writes as `local`, role owner, the person `grid task status` and `grid task close` record in that shell. Test `TestR23ApproveNeedsNoGridMember` (internal/cli/r23_approve_person_test.go): fails on main (line above), passes on the head. It runs three shells (empty env, GRID_ROLE=cli, GRID_ROLE=owner) and asserts exit 0, no GRID_MEMBER on stderr, exactly one answer record authored `{local, owner}` and the plan approve Job answered. |
| 2b. agent environment refused for approve | HOLDS as before, nothing new. `TestR23ApproveRefusesAgentEnvironment` (5 shells: GRID_WORKER_ID, GRID_AGENT, GRID_WORKER, GRID_ROLE=worker, GRID_AGENT with GRID_ROLE=owner, all with no GRID_MEMBER): exit 3, `approving is for a person of the team: set GRID_MEMBER`, no answer, no approve Job. It passes on main and on the head: it pins the refusal that already existed; the lane did not add it. Guest: `TestFvGuestApproveRefused` unchanged and green. |
| 2c. orch row 3: task status refused from an agent | NOT DONE. Confirmed above, not fixed. Refusing an agent on `grid task status` is a new guard on agents, and the lane rules say to stop and report when a brief seems to need one. The code says the same: writeAuthor (internal/cli/task_write.go:46-47) "only records: no command admits or refuses anyone because of it", and docs/lanes/P-person-check.md:18 says `grid person check` "adds no guard, gate or hold on any agent". `agents/orchestrator.md:27,48` tells the orchestrator agent to run `grid task status <task> waiting` and `grid task status <task> running`, so a refusal would break that agent. The row's own fix was the doc: change docs/delivery.md:214-217 to say `grid task status` is agent-callable. I left that paragraph's claim alone (the brief chose the other direction) and added only the approve sentence. The owner decides: (a) the guard, which also needs orchestrator.md:27,48 changed, or (b) the one-sentence doc fix. |
| 3a. go test of changed packages and importers | HOLDS, see "Runs" below. `go list` importers of internal/cli: cmd/grid (no test files), lib. |
| 3b. make lint | HOLDS, exit 0 (`go vet ./...`). |
| 3c. Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok github.com/gridconsole/grid-plugins/internal/cli 0.579s`. `go test -count=1 ./internal/job/lint/`: `ok ... 0.939s`. |
| `git fetch origin && git merge origin/main` | HOLDS: `Already up to date.` The app-repo walk tests (webui-test, TestFvArgvCollects, realplugin guest tables) are for the app repo: not run, not asked. |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| gofmt | HOLDS: `gofmt -l internal/cli` empty. |
| No guard, gate, cap or hold added | HOLDS. The change loosens one refusal for a person's terminal; every caller refused before is still refused. |
| *RealPlugin* tests | None apply. |
| Whole-module test, cross-repo set | not run, not asked (the orchestrator runs them at landing). |

## Runs

- `go test -count=1 -run 'TestApproveRecord|TestR23|TestFv|TestFu2|TestApprove' ./internal/cli/`: `ok` 7.508s (exit 0), after the change to the existing test.
- `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` (full, last run on the final code): FULL_RUN_PENDING

## An existing test changed

`TestApproveRecordCallerWithoutMemberWritesNothing` (internal/cli/approve_record_test.go:334) asserted that an empty environment (no GRID_MEMBER, no lease epoch) is refused with exit 3. That is the exact case the brief reverses ("a person in their own terminal must be able to approve without exporting GRID_MEMBER"; cli row 1; the SessionStart text at internal/hook/hook.go:376 tells every session to approve this way). The assertions are kept; only the environment of the refused case changed, from empty to `GRID_AGENT=worker-1`, so the test still proves a worker's shell with no member writes nothing. The empty-environment case is now covered by `TestR23ApproveNeedsNoGridMember`. It is the only existing test touched.

## What changed

- internal/cli/approve_record.go: `approvalWriter` resolves a person's own terminal (no GRID_MEMBER, GRID_ACTOR or worker mark) as `local`, role owner, instead of refusing. The lease path, the GRID_MEMBER path and the refusal line and exit 3 are unchanged; exit 3 is kept (the row's fix said exit 2) because `TestFvGuestApproveRefused`, docs/delivery.md:214 and `grid person check` all use 3 for a refused person check. The SessionStart text is true as written and was not changed.
- docs/delivery.md: one sentence after the person-only paragraph says a person in their own terminal approves without GRID_MEMBER.
- internal/cli/r23_approve_person_test.go (new), internal/cli/approve_record_test.go (the one test above).

## What this means for agents (read before landing)

After this lane an agent session that carries no GRID_* variable at all (a Claude Code session started by hand, as the orch row's own check was) can run `grid approve` and is recorded as the local owner. Before, only the missing GRID_MEMBER refused it, and nothing in the plugin detects such a session (no parent-process or CLAUDECODE marker is read in internal/cli). A shell with a worker mark or an agent role and no GRID_MEMBER is still refused. A shell with a worker mark and GRID_MEMBER set and a person role can approve, as before (`TestFu2ApproveJobWaitingRoles/agent` pins that GRID_AGENT=1 reaches the Job wait). Closing that gap is the same guard decision as row 2c.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run against the real app or a real terminal session; the test drives `cli.Run` with a stubbed environment and a temp Grid home.
