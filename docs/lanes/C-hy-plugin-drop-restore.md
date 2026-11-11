# C-hy-plugin-drop-restore: lane report (fix round 1)

Base: main 91fadeff (`drop` and `restore` were not in command.go there). This round adds what fix round 1 listed as missing: the CLI verbs, `Env.Member`, the shared tables, the goldens, the README, `make generate`, and the three braincmd tests. The store layer (internal/brain/hy_drop.go, Store.Drop, the read paths, the job source) is from the first round and is unchanged.

## Proof table

| Row / rule | Result |
|---|---|
| 1. TestHyDrDropAndRestore (internal/braincmd/hy_drop_test.go, through `cli.Run` with GRID_MEMBER `ana`, GRID_ROLE `member`) | HOLDS. `go test -count=1 -run HyDr ./internal/braincmd/` exit 0. Drop exits 0; note bytes and versions folder are equal; search and the index entry count drop it; `show --json` has `"dropped":true`; `changes --json` has `dropped` then `restored` with origin `person:ana`. Origin: GRID_WHO `bo` beside GRID_MEMBER `ana` gives `person:bo`; GRID_ACTOR `cy` beside MEMBER gives `person:cy`; MEMBER alone gives `person:ana`; all three empty gives kind `session`. `--by ana` exits 2. |
| 2. TestHyDrRoles (both verbs x owner, member, empty, guest) | HOLDS. Same command, exit 0. owner, member and empty exit 0; guest exits 3, stdout empty, stderr exactly `grid brain drop: caller role is not allowed to drop brain entries` (restore: `... to restore brain entries`), log.md byte-equal. |
| 3. TestHyDrRefusals (18 cases) | HOLDS. Same command, exit 0. Stale rev exits 6 with stderr `{"error":"conflict","rev":"<current>"}` (drop and restore); missing note exits 8 `{"error":"no_such_note"}`; a dropped entry, restore of a never-dropped one, a superseded one, `machines` and `skills` exit 9 (`{"error":"refused","reason":...}` where the reason is checked); `../x`, no operand, no `--expect`, `--expect xyz` and a short hex exit 2 with the usage line; an unregistered `--project` exits 1. Each case compares the whole Grid home (notes, versions, log.md, index) before and after: no write. |
| 4. TestHyDrConcurrent (internal/brain/hy_drop_test.go) | HOLDS (first round, unchanged). `go test -count=1 ./internal/brain/` ok. Two `DropWith` calls behind a channel barrier: one nil, one `ErrDropRefused`, one new `dropped` line. |
| 4. TestHyDrJobSource (internal/job/hy_drop_source_test.go) | HOLDS (first round, unchanged). `go test -count=1 ./internal/job/` ok. Resolve on a dropped note fails with `note is dropped`, works after restore. |
| 5. `make generate` twice, `git diff --exit-code contract/commands.json` | HOLDS. `make generate` exit 0 three times; after the last, `git diff --exit-code contract/commands.json` exit 0 and `git status --short` empty. commands.json gained `drop` and `restore` with `--expect` required. |
| 5. `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/job/ ./internal/cli/` | HOLDS, exit 0 (brain, braincmd, job, cli ok). Verbose: TestContractCommandsParserInventory, TestGuestProjectParity (brain_drop and brain_restore, plain and json, both projects), TestGgUnknownProjectWalk, TestGgUnknownProjectWalkCoversTheContract, TestExHelpLinesFitOneLine, TestExHelpSummariesNameEveryVerb PASS. TestRemainingTaskOperandHelp passes in the full cli package run (the verbose filter cut its line off; the package is ok). |
| 5. `make golden`, `make lint`, `make cross` | HOLDS. `make golden` exit 0 (`ok internal/golden`). `make lint` exit 0 after gofmt of my test file (the first run failed on gofmt, fixed in its own commit). `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile. |
| Importers of the changed packages | HOLDS. `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep -E 'internal/(brain\|braincmd\|job\|cli)( \|$)'` lists cmd/grid, braincmd, cli, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, lease, migrate, orchestrator, people, ports, routine, task, workflow, lib. `go test -count=1` on all of them exit 0, all ok (cli 98s, orchestrator 116s). |
| New tests fail on base | HOLDS. The three braincmd tests run in a worktree of 91fadeff fail: `TestHyDrDropAndRestore: hy_drop_test.go:183: drop: 2 brain: unknown command "drop". Run: grid brain help`; `TestHyDrRoles/drop/role=owner: exit 2, stderr "brain: unknown command \"drop\"..."`; `TestHyDrRefusals/stale_rev: exit 2, want 6`; `.../no_such_note: exit 2, want 8`. The store tests do not compile on base (`DropWith` is missing). |
| No existing test edited or deleted | HOLDS. The only edits in existing tests are the two rows per verb in contract_inventory_test.go and guest_parity_test.go that the brief names. |
| No guard, gate, cap or hold on agents | HOLDS. The role check is the owner's decision (`brain.RevertAllowed`, as `revert`); nothing else blocks. |
| Exit codes | HOLDS. internal/braincmd/hy_drop.go returns 0, 1, 2, 3, 6, 8, 9 only; never 4, 5 or 7. |
| Commits | One sentence each, no attribution lines, no personal paths, hosts or emails. |

## What changed (this round)

- `internal/braincmd/hy_drop.go` (new): `runDrop`, `runRestore` over `hyDrRun`, in the order of `runRevert`: parse, usage (operand, `ValidName`, 16 lower-case hex `--expect`), `brain.RevertAllowed(env.Role)`, `levelStore(..., true)`, then `store.Drop`. Failures 6, 8 and 9 print one JSON line on stderr with or without `--json`; other store errors go through `failWith` (exit 1). Origin is `person:<Member>` when `brain.ValidName(Member)`, else `brain.SessionOrigin()`. No flag names the person.
- `internal/braincmd/command.go`: `Env.Member`, `drop|restore` in the verb list, two usage lines under revert with their exit lists, dispatch, Run's doc.
- `internal/cli/brain.go`: `Member: brainMember(env)`, GRID_WHO, else GRID_ACTOR, else GRID_MEMBER.
- `internal/cli/cli.go`: the brain summary now names `drop` and `restore`.
- `contract/README.md`: a section per verb after `grid brain revert`; `contract/commands.json` by `make generate` only; both goldens edited.
- `internal/cli/contract_inventory_test.go`, `guest_parity_test.go`: two rows per verb.
- `internal/braincmd/hy_drop_test.go` (new): the three tests, in package `braincmd_test` so they can run through `cli.Run` and the real environment-to-person mapping.

## Decisions to check in review

- The summary line in `grid help` is joined with `/` (`show/search/.../revert/drop/restore`). With `, ` the line is 122 columns and the existing TestExHelpLinesFitOneLine requires under 120; every verb is still named, so TestExHelpSummariesNameEveryVerb holds. Other lanes that add a brain verb will hit the same limit.
- 6, 8 and 9 print their JSON on stderr also without `--json`, because the brief gives the codes as a machine table and row 3 does not pass `--json`. Success prints the JSON only with `--json`, else `dropped decisions/checks v1 rev <rev>`.
- `--expect` must be exactly 16 lower-case hex; anything else is a usage error (exit 2), so a short or upper-case rev is never reported as a stale rev.
- Open from the first round: whether a merge-into decision on a dropped entry (a later `accepted` line) un-drops it was not checked.

## NOT VERIFIED on a live screen

Nothing here touches a screen. The app's read of the `dropped` and `restored` audit statuses and its call of `grid brain drop` are not run from this repo.

DONE
