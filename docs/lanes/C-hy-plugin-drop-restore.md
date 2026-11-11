# C-hy-plugin-drop-restore: lane report

Base: main 91fadeff (`git rev-parse origin/main` at the start of the lane; `drop` and `restore` were not in command.go).
Stopped at the call budget: the store layer is done and tested; the CLI verbs, the shared tables, the goldens and the README are NOT DONE. Split below.

## Proof table

| Row / rule | Result |
|---|---|
| 1. TestHyDrDropAndRestore in internal/braincmd (CLI, GRID_MEMBER/GRID_WHO/GRID_ACTOR origin, `--by` exits 2) | NOT DONE. No runDrop/runRestore, no `Env.Member`. Store-level equivalent HOLDS: `go test -count=1 -run HyDr ./internal/brain/ ./internal/job/` exit 0, TestHyDrDropAndRestore in internal/brain/hy_drop_test.go checks bytes unchanged, versions unchanged, search and index drop it, show has `Dropped`, changes lists `dropped` and `restored` with origin `person:ana`, and a drop-restore-drop writes two `dropped` lines. |
| 2. TestHyDrRoles (exit 3 for guest, log byte-equal) | NOT DONE. The role check lives in the CLI verb, which is not written. |
| 3. TestHyDrRefusals (exit codes 6, 8, 9, 2, 1) | NOT DONE as an exit-code test. The store half HOLDS: TestHyDrRefusals and TestHyDrDropTwiceIsRefused in internal/brain/hy_drop_test.go: stale rev gives `ErrConflict` carrying the rev, missing note `ErrNoSuchNote`, restore of a never-dropped note, `machines`, `skills`, `TOPIC`, a superseded note and a second drop give `ErrDropRefused`; note bytes and log.md are unchanged after each. |
| 4. TestHyDrConcurrent (internal/brain/hy_drop_test.go) | HOLDS. Two `DropWith` calls released by a channel barrier (no sleep, no clock): one nil, one `ErrDropRefused`, one new `dropped` line. `go test -count=1 -run HyDr ./internal/brain/ ./internal/job/` exit 0. |
| 4. TestHyDrJobSource (internal/job/hy_drop_source_test.go) | HOLDS. Resolve on a dropped note's pointer fails with `brain:project/deploy: note is dropped`, and succeeds after restore. Same command, exit 0. |
| 5. `make generate` twice, `git diff --exit-code contract/commands.json` | NOT DONE (no verb, so no commands.json change). |
| 5. `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/job/ ./internal/cli/` | HOLDS for the code on this head: `go test -count=1 ./internal/brain/... ./internal/job/... ./internal/braincmd/... ./internal/cli/...` exit 0 (cli 68.8s). The named inventory, parity, unknown-project-walk and help tests do not walk the new verbs, because there are none. |
| 5. `make golden`, `make lint`, `make cross` | NOT DONE (not run). `gofmt -l internal` is empty. |
| Importers of brain and job | HOLDS. `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep -E 'internal/(brain\|job)( \|$)'` lists braincmd, cli, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, lease, migrate, orchestrator, people, ports, routine, task, workflow. `go test -count=1` on each: all ok, exit 0. |
| New tests fail on base | NOT RUN. By construction they do not compile on base 91fadeff: `DropWith`, `Store.Drop`, `NoteRev`, `OpDrop`, `OpRestore`, `ErrConflict`, `ErrDropRefused`, `ErrNoSuchNote` and `Entry.Dropped` do not exist there. |
| No existing test edited or deleted | HOLDS. `git diff --stat origin/main..HEAD` shows only the two edited read paths and three new test files. |
| No guard, gate, cap or hold on agents | HOLDS. Nothing here blocks an agent; the role check is the owner's decision and sits in the CLI verb (not yet written). |
| Commits | One sentence each, no attribution lines. No personal paths, hosts or emails. |
| Windows and macOS builds | NOT DONE (`make cross` not run). |

## What changed

- `internal/brain/hy_drop.go` (new): `DropWith(root, topic, name, op, expect, origin, reason)` under `locked`; `NoteRev` (first 8 bytes of sha256, 16 lower hex); `Dropped(root)` (one log.md read, keyed `topic/name`); errors `ErrDropRefused` (`*RefusedError` with the reason), `ErrNoSuchNote`, `ErrConflict` (`*ConflictError` with the rev); `Store.Drop(level, ...)` which runs `DropWith` and then `RebuildIndex` after the lock.
- `internal/brain/store.go`: `Entry.Dropped` (`json:"dropped,omitempty"`, so no other JSON changes).
- `internal/brain/query.go`: `entries()` reads `Dropped` once per level; Show returns the entry with `Dropped` set; Search skips it.
- `internal/brain/index.go`: the entry count skips a dropped entry, one `Dropped` read per `rows` call.
- `internal/job/source.go`: a dropped note is refused with `<pointer>: note is dropped`.
- Tests: `internal/brain/hy_drop_test.go`, `internal/job/hy_drop_source_test.go`.

## Decisions to check in review

- `DropWith` has no level argument (the brief's signature), but `topicAt` needs one. `hyDrKnowledgeTopic` takes built-in topics by Kind, refuses the reserved names (`skills`, `workflows`, `design`, `research`, `incidents`, `versions`), and for a custom topic accepts a topic that is a knowledge topic at either level; a topic with no TOPIC.md is `ErrNoSuchNote`.
- `logAudit` drops a line already present in the log tail as a retry. A drop, restore and drop of one entry in one millisecond would write an identical second `dropped` line and lose it, so `DropWith` stamps each line strictly after the entry's last accepted, dropped or restored line. TestHyDrDropAndRestore covers this.
- State is the last `accepted`, `dropped` or `restored` line of the entry, in file order. NOT VERIFIED: whether a later `accepted` line written by a merge-into decision on a dropped entry un-drops it. That follows the brief's rule; I did not read `DecideTask` to check which status it logs.
- `RebuildIndex` still lists a dropped entry's outgoing links in backlinks.json, as it does for a superseded one. The brief did not ask for a change.

## Split for the rest (new lane, about 25 calls)

1. `internal/braincmd/hy_drop.go`: `runDrop`, `runRestore` in the order of `runRevert`; `Env.Member`; origin `person:<member>` when `brain.ValidName`, else `brain.SessionOrigin()`; role check `brain.RevertAllowed`; exits 0, 2, 3, 6, 8, 9, 1 with the JSON on stderr; call `store.Drop(level, ...)`.
2. `internal/braincmd/command.go` (verb list, two usage lines, dispatch, Run doc), `internal/cli/brain.go` (`Member` from GRID_WHO, else GRID_ACTOR, else GRID_MEMBER), `internal/cli/cli.go:121`, `contract/README.md`, both goldens, `make generate`.
3. One row per verb in `contract_inventory_test.go` and `guest_parity_test.go`.
4. `internal/braincmd/hy_drop_test.go`: TestHyDrDropAndRestore, TestHyDrRoles, TestHyDrRefusals as in the brief.
5. `make golden`, `make lint`, `make cross`.

## NOT VERIFIED on a live screen

Nothing here touches a screen. The app's read of the `dropped` and `restored` audit statuses is not run from this repo.

NOT DONE
