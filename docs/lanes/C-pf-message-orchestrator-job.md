# C-pf-message-orchestrator-job: NOT DONE (STUCK)

The brief cannot be met on this base. Two of its requirements contradict each other, and the schema that decides is outside Touches. I stopped at call ~15 and did not build around it.

## The conflict

Row 1 asks for an orchestrator Job with `discovered_from` = the post id, and a body that passes `contract.Validate("job-record", ...)`.

`contract/schemas/job-record.json:349-355` admits `discovered_from` only as null or a string matching `^J-[1-9][0-9]*(\.[1-9][0-9]*)*$`. A Feed post id never matches.

Run as seen (the request is `PfMsgJobBody(next, postID, text)`, `discovered_from` = post id, posted by `pfMsgReplan` in the tick):

```
go test -count=1 ./internal/orchestrator/ -run 'TestPfMsg' -v
    pf_message_replan_test.go:60: $.body: oneOf matched 0 branches: $.body.discovered_from: anyOf matched 0 branches: $.body.discovered_from: expected null
--- FAIL: TestPfMsgOwnerMessageGetsOneJob
    pf_message_replan_test.go:125: ... same message
--- FAIL: TestPfMsgTwoMessagesTwoJobs
    pf_message_replan_test.go:142: ... same message
--- FAIL: TestPfMsgJobSetsOffNoOtherReplan
```

`Feed.Post` accepts the record. Only `contract.Validate` refuses it. A Job on the Feed that fails its own schema is a risk for `grid job lint` (`internal/job/lint/lint.go` reads `discovered_from`) and for the `cmd-feed-state` schema.

## Proof table

| Row | Result |
|---|---|
| Base check: no replan on 00413d82 reads a post of kind message | HOLDS. `git grep -n 'gkReplanRole' -- internal/orchestrator/` answers replan.go:16, :71, :160, replan_owner.go:30, replan_plan_fault.go:44, found_security.go:65 and ji_plan_fault_replan_test.go:49, all Job-to-Job. tick.go:82-105 calls gkReplan, guReplanRejected, guReplanAskOwner and guReplanPersonCancelled, none of which reads a message. |
| 1. Job per owner message, passes `job-record` | NOT DONE. The schema refuses `discovered_from` = post id (above). Base Job list and the `--- PASS` line were not produced. |
| 2. Not-a-request table | NOT DONE. The guest, `To J-1`, `Lease: "task"` and close cases PASS, but the answer-notice case fails to rig (see open items). I did not re-run the table to a full pass. |
| 3. Two messages, two Jobs, third pass none | NOT DONE, same cause as row 1. |
| 4. New Job sets off no other replan | NOT DONE, same cause. Reading replan_owner.go:30 shows it acts only on a cancelled or rejected orchestrator Job, and found_security.go:65 reads only the A1 text, so I expect it to hold once row 1 is settled. |
| 5. `go test ./internal/orchestrator/ -run 'TestPfMsg\|Replan\|Gk'` and the walk tests | NOT DONE. Not run. |
| 6. `make lint`, `make cross`, `git diff main --stat` | NOT DONE. Not run. The diff touches only the three files in Touches. |

Rules: no guard, gate or cap was added. No existing test was edited or deleted. The Windows and macOS builds, lint and the walk tests were not run.

## What changed (commit 472fa349, WIP, not for landing)

- `internal/orchestrator/pf_message_replan.go`: `pfMsgReplan`, `pfMsgRequests`, `pfMsgIsRequest` and `PfMsgJobBody`. The Job body is `ReplanJobBody` with the title and the acceptance text replaced. The request test is the one in the brief. An empty or non-string text is not a request.
- `internal/orchestrator/tick.go`: a `Load` and a `pfMsgReplan` call after `guReplanPersonCancelled`.
- `internal/orchestrator/pf_message_replan_test.go`: the `TestPfMsg*` tests.

Landing this as it stands would put schema-invalid Jobs on the Feed.

## Split for the orchestrator (pick one)

- (a) A contract lane widens `discovered_from` in `contract/schemas/job-record.json` to admit a Feed record id, with fixtures and a check of `lint.go`. This lane then re-runs unchanged.
- (b) Re-brief with the marker as A1 lead text, as `kbSfPosted` does (found_security.go:59-75), and `discovered_from` null. That fits in the current Touches. Row 1 then reads "marked by its A1 text" instead of "discovered_from the post id". The ordering and request logic in this commit carry over.

## Open items

- The answer-notice case of row 2 fails to rig. A hand-built control (`status: accepted`, `target: J-1`) is refused by the control schema: `$: oneOf matched 0 branches: $.body: missing required question_event`. The faithful rig makes J-1 a person Job and answers it through the real answer path in person.go (around :255-280). `person_fields_test.go:68` already drives that path.
- NOT VERIFIED: nothing on a live screen. This lane has no UI.

NOT DONE
