Base: lane-P-run-jobs-decide, cut from lane-P-run-jobs at 43d26ad5 ("Add grid run, which starts one agent per assigned Job in its own worktree and posts its ack and exit"). Every path:line below was read on that commit unless marked UNVERIFIED. Research only: no code changes.

# The launcher (`grid loop`): the Go process that starts agents for the orchestrator's decisions

Source: the owner's design run-jobs-design, 17 Nov 2026. That covers the first part, the Correction, and Addenda 2, 4, 5 and 7. Addendum 3 (judgement and landing) belongs to the sibling lane P-judge-land-decide. Brain-obsidian Addendum 6 (no task lease) also applies. Addendum 7 (production actions; Job code on hidden refs) was folded in by lane P-run-jobs-a7, which read the plugin on this lane's head and the app on main @e60fab587. This doc follows the owner's words where they settle a question and decides only what they leave open.

## Ledger rows this doc decides (decision gate)

The decision ledger of 17 Nov names this lane in its "Decided in" column for these rows.

| Row | Decision (ledger words, short) | Decided in this doc |
|---|---|---|
| V5 | The orchestrator creates and starts the builder, reviewer and runner Jobs | Decision note items 1, 2 and 7; Q1 (auto-assign goes); Q4 |
| V6 | Nothing depends on the desktop app | Q5 (hook and login service); Q6 (grid-worker leaves the run path); lane 12 |
| V7 | The orchestrator decides; the launcher reports numbers and starts on start conditions; no capacity rule in Go | Decision note items 1 and 5; Q2; Q3 |
| V8 | Machines and accounts are notes in the brain, never secrets | Q3 (machine note, account notes) |
| V9 | No task lease; a reassigned Job gets a new attempt | Decision note items 2 and 3; Q1 (exactly once) |
| V10 | Remote launch on other machines and other logins | Q6 |
| V18 | Only the brain syncs, through the Grid sync server | Q5 (`grid sync`), Q6; Addendum 7 B (code moves through git) |
| V20 | A task stays on its machine; moving it is a deliberate act | Q4 (task move) |
| V21 | Routines are time start conditions | Q2 (`at`, `every`, recurring Jobs) |
| V22 | An agent posts as the person only under the person's grant | Q8 (posting as a person); Addendum 7 A6 (grants and approvals) |
| V23 | Inbox in the brain; notifications belong to the app and sync server; the hosted web UI belongs to the app and grid-sync, not the plugin | Q8 (inbox) |
| V24 | The launcher starts without the app and starts `grid sync` with a licence | Q5 |
| V26 | Production actions: workflow approval, command Job, exact allow rule, blocked post | Addendum 7 A; decision note item 8 |
| V27 | Job code on hidden git refs, verified before it merges, readable from other machines | Addendum 7 B; decision note item 9 |

V26 and V27 are shared with P-judge-land-decide (docs/research/judge-land.md). Section "C. Shared with judge-land" in Addendum 7 names one owner lane and one set of names for every shared point. This doc owns `internal/gitref`, the secrets check, submit, the permissions and the running of command and `wait` Jobs (lanes 14, 15 and 4). Judge-land owns the integrate, land and cleanup verbs and the deploy approval, deploy command Job and `wait` verb (its lanes 4 and 9).

## Decision note

```
---
type: decision
topic: decisions
title: The launcher measures, starts and wakes; the orchestrator decides
status: proposed
source: lanes P-run-jobs-decide and P-run-jobs-a7 (Addendum 7), 17 Nov 2026
---
```

**What was decided**
1. One Go process per machine and login, `grid loop` (package `internal/loop`), called "the launcher" in words. It does four things:
   - it reads the board posts in the brain;
   - it starts an agent for each assignment to its own `machine:login` once that Job's start condition holds;
   - it posts `started` and `exit` facts with measured cost;
   - it writes its machine's note.

   On the task's home machine it also wakes the orchestrator and rewrites the task.md status block. It makes no decisions:
   - no cap and no priority;
   - no retry of its own choosing (only the two mechanical attempts of section C, point 18), no park and no learning;
   - no account choice.
2. The orchestrator decides everything in the posts it writes:
   - target `machine:login`, account, model and effort;
   - the start condition and the wait timeout.

   Every assignment is a new attempt. There is no task lease and no Job lease. A post from an older attempt is ignored by the fold.
3. Exactly-once start comes from a **claim**: the `started` post is created only if absent, keyed `(task, job, attempt[, firing])`. On one machine, evaluation and claim happen under one machine-folder lock, so two logins never both start on the same free capacity or the same "no test run".
4. Agents outlive the launcher. Each agent runs under a small shim (`grid loop exec`) that writes the exit record. A restarted launcher reads it from the machine folder, so a kill or a restart never runs a Job twice and never loses an exit.
5. Start conditions use a closed vocabulary of measured atoms:
   - `now`, `cores_free`, `mem_free_gb`, `load_below`, `io_below`, `no_test_run`, `account_room`, `at`, `every`;
   - atoms are combined with `all` or `any`;
   - each has a `wait_max`, after which a `waiting` post wakes the orchestrator.

   "After J-4 lands" is a dependency: a new `landed` kind in the Job record's `after`, read by the same Ready rule as today.
6. Waking is done only by the launcher on the task's home machine, which is moved only by `grid task move`. Which machine is the home is decided in one place, brain-board.md decision point 4 ("The task home"), and this note does not restate it (ledger V20, V16 option C). It starts one fresh orchestrator session per task, giving it the unacknowledged event posts. Events are acknowledged by the orchestrator's posts through the existing `wake` field.
7. Remote launch: `grid run J-n --on <machine>:<login>` writes the assignment to the brain.
   - Same machine: a nudge file in the shared machine folder.
   - Another computer: the Grid sync server pushes a nudge.
   - ssh only starts a launcher that is not running.

   grid-worker leaves the run path completely.
8. Production actions (Addendum 7 A):
   - Every agent Job starts with `--permission-mode dontAsk` and an `--allowedTools` list. It is never started in auto mode and never with bypassPermissions. The orchestrator's CLI check showed that auto mode ran a production stand-in with and without an allow rule, so auto mode is no gate.
   - Approval is a person step of the workflow before the Job, and the Job waits on it with an `after` edge.
   - A deploy is a command Job: the launcher runs the deploy-topic command with no agent. A `wait` Job polls the pipeline with no LLM.
   - An agent that must act gets exactly the approved Claude Code rule, for example `Bash(make deploy-prod)`, added to its tools.
   - An agent that is denied posts `blocked` with the exact command and why, and ends. On approval, the launcher that ran it posts the next attempt with that rule. On decline, the orchestrator replans.
9. Job code on hidden refs (Addendum 7 B):
    - An attempt works in a detached worktree at its base sha, in the machine folder. No local branch is made.
    - At exit, the launcher commits what is left, checks every new commit for secrets, and pushes `refs/grid/<T>/jobs/<J>/a<n>`. It posts the sha on the board.
    - Other machines fetch `+refs/grid/*:refs/grid-remotes/origin/*`.
    - Accepted work is squashed onto `refs/grid/<T>/head` by an `integrate` command Job. The land Job lands head and moves head to the landed sha in the same atomic push.
    - A `cleanup` command Job deletes `refs/grid/<T>/*` when the task closes.
10. Lifecycle:
   - a singleton lock per login in the machine folder;
   - started detached by the SessionStart hook;
   - optionally a login service: systemd user unit (always on the VM, one per login), launchd agent, or Task Scheduler;
   - `grid sync` started beside it when the config holds a licence and a server.

**Why**
- The owner's Correction: "the Go app should not make decisions on how many it can run, it reports the numbers".
- Addendum 2.3: "Task leases are not an issue if the orchestrator creates the task, assigns it, and is responsible for spinning the agent that picks it up".
- Brain-obsidian Addendum 6: attempts replace leases.
- Today's code already does most of the mechanical part:
  - `internal/runner` (worktree per attempt, command, environment, exit classification);
  - `internal/contract/fold.go` (attempt ordering);
  - `internal/orchestrator/events.go` (wake candidates);
  - `internal/routine` (systemd, launchd and schtasks writers).

  What moves out of Go is every rule that chose something: auto-assign, retry counts, the three-failure park, usage-limit requeue and account pick.

**Rejected**
- **Keep the task lease and fence posts.** The owner removed the task lease (Addendum 2.3, brain-obsidian Addendum 6), and a fence needs one writer of a lease file per task across machines.
- **Capacity reservation or a per-machine slot count in the launcher.** That is a capacity decision. The orchestrator expresses it as `expect` and conditions instead.
- **Keep the usage-limit retry on the same attempt** (`internal/orchestrator/tick.go:241-243`). It would collide with the claim key. A new attempt that bases on the previous attempt's ref (`refs/grid/<T>/jobs/<J>/a<n>`, Addendum 7 B) keeps the work.
- **Launcher is the agents' parent with no shim.** A service restart or a kill would lose the exit code and the cost.
- **Per-machine inbox notes.** The owner asked for one inbox note. Each home launcher rewrites only its own section with a version-checked write.
- **A grid-worker relay for remote nudges.** Addendum 5B replaces it with the sync server push.

---

## Q1. What of 43d26ad5 survives, what changes, what goes

**Answer:** The runner's agent-starting core survives. Its account pick, its control posts and every task-lease and fence path go. `Stream` stops being the driver. Its per-task pass becomes the launcher pass with the lease taken out, and its decision arms move to the orchestrator. Exactly-once moves from fences to attempt plus claim.

### internal/runner (43d26ad5)

| Part | Where | Verdict |
|---|---|---|
| Worktree per attempt on `grid-<task>-<job>-a<n>`, reused when the branch exists | `internal/runner/runner.go:161, 255-296` | **Changes** (Addendum 7 B, section "Addendum 7" below). The worktree per attempt survives and moves from `<GridHome>/worktrees` to the machine folder (Addendum 2.1). The branch goes: the worktree is detached at the base sha, and the work leaves the machine as `refs/grid/<T>/jobs/<J>/a<n>`. A new attempt may name `base: {ref: refs/grid/<T>/jobs/<J>/a<n-1>}` to continue work (replaces the same-attempt usage-limit retry, `runner.go:273-274`). |
| Attempt record `attempt.json` and `agent.log` | `runner.go:61-84, 163-170, 630-636` | **Survives**, in the machine folder under `runs/<task>/<job>/a<n>/`; gains `exit.json` written by the shim. |
| Command builder (`claude -p --output-format stream-json --verbose`, model, effort, tools, prompt after `--`) | `runner.go:375-395` | **Survives**; adds `codex exec` for a Job whose account kind is codex (design part 1). |
| Agent environment: drops session keys, sets GRID_JOB/GRID_ATTEMPT/GRID_TASK | `runner.go:397-452` | **Changes.** Drop `GRID_LEASE`, `GRID_LEASE_EPOCH`, `GRID_FENCE`, `GRID_TASK_EPOCH` (`runner.go:433-441`). Keep `GRID_ATTEMPT`. Add `GRID_AGENT` (role) for post authorship. |
| `pick()`: account with most room | `runner.go:308-373` | **Goes.** It is a decision. The orchestrator names `account` on the assignment (design part 1: "posts Jobs with role, machine, login, account, model"). The launcher maps the account id to this login's config folder from its machine note. An account not signed in on this login gives `exit` with code `account_missing` at once. |
| `classify()` and `UsageLimit()` | `runner.go:454-558` | **Survives.** They become the shim's and the launcher's exit reader. The reasons (`done`, `crash`, `usage_limit` with reset, `cancelled`) stay as facts on the exit post. |
| `ack` post under a Job-lease writer | `runner.go:560-584` | **Changes** into the `started` claim post: author is the launcher, plus `attempt`, no lease. |
| `finish()`: worker-exit plus a control post (Ready on worker stop, Parked plus loop-detected on no result) under the task lease | `runner.go:586-628` | **Changes.** Only the `exit` post remains, with `reason`, `code` and `cost`. Ready, Parked and loop-detected are decisions; the orchestrator takes them after the wake. `taskWriter` (`runner.go:564-574`) goes. |
| Dedupe key `(task, job, fence)` in process memory | `runner.go:139-150` | **Goes**, replaced by the claim in the brain (Q1 "exactly once" below). |
| SIGINT interrupts every agent and posts `cancelled/worker_stopped` | `runner.go:213-222, 474-477`; `internal/cli/run.go:29-30` | **Changes.** Stopping the launcher leaves agents running under their shims. A new launcher re-adopts them. Interrupting an agent is an orchestrator act: a `cancel` post for that attempt, which the target launcher carries out. |

