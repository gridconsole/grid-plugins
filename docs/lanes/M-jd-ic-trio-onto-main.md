# M-jd-ic-trio-onto-main

Carries C-jd-ic-job-post-exists-fix, C-jw-question-job-repick (both already on this lane) and C-jv-approve-send-back, onto plugin main dc301fce.

## Codes kept (internal/cli/error_codes.go, contract/README.md)

No value clash: no two codes share a value, no code has two values.

- From main (M-jw-record-id and the earlier lanes): `no_such_project`, `no_such_task`, `no_such_note`, `invalid_argument`, `invalid_policy`, `workflow_file_changed`, `record_id_used`, `note_state`, `note_exists`, `notes_full`, `guest_refused`. Main's `errorCode` cases (`RecordIDUsedError`, `ilGuestRefused`, `signal.RefusedError`) are kept. The settings codes and `index_rebuild_failed` (internal/braincmd/changes.go) are untouched.
- From the jd-ic lane: `job_exists` (`CodeJobExists`, `feed.ErrJobExists` case), placed after `guest_refused` in the file and in the README table (the README table lists the cli constants first, in file order, then `person_required`, `job_waiting` and the settings codes). The README sentence now reads "The first twelve constants".
- From jv (approve send-back): none; it adds no code. The merge was clean.
- From jw-question-job-repick: none.

## Proof table

| Row | Result |
| --- | --- |
| 1 merges | HOLDS. `git diff --name-only --diff-filter=U` empty; `git merge-base --is-ancestor origin/main HEAD` exit 0; `git merge-base --is-ancestor origin/lane-C-jv-approve-send-back HEAD` exit 0. |
| 2 jd-ic rows | HOLDS. `go test -count=1 -run '^TestApproveWaitsForTheAfterJob$\|^TestFu2ApproveJobWaiting\|^TestGlJobWaitingCode\|^TestIcj\|...' -v ./internal/cli/ ./internal/feed/` exit 0: PASS TestApproveWaitsForTheAfterJob, TestFu2ApproveJobWaitingJSON, ...WithoutJSON, ...Roles (owner, member, agent), TestGlJobWaitingCode, TestIcjJobExistsRefused, TestIcjOneWinner, TestIcjJobPostExistsCLI, TestIcjUnleasedKeepsFirstJob, TestIcjErrorCode. No SKIP. |
| 2 jw repick rows | HOLDS. `go test -count=1 -run 'TestJwRp\|Question\|PersonJob' -v ./internal/cli/` exit 0: PASS TestJwRpPersonAskRepicksWhenItsPickWasTaken, TestJwRpLeaseAskRepicksWhenItsPickWasTaken, TestJwRpAskFailsWhenEveryPickIsTaken (person, lease). See the test change below. |
| 2 jv rows | HOLDS. Same run: PASS TestContractCommandsParserInventory, TestJvSendBackRejects, TestJvSendBackRefusals (7 rows), TestJvSendBackOldReader; `./internal/orchestrator` PASS TestJvSendBackTeamOnly; `./internal/contract` TestFixtures ok. |
| 3 step 4 | HOLDS. `go test -count=1 -race ./internal/feed/... ./internal/cli/... ./internal/question/...` exit 0: `ok internal/feed 45.051s`, `ok internal/cli 104.749s`, `ok internal/question 6.590s`. `TestHfFsCodeTable` (the README table against error_codes.go) PASS. |
| importers | HOLDS. The importers of the changed packages are most of the module, so `go test -count=1 ./...` was run: exit 0, every package ok (internal/approvefix has no test files). |
| 4 lint, cross | `make lint` exit 0 (`go vet ./...`). `make cross`: see the end of this file. |
| 6 no test deleted or skipped | HOLDS. No test deleted; no SKIP in the runs above. |
| Rules | No attribution lines; no guard, gate or cap added; plugin repo has no personal paths or emails in the diff. |

## What changed

- Merge of origin/main: conflicts in contract/README.md and internal/cli/error_codes.go only, resolved as above.
- Merge of origin/lane-C-jv-approve-send-back: clean.
- Two test setups changed, both because main moved under the stack. They are named here for the reviewer:
  1. internal/cli/jw_question_job_repick_test.go, `jwRpQuestionFiles` (a test file of this lane's own carried work, not on main). Main's lock-order change (internal/question/question.go `add`) now writes a zero-byte `Q-n.feed` id marker (ReserveGate) before the Job post and keeps it on a failed post, by design (ask_job_test.go `publish error keeps the marker`). Row 3 of the repick brief ("no question file left half-written") failed on it. The helper now skips zero-byte files; a file with content still fails the test, and the test still checks that no question post and no question Job exist.
  2. internal/orchestrator/plan_test.go, `approveRevision` (an existing test helper on main). It numbered the approve Job 90 plus the count of approve Jobs. In TestMalformedPlanDoesNotBlockOtherTasks the second approval got J-91, the id the tick's re-plan Job already holds, and the `job_exists` refusal failed the test at plan_approval_test.go:99. The helper now steps past ids the Feed holds. No assertion changed; the test passes with its assertions as they were. The jd-ic brief allowed such a fix only when the assertions do not depend on the second record: the final `Assigned == 1` depends on the approval of the new plan text, which the new id still carries, so I judged it the allowed kind. A reviewer who reads it otherwise should treat it as a must-fix question.

## NOT VERIFIED on a live screen

Nothing here touches a screen. No RealPlugin test was part of this brief.
