# Dogfood gaps: what grid-plugins needs to run this build instead of the shell scripts

Research lane R-dogfood-gaps-2, 17 Nov. It supersedes R-dogfood-gaps, which judged "plugin + grid-worker".

The owner corrected the target on 17 Nov at about 01:00 UTC:
- **The plugin runs parallel Jobs across the local accounts of one machine.** Here those accounts are the Unix logins grid, grid2, grid3 and grid4.
- **grid-worker is only for running work on other computers.** It is out of scope for the first stream.
- Every place the plugin cannot do what a script does is a plugin gap, never a new script.

How things are cited:
- `plugin:` is this repo at the lane base (main 62876ff3).
- `app@e7113f02b:` is the app repo, read only from an app lane clone. These lines were grepped again for this doc.
- Script names are the orchestrator's VM scripts, all read only. They are `launch-claude.sh`, `launch.sh`, `hand/handlaunch.sh`, `hand/pickuser.sh` and `hand/dispatch.sh`, re-read for this doc.
- Lines marked "(R-dogfood-gaps, not re-read)" are taken from the first lane's doc. They name `hand/collect.sh`, `hand/landpair.sh`, `hand/mark-limits.sh`, `hand/throughput.sh` and `hand/reserve-cpu.sh`.
- STATE.md, the run log and the autolauncher are in the orchestrator's own home, which a lane login cannot read.

## Gap table

