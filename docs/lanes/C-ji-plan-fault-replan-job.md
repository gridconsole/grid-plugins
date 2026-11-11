# C-ji-plan-fault-replan-job

When plan.md cannot be published, the tick posts one re-plan Job to the orchestrator role with the cause.

Base: 3306580b (lane-C-ji-plan-fault-replan-job, with lane-C-jf-plan-job-taken-notice 51e9c8be merged; `git merge-base --is-ancestor 51e9c8be HEAD` exit 0).
Base check: `planUnpublishable` on the base (plan.go:114-126) only posts the task message; it posts no Job. The skip `if snap.State.Jobs[j.ID] != nil` is plan.go:161. Both as the brief says.
Head: the lane commit after 3306580b (`git log 3306580b..HEAD`).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1 `TestJiPfReplanJobPosted` | HOLDS | `go test ./internal/orchestrator/ -run TestJiPf -v` PASS (ji_plan_fault_replan_test.go:64). One orchestrator job record titled `Re-plan: plan.md cannot be published`, id J-4 (plan has J-3), lease task, A1 text holds "each jobs entry must be an object", every field equals `ReplanPlanFaultJobBody("J-4", <cause from the notice>)` and the Feed body has no extra field. Fails on base, below. |
| 2 `TestJiPfOnceWhileOpen` | HOLDS | ji_plan_fault_replan_test.go:116: three ticks, one re-plan Job, one notice (`hvPlanNotices`). PASS. Fails on base, below. |
| 3 `TestJiPfAgainAfterAccepted` | HOLDS | ji_plan_fault_replan_test.go:130: J-4 closed with `f.s.control(state.Jobs["J-4"], contract.Accepted, "accepted", "")` (J-4 was Assigned; the fold reads Accepted, asserted), one tick posts J-5, notices stay at one. PASS. Fails on base, below. |
| 4 `TestJiPfIdAbovePlanIds` | HOLDS | ji_plan_fault_replan_test.go:169: plan `jobs: [J-7]` gives J-8. PASS. Fails on base, below. |
| 5 `TestJiPfBodyPassesRenderLint` | HOLDS | `go test ./internal/cli -run 'TestJiPf\|TestGuRj\|TestGk' -count=1 -v` exit 0: TestGkReplanBodyPassesRenderLint, TestGuRjBodyPassesRenderLint, TestJiPfBodyPassesRenderLint (plan error, empty, too long) PASS. `postFindings` has no lint error; `contract.Validate("job-record", data)` nil. ji_plan_fault_render_test.go:16. |
| 6a `go test ./internal/orchestrator/ ./internal/cli/ -count=1` | HOLDS | exit 0 (orchestrator ok 131.9s, cli ok 95.4s). TestHvPlan*, TestPlanPublication*, TestMalformedPlanDoesNotBlockOtherTasks, TestGk*, TestGu* are in those packages. |
| 6b importers of the changed package | HOLDS | `go list` of direct importers of internal/orchestrator: internal/cli, internal/migrate. `go test ./internal/migrate/ -count=1` ok 16.4s. |
| 6c `go vet ./internal/orchestrator/ ./internal/cli/` | HOLDS | exit 0, no output. |
| 6d `gofmt -l internal/orchestrator internal/cli` | HOLDS | empty. |
| 6e `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 6f `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok. |
| 6g diff names only files in Touches | HOLDS | `git diff 3306580b --stat`: ji_plan_fault_render_test.go (new), hv_plan_unreadable_test.go (1 line), ji_plan_fault_replan_test.go (new), plan.go, plan_test.go, replan_plan_fault.go (new). 6 files, 367 insertions, 8 deletions. |
| Body builder (step 1) | HOLDS | replan_plan_fault.go:28 `ReplanPlanFaultJobBody`: built from `ReplanJobBody`, role orchestrator (gkReplanRole), title const :15, `discovered_from` nil, A1 ends with " Cause: " + `gqCause(cause)`, A1 names the Job's own id. |
| Open-Job check (step 2) | HOLDS | replan_plan_fault.go:42 `jiPlanFaultOpen`: role orchestrator, that title, folded status not Accepted/Rejected/Cancelled (Submitted, Stale, Parked count as open), first job record carries lease task. `TestJiPfPersonJobDoesNotSilence` (:157): a person's Job with the title and no lease does not stop the tick's Job (J-10 beside the person's J-9). |
| Id (step 3) | HOLDS | replan_plan_fault.go:72 `jiPlanFaultJobID`: `gkNextJobID(snap)` for the fold, no second loop over Items or State.Jobs; plan ids from the text, an id with more than 9 digits or one Atoi refuses is skipped. `TestJiPfPlanIDNumbers` (:191) and `TestJiPfIdAboveFold` (:180). |
| New tests fail on base | HOLDS | Below. |
| No existing test edited to pass | HOLDS | Two lines changed, both allowed by the brief and both listed below. |
| No guard, gate, cap, hold or blocking hook | HOLDS | The change posts one Job and returns. It refuses, holds and counts nothing; `planUnpublishable` still returns nil after the post. |
| Small commits, no attribution, no personal paths | HOLDS | One commit with a one-sentence message, no Co-Authored-By or session line; this note has no personal path, host or email. |
| Edit only the named packages | HOLDS | internal/orchestrator and internal/cli only. |
| `*RealPlugin*` tests | n/a | No row names one. |

## New tests fail on base

Run with the lane's replan_plan_fault.go kept and plan.go taken from the base (`git checkout 3306580b -- internal/orchestrator/plan.go`, then restored with `git checkout HEAD -- internal/orchestrator/plan.go`; `git status --short` clean after). On the true base the files do not compile, since ReplanPlanFaultJobBody does not exist there:

```
ji_plan_fault_replan_test.go:70: want one re-plan Job, got 0: []
--- FAIL: TestJiPfReplanJobPosted
ji_plan_fault_replan_test.go:123: want one re-plan Job after three ticks, got 0: []
--- FAIL: TestJiPfOnceWhileOpen
ji_plan_fault_replan_test.go:136: J-4 is not on the Feed: <nil>
--- FAIL: TestJiPfAgainAfterAccepted
ji_plan_fault_replan_test.go:165: want the tick's own Job J-10 beside the person's J-9, got [...only J-9...]
--- FAIL: TestJiPfPersonJobDoesNotSilence
ji_plan_fault_replan_test.go:175: want one re-plan Job J-8, got []
--- FAIL: TestJiPfIdAbovePlanIds
ji_plan_fault_replan_test.go:187: want one re-plan Job J-13, got []
--- FAIL: TestJiPfIdAboveFold
```

`TestJiPfPlanIDNumbers` is a unit test of `jiPlanFaultJobID` and passes on any plan.go.

## Existing test lines changed

1. internal/orchestrator/plan_test.go:190, in `planJobRecords` (the brief names :181-194), so the "no Job from the plan" checks keep their meaning:
   - old: `if r.Body["kind"] != "approve" {`
   - new: `if r.Body["kind"] != "approve" && r.Body["title"] != jiPlanFaultTitle {` (and the comment above it says so)
2. internal/orchestrator/hv_plan_unreadable_test.go:88, `TestHvPlanOtherJobStillAssigned`. It went red because the re-plan Job (J-91, role orchestrator) is assigned in the same tick beside J-1: `Assigned:2`.
   - old: `if got := f.state(t); out.Assigned != 1 || got == nil || got.Status != contract.Assigned {`
   - new: `if got := f.state(t); jiPfPlanAssigned(out) != 1 || got == nil || got.Status != contract.Assigned {`
   - `jiPfPlanAssigned` (ji_plan_fault_replan_test.go:46) counts the spawns whose role is not orchestrator.

`plan_approval_test.go:97-101` did not go red and is unchanged: the re-plan Job was assigned on the faulty tick, so the tick after the fix assigns J-1 alone. No other existing test went red.

## What changed

- `replan_plan_fault.go` (new): `jiPlanFaultTitle`, `ReplanPlanFaultJobBody`, `jiPlanFaultOpen`, `jiPlanFaultJobID`, `postPlanFaultJob`.
- `plan.go`: `planUnpublishable` takes the plan text (call site :55 passes it), no longer returns early on a found notice, and ends with `postPlanFaultJob(snap, planText, oneLine(cause))`. The notice and its text dedup are as before.
- The cause put in the Job is `oneLine` of the error `planJobInputs` returned, the only error that reaches `planUnpublishable`; the errors of `read()`, the Feed and the file system never do.
- Tests: ji_plan_fault_replan_test.go and ji_plan_fault_render_test.go (new), and the two lines above.

Behaviour to know: the re-plan Job has role orchestrator and no `after`, so the same tick that posts it also assigns it (a wake for the orchestrator role). The text dedup of the notice still has its silent case (fault A, fixed, fault A again: no second notice); the Job closes it as row 3 shows.

## NOT VERIFIED on a live screen

- How the Job reads on the Feed and in Needs you in the app: only the record, the lint and the job-record contract are proved here.
- That an orchestrator agent started by the wake fixes plan.md from the cause text.
