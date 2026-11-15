# P-r23-docs-delivery

Docs and topics match the delivery flow. Rows: U-chk-plugin-rev-cli row 5; U-chk-plugin-rev-skills rows 7 and 8.

## Proof table

| Row | Result |
|---|---|
| cli 5 confirmed | HOLDS. `grid approve --help` lists `--plan-hash --note --send-back --json`, no `--gate`. No preview command exists (preview.go removed in a908a013). docs/delivery.md:149-201 on main named `grid preview`, `--gate`, `approval-ship.json`, `gate: ship`. |
| cli 5 fixed | HOLDS. docs/delivery.md: the ship, preview and "approve step a person saw" sections are replaced by "Approval after delivery" (one approval, the plan Job; delivery posts a ship person Job, `person_job` in `--json`, internal/cli/deliver.go:106-114); the write no longer says `gate: ship` (internal/task/deliver.go:251 writes none). |
| skills 7 confirmed | HOLDS. topics/delivering and topics/reviewing line 9 said "Read by the hooks when the ... skill starts"; topics/README.md:7 says no hook does. |
| skills 7 fixed | HOLDS. Both TOPIC.md now say they are reached through the SessionStart index and `grid brain search --topic`. |
| skills 8 confirmed | HOLDS. topics/delivering said proposals "block delivery" and grid deliver "refuses (waiting proposals"; docs/delivery.md and internal/task/deliver.go:41 say a waiting proposal never stops a delivery. |
| skills 8 fixed | HOLDS. topics/delivering description and pointers say a waiting proposal gets a curator Job and does not block `grid deliver`. |
| Test per fixed row | `TestR23DeliveryDocNamesNoRemovedCommand` (internal/cli/r23_docs_delivery_test.go) and `TestR23DeliveringTopicsMatchDelivery`. On base (doc and topics stashed): FAIL, e.g. `docs/delivery.md still names "grid preview"`, `still names "--gate"`, `topics/delivering/TOPIC.md says a hook reads it; topics/README.md says none does`, `still says "block delivery"`. On the head: `ok github.com/gridconsole/grid-plugins/internal/cli`. |
| Go tests changed packages | HOLDS. `go test -count=1 ./internal/cli` `ok ... 323.220s`; importers from `go list` (cmd/grid, lib): `./cmd/grid` no test files, `./lib` `ok 98.430s`; also `./internal/packaging ./internal/brain ./internal/job/lint/` ok. |
| `make lint` | HOLDS. Output `go vet ./...`, no error lines, exit 0. |
| Walk tests | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok (run with the R23 tests, `ok ... internal/cli`). `./internal/job/lint/` ok. |
| Windows and macOS builds | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin ...` both exit 0, no output. |
| Merge of origin/main before last proof | HOLDS. `git fetch origin && git merge origin/main`: "Already up to date." |

## What changed

docs/delivery.md, topics/delivering/TOPIC.md, topics/reviewing/TOPIC.md, and the new test file. No code change, no guard added.

## NOT VERIFIED on a live screen

Nothing here needs a screen.
