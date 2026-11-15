# C-r6-ship-answer-lands: after the ship person Job is answered, the task lands and the result is recorded

Base: 3adc0590. origin/main merged before the last proof run. PLUGIN-ONLY: the wake is in the plugin, not the app.

## Base check

- internal/cli/deliver_person_job.go:49 `postShipPersonJob`. At :68 it posts `migrate.PlanJobBody(id, owner)`, whose acceptance (internal/migrate/receipt.go:323) is "The plan revision is approved".
- The deliver outcome line that names J-4 is not in the plugin source. `git grep "Ship step on"` finds nothing. The r6 Outcome was the agent's own `--outcome` words. deliver.go prints `"%s delivered; waiting for the person"` (deliver.go:124) and records the action `"Post person Job "+posted+" for the ship step"` (deliver.go:112).
- internal/land/land.go:219-220: `if p.Gate != "none" { approval, err := s.tasks().ApprovalForGate(cwd, t.ID, p.Gate)`.
- internal/task/gates.go:140-143: "ApprovalForGate is Approval under the task and plan locks. The gate name is ignored: the plan Job is the only approval …".
- On base, nothing runs land after the answer (see Do 1). The four new tests fail on base.

## Do 1: what runs after a person Job is answered

1. `grid approve` internal/cli/task_write.go:228. `planPersonJob` (orchestrator.go:550) picks the open ship Job, then land.Store.Approve (internal/land/approval.go:30) calls task.Store.Approve (gates.go:32). When an earlier plan Job result already matches, Approve writes nothing (gates.go:87-95). That is why r6 had no `approve` history line.
2. recordApproval internal/cli/approve_record.go:68:
   - A person with no lease (r6 person.sh, GRID_MEMBER) posts an `answer` through orchestrator.AnswerPersonWith (internal/orchestrator/person.go:257).
   - A leased writer posts the accepting control at once (personApproval).
3. The holder's pass is `grid worker next` (internal/cli/worker.go:155, `orchestrator.Stream`). It runs Stream.task (internal/orchestrator/stream.go:209), then Store.Tick (stream.go:291, tick.go:21), then personAnswers (tick.go:62, person.go:56-123). personAnswers posts the "answered" message and the accepting control.
4. After that, Tick only frees and assigns Jobs whose `after` names the person Job (tick.go:146-187, spawn lines at stream.go:300-342). The wake (tick.go:217, events.go:193) covers only results, rejections and wakes. The ship Job has no `after` dependents, so on base nothing posts or runs land.

That wake lives in the plugin, so this lane is PLUGIN-ONLY. No app lines need to change.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Red on base, then green | HOLDS | Base: worktree at 3adc0590 with ship_land_test.go copied in. `go test -count=1 -run 'TestShipJobTextNamesLanding\|TestShipAnswerLandsTask\|TestShipLandFailureRecorded\|TestShipLandFailurePostsNoSecondJob' ./internal/cli/` exit 1. Failing lines below. Lane: the same run, plus TestDeliverPostsShipPersonJob, TestApproveAfterDeliveryRecordsPlanApproval and TestLandRejects*, gives `ok internal/cli` exit 0. |
| 2 TestApproveAfterDeliveryRecordsPlanApproval, approve_no_gate_test.go, every TestDeliver*/TestApprove* pass unedited | HOLDS | All inside `go test -count=1 ./internal/cli/` exit 0. One test is edited, and only for J-6's text: internal/cli/deliver_person_job_test.go:84 now expects "The delivered work is approved to land" (TestDeliverPostsShipPersonJob). |
| 3 `make lint` and `go test ./internal/cli/ ./internal/land/ ./internal/task/` exit 0, unpiped | HOLDS | `make lint` exit 0 (`go vet ./...`). `go test -count=1 ./internal/cli/ ./internal/land/ ./internal/task/` exit 0: cli 30.7s, land 0.4s, task 2.5s. |
| 4 Lane doc holds the path:line map and PLUGIN-ONLY or APP-FOLLOW-UP | HOLDS | "Do 1" above; PLUGIN-ONLY. |
| Do 2 ship Job text | HOLDS | deliver_person_job.go `shipAcceptance` = "The delivered work is approved to land"; `shipJobBody`. TestShipJobTextNamesLanding. |
| Do 2 deliver outcome names the posted Job, not J-4 | NOT DONE (partly) | No plugin code writes J-4. The J-4 words were the agent's `--outcome`. deliver already names the posted Job in its action "Post person Job J-N for the ship step" (deliver.go:112) and in `--json person_job`. The plain stdout line can't add the id: it is pinned verbatim by TestDeliverWithNoProposalsPostsNoJobAndPrintsAsBefore (deliver_keeper_job_test.go:153) and TestVdBr* (vd_deliver_branch_test.go:52, :81), which must pass unedited. Changing what the agent writes needs a skills/deliver wording change, which is not in this lane's files. |
| Do 3 answer, then land runs, then the result is recorded | HOLDS | TestShipAnswerLandsTask: a real git repo. The deploy command `git merge --ff-only lane` moves main to the delivered sha. History gets `ship answered`, `land started` and `land landed`, and the task is closed. |
| Do 3 land failure recorded, no second person Job | HOLDS | TestShipLandFailureRecorded: a `land failed` line with exit, main unmoved, task still waiting, approve exit 0. TestShipLandFailurePostsNoSecondJob: two presses give one `land failed` line and one approve person Job. |
| Do 3 approval as LA1 (plan Job only, no new kind) | HOLDS for the approval source | Land asks ApprovalForGate (land.go:219-226). That reads only the plan Job result (gates.go:143). The ship answer is that result (S6c4). No new approval kind, gate, hold or cap. The `land-approval` field read (approval.go:50) stays: retiring it is LA1/LA2's own brief, and land_test.go:325 and :412-414 pin it. |
| Tests of every package importing the changed ones | not run, over budget | Run: internal/cli, internal/land, internal/task and internal/orchestrator (the orchestrator run was before the last cli-only fix; `ok` in 82.6s). `go list` of importers not run. The whole module is the orchestrator's at landing. |
| Walk tests (plugin) | HOLDS | TestHsIpWalksEveryCommand and TestIlGuestSiteList ran inside `go test -count=1 ./internal/cli/` exit 0. `go test -count=1 ./internal/job/lint/` exit 0. The first cli run failed TestJzQwEveryStoreLiteralSetsWarn (ship_land.go:43). Fixed with `Warn: env.Stderr`. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin`, then `git merge origin/main`, before the runs above. |
| Reviewer runbook read | not run, over budget | /srv/gridwork/tmp/hand/runbook-reviewer.txt not read. |
| Unleased answer through the worker stream (`Ticked`) | NOT DONE (no test) | The wiring is in, at stream.go after a successful Tick and at worker.go `Ticked`. No test drives a `grid worker next` pass. The four tests use the leased path, so it is the same `landAfterShip`, but not the same trigger. |

Base failing lines (row 1):

```
--- FAIL: TestShipJobTextNamesLanding
    ship_land_test.go:92: ship person Job J-1 acceptance [{ID:A1 Text:The plan revision is approved Traces:item State:}], want "The delivered work is approved to land"
