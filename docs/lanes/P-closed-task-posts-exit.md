# P-closed-task-posts-exit: the close pass posts the cancelled Job's worker-exit

Base: plugin main 885dc9a6, merged into the lane (`git fetch origin && git merge origin/main`; main moved from cba64cdf by docs only: docs/lanes and docs/research). Lane commits: 483a2e72 (code and tests), 824ea6cb (this file), the merge of 885dc9a6, a49d9208 (contract line scoped), and the commit that brings this file up to date.
Decision: 2026-11-17-cancelled-job-feed-exit.md in the orchestrator's briefs. Every attempt ends with one Feed worker-exit, also when the plugin's own pass cancelled the Job ("cancelled task stopped").

## The pick: the close pass posts the exit, before the cancel

The brief offered two ways. This lane takes the first: the closed-task pass posts the exit itself. Why the contract allows that one and not the other:

- **The body is already allowed.** contract/schemas/worker-exit.json: `reason` is an enum (`done`, `crash`, `usage_limit`, `cancelled`, `loop`), `code` is any non-empty string, and `x-roles` holds `cli`, the stream's role. So the pass posts `{"reason": "cancelled", "code": "task stopped"}`. "task stopped" is not a reason value, so it goes in `code`, the field where the app worker puts `stop_requested` and `worker_stopped`. It is `pgClosedReason`, the close reason the pass already writes on the cancel.
- **The tenure the fold reads.** The fold counts an exit only as a Job-side record of the grant's tenure (internal/contract/fold.go:216-238: same worker, fence, gen and attempt as the assign). The pass therefore writes it under the Job's own lease (`j.Assign.To`, `j.ID`, `j.Generation`, `j.Assign.Fence`, `j.Attempt`) through `writeLocked`, the same check the worker's own `grid job` post goes through. The stream assigns its Jobs to its own worker id (tick.go:149-151, `Assignee: s.Holder()` at stream.go:285, `Holder` is member@machine at stream.go:159, so it is the same id after a restart), and pgClosedCancel is called only from the stream (pg_closed_task.go:252). So the pass is the grant's holder.
- **Why not accept the worker's late exit.** The other way would need the Feed to admit a post after `EndJob` has written the takeover line and cleared the holder. That post is refused by internal/feed/store.go:143 (holder) and :153 (ended tenure), and the fold reads any line past the end offset as stale (fold.go:82-83). Accepting it means opening the fence for a closed tenure. A worker that died would still leave no exit.
- **Never two.** The worker's exit goes through `orchestrator.Write` (internal/cli/orchestrator.go:430). That call takes the same `.orchestrator.lock` (write.go:50) and refuses an attempt already exited (write.go:87) or a Cancelled Job (write.go:79). The pass holds that lock across the exit, the cancel and `EndJob`, and posts the exit only when `Exited == ""`; `writeLocked` reloads the Feed and checks `Exited` again before it posts. When the worker came first, its exit stays the only one. When the pass came first, the worker's exit is refused with `stale lease`, which the app records as `exit_superseded`.
- **Exit before cancel.** A pass cut off between the two writes heals: the next pass sees the exit and only cancels. Cancel-first would lose the exit for good, because the loop skips Cancelled Jobs.
- **When the pass posts nothing.** A Job that is not Running, because before the ack the fold ignores any Job-side record. A Job granted to another holder, which gets one Diagnostics line, since its exit is that holder's to write. When the exit post fails, that is one Diagnostics line and the cancel still goes out.

## Proof table

