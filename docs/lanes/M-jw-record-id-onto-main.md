# M-jw-record-id-onto-main

Carries C-jw-record-id-whole-feed onto plugin main 2ae36f4a (merged `origin/main`).

## Proof table

| Row | Result | Proof |
| --- | --- | --- |
| 1. Merge done | HOLDS | `git diff --name-only --diff-filter=U` prints nothing; `git merge-base --is-ancestor origin/main HEAD` exit 0; no conflict markers left in `contract` or `internal` |
| 2. Lane rows of C-jw-record-id-whole-feed (rerun) | HOLDS | `go test -count=1 -v -run 'TestJwRi\|TestHfFsCodeTable' ./internal/feed/ ./internal/cli/` exit 0: `--- PASS: TestJwRiOtherTypeRefused` (row 1, other type refused with `record_id_used`, Feed file byte-identical), `--- PASS: TestJwRiTenure` (row 2, same tenure same request returns the stored record, different request refused, other tenure refused `record_id_used`), `--- PASS: TestJwRiNewIDs` (row 3), `--- PASS: TestHfFsCodeTable` (row 5). Row 4: `go test -count=1 -run 'Feed\|Post\|Record' ./internal/cli/ ./internal/mcp/` exit 0 (`ok internal/cli 10.586s`, `ok internal/mcp 0.210s`). Base-red proof of row 1 is in docs/lanes/C-jw-record-id-whole-feed.md |
| 3. Step 4 | HOLDS | `go test -count=1 -race ./internal/feed/... ./internal/cli/...` exit 0: `ok internal/feed 82.624s`, `ok internal/cli 154.288s`. README check: `TestHfFsCodeTable` (internal/cli/hf_failure_shapes_test.go:189) passes |
| 4. `make cross`, `make lint` | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile); `make lint` (go vet ./...) exit 0 |
| 5. Lane doc | HOLDS | this file, section below |
| Importers of internal/feed tested | HOLDS | `go list` importers: approvefix, cli, hook, job/jobcmd, lease, mcp, migrate, orchestrator, question, task, usage. `go test -count=1` on each exit 0 (approvefix has no test files) |
| No test deleted, none skipped | HOLDS | `git diff origin/main --stat` lists only the new test file `internal/feed/jw_record_id_test.go`, store.go, error_codes.go, README, lane doc; no test file removed or edited; no skip used |
| No guard on agents, no sudo, no push | HOLDS | only a Feed record-id refusal in the store; nothing pushed |

## Codes kept from each side (internal/cli/error_codes.go)

From main (10): `no_such_project`, `no_such_task`, `no_such_note`, `invalid_argument`, `invalid_policy`, `workflow_file_changed` (these six were common to both sides), `note_state`, `note_exists`, `notes_full`, `guest_refused`.
From this lane (1): `record_id_used` (`CodeRecordIDUsed`, mapped from `*feed.RecordIDUsedError` in `errorCode`).
Eleven constants in total, each name and value once; no two codes share a value and no code has two values. `errorCode` keeps both main's `*signal.RefusedError` and `ilGuestRefused` cases and this lane's `*feed.RecordIDUsedError` case.

## What changed

- `internal/cli/error_codes.go`: conflict resolved by keeping both sides; `record_id_used` sits after `workflow_file_changed`, main's four new codes follow.
- `contract/README.md`: the code table lists the eleven `error_codes.go` codes in the file's order, then `person_required`, `job_waiting`, the settings codes and `index_rebuild_failed`; `guest_refused` moved up next to its siblings. The counting sentence now says "The first eleven constants" (main said ten).

## NOT VERIFIED on a live screen

Nothing here is a UI change; no live check needed.
