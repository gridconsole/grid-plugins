# F-cli-keep-tests

Lane branch lane-F-cli-keep-tests, repo plugin. Adds tests only: `internal/cli/history_test.go` and `internal/cli/proof_test.go`. No change to `history.go` or `proof.go`; no bug found.

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. Subcommands and flags, file:line | HOLDS | history: `add` only, `internal/cli/history.go:16`; flags `--task` `:26`, `--json` `:27`; `--help`/`-h` `:11`. proof: `add` and `index`, `internal/cli/proof.go:16-17`; flags `--task` `:31`, `--json` `:32`; add-only flags `--text` `:35`, `--kind` `:36`, `--by` `:37`, `--note` `:38`; one `<file>` operand `:71-75`. Both are registered at `internal/cli/cli.go:120-121`. history has no read subcommand: lines are read back with `grid task history` (`internal/cli/task.go:58`). |
| 2a. history: append a line | HOLDS | `TestHistoryAddAppendsLine` (plain and `--json`), `go test ./internal/cli/ -run 'TestHistory|TestProof'` exit 0 |
| 2b. history: read lines back | HOLDS | `TestHistoryAddReadBack` reads history.md and `grid task history --json`; same run, exit 0 |
| 2c. history: task with no history.md | HOLDS | `TestHistoryAddWithoutHistoryFile`; same run, exit 0 |
| 2d. history: guest call | HOLDS | `TestHistoryAddUsageAndGuest`: guest gets exit 1 "task not found", history.md unchanged; also usage errors exit 2 |
| 3a. proof: add for a plan item | HOLDS | `TestProofAddText` (text, JSON and plain), `TestProofAddFile` (relative file, screenshot) |
| 3b. proof: list proofs | HOLDS | `TestProofIndexLists` (plain before/after add, `--json` items and evidence) |
| 3c. proof: missing file | HOLDS | `TestProofMissingFile` (missing file, unknown item, no evidence; no proof.json left) |
| 3d. proof: guest call | HOLDS | `TestProofGuestCall`: add and index both exit 1 "task not found"; no proof folder created. `TestProofUsage` covers exit 2 |
| 4. `go test ./internal/cli/...` and `make lint`: 0 fail | HOLDS | `go test ./internal/cli/... ./cmd/grid/...` exit 0 (`ok internal/cli 90.093s`, cmd/grid has no test files); `make lint` (`go vet ./...`) exit 0 |
| New tests fail on base, pass on lane | HOLDS (by mutation) | The tests cover existing behaviour, so they pass on base too; nothing was missing. To show they can fail, I temporarily changed history.go (wrote "x" instead of the line) and proof.go (index always "missing"), then restored with `git checkout`. Failing lines: `history_test.go:42: history.md lacks the line:`, `history_test.go:70: lines out of order or missing:`, `history_test.go:91: history.md after first add:`, `proof_test.go:80: after add "i-1: check proof missing..." want "i-1: check proof saved..."`. Tree clean afterwards (`git status --short` empty). |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat` shows only the two new files |
| No guard, gate, cap, hold or blocking hook added | HOLDS | tests only |
| Packages importing internal/cli | HOLDS | `go list` importers: `cmd/grid` only; its test run is in the exit-0 run above |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Small commits, one sentence, no attribution lines, no personal paths | HOLDS | `490c11de Add tests for the history and proof verbs.` |

## What changed

- `internal/cli/history_test.go`: four tests for `history add`.
- `internal/cli/proof_test.go`: six tests for `proof add` and `proof index`.

Note on the brief: history.go and proof.go were not untested. `TestHistoryCommandTable`, `TestProofCommandTable` and `TestRemainingTaskOperands` already cover usage errors and task operands. The new tests cover the content written (history.md, proof.json, evidence files), the read-back, and the guest refusal.

## NOT VERIFIED on a live screen

Nothing here is UI. That the task page and the Result tab show the history lines and proof written by these verbs is not checked here.
