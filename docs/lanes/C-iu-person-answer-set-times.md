# C-iu-person-answer-set-times

Head: see the report (the sha of the commit that adds this file).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 no sleep | HOLDS | `git grep -n 'time.Sleep' -- internal/orchestrator/person_lost_test.go internal/cli/person_lost_test.go` prints nothing. `go test ./internal/orchestrator ./internal/cli -run '^(TestSecondPersonAnswerIsNamedNotUsed\|TestSecondPersonSeesTheUnusedAnswerInFeedRead)$' -count=1 -v` exit 0: `--- PASS: TestSecondPersonAnswerIsNamedNotUsed (1.25s)` (1.25s is the tick, not a sleep) and `--- PASS: TestSecondPersonSeesTheUnusedAnswerInFeedRead (0.30s)`. |
| 1 scratch (sam earlier) | HOLDS | Orchestrator test, sam at t0, dana at t0+1s: `person_lost_test.go:108: want the first answer taken once, got []`, exit 1. CLI test, dana stamped one hour ahead of the real clock: `person_lost_test.go:80: feed read does not tell sam the answer was not used:` (plus feed dump), exit 1. Both reverted with `git checkout --`. |
| 2 same second, earlier ms | HOLDS | `TestIuPaSameSecondEarlierMillisecondWins` passes (`go test ./internal/orchestrator ./internal/cli -run ... -v`, exit 0, `--- PASS ... (0.47s)`). Scratch: `answers[a].At > answers[b].At` in person.go:77 gives `iu_person_answer_test.go:62: want dana's answer taken: sam answered J-1 at 2026-01-02T03:04:05.900Z: Call it Wren.`, exit 1. Reverted. |
| 3 same At, lower record | HOLDS | `TestIuPaSameTimeLowerRecordWins` passes (exit 0). It compares the two returned IDs and expects the author of the lower one. Scratch: `answers[a].ID > answers[b].ID` in person.go:79 gives `iu_person_answer_test.go:96: want dana's answer 01M757T8E2BE205FYH1M1ERF1K taken: sam answered J-1 at 2026-01-02T03:04:05.000Z: Call it Wren.`, exit 1. Reverted. This failure shows in the runs where the random part orders dana's ID lower; in the other runs the test fails for the opposite writer. It checks whichever ID is lower, so the inversion always breaks it. |
| 4 tests and lint | HOLDS | `go test ./internal/cli ./internal/orchestrator -count=1`: both ok. Importers (`go list` over Imports and TestImports): `cmd/grid`, `internal/cli`, `internal/migrate`, `internal/orchestrator`, `lib`; `go test ./cmd/grid ./internal/cli ./internal/migrate ./internal/orchestrator ./lib -count=1` exit 0 (cmd/grid has no test files). `make lint` (go vet ./...) exit 0. `gofmt -l` prints nothing. |
| 5 diff | HOLDS | `git diff main --stat` names `internal/cli/person_lost_test.go`, `internal/orchestrator/iu_person_answer_test.go`, `internal/orchestrator/person_lost_test.go`, plus this report. `git diff main --stat -- ':!*_test.go'` before this file was added printed nothing; no non-test code changed. |
| New tests fail on base | NOT APPLICABLE | The rule is unchanged, so the two new tests pass on base; they pin existing behaviour. The mutation runs above show each fails by name when the rule is inverted. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `GOOS=windows CGO_ENABLED=0 go vet ./internal/cli ./internal/orchestrator` exit 0. |
| *RealPlugin* tests | NOT APPLICABLE | No row names one. |
| No guard, no test deleted or skipped | HOLDS | No guard added. No test deleted or skipped. Only the sleep and its stale comment were removed from the two existing tests; their assertions are unchanged. |

## What changed

- `internal/orchestrator/person_lost_test.go`: dana's answer is written through a copy of `f.s.Feed` with `Now` = t0 (2026-01-02T03:04:05Z), sam's through a copy with `Now` = t0+1s. The sleep and the "stamped to the second" comment are gone.
- `internal/cli/person_lost_test.go`: dana's answer is written through `orchestrator.AnswerPerson` on a copy of the harness store with `Now` = real clock minus one minute. The writer is the one the CLI builds for an unleased member answer: `feedWriter(env, "", "")` at `internal/cli/orchestrator.go:295`, defined at `internal/cli/feed.go:337` (author dana, role member, worker `dana@<machine>`, no lease). sam's answer stays the CLI `job result`.
- New `internal/orchestrator/iu_person_answer_test.go`: `TestIuPaSameSecondEarlierMillisecondWins`, `TestIuPaSameTimeLowerRecordWins`, helpers `iuPaAnswer` and `iuPaNotices`.

## NOT VERIFIED on a live screen

Nothing here has a screen; all rows are proved by go test.
