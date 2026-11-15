# C-fix-plugin-reject-caller-ref-body

A caller can no longer post a body that is only `{"ref":"bodies/..."}`. Before this lane the Feed recorded one with exit 0, then the read counted it Bad and dropped it without a word (r10 J-3-a1).

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. A bare ref body is refused when the record is written; only the store's own spill writes one | HOLDS | `internal/feed/store.go:208` refuses `bodyRef(r.Body)` before `validate`. The spill at `internal/feed/store.go:422` builds its wire body after that check, so it is the only writer of the form. The predicate `bodyRef` (`internal/feed/read.go:141`) is the same one the read uses to drop or expand a ref (`internal/feed/read.go:67`). |
| 1. Every entry point that records goes through the same path | HOLDS | `appendRecord` has one caller, `internal/feed/store.go:433`, inside `Store.Post`. CLI `grid feed post`: `internal/cli/feed.go:169`. CLI `grid job result`/`post`/`ack`: `internal/cli/orchestrator.go:400`, then `internal/orchestrator/write.go:97` (leased) or `internal/orchestrator/person_write.go:74` (person). MCP `feed_post`: `internal/mcp/tools.go:228`. Handover: `internal/feed/identity.go:221`. Worker: `internal/worker` imports neither `internal/feed` nor `internal/orchestrator` (`grep -rln 'internal/feed"\|internal/orchestrator"' internal/worker` prints nothing). Its agents post through the CLI above, and the worker-exit hook (`internal/hook/events.go:281`) writes session activity, not the Feed. |
| 2. Exit 3 with a message that says the body must be inline | HOLDS | The message is `$.body: must be inline; a body that is only {"ref":"bodies/..."} is written by the Feed itself` (`internal/feed/store.go:209`), sent as an `InvalidRecordError`. `TestJobResultRefusesBareRefBody` (`internal/cli/job_result_ref_body_test.go`) runs `grid job result J-1 --attempt 1 --body '{"ref":"bodies/<task ulid>.json"}'` and checks exit 3, `must be inline` on stderr, and that no record was added. |
| `grid job result` with a bare ref body exits 3 with the message | HOLDS | `go test -count=1 ./internal/feed ./internal/cli -run 'TestPostRefusesCallerRefBody\|TestJobResultRefusesBareRefBody' -v` exit 0: `--- PASS: TestJobResultRefusesBareRefBody (0.09s)` |
| A spilled body over 32 KB still round-trips | HOLDS | Same run: `--- PASS: TestPostRefusesCallerRefBody/spilled_body_round-trips`. The subtest posts a body of 2×MaxLine, retries with the same id, and reads it back counted with the full text. The existing `TestReadBodyRefs` is in the green `./internal/feed` run below. |
| The new test fails on main and passes on the lane | HOLDS | On main (production edits stashed, tests kept), exit 1: `reject_ref_body_test.go:30: post of a bare ref body: <nil>, want an invalid record that says the body must be inline` (another_id, own_id); `not_a_ulid` failed with the schema's pattern error, not the new message; `job_result_ref_body_test.go:34: bare ref body: exit 0, want 3:`. On the lane, exit 0: all four subtests and `TestJobResultRefusesBareRefBody` PASS. |
| `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`) |
| Tests of internal/feed, internal/contract and importers | HOLDS | The importer list comes from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E 'internal/(feed\|contract)( \|$)'`: contract, contract/formats, internal/{approvefix, artifact, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, ports, question, session, settings, signal, task, updates, usage, where, workflow}, lib. `go test -count=1` over all of them, exit 0, every package `ok` (approvefix and updates have no test files). |
| GOOS=windows build | HOLDS | `GOOS=windows go build ./...` exit 0 |
| GOOS=darwin build | HOLDS | `GOOS=darwin go build ./...` exit 0 |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0 |
| Merge of origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main` brought in nothing, so the head stayed on the lane commit. The runs above are on that head. |
| New tests fail on base, pass on lane; no existing test edited | HOLDS | Only new files: `internal/feed/reject_ref_body_test.go`, `internal/cli/job_result_ref_body_test.go`. No test was deleted. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change is a record-validation refusal of one malformed body form, the same kind as the schema's existing checks. It adds no hook or gate. |
| Touches | HOLDS | Production change only in `internal/feed`. `internal/contract` is unchanged: its one-key ref skip (`internal/contract/schema.go:412`) still has to accept the stored wire line the spill writes. The CLI test lives in `internal/cli`, which imports feed, because the acceptance row names `grid job result`. |
| Public repo hygiene | HOLDS | No personal paths, hosts or emails in the diff; no attribution lines in commits. |
| Cross-repo set, whole-module test | not run, not asked | The orchestrator runs these at landing. |
| RealPlugin tests | not run, not asked | The brief names none. |

## What changed

- `internal/feed/read.go`: `bodyRef(body)` is now one predicate. The read's ref check uses it, with the same behaviour as before.
- `internal/feed/store.go`: `Post` refuses a caller body that `bodyRef` matches, before validation and before any lock or write, with `InvalidRecordError` (exit 3) and the inline message.
- Tests: `TestPostRefusesCallerRefBody` covers a ref to another id, a ref to its own id and a ref that is not a ulid, plus the spill round trip. `TestJobResultRefusesBareRefBody` covers the CLI exit 3 and its message.

## NOT VERIFIED on a live screen

Nothing. The lane has no UI.

## Open items

- A record already on a Feed with a caller-written bare ref (for example 01M7EMG1KYRWJRTVY6F04CXKBJ from r10) is still counted Bad and dropped in silence on read. This lane only stops new ones. Showing the existing ones is a separate item.

DONE
