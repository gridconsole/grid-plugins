# C-nq-plan-step-needs

Base: main 00413d824ddeb0429d8b0d49704a4fb7a4592438. Base check: `git grep -n 'Needs' -- internal/plan/items.go` printed nothing (exit 1), so there was no needs field.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1 plan.Items reads needs | HOLDS | `go test -count=1 -v -run TestNqNeeds ./internal/plan/` exit 0: `--- PASS: TestNqNeedsItemsReadTheLine`, `--- PASS: TestNqNeedsYamlPartStillRefusesNeeds`. Test: internal/plan/nq_needs_test.go. Red on base: `internal/plan/nq_needs_test.go:27:34: items[i].Needs undefined (type Item has no field or method Needs)` |
| 2 CheckPlanReport warnings | HOLDS | `go test -count=1 -v -run TestNqNeeds ./internal/jobgraph/` exit 0: `--- PASS: TestNqNeedsWarnings` (5 subtests), `TestNqNeedsSilentWithoutAnyLine`, `TestNqNeedsCleanPlanHasNoWarnings`. Red on base: `nq_needs_warn_test.go:39: warnings [] lack "item 2: no needs: line"` (same for the other four texts) |
| 3 plan items --json / plan check | HOLDS | `go test -count=1 -v -run TestNqNeeds ./internal/cli/` exit 0: `--- PASS: TestNqNeedsPlanItemsAndCheck`. Red on base: `items output lacks "needs":[]: [{"n":1,"id":"i-1","title":"One","prose":"needs: none",...` |
| 4 trap and legacy tests unedited | HOLDS | `go test -count=1 -v -run 'TestPlanCheckVerifyCountWarnings\|TestPlanPublicationRejectsChangedStepMetadata\|TestDigestIncludesStepMetadata\|TestDerivedJobsAndAnchors\|TestUpgradeLegacyProse\|TestUpgradeLegacyOptionalProse' ./internal/cli/ ./internal/orchestrator/ ./internal/plan/` exit 0, a `--- PASS:` for all six, no FAIL. `git diff main --stat`: no change under internal/plan/testdata/, testdata/job/, golden/, and no existing `_test.go` file. See the DEVIATION below for testdata/golden/. |
| 5 package tests | HOLDS | `go test -count=1 ./internal/plan/ ./internal/jobgraph/ ./internal/cli/ ./internal/orchestrator/ ./internal/approvefix/ ./internal/job/... ./internal/land/ ./internal/proof/ ./internal/task/` exit 0 (this list is the `go list` importers of plan and jobgraph) |
| 5 lint | HOLDS | `make lint` exit 0 (go vet ./...); `gofmt -l internal/` prints nothing |
| 5 cross | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| 5 walk | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 |
| Walk: lint | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0 |
| Walk: app-only walks (webui, plugincli, grid-server, slices) | not applicable | plugin repo has none of them |
| New tests fail on base | HOLDS | lines pasted in rows 1-3 (source stashed, tests kept) |
| No guard, gate, cap or hold on agents | HOLDS | warnings only; the plan check exit code counts problems only and is unchanged |
| Trap 1 (Prose and Anchor keep the line) | HOLDS | `TestNqNeedsItemsReadTheLine` asserts Prose and `Anchor(text, "step-2")` hold `needs: 1`; `TestPlanPublicationRejectsChangedStepMetadata` passes |
| Trap 2 (missing-line warning only when some step has one) | HOLDS | `TestNqNeedsSilentWithoutAnyLine`; `TestPlanCheckVerifyCountWarnings` passes with the shared fixture unedited |
| Hash unchanged | HOLDS | hash.go not edited; `TestHashGolden` and `TestDigestIncludesStepMetadata` pass |

## DEVIATION: two goldens outside the Touches list

The brief says no golden records `plan items`, looking in `golden/`. Two goldens do, under `testdata/golden/`: `plan-items.golden` (used by `TestItemsGolden`, internal/plan/hash_test.go:82) and `plan-cli-items-json.golden` (used by `TestPlanReadGolden/items/json`). A `Needs []int` field with tag `json:"needs"` and no omitempty, as the brief specifies, adds a `needs` key to every item, so both tests failed:

```
--- FAIL: TestItemsGolden (0.00s)
    hash_test.go:82: plan-items differs from the golden file
--- FAIL: TestPlanReadGolden/items/json (0.01s)
    plan_test.go:48: plan-cli-items-json differs from the golden file
```

I regenerated only those two with `make test UPDATE_GOLDEN=1 "GOFLAGS=-count=1 -run=TestItemsGolden|TestPlanReadGolden"`. The diff adds only the `"needs"` lines (`[]`, `[1]`, `[]`); `plan-cli-items-plain.golden` did not change. No test file was edited. The orchestrator decides whether this stands; the other way is `json:"needs,omitempty"`, which still would not print `"needs":[]`, so the brief's row 1 forbids it.

## What changed

- internal/plan/items.go: `Item.Needs []int` (`json:"needs"`); exported `NqNeedsRead(prose)` returns the numbers, the raw value and whether a `needs:` line exists (first line wins; grammar `none` or `N(, ?N)*`). Items fills `Needs` from Prose; it never errors on a needs value, and the YAML-part refusal of unknown keys is unchanged.
- internal/jobgraph/plan.go: `nqNeedsWarnings`, appended to `report.Warnings` after the Job checks, with the five exact texts. Cycles come from a depth-first walk over edges that name another plan step.
- New tests: internal/plan/nq_needs_test.go, internal/jobgraph/nq_needs_warn_test.go, internal/cli/nq_plan_needs_test.go.

## NOT VERIFIED on a live screen

Nothing here has a screen. The planner-agent behaviour (sweeping open steps by `needs:`) was not exercised; no claude run.

Head sha: see the handoff message.
