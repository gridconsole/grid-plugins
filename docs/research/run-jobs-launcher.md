Base: lane-P-run-jobs-decide, cut from lane-P-run-jobs at 43d26ad5 ("Add grid run, which starts one agent per assigned Job in its own worktree and posts its ack and exit"). Every path:line below was read on that commit unless marked UNVERIFIED. Research only: no code changes.

# The launcher (`grid loop`): the Go process that starts agents for the orchestrator's decisions

Source: the owner's design run-jobs-design, 17 Nov 2026. That covers the first part, the Correction, and Addenda 2, 4 and 5. Addendum 3 (judgement and landing) belongs to the sibling lane P-judge-land-decide. Brain-obsidian Addendum 6 (no task lease) also applies. This doc follows the owner's words where they settle a question and decides only what they leave open.

## Decision note

```
---
type: decision
topic: decisions
title: The launcher measures, starts and wakes; the orchestrator decides
status: proposed
source: lane P-run-jobs-decide, 17 Nov 2026
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
   - no retry, no park and no learning;
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
6. Waking is done only by the launcher on the task's home machine, named in task.md and moved only by `grid task move`. It starts one fresh orchestrator session per task, giving it the unacknowledged event posts. Events are acknowledged by the orchestrator's posts through the existing `wake` field.
7. Remote launch: `grid run J-n --on <machine>:<login>` writes the assignment to the brain.
   - Same machine: a nudge file in the shared machine folder.
   - Another computer: the Grid sync server pushes a nudge.
   - ssh only starts a launcher that is not running.

   grid-worker leaves the run path completely.
8. Lifecycle:
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
- **Keep the usage-limit retry on the same attempt** (`internal/orchestrator/tick.go:241-243`). It would collide with the claim key. A new attempt that bases on the previous branch keeps the work.
- **Launcher is the agents' parent with no shim.** A service restart or a kill would lose the exit code and the cost.
- **Per-machine inbox notes.** The owner asked for one inbox note. Each home launcher rewrites only its own section with a version-checked write.
- **A grid-worker relay for remote nudges.** Addendum 5B replaces it with the sync server push.

---

## Q1. What of 43d26ad5 survives, what changes, what goes

**Answer:** The runner's agent-starting core survives. Its account pick, its control posts and every task-lease and fence path go. `Stream` stops being the driver. Its per-task pass becomes the launcher pass with the lease taken out, and its decision arms move to the orchestrator. Exactly-once moves from fences to attempt plus claim.

### internal/runner (43d26ad5)

| Part | Where | Verdict |
|---|---|---|
| Worktree per attempt on `grid-<task>-<job>-a<n>`, reused when the branch exists | `internal/runner/runner.go:161, 255-296` | **Survives.** Worktrees move from `<GridHome>/worktrees` to the machine folder (Addendum 2.1). A new attempt may name `base: <branch of attempt n-1>` to continue work (replaces the same-attempt usage-limit retry, `runner.go:273-274`). |
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
base: grid-T-12-J-7-a2  # optional: continue the previous attempt's branch
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
  | `every: "0 3 * * *"` with `tz: Europe/Amsterdam` | recurring time, 5 cron fields, as `internal/routine/interval.go:19-27` parses them (UTC there; the post names its zone, default UTC) |

- **Combinations.** `all: [..]` and `any: [..]`, nestable. An atom whose measurement is unknown on this OS (Q3) does not hold, so the wait timeout reaches the orchestrator instead of starting blind.
- **"After J-4 lands"** is not a start atom. It is a dependency. The Job record's `after` already holds `{job, kind}` with kinds `done`, `failed`, `children-done` (`contract/schemas/job-record.json:292-336`), read by `internal/jobgraph/graph.go:70-150`. Add the kind `landed`, true when a `landed` post exists for that Job (P-judge-land-decide). The launcher's rule is "Ready by `after`, and `start` holds", so the orchestrator's "after J-4 lands, and no test run" is one Job with both.
- **Timeout.** When `start` has not held for `wait_max` since the assignment, the target launcher posts `waiting` once per attempt. The post holds the condition, the unmet atoms and their measured values. The home launcher treats it as a wake event (Q4). The launcher keeps evaluating, and if the condition later holds it still starts, unless the orchestrator posted a new attempt or a cancel. The default comes from a workflow key `start-wait` in the brain workflow. With no key it is 30 minutes, a default the orchestrator overrides per Job (Addendum 3's "default from the workflow, not code" pattern).
- **Recurring Jobs (routines).** For `every`, each firing is claimed separately with the key `(task, job, attempt, firing)`, where `firing` is the scheduled minute in UTC. A firing missed while the launcher was down runs once on start when the post says `missed: run-once` (the default). `missed: skip` drops it. Older missed firings are never run. This replaces the per-minute OS timers of `internal/routine` (`timer_systemd.go:21-30`, `timer_plist.go:21-43`, `timer_schtasks.go:34-50`) for planner refill, end reviews and the like (Addendum 4.3).

### How the launcher evaluates

- Each pass (one second, as the stream does, `stream.go:114-115`) reads the newest sample (Q3, every 5 s), then takes the machine-folder lock `loop/start.lock` (`internal/fslock` `LockExclusive`, `fslock_unix.go:12-19`, `fslock_windows.go:27-33`).
- Under the lock it evaluates each pending attempt of this login in the order the posts were written. For each one that holds, it writes the claim, registers the run in `loop/runs/<task>-<job>-a<n>.json` (login, pid, `test_run`, `expect`, started), then releases the lock.
- So a second login's launcher, evaluating a moment later, sees the first start in `no_test_run` and in `expect`. That is measuring what is running, not reserving.
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
- `push-lost` or `conflict` from landing (P-judge-land-decide).

### How

- **Home.** task.md gets `home: <machine>` at `grid task new` (today task.md has no such field: `internal/task/create.go:107-111, 144-148`). Only that machine's launchers act on the task's wakes, the status block and the inbox section. On the VM several logins share the machine, so the home role is held by whichever login holds `loop/home.lock`, the same way as the note lock (Addendum 2.3: "never a second orchestrator on the same task").
- **Session.** The home launcher posts `wake` (event post ids, session pid) and then starts the orchestrator in the task folder. The prompt lists only those post ids with their paths, so the session reads the new posts, not the whole board.
- **Acknowledgement.** The orchestrator's own posts name the event they answer in the existing `wake` field (acknowledged set built at `events.go:233-238`).
- **Re-wake.** When the session ends, the launcher re-reads the board. Events that are still unacknowledged, plus new ones, go to the next session. So a killed orchestrator loses nothing; the owner's acceptance "kill the orchestrator session mid-task" holds by construction.
- **No tight loop.** If two sessions in a row end without acknowledging the same event, the launcher posts a person question ("the orchestrator ended twice without deciding on <event>") instead of a third session. This mirrors the existing loop-detected park (`runner.go:609-613`), and Addendum 3's "a second fail, or STUCK -> post a person Job".
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

type TaskRef struct{ Dir, ID, Home string }
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
4. The post types `assign`, `started`, `exit`, `waiting`, `wake`, `cancel`, `task-move`, `landed`, `push-request`, `push-lost`, with `for`/`by`/`grant` on person posts.
5. A sync conflict on a post is impossible (immutable), and a claim that two synced copies both create is resolved by attempt. Two machines can never claim the same attempt, because one attempt names one `machine:login`.

**From P-brain-obsidian-decide:**
- the paths `<brain>/grid/machines/<machine>.md`, `<brain>/grid/accounts/<id>.md`, `<brain>/grid/people/` (grants) and the workflow file holding `start-wait`;
- where task.md lives, and that `home:` is a Grid-owned front-matter key;
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
  - Notifications stay the app's.
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
  - `cores_free` and `expect` let a land's test run wait for room.
- **Mechanical push retry.**
  - A land agent ends with a `push-request` post (repo, remote, target ref, sha, lease = the main sha it rebased on) and exits.
  - The launcher that ran it runs `git push --force-with-lease=refs/heads/main:<lease> <remote> <sha>:refs/heads/main`.
  - On success it posts `landed` with the new main sha.
  - On a lost race it fetches and rebases the land worktree onto the new main. If the rebase is clean it pushes again with the new lease, repeating while clean. On a conflict it aborts the rebase and posts `push-lost` with the conflict, which wakes the orchestrator.
  - Whether a clean mechanical rebase must re-run the test Job before the push is P-judge-land-decide's rule; the launcher offers both (push now, or post `push-lost` with reason `retest`).
- **Not given:** no judgement, no "one rework" count, no bisect. Those are Jobs and workflow defaults in that lane.

## Lane plan

All in the plugin repo unless marked app. The launcher, conditions, remote launch and services touch concurrency and cross-platform behaviour, so builders run opus at xhigh (brief). Each lane runs the walk tests of the runbook.

| # | Lane | Model / effort | Files | Acceptance rows | Depends on |
|---|---|---|---|---|---|
| 1 | P-loop-board | opus xhigh | new `internal/board` (interface, jsonl adapter over `internal/feed`); `internal/contract/fold.go` (attempt-stale rule, drop fence compare at :206, `exit`/`started`/`waiting`/`cancel` folds, Submitted on done-with-result); `contract/schemas` (assign with `on`/`account`/`start`/`expect`/`test_run`, started, exit with cost, waiting, wake; `after` kind `landed`) | (1) a post with a lower attempt folds stale and changes nothing; (2) `Claim` twice on one key gives `ErrClaimed`, also from two processes; (3) exit done with result folds Submitted with no control post; (4) `after` kind `landed` is Ready only after a `landed` post; (5) schemas validate the examples in this doc | board format from P-brain-board-decide (adapter for per-post files added by that lane's build) |
| 2 | P-loop-measure | opus xhigh | new `internal/measure` (`linux.go` /proc, `darwin.go` sysctl/vm_stat/iostat/ps, `other.go` unknown), process-tree sampler, cost line | (1) fixture /proc files give the cores, idle, load, memory and disk numbers; (2) darwin parsers on recorded tool output; (3) a tree of a stub agent with a child shows the child's CPU in the sample; (4) `GOOS=windows` and `GOOS=darwin` builds; unknown measures report unknown | none |
| 3 | P-loop-conditions | opus xhigh | new `internal/loop/cond` (parse, evaluate, `wait_max`, firing keys; reuse `internal/routine/interval.go`) | (1) each atom holds and fails on a fixture sample; (2) `all`/`any` nesting; (3) an unknown measure does not hold; (4) `every` with a zone gives the right firing keys across a DST change; missed firing run once / skip; (5) timeout yields one `waiting` per attempt | 2 |
| 4 | P-loop-core | opus xhigh | new `internal/loop` (pass, claim under `loop/start.lock`, run records, shim `grid loop exec`, resume); `internal/runner` (drop `pick`, lease keys, `finish` controls; account from the assignment); `internal/cli` (`grid loop`, `grid run` writes an assignment; contract and help golden) | (1) two assignments, "now": two agents start, each one `started` and one `exit` with cost; (2) a build waits on `no_test_run` while a `test_run` Job runs, then starts; posts show the wait; (3) a research Job with "now" starts beside a running build; (4) two launchers (two logins, test as two processes) racing on `cores_free` and `no_test_run` start only what the measurement allows; (5) kill -9 the launcher mid-run: the new one posts the exit once and starts nothing twice; (6) crash between claim and start gives `start_lost`; (7) usage limit gives exit with reset and no Ready; the next attempt with `at` starts then; (8) a reassigned (higher) attempt stops the old claim | 1, 2, 3, P-grid-config-read (machine folder) |
| 5 | P-loop-home | opus xhigh | `internal/loop` (home lock, wake, session start, re-wake, two-ended-without-ack person question; first-Job repost, question defaults, closed-task cancel moved from `stream.go`); `internal/task` (`home:` at create); `internal/cli/intent.go` (`grid task move` as a person post plus `home:` rewrite); status block and inbox writers | (1) an exit wakes one orchestrator session (stub) with only the new post ids; (2) kill the session: the next wake carries the unacknowledged events; (3) a remote login's launcher never wakes; (4) status block rewritten from posts; an edit to the rest of task.md between read and write is kept; (5) inbox section rewrite keeps other sections; (6) after `grid task move` only the new home wakes | 4, P-brain-board-decide (status block format) |
| 6 | P-loop-notes | opus xhigh | `internal/loop` (note lock, per-login files, merge, 60 s write); `internal/account` (`grid/accounts/<id>.md` writer at `grid account add`, reader; registry.json migration) | (1) two logins: one note, one writer, takeover when the writer dies; (2) `updated` advances every minute; (3) no token or credential string in the note (secret scan on fixtures); (4) the account id in an assignment maps to this login's config folder | 4, P-brain-obsidian-decide (paths) |
| 7 | P-loop-service | opus xhigh | `internal/hook` (SessionStart detached start with a clean environment); `internal/loop/install` (systemd `KillMode=process`, launchd `KeepAlive`/`AbandonProcessGroup`, schtasks at logon); `grid sync` child when licence and server are set | (1) SessionStart starts one launcher and a second SessionStart starts none (lock); hook stays under its timeout; (2) unit, plist and XML goldens; (3) restart of the service leaves a running stub agent alive and the new launcher adopts it; (4) no `GRID_JOB`/`CLAUDE_*` from the session in the launcher; (5) `grid sync` started only with licence and server | 4 |
| 8 | P-loop-remote | opus xhigh | `internal/cli` (`grid run --on`), nudge file, sync-push hook point, ssh start of a stopped launcher | (1) `grid run J-n --on vm:grid3` from grid2 starts it as grid3 within a pass (nudge); (2) a stopped target with no service: ssh to the login starts it; (3) on the VM from the Mac: J-n starts as grid2 and grid3, posts show in the Mac brain (live, checker) | 4, 7, grid sync server push (app/sync side) |
| 9 | P-loop-routines | opus high | `internal/routine` (routines become recurring Jobs with `every`; timers removed after migration) | (1) planner refill and end review run from `every` conditions with no OS timer; (2) migration lists and removes the old timers | 3, 4 |
| 10 | P-loop-grants | opus high | `internal/contract/fold.go`, `internal/orchestrator/person*.go`, `internal/mcp` (author from environment; `for`/`by`/`grant`) | (1) a granted answer folds as the person's; (2) expired, wrong-agent or no grant folds as the agent's own; (3) mcp ignores an author argument | 1, P-brain-obsidian-decide (people folder) |
| 11 | P-loop-retire | opus high | remove `internal/lease`, `grid lease`, the task-lease paths in `stream.go`/`tick.go`, `feed.leaseLock`; `grid worker next/list` and `internal/worker` after the app lane | (1) `go list` shows no importer of `internal/lease`; (2) walk tests green; (3) every removed test names its covering test or the owner's direction | 4, 5, A-worker-out |
| 12 | A-worker-out (app) | opus high | app: grid-worker out of the run path; board, machine-note and status-block readers for the view | (1) the app runs no agent and no Judge; (2) the task page shows the launcher's posts and the machine notes | 1, 6 |
| 13 | E-run-jobs-acceptance | opus xhigh (runner and checker) | none (run only) | the owner's acceptance below, on the VM with the app not installed | 1-8, P-judge-land-decide lanes |

### The owner's acceptance mapped to lanes

| Owner acceptance | Proven in |
|---|---|
| A task from `grid task new` runs to delivered on the VM with no app and no hand step (orchestrator, Jobs, launcher on grid..grid4, review, land) | 13, built from 4, 5, 7 and P-judge-land-decide |
| A research Job starts beside a running build because the orchestrator posted "now" | 4 row (3), 13 |
| A build waits on "no test run" until the test run ends, shown in the posts | 4 rows (2) and (4), 13 |
| Kill the orchestrator session mid-task: Jobs keep running and a new orchestrator wakes when one finishes | 5 rows (1) and (2), 13 |
| Kill a launcher: the hook or service restarts it, it resumes from the brain, no Job runs twice | 4 rows (5) and (6), 7 rows (1) and (3), 13 |
| From the Mac, start J-n on the VM as grid2 and grid3 with no hand step; the posts show in the Mac brain | 8 row (3), 13 (live) |

### Open items
- The owner's question in `docs/research/usage-direct-read.md:141` (`get_usage` probe or direct call) decides one source of `account_room`.
- Whether VM logins may enable their own linger (UNVERIFIED). Otherwise it is a one-time step by the person.
- Inbox: one note with sections by several writers (decided here), or strict single writer with one note per machine (owner).
- Re-running tests after a mechanical clean rebase before the push: P-judge-land-decide.
- `internal/lease/activity.go` and the full MCP tool list were not read here (UNVERIFIED); lane 11 reads them before removal.

DONE
