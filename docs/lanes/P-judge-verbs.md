# P-judge-verbs: `grid judge` verbs in the plugin (verdict, review, rework, person, request id)

Lane branch lane-P-judge-verbs. `git fetch origin && git merge origin/main` printed "Already up to date." before the last proof run. Owner direction V11: review, one rework, then a person; no Grid-made limits. The verbs post what the orchestrator chooses; none decides, counts down or blocks.

## Proof table

| Row | State | Proof |
|---|---|---|
| A1 verdict parse ports the app's verdict cases and adds STUCK | HOLDS | `go test -count=1 -v ./internal/judge/` exit 0: `--- PASS: TestVerdictParse` (the app's 19 cases), `--- PASS: TestPjVbVerdictParseAddsStuck`; `internal/judge/verdict.go:24` |
| A2 rework body ports `TestReworkPostsANewJobIDWithOnlyJobRecordKeys` | HOLDS | `--- PASS: TestReworkPostsANewJobIDWithOnlyJobRecordKeys` (keys read from contract/schemas/job-record.json); `internal/judge/rework.go:16` |
| A3 request id ports both `judge_post_request_id` tests (no epoch) | HOLDS | `--- PASS: TestJudgePostRequestIDFollowsThePost`, `--- PASS: TestJudgeControlCarriesARequestID`; `internal/judge/id.go:26` hashes task, type, job, body, wake, person only |
| A4 a second call with the same wake succeeds with nothing posted (`ErrDuplicateWake`) | HOLDS | `--- PASS: TestPjVbJudgeReviewSameWakePostsNothing` (real Feed through the CLI: second `grid judge review J-4 --wake <same>` exits 0, prints "nothing posted" with the review record id, record count unchanged); `internal/cli/judge.go:184-197` |
| A5 a cancelled review gives one person Job (ports both `judge_cancelled_review` tests) | HOLDS | `--- PASS: TestJudgeReviewCancelledAtSpawnRaisesOnePersonJob`, `--- PASS: TestJudgeReviewCancelledAfterAVerdictRaisesNoPersonJob`, `--- PASS: TestPjVbJudgePersonForACancelledReviewIsRaisedOnce` (CLI, `job cancel` then three `judge person` calls give one person Job) |
| A6 `verdict` reports rounds over the `continues` chain | HOLDS | `--- PASS: TestPjVbVerdictReportsRoundsOverTheContinuesChain`; `internal/judge/state.go:133` |
| A7 a verdict from an older attempt is reported stale | HOLDS | `--- PASS: TestPjVbVerdictFromAnOlderAttemptIsStale`; `internal/judge/verdict.go:113` |
| A8 (V27) review base is the submitted SHA of the reviewed attempt; rework base is the head SHA, else the target's tip | HOLDS | `--- PASS: TestPjVbBaseIsTheSubmittedShaThenTheHeadOrTheTarget`; `internal/judge/review.go:35`, `internal/judge/rework.go:62` |
| `go test -count=1 ./internal/judge/... ./internal/cli/` exits 0 | NOT DONE | `./internal/judge` ok; `./internal/cli` exit 1: the new command is missing from four existing walk tables, `TestGgUnknownProjectWalk` (judge verbs exit 1 with empty stdout for an unknown project), `TestGgUnknownTaskWalkCoversTheContract`, `TestGuestProjectParity` ("missing guest parity command: judge ...") and `TestLtReadmeNamesEveryCommand` (README `## Commands` does not name `grid judge`). Not fixed: the call budget ran out (85% rule) |
| Inventory and walk after the merge of main | HOLDS | `go test -count=1 -v -run 'TestContractCommandsInventory\|TestContractCommandsParserInventory\|TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli/` exit 0 with `--- PASS:` for each |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0 (`ok .../internal/job/lint`) |
| docs/job-contract.md says the plugin posts the review and the rework | NOT DONE | `docs/job-contract.md:133` now names `grid judge rework`. The sentence at `:138` ("The review of a submitted Job and the rework after a rejection are posted by the app's Judge, not by the plugin") is pinned verbatim by `internal/job/lint/r23_judge_text_test.go:25`; changing it needs that existing test changed, which the rules forbid. A person must choose: update the pin, or keep the sentence |
| App check | HOLDS | `git -C /srv/gridwork/ref/app.git grep -n -E '"judge"\|never_judged' main -- '*realplugin*' '*.go'` printed nothing (exit status not captured) |
| Remove nothing; `never_judged` untouched | HOLDS | `git diff origin/main..HEAD --stat` shows additions only besides the two edited lines (help.golden, job-contract.md:133); write.go and the `never_judged` sentence are unchanged |
| Importers tested | HOLDS | `go list` importers of internal/judge and internal/cli: `cmd/grid`, `internal/cli`, `lib`; `go test -count=1 ./cmd/grid/ ./lib/` exit 0; internal/cli as above |
| `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| `make cross` (Windows and macOS) | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile |
| New tests fail on base | HOLDS | base: `internal/judge/pj_vb_helpers_test.go:12:18: undefined: State` ... `:59:40: undefined: str`; `--- FAIL: TestPjVbJudgeReviewSameWakePostsNothing` `pj_vb_judge_test.go:89: judge review J-4 --wake ...: exit 2: grid: unknown command "judge"`; `--- FAIL: TestPjVbJudgePersonForACancelledReviewIsRaisedOnce` `pj_vb_judge_test.go:137: ... unknown command "judge"` |
| New tests in new files, helpers prefixed `pjVb` | HOLDS | internal/judge/pj_vb_*_test.go, internal/cli/pj_vb_judge_test.go |
| No existing test edited to pass | HOLDS | the only existing test file touched is the annotation map of internal/cli/contract_inventory_test.go (four rows), a walk table the new route forces ("new parser route needs operand/output annotations"), as 43d26ad5 (grid run) and d296b09b (grid shot) did |
| No guard, gate, cap, hold or blocking hook | HOLDS | no limit on rounds: `--rounds` only prints beside `rounds_used`; nothing refuses a second review or rework |
| Rows wait on posts, never on a sleep | HOLDS | no sleep in the new code or tests |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` printed `0` |
| No sudo, no root, nothing outside /srv/gridwork, no claude/curl, no push or deploy | HOLDS | none run |
| Public repo: no personal paths, hosts or emails | HOLDS | the new files name only repo paths |