--- FAIL: TestShipAnswerLandsTask
    ship_land_test.go:104: main is d79e667b… after the answer, want the delivered bae642e8…; stderr ; history: (approve line only)
--- FAIL: TestShipLandFailureRecorded
    ship_land_test.go:126: history does not record the failed land:
--- FAIL: TestShipLandFailurePostsNoSecondJob
    ship_land_test.go:146: 0 land failed lines after two presses, want 1 (land ran once):
FAIL	github.com/gridconsole/grid-plugins/internal/cli
```

## What changed

- internal/cli/deliver_person_job.go: `shipAcceptance`, `shipJobBody` (PlanJobBody with the ship text; same kind and item, so its answer is still the plan approval) and `isShipJob`.
- internal/land/ship.go (new): `Store.AfterShip`.
  - It writes a `ship answered · J-N accepted; grid land runs` history line under .land.lock, once per ship Job.
  - Then it runs `Run`. A land that stops before its command runs gets a `land failed` line. A land that passes closes the task (`landed after J-N`).
  - It posts no Job.
- internal/cli/ship_land.go (new): `landAfterShip`. It acts only on a waiting task with an accepted ship Job, and says a failure on stderr; it never fails the caller.
- internal/cli/task_write.go (runApprove) and internal/cli/orchestrator.go (runPersonApproval): call `landAfterShip` after a recorded approval. These files are outside the brief's list; this is the leased answer's trigger.
- internal/orchestrator/stream.go: new `Stream.Ticked` callback after a successful Tick. internal/cli/worker.go sets it. These files are outside the brief's list; this is the r6 trigger, the unleased answer accepted by the holder's pass.
- internal/cli/ship_land_test.go (new), deliver_person_job_test.go:84 (text only).

## Who can trigger the land

A hosted owner or member answer on the ship Job runs `grid land` on the owner's machine; the guest path cannot answer a ship Job. (Note: the ship Job is kind `approve` (deliver_person_job.go:58), and `contract.GuestAddressee` refuses a guest for kind approve (internal/contract/guest_person_answer.go:19); `AnswerPersonWith` takes an unleased guest only when that holds (internal/orchestrator/person.go:286, via person.go:65 in personAnswers).)

## Open items

- The deliver outcome still can't name the Job on the plain stdout line (see the NOT DONE row).
- No test for the worker stream trigger.
- A land runs inside the stream pass, up to the 10-minute land timeout, and blocks that pass for other tasks.
- When the land passes, the task is closed. This is my reading of "task state"; check it with the owner.

## NOT VERIFIED

- No live screen; no UI in this lane. The r6 end-to-end run (person.sh, then a worker pass, then a merge on the handbook) was not rerun.

CLOSED BY C-r6-ship-ticked-test: accepted 00:33 UTC
