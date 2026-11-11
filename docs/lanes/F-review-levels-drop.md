# Lane F-review-levels-drop: review levels L0-L3 and max_rounds leave lint and the contract

Owner direction (the brief, written at /srv/gridwork/tmp/hand/prompt-builder-F-review-levels-drop.txt, "answers 11 Nov q-levels and verdicts v-10"): review levels are dropped and the orchestrator judges per step whether a review runs; no Grid-made guards, caps or limits.

A Job no longer carries a level or a round count. lint does not check them, the Feed fold does not read them, and an old record or plan that still carries them reads without error with the value ignored.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. List every read and write of the level and max_rounds field, and every skill or doc line naming L0-L3 | HOLDS | Section "Row 1 list" below, from `grep -rnIE "ReviewLevel\|review_level\|MaxRounds\|max_rounds\|..."` and `grep -rnIE "\bL[0-3]\b"` over the whole clone, plus a grep of `skills/` that returned nothing. |
| 2. lint does not check a level or max rounds; a Job with no level lints clean | HOLDS | `internal/job/lint/lint.go` lost the two checks (base :137-142). New tests `TestJobWithoutLevelOrRoundsLintsClean` and `TestRetiredLevelAndRoundsAreNotChecked` in `internal/job/lint/lint_test.go`; `go test ./internal/job/lint/` exit 0. |
| 3. Contract types and fold drop the field; an old Feed record that still carries it folds without error and the field is ignored | HOLDS | `internal/contract/types.go` lost `Review`, `LevelRaised`, `MaxRounds` and the `LevelRaised` type; `internal/contract/fold.go` no longer decodes them. New test `TestFoldIgnoresRetiredLevelFields` in `internal/contract/fold_test.go` folds the fixture record `contract/fixtures/job/valid/basic.json` (it carries `review: "L1"`, `max_rounds: 2`, `level_raised: null`; the test asserts that first) with `review: L3`, `max_rounds: 7` and a `level_raised` object added, expects status running, a verdict that is not "ignored", and no `review`, `max_rounds` or `level_raised` key in the folded state. `go test ./internal/contract/` exit 0. |
| 4. Skills and docs no longer tell an agent to pick a level (`git grep -n "L0\|L1, L2" -- skills docs`: no hit) | PARTLY. `skills/`: no hit. `docs/job-contract.md` and `docs/plan.md`: clean. NOT DONE for 7 lines in two frozen lane proof reports | `docs/job-contract.md` planned-fields and review paragraphs rewritten; `docs/plan.md` example lost `review: L1` and `max_rounds: 2`. Remaining hits are only in `docs/lanes/L25.md` (:53, :92, :94, :160) and `docs/lanes/L37.md` (:5, :30, :35): past lane reports that describe what those lanes built (the floor, and "the L0 code check"). They do not instruct an agent to pick a level. I did not rewrite past lane reports inside the budget. |
| 5. go test ./... and make lint: 0 fail | HOLDS | `go test ./...` over 64 packages (`go list ./... \| wc -l` = 64): no line other than `ok` and `? ... [no test files]`. `make lint`: `go vet ./...` then no gofmt finding (first run listed `internal/job/job.go` and `internal/job/lint/lint_test.go`; fixed with `gofmt -w`, rerun clean). Both commands were piped, so the exit code shown is the pipe's; the failure text is what I checked. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./... && GOOS=darwin CGO_ENABLED=0 go build ./...` printed `builds-ok`, exit 0. |
| New tests fail on base | HOLDS | Stashed the six source files, ran the three tests, restored. Failing lines: `--- FAIL: TestJobWithoutLevelOrRoundsLintsClean ... findings: [{Rule:job.max-rounds ...} {Rule:job.required ... Missing Job field max_rounds} {Rule:job.required ... Missing Job field review} {Rule:job.review ... Invalid review level}]`; `--- FAIL: TestRetiredLevelAndRoundsAreNotChecked ... lint still checks job.review`; `--- FAIL: TestFoldIgnoresRetiredLevelFields ... state still carries review: {"id":"J-1",...,"review":"L3","level_raised":null,...,"max_rounds":7,...}`. |
| No existing test edited to pass; deleted or changed tests named | HOLDS (with the changes below, each for behaviour the owner removed) | See "Existing tests changed". |
| No guard, gate, cap, hold or blocking hook added | HOLDS | None added. Two Grid-made limits were removed: the `max_rounds` exhaustion that rejected a Job (`internal/orchestrator/tick.go`, base :315-319) and the review-slot cap of 3 for L2/L3 reviewers (`internal/orchestrator/ready.go`, base :270-293). |
| Tests of changed packages and of every importer | HOLDS | Changed: `internal/job`, `internal/job/lint`, `internal/job/jobcmd`, `internal/contract`, `internal/jobgraph`, `internal/orchestrator`, plus the fixtures read by `contract/formats`. Importers of those cover nearly the module, so the whole module ran (`go test ./...`, 64 packages, no failure). The orchestrator reruns the cross-repo set at landing. |
| No attribution lines, no personal paths | HOLDS | Commits carry a one-sentence message and no Co-Authored-By or session line (the owner's rule over the reminder). The diff has no host or personal path. |
| UI rows | none | No UI change. `grep` over `.mjs .js .html .ts` for `level_raised`, `max_rounds`, `.review`, `review_capacity` found no hit. |

## What changed

- `internal/job/lint/lint.go`: the level check and the max_rounds check are gone; the planned-field presence check no longer asks for `review` or `max_rounds`.
- `internal/job/job.go`: `MaxRounds` removed; `review` and `max_rounds` leave `PlannedFields`; new `RetiredFields` (`max_rounds`); `Review` stays as an optional `omitempty` field, read and ignored.
- `internal/job/planned.go`: a plan that still carries `max_rounds` or `review` decodes without error; `max_rounds` is dropped before the strict decode.
- `internal/job/source.go`: the rendered brief no longer writes `max_rounds`, and writes `review` only when the plan carried one.
- `internal/contract/types.go`, `fold.go`: `Review`, `LevelRaised`, `MaxRounds` removed; the fold ignores them in an old record. A security finding no longer raises a level; it still sets `needs_lead`.
- `internal/orchestrator/tick.go`, `ready.go` (forced by the contract change, outside the named touch list): no `max_rounds` rejection after a failed attempt, no L2/L3 review-slot cap.
- `internal/jobgraph/floor.go` (outside the named touch list): a Job with no level raises no error. A level that an old plan still declares is checked against the floor as before.
- `contract/schemas/job-record.json`, `cmd-feed-state.json`: `review`, `max_rounds` (and `level_raised` in the state schema) are no longer required; the property definitions stay so old records still validate.
- Fixtures and golden: `testdata/golden/job-brief.golden` and `contract/fixtures/fmt-job-render/valid/{full,elided}.json` lose the `max_rounds` line.
- Docs: `docs/job-contract.md`, `docs/plan.md`.

## Existing tests changed (behaviour removed by the owner's direction, written in the brief above)

- `internal/job/lint/lint_test.go` `TestContractRules`: the `review` and `rounds` rows removed; covered by the new `TestRetiredLevelAndRoundsAreNotChecked`, which asserts the opposite.
- `internal/contract/fold_test.go`: scenario `security-finding-raises-level` is now `security-finding-needs-lead`; it still asserts `NeedsLead`.
- `internal/orchestrator/orchestrator_test.go`: `TestProgressResetsCrashLoop` and `TestAppliedRetrySurvivesMetadataLoss` assert the Job carries on and is not parked, instead of rejected; `TestReviewCapacityUsesReady` is now `TestReviewerJobsHaveNoCapacityCap` (four reviewer Jobs all ready and assigned).
- `internal/jobgraph/floor_test.go`: the `missing` case expects 0 errors, not 1.
- `internal/job/jobcmd/feedjob_pick_test.go`: used `max_rounds` only as an arbitrary wrong-typed field; now `item`. The behaviour it covers (the error names the record and the field path) is unchanged.

## Row 1 list (base main; line numbers are before this lane)

max_rounds:
- write/field: `internal/job/job.go:26`, `internal/contract/types.go:148`; `internal/question/job.go:42` writes `"max_rounds": 1` (still does, see open items)
- read: `internal/contract/fold.go:129`, `:139`; `internal/orchestrator/tick.go:317-318`
- field lists: `internal/job/job.go:179` (PlannedFields), `internal/job/source.go:501` (render keys); `internal/job/planned.go:15` and `internal/job/lint/lint.go:99` loop PlannedFields
- check: `internal/job/lint/lint.go:140-141`
- schema: `contract/schemas/job-record.json:75,529`; `contract/schemas/cmd-feed-state.json:217,360`
- tests and fixtures: `lint_test.go:85`, `orchestrator_test.go:233`, `jobcmd/feedjob_pick_test.go:39,48`, plan testdata, `testdata/golden/job-brief.golden:10`, `contract/fixtures/**` (job, job-record, cmd-feed-state, fmt-job-render)

Review level:
- field: `internal/job/job.go:24`, `internal/contract/types.go:145-146` (`Review`, `LevelRaised`)
- check: `internal/job/lint/lint.go:137-139`; `internal/jobgraph/floor.go:28,62,109-114`; `internal/cli/job_graph.go:111` (prints it)
- read/write in fold: `internal/contract/fold.go:128,139`, `:254-256` (security finding raised the level to L3)
- read in orchestrator: `internal/orchestrator/ready.go:273` (L2/L3 reviewer slots); `internal/question/job.go:42` writes `"review": "L0"`
- docs naming levels: `docs/job-contract.md:11,43-46`, `docs/plan.md:70`; lane reports `docs/lanes/L25.md:17,53,62,92,94,160,161`, `docs/lanes/L37.md:5,30,35`; `skills/`: none

## Open items

1. Named split, lane `F-review-floor-drop`: `grid job floor` still reports a floor and still checks a level that an old plan declares (`internal/jobgraph/floor.go`, `internal/cli/job_graph.go`, goldens `job-floor-*.golden`, `job.Job.Review`, `job.Job.LevelRaised` and `review_reason`, and `level_raised` still required in `contract/schemas/job-record.json`). I kept that behaviour because the brief does not name the floor and about a dozen tests assert it.
2. `internal/question/job.go:42` still writes `"review": "L0"` and `"max_rounds": 1` into the Job body it posts; it is read and ignored. Stop writing them.
3. `docs/lanes/L25.md` and `L37.md` still hit the row 4 grep (7 lines, frozen lane reports).
4. Behaviour to confirm: with the `max_rounds` rejection gone, a failed attempt retries by itself until the crash-loop park (three failures) or a person acts. That matches "tasks carry on by themselves until a person points", but it is a change in the orchestrator, outside the named touch list.

## NOT VERIFIED on a live screen

No UI was changed. Not run live: the orchestrator tick loop with real workers (covered only by `internal/orchestrator` tests), and `grid job floor` and `grid plan check` against a real plan.
