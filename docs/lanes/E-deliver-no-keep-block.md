# Lane E-deliver-no-keep-block: deliver no longer waits for grid brain keep

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Delivery goes ahead while proposals wait; count is in the result; no refusal text | HOLDS | `internal/task/deliver.go` `check` returns the waiting list, `Deliver` puts it in `Delivery.Waiting` (json `waiting_proposals`); CLI prints `N brain proposals wait for the curator`. `grep -rn "Deliver blocked" internal` prints nothing (exit 1 = no match). |
| 2. Go test: two waiting proposals deliver and the result names both; fails before | HOLDS | `TestDeliverGoesAheadWithWaitingProposalsAndNamesThem` in `internal/task/deliver_waiting_test.go`; also `TestDeliverGoesAheadWhileProposalsWait` (cli). With the old refusal put back, both fail: `deliver_waiting_test.go:19: dry run refused: Deliver blocked: 2 brain proposals wait. Run: grid brain keep` and `delivery_test.go:176: dry run exit 1: deliver: Deliver blocked: 1 brain proposals wait. Run: grid brain keep` (exit 1). With the change: `go test ./internal/task/ ./internal/cli/ -run "Deliver\|Delivery"` exit 0. |
| 3. Tests that pinned the old refusal changed, each named | HOLDS | `internal/cli/delivery_test.go` `TestDeliveryCommandRefusals`: rows `pending`, `home-pending`, `dry-pending` removed; their behaviour is now covered by `TestDeliverGoesAheadWhileProposalsWait` (subtests `pending`, `home-pending`, with dry run and real run). The refusal rows `home-malformed` and `malformed-proposal` are unchanged and still pass. |
| 4. go test task + cli, make lint: 0 fail | HOLDS | Packages that import task or cli (go list): cmd/grid, braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, signal, task. `go test ./cmd/grid ./internal/braincmd ./internal/cli/... ./internal/guard ./internal/hook ./internal/job/... ./internal/land ./internal/signal ./internal/task/...` exit 0. `make lint` (go vet ./...) exit 0. |
| Windows build | HOLDS | `GOOS=windows go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin go build ./...` exit 0 |
| Small commits, one-sentence messages, no attribution lines | HOLDS | 3 commits on the lane (`git log origin/main..HEAD`); none carries Co-Authored-By. |
| No personal paths, hosts, emails in the public repo | HOLDS | Changed files hold none. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The change removes a refusal and adds none. |
| No existing test edited to pass | HOLDS | Only the three refusal rows above, which pinned the removed behaviour, were removed, with replacements named. |

## What changed

- `internal/task/deliver.go`: the refusal is gone. `check` returns the waiting proposals; `Delivery` gains `Waiting` (`waiting_proposals`, always an array); a dry run reports it too; `write` re-reads it under the locks.
- `internal/cli/deliver.go`: the stderr prefix `Deliver blocked:` is removed (it was added to every refusal), so refusals now read `deliver: <reason>`. Success prints the waiting count when above zero.
- `internal/cli/delivery_test.go`, `internal/task/deliver_waiting_test.go`: tests as above.
- `docs/delivery.md`: describes the new behaviour.

## Open items

- `contract/README.md:937-939` still says `grid deliver` prints `Deliver blocked: plan not approved ...` for `workflow_changed`. The prefix is now `deliver:`. That file is outside this brief's touches; the orchestrator should decide whether to change the text (the JSON envelope message there is not produced by `internal` code in this repo).
- A proposal that cannot be parsed (malformed file) still stops delivery with an error; the brief covered waiting proposals only.

## NOT VERIFIED on a live screen

- The orchestrator starting the curator from `waiting_proposals`; nothing here starts it.
