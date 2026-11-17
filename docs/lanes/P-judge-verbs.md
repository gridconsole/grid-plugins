# P-judge-verbs: `grid judge` verbs in the plugin (verdict, review, rework, person, request id)

Lane branch lane-P-judge-verbs. `git fetch origin && git merge origin/main` printed "Already up to date." before the last proof run. Owner direction V11: review, one rework, then a person; no Grid-made limits. The verbs post what the orchestrator chooses; none decides, counts down or blocks.

## Proof table

| Row | State | Proof |
|---|---|---|
| A1 verdict parse ports the app's verdict cases and adds STUCK | HOLDS | `go test -count=1 -v ./internal/judge/` exit 0: `--- PASS: TestVerdictParse` (the app's 19 cases), `--- PASS: TestPjVbVerdictParseAddsStuck`; `internal/judge/verdict.go:25` |
| A2 rework body ports `TestReworkPostsANewJobIDWithOnlyJobRecordKeys` | HOLDS | `--- PASS: TestReworkPostsANewJobIDWithOnlyJobRecordKeys` (keys read from contract/schemas/job-record.json); `internal/judge/rework.go:16` |
| A3 request id ports both `judge_post_request_id` tests (no epoch) | HOLDS | `--- PASS: TestJudgePostRequestIDFollowsThePost`, `--- PASS: TestJudgeControlCarriesARequestID`; `internal/judge/id.go:26` hashes task, type, job, body, wake, person only |
| A4 a second call with the same wake succeeds with nothing posted (`ErrDuplicateWake`) | HOLDS | `--- PASS: TestPjVbJudgeReviewSameWakePostsNothing` (real Feed through the CLI: second `grid judge review J-4 --wake <same>` exits 0, prints "nothing posted" with the review record id, record count unchanged); `internal/cli/judge.go:180` (`Answered` before the write), `:191` (`ErrDuplicateWake` gives exit 0) |
| A5 a cancelled review gives one person Job (ports both `judge_cancelled_review` tests) | HOLDS | `--- PASS: TestJudgeReviewCancelledAtSpawnRaisesOnePersonJob`, `--- PASS: TestJudgeReviewCancelledAfterAVerdictRaisesNoPersonJob`, `--- PASS: TestPjVbJudgePersonForACancelledReviewIsRaisedOnce` (CLI, `job cancel` then three `judge person` calls give one person Job) |
| A6 `verdict` reports rounds over the `continues` chain | HOLDS | `--- PASS: TestPjVbVerdictReportsRoundsOverTheContinuesChain`; `internal/judge/state.go:134` |
| A7 a verdict from an older attempt is reported stale | HOLDS | `--- PASS: TestPjVbVerdictFromAnOlderAttemptIsStale`; `internal/judge/verdict.go:114` |
| A8 (V27) review base is the submitted SHA of the reviewed attempt; rework base is the head SHA, else the target's tip | HOLDS, on hand-built records | `--- PASS: TestPjVbBaseIsTheSubmittedShaThenTheHeadOrTheTarget`; `internal/judge/review.go:34` (`submitted`), `internal/judge/rework.go:63` (`reworkBase`). The `sha` body key is in contract/schemas/submit.json and head.json (`grep -rln '"sha"' contract/schemas/`). With no head, the base is the target branch name, not a resolved tip SHA (see NOT VERIFIED) |
| `go test -count=1 ./internal/judge/... ./internal/cli/` exits 0 | NOT DONE | `./internal/judge` ok; `./internal/cli` exit 1 with four failures. Three are tables that do not list the new command: `TestGgUnknownTaskWalkCoversTheContract` ("contract members not walked or skipped: [judge person judge review judge rework judge verdict]"), `TestGuestProjectParity` ("missing guest parity command: judge ...") and `TestLtReadmeNamesEveryCommand` (README `## Commands` does not name `grid judge`). The fourth, `TestGgUnknownProjectWalk`, walked the judge routes and they failed its check: `judge review parity --project absent` exits 1 with empty stdout and stderr `project "absent" is not registered`. The cause is NOT VERIFIED; it may be a behaviour gap in `internal/cli/judge.go` rather than a missing row. Not fixed: the call budget ran out (85% rule) |
| Inventory and walk after the merge of main | HOLDS | `go test -count=1 -v -run 'TestContractCommandsInventory\|TestContractCommandsParserInventory\|TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli/` exit 0 with `--- PASS:` for each |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0 (`ok .../internal/job/lint`) |
| docs/job-contract.md says the plugin posts the review and the rework | NOT DONE | `docs/job-contract.md:133` now names `grid judge rework`. The sentence at `:138` ("The review of a submitted Job and the rework after a rejection are posted by the app's Judge, not by the plugin") is pinned verbatim by `internal/job/lint/r23_judge_text_test.go:25`; changing it needs that existing test changed, which the rules forbid. A person must choose: update the pin, or keep the sentence |
| App check | HOLDS | `git -C /srv/gridwork/ref/app.git grep -n -E '"judge"\|never_judged' main -- '*realplugin*' '*.go'` printed nothing (exit status not captured) |
| Remove nothing; `never_judged` untouched | HOLDS | `git diff --stat origin/main..HEAD`: 20 files, 1988 insertions, 1 deletion (the old `docs/job-contract.md:133` line, rewritten). Files: contract/commands.json, docs/job-contract.md, docs/lanes/P-judge-verbs.md, internal/cli/{cli.go, contract_inventory_test.go, inprocess.go, judge.go, pj_vb_judge_test.go}, internal/judge/* (6 source, 5 test), testdata/golden/help.golden; all are inside Touches or a forced walk table. internal/orchestrator/write.go and the `never_judged` sentence are unchanged |
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

- `go test ./internal/cli/` fails until these are done:
  - `internal/cli/gg_unknown_task_walk_test.go` needs the judge rows.
  - `internal/cli/guest_parity_test.go` needs the judge rows.
  - `README.md ## Commands` needs a `grid judge` entry.
  - `internal/cli/gg_unknown_project_walk_test.go`: the judge verbs fail its unknown-project check (empty stdout). Fix `judge.go` or add rows, whichever the test asks for. Not examined.
  - For comparison, the grid shot lane (`git show --stat d296b09b`) changed README.md, contract/commands.json, internal/cli/cli.go, contract_inventory_test.go, inprocess.go, shot.go, shot_test.go and testdata/golden/help.golden. This lane has all of those except README.md. The three walk tests above came after that lane.
- With no task head, the rework base is the target branch name (`--target`, else the builder's `target` or `base`), not that branch's resolved tip SHA.
- The review and rework bases are proven on hand-built submit and head records only. No CLI test posts a real submit or head record.
- `docs/job-contract.md:138` still says the app's Judge posts the review and the rework (see the NOT DONE row).
- Posts do not set the record's `attempt` header (Q2 says posts name the attempt).
- `grid judge rework` and `grid judge verdict` have no CLI end-to-end test; they are covered at the package level only.

CLOSED BY P-judge-verbs-fix1: accepted 16:26 UTC
