# C-hf-job-record-fields

Base: 91fadeff (lane-Pint99 merge; the brief quoted d1c76719). Read on the base:
docs/job-contract.md, contract/schemas/job-record.json (37 properties, 27
required), internal/job/job.go:179-180 (`PlannedFields`).

## Proof table

| Row | Result |
| --- | --- |
| 1. TestHfJrFields compares table and schema both ways, plus the required column | HOLDS: `go test ./internal/job -run TestHfJr -count=1 -v` exit 0, `--- PASS: TestHfJrFields`; `internal/job/hf_record_doc_test.go:88` |
| 2. TestHfJrPlanned compares the "Every entry supplies" names, in order, with `job.PlannedFields` | HOLDS: same run, `--- PASS: TestHfJrPlanned`. It also passes on base: the sentence already matched the code, so it is a guard, not a fix |
| 3. TestHfJrWriters: every written-by path exists | HOLDS: same run, `--- PASS: TestHfJrWriters` (os.Stat from the module root) |
| 4. "no round count" is gone or qualified with "plan" | HOLDS: docs/job-contract.md:14-18 now reads "A plan names no review level and no round count ... The record the run keeps does carry `review_rounds`"; `--- PASS: TestHfJrNoRoundCount` |
| 5. `go test ./internal/job -run TestHfJr -count=1` 0 fail; `make lint` exit 0; diff names two files | HOLDS (plus this proof file, which the runbook requires): test exit 0 (5 tests PASS); `make lint` (go vet ./...) exit 0; `git diff main --stat` shows docs/job-contract.md and internal/job/hf_record_doc_test.go only |
| New tests fail on base | HOLDS (3 of 5): with docs/job-contract.md stashed to base, `go test ./internal/job -run TestHfJr -count=1` exit 1: `TestHfJrFields: docs/job-contract.md has no section "Job record fields"`; `TestHfJrWriters: ... same`; `TestHfJrNoRoundCount: the sentence " A Job names no review level and no round count: ..." says "no round count" without saying it is the plan`. TestHfJrPlanned and TestHfJrKnownOpen pass on base by design |
| Tests of changed package and importers | HOLDS: `go test -count=1 ./internal/job/... ./contract/... ./internal/hook ./internal/jobgraph ./internal/lease ./internal/migrate ./internal/orchestrator ./internal/plan ./internal/ports ./internal/cli` exit 0, all ok. Importers of internal/job (from go list): contract/formats, internal/cli, hook, job/jobcmd, job/lint, jobgraph, lease, migrate, orchestrator, plan, ports |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| gofmt | HOLDS: `gofmt -l internal/job` prints nothing |
| Temp home, no clock, no sleep, no expected-fail test | HOLDS: every test calls `hfJrTempHome` (HOME and GRID_HOME to t.TempDir) |
| No guard, gate, cap or hold added; nothing removed | HOLDS: diff is a doc section, one reworded sentence and a test file |
| Tests in the one new file | HOLDS: only internal/job/hf_record_doc_test.go; helpers `hfJr*`, tests `TestHfJr*` |
| No attribution lines, no personal paths | HOLDS: one commit, one-sentence message, no trailer |
| *RealPlugin* tests | Not applicable: no row names one |

## What changed

- docs/job-contract.md: new section "Job record fields" (after "Planned fields")
  with one row per schema property (37), each with required/optional, writer
  (plan, run or none, with file paths) and one-line meaning. The sentence that
  said a Job has no round count now says a plan has none and points at
  `review_rounds`.
- internal/job/hf_record_doc_test.go: TestHfJrFields, TestHfJrPlanned,
  TestHfJrWriters, TestHfJrNoRoundCount, TestHfJrKnownOpen.

## Code that disagrees with the doc (no behaviour changed)

Kept in `hfJrKnownOpen` in the test, which fails when an entry is stale or when
a writer is missing from the list:

- Retired `max_rounds` (and `review` for five of them) is still written by
  internal/cli/task_new_first_job.go:32, internal/cli/deliver_keeper_job.go:71,
  internal/question/job.go:42, internal/orchestrator/replan.go:158,
  internal/approvefix/approvefix.go:123, internal/migrate/receipt.go:229. The doc says
  a Job declares no review level and that the field is retired
  (docs/job-contract.md "Dependencies and review", first paragraph).
- `review_rounds` counts worker exits with reason `loop`
  (internal/contract/fold.go:376), not review rounds; the name and the
  "round count" wording suggest otherwise. The doc table says what the code does.

Also noted, not wrong: `level_raised` and `review_override` have no non-null
writer or reader anywhere (the schema allows them); `created_at` is only read
(internal/orchestrator/ready.go:71). The table says `none` for `review_override`
and `created_at`.

## Not verified on a live screen

Nothing here is UI. contract/README.md:107 still points at job-contract.md:5-76
for the record; that file is outside Touches and its other line references were
already out of date, so it is left alone.

DONE