| # | Pipeline piece | Scripts today | Plugin today | Gap | Lane |
|---|---|---|---|---|---|
| G1 | **Start an agent for a posted Job** | `launch-claude.sh:26` runs `su - <user> ... claude -p --model --effort --permission-mode dontAsk --settings --max-turns` detached. Only root can run it. | Post, lint, assign, render all work (smoke steps 4-9). The tick assigns a Ready Job to the task holder and records a spawn (`plugin: internal/orchestrator/tick.go:207`). `grid worker next` prints one `spawn` line per assign (`plugin: internal/orchestrator/stream.go:31-43`, `internal/cli/worker.go:84-216`). **Nothing in the plugin reads the spawn line and starts an agent.** The agents say so: "The loop that hands Jobs to builders (`grid feed tick`, `grid job next`) runs in code; you do not call it" (`plugin: agents/orchestrator.md:46`, `agent-templates/grid-planner.md:44`). The only place the plugin starts `claude -p` is a skill routine (`plugin: internal/routine/execute.go:187-199`). | A runner in the plugin. It reads the spawn lines, makes the worktree, starts `claude -p` (or `codex exec`) with the Job's rendered brief, and posts ack, exit and the no-result case. Today the app does all of this (`app@e7113f02b: internal/kernel/worker/agentbuild.go:100,169,210-229`). | **P-run-jobs (first lane)** |
| G2 | **Dispatch across local accounts (logins) by their limits** | `hand/pickuser.sh:5,10` picks the login with the fewest `claude -p`, skipping one whose `limit-<user>.until` is in the future. `hand/dispatch.sh:34-37` writes that file from the agent's `usage_limit_reached` text (weekly and 5-hour). | The plugin registers accounts as **config folders inside one Unix user** (`~/.<id>`, `plugin: internal/account/registry.go:18-27`). It reads their 5-hour and weekly windows (`grid account windows`, `internal/account/windows.go:37-53,148`). A usage-limit park becomes Ready again at the reset time put in the exit code, or when an account has room (`internal/orchestrator/ya_usage_ready.go:11-35`). Nothing picks a Unix login. A Grid home made by one login is mode 0600/0700 for that login (smoke step 12), so a second login can read neither its feed nor its leases. The app's account pick is also one Unix user, `CLAUDE_CONFIG_DIR` only (`app@e7113f02b: internal/kernel/worker/account.go:45,72`). | (a) Run a Job as another login. Either one runner per login on a home they share (group-readable, one holder per runner), or one privileged launcher. This is an owner decision, see section 3. (b) The runner picks the login by its windows and running count, and writes the reset into the exit code that `ya_usage_ready` already reads. | P-run-jobs (b), P-shared-home (a) |
| G3 | **Review: Judge, verdicts, one rework, second reject** | `hand/dispatch.sh:16-29` reads the first line (ACCEPTED, REJECTED or STUCK), does one fix round with must-fix appended (`:22-27`), and sends the second reject to the orchestrator (`:23`). `Review: none` goes straight to integration (`:51`). | Nothing in the plugin reads a review's verdict. A done Job stays `submitted` "awaiting judgement" (`plugin: internal/orchestrator/tick.go:330`, `docs/job-contract.md:128`). The orchestrator agent is told to leave such a Job "waiting for the app's Judge" (`agents/orchestrator.md:28`). The Judge is in the app (`app@e7113f02b: internal/kernel/worker/judge.go:243` Wake, `:1003` verdict, `:1115` rework, `:1226` person). | Move the Judge into the plugin as a pass of the stream: post the review Job, read the verdict, rework once, then a person or lead Job. Also add STUCK as a verdict, and `review: none` per Job or per workflow step. | P-judge |
| G4 | Builder "NOT DONE" and no result | `hand/dispatch.sh:43-50` reads the report's proof table and last line. A missing bundle is STUCK (`:39`). | `grid job tree` shows "no result posted". The orchestrator agent decides what to do (`agents/orchestrator.md:28`). | A runner that sees exit 0 with no result posts it as stuck for the lead, not to review. | P-run-jobs |
| G5 | Briefs into Jobs | A brief file has `Repo:`, `Model:`, review, acceptance rows and a Never line. `launch-claude.sh:22` pastes the runbook and the brief into one prompt. | `grid job post --type job --body <json>` with a full job-record. Smoke: a hand-written body passed lint with one warning (step 7). `read` accepts `repo:<file>:<lines>` (`plugin: contract/fixtures/job-record/valid/basic.json:19`). There is no model, effort or base field. Model and effort come from the role's agent settings (`internal/cli/worker.go:263,297`). | `grid job post --from-brief`, plus per-Job `model`, `effort` and `base`. | P-job-from-brief |
| G6 | One clone and branch per lane, base branch | `hand/handlaunch.sh:15-19` clones from the bare ref repo and branches from main or from `lane-X`, taken from the brief's `Repo:` line. | No worktree code in the plugin (only tests call `git worktree add`). `Repo` on a spawn names the project repo folder (`plugin: internal/cli/worker.go:162-165`). | The runner makes the worktree from the Job's `base` after a fetch. | P-run-jobs + P-job-from-brief |
| G7 | Codex lanes | `launch.sh:18` runs `codex exec --skip-git-repo-check` in tmux. | The plugin only checks that codex is installed and signed in (`plugin: internal/agent/runner.go:103`). It reads Codex windows from session logs (`internal/account/codex_read.go`). | A `codex exec` runner choice per Job or role. | P-run-jobs-codex |
| G8 | Landing | `hand/collect.sh` → `landpair.sh` → `landrun.sh` (R-dogfood-gaps, not re-read). | `grid land` runs the deploy topic after the ship person Job (`plugin: internal/cli/worker.go:170-173`). There is no merge, no eject and no flake rerun. | Owned by R-land-job-type. **Conflict with the correction:** its lane L5 puts "Judge posts the land Job" and "command spawn" in the app (`hand/R-land-job-type.md` section 6). Under the correction both belong to the plugin, on top of P-judge and P-run-jobs. | R-land-job-type L1-L4, then L5 re-scoped to the plugin |
| G9 | Stuck rulings | Marker files `dispatch-stuck-<id>` (`hand/dispatch.sh:29,39,50`). Rulings are dispatch.log lines. | Acceptance rows can be `dropped` with a decision id. `grid job cancel`. `grid brain decide`. | One verb: accept a Job with named rows moved to a new Job, recorded as a decision. | P-job-ruling |
| G10 | Flake handling | Landing reruns a failed check (R-dogfood-gaps, not re-read). | None. | Owned by R-land-job-type L3. | R-land-job-type |
| G11 | State and run log | STATE.md, dispatch.log, `dispatch-accepted.txt` (`hand/dispatch.sh:20,51`), marker files. | The feed is the event log. `grid job tree` works per task. Usage records exist per task (`plugin: internal/usage/usage.go`). | A list across tasks: every Job, its state, account and cost. | P-jobs-board |
| G12 | Monitor alerts to the orchestrator | dispatch.log lines such as "STUCK", "SECOND REJECT" and "usage limit" (`hand/dispatch.sh:23,29,37`). | Feed types `question`, `loop-detected`, `usage-notice` and `worker-exit` (`plugin: internal/cli/contract_commands_test.go:389`). UNVERIFIED whether `grid feed read --wait` crosses tasks. | One wait stream for the lead across tasks. | P-lead-feed |
| G13 | Planners and backlog refill | `hand/collect.sh` scope map → planner lane (R-dogfood-gaps, not re-read). | `grid:orchestrator` and `grid:planner` post Jobs for one task. A workflow routine runs `grid task new --workflow` and the task posts its own J-1 (`plugin: internal/routine/execute.go:137-145`). | A backlog is a project with tasks. Refill is a routine. Needs G1 to run anything. | after P-run-jobs |
| G14 | Scheduled reviews (end review, plugin health, throughput) | Started by the orchestrator and by collect.sh (R-dogfood-gaps, not re-read). | `grid routine add/schedule` with cron or interval timers (`plugin: internal/routine/timer_systemd.go`). A routine runs `grid` argv, a skill, or a workflow task (`execute.go:147-199`). | An end review as a workflow routine works once G1 and G3 exist. | none beyond G1, G3 |
| G15 | Concurrency (~30 lanes) | `hand/pickuser.sh:10` counts running agents per login. The 30-lane target is in throughput (R-dogfood-gaps, not re-read). | "The stream adds no cap, delay or rate hold on Jobs" (`plugin: internal/orchestrator/stream.go:42-43`). | No lane. A limit is a cap, and the lane rules add no cap or hold. Spreading across logins (G2) is the only balancing. Whether to cap is an owner decision. | owner decision |
| G16 | **grid-worker (one row)** | — | — | Across computers, grid-worker adds the hosted relay pull and share push (`app@e7113f02b: internal/kernel/worker/relaypull.go:66`, `sharepush.go`), a worker instance with a control address (`instance.go`), and licence and invite (`cmd/grid-worker/licence.go`, `invite.go`). It also holds today's runner and Judge. Once those move into the plugin (G1, G3), grid-worker keeps only the work across computers. | out of scope for the first stream |

