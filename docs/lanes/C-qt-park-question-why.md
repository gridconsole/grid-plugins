# C-qt-park-question-why

Base check: on base 44c7de5b, internal/orchestrator/events.go:112 read `"why": strings.Join(m.Failures, ", ")`. It did not read the exit bodies, so the brief stands.

## Proof table

| Row | Result |
|---|---|
| 1. Walk: three worker-exits (code 1, code "claude not found", no code), three ticks; why is `Attempt 1: crashed, exit 1; Attempt 2: claude not found; Attempt 3: crashed, no exit code reported` | HOLDS. `go test -count=1 -v -run 'TestQtPk' ./internal/orchestrator/` exit 0, `--- PASS: TestQtPkWhyWalk`. Red on base (events.go stashed): `why "01M76P6F8T2C6VFWD1NPKN343Z, 01M76P6FBGD78C4T1Q3DR4TA37, 01M76P6FFD7VQ0VG2DMKV41JSH", want "Attempt 1: crashed, exit 1; ..."`. The question file's `why` is checked too. |
| 2. Direct park: worker-exit id, J-1 job post id, id not in Feed | HOLDS. `--- PASS: TestQtPkWhyDirect`, why `Attempt 1: crashed, exit 1; Attempt 2: the agent stopped answering; Attempt 3: r-missing`. Red on base: `why "01M76P6FN2REMGVC65464XJY2D, 01M76P6FKXAPVAAVJDW88VFV1H, r-missing"`. |
| 3. Question text, options, recommended, Parked with needs_person | HOLDS. qtPkCheckQuestion runs in both park tests (qt_park_why_test.go). Existing TestJrParkWritesQuestionFile and TestEventsTickPark pass unchanged. |
| 4. `go test -count=1 ./internal/orchestrator/ ./internal/contract/` | HOLDS, exit 0 (orchestrator ok 114s, contract ok). |
| 5. `make lint` / `make cross` | HOLDS. Both exit 0 (`make lint >/dev/null 2>&1 && echo`; cross vets and test-compiles darwin/arm64 and windows/amd64). |
| Extra: other words (loop, cancelled with code, missing reason, textual code) | HOLDS. `--- PASS: TestQtPkWhyReasonWords`. Wording for non-crash reasons with a code is `loop, exit 3` or `cancelled, <text code>`; the brief left that format open. |
| Importers of orchestrator (go list) | HOLDS. `internal/cli`, `internal/mcp`, `internal/migrate`: `go test -count=1` all ok (cli 50s, mcp, migrate). |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. App walk tests are not in this repo. |
| No existing test edited or deleted; no guard, gate or cap added | HOLDS. Diff touches events.go (one line), qt_failure_words.go (new), qt_park_why_test.go (new), this report. |
| RealPlugin tests | Not applicable: no row names one. |

## What changed
- `internal/orchestrator/events.go:112`: `why` is `qtPkWhy(snap, m.Failures)`.
- `internal/orchestrator/qt_failure_words.go`: `qtPkWhy` and `qtPkWords`. They read `snap.Records[id]` and reuse `exitCodeText` (write.go:154). A record that is not a worker-exit reads "the agent stopped answering". An id not in the snapshot stays as the id.
- `internal/orchestrator/qt_park_why_test.go`: `TestQtPkWhyWalk`, `TestQtPkWhyDirect`, `TestQtPkWhyReasonWords`.

## NOT VERIFIED on a live screen
The `#/inbox` question row's why line at 1440x900, against the design page (the Check section). The UI track shoots it after landing.

Verdict: DONE
