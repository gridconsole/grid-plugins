# M-plugin-held-f-e

Lands the held lanes F-cli-drop-stubs and E-receipts-e2e3 (with E-verdict-12-receipts beneath it) on main.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. Merge `origin/lane-F-cli-drop-stubs` | HOLDS | `git merge --no-edit origin/lane-F-cli-drop-stubs`: one conflict, `internal/cli/contract_inventory_test.go` (annotation map). Main had gofmt-realigned and edited the map (`lease take task`, `brain supersede`, no `usage threshold` rows); the lane only deleted the four `ports *` rows. Resolution: main's side kept whole, the four `ports claim/release/list/gc` rows removed. Merge commit a5b57fab. |
| 2. Regenerate the contract command golden | NOT DONE (command refused), and not needed | Exact command: `make generate` (its last line), or alone `UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1`. My sandbox refused it (env var prefix not on the allow list), so I did not run it or hand-edit `contract/commands.json` beyond what the lane's own commit changed. Evidence that no regen changes it: after both merges `go test -count=1 ./internal/cli -run 'TestContractCommands'` exit 0, `TestContractCommandsParserInventory` and `TestContractCommandsInventory` PASS, which compare the file with the generator's bytes. The orchestrator can run the command once at landing and expect no diff. |
| 3. Merge `origin/lane-E-receipts-e2e3` | HOLDS | Two conflicts. `internal/task/gates.go`: main still had `receiptForGate`, the lane deletes it; resolved by deleting it (main's added `gate != "plan"` refusal inside it goes with it, see note 3). `internal/task/write.go`: main moved the body of `Change` into `Store.change` (also used by `RunningFromQueued`); the lane edited the old inline body. Resolved by keeping main's `Change` calling `s.change` and applying the lane's edit inside `change` (the `reopen` block that rewrote approval.json is gone; the comment is the lane's). Merge commit a0d4e1c3. |
| 4. Both merges in the log | HOLDS | `git log --oneline main..HEAD` shows `a0d4e1c3 Merge ... lane-E-receipts-e2e3` and `a5b57fab Merge ... lane-F-cli-drop-stubs` (plus the lanes' commits and one fix commit). |
| 5a. `go test -count=1 ./...` | HOLDS | Run over all 66 packages, filtered with `grep -v "^ok\|no test files"`: no line printed, so no FAIL. (The pipe hides the exit code.) |
| 5b. `make lint` | HOLDS | exit 0 (`go vet ./...`, `gofmt -l .` empty). |
| 5c. `GOOS=windows go vet ./...` | HOLDS | exit 0. |
| 5d. `GOOS=darwin go vet ./...` | HOLDS | exit 0. |
| 6. Test changes explained | HOLDS | Only one: `internal/plan/store_items_test.go`, six calls `s.Migrate(x)` became `s.Migrate(x, nil)` (lines 215, 236, 248, 258, 272, 285). Main added that test file (lane-C-pcov-plan-store-items) against the old one-argument `Migrate`; the E lane changed the signature to `Migrate(yes, approved func() string)` (`internal/plan/migrate.go:12`, nil means no approval, documented there). The tests seed no receipt, so nil keeps their meaning. Commit "Pass no approved-hash reader...". |
| Rule: lane tests that fail on base | HOLDS | Both lanes' new tests were proved on their own bases in their reports (F: `TestRemovedVerbsAreUnknown`; E: compile failure on base). Rechecked on the merge: `TestRemovedVerbsAreUnknown`, `TestApproveStaleHashWritesNoJobResult`, `TestContractCommands*` PASS (`go test -count=1 ./internal/cli -run ... -v`). No new test is written here. |
| Rule: no guard/gate/cap/hold added | HOLDS | Nothing added. One refusal is removed with `receiptForGate` (note 3). |
| Rule: nothing outside the removal lists removed | HOLDS | Removed content is exactly the two lanes' removals. |
| Check: no approval.json outside migrate | HOLDS | `grep -rn "approval.json\|approvalFile" internal --include=*.go \| grep -v _test.go \| grep -v "^internal/migrate/"` prints nothing. |
| Rule: importers of changed packages | HOLDS | The whole module (66 packages from `go list ./...`) ran in 5a, which includes every importer. |
| Rule: small commits, no attribution lines | HOLDS | Merge commits and one one-sentence commit; no Co-Authored-By. |

## What changed

- Merge of F-cli-drop-stubs (verbs views, ask, machine, deploy, sync, ports removed; `env` kept) and E-receipts-e2e3 (approval only as the plan Job result; old approval.json moved into the Job by `migrate.ReceiptToJob`).
- Conflict fixes: `contract_inventory_test.go`, `internal/task/gates.go`, `internal/task/write.go`; stale-signature fix in `internal/plan/store_items_test.go`.

## Notes for the reviewer

1. Row 2 is NOT DONE as the brief anticipated: the command was refused. The golden file was changed only by the F lane's own commit; the generator test passes against it.
2. The E lane's contract change (`contract/schemas/answer.json`, two goldens) comes along: the cross-repo set at landing must run (app mirrors the contract). App follow-up `internal/slices/gates/model.go:477` reads approval.json (E report row 6).
3. Main had added a `gate != "plan"` refusal inside `receiptForGate` ("only the plan is approved"). The E lane removes that function whole, so the refusal goes with the fallback it guarded; the Job path never checked the gate name (E report open item 3: `TestWorkflowApproval` `{"ship","plan"}`). The test suite passes, so no test pins the old refusal. If it must stay, `ApprovalForGate` has to refuse `gate != "plan"`: a decision for the owner, not made here.

## NOT VERIFIED on a live screen

No UI row. The dev UI approve button against a real worker (approval shows at the holder's next tick) and the app's replay of the plan-digest transcript need a live check.