### internal/cli/run.go (43d26ad5)

`grid run` drives `Stream.Pass` as the task holder and starts each spawn line (`internal/cli/run.go:31-88, 94-110`). Under the design, `grid run` becomes "write an assignment": `grid run J-n [--on <machine>:<login>] [--account <id>] [--model ..] [--start <json>]` posts a new attempt. The loop part moves to `grid loop`. The contract (`contract/commands.json:3663`) and `testdata/golden/help.golden` change with it. `gridRunRender` and `gridRunBase` (`run.go:127-157`) survive in `grid loop`. `gridRunAccounts` (`run.go:159-183`) goes with `pick()`.

### The tests of the base (the uncommitted rj_run_test.go next to the base lane, read, not copied)

- `TestRjRunStartsOneAgentPerJob` (:224): **carries over**, driven by assignment posts instead of the tick's spawn.
- `TestRjRunUsageLimitResetMakesJobReady` (:278): **changes**. The exit post carries `usage_limit` and the reset, and nothing makes the Job Ready. A new test shows the orchestrator's next attempt with `at: <reset>` starting at that time.
- `TestRjRunSIGINTMarksWorkerStopped` (:316): **replaced** by "launcher killed while the agent runs; the new launcher posts the exit once". The behaviour it pinned (stop puts the Job back to Ready) is removed by the owner's direction: run-jobs-design Correction and Addendum 2.3.

### internal/orchestrator/stream.go and Store.Tick

| Rule | Where | Verdict |
|---|---|---|
| Task lease acquire, adopt, handover, release per pass | `internal/orchestrator/stream.go:266-276, 358-365, 415-433, 476-504` | **Goes** (no task lease). |
| Auto-assign every Ready Job to the pass holder | `internal/orchestrator/tick.go:177-195, 240-257` | **Goes.** The orchestrator posts the assignment with its target. The launcher still uses `Ready()` (`stream.go:263`) for dependencies: it starts an attempt only when the Job is Ready by its `after` list **and** the start condition holds. |
| Retry after a failed attempt; park after three failures | `tick.go:367-387` | **Goes** (a decision). A crash exit wakes the orchestrator, which posts the next attempt or a person Job. The "three" moves to the brain workflow, as Addendum 3 says for "one rework". |
| Stale on liveness (lease expired, process gone, host offline) | `tick.go:334-366, 393-` | **Changes.** The target launcher posts `exit` with code `lost` when it finds a claimed attempt whose shim is gone with no exit record (Q5). A machine whose note is stale is the orchestrator's to reassign (design part 1, last bullet). |
| usage_limit park, then Ready at reset | `tick.go:292-293, 320-322`; `internal/orchestrator/ya_usage_ready.go` | **Goes** (a decision). The exit post carries the reset, and the orchestrator posts the next attempt with `at`. |
| Exit done becomes Submitted, "awaiting judgement" | `tick.go:329-330` | **Survives as a fold rule.** An `exit` with reason `done` and a result of that attempt folds to Submitted with no control post. It is the "awaiting judgement" fact P-judge-land-decide builds on. |
| Worker stopped becomes Ready | `tick.go:282-287, 323-326`; `internal/orchestrator/yc_worker_stopped.go` | **Goes** with the SIGINT behaviour above. |
| First Job reposted when `grid task new` died between writes | `stream.go:369-461` | **Survives in the home launcher**, minus the lease. Reposting the first Job of the task kind is mechanical. |
| Question defaults applied when due | `stream.go:293-296` | **Survives in the home launcher.** A default at a set time from the workflow is mechanical. |
| Closed task cancels open Jobs; curator Job posted | `stream.go:93-103, 290-292` | **Changes.** The cancel stays in the home launcher (mechanical). Posting the curator Job becomes the orchestrator's, after a wake on close. |
| `Ticked` runs `grid land` after the ship person Job | `stream.go:104-109` | **Goes** to P-judge-land-decide (land Jobs). |
| Receipt move | `stream.go:79-86, 278-282` | **Survives** until the migration lane drops old receipts. |
| Quiet-task memo, skip lines once per reason | `stream.go:237-243, 506-513` | **Survives** as a read-cost cache and as the `skip` event. |
| JSON event lines `spawn`/`wake`/`skip` | `stream.go:31-43, 515-523` | **Survives** as `grid loop --events` output for tests and debugging, with `wait`, `start`, `exit` and `note` added. |

### internal/lease, the feed fences, mcp

- **Goes:** `internal/lease/task.go` (Tenure, `AcquireTask`, `GrantJob`, `ReleaseTask`: `task.go:22-29, 237-307, 360-367`) and `internal/lease/job.go` (Job lease heartbeat, `job.go:27-28, 46-110`). The `grid lease` command group (`contract/commands.json:1740`) is retired. `internal/lease/activity.go` was not read (UNVERIFIED).
- **Goes:** the feed write check `leaseLock`, which matches holder, lease, fence, gen and attempt against the lease file and the takeovers (`internal/feed/store.go:126-157`, called at `:240`). So does the fence floor in `internal/feed/guard.go:11-22` and `internal/feed/identity.go:214`. They are replaced by the attempt rule in the fold.
- **The fold already orders by attempt.** An assignment with a lower attempt is ignored (`internal/contract/fold.go:202`), and so is a control post naming another attempt (`fold.go:251`). The fence comparison at `fold.go:206` goes. The new rule: every Job-side post (`started`, `exit`, `result`, `cancel`, `waiting`) whose `attempt` is lower than the Job's current attempt folds as `stale` and changes nothing. Today the reader marks a record stale when its tenure file ended (`internal/feed/read.go:113-114`); with attempts it is the same flag from a different test.
- **The per-writer file name** keyed by fence (`internal/feed/store.go:290`) is the board format, which is P-brain-board-decide's call (Q7).
- **mcp:** agents post with `feed_post` and friends (`internal/mcp/tools.go:192-229`). `GRID_JOB` already wins over the argument (`tools.go:54`). `GRID_ATTEMPT` gets the same treatment, and the author comes from `GRID_AGENT`/`GRID_MEMBER`. Today no MCP code reads `GRID_FENCE` (scout grep, UNVERIFIED beyond that grep). `internal/mcp/stale.go` is a plugin-version notice, not a fence check.

### Exactly once, late starts, restarts

- **Claim.** Before it starts an attempt, the target launcher creates the `started` post with a key `(task, job, attempt)`, plus `firing` for a recurring Job (Q2). "Create only if absent" is a board operation (Q7). If the key exists, whoever wrote it, the launcher does not start.
- **Late start.** If the Job's current attempt is higher than the assignment's (the orchestrator reassigned it), the launcher does not claim. A claim or exit that races the reassignment carries the old attempt and folds as stale. The agent of a stale attempt keeps running until a `cancel` for that attempt; its posts are ignored.
- **After a launcher restart**, the launcher lists its own claims with no exit (Q5 resume) and never claims them again. A crash between claim and shim start leaves a claim with no shim pid. The new launcher posts `exit` code `start_lost`, and the orchestrator decides the next attempt.
- **Across machines** each attempt names one `machine:login`, so only one launcher may claim it.

## Q2. Start conditions

**Answer:** The orchestrator writes them in the assignment post as a `start` block of measured atoms, plus `wait_max`. The launcher only evaluates them. It never edits a condition, never ranks Jobs and never caps them.

### Post shape (the body of `assign`; field names fixed by the post contract lane)

```yaml
type: assign
job: J-7
attempt: 3
on: vm:grid2            # machine name in Grid : login
account: claude-team-2  # id of grid/accounts/<id>.md
model: opus
effort: xhigh
base: {ref: refs/grid/T-12/jobs/J-7/a2, sha: 3f2c9e1}  # optional: continue the previous attempt's work (Addendum 7 B)
allow: ["Bash(make deploy-prod)"]   # optional: exact rules, folded only under a person's approval (Addendum 7 A)
grant: <approval record>            # the approval post or accepted control that allow rests on (A2)
expect: {cores: 4, mem_gb: 6}   # optional: the orchestrator's estimate, from past exit costs
test_run: false         # true for a runner Job that runs a test suite (counts for no_test_run)
start:
  all:
    - cores_free: 4
    - no_test_run: true
  wait_max: 30m         # omitted: the workflow's start-wait default
```

- **`start` omitted, or `start: now`**, starts as soon as the Job is Ready by its `after` list.
- **Atoms**, one key each:

  | Atom | Holds when |
  |---|---|
  | `cores_free: N` | measured idle cores, minus the `expect.cores` of runs started on this machine within the last sample window, is at least N |
  | `mem_free_gb: N` | available memory minus `expect.mem_gb` of recent starts is at least N |
  | `load_below: X` | 1-minute load per core is below X |
  | `io_below: P` | disk busy percent over the last sample is below P |
  | `no_test_run: true` | no live run with `test_run: true` on this machine, for any login |
  | `account_room: {window: 5h, below: 90}` | the named account's newest reading for that window is under 90 percent |
  | `at: 2026-11-18T03:00Z` | one-shot time: holds from then on |
  | `every: "0 3 * * *"` with `tz: <IANA zone>` | recurring time, 5 cron fields, as `internal/routine/interval.go:19-27` parses them (UTC there; the post names its zone, default UTC) |