## 1. Each pipeline piece: what exists, what is missing

**Dispatch (G1, G2).** The scripts:
- `launch-claude.sh:7` allows only grid..grid4.
- `:16-19` clones the ref repo, branches and chowns the clone.
- `:22` builds the prompt from the runbook and the brief.
- `:26` switches login with `su -` and starts `claude -p` detached.
- `hand/handlaunch.sh:23` picks the login (Fable always on grid3).

The plugin's half:
- `grid task new` posts J-1 (smoke step 4).
- `grid job post` and `grid job lint` work (step 7).
- One pass of `grid job next` as task holder assigns both Ready Jobs with "why: starting" (step 9, `tick.go:216`).
- `grid job render` writes `jobs/J-2/brief.md` and `inputs.json`, the prompt an agent would get (step 10).
- `grid worker next` streams the same assigns as `spawn` lines with model, effort, tools and repo (`stream.go:125-140`).

The missing piece is the one that reads a spawn and starts a process. The app's grid-worker is that piece today, and it runs everything as the one Unix user it was started as. No `Credential` or setuid appears in `agentbuild.go`. So neither repo can start a Job as another login.

**Briefs (G5).** Each brief line has a home in the job-record:
- `Repo:` → `repo` plus a new `base`.
- `Model:` → new `model` and `effort` fields.
- Acceptance rows → `acceptance[]` (A1.., traces).
- Touches → `write[]`.
- Never line → `out_of_scope[]`.
- "About N calls" → `budget.turns_estimate`.

The runbook rows become the role's agent template and brain conventions, not pasted text.

**~30 parallel lanes across grid..grid4 with limit switching (G2, G15).** The plugin's account model is one Unix user with several config folders. This build's accounts are four Unix logins, each signed in to its own Claude account. The window readings (`grid account windows`) and the reset park (`ya_usage_ready.go`) are the right parts. What is missing:
- a runner per login, or a launcher that can switch login;
- a Grid home that all the logins can share.

**One clone and branch per lane (G6).** The plugin has no worktree code.

**Codex lanes (G7).** The plugin reads Codex windows but has no `codex exec` runner.

**Review (G3).** The plugin stops at `submitted` (`tick.go:330`). The app's Judge does launch, verdict, rework and the second reject. Under the correction this is a plugin gap, not an app dependency.

**Stuck rulings (G9).** The pieces exist (dropped rows with a decision id, `grid job cancel`, `grid brain decide`), but there is no single verb that rules a stuck Job.

**Landing (G8).** Owned by R-land-job-type L1-L4. Its L5 must move from the app to the plugin.

**Flake handling (G10).** Owned by R-land-job-type L3. The plugin has none today.

**State and run log (G11).** The feed is the log per task and `grid job tree` reads it. A view across all tasks, like STATE.md, is missing.

