# C-jv-agent-write-author: proof table

Repo plugin, branch lane-C-jv-agent-write-author, built from main. It records, it refuses nothing.

## Proof table

| Row | Result |
|---|---|
| 1. `go test ./internal/cli -run 'TestJvAuthor' -count=1 -v`: TestJvAuthorTable, TestJvAuthorEverySite, TestJvAuthorDeclineTrash | HOLDS, exit 0: `--- PASS: TestJvAuthorTable`, `--- PASS: TestJvAuthorEverySite`, `--- PASS: TestJvAuthorDeclineTrash`. 25 table rows (the 13 of the brief, the six of TestTaskWriteRecordsRole, and the fallback order GRID_WORKER_ID, GRID_AGENT, GRID_WORKER, member/owner/cli with a worker mark). The table replaces `h.env.Getenv` whole (jv_agent_author_test.go:23). On main (task_write.go put back to HEAD~1) it fails: `recorded local/owner, want dana@01M/worker`, `recorded local/owner, want local/guest`, `recorded lee/owner, want lee/lead`, `recorded local/owner, want local/external`, `recorded ann/owner, want ann/worker`, and the walk fails with `assignments to Who:`. |
| 2. TestJvAuthorEverySite: AST walk and end to end | HOLDS for the walk (a): exactly 8 `writeOptions` sites: history.go runHistory, land.go runLand, orchestrator.go runPersonApproval, task.go runTaskAttach, task_lane.go runTaskLane, task_write.go runApprove, runTaskTrash, runTaskWrite. (b) differs from the brief's wording, see "Differences" 1. End to end under the agent shell (`GRID_ROLE` empty, `GRID_WORKER_ID=dana@01M`, `GRID_MEMBER=dana`): `task status <t> running`, `history add` and `task close` each exit 0 and append a line with `"who":{"id":"dana@01M","role":"agent"}`. Guest shell with GRID_WORKER_ID: see "Differences" 2, no command wrote a line, so no line names another role; role guest is pinned by the table rows. |
| 3. TestJvAuthorDeclineTrash | HOLDS. Agent shell: `task decline <t> --reason x` exit 0, history `{"id":"dana@01M","role":"agent"}`, task.md has `declined by dana@01M`; `task delete <t> --project personal` exit 0, the trashed copy's history line records dana@01M/agent. GRID_ROLE=worker: decline exit 1 `scope: role cannot decline tasks`, delete exit 3 `a person with an allowed role must perform this task move`, no history line, task folder still there. A member shell still reads `declined by you` and records local/member. |
| 4. `go test ./internal/cli -count=1` | HOLDS, exit 0 (`ok internal/cli 62.658s`). No existing test turned red; none was edited. Importers (`go list`): cmd/grid, lib, with internal/cli itself: `go test ./internal/cli ./cmd/grid ./lib -count=1` exit 0 (cmd/grid has no test files, lib ok 18.315s). Also ran `./contract/... ./internal/packaging/... ./internal/task ./internal/land` exit 0. |
| 5. `git grep -nE 'Who\.(Role\|ID)' -- 'internal/*.go' ':!*_test.go'` | HOLDS. Same 16 matching lines in the same six files as origin/main (`git grep -c` on origin/main gives 4+1+2+6+1+2 = 16); only task_write.go moved lines (:163-165, :337). No new reader outside task_write.go. In task_write.go no check reads `Who.Role` or `Who.ID`: :164 is the existing `if o.Who.ID == ""` that falls back from GRID_ACTOR to GRID_WHO, not a refusal. |
| 6. `make lint`, `make cross` | HOLDS, both exit 0 (`go vet ./...`; darwin/arm64 and windows/amd64 vet and test compile). |
| Rule: small commits, one-sentence messages, no attribution | HOLDS (`git log`: no Co-Authored-By, no session line). |
| Rule: new tests fail on base and pass on the lane | HOLDS, see row 1. |
| Rule: no existing test edited, none deleted | HOLDS. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS. `writeAuthor` (task_write.go:55) only returns an author; decline and trash skip an overwrite, they add no refusal. |
| Rule: Windows and macOS builds | HOLDS, `make cross` exit 0. |
| Rule: RealPlugin tests, UI | Not applicable: no *RealPlugin* row, no UI row. |

## What changed

- internal/cli/task_write.go: `writeOptions` now takes its author from the new `writeAuthor(env)` (and `workerAuthorID`). Person shells (no role, owner, member, cli) without marks record what main recorded. With no GRID_ACTOR and a worker mark they record the worker id with role `agent`. lead, guest, external record `GRID_ACTOR` else `local` in their own role, whatever marks the shell carries. Any other role (worker, builder, agent, ...) is an agent: GRID_ACTOR else the worker id (GRID_WORKER_ID, GRID_AGENT, GRID_WORKER, else `grid`), in the normalized role. The decline overwrite of `Who.ID` (GRID_ACTOR/GRID_WHO) and the trash overwrite of `Who.Role` run only when the author is not an agent. `o.Role`, which the delete and restore policy reads, is unchanged.
- contract/README.md: the decline paragraph (:584) and the delete paragraph (:590) name the agent author. The delete paragraph said "agent shells are refused"; the policy refuses an agent role, not an empty role with a worker mark (TestTaskTrashAgentEnvironmentAllowed), so I reworded it.
- internal/cli/jv_agent_author_test.go: the new tests.

## Differences from the brief

1. The brief's walk (b) expects only the decline branch and the trash line. `internal/cli/signal.go:115,117` also assigns `act.Who` (a string field of the signal action, not a `contract.Author`). A parser walk cannot tell the two apart without types, so the test lists those two lines as a named exception; the assignments in task_write.go must also sit under an `if !agent`.
2. The brief names `grid note <t> <text>`. The plugin has no `note` command; the history.go site is `grid history add <text> --task <t>` (action `note`), which is what the test runs. For the guest shell, every command was refused before any write (`task not found`, or `role "guest" may not close a task`), with and without `GRID_LEASE=task`, in the Personal project and in a shared project that has no guest invitation. So "every history line names role guest" holds with no line, and "a refused command writes no line" is asserted. A guest write that really runs needs an invited guest fixture, which I did not build. Guest → guest is pinned at `writeOptions` by three table rows.
3. Test output of the guest subtests logs the refusals (`t.Logf`).

## Open items (notes, not rows)

- An agent role or worker id that holds control, bidi or noncharacter code points now fails `validateText` (internal/task/text.go:34) with exit 2, as a GRID_ACTOR with such text already does on main. Main never recorded the raw role for a write other than trash, so this is a new way to refuse for a malformed env. The app's ids (`<member>@<ULID>`) and roles are clean. Fix, if wanted, is a sanitize in `writeAuthor`; I did not add one because it would need an exported validator from internal/task, outside the Touches list.
- A role that normalizes to `grid-` (a bare prefix) is Agent tier and is recorded as `grid-`.

## NOT VERIFIED on a live screen

Nothing here needs a screen. Not run: the app worker end to end (no claude, no app in this sandbox); the env shapes come from the brief's reading of app job.go:366-376, judge.go:162-163 and job.go:319-323.