| row | status | proof |
|---|---|---|
| 1 pass first: one worker-exit, "cancelled" with reason "task stopped" | HOLDS | `TestPcClosedPassPostsTheExit` (internal/orchestrator/pc_closed_exit_test.go:89). It checks one worker-exit for J-1 attempt 1 with body exactly `{"reason":"cancelled","code":"task stopped"}`, verdict `counted`, and the fold's `Exited` naming it. J-1 is Cancelled with close reason "task stopped". The worker's late exit returns `feed.ErrStaleLease`, later passes write nothing, and the lease is free. **Base fails** (taken on cba64cdf; main has since moved by docs only): `go test -count=1 -run TestPc ./internal/orchestrator/` with only the test file → `pc_closed_exit_test.go:93: J-1's worker-exits are 0, want one: cancelled task stopped`, `FAIL`. Lane: PASS (row 3) |
| 2 worker beat first: still one worker-exit, unchanged text | HOLDS | `TestPcWorkerExitFirstStaysTheOne` (pc_closed_exit_test.go:110), two subtests: "exit only" (the worker's exit landed, its cancel not yet) and "exit and cancel" (both landed). Each checks one worker-exit with body exactly `{"reason":"cancelled","code":"stop_requested"}`, counted, J-1 Cancelled "task stopped". It passes on base by nature: it pins behaviour that is unchanged |
| race (no row; rule "never two") | HOLDS | `TestPcClosedPassRacesTheWorkerExit` (pc_closed_exit_test.go:140): the worker's `Write` runs in a goroutine against the pass. A nil error requires the worker's exit as the one; `ErrStaleLease` requires the pass's; any other error fails. **Base fails 2 of 20** (cba64cdf): `go test -count=20 -run TestPcClosedPassRacesTheWorkerExit ./internal/orchestrator/` → `pc_closed_exit_test.go:151: J-1's worker-exits are 0, want one: cancelled task stopped` twice, `FAIL` |
| 3 `go test -race -count=20` of the new tests | HOLDS | `grep -rn "func TestPc" internal/orchestrator/` → only the three tests above. `go test -race -count=20 -timeout 20m -v -run '^TestPc' ./internal/orchestrator/` on the merged head → 20 × `--- PASS: TestPcClosedPassPostsTheExit`, 20 × `--- PASS: TestPcWorkerExitFirstStaysTheOne` (both subtests), 20 × `--- PASS: TestPcClosedPassRacesTheWorkerExit`, no race report, `PASS`, `ok  github.com/gridconsole/grid-plugins/internal/orchestrator 168.533s`, exit 0 |
| 4 docs/job-contract.md states the rule in one line | HOLDS | docs/job-contract.md:130 |
| 5a go test of changed packages and importers | HOLDS | `go list -test -f '{{range .Deps}}{{if eq . "github.com/gridconsole/grid-plugins/internal/orchestrator"}}{{$.ImportPath}}{{"\n"}}{{end}}{{end}}' ./...` → cmd/grid, internal/cli, internal/doctor, internal/mcp, internal/migrate, lib, and through tests contract, contract/formats, internal/account, internal/braincmd, internal/golden, internal/job/jobcmd, internal/land, internal/packaging. `go test -count=1 -timeout 40m` on all of them plus internal/orchestrator → `ok internal/orchestrator 1031.280s`, `ok internal/cli 436.379s`, `ok internal/doctor 54.480s`, `ok internal/mcp 5.290s`, `ok internal/migrate 77.137s`, `ok internal/job/jobcmd 55.653s`, `ok internal/account 15.849s`, `ok internal/braincmd 43.207s`, `ok internal/golden 13.999s`, `ok internal/land 19.958s`, `ok internal/packaging 68.094s`, `? cmd/grid [no test files]`, `ok lib 119.104s`, `ok contract 1.867s`, `ok contract/formats 2.299s`, exit 0 |
| 5b make lint | HOLDS | `make -C . lint` → `go vet ./...`, exit 0; `gofmt -l internal/orchestrator` empty |
| 5c walk: TestHsIpWalksEveryCommand, TestIlGuestSiteList | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` → `walked 40 commands, 40 table entries`, `--- PASS: TestHsIpWalksEveryCommand (0.09s)`, `--- PASS: TestIlGuestSiteList (0.15s)`, `ok ... 0.451s`, exit 0 |
| 5d walk: internal/job/lint | HOLDS | `go test -count=1 ./internal/job/lint/` → `ok ... 0.900s`, exit 0 |
| 5e merge main before the proof | HOLDS | `git fetch origin` → `cba64cdf..885dc9a6 main`; `git merge origin/main` → merge made, 4 docs files, no code. Every row from 3 on ran on the merged head. A second `git fetch origin && git merge origin/main` after the runs → `Already up to date` |
| rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| rule: new tests fail on base | HOLDS | row 1 and the race test fail on base (lines above). Row 2 passes on base by design: the brief asks that it stay unchanged |
| rule: no existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD`: one new test file, no existing test changed |
| rule: no guard, gate, cap, hold or blocking hook | HOLDS | the change adds one write to an existing pass. It refuses, delays and counts nothing, and a failed write is a Diagnostics line |
| rule: small commits, one sentence, no attribution | HOLDS | `git log origin/main..HEAD` |
| rule: edit only the brief's packages | HOLDS | internal/orchestrator, docs/job-contract.md, docs/lanes |
| whole-module test, cross-repo set | not run, not asked | run by the orchestrator at landing |
| RealPlugin and app tests | not run, not asked | the app lane C-e0812-pin-one-outcome pins the app test after the plugin pin moves |

## What changed

- internal/orchestrator/pg_closed_task.go: `pcClosedExit` and its comment. `pgClosedCancel` calls it for each Job just before the cancel control.
- internal/orchestrator/pc_closed_exit_test.go: the three tests above.
- docs/job-contract.md: the one-line rule, scoped to Jobs the stream granted to its own worker.
- docs/lanes/P-closed-task-posts-exit.md: this file.

## For the app lane (C-e0812-pin-one-outcome)

After a task stop, J-2 has exactly one Feed worker-exit, in one of two shapes:

- worker first: `{"reason":"cancelled","code":"stop_requested"}`, author the agent's worker;
- pass first: `{"reason":"cancelled","code":"task stopped"}`, author `worker` with role `cli`, on J-2's lease and attempt. worker.log still has `exit_superseded` "task stopped" for J-2: the worker's own post is refused as before.

## Open items and NOT VERIFIED

- A Job that is Assigned but not yet acked when the pass cancels it gets no worker-exit. The fold ignores a Job-side record before the ack ("Job tenure must start with ack"), and the worker cannot post one either. Whether such an attempt needs a record is a contract question; this lane does not change it.
- A Job granted to another worker id gets no exit from the pass, only a Diagnostics line (pg_closed_task.go, `pcClosedExit`). The stream always grants to its own member@machine id, so this needs `grid job tick --to <other>` (internal/cli/orchestrator.go:339) or a stream of another member or machine holding the task lease at close time. Posting under another holder's id would be writing as that holder, so the pass leaves it. That case has no test; the contract line is scoped to Jobs the stream granted to its own worker.
- A person's cancel (D-r19, `personCancels`) and the worker's own cancel still leave the exit to the worker. The contract line is scoped to the closed-task pass.
- No live run against the app worker. The app test is the next lane's.
- No UI change, so there is nothing to check on a live screen.

DONE