**Monitor alerts (G12).** The feed types exist (`question`, `loop-detected`, `usage-notice`, `worker-exit`). One wait stream for the lead across tasks is missing.

**Planners and backlog refill (G13).** The planner and orchestrator agents post Jobs for one task. Refill can be a routine, but nothing runs the Jobs it posts until G1.

**Scheduled reviews (G14).** Routines with timers exist and a workflow routine starts a task. They need G1 and G3 to do anything useful.

## 2. Smoke run: the plugin alone, one Job

**Deviation from the brief.** The brief asked for the login grid4 and a home under that login's home. This session runs as the lane login **grid2** and cannot switch login (no sudo, by rule). `mkdir -p` on the brief's path was refused by this session's command rules. So the scratch Grid home and the scratch repo are under the lane folder (`.smoke/`, untracked, not committed). The finding does not depend on which login ran it. The plugin was built from this lane and run with `go run` from the scratch repo, because this session may not run a built binary directly.

| Step | Command (home flag left out) | Result |
|---|---|---|
| 1 | `go build ./cmd/grid` to `.smoke/bin/grid` | ok |
| 2 | `.smoke/bin/grid project add` | refused by this session's allow list (a built binary). From here on, `go run github.com/gridconsole/grid-plugins/cmd/grid` in the scratch repo folder. |
| 3 | `git init`, README.md, one commit | ok |
| 4 | `grid project add -name smoke` | ok, project and machine id |
| 5 | `grid task new "Add one line to README.md ..." --project smoke --size small --json` | ok, T-01. It also posts J-1, an orchestrator Job. |
| 6 | `grid job tree --task T-01` | `J-1 orchestrator posted` / `orphan never_picked_up ...: ready and not assigned to any worker` |
| 7 | `grid job lint j2.json`, then `grid job post --type job --job J-2 --body '<builder job-record>'` | lint `ok: true`, one warning (`acceptance-coverage: ... title`). Post accepted. `read: ["repo:README.md:1-3"]` is allowed now. |
| 8 | `grid job next --json` with no holder | **plugin refusal** exit 1: `GRID_MEMBER is required`. With `--member`: `GRID_LEASE_EPOCH or --lease-epoch must be gen.fence; the task holder gets it from: grid lease take task`. `grid lease take task` without a process: `--pid and --pid-start, --session, or --hold, are required`. With `--hold`: ok, fence 2. |
| 9 | `grid --member grid2 --lease-epoch 0.2 job next --task T-01 --json` | `[{"id":"J-1","role":"orchestrator","state":"assigned","why":"starting"},{"id":"J-2","role":"builder","state":"assigned","why":"starting"}]` |
| 10 | `grid job render T-01 J-2` | ok: `jobs/J-2/brief.md` and `inputs.json` written. |
| 11 | `grid job tree --task T-01` | `J-1 orchestrator assigned / attempt 1 fence 3 assigned`, `J-2 builder assigned / attempt 1 fence 4 assigned` |
| 12 | `ls -la` of the home and the task feed | Lock files, `machine`, the feed `*.jsonl`, `state.json` and `lease/` are `-rw-------` / `drwx------` grid2. Another login could not read or append to them. |
| 13 | `grid lease release task --epoch 0.2` | ok, holder null. Nothing else was started, so there was nothing to stop. No kill and no pkill. |

**The exact point where the plugin stops.** After step 9 both Jobs are `assigned`, "starting", to the holder grid2. No process starts and nothing ever will. The plugin has no code that turns an assign or spawn line into an agent (`tick.go:207` records the spawn; `stream.go:31-43` prints it; nothing reads it in this repo). That point is the first plugin lane: **P-run-jobs**.

Other missing steps the scripts do and the plugin does not:
- make a clone or worktree from a base;
- pick a login by its limits;
- start the agent as that login;
- see "no result" or NOT DONE;
- start a reviewer and read its verdict.

## 3. Cutover order

**Owner decision needed first: how the plugin runs a Job as another local login.** The constraints:
- The scripts switch login as root (`launch-claude.sh:26` `su -`). The plugin runs unprivileged and must stay that way.
- One task holder gets every assign of a task (`stream.go:31-34`). So "which login runs it" has to be decided by who holds, or the runner must hand off.
- A Grid home is private to the login that made it (smoke step 12).

