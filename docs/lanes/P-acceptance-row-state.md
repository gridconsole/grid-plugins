# P-acceptance-row-state: a running Job can mark its acceptance rows met or not met

Repo: plugin. Branch lane-P-acceptance-row-state. Merged origin/main before the last proof run (already up to date).

## The command a worker runs

When a worker proves a row, it marks it with the existing post command and the existing `checkpoint` type, under its Job lease:

```
grid job post --task <task> --job J-1 --type checkpoint --body '{"job":"J-1","acceptance":{"id":"A1","state":"met","note":"go test ./... exit 0"}}'
```

`state` is `met`, `not met` (also written `not-met`, lint's word) or `pending` (to take a mark back). `note` is optional. Dropping a row still goes through the result, with a `decision`.

## Example fold

A Job record with rows A1 to A5 (no states), then marks A1, A3 and A4 met during the run. `grid feed state --json` prints, under `jobs.J-1`:

```json
"acceptance": [
  {"id": "A1", "state": "met", "note": "go test exit 0"},
  {"id": "A2", "state": "pending"},
  {"id": "A3", "state": "met", "note": "go test exit 0"},
  {"id": "A4", "state": "met", "note": "go test exit 0"},
  {"id": "A5", "state": "pending"}
]
```

That is 3 of 5. The app counts `met` over rows that are not `dropped`.

## Design

- No new Feed type. `checkpoint` gets a third body branch, `{job, acceptance: {id, state, note?}}`, after the step branch (contract/schemas/checkpoint.json). The step branch and the ref branch are unchanged, so every old checkpoint still validates and folds as before. A new type would have changed the `grid feed types` list that internal/cli/contract_commands_test.go:389 pins.
- The fold (internal/contract/fold.go) seeds `Job.Acceptance` from the Job record's rows in record order. A row without a state reads `pending`; a row the record drops reads `dropped`. A row-mark checkpoint in the running tenure sets that row's state and note; the latest mark wins. A result's `acceptance` states overwrite the marks. A row-mark checkpoint does not touch `Job.Checkpoint`, so a later attempt still continues from the last step checkpoint. A mark of a row the record lacks gets the verdict `ignored` with the reason "Job has no acceptance row A9". This is the fold's normal verdict, not a refusal at the CLI. Like every Job-side post, a mark counts only inside a tenure that began with an ack.
- The fold writes `not-met` as `not met`, the result schema's word (internal/contract/types.go `rowState`).
- `grid feed state` output: `jobs.*.acceptance` is added to cmd-feed-state.json as an optional property (array of `{id, state, note?}` or null). It is not in `required`, so older output still validates.
- Orchestrator effect: a row-mark checkpoint is a checkpoint, so `recoverMemory` (internal/orchestrator/events.go:407) clears the crash-loop failure count on it, and the progress hash (events.go:67) changes, as it does for a step checkpoint. Proving a row is progress, so this was kept on purpose.
- `posts_back` is not enforced anywhere (grep shows only writers), so Jobs whose `posts_back` omits checkpoint can still mark rows.

## Proof table

| Row | State | Proof |
|---|---|---|
| A Job with 5 rows, 3 marked met mid-run, reads 3 of 5 from the Job read | HOLDS | `go test -count=1 -race -run TestArJobPostMarksRows -v ./internal/cli/` exit 0: `--- PASS: TestArJobPostMarksRowsFeedStateReadsThreeOfFive`. The test runs real `grid job next`, `grid job ack`, three `grid job post --type checkpoint`, then `grid feed state --json`; the output validates against cmd-feed-state. Fold level: `TestArFiveRowsThreeMetReadThreeOfFive` (internal/contract/ar_acceptance_rows_test.go), exit 0 |
| Old records unchanged | HOLDS | `TestArOldRecordReadsPending`: an old record with no states reads pending, and an old step checkpoint folds as before. The CLI test's J-2 (the fixture record) reads `A1 pending` before and after the marks. The old checkpoint fixtures `valid/basic.json`, `invalid/missing-step.json` and `invalid/unknown-body-field.json` are unchanged and still pass the fixture walk (`go test ./internal/contract/` exit 0). `TestArFiveRows…` asserts that the step checkpoint survives row marks |
| Fold keeps the latest state per row; result states overlay; unknown row ignored; mark needs the tenure | HOLDS | `TestArFiveRows…` (A4 not-met then met reads met), `TestArResultStatesAreLatest`, `TestArUnknownRowIgnored`, `TestArMarkNeedsTheTenure`, `TestArCheckpointRowSchema`: `go test -count=1 -race -run TestAr ./internal/contract/` exit 0 |
| Schema updated, additively | HOLDS | contract/schemas/checkpoint.json (third oneOf branch); contract/schemas/cmd-feed-state.json (optional `acceptance`). New fixtures: checkpoint/valid/acceptance-row.json, checkpoint/invalid/acceptance-row-dropped.json, cmd-feed-state/valid/acceptance.json. `go test ./contract/... ./internal/contract/` exit 0 |
| Lint updated | HOLDS (no change needed) | No new type was added, so no type lint changed. Job lint's state words (internal/job/lint/lint.go:237: pending, met, not-met, not met, dropped) are the words the mark accepts |
| run-job skill tells a worker to mark a row when it proves it (one sentence) | HOLDS | skills/run-job/SKILL.md step 7. Skill tests: `go test ./internal/contract/ ./internal/job/... ./internal/hook/` exit 0 |
| Doc with the worker's command and an example fold | HOLDS | this file; contract/docs/feed-posts.md checkpoint section |
| New tests fail on base | HOLDS | With main's fold.go, types.go and the two schemas checked out (`git checkout origin/main -- …`), `go test -run TestAr ./internal/contract/ ./internal/cli/` exit 1: `internal/contract/ar_acceptance_rows_test.go:37:19: undefined: AcceptanceRow`, `ar_acceptance_rows_test.go:74:26: j.Acceptance undefined (type *Job has no field or method Acceptance)`, `internal/cli/ar_job_row_mark_test.go:17:72: undefined: contract.AcceptanceRow`. Restored with `git checkout HEAD -- …`; `git status --short` clean |
| No existing test edited or deleted | HOLDS | The lane's commits (`git log origin/main..`) add two test files, internal/contract/ar_acceptance_rows_test.go and internal/cli/ar_job_row_mark_test.go, and touch no other test file. `git status --short` was clean after the base run |
| go test -race of touched packages | HOLDS | `go test -race -count=1` passed (`ok`) for ./contract, ./contract/formats and ./internal/contract (the only production Go package changed). The first run (all importers, -race, default 10m timeout) found a real gap: `TestSessionGolden` (internal/mcp) failed because feed_types prints the checkpoint schema. testdata/golden/mcp-session.golden now holds the new branch; this change forces that golden, so it counts as inside Touches. After that, `go test -race -count=1 ./internal/mcp/` passed (`ok 4.7s`) |
| go test -race ./internal/cli (whole package; it has only a new test file) | HOLDS | Run alone: `go test -race -count=1 -timeout 100m ./internal/cli/` exit 0, `ok internal/cli 1019.971s`. The two earlier timeouts happened when it ran beside orchestrator under -race |
| Tests of every importer (go list set below) | HOLDS | With -race and the default 10m timeout, `ok` for artifact, brain, braincmd, guard, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, migrate, ports, question, session, settings, signal, task, usage, where, workflow, lib; internal/updates has no tests. The second -race run (40m timeout) passed feed (`ok 533.8s`) and hook (`ok 62.3s`); in the first run hook had failed only on the timing line `hook SessionEnd slow: 3349 ms` under race load. When cli and orchestrator ran together under -race they timed out twice, each time in a different test that had run only for seconds (TestE1012…, TestTcPressLaterStep, TestGyGpAnswerPersonGuest), with no assertion failure. cli then passed alone under -race (row above). orchestrator, an importer that is not touched, was not rerun alone under -race. Without -race: `go test -count=1 -timeout 30m ./internal/cli/ ./internal/orchestrator/` exit 0 (`ok internal/cli 647.6s`, `ok internal/orchestrator 1390.8s`) |
| Plugin CLI tests (internal/cli) | HOLDS | in the rows above |
| make lint | HOLDS | `make -C . lint` (go vet ./...) exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| git fetch and merge origin/main before the last proof run | HOLDS | fetch and merge exit 0, already up to date; walk tests and race run after it |
| No guard, gate, cap, hold or blocking hook | HOLDS | none added. An unknown row is the fold's `ignored` verdict; the CLI refuses nothing new |
| Small commits, one sentence, no attribution lines | HOLDS | `git log origin/main..` |
| Whole-module test, cross-repo set | not run, not asked | the orchestrator runs them at landing |
| make realplugin / RealPlugin tests | not run, not asked | the brief names none for the plugin repo |

Importers of internal/contract (`go list -f '{{.ImportPath}} {{.Imports}} {{.TestImports}}' ./... | grep internal/contract`): internal/artifact, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, ports, question, session, settings, signal, task, updates, usage, where, workflow, lib; plus ./contract/... for the schemas. Importers of the root `contract` package (the embedded schemas), from the same `go list` with XTestImports: internal/braincmd, cli, contract, hook, job/lint, packaging. internal/packaging and the module root package (it embeds `skills/`) were not in the set above, so they were run as well: `go test -race -count=1 . ./internal/packaging/` gave `ok grid-plugins 1.574s` and `ok internal/packaging 141.072s`. Test files that name `run-job` or `feed-posts` (`grep -rlE 'run-job|feed-posts' --include=*_test.go .`) are all in internal/cli, internal/job/lint, internal/hook, internal/contract and ./contract, and all of these passed.

## What changed

- contract/schemas/checkpoint.json: row-mark body branch.
- contract/schemas/cmd-feed-state.json: optional `jobs.*.acceptance`.
- contract/fixtures: three new fixtures (above).
- contract/docs/feed-posts.md: checkpoint row-mark paragraph.
- internal/contract/types.go: `Job.Acceptance`, `AcceptanceRow`, row state words, `rowState`.
- internal/contract/fold.go: seeds rows from the Job record, applies row marks and result states, and `applySide` returns an ignore reason.
- skills/run-job/SKILL.md: one sentence in step 7.
- testdata/golden/mcp-session.golden: the checkpoint schema that feed_types prints now has the row-mark branch (forced by the schema change).
- Tests: internal/contract/ar_acceptance_rows_test.go, internal/cli/ar_job_row_mark_test.go.

## Open

- The app does not read `acceptance` yet: R5 "3 of 5 rows" needs an app lane that counts `met` over non-dropped rows from `grid feed state`.
- Row states carry across attempts. A re-assigned Job keeps the previous attempt's marks and result states until a worker marks the rows again.
- A result overwrites a row's note: it is empty for met and not met, and the decision id for dropped.
- How the app handles the new, always-present `acceptance` key was not checked. The cross-repo set at landing covers it.
- The `Read:` line refs in the checkpoint section of contract/docs/feed-posts.md predate this lane and no longer point at the right lines.
- internal/orchestrator was not run alone under -race; it passed without -race (`ok 1390.8s`).
- NOT VERIFIED on a live screen: nothing here has a screen; the app side is a later lane.

DONE
