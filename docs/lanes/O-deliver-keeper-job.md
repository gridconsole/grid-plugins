# O-deliver-keeper-job: `grid deliver` posts a curator Job when brain proposals wait

Lane base: 330256ce (lane clone's main). Code commit: 17d99764.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Test fails on main through the real path | HOLDS | The six new tests run through `Run(["deliver", ...])` with a plan, a passing proof index, two `status: proposed` files, a held task lease and `GRID_MEMBER`/`GRID_LEASE_EPOCH` set. Run with deliver.go unchanged (a no-op `postCuratorJob` stub, so main's behaviour): `go test -run TestDeliverPostsOneCuratorJob ./internal/cli/` exit 1: `deliver_keeper_job_test.go:84: want one curator Job on the Feed, got 0 (stdout "T-01 delivered; waiting for the person\n2 brain proposals wait for the curator\n")`. The other five failed too, e.g. `dry run does not list the Job: "Would write Outcome in task.md\n..."`. |
| 2 One valid curator Job after the change | HOLDS | `TestDeliverPostsOneCuratorJobForWaitingProposals` passes: one Job, id J-1, role `curator`, skills `[brain-keeper]`, review L0, `after` empty, title `Curate 2 brain proposals of T-01`, read pointers `brain-proposals/<id>.md` (no proposal text), the brief's acceptance text, `contract.Validate("job-record", ...)` nil, status not closed, `grid job tree` (run in the task folder) lists J-1. Body: internal/cli/deliver_keeper_job.go:58-89. |
| 2 verify choice | HOLDS | `file-exists` on `task.md` (deliver_keeper_job.go:80). A command needs `min_tests` >= 1 (lint.go:165) and this Job runs no tests; `reviewer-check` needs a reviewer Job id and an L0 Job has none; file-exists needs only a path (lint.go:170). The check that counts is the acceptance row. |
| 3 No second Job; none for zero proposals | HOLDS | `TestDeliverKeeperJobPostsOnceWhileOpen`: a second deliver and a direct second post leave one curator Job; after `grid job cancel J-1` the next post is J-2. The open check is made again under `.orchestrator.lock` via `orchestrator.WriteUnless` (deliver_keeper_job.go:125-132). `TestDeliverWithNoProposalsPostsNoJobAndPrintsAsBefore`: stdout is exactly main's line, stderr empty, no Job. |
| 4 Cannot post: still delivers, one stderr line; dry run posts nothing | HOLDS | `TestDeliverStillDeliversWhenTheCuratorJobCannotBePosted` (no `GRID_LEASE_EPOCH`, and epoch `0.999` for a lease held by someone else): exit 0, task state `waiting`, one stderr line `deliver: the curator Job was not posted: <why>`, no Job. `TestDeliverDryRunPostsNothingAndListsTheCuratorJob`: no Job, `Would post a curator Job for 2 proposals`, JSON action `Post a curator Job for 2 proposals`. deliver.go:60-81. |
| 5 `--json` gains `curator_job` | HOLDS | `TestDeliverJSONNamesTheCuratorJobAndKeepsWaitingProposals`: `curator_job` "J-1", `waiting_proposals` still 2 rows; empty string when none (the dry-run test). Plain count line: `2 brain proposals wait for the curator (Job J-1)`. `deliverOutput` embeds `task.Delivery`, so internal/task is untouched. |
| 6 Tests, vet, builds | HOLDS | `go test -count=1 ./internal/cli/ ./internal/task/` exit 0 (cli ok 41.7s, task ok 2.7s); `go vet ./internal/cli/` exit 0; `GOOS=windows CGO_ENABLED=0 go vet ./internal/cli/` exit 0; `GOOS=darwin CGO_ENABLED=0 go vet ./internal/cli/` exit 0; `gofmt -l internal/cli/` empty. No command or flag changed, so `contract/commands.json` and the golden files are unchanged (the cli golden tests pass). |

## Rules

| Rule | Result | Proof |
|---|---|---|
| Importers of the changed package | HOLDS | `go list` importers of internal/cli: `github.com/gridconsole/grid-plugins/cmd/grid` only; `go test ./cmd/grid/` exit 0, no test files. internal/task is not changed. |
| New tests fail on base, pass on lane | HOLDS | Row 1 above; all six pass on the lane: `go test -run 'TestDeliver(PostsOne|KeeperJob|WithNoProposals|StillDelivers|DryRun|JSONNames)' ./internal/cli/` exit 0. |
| No existing test edited or deleted | HOLDS | `git diff 330256ce..HEAD --stat` touches only deliver.go, deliver_keeper_job.go, deliver_keeper_job_test.go and docs/delivery.md. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The post is best effort: an error prints one line and the delivery exits 0. The retry (3 tries) is of this writer's own id choice, not a limit on an agent. The Job sets `on_blocked: exit` so the keeper never asks a person. |
| Files touched | HOLDS | deliver.go, new deliver_keeper_job.go, new deliver_keeper_job_test.go, docs/delivery.md. Not touched: internal/task/*, task.go, task_new_first_job.go. |
| Small commits, one sentence, no attribution | HOLDS | `17d99764` "grid deliver posts a curator Job on the Feed when brain proposals wait." plus the report commit. No path, host or email of a person in the plugin files. |
| Whole-module test, cross-repo set | NOT RUN | The orchestrator runs them at landing. |

## What changed

- `internal/cli/deliver.go`: after `Deliver` returns (no task, plan or brain lock held), a non-dry run posts the curator Job and names it in the count line and in `--json`; a dry run lists the post when no curator Job is open.
- `internal/cli/deliver_keeper_job.go`: the job-record, the open-Job check, the next free `J-<n>`, and the post through `orchestratorWriter(env, "task", GRID_LEASE_EPOCH)` and `orchestrator.WriteUnless`.
- `internal/cli/deliver_keeper_job_test.go`: the six tests.
- `docs/delivery.md`: the count sentences.

## Notes for the reviewer

- Concurrency: two deliveries cannot both post. The open-curator check and the id check are repeated under `.orchestrator.lock` in `WriteUnless`; on a taken id the id is chosen again (3 tries, then one stderr line).
- Intent pointers are `task.md#outcome` (a file in the task folder, as job/source.go:121 resolves a plain pointer); `worktree` is null (the Job writes only the brain); `posts_back` is `["result"]`.
- The read pointer is `brain-proposals/<id>.md` as the brief says; a project-store or home-level proposal has another path (`WaitingProposal.Path`), and `grid brain keep --task` still lists it.
- The tests use the local owner as the lease holder: a `member` identity cannot see a task of the `personal` project.

## NOT VERIFIED on a live screen

- That a worker started for the curator Job (role `curator`, null worktree) runs `grid-curator` and closes the Job. The Feed record and the Job tree are proved; the spawn is not.
- That the dev UI shows the Job in the Feed with title `Curate N brain proposals of <task>`.