There are two options:
- **(A) One runner per login, on one shared Grid home (recommended).** Each login runs `grid run` (P-run-jobs) as itself. The home is group-shared: a `grid` group, mode 0660/2770, which needs a plugin change in how files are created (P-shared-home). Runners pick tasks, not logins. An account at its limit is handled by the existing usage-limit park (`ya_usage_ready.go`), which makes the Job Ready again at the reset. No new hold is added.
  - No privilege is needed.
  - It matches "the plugin runs across local accounts".
  - It works the same on a laptop with one login and one runner.
- **(B) One privileged launcher** that starts each agent as the chosen login.
  - It needs root or a sudoers rule, which the plugin should not ask for.
  - Not recommended.

**First stream: R- research lanes on the plugin repo, posted as Grid Jobs and run by `grid run` on one login (grid4).** The old pipeline keeps everything else. Why R- first:
- They are docs-only and `Review: none`.
- They need no landing merge, no Codex and no checker.
- They exercise post → assign → run → result.

**Smallest set before the first stream can start:**
1. P-run-jobs for one login: worktree, `claude -p` with the rendered brief, ack, exit and the no-result case.
2. P-job-from-brief: post from a brief file, with model, effort and base.

**How it grows:**
1. R- lanes on grid4 only. Pass mark: 5 in a row end with a result and no hand step.
2. Add P-judge. Then P- plugin builder lanes with review run on the plugin, and landing stays on the scripts.
3. Add P-shared-home and a runner on a second login, then all four. Login choice follows the windows.
4. Landing as Jobs (R-land-job-type L1-L4 and L5 in the plugin), plugin repo first, then app.
5. C- app lanes.
6. Codex lanes (P-run-jobs-codex), then UI and checker lanes.
7. Planner refill and end reviews as routines (G13, G14). The scripts retire one at a time as each piece moves.

## 4. Lane plan (priority order)

Landing comes first in the overall order, per R-land-job-type (L1-L4 in the plugin). Its L5 must be re-scoped from the app to the plugin, after P-judge and P-run-jobs. The lanes below are for the rest. Everything is in the plugin repo (P-) unless named.

