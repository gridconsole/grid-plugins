# C-ln2-env-cli-forced-rows

Adds the walk-table rows that `grid project env` forces, exactly as docs/lanes/C-ln-plugin-env-cli.md gives them.

## Proof table

| Row | Claim | Result | Evidence |
|---|---|---|---|
| 1 | The three named tests pass | HOLDS | `go test -count=1 -run 'TestContractCommandsParserInventory\|TestGuestProjectParity\|TestExHelpSummariesNameEveryVerb' -v ./internal/cli/` exit 0: `--- PASS: TestContractCommandsParserInventory (1.98s)`, `--- PASS: TestExHelpSummariesNameEveryVerb (0.00s)`, `--- PASS: TestGuestProjectParity (2.34s)`, `ok internal/cli 4.344s` |
| 1b | The new rows fail without the patch | HOLDS | With the rows stashed: `contract_inventory_test.go:384: new parser route "project env" needs operand/output annotations` and `guest_parity_test.go:361: missing guest parity command: project env` |
| 2 | `go test -count=1 ./internal/cli/ ./internal/brain/ ./internal/projectenv/` | HOLDS | exit 0: `ok internal/cli 100.061s`, `ok internal/brain 2.091s`, `ok internal/projectenv 0.094s` |
| 3 | Plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok 0.150s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok 0.642s`) |
| 4 | `make lint`, `make cross` | HOLDS | both exit 0 (go vet; darwin/arm64 and windows/amd64 vet and test compile) |
| 5 | Diff names only Touches files | HOLDS | `git diff origin/lane-C-ln-plugin-env-cli --stat`: internal/cli/contract_inventory_test.go (+1), internal/cli/guest_parity_test.go (+2 -1), plus this doc |
| R-imp | Importers of the changed packages | HOLDS | Only `_test.go` files in internal/cli changed, and a `_test.go` file is not compiled into any importer, so no importer is affected. The orchestrator runs the whole module at landing |
| R-nw | No existing row changed, no product code | HOLDS | The diff adds three rows; guest_parity_test.go:427 gets one more name in an existing case list |
| R-g | No guard, gate, cap, hold or blocking hook | HOLDS | None added |

No cli.go:161 or golden change was needed; the help line is already on the base lane.

## What changed

- internal/cli/contract_inventory_test.go: annotation row `"project env"`.
- internal/cli/guest_parity_test.go: `"project env": {"list", "{project}"}` in the parity commands, and `"project env"` in the guest-refused case list next to `"project repo"`.

## Note on C-ln row 3

The C-ln doc's row 3 wording is off: on main, EditConfig stamps kind `config`, not `session`.

## NOT VERIFIED

Nothing needs a live screen.
