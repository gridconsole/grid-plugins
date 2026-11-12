# C-pf2-marker-lead-text: the message Job carries its post id as A1 lead text, discovered_from null

Split (b) of C-pf-message-orchestrator-job (fde91ea3): no contract change. `PfMsgJobBody` posts `discovered_from: null` and A1 begins `Person message post <post id> (`. `pfMsgPosted` reads that lead from the snapshot (as `kbSfPosted`, found_security.go:59-75), so a second pass posts nothing. The reader, `pfMsgRequests`, the tick.go call and the ordering are as C-pf left them.

## Proof table

| Row | Result |
|---|---|
| Base check: no replan on 00413d82 reads a post of kind message | HOLDS. `gkReplanRole` is read at replan.go:16, :71, :160, replan_owner.go:30, replan_plan_fault.go:44, found_security.go:65, all Job-to-Job (C-pf report). tick.go calls gkReplan, guReplanRejected, guReplanAskOwner and guReplanPersonCancelled, none reads a message. |
| 1. One Job for one owner message, passes `job-record`, red on base then green | HOLDS. Green: `--- PASS: TestPfMsgOwnerMessageGetsOneJob (0.13s)`; the test checks `contract.Validate("job-record", body)`, `discovered_from` present and null, the title and the quoted message (pf_message_replan_test.go:63-95). Red on base (tick.go checked out from 00413d82, base Job list is `[J-1]` only): `pf_message_replan_test.go:72: want J-1 and one new job record, got 1` / `--- FAIL: TestPfMsgOwnerMessageGetsOneJob`. tick.go restored with `git checkout HEAD -- internal/orchestrator/tick.go`. |
| 2. Not a request, no Job after two passes (table) | HOLDS. `--- PASS: TestPfMsgNotARequest/guest_message`, `/owner_message_to_a_job`, `/tick_note_with_the_task_lease`, `/answer_notice_with_its_control`, `/close_post`. The answer-notice case posts a real person Job (`approvefix.JobBody("J-90")`) and answers it with `AnswerPersonWith` (person.go:263-276), as person_fields_test.go:37 does; the test asserts the accepting control and the one notice `lead answered J-90 ...` are on the Feed. Those cases pass on base too (they assert nothing is posted); they guard the rules, row 1 and 3 are the red ones. |
| 3. Two owner messages, two Jobs with distinct ids, a third pass posts none | HOLDS. `--- PASS: TestPfMsgTwoMessagesTwoJobs (0.30s)`. Red on base: `pf_message_replan_test.go:152: want two Jobs with distinct ids, got map[]`. |
| 4. The new Job sets off no other replan; the Feed holds no further job record | HOLDS. `--- PASS: TestPfMsgJobSetsOffNoOtherReplan (0.20s)` (three passes, J-1 and the one message Job only). discovered_from is null, so gkReplanned (replan_owner.go:30) does not take it as the re-plan of a Job. Red on base: `pf_message_replan_test.go:172: want J-1 and the message Job only, got [J-1]`. |
| 5a. `go test -count=1 ./internal/orchestrator/ -run 'TestPfMsg\|Replan\|Gk'` | HOLDS, exit 0: `ok github.com/gridconsole/grid-plugins/internal/orchestrator 13.867s` |
| 5b. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0: `ok .../internal/cli 0.055s` |
| 5c. `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0: `ok .../internal/job/lint 0.187s` |
| 6a. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output); `gofmt -l internal/orchestrator` prints nothing. |
| 6b. `make cross` | HOLDS, exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok`. |
| 6c. Diff names only the Touches files | HOLDS. `git diff 00413d82 --stat` names pf_message_replan.go, pf_message_replan_test.go, tick.go, and the lane reports under docs/lanes/. (`git diff main` also lists unrelated files because local `main` is ahead of this branch's base; the diff to the base is the four files.) No schema, fixture or contract file. |
| Importers of internal/orchestrator tested | HOLDS. `go list` importers: internal/cli, internal/mcp, internal/migrate (cmd/grid and lib import cli). `go test -count=1 ./internal/orchestrator/ ./internal/mcp/ ./internal/migrate/ ./internal/job/lint/` exit 0 (orchestrator 90.551s, mcp 0.539s, migrate 8.784s, lint 0.187s); `go test -count=1 ./internal/cli/` exit 0 (`ok .../internal/cli 48.474s`). |
| Walk tests (plugin) | HOLDS. `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok (5b); `internal/job/lint` ok (5c). The app walk tests are not in this repo. |
| No guard, gate, cap or hold on agents | HOLDS. `pfMsgReplan` posts a Job and nothing more. |
| No existing test edited or deleted | HOLDS. `git diff 00413d82 --stat` shows only additions; the TestPfMsg tests are the new tests of C-pf, which this lane carries and changes (WIP in C-pf, never landed). |
| No attribution lines, no personal paths/hosts/emails | HOLDS. Commits carry one sentence; the files hold no path outside the repo. |

## What changed

- `internal/orchestrator/pf_message_replan.go`: `pfMsgA1Lead`, `pfMsgPosted` (reads the A1 lead of orchestrator Jobs), `PfMsgJobBody` with `discovered_from` null and A1 `Person message post <id> (a message to the task) reads: "<text>". ...`; `pfMsgReplan` asks `pfMsgPosted` instead of `gkReplanned`.
- `internal/orchestrator/pf_message_replan_test.go`: the helper finds a message Job by its A1 lead; the test checks `discovered_from` is present and null; the answer-notice case goes through `AnswerPersonWith`.
- `internal/orchestrator/tick.go`: as C-pf (one `pfMsgReplan` call after `guReplanPersonCancelled`, with a `Load` before the next replan).

## Note for the orchestrator

The answer notice AnswerPersonWith posts carries `Lease: "task"` (a person answers through the lease holder's writer), so the lease rule excludes it as well as the control rule; the control rule alone is read in `pfMsgRequests` and has no separate case, because the path posts nothing else with a control `Re`.

## NOT VERIFIED on a live screen

Nothing: this lane has no UI.

DONE