| # | Lane | Model / effort | Files (expected) | Acceptance rows | Depends on |
|---|---|---|---|---|---|
| 1 | **P-run-jobs** | opus, high | new `internal/runner/`, `internal/cli/run.go` (`grid run`), the `inprocess.go` walk table, tests with `internal/testgit` | A1 `grid run` as the task holder starts one agent per assigned Job. In a test the agent is a stub command. The worktree is on `grid-<task>-<job>-a<n>` from the Job's repo HEAD, and ack and exit are posted. A2 an exit 0 with no result posts the Job as stuck for the lead, not `submitted`. A3 a `usage_limit_reached` exit posts the reset epoch in the exit code, and `ya_usage_ready` reads it. A4 the account (config folder) is picked by `grid account windows`, with the pick reason recorded. A5 `grid run` stops on SIGINT and leaves running agents to finish or be marked stopped (`worker_stopped`, `yc_worker_stopped.go`). A6 no cap, delay or hold is added. | — |
| 2 | P-job-from-brief | opus, medium | `internal/cli/orchestrator.go`, new `internal/job/frombrief.go`, `contract/schemas/job-record.json` (`model`, `effort`, `base`) | A1 `grid job post --task T --from-brief b.md` posts a record that lints with 0 errors. A2 the brief is copied into the task's inputs and intent points at its lines. A3 `model`, `effort` and `base` reach the spawn line, and an empty Job keeps today's per-role values. A4 golden test on 3 real briefs (paths scrubbed). | — |
| 3 | P-judge | opus, high | new `internal/judge/` (port of the app's Judge rules), a stream pass in `internal/orchestrator/stream.go` | A1 a submitted builder gets one review Job. A2 ACCEPTED accepts. REJECTED posts one rework with must-fix, and a second REJECTED raises a lead Job. A3 a STUCK first line raises the lead Job with no rework. A4 a workflow step or Job with no review accepts on exit with a result. A5 the app's Judge tests read the same verdicts: cross-repo check. | 1 |
| 4 | P-shared-home | sonnet, high | `internal/atomicfile`, `internal/feed`, `internal/fslock`, `internal/gridhome` (file modes) | A1 with `GRID_SHARED_GROUP` set (or a home setting), new files and folders are group read/write and setgid. A2 a test with two processes as two holders on one home never assigns the same Job twice. A3 a home with no setting keeps today's modes. | 1 |
| 5 | P-run-login-pick | sonnet, medium | `internal/runner/` | A1 a Job parked on `usage_limit_reached` is started again on another login whose windows have room, through the existing park in `ya_usage_ready.go`. No new hold. A2 the pick reason (`5h 97%, resets 23:30`) is in the attempt record. | 1, 4 |
| 6 | P-run-jobs-codex | opus, medium | `internal/runner/` | A1 a Job with `runner: codex` runs `codex exec` in its worktree. A2 its result post works the same. | 1 |
| 7 | R-land-job-type L5, re-scoped | opus, high | `internal/judge/` (post the land Job on ACCEPTED), `internal/runner/` (command spawn) | As written in R-land-job-type L5 A1-A3, with "app" replaced by the plugin. | 3, L4 |
| 8 | P-job-ruling | sonnet, medium | `grid job` verb, decisions | A1 `grid job rule J-n --move A2,A3 --to-new "<title>" --why` drops the rows with a decision id, posts the new Job and accepts J-n. | 2 |
| 9 | P-lead-feed | sonnet, medium | `grid feed read` | A1 `--for lead --wait` streams stuck, second-reject and landed events across a project's tasks. | 3 |
| 10 | P-jobs-board | sonnet, medium | `grid job list --all` | A1 one list across tasks with state, attempt, login, model and cost. | 1 |
| 11 | C-app-uses-plugin-runner | app, opus, high | `internal/kernel/worker` | A1 grid-worker delegates the local runner and the Judge to the plugin and keeps relay and share. A2 real-plugin tests green. | 1, 3 |

Owner decisions, not lanes:
- (a) Option A or B in section 3.
- (b) Whether any cap on running agents is wanted (G15).
- (c) Which login's Grid home is the shared one, and its group.

## 5. Brain notes to write

Short drafts. Each is filed with `grid brain propose` and accepted by the curator.

- **decisions / plugin-one-machine-many-accounts.** The plugin runs parallel Jobs across the local accounts of one machine (here the logins grid..grid4). grid-worker is only for running work on other computers. Why: the owner's correction of 17 Nov, about 01:00 UTC. The earlier "plugin + grid-worker" target was dropped.
- **decisions / dogfood-gaps-are-plugin-lanes.** This build moves onto grid-plugins step by step. A missing capability is a plugin lane, never a new script. Why: the owner's decision of 17 Nov. grid-plugins is meant as the standard orchestrator, message board, brain and workflow.
- **decisions / review-one-rework.** One rework round. A second reject goes to the lead. STUCK is a verdict that goes to the lead with no rework. Why: `hand/dispatch.sh:16-29`. Keep the same rules when the Judge moves into the plugin.
- **decisions / no-cap-on-agents.** The plugin adds no cap, delay or hold on Jobs, and balancing is by spreading over accounts. Why: `stream.go:42-43` and the lane rules. A cap would need the owner.
- **conventions / brief-to-job-record.** `Repo:` → repo + base, `Model:` → model + effort, rows → acceptance A1.., Touches → write, Never → out_of_scope, "About N calls" → budget. Why: section 1, Briefs.
- **conventions / walk-tests-before-handoff.** Plugin: `go test ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` and `go test ./internal/job/lint/`. Why: these tests read source files and fail on any lane (end review 11 Nov 14:15).
- **conventions / merge-main-before-proof.** Why: 9 ejects in two hours came from main moving under a lane (end review 12 Nov 10:15).
- **conventions / no-attribution-lines.** No Co-Authored-By or session lines. Committed files carry no private paths, hosts or emails. Why: the owner's rule, and the plugin repo is public.
- **machines / vm-lane-logins.** The VM has four lane logins, grid..grid4, each signed in to its own Claude account. Fable runs on grid3. grid3 shares an account with the orchestrator and is picked last while a budget cut is due. Why: `hand/pickuser.sh:4-10`, `hand/handlaunch.sh:23`.
- **machines / grid-home-is-per-login.** A Grid home is 0600/0700 for the login that made it. A second login cannot use it until P-shared-home. Why: smoke step 12.
- **agents / account-limits.** On `usage_limit_reached`, the reset time ("resets 11:30pm (UTC)"; weekly with a date) marks the account until then. New work goes to other accounts, and the stopped Job resumes in its own worktree. Why: `hand/dispatch.sh:34-37`, `internal/orchestrator/ya_usage_ready.go:11-35`.
- **routines / scheduled-checks.** End reviews, plugin health after a plugin landing, and a throughput sample become workflow routines once the plugin runs Jobs. Why: `internal/routine/execute.go:137-145` (a workflow routine starts a task).

DONE