## What changed

- `internal/judge/` (new): `state.go` (State, chains, rounds, child and top ids, `Answered`), `id.go` (Post, request id, Control), `verdict.go` (Parse with STUCK, Report, Verdict), `review.go`, `rework.go`, `person.go`.
- `internal/cli/judge.go` (new): `grid judge verdict|review|rework|person`; posts go through `orchestrator.Write` under the task lease (GRID_TASK_EPOCH, else GRID_LEASE_EPOCH); a wake already answered, or the same post already on the Feed, prints "nothing posted" and exits 0.
- `internal/cli/cli.go` (command row), `internal/cli/inprocess.go` (`"judge": {}`), `testdata/golden/help.golden`, `contract/commands.json` (judge entry, hand-written to match the parser; the `UPDATE_CONTRACT_COMMANDS=1` run was refused by the shell), `internal/cli/contract_inventory_test.go` (four annotation rows), `docs/job-contract.md:133`.

## NOT VERIFIED and open items

- The four walk tables above (`gg_unknown_project_walk_test.go`, `gg_unknown_task_walk_test.go`, `guest_parity_test.go`, README `## Commands`) still need the judge rows; `go test ./internal/cli/` fails until they are added.
- `docs/job-contract.md:138` still says the app's Judge posts the review and the rework (see the NOT DONE row).
- Posts do not set the record's `attempt` header (Q2 says posts name the attempt).
- `grid judge rework` and `grid judge verdict` have no CLI end-to-end test; they are covered at the package level only.

NOT DONE