- **Combinations.** `all: [..]` and `any: [..]`, nestable. An atom whose measurement is unknown on this OS (Q3) does not hold, so the wait timeout reaches the orchestrator instead of starting blind.
- **"After J-4 lands"** is not a start atom. It is a dependency. The Job record's `after` already holds `{job, kind}` with kinds `done`, `failed`, `children-done` (`contract/schemas/job-record.json:292-336`), read by `internal/jobgraph/graph.go:70-150`. Add the kind `landed`, true when a `landed` post exists for that Job (P-judge-land-decide). The launcher's rule is "Ready by `after`, and `start` holds", so the orchestrator's "after J-4 lands, and no test run" is one Job with both.
- **Timeout.** When `start` has not held for `wait_max` since the assignment, the target launcher posts `waiting` once per attempt. The post holds the condition, the unmet atoms and their measured values. The home launcher treats it as a wake event (Q4). The launcher keeps evaluating, and if the condition later holds it still starts, unless the orchestrator posted a new attempt or a cancel. The default comes from a workflow key `start-wait` in the brain workflow; the shipped workflows carry `start-wait: 30m`, and the orchestrator overrides it per Job (Addendum 3's "default from the workflow, not code" pattern). With no key and no `wait_max` there is no `waiting` post: the code holds no default, and the status block still shows the wait to the person.
- **Recurring Jobs (routines).** For `every`, each firing is claimed separately with the key `(task, job, attempt, firing)`, where `firing` is the scheduled minute in UTC. A firing missed while the launcher was down runs once on start when the post says `missed: run-once` (the default). `missed: skip` drops it. Older missed firings are never run. This replaces the per-minute OS timers of `internal/routine` (`timer_systemd.go:21-30`, `timer_plist.go:21-43`, `timer_schtasks.go:34-50`) for planner refill, end reviews and the like (Addendum 4.3).

### How the launcher evaluates

- Each pass (one second, as the stream does, `stream.go:114-115`) reads the newest sample (Q3, every 5 s), then takes the machine-folder lock `loop/start.lock` (`internal/fslock` `LockExclusive`, `fslock_unix.go:12-19`, `fslock_windows.go:27-33`).
- Under the lock it evaluates each pending attempt of this login in the order the posts were written. For each one that holds, it writes the claim, registers the run in `loop/runs/<task>-<job>-a<n>.json` (login, pid, `test_run`, `expect`, started), then releases the lock.
- So a second login's launcher, evaluating a moment later, sees the first start in `no_test_run` and in `expect`. That is measuring what is running, not reserving. The `expect` subtraction corrects the lag of the 5 s sample, so a run started a moment ago counts before the OS shows its use. It ends with the next sample window, holds nothing back after that, and is zero when the orchestrator gives no `expect`.
- The order is the post order because the board is read in sequence. That is not a priority rule: the orchestrator writes conditions such as "no_test_run" on builders to express "tests first" (Correction).

## Q3. Measurements, the cost line, machine and account notes

**Answer:** The launcher samples the OS every 5 s and each agent's process tree. It writes grid/machines/<machine>.md about every minute, with one writer per machine. It puts a cost line on every exit post. It reads accounts from grid/accounts/<id>.md and puts each login's sign-ins and usage windows in the machine note.

### What is measured, and from where

The repo reads none of this today. The only `/proc` reads are process start times (`internal/session/procstart_linux.go:24-33`, `internal/processid/start_unix.go:31,46`), and go.mod has no `golang.org/x/sys` (`go.mod`, no such line). So macOS reads come from system tools, not cgo.

| Measure | Linux | macOS | Windows (first build) |
|---|---|---|---|
| Cores | `runtime.NumCPU` | `runtime.NumCPU` | `runtime.NumCPU` |
| CPU idle | `/proc/stat` deltas | `iostat -c 2 -w 1` CPU idle, or `ps -A -o %cpu` sum | unknown |
| Load 1/5/15 | `/proc/loadavg` | `sysctl -n vm.loadavg` | unknown |
| Free memory | `/proc/meminfo` MemAvailable | `vm_stat` free + inactive + speculative pages × page size; `sysctl -n hw.memsize` | unknown |
| Disk busy | `/proc/diskstats` io-ticks delta | `iostat -d -c 2 -w 1` | unknown |
| Agent CPU, RSS, wall | `/proc/<pid>/stat` utime+stime, `/proc/<pid>/status` VmRSS, summed over the tree (children by ppid scan) | `ps -axo pid,ppid,%cpu,rss,etime` over the tree | unknown |
| Agent totals at exit | shim's `ProcessState.SysUsage()` (rusage: utime, stime, maxrss of the waited child) | same | `ProcessState` user/system time |
| Tokens and USD | `usage.ParseResult` on the attempt log's `result` line (`internal/usage/result.go:15-38`) | same | same |
| Test runs | live runs registered with `test_run: true` in `loop/runs/` (Q2), any login | same | same |
| Account 5h/7d windows | `account.Windows` (`internal/account/registry.go:277`): status-line note, stream `rate_limit_event`, codex rollouts (`internal/account/windows.go:51-58`, `yb_stream.go:19-20`, `codex_read.go:137-155`); plus the `get_usage` probe recommended in `docs/research/usage-direct-read.md:126-141` once the owner answers its question (`:141`) | same | same |

"Unknown" atoms do not hold (Q2), so Windows gets the full launcher with only time, `now`, `no_test_run`, `account_room` and dependency conditions until a Windows measure lane adds the rest.

### The cost line on each exit post

```yaml
type: exit
job: J-7
attempt: 3
reason: done            # done | crash | usage_limit | cancelled | lost | start_lost | account_missing
code: 0                 # exit code, no_result, or the usage reset (unix seconds)
cost: {wall_s: 2140, cpu_s: 3810, cpu_avg_cores: 1.8, rss_peak_mb: 2900, io_read_mb: 410, io_write_mb: 95,
       tokens_in: 81234, tokens_out: 21011, cache_read: 1904400, usd: 6.12, model: opus, account: claude-team-2}
```

`cpu_avg_cores` and `rss_peak_mb` come from the 5 s tree samples, so they include grandchildren such as `go test`. The orchestrator learns what each kind of Job costs by reading these posts (Correction). The launcher keeps no model of it.

### grid/machines/<machine>.md

- **Writer.** One per machine: the launcher of whichever login holds `loop/note.lock` in the shared machine folder (`fslock` `TryLockExclusive`). Every launcher writes its own `loop/logins/<login>.json` each pass, and the note writer merges them. A different login takes over the lock when the writer dies, so the note keeps one writer at a time (Addendum 2.2: "every file keeps a single writer").
- **Cadence.** Rewritten every 60 s with a fresh `updated`, by atomic rename. The machine folder never syncs, but the note is in the brain, so it does (Addendum 4.1). The orchestrator treats a note older than 3 minutes as the machine being offline (design part 1, last bullet).
- **Content**, front matter for the orchestrator and a short body for a person:

```yaml
---
machine: vm
updated: 2026-11-17T09:41:00Z
online: true
launcher: {version: 0.x, service: systemd}
cores: 16
cpu_idle_cores: 9.4
load: [5.1, 4.8, 4.2]
mem_free_gb: 14.2
io_busy_pct: 12
test_runs: 1
repos: [plugin, app]      # project repo names with a folder on this machine
tools: [claude 2.x, codex 0.x, go 1.27]
logins:
  grid2:
    launcher: {pid_alive: true, since: 2026-11-17T04:30:00Z}
    accounts:
      - {id: claude-team-2, config_dir: "~/.claude-team-2", windows: {5h: 41, 7d: 63}, read_at: 2026-11-17T09:40:12Z, source: stream}
    running:
      - {task: T-12, job: J-7, attempt: 3, role: builder, since: 2026-11-17T09:05:00Z, cpu_cores: 2.1, rss_mb: 2400, tokens: 64000}
---
```

The config folder is shown with `~`. No tokens or credentials ever go in.

### grid/accounts/<id>.md (Addendum 2.2)

- Front matter: `id`, `kind` (claude or codex), `name`, `plan`, `default` tag; the email may be there (grid/ is never shared with guests). No token or credential.
- It is written by the person's act that registers the account (`grid account add` or setup), not by the launcher. It replaces `GRID_HOME/accounts/registry.json` (`internal/account/registry.go`). Where it is signed in, and its windows, go in the machine note above.
- The launcher reads both to map an assignment's `account` to this login's config folder (`CLAUDE_CONFIG_DIR`, `runner.go:442-444`).

## Q4. Waking the orchestrator

**Answer:** Only the launcher on the task's home machine wakes it. It starts a fresh `claude -p --agent grid:orchestrator` session for the task, whose input is the unacknowledged event posts. At most one such session runs per task, and events that arrive meanwhile wait for the next one.

### When: the wake events

The existing candidates in `internal/orchestrator/events.go:193-224` are results, rejected verdicts, open questions with answers, and explicit wake records. The new board adds:
- every `exit` post (done with result means "awaiting judgement", plus no result, crash, usage limit, lost, account missing);
- `waiting` (a condition timed out);
- verdict posts (accepted, rework or rejected, STUCK);
- person posts: an answer, a `task-move`, a cancel request;
- a closed task (for the curator decision, Q1);
- the land results `conflict`, `red` and `preexisting`, and a `landed` with flakes (P-judge-land-decide).

### How

- **Home.** The home is the one brain-board.md decision point 4 ("The task home") defines; this note only uses it. `grid task new` writes the home as brain-board.md says, including task.md `home: <machine>` for people (today task.md has no such field: `internal/task/create.go:107-111, 144-148`). Only the home machine's launchers act on the task's wakes, the status block and the inbox section. A hand edit of task.md `home:` is kept and not rewritten, as brain-board.md "Decided by Abdu: V16 home edit" says. On the VM several logins share the machine, so the home role is held by whichever login holds `loop/home.lock`, the same way as the note lock (Addendum 2.3: "never a second orchestrator on the same task").
- **Session.** The home launcher posts `wake` (event post ids, session pid) and then starts the orchestrator in the task folder. The prompt lists only those post ids with their paths, so the session reads the new posts, not the whole board.
- **Acknowledgement.** The orchestrator's own posts name the event they answer in the existing `wake` field (acknowledged set built at `events.go:233-238`).
- **Re-wake.** When the session ends, the launcher re-reads the board. Events that are still unacknowledged, plus new ones, go to the next session. So a killed orchestrator loses nothing; the owner's acceptance "kill the orchestrator session mid-task" holds by construction.
- **No tight loop.** The count is a workflow key, not code: `wake-unanswered: N` (the shipped workflows carry 2). After N sessions in a row end without acknowledging the same event, the home launcher posts a person question ("the orchestrator ended N times without deciding on <event>") and wakes again on the answer. With no key it re-wakes on every pass with an unanswered event. This mirrors the existing loop-detected park (`runner.go:609-613`), and Addendum 3's "a second fail, or STUCK -> post a person Job". Whether the owner wants the key at all is an open item.
- **Task move.** `grid task move T-n --to <machine>` exists today as an intent writer (`internal/cli/intent.go:99-112`, `contract/commands.json:387`). It changes to a person post `task-move` on the board, plus a rewrite of `home:` in task.md. The old home's launchers stop at that post and the new home's start. Nothing moves a task by itself (Addendum 4.2).
- **No task lease.** Because only the home login wakes, there is no race to guard (Addendum 2.3, brain-obsidian Addendum 6).

## Q5. Lifecycle

**Answer:** There is one `grid loop` per machine and login, made a singleton by a lock in the machine folder. It is started detached by the SessionStart hook on every machine with Claude Code, and by a login service where setup installed one (always on the VM, one per login). Agents survive it, and a restart resumes from the brain plus the machine-folder run records.

- **Name.** The command is `grid loop` and the package `internal/loop`. "Launcher" is already the name of the stable `<GridHome>/bin/grid` link that timers call (`internal/routine/launcher.go:13-39`, also looked up by the hook). The code name avoids that clash; docs say "the launcher" as the owner does.
- **Singleton.**
  - `<machine folder>/loop/logins/<login>/loop.lock` is held with `TryLockExclusive` for the life of the process (`internal/fslock/fslock_unix.go:12-19`; `LockFileEx` on Windows, `fslock_windows.go:27-33`).
  - `loop.json` beside it holds pid, pid start, version and started.
  - There is no pidfile guessing: the lock is the truth, and the OS frees it on death.
  - The machine folder is the OS standard place from Addendum 2.1 (P-grid-config-read decides the paths).
- **SessionStart.**
  - Today SessionStart runs one synchronous `grid hook sessionstart` with a 30 s timeout (`hooks/hooks.json:69-80`, `internal/hook/events.go:44`, `internal/hook/hook.go:91-164`) and starts nothing in the background.
  - It gains a quick `TryLockExclusive` probe. If the lock is free, it starts `<GridHome>/bin/grid loop` detached, then returns. The detach pattern is the one `grid open` uses: `Setsid` (`internal/cli/open_start_unix.go:12-13`), `Start` plus `Release` (`open_start.go:247-251`), and `DETACHED_PROCESS` on Windows (`open_start_windows.go:16`).
  - The child gets a clean environment: the session and Job keys `runner.go:398` lists, plus `CLAUDE_*` and `GRID_JOB`, are dropped, so a launcher started from a worker session does not inherit that Job.
  - Addendum 5B: "on every machine where Claude Code is used", not only orchestrator sessions as the first part said.
- **Login service.** `grid loop install` (offered by setup as "let other machines start work here"; always on the VM) writes one of three units, reusing the routine writers' patterns:
  - **systemd user unit** `~/.config/systemd/user/grid-loop.service`: `ExecStart=<GridHome>/bin/grid loop`, `Restart=always`, `KillMode=process` (so agents and their shims survive a restart), enabled with `systemctl --user enable --now`, as `internal/routine/timer_systemd.go:21-30` does for timers. On a headless VM login, a user unit runs without a session only with linger on. `loginctl enable-linger <login>` is a one-time step the person does at setup. Whether this VM's polkit allows a login to enable its own linger is UNVERIFIED.
  - **launchd agent** `~/Library/LaunchAgents/com.gridconsole.loop.plist`: `RunAtLoad`, `KeepAlive`, `AbandonProcessGroup` true, loaded with `launchctl bootstrap gui/<uid>` (`internal/routine/timer_plist.go:21-43`).
  - **Task Scheduler**: an at-logon task with restart on failure, created with `schtasks /Create /XML` (`internal/routine/timer_schtasks.go:34-50`). The routine writer is preview-only on Windows today (`internal/routine/native_windows.go:9`).
- **`grid sync` beside it.** When the config holds a licence and a server (keys from P-grid-config-read; no such key is read in the plugin today), the launcher starts `grid sync` as its child, restarts it when it exits, and stops it when it stops. Otherwise it is not started (Addendum 5B).
- **Agents outlive the launcher.**
  - Each agent runs under `grid loop exec <attempt dir>`, a shim in its own session (`Setsid`). The shim starts the agent (`ownGroup`, `internal/runner/proc_unix.go:12`), waits for it, writes `exit.json` (code, ended, rusage) and exits.
  - The launcher records the shim pid and pid start in the run record. It probes the shim with `session.ProbeLocal` (`internal/session/probe_unix.go:56`) and the start time with `internal/processid`.
- **Resume after a kill.** For each claimed attempt of this login with no exit post:
  - `exit.json` present: post the exit with the cost (log plus samples).
  - Shim alive: adopt it. Sample it, and post the exit when `exit.json` appears.
  - Shim gone with no `exit.json`: post `exit` code `lost`, with the cost read from the log.
  - Claim with no run record (killed between claim and start): post `exit` code `start_lost`.

  Unclaimed assignments are evaluated as usual. The launcher never re-claims, so no Job runs twice (owner acceptance "kill a launcher").
- **Stop.** SIGTERM stops claiming and exits; agents keep running. `grid loop stop --agents` also interrupts this login's agents. Their exits post `cancelled` (shim) and the orchestrator decides.

## Q6. Remote launch

**Answer:** `grid run J-n --on <machine>:<login>` only writes the assignment, a new attempt, to the task board in the brain. The target launcher picks it up from its own copy of the brain. A nudge makes that immediate, and ssh is only for starting a launcher that is not running. Nothing of grid-worker stays in the run path.

- **Same machine, other login.**
  - The logins of one machine read the same brain folder (P-shared-home, option A).
  - `grid run` writes the post, then touches `<machine folder>/loop/logins/<login>/nudge`. The target launcher watches that file (a stat each pass; fsnotify is not in go.mod) and passes at once.
  - If the target's lock is free (launcher not running), the login service starts it (always installed on the VM). Without a service, `grid run` cannot start another Unix user's process without sudo: it uses ssh to `<login>@localhost` when a key exists. Otherwise it says the launcher is down and leaves the assignment for the next start.
- **Other computer.**
  - The brain sync carries the post (Addendum 4.1: only the brain syncs).
  - The Grid sync server pushes "new Job for <machine>:<login>" to that machine's `grid sync`, which touches the same nudge file.
  - Without a sync server there is no cross-computer run: the task stays on the machine that created it (design part 1).
  - If the target's machine note is stale or says no service, `grid run` tries `ssh <host> <GridHome>/bin/grid loop --detach`, with the host from the machine note's `ssh` field written at setup. ssh starts the launcher only; it never carries the Job.
- **Posts from the remote launcher** (`started`, `exit`, `waiting`) go to the task board in its brain copy and sync back. The home launcher sees them and wakes (owner acceptance: "the posts show up in the Mac brain").
- **grid-worker.**
  - Nothing remains in the run path: no relay, no spawn, no Judge.
  - `grid worker next` (`internal/cli/worker.go:84`, which drives the same `Stream`) and `grid worker list` with `internal/worker` (reads the app worker's files, `internal/worker/worker.go:1-3`) stay only while the app still calls them. A retire lane removes them after the app lane drops its readers.
  - The app becomes a view plus notifications over the brain (Addendum 4.6, 5B).

## Q7. Interfaces with the parallel decisions

**Answer:** The launcher is written against a small board interface with two operations that matter, "read posts in order since a cursor" and "create a post only if its key is absent". Either board format can provide both.

```go
package board

type TaskRef struct{ Dir, ID, Home string } // Home: the fold's home (brain-board.md decision point 4), not read from task.md
type Key struct{ Job string; Attempt int; Type string; Firing string } // Firing "" unless recurring
type Post struct {
	ID, Type, Job, Re, Wake string
	Attempt              int
	Author               Author // id, role; For/By/Grant for a granted person post (new Q)
	At                   time.Time
	Body                 map[string]any
}
type Board interface {
	Tasks(ctx context.Context) ([]TaskRef, error)
	Read(ctx context.Context, t TaskRef, since Cursor) ([]Post, Cursor, error) // in write order
	Post(ctx context.Context, t TaskRef, p Post) (Post, error)                 // immutable
	Claim(ctx context.Context, t TaskRef, k Key, p Post) (Post, error)        // ErrClaimed when k exists
}
```

**From P-brain-board-decide** the launcher needs:
1. `Claim`: per-post files give it with a deterministic name, for example `board/<job>-a<n>-started.md`, created with `O_EXCL`; the jsonl feed gives it with a check-then-append under the existing feed lock.
2. `attempt` in every post header, plus the fold rule that a lower attempt is stale (Q1).
3. A cheap `Read` since a cursor, for a 1 s pass over every open task.
4. The post types `assign`, `started`, `exit`, `waiting`, `wake`, `cancel`, `task-move`, plus Addendum 7's `blocked`, `approval`, `submit` and `head` (the last two exist: `contract/schemas/submit.json`, `head.json`), plus judge-land's `landed`, `conflict`, `red`, `preexisting` and `push_lease_lost` results, with `for`/`by`/`grant` on person posts.
5. A sync conflict on a post is impossible (immutable), and a claim that two synced copies both create is resolved by attempt. Two machines can never claim the same attempt, because one attempt names one `machine:login`.

**From P-brain-obsidian-decide:**
- the paths `<brain>/grid/machines/<machine>.md`, `<brain>/grid/accounts/<id>.md`, `<brain>/grid/people/` (grants) and the workflow file holding `start-wait`;
- where task.md lives, and the `home:` front-matter key, which Grid writes at create and move and a person may also edit (what a hand edit does is brain-board.md's, decision point 4);
- that the machine note and the status block are Grid-written, single-writer, and not drift.

**From P-grid-config-read:** the machine folder path per OS, the brain folder key, and the licence and server keys.

**From P-shared-home** (needs, not decided here):
- the logins of one machine read the same brain folder;
- the shared machine folder is group-writable for the grid logins;
- `loop/`, `runs/` and the lock files are mode 0660/0770 with a shared group, while worktrees and logs stay per login at 0700/0600 as the runner writes them today (`runner.go:176, 197, 635`).

## Q8 (ruling). task.md status block, the inbox, posting as a person

**Answer:** The home launcher is the only writer of the status block. It renders the block from the fold with no LLM and replaces only that region after a hash check. The inbox is one note in which each home machine rewrites its own section. A person post by an agent counts only under a grant note, checked by the fold on read.

- **Status block.**
  - A region at the top of task.md between `<!-- grid:status -->` and `<!-- /grid:status -->`, one to three lines, for example "building · J-7 running on vm:grid2 since 09:05 · J-8 waits on no test run · 1 question for you".
  - The exact markers and words are P-brain-board-decide's (its Q4). The writer is decided here.
  - When it is written: on every pass where the rendered block changed.
  - How: read task.md and its hash, replace the region in those bytes, and before the rename re-hash the file on disk. If it changed (an Obsidian save), drop the write, re-read and retry next pass. With a sync server, the write carries the version it was based on and a rejected write is retried the same way (Addendum 5A). It never overwrites the rest of task.md.
- **Inbox.**
  - One note, for example `<brain>/Needs you.md`, with one section per home machine between `<!-- grid:inbox <machine> -->` markers.
  - Each home launcher rewrites only its own section, version-checked (sync server) or under the local brain lock (one machine).
  - So the note is one file, as the owner asked (Addendum 4.6), but has several writers. That is allowed by Addendum 5A's version check, and it is an open item for the owner if strict single-writer is wanted (then one note per machine, embedded).
  - Notifications stay the app's and the sync server's, and the hosted web UI belongs to the app and grid-sync, not the plugin (ledger V23). The plugin writes the inbox section; it serves no web page and sends no notification.
- **Posting as a person** (Addendum 4.5).
  - A grant is a note the person writes, `grid/people/<person>/grants/<name>.md` or a workflow section, naming the agent or role, what it may answer (question kinds, verdicts), the scope (tasks or all) and `until`.
  - An agent's post may carry `for: <person>`, `by: <agent>`, `grant: [[link]]`.
  - The fold treats it as that person's post only when the grant exists, names that agent, covers that post type and has not expired at the post's time. Otherwise it folds as the agent's own post, with a note in the status block.
  - This is a read-side rule, not a write gate (no guard on agents). Today no grant concept exists (`internal/orchestrator/person.go`, `person_write.go`: person is `contract.TierFor(role) == PersonTeam`, `internal/contract/types.go:13-35`).
  - Without a grant, agents post as themselves: the author comes from the environment the launcher sets (`GRID_AGENT`, `GRID_MEMBER`), and mcp ignores an author argument, as it does for `GRID_JOB` (`internal/mcp/tools.go:54`).

## Q9 (ruling). What the launcher gives P-judge-land-decide

- **Wake on exit.** An `exit` with reason `done` and a result of that attempt folds to Submitted, "awaiting judgement" (`tick.go:329-330` today, as a fold rule). It wakes the orchestrator on the home machine (Q4). Verdict posts are wake events.
- **Start conditions:**
  - `after: {job: J-7, kind: landed}` (new `after` kind, Q2) serialises lands per repo;
  - `no_test_run` with `test_run: true` on runner Jobs gives "no other test run on this machine";
  - `cores_free` and `expect` let a land's test run (the runner Job before the land Job, judge-land V12.c) wait for room.
- **Mechanical push retry**, as the sibling decision `docs/research/judge-land.md` settles it (read from its hand copy R-judge-land.md, lines 42-53 and 315-326):
  - The land Job is a command Job: the launcher runs a plugin verb (`grid land change`) with no agent. It goes through the same claim, shim and exit post as an agent. Lane 4 adds this `command` start, which the judge-land plan moves to the launcher lanes.
  - Under judge-land V12.c (decided by Abdu, 17 Nov) a runner Job before it, `grid land test`, rebases and checks, posting `tested`, `conflict`, `red` or a `preexisting` note. The land verb pushes only that tested SHA with `git push --atomic --force-with-lease`, and posts the result `landed` (with `main_sha`), `base_moved` or `push_lease_lost`.
  - On `push_lease_lost`, the launcher that ran it posts a new attempt of the same land Job with the same target and no other change. It is mechanical, with no count and no choice (Addendum 3); the new attempt finds the ref that moved and ends `base_moved` (judge-land Q3 step 6). A launcher writes an assignment in only two cases: this one (also for `integrate`, Addendum 7 B), and the restart of a `blocked` attempt after a person approved it (Addendum 7 A, owner's words "the launcher restarts it as a new attempt").
  - Every other result wakes the orchestrator.
- **"After L-7 lands".** Judge-land models this as an `after` edge of kind `done` on the land Job L-7 (held by the next change's runner Job, judge-land Q3 "Order per repo"), because a land Job is accepted only when it posted `landed` (R-judge-land.md:282-286). That needs no new kind. The `landed` kind of Q2 is only needed for "after build Job J-4 lands", which is a cross-task edge from a build task to the landing task. That edge, cross-task `after`, is what lane 1 must add; both lanes need it.
- **Agent niceness** (judge-land leaves it to this lane). A capacity setting in the launcher would be a decision. If the owner wants land checks favoured, the orchestrator names `nice: N` on an assignment and the launcher applies it at start. With no field, nothing changes. Open item.
- **Not given:** no judgement, no "one rework" count, no bisect. Those are Jobs and workflow defaults in that lane.

## Addendum 7

Owner's Addendum 7 (17 Nov), ledger rows V26 and V27. Evidence is from the plugin on this lane's head and from the app on main @e60fab587 ("app main" below). The CLI fact is the orchestrator's check of 17 Nov (Claude Code 2.1.325, headless `claude -p`; brief a7-allow-rule-check-2026-11-17.md):
- Under dontAsk with `--allowedTools "Bash(make deploy-prod)"`, the command ran.
- Under dontAsk without the rule, it was refused, with one permission denial.
- Under auto mode, it ran with and without the rule.

### A. Production actions (V26)

**Answer:** Jobs run under dontAsk plus an exact allow list, always, and never under auto or bypassPermissions. Approval is a workflow person step the Job waits on. A deploy is a command Job plus an LLM-free `wait` Job. An agent gets one exact extra rule only under a person's approval. A denied agent posts `blocked` and ends. The launcher restarts it on approval, and the orchestrator replans on decline. Grid adds no gate: every refusal is Claude Code's own dontAsk.

**A1. Permission mode. This is a change from today.**

Today:
- The routine path is the only one that fixes the mode. It passes `--allowedTools <list> ... --permission-mode dontAsk` (`internal/routine/execute.go:192-199`).
- The Job path passes a mode only when one is given. `Runner.PermissionMode` says "Grid writes no default" (`internal/runner/runner.go:95-96`). It is appended only when set (`runner.go:391-393`), and `grid run --permission-mode` defaults to empty (`internal/cli/run.go:34, 57`). So a Job today runs in whatever mode the person's Claude Code settings hold, auto or bypass included.
- The app does the same: it passes `--permission-mode` only when the person set one (app main `internal/kernel/worker/agentbuild.go:234-235`; `internal/slices/settings/agent.go:26, 33`, "no flag is passed and Claude Code's own setting applies").

Decided:
- The launcher starts every agent Job with `--permission-mode dontAsk`, the owner's mode ("never bypassPermissions") and the verified one.
- The `grid run --permission-mode` flag goes with lane 4's rewrite of `grid run`. The app setting leaves with the app run path (lane 12).
- A command-line `--permission-mode` overrides the person's `defaultMode`, so the person's configured mode no longer reaches Jobs.
- The person's own allow rules in their user and project Claude Code settings still apply under dontAsk. That is Claude Code's built-in, and Grid does not strip them (open item).
- Interactive sessions the person starts are untouched.

**A2. The allow list.**
- **Base.** The list is the role agent's `tools:` front matter, as today: `internal/cli/worker.go:317-336` reads it, `internal/orchestrator/stream.go:331-337` puts it on the spawn line, and `runner.go:388-390` passes it.
- **Exact rule.** The assignment may carry `allow: [<rule>]` and `grant: <approval record>` (an `approval` post or the accepted control; Q2 post shape). The launcher appends each rule to `--allowedTools` as written, and nothing wider. It never turns `Bash(make deploy-prod)` into `Bash(make:*)`, and it never adds `--dangerously-skip-permissions`. The launcher decides nothing here: the rule text comes from the approval.
- **When `allow` counts.** It folds only when `grant` names an approval record (C, point 11: an `approval` post, or the accepted control of an approve person Job) by a person (tier person, `internal/contract/types.go:13-35`), or by an agent under a Q8 grant that covers `approval` (A6). The approval must be for this task and Job and name exactly these rules. Otherwise the fold ignores the field and the agent starts with its role tools. This is the same read-side rule as Q8, not a gate on the agent. An agent cannot widen its own next attempt by writing `allow`.
- **Codex Jobs.** Codex Jobs have no exact-rule equivalent in this design. An agent Job that needs a production action runs on a claude account (open item).

**A3. Approval comes from the workflow, before the Job.**
- Workflows already have an `approve` step kind with an approver (`internal/workflow/parse.go:441-451`, approvers at `:234-240`), and `grid approve <task>` exists for a person or the app (`contract/commands.json:2525-2526`).
- The project's deploy workflow note gets an approve step before the deploy step. The orchestrator posts it as a person Job (role person) whose body holds the exact command text, the deploy note and its version, and, for an agent Job, the exact rule.
- It shows in the inbox (Q8) and in the status block, as "J-9 waits for your approval: make deploy-prod".
- The person answers with `grid approve`, in the brain inbox, or in the app. Approving posts the existing accepted control (`plan_hash`, `plan_digest`, `lands_hash`; judge-land A.2), and the person Job is done. Declining marks it failed, which wakes the orchestrator (Q4). The `approval` post of A2 is the answer to a `blocked` post, not to this Job (C, point 11).
- **As the start condition.** The deploy Job carries `after: {job: <approve Job>, kind: done}` (C, point 14). There is no `approved` kind; `land.Run`'s plan-hash and lands-hash check is the content check. The closed vocabulary of atoms (decision note item 5) does not grow. Approval is a dependency, read by the same Ready rule (`internal/jobgraph/graph.go:70-150`).
- **wait_max.** It counts only while the Job is Ready by its `after` list. A Job waiting on a person therefore posts no `waiting`: the person's inbox line is the signal.

**A4. The deploy is a command Job; the pipeline is a `wait` Job.** Judge-land owns both (C, points 15-17; judge-land A.3 and A.4, lane JL 9). The launcher's part (lane 4 row 9):
- **Command Job.** A Job record `kind: command`, `command: [land, T-n]`. The launcher runs `grid land T-n` under the same claim, shim and exit post as an agent. It starts no agent and no LLM. `land.Run` refuses a changed plan or lands rule, so what runs is what the person approved. The exit post carries the exit code, the log path and the link (the first URL matching the deploy note's `link` pattern). These three are the proof.
- **`wait` Job.** A Job record `kind: wait`, from the 7 Nov design ("a Job of type `wait` the loop runs without an LLM: poll the pipeline status", refs-design-2026-11-07.md, section Landing and deploy). The launcher runs `grid wait T-n --job W`. That verb polls with the deploy note's command and patterns, every `every` until `timeout` (from the Job, else the workflow keys `wait-every` and `wait-timeout`), and posts `waited {status: green|red|timeout, link, log}`, plus `deploy-failed` on red or timeout. The exit reason stays `done` or `failed` by exit code, and the exit wakes the orchestrator.
  - A manual approval inside the pipeline is not polled for. The orchestrator models it as another approve person Job.

**A5. An agent denied mid-Job.**
- **Agent rule.** The `agents/*.md` and the run-job skill gain this rule: on a permission denial, do not try another route (no wrapper script, no `sh -c`, no other tool). Instead:
  - post `blocked` with `need: permission`, the exact command, the exact rule (`Bash(<command>)`) and why;
  - then end.

  `grid job blocked` today is a reader of Jobs waiting on dependencies (`contract/commands.json:866`; `internal/cli/orchestrator.go:50`). The new item is a post type, `blocked`, not a verb.
- **Fold.** An attempt whose `blocked` post is followed by its `exit` folds as Blocked, not Submitted.
  - The home launcher puts it in the inbox and the status block ("J-7 needs permission: make deploy-prod").
  - It is also a wake event (Q4), so the orchestrator can choose to replan at once.
- **Denials are facts on the exit post.** The CLI reports permission denials in its result (the check above counted "1 permission denial"). The launcher copies the result's `permission_denials` list to the exit post under the same name (C, point 12). A denied agent that forgot to post `blocked` still shows the command.
- **Code of the blocked attempt.** At exit it is committed, checked and pushed like any exit (B3). The restart therefore has a base on any machine.
- **On approval** (an `approval` post that answers the `blocked` post), the launcher that ran the blocked attempt posts the next attempt:
  - the same target, account, model and effort;
  - `base: {ref: refs/grid/<T>/jobs/<J>/a<n>}`;
  - `allow: [<rule>]` and `grant: <approval>`.

  This follows the owner's words ("Approved: the launcher restarts it as a new attempt with that allow rule"). It is the second mechanical assignment a launcher writes (Q9). It has no count and no choice: one approval gives one attempt.
- **On decline**, the orchestrator is woken and replans. The launcher writes nothing.

**A6. Grants (V22) and approvals.**
- The Q8 rule is unchanged: a grant counts for the post types it names.
- An `approval` from an agent counts as the person's only under a grant note that names `approval`, this agent or role, and covers the task. Without that, the approval folds as the agent's own, and A2 ignores the `allow` that rests on it.
- Whether the owner wants production approvals ever granted is an open item. The shipped workflow notes grant none.

### B. Job code on hidden refs (V27)

**Answer:** Lane branches go. An attempt works in a detached worktree in the machine folder. Its work leaves the machine only as a create-only ref (C, point 4), `refs/grid/<T>/jobs/<J>/a<n>`, pushed by the launcher after a secrets check over every new commit. Reviewers and runners on any machine fetch that ref. Accepted work is squashed onto `refs/grid/<T>/head` by an `integrate` command Job. The land Job lands head and moves head to the landed sha in one atomic push. A `cleanup` command Job deletes `refs/grid/<T>/*` when the task closes. Only the brain syncs; code moves through git.

**B1. What this replaces.**

| Today | Where | Becomes |
|---|---|---|
| Plugin: a named branch per attempt, `grid-<task>-<job>-a<n>`, made with `worktree add -b` and reused when the branch exists | `internal/runner/runner.go:160-161, 255-256, 272-296` | `git worktree add --detach <machine folder>/worktrees/<T>/<J>-a<n> <base sha>`. No local branch is made, and nothing is reused by branch name. Continuing work names the previous attempt's ref as `base`. |
| App: a branch per attempt, plus the merge of the dependency branches and of earlier attempts into a new worktree | app main `internal/kernel/worker/agentbuild.go:150-169, 552-568, 576-598`; `internal/kernel/worktree/worktree.go:153-167, 248-252` | Gone with the app run path (lane 12). Dependency work reaches a Job through `refs/grid/<T>/head` once accepted, or through the attempt ref named as `base` for a reviewer. Those local branches only ever worked on one machine. |
| Land: the orchestrator's land script merges lane branches into main with an update-ref compare-and-swap (R-judge-land.md, citing `land.sh:467-469`). Judge-land's `grid land change` rebases the Job's `branch` (R-judge-land.md:271-326); under judge-land V12.c the rebase and the checks move to the runner Job `grid land test`, and the land pushes the tested SHA. | outside the repo; sibling doc | The land Job lands `refs/grid/<T>/head` (B6). The land Job's `branch` field holds that ref name, and `sha` holds the head sha it was posted for (judge-land lane 1 row 4). |
| Result post with `lease` and `fence` | `contract/schemas/submit.json:68-76` (and `head.json`) | `submit` keeps `sha` and `ref` (`submit.json:33-41`) and gains `base_sha`. `lease` and `fence` go with the lease (lane 11), replaced by the `attempt` header. |

**B2. Job start.**
- The target launcher keeps each repo folder that its machine note lists (Q3 `repos:`). Before the first Job in a repo, it makes sure the owner's fetch refspec is in that clone's config, adding it only when `git config --get-all remote.origin.fetch` lacks it:
  ```
  git config --add remote.origin.fetch '+refs/grid/*:refs/grid-remotes/origin/*'
  ```
  `grid setup` and `grid project repo add` add it too.
  - The refspec maps to `refs/grid-remotes/origin/` and not to `refs/remotes/origin/grid/`, so `git fetch --prune` never touches a real `grid/...` branch (refs-design-2026-11-07.md, section Git).
  - The remote is the one the landing rule names (`land.go:117`, `Rule`), `origin` by default.
- Per Job, it fetches narrowly with prune, so other tasks' refs are not fetched on every start (C, point 6):
  ```
  git fetch --prune origin '+refs/grid/<T>/*:refs/grid-remotes/origin/<T>/*' <target>
  ```
  A full `git fetch --prune origin` runs on a task's close post and once an hour for each repo where the launcher holds a Grid worktree.
- **The base.** The assignment's `base: {ref, sha}`.
  - With `sha` given, the launcher checks it is present after the fetch, and a missing sha gives `exit` with reason `base_missing`.
  - With only `ref` given, it resolves the ref.
  - With `base` omitted, it is the task head `refs/grid-remotes/origin/<T>/head` when that exists, otherwise `origin/<target>`.
  - The resolved sha goes on the `started` post as `base_sha`, so every machine knows the exact base.
- **The worktree.** It is `git worktree add --detach` at that sha, in the machine folder (Addendum 2.1). It is never synced (Addendum 4.1).

**B3. Job done: commit, secrets check, push, post. Run by the shim after the agent exits, or by the resuming launcher (Q5).**
1. **Commit what is left.** If the worktree has uncommitted changes, the launcher commits them with `git add -A`, which respects `.gitignore`. The message is "Uncommitted work at the exit of <J> a<n>", under a Grid identity, as app main `worktree.go:252` does for merges. Agents still make their own commits.
2. **Secrets check, owner-directed** (Addendum 7 B: "Job done: commit, secrets check, push"; "a Job commit must never hold a secret").
   - It covers every commit in `base_sha..HEAD`, not only the final tree, because GitHub serves any pushed commit by sha even after its ref is deleted (refs-design-2026-11-07.md, section Git). A secret added in one commit and removed in the next is still a hit.
   - It scans the added lines of `git log -p base_sha..HEAD`, the added file names, and the commit messages.
   - Content uses the patterns of the note checker (`brain.LooksSecret`, `internal/brain/ln_secret.go:3-13`), moved to a new `internal/secret` that the brain keeps calling, plus any patterns the project's environments note lists. It never returns the matched text (C, point 2).
   - File names cover key and env files: `.env*`, `*.pem`, `*.p12`, `id_rsa*`, `id_ed25519*`.
   - The check stops the launcher's own push. It does not stop or limit the agent, which has already exited.
3. **On a hit**, nothing is pushed (C, point 5).
   - The exit post has reason `secret_found`, with commit, file, line and pattern name, and never the text. It is not `done`, so the Job never folds Submitted.
   - The worktree stays, and the orchestrator is woken.
   - Its answer is a rework Job (`grid judge rework`, `continues` J; judge-land lane 3 row 6) on the same `machine:login`, with `base: {sha: <base_sha>}` and `carry: {job: J, attempt: a<n>}`. The launcher applies the squashed diff of that attempt as uncommitted changes in the new worktree. The agent removes the secret, and its exit is checked again, so the commit that held the secret is never pushed.
   - `carry` works only on the machine that holds the attempt. Anywhere else the exit reason is `carry_missing`.
4. **Push**, create-only (C, point 4):
   ```
   git push --force-with-lease=refs/grid/<T>/jobs/<J>/a<n>: origin <HEAD sha>:refs/grid/<T>/jobs/<J>/a<n>
   ```
   - The empty expected value is git's form for "the ref must not exist yet"; a plain push could fast-forward an existing ref.
   - If the ref already holds that sha (a resumed launcher), that counts as done.
   - If it holds a different sha, the exit reason is `ref_exists`. An attempt ref never moves.
   - With no commit past the base (most reviewer and research Jobs), nothing is pushed, and `sha` is the base.
5. **Post.** The shim runs `grid job submit`, which posts `submit {repo, ref, sha, base_sha, commits}`; then the launcher posts `exit` with the same `sha`. Both carry the attempt header.

**B4. Verify.**
- A reviewer, checker or runner Job is assigned with `base: {ref: refs/grid/<T>/jobs/<J>/a<n>, sha}`.
- Its launcher, on any machine, runs the fetch of B2 and opens a detached worktree at that sha.
- Because attempt refs are pushed create-only and the post names the sha, every verifier sees exactly the submitted tree ("verified before it merges", V27).

**B5. Integrate onto `refs/grid/<T>/head`.** Judge-land owns it (C, point 7; judge-land B.7, lane JL 4). The launcher's part is to run the `grid task integrate T J` command Job under the same claim, shim and exit (lane 4 row 9), and to post the one mechanical new attempt after `push_lease_lost` (lane 4 row 10).

**B6. Land, and what the land Job does with `refs/grid/<T>/head`.** Judge-land owns it (C, point 8; judge-land B.8, lane JL 4). The land Job's `branch` field holds `refs/grid/<T>/head` and `sha` the head sha it was posted for (JL 1 row 4). Under rule `main` it pushes main and head in one atomic push with leases, so head equals the landed sha afterwards. Under `pr` the land result carries the PR link and the orchestrator posts the `wait` Job. The deploy follows as A3 and A4.

**B7. Cleanup.** Judge-land owns the remote side (C, point 9; judge-land B.9, lane JL 4): each land deletes the attempt refs it landed, and `grid task cleanup T` at close deletes the rest, the head included. The machine side is this doc's (C, points 1 and 6; lane 14):
- On a task's close post, every launcher removes its own worktrees of T with `git worktree remove`, keeps a worktree with uncommitted changes and names it in a post, and runs `git fetch --prune origin`. The refspec maps only into `refs/grid-remotes/origin/`, so prune drops the task's refs and nothing else.
- A secret-hit worktree (B3) is removed then too.
- Cleanup is housekeeping, not secrecy. GitHub still serves deleted commits by sha, which is why B3 runs before the push.

**B8. Hosts other than GitHub.** The 7 Nov check verified GitHub: hidden refs are accepted, are not shown as branches, and are not fetched by a default clone. GitLab and other hosts are checked when a project uses them (owner's words). Until then such a project's refs are UNVERIFIED, and a refused push gives `push_refused`, which wakes the orchestrator.

**B9. Only the brain syncs.**
- Posts carry shas and ref names, never code.
- The machine folder, with its worktrees, never syncs (Addendum 4.1).
- A machine that sees a post with a sha gets the code with `git fetch` (B2).

### C. Shared with judge-land: one owner and one name per point (V26, V27)

This table is the one decision for both docs (docs/research/judge-land.md, section "Addendum 7"). Each point has one owner lane and one set of names. The other doc points here and does not restate what it does not own. "JL" is judge-land's lane table (its Q6), "RJ" is this doc's.

| # | Point | Decision, with the one set of names | Owner lane | Detail in |
|---|---|---|---|---|
| 1 | `internal/gitref` | Refspec ensure, start fetch, base resolve, detached worktree in the machine folder, exit commit, create-only attempt-ref push, `carry`, and the prune and worktree removal on every machine. | RJ 14 P-loop-refs. JL 7 P-code-refs is merged into it. | B2, B3, B4 here |
| 2 | Secrets check | New `internal/secret` holds the patterns moved from `internal/brain` (the brain keeps calling it), plus the patterns the project's environments note lists. It scans the added lines and added key or env file names of every commit in `base_sha..HEAD`, and the commit messages. | RJ 14 | B3 here |
| 3 | Submit | The shim runs `grid job submit`, once per attempt, after the agent exits and before `exit`. It posts `submit {repo, ref, sha, base_sha, commits}`. The name is `base_sha`, as on `started`, not `base`. | RJ 14 (verb), RJ 1 (schema) | B3 here |
| 4 | Attempt-ref push | Create-only: `git push --force-with-lease=refs/grid/<T>/jobs/<J>/a<n>: origin <sha>:refs/grid/<T>/jobs/<J>/a<n>`. A ref that already holds the same sha counts as done, because a resumed launcher re-runs the step. A ref with another sha gives exit reason `ref_exists`. `push_refused` is kept only for a host's other refusals (B8). | RJ 14 | B3 here |
| 5 | Secret hit | Exit reason `secret_found` with commit, file, line and pattern name, never the text. It is not `done`, so it never folds Submitted, and it wakes the orchestrator. Its answer is a rework Job (`grid judge rework`, `continues` J), posted on the same `machine:login` with `base: {sha: <base_sha>}` and `carry: {job, attempt}`. Anywhere else the launcher exits `carry_missing`. | RJ 14 (scan, `carry`); JL 3 row 6 (the rework post) | B3 here |
| 6 | Fetch | At each Job start a narrow fetch with prune: `git fetch --prune origin '+refs/grid/<T>/*:refs/grid-remotes/origin/<T>/*' <target>`. A full `git fetch --prune origin` runs on a task's close post and once an hour for each repo where the launcher holds a Grid worktree. | RJ 14 | B2 here |
| 7 | Integrate | `grid task integrate T J` (`internal/land/integrate.go`), a command Job. It squashes `base_sha..sha` of the accepted attempt onto `refs/grid/<T>/head` with a lease. The commit carries the Job's title and id and the trailer `Grid-Ref: refs/grid/<T>/jobs/<J>/a<n>`. It posts `integrated {head_sha}` plus `head {sha, ref}`, or `conflict {files}`, or `push_lease_lost`. | JL 4 P-land-change | judge-land B.7 |
| 8 | Land and refs | `grid land test` (a runner Job, judge-land V12.c) rebases head onto main and runs the checks; `grid land change` pushes that tested SHA to main and head in one atomic push with leases, or ends `base_moved`. Under a `pr` rule the land result carries the PR link, and the orchestrator posts the `wait` Job; the verb posts no Job. | JL 4 | judge-land B.8 |
| 9 | Cleanup | Two steps. Each land deletes the attempt refs it landed. At close, `grid task cleanup T` (`internal/land/cleanup.go`) deletes every `refs/grid/<T>/*` on the remote, the head included. Both come after landing. The machine side (prune, worktree removal, keeping and naming a dirty worktree) is point 1. | JL 4 (remote); RJ 14 (machines) | judge-land B.9 |
| 10 | Mode and allow | `--permission-mode dontAsk` on every agent Job, never `auto` or `bypassPermissions`. `allow` rules are appended to `--allowedTools` verbatim. Exactness is held both ways: lint `job.allow-exact` reports `*` and `:*` on the post, and the fold reads `allow` only under a covering `grant`. Without one, the attempt starts with its role tools and `started` carries `allow_dropped: [<rule>]`. | RJ 15 P-loop-permissions. JL 8 P-prod-actions is merged into it. | A1, A2 here |
| 11 | Approval records | Two records, one per purpose. A deploy approval is the existing accepted control of the approve person Job, posted by `grid approve` (`plan_hash`, `plan_digest`, `lands_hash`). The answer to a `blocked` post is an `approval {re, job, attempt, rules, decision}` post. `grant` names either one. It counts when its author is a person, or an agent under a Q8 grant note that covers `approval`. | RJ 1 (schema, fold row 7); JL 9 (deploy approval) | A2, A6 here; judge-land A.2 |
| 12 | Denials | `permission_denials` on `exit`, copied from the CLI's stream-json result, which uses that name. | RJ 15 | A5 here |
| 13 | Blocked and restart | `blocked {need: permission, rule, command, why}`, then the agent ends. On an `approval` answering it, the launcher that ran the attempt posts the next attempt: same target, account, model and effort; `base: {ref: refs/grid/<T>/jobs/<J>/a<n>}` of the blocked attempt; `allow` and `grant`. A decline wakes the orchestrator, and the launcher writes nothing. | RJ 15 | A5 here |
| 14 | Deploy dependency | `after: {job: <approve Job>, kind: done}`. There is no `approved` kind. The content check is `land.Run`'s existing plan-hash and lands-hash check. | JL 9 P-deploy-jobs | A3 here; judge-land A.2 |
| 15 | Deploy command Job | A Job record `kind: command`, `command: [land, T-n]`: the launcher runs `grid land T-n`, the existing executor (`internal/land/land.go:208`). The Job holds no command text. `land.Run` refuses a changed plan or lands rule, so what runs is what was approved. The exit post holds the exit code, the log path, and the link, which is the first URL matching the deploy note's `link` pattern. | JL 9 (content); RJ 4 row 9 (runs it) | judge-land A.3 |
| 16 | `wait` Job | A Job record `kind: wait`: the launcher runs `grid wait T-n --job W` (`internal/land/wait.go`). The poll command and its green and red patterns come from the deploy note. `every` and `timeout` come from the Job, defaulting to the workflow keys `wait-every` and `wait-timeout`. The code holds no default, and lint refuses a `wait` Job with neither. It posts `waited {status: green\|red\|timeout, link, log}`, and on red or timeout also `deploy-failed {env, stage, reason}`, which reopens the task. The exit reason stays `done` or `failed` by exit code. `wait_max` stays the start-condition name only. | JL 9 (verb, lint, keys); RJ 4 row 9 (runs it) | judge-land A.4 |
| 17 | Job shape | Command and wait Jobs are Job records with `kind: command\|wait`, not `run:` on the assignment. | JL 1 (job-record kinds); RJ 4 row 9 | judge-land A.3, A.4 |
| 18 | Mechanical attempts | A launcher writes exactly two kinds of assignment: a new attempt after `push_lease_lost`, from a land or an integrate Job, and the restart of point 13. | RJ 4 row 10, RJ 15 | Q9 here |
| 19 | Order | Land order within a repo is `after: {job: <previous land Job>, kind: done}`, on the next change's runner Job; its land Job waits on that runner Job (judge-land V12.c). `after` kind `landed` is for cross-task edges. | RJ 1 row 4 | judge-land Q3 |

**Duplicate lanes, merged.**
- JL 7 P-code-refs merges into RJ 14. Its rows map to RJ 14 as follows: 7.1 → (1) and (2); 7.2 → (5); 7.3 → (3); 7.4 → (8); 7.5 → (9); 7.6 → (11); 7.7 → (12); 7.8 → (13); 7.9 → (6); 7.10 → (14).
- JL 8 P-prod-actions merges into RJ 15. Its rows map to RJ 15 as follows: 8.1 → (1); 8.2 → (2) and (3); 8.3 → (4) and (5); 8.4 → (5); 8.5 → (9); 8.6 → (10).
- The integrate and cleanup verbs leave RJ 14 for JL 4. RJ 14 row 7 is JL 4 (c) and (l). The remote delete of RJ 14 row 8 is JL 4 (i). Its prune half stays in RJ 14 (8).
- The deploy command Job and the `wait` Job leave RJ 15 for JL 9. RJ 15 rows 6 and 7 are JL 9 rows 1-3.

## Lane plan

All in the plugin repo unless marked app. The launcher, conditions, remote launch and services touch concurrency and cross-platform behaviour, so builders run opus at xhigh (brief). Each lane runs the walk tests of the runbook.

Every lane names the ledger rows it carries (ledger check 2); its acceptance rows come from them. Lanes 14 and 15 are new with Addendum 7. Lanes 1, 4, 12 and 13 changed with it.

| # | Lane | Model / effort | V-numbers | Files | Acceptance rows | Depends on |
|---|---|---|---|---|---|---|
| 1 | P-loop-board | opus xhigh | V9, V26, V27 (schemas) | new `internal/board` (interface, jsonl adapter over `internal/feed`); `internal/contract/fold.go` (attempt-stale rule, drop fence compare at :206, `exit`/`started`/`waiting`/`cancel` folds, Submitted on done-with-result; **A7:** Blocked on `blocked` then `exit`; `allow` folds only under an approval, A2); `contract/schemas` (assign with `on`/`account`/`start`/`expect`/`test_run`, **A7:** `base: {ref, sha}`, `allow`, `grant`, `carry: {job, attempt}`; started with `base_sha` and `allow_dropped`; exit with cost, `sha`, `permission_denials`, and the reasons `secret_found`, `ref_exists`, `push_refused`, `base_missing`, `carry_missing`; **A7:** new `blocked` and `approval` posts, and `integrated`, `waited` (C, points 7 and 16); `submit` and `head` lose `lease`/`fence` (`submit.json:68-76`), `submit` gains `repo`, `base_sha` and `commits`, and `head` carries `ref`; the Job kinds `command` and `wait` are JL 1's; waiting, wake; `after` kind `landed`; cross-task `after` targets, which judge-land also needs) | (1) a post with a lower attempt folds stale and changes nothing; (2) `Claim` twice on one key gives `ErrClaimed`, also from two processes; (3) exit done with result folds Submitted with no control post; (4) `after` kind `landed` is Ready only after a `landed` post; (5) schemas validate the examples in this doc; (6, V26) `blocked` then `exit` folds Blocked, not Submitted; (7, V26, V22) `allow` with a person's `approval` folds; with an agent-authored approval and no grant, or an approval naming another rule or Job, it folds as absent | board format from P-brain-board-decide (adapter for per-post files added by that lane's build) |
| 2 | P-loop-measure | opus xhigh | V7 | new `internal/measure` (`linux.go` /proc, `darwin.go` sysctl/vm_stat/iostat/ps, `other.go` unknown), process-tree sampler, cost line | (1) fixture /proc files give the cores, idle, load, memory and disk numbers; (2) darwin parsers on recorded tool output; (3) a tree of a stub agent with a child shows the child's CPU in the sample; (4) `GOOS=windows` and `GOOS=darwin` builds; unknown measures report unknown | none |
| 3 | P-loop-conditions | opus xhigh | V7, V21 | new `internal/loop/cond` (parse, evaluate, `wait_max`, firing keys; reuse `internal/routine/interval.go`) | (1) each atom holds and fails on a fixture sample; (2) `all`/`any` nesting; (3) an unknown measure does not hold; (4) `every` with a zone gives the right firing keys across a DST change; missed firing run once / skip; (5) timeout yields one `waiting` per attempt | 2 |
| 4 | P-loop-core | opus xhigh | V5, V6, V7, V9, V26, V27 | new `internal/loop` (pass, claim under `loop/start.lock`, run records, shim `grid loop exec`, resume; **A7:** the shim's exit step calls lane 14's commit, check and push, and resume runs it for a dead shim); `internal/runner` (drop `pick`, lease keys, `finish` controls; account from the assignment; **A7:** `--permission-mode dontAsk` always, the `worktree` branch code replaced by lane 14's detached worktree); `internal/cli` (`grid loop`, `grid run` writes an assignment and loses `--permission-mode`; contract and help golden) | (1) two assignments, "now": two agents start, each one `started` and one `exit` with cost; (2) a build waits on `no_test_run` while a `test_run` Job runs, then starts; posts show the wait; (3) a research Job with "now" starts beside a running build; (4) two launchers (two logins, test as two processes) racing on `cores_free` and `no_test_run` start only what the measurement allows; (5) kill -9 the launcher mid-run: the new one posts the exit once and starts nothing twice; (6) crash between claim and start gives `start_lost`; (7) usage limit gives exit with reset and no Ready; the next attempt with `at` starts then; (8) a reassigned (higher) attempt stops the old claim; (9, V26, V27) a Job of `kind: land`, `command` or `wait` (a plugin verb, no agent: `grid land change`, `grid land test` (judge-land V12.c: the runner Job the land Job waits on), `grid task integrate`, `grid task cleanup`, `grid land T-n`, `grid wait`) runs under the same claim, shim and exit (C, points 15-17); (10, V27) a `push_lease_lost` result gives exactly one new attempt of that land or integrate Job | 1, 2, 3, P-grid-config-read (machine folder) |
| 5 | P-loop-home | opus xhigh | V5, V20, V23 | `internal/loop` (home lock, wake, session start, re-wake, two-ended-without-ack person question; first-Job repost, question defaults, closed-task cancel moved from `stream.go`); `internal/task` (`home:` at create); `internal/cli/intent.go` (`grid task move` as a person post plus `home:` rewrite); status block and inbox writers; the home check reads the home that brain-board.md decision point 4 defines | (1) an exit wakes one orchestrator session (stub) with only the new post ids; (2) kill the session: the next wake carries the unacknowledged events; (3) a remote login's launcher never wakes; (4) status block rewritten from posts; an edit to the rest of task.md between read and write is kept; (5) inbox section rewrite keeps other sections; (6) after `grid task move` only the new home wakes; plus the rows brain-board.md Q9 adds to P-loop-home (including the V9 wake row and the V16 hand-edit row), not copied here | 4, P-brain-board-decide (status block format; the home source, decision point 4) |
| 6 | P-loop-notes | opus xhigh | V8 | `internal/loop` (note lock, per-login files, merge, 60 s write); `internal/account` (`grid/accounts/<id>.md` writer at `grid account add`, reader; registry.json migration) | (1) two logins: one note, one writer, takeover when the writer dies; (2) `updated` advances every minute; (3) no token or credential string in the note (secret scan on fixtures); (4) the account id in an assignment maps to this login's config folder | 4, P-brain-obsidian-decide (paths) |
| 7 | P-loop-service | opus xhigh | V6, V18, V24 | `internal/hook` (SessionStart detached start with a clean environment); `internal/loop/install` (systemd `KillMode=process`, launchd `KeepAlive`/`AbandonProcessGroup`, schtasks at logon); `grid sync` child when licence and server are set | (1) SessionStart starts one launcher and a second SessionStart starts none (lock); hook stays under its timeout; (2) unit, plist and XML goldens; (3) restart of the service leaves a running stub agent alive and the new launcher adopts it; (4) no `GRID_JOB`/`CLAUDE_*` from the session in the launcher; (5) `grid sync` started only with licence and server | 4 |
| 8 | P-loop-remote | opus xhigh | V10, V18 | `internal/cli` (`grid run --on`), nudge file, sync-push hook point, ssh start of a stopped launcher | (1) `grid run J-n --on vm:grid3` from grid2 starts it as grid3 within a pass (nudge); (2) a stopped target with no service: ssh to the login starts it; (3) on the VM from the Mac: J-n starts as grid2 and grid3, posts show in the Mac brain (live, checker) | 4, 7, grid sync server push (app/sync side) |
| 9 | P-loop-routines | opus high | V21 | `internal/routine` (routines become recurring Jobs with `every`; timers removed after migration) | (1) planner refill and end review run from `every` conditions with no OS timer; (2) migration lists and removes the old timers | 3, 4 |
| 10 | P-loop-grants | opus high | V22 | `internal/contract/fold.go`, `internal/orchestrator/person*.go`, `internal/mcp` (author from environment; `for`/`by`/`grant`) | (1) a granted answer folds as the person's; (2) expired, wrong-agent or no grant folds as the agent's own; (3) mcp ignores an author argument | 1, P-brain-obsidian-decide (people folder) |
| 11 | P-loop-retire | opus high | V6, V9 | remove `internal/lease`, `grid lease`, the task-lease paths in `stream.go`/`tick.go`, `feed.leaseLock`; `grid worker next/list` and `internal/worker` after the app lane | (1) `go list` shows no importer of `internal/lease`; (2) walk tests green; (3) every removed test names its covering test or the owner's direction | 4, 5, A-worker-out |
| 12 | A-worker-out (app) | opus high | V6, V27 | app: grid-worker out of the run path, with its per-attempt branches and branch merges (`internal/kernel/worker/agentbuild.go:150-169, 552-598`, `internal/kernel/worktree/worktree.go:153-167`) and its `--permission-mode` setting (`agentbuild.go:234-235`); board, machine-note and status-block readers for the view | (1) the app runs no agent and no Judge; (2) the task page shows the launcher's posts and the machine notes; (3, V27) the app makes no `grid-<task>-...` branch; the task page shows each attempt's `sha` and `ref` from the posts | 1, 6 |
| 13 | E-run-jobs-acceptance | opus xhigh (runner and checker) | V5, V6, V7, V8, V9, V10, V18, V20, V21, V22, V23, V24, V26, V27 | none (run only) | the owner's acceptance below, on the VM with the app not installed, plus the V26 and V27 rows of the table below | 1-8, 14, 15, P-judge-land-decide lanes |
| 14 | P-loop-refs (owns C points 1-6; takes JL 7 P-code-refs) | opus xhigh | V27 (with V18, V9) | new `internal/gitref` (refspec ensure, narrow fetch with prune, base resolve, detached worktree in the machine folder, exit commit, create-only push, `carry`, hourly and close-post prune, worktree removal on close); new `internal/secret` (patterns moved from `internal/brain/note.go:199-204`, the brain calls it; environments-note patterns); `internal/cli` (`grid job submit` run by the shim; `grid setup` and `grid project repo add` add the refspec); `internal/runner` `worktree` (`runner.go:272-296`) replaced; contract and help golden | (1) a Job starts in a detached worktree at the assignment's base sha under the machine folder; `git branch` in the repo is unchanged; `started` carries `base_sha`; (2) at exit the work is pushed to `refs/grid/<T>/jobs/<J>/a<n>` on a bare test remote and `submit {repo, ref, sha, base_sha, commits}` posts that sha; a default clone of the remote lists no new branch, and a clone with the refspec has `refs/grid-remotes/origin/<T>/jobs/<J>/a<n>`; (3) a secret pattern added in one commit and removed in the next, a key file name, or a secret in a commit message is not pushed: exit `secret_found` names the commit, file, line and pattern, and no post or log holds the secret text; (4) `carry` on the same login brings the work into the rework Job's attempt with no secret commit in its pushed history; elsewhere it gives `carry_missing`; (5) a reviewer Job on a second clone (another machine) fetches the ref and opens a detached worktree at exactly that sha; (6) a submit with a different sha to an existing attempt ref gives `ref_exists` and the ref keeps its first sha; the same sha after a launcher restart is done; (7) moved to JL 4 (c) and (l); (8) after `refs/grid/<T>/*` is deleted on the remote (JL 4 (i)), `git fetch --prune` on a second clone drops `refs/grid-remotes/origin/<T>/*` and keeps a real `grid/x` branch and its remote-tracking ref; (9) the refspec is added once, never twice; (10) `GOOS=windows` and `GOOS=darwin` builds; (11) after a task's close post, a second machine's close pass leaves no `refs/grid-remotes/origin/<T>/*` and no worktree of T, and a worktree with uncommitted changes is kept and named in a post; (12) HEAD equal to the base pushes nothing and `sha` is the base; (13) a non-GitHub remote's first submit is recorded in the result as "host not checked"; (14) a reassigned Job pushes `a2`, and `a1` stays as it was | 1, 4, P-grid-config-read (machine folder) |
| 15 | P-loop-permissions (owns C points 10, 12, 13; takes JL 8 P-prod-actions) | opus xhigh | V26 (with V22, V23) | `internal/runner` (`command`: dontAsk always, role tools plus folded `allow`); `internal/loop` (restart of a blocked attempt on approval; `allow_dropped` on `started`; `permission_denials` copied to `exit`); `internal/job/lint` (`job.allow-exact`); `agents/*.md` and the run-job skill (on a denial post `blocked`, no other route); inbox and status-block line for blocked and pending approval (with lane 5) | (1) argv golden: every agent start carries `--permission-mode dontAsk`, and none carries `auto`, `bypassPermissions` or `--dangerously-skip-permissions`, also when the person's settings set another `defaultMode`; (2) an assignment with `allow: ["Bash(make deploy-prod)"]` under a person's approval adds exactly that rule to `--allowedTools`; without the approval the argv is the role tools alone; (3) real CLI, live (runner): a stub `make deploy-prod` runs under dontAsk with the rule and is refused with one permission denial without it, as in the orchestrator's 17 Nov check; (4) a denial in the agent's log appears as `permission_denials` on the exit post even when the agent posted no `blocked`; (5) a `blocked` post shows in the inbox and the status block ("needs you") with the exact command; an approval gives exactly one new attempt with `base` the blocked attempt's ref and the rule; a decline wakes the orchestrator and writes no attempt; (6) and (7) moved to JL 9 rows 1-3; (8) a Job waiting on a person's approval posts no `waiting`; (9) lint reports `Bash(make:*)` and `Bash(*)` in `allow` (`job.allow-exact`); (10) `allow` with no covering grant starts the attempt without the rule, and `started` carries `allow_dropped` | 1, 4, 5, 14, 10 |

### The owner's acceptance mapped to lanes

| Owner acceptance | Proven in |
|---|---|
| A task from `grid task new` runs to delivered on the VM with no app and no hand step (orchestrator, Jobs, launcher on grid..grid4, review, land) | 13, built from 4, 5, 7 and P-judge-land-decide |
| A research Job starts beside a running build because the orchestrator posted "now" | 4 row (3), 13 |
| A build waits on "no test run" until the test run ends, shown in the posts | 4 rows (2) and (4), 13 |
| Kill the orchestrator session mid-task: Jobs keep running and a new orchestrator wakes when one finishes | 5 rows (1) and (2), 13 |
| Kill a launcher: the hook or service restarts it, it resumes from the brain, no Job runs twice | 4 rows (5) and (6), 7 rows (1) and (3), 13 |
| From the Mac, start J-n on the VM as grid2 and grid3 with no hand step; the posts show in the Mac brain | 8 row (3), 13 (live) |
| V26: a production deploy runs only after the workflow's approval, as a command Job plus a `wait` Job, and the proof is the log, the exit code and the pipeline link | JL 9 rows 1-3 and 5, 4 row (9), 13 |
| V26: an agent gets only the exact allow rule, never auto or bypassPermissions; a denied agent posts `blocked`, and approval restarts it as a new attempt with that rule | 15 rows (1)-(5), (9), (10), 1 rows (6)-(7), 13 |
| V27: Job code goes to `refs/grid/<T>/jobs/<J>/a<n>` after a secrets check, is verified from another machine before it merges, and the refs are gone after the task closes | 14 rows (1)-(6), (8), (11)-(14), JL 4 (a)-(c), (f), (i), (l), 13 (VM and Mac clones of one GitHub repo) |

### Open items
- The owner's question in `docs/research/usage-direct-read.md:141` (`get_usage` probe or direct call) decides one source of `account_room`.
- Whether VM logins may enable their own linger (UNVERIFIED). Otherwise it is a one-time step by the person.
- Inbox: one note with sections by several writers (decided here), or strict single writer with one note per machine (owner).
- `wake-unanswered` (Q4): whether the owner wants this workflow key at all.
- `nice` on assignments for land checks (Q9): owner.
- P-brain-board-decide's output (R-brain-board.md) was not in the hand folder at the end of this lane. Q7 states what the launcher needs; the board lane decides the format. Since then brain-board.md decision point 4 is the one source for the task home, and decision 6, Q4's Home item and lane 5 point to it (lane P-jobs-land-g2). R-brain-obsidian.md arrived late and was not read here (UNVERIFIED against Q7).
- `internal/lease/activity.go` and the full MCP tool list were not read here (UNVERIFIED); lane 11 reads them before removal.
- Addendum 7 A: the person's own allow rules in their user and project Claude Code settings still apply to Jobs under dontAsk (A1). Grid could exclude them with Claude Code's setting-source options, but that would be a Grid-made restriction, so it is left to the owner.
- Addendum 7 A6: whether a grant may ever cover a production `approval` (owner). The shipped workflow notes grant none.
- Addendum 7 A2: Codex Jobs have no exact-rule equivalent in this design. A production action with an agent runs on a claude account until a Codex lane decides otherwise.
- Addendum 7 A5: the denials field is `permission_denials`, the name in the CLI's stream-json result (C, point 12). Lane 15 row (4) confirms it on the recorded check outputs.
- Addendum 7 B: read together with judge-land's Addendum 7 (lane P-judge-land-a7). Every shared point has one owner and one name in section C.
- Addendum 7 B8: hosts other than GitHub are UNVERIFIED for hidden refs until a project uses one.

DONE
