# C-jw-record-id-whole-feed: a record id is refused when any record on the Feed already has it

Base: main 750ac8de; merged origin/main 17efd4a3 before the last proof run.

## Base check

On the base, the reuse check did not cover the whole Feed:
- The tenure scan at store.go:276-299 reads only the writer's own tenure file. The comment at :268-270 says "the whole Feed is read only by the checks that depend on other tenures".
- The whole-Feed id check at :300-309 ran only when `r.Wake != nil || r.Type == "ack" || guestAnswer`.
- So a plain post, or a record of another type, from another tenure with an id already on the Feed was accepted. That shows as red on the base in row 1. The lane was needed.

## Proof table

| Row / rule | Status | Proof |
| --- | --- | --- |
| 1. A record of another type with an existing id is refused with record_id_used, and the Feed file is byte-identical | HOLDS | `TestJwRiOtherTypeRefused`, internal/feed/jw_record_id_test.go:50. It posts a `post`, then a `knowledge` record with the same id from another tenure. It checks `errors.As(*RecordIDUsedError)`, that the first tenure file keeps its exact bytes, that the refusing tenure file is empty, and that one record is read. Red on the base behaviour (type present, check absent), exit 1: `jw_record_id_test.go:57: second record with id 01M75C2KEKYANE19QMXYBT6YNN accepted as knowledge 01M75C2KEKYANE19QMXYBT6YNN`. Green: `--- PASS: TestJwRiOtherTypeRefused`. |
| 2. Same tenure and same request returns the stored record (exit 0). Same tenure and a different request gives the :287 text. Another tenure with the same request gives record_id_used, exit 3 | HOLDS | `TestJwRiTenure`, jw_record_id_test.go:81. The retry returns the same ID and Seq with a nil error. The different request gets `ErrInvalidRecord` with text `record id already used for a different post` (store.go:296, the old :287) and is not a RecordIDUsedError. The other tenure gets RecordIDUsedError inside InvalidRecordError, so `errors.Is(ErrInvalidRecord)` holds and exitCode returns 3 (internal/cli/exitcode.go:17, exitcode_test.go:20). Red on base, exit 1: `jw_record_id_test.go:103: want RecordIDUsedError for 01M75C2KF4SJ43KD94N48CTQGM, got <nil>`. Green: `--- PASS: TestJwRiTenure`. The code mapping is in internal/cli/error_codes.go:41-46. |
| 3. New ids are accepted; existing reuse tests are unchanged | HOLDS | `TestJwRiNewIDs` passes. No existing test was edited. `TestInvalidRecordRejections` ("same tenure id", "other tenure id") and `TestRecordExitFeedAdditionalRejections` / `TestRecordExitDuplicateWake` pass in `go test -count=1 ./internal/feed/` (exit 0) and in the full `./internal/cli/` run (exit 0). |
| 4. feed and cli/mcp tests, lint, cross, diff stat | HOLDS | `go test -count=1 ./internal/feed/`: `ok ... internal/feed 26.601s`, exit 0. After the merge, `go test -count=1 -v -run "Feed\|Post\|Record\|TestHfFsCodeTable\|TestJwRi" ./internal/feed/ ./internal/cli/ ./internal/mcp/`: all ok, with `--- PASS: TestJwRiOtherTypeRefused`, `--- PASS: TestJwRiTenure` and `--- PASS: TestJwRiNewIDs`, exit 0. `make lint` exit 0 (go vet; gofmt clean after commit 2). `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). `git diff origin/main --stat` lists only contract/README.md, internal/cli/error_codes.go, internal/feed/jw_record_id_test.go and internal/feed/store.go. This report is the one file added outside Touches, which the runbook requires. |
| 5. Merge origin/main; the table holds every code once plus record_id_used; the counting sentence matches; TestHfFsCodeTable passes | HOLDS for the current main, open for M-fix | `git fetch origin` and `git merge origin/main` merged main **17efd4a3** cleanly. `--- PASS: TestHfFsCodeTable (0.19s)`. README:85-86 names the seven constants of error_codes.go. **M-fix-js-iy-if-onto-main has not landed**: `git merge-base --is-ancestor origin/lane-M-fix-js-iy-if-onto-main origin/main` gives not-landed. See the open items for the conflict it will raise. |
| Rule: tests of changed packages and their importers | HOLDS | Importers from `go list -f "{{.ImportPath}} {{join .Imports \" \"}}" ./... \| grep internal/(feed\|cli)`: cmd/grid, internal/approvefix, internal/cli, internal/hook, internal/job/jobcmd, internal/lease, internal/mcp, internal/migrate, internal/orchestrator, internal/question, internal/task, internal/usage, lib. `go test -count=1` over all of them: every package ok (cmd/grid and approvefix have no test files), exit 0 for those packages. I also listed internal/kernel/worker in the same command by mistake; it is an app-repo path, so that line was `setup failed`, not a test failure. |
| Rule: new tests fail on base, pass on lane | HOLDS | See the red lines in rows 1 and 2. They were run with the type added and the check absent, because the test refers to the new type. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | The change is a record-id refusal in Post with no new lock and no hold. Every caller keeps its retry; the refusal is exit 3 as before. |
| Rule: small commits, no attribution, no personal paths | HOLDS | `git log origin/main..HEAD`: 3 one-sentence commits plus the merge, with no trailers. |
| Rule: remove nothing / do not edit existing tests | HOLDS | Diff: 1 line removed (the README sentence that was rewritten). No test file was changed except the new one. |

## What changed

- internal/feed/store.go:23-29 adds `RecordIDUsedError{ID}`. Its text is the existing reuse text, because two existing tests pin that text for the other-tenure wake case.
- internal/feed/store.go:309-319: after the tenure scan, Post reads the Feed with `ReadOnly(Filter{})`. If any record already has the id, Post returns `&InvalidRecordError{Err: &RecordIDUsedError{}}` before anything is written. A same-tenure repeat still returns the stored record at :290-303, and a same-tenure different request keeps its :296 text with no code.
- internal/cli/error_codes.go adds `CodeRecordIDUsed = "record_id_used"` and maps it with `errors.As(err, &used)`.
- contract/README.md adds the `record_id_used` row and makes the constants sentence name all seven constants.
- internal/feed/jw_record_id_test.go is the new test file: tests `TestJwRi*`, helpers `jwRi*`.

## Open items

- **Conflict when M-fix-js-iy-if-onto-main lands.** Both lanes edit the const block after `CodeWorkflowFileChanged` in error_codes.go and the "first six constants" sentence in the README.
  - Resolve by keeping both const sets and both errorCode cases.
  - Keep every table row of both sides.
  - Write the sentence as "The first ten constants and `record_id_used` live in `internal/cli/error_codes.go`, the settings codes in `internal/settings/claude_user.go` and `index_rebuild_failed` in `internal/braincmd/changes.go`; ...". That makes eleven constants.
  - Then run `TestHfFsCodeTable`.
- **The wake, ack and guest paths now get record_id_used too.** For an other-tenure id clash on these paths, the new check runs first, so the :329 refusal (old :309) is now reached only in a race. The text is unchanged; stdout now carries the code under `--json`.
- **Concurrent posts can race.** Two tenures posting the same caller-chosen id at the same moment can both pass. The check reads the Feed under the writer's own tenure lock only, with no Feed-wide lock, as the wake/ack check did before. NewID ULIDs make this matter only for `--id` callers. C-jw-feed-lock-context, which lands behind this lane, is where a Feed-wide lock would go.
- **Every Post now reads the Feed.** This uses the reader cache read-only. It is a performance cost and was not measured.
- **CLI envelope not tested end to end.** No test runs `grid feed post --json` through the CLI to show `"code":"record_id_used"` in the envelope. The new tests sit in package feed, as Touches requires. The mapping is error_codes.go:41-46, reached through feed.go:89 `env.recordError`.

## NOT VERIFIED

Nothing in this lane needs a live screen.

DONE
