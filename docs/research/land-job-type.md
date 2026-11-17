# Landing as a first-class Grid Job (R-land-job-type)

Research and plan only; no code changed. Owner question of 17 Nov: why does this build land through serial, all-or-nothing shell batches instead of on Grid's own feed, Jobs, planners and reviewers, and what would a land Job look like.

Sources were read on 17 Nov 2026:
- Plugin files are cited as `path:line` in this repo at the lane base 2fdfe3eb.
- App files are cited as `app <path>:line`, read with `git show main:<path>` on the app ref repo.
- The landing scripts are cited as `land.sh:NN` (and the other script names). They live in the build's hand folder, were read only and are not in this repo.
- A fact without a source is marked UNVERIFIED.

The project brain holds no notes yet: the SessionStart summary shows 0 entries in every topic. Every fact below therefore comes from code or scripts.

## 1. What the plugin has today for the life of a change after review

**Answer:** a change ends at "accepted" on its task's Feed. Nothing in the plugin or the worker merges an accepted change onto a shared branch, checks the merged tree, or orders several changes. Landing exists only as a person-run `grid task merge` and as the deploy-after-ship `grid land`.

**Job kinds and roles**
- A Job has a free-form `kind` (`internal/job/job.go:15`). The only kinds code branches on are `non-code`, `research` and `scout`, which skip the code floor (`internal/jobgraph/plan.go:139`, `internal/jobgraph/floor.go:43`). The verify kinds are `file-exists` and `reviewer-check` (`internal/jobgraph/floor.go:48`).
- `Person` is the one special role (`internal/job/job.go:174`). Every other role is an agent file checked by `KnownRole` (`internal/job/lint/lint.go:145`).
- Dependencies are `after` edges of kind `done`, `failed` or `children-done` (`internal/job/lint/lint.go:252`). They point at Jobs of the same task's Feed. A target that is not on the Feed is held back (`internal/orchestrator/ready.go:100`, `splitMissingTargets`).

**Run states and how a Job ends**
- The states are posted, ready, assigned, running, submitted, accepted, stale, parked, rejected and cancelled (`internal/contract/types.go:89-100`). An `integrated` status exists in the control schema and reads as Needs review, but no writer sets it (`internal/contract/types.go:172-179`).
- When a worker exits with `done`, the tick moves the Job to submitted with the reason "awaiting judgement" (`internal/orchestrator/tick.go:329-330`). Accepted, cancelled, rejected, posted and ready Jobs are not advanced further (`internal/orchestrator/tick.go:288`).
- The verdict is applied by the app worker's Judge, not by the plugin:
  - After a build, the Judge posts the review Job when the next workflow step is a `check` (app `internal/kernel/worker/judge.go:19-31`).
  - On ACCEPTED it writes the control `accepted`, "review accepted by R" on the build Job (app `judge.go:1039-1052`).
  - On REJECTED it posts a rework while review rounds remain, then a person Job (app `judge.go:1054-1066`).
- Accepted is the end. The workflow step kinds are only `check`, `ask`, `approve`, `make-together`, `run` and `schedule` (`internal/workflow/parse.go:440-443`). There is no merge or land step.

**The feed and the launch path**
- `grid worker next` is one long stream:
  - Each pass takes every task's lease, ticks it and prints `spawn`, `wake` or `skip` lines (`internal/orchestrator/stream.go:31-43`, `internal/cli/worker.go:84-216`).
  - It states that it "adds no cap, delay or rate hold on Jobs" (`stream.go:42-43`).
  - A spawn line carries the task, Job, fence, model, effort, tools and repo, but no command (`stream.go:125-145`).
- The app's supervisor dispatches every assigned spawn; the scheduling itself stays in grid (app `internal/kernel/worker/supervisor.go:369-372`).
  - A spawn without `Command` is built into a `claude -p` run in a fresh worktree (app `supervisor.go:374`, `agentbuild.go:100-270`).
  - A spawn with `Command` is run as given, with no agent (app `internal/kernel/worker/job.go:37-45`, `agent.go:73`). No plugin path sends one today.
- The worker already merges branches into a Job's worktree. A Job's worktree starts from HEAD and merges the submitted branches of the Jobs it waits on, "so it needs no one to land first" (app `agentbuild.go:154-170`). A conflict becomes the Job's result (app `supervisor.go:443-449`).
  - Branches are named `grid-<task>-<job>-a<attempt>` (app `agentbuild.go:148-150`).
  - The worker comments that "the land merges builds" (app `agentbuild.go:427`), but no land Job exists to do it.

**What merges onto a branch today**
- `grid task merge --task` merges the branch `grid deliver` recorded into its base branch with `--no-ff`, in the project's checkout. On a conflict it aborts and tells the person to merge by hand. It runs no check and is started by a person (`internal/cli/task.go:108-236`).
- `grid land` (package `internal/land`) runs the command of the project's deploy topic after a ship person Job is accepted, once per task (`internal/land/land.go:1`, `internal/cli/ship_land.go:28-89`). It is a deploy, not an integration: no merge and no check.

**Missing for landing:**
- **M1. No land Job.** Nothing takes an accepted change, merges it onto the current target, runs the project's check, and moves the target only on green.
- **M2. No poster.** The Judge stops at `accepted` (app `judge.go:1051`). No workflow step kind says "land after accept" (`parse.go:441`).
- **M3. No dependencies between changes of different tasks.** `after` edges point inside one Feed (`ready.go:100`). Stacked changes (a fix on a lane, an M- merge lane) have no way to say "land after that one".
- **M4. No command spawn from the plugin.** `spawnLine` has no `command` (`stream.go:125-145`), so every Job, even a mechanical one, starts an agent.
- **M5. No check command in the brain.** `knowledge/deploy` rules hold target, branch, command and needs-approve for deploys (`internal/land/land.go:28-35`), but no "check before merge" entry.
- **M6. No order between Job kinds on CPU.** The worker sets no niceness or priority on any spawn: `git grep` for setpriority or nice in the app returns nothing outside machine stats (app `internal/platform/machinestats/stats.go:130`).

## 2. What the scripts do, and where each rule goes

The land pipeline is: `collect.sh` runs every pass and starts `landpair.sh` (`collect.sh:79-89`); `landpair.sh` runs `landbatch.sh`, which picks lanes and starts `landrun.sh` per repo (`landbatch.sh:112-113`); `landrun.sh` runs `land.sh`, then `skip2merge.sh` and `eject2fix.sh` (`landrun.sh:8-25`).

Where each rule goes:
- **Land Job:** what one change's landing does.
- **Brain:** the project's workflow or conventions note; a project choice, read at run time.
- **Disappears:** gone once each change lands on its own.

| Rule (script line) | What it does | Where it goes |
|---|---|---|
| One landing per repo at a time, `flock` (`land.sh:20-21`) | Serialises landings | **Disappears.** The move of the target is a compare-and-swap, `git update-ref refs/heads/main <new> <old>`, already used at `land.sh:315`. Two land Jobs can check in parallel; the loser of the swap re-merges. |
| Attribution check (`land.sh:18-19`, `landbatch.sh:66-67`) | Reports Co-Authored-By and similar lines | **Brain** (conventions note: "no attribution lines"), checked by the reviewer of the change, not at land time. Today it only prints, so nothing is lost. |
| Merge each lane, SKIP on conflict (`land.sh:55-61`) | A conflict leaves the lane out | **Land Job.** A conflict fails that land Job alone and proposes a merge Job for a builder. Today `skip2merge.sh` writes an M- brief per SKIP (`skip2merge.sh:11-50`). |
| Generated-file conflicts: take theirs and rerun `make ops` (`land.sh:23-25`, `40-41`) | Resolves conflicts in generated tables and goldens | **Brain** (environments note: "files that `make ops` writes; regenerate on conflict"). The land Job reads the generated-file list and the regenerate command from the brain and applies them in its worktree. The rule stays, but per change. |
| Union merge of ui-seed files plus `node --check` (`land.sh:33-37`, `53`) | Lets shared seed files merge | **Brain** (environments note, as a `.gitattributes` line that the repo itself can carry). Git's built-in `merge=union` does it, and the check command catches a broken result. |
| TRAIN-DROP: drop a merged lane whose stacked lane was skipped (`land.sh:62-72`) | Never lands half a stack | **Land Job dependencies.** A land Job waits (`after`) for the land Jobs of the changes it stacks on, so half a train cannot land. |
| MAXB, at most 12 lanes a batch, the rest LATER (`land.sh:56-57`, `landbatch.sh:106-108`) | Batch cap | **Disappears**: there are no batches. |
| Checks: `go test ./...`, the Windows and macOS builds, `make lint`; app: `ops-check`, `webui-test`, `test-lib` (`land.sh:75-98`) | The project's check | **Brain** (environments note: "check command", e.g. `make check`). The land Job runs whatever the note names. |
| Cross-repo r18 and the cross-floor pass count (`land.sh:101-108`) | Runs the app's real-plugin suite against a plugin change | **Brain** (part of the plugin project's check command), run by the land Job. The floor is a guard that a check can add; it is not a Grid rule. |
| Rerun the failing packages once, RERUN-PASS as a flake finding (`land.sh:83-92`) | Flake tolerance | **Land Job:** rerun the failing tests alone; pass means land and propose a flake Job (section 3). |
| Bisect before blame, EJECT, MAIN-FAULT, DEFER, package rule (`land.sh:144-271`) | Finds which lane of a batch broke a test | **Disappears.** With one change per land Job the culprit is that change. A test red on main alone is the MAIN-FAULT case: the land Job runs the failing test on the target alone and, when red there too, says so and proposes a fix Job against main, not against the change. |
| FLAKE-RERUN of the whole batch (`land.sh:286-291`) | Batch-level flake retry | **Disappears**, replaced by the per-test rerun. |
| Eject leads to a `-fixN` brief and launch (`eject2fix.sh:7-47`, `landrun.sh:19-25`) | A fix round for the builder | **Land Job:** a real red fails the land Job and proposes a fix Job, which continues the change's build Job. The Judge's rework path already exists (app `judge.go:1059-1062`). |
| SKIP leads to an `M-<id>-onto-main` brief and launch (`skip2merge.sh`, `landrun.sh:10-18`) | A merge round | **Land Job:** a conflict proposes a merge Job (builder, base = the change's branch, merges the target). |
| HOLD and HARD lists by hand (`landbatch.sh:3-10`, `22-27`) | Keeps lanes out | **Disappears**, replaced by a dependency or a person Job. A held change is one whose land Job waits on a person Job ("owner decides X") or on another land Job. |
| STACKED-OUT, BASE-OF-STACK, BASE-OF-M, CARRIES, CARRIED-OUT (`landbatch.sh:14-20`, `40-49`, `90-101`) | Lands a stack through its top lane | **Land Job dependencies.** With `after` edges the base lands first and the stacked change after it. Carrying is not needed. |
| EJECTED-PENDING: an ejected lane stays out until its head moves (`landbatch.sh:31-38`) | Avoids relanding a red lane | **Disappears**: a failed land Job is closed, and the fix Job's acceptance posts a new land Job. |
| OBSOLETE-M, SUPERSEDED-M, SKIP-TWICE (`landbatch.sh:68-85`) | Prunes merge-lane generations | **Land Job:** `git merge-tree --write-tree` equal to the target's tree means "nothing to land" and the Job ends accepted with that reason (`landbatch.sh:70` already uses it). Generations disappear because a merge Job continues the same land chain. |
| NOT DONE closing (closedby.sh), `landbatch.sh:53-65`, `skip2merge.sh:46` | Rewrites NOT DONE lines in lane docs before landing | **Disappears.** On Grid, a change's state is the Job's status on the Feed, not a line in a lane doc. Until the cutover it stays a brain convention for lane docs. |
| Order: fixes first, then M-, then the rest (`landbatch.sh:102-105`) | Batch order | **Disappears.** Land Jobs run as they become ready. The Feed's `order` record can still move one ahead before it starts (`internal/contract/fold.go:241-249`). |
| `main moved` refuse (`land.sh:313-315`) | Tested tree equals landed tree | **Land Job:** kept, as the compare-and-swap (section 3). |
| landpair: rebuild plugin-dist, repin the app's PLUGIN_SHA, rebuild the dev UI, run-log row (`landpair.sh:17-43`) | After-land steps | **Brain workflow:** a `run` step after land (or a routine). The repin is a normal Job posted on the app project. |
| collect.sh: start landings when main has been unmoved for 5 minutes and the inputs are new (`collect.sh:77-90`) | Pacing | **Disappears**: land Jobs are ready as soon as they are posted. |
| `land-hold`, `m-hold` files (`collect.sh:78`, `landrun.sh:12-13`) | Person stop | **Brain workflow:** an `approve` step, or a person Job that the land Jobs wait on. |

## 3. Design of a land Job

**Answer:** a land Job is a runner Job with `kind: land`, one per accepted change. It runs a plugin verb, not an agent:
1. Merge the change onto the current target in a worktree.
2. Run the project's check command from the brain.
3. Move the target by compare-and-swap on green.

Changes land in parallel through speculative stacking, so the tree that was tested is the tree that lands.

**Posting**
- The project's workflow table gains a step kind `land` after the check step (`internal/workflow/parse.go:440-443`).
- When the review verdict is ACCEPTED, the Judge already writes `accepted` on the build Job (app `judge.go:1050-1052`). With a `land` step next, it also posts the land Job, as it posts the review Job after a build today (app `judge.go:19-31`).
- With no `land` step the workflow is unchanged, so projects that do not want this get nothing new.

**Where land Jobs live**
- All land Jobs of a repo go into one standing landing task of the project, the way the project has a standing Planner task (`internal/task/lane.go:50`).
- The reason is that `after` edges stay inside one Feed (`ready.go:100`, gap M3). The edge "land after the change it stacks on" is then a normal same-Feed edge between two land Jobs.
- The dependency on the change's own build Job is met by construction: the land Job is posted only after that Job is accepted.

**Body**
- `repo`, `target` (default the project's base branch), `branch` (the accepted Job's submitted branch, app `agentbuild.go:602-620`) and `sha` (pinned at accept).
- `after`: the land Jobs of the changes whose commits `branch` contains. The poster finds them with `git merge-base --is-ancestor <other branch> <branch>`, the test `landbatch.sh:46-51` uses today.
- `verify.command`: the land verb, e.g. `grid land change --task <landing task> --job J-N`.

**Run (no agent)**
1. **Spawn.** The plugin's spawn line gains a `command` for `kind: land` (gap M4). The app supervisor already runs a spawn with a Command without building an agent (app `supervisor.go:374`), and posts the ack and the exit (`supervisor.go:425`).
2. **Worktree.** `git worktree add --detach <home>/worktrees/<task>/<job> <base>`, where `<base>` is the target plus the pending land Jobs ahead of it (below).
3. **Merge.** `git merge --no-ff <sha>`.
   - On a conflict: if every file is on the brain's generated-file list, take theirs and run the regenerate command (rule from `land.sh:40-41`).
   - Otherwise abort, fail the Job, and propose a merge Job naming the files (`git merge-tree --write-tree --name-only`, as `skip2merge.sh:27` does).
   - If the result's tree equals the base's tree, there is nothing to land: accept with that reason (`landbatch.sh:70`).
4. **Check.** Run the brain's check command in the worktree.
5. **Red check.**
   - Read the failing tests from `go test -json` output, the way `land.sh:235-245` reads r18.
   - Rerun those tests alone (`go test -count=1 -run '^T$' ./pkg`) on the same tree.
   - All pass: it is a flake. Land, and propose a flake Job naming the test and both runs (`land.sh:83-92` treats this as a finding).
   - Some still fail: run them on the base alone.
     - Red on the base too: this is MAIN-FAULT. Propose a fix Job against the target and fail the land Job with "target red". This Job does not wait on that fix: it fails, and the next accepted change re-posts.
     - Green on the base: the change broke the test. Fail the land Job and propose a fix Job that continues the build Job (rework, app `judge.go:1059-1062`).
6. **Land.** On green, run `git update-ref refs/heads/<target> <tested sha> <expected old>`.
   - If the swap fails because the target moved, the land Job re-merges onto the new target and checks again (step 3).
   - The tested tree is always the landed tree, which keeps the guarantee of `land.sh:313-315`.
7. **Result.** Post `grid job result` with the target sha, the check exit and the flake list. Proposed Jobs go in `proposed_jobs` (`internal/contract/types.go:146`), where the orchestrator turns them into Jobs.

**Parallelism: speculative stacking, chosen with numbers from this box**
- **Check time.** The time from the LOCK line to the end of the landing log:
  - Plugin: Pint218 19:44 to 19:51, 7 minutes; Pint217 16:55 to 17:07, 12 minutes. Each is one lane, with test, lint and cross.
  - App: Aint1785 21:32 to 21:37, about 5 minutes; Aint1788 00:13 to 00:28, 15 minutes for two attempts, about 7 minutes each.
  - Call it T ≈ 5-12 minutes per full check.
- **Arrivals.** Aint1789 listed 16 accepted app lanes waiting (12 merged, 4 LATER), and collect.sh only counts lanes accepted within 3 hours (`collect.sh:65-76`). That is at least about 5 an hour per repo; the peak rate is UNVERIFIED.
- **Serial throughput.** One change at a time lands 60/T ≈ 5-12 an hour, about equal to arrivals, so a single flake stalls the queue. That is today's 3-hour freeze.
- **Per-change red rate.** It is low: 1 EJECT in each of Aint1788 (1 of 13 lanes) and Pint210, and none in the 14 other recent logs I counted. Most NOT LANDED batches were conflicts or main-fault flakes (Aint1787: `TestRealPluginE0812TaskStopClosesTheJobForGood` red on main alone).
- **Option A: test each on main, merge-tree check at merge.** No check is wasted. But the tested tree is not the landed tree. Two changes each green on main can be red together: Aint1788's eject was red only together with the other lanes, and the union-seed double declaration of `land.sh:33`. That loses today's guarantee, so A is rejected.
- **Option B: speculative stacking, a merge queue.**
  - Land Job N checks target + the merges of the pending land Jobs ahead of it + N. When those land, N's tested tree is exactly what lands, so it can swap at once.
  - A red at position k wastes the checks behind k. The expected waste is p × (queue depth) × T: at p ≈ 0.05 and depth 6 this is 0.3 T per change, cheap.
  - B keeps the land.sh guarantee and lets one red change fail alone. **Pick B.**
- **CPU.** `go test ./...` already spreads over all 12 cores (the core count is from `reserve-cpu.sh:3`), so parallel checks buy independence (one red never blocks the rest), not much speed. Queue depth is not a Grid cap. It follows from the dependencies: a land Job is ready when the land Jobs it stacks on are posted, and the person can still order one ahead (`fold.go:241-249`).

**Built-ins only**
- git worktree, merge, merge-tree, update-ref compare-and-swap and merge=union attributes; `go test -json` and `-run`.
- No lock file, no batch cap, no hold list. Approvals stay in the brain workflow as `approve` steps.

## 4. Capacity: CPU for land Jobs before builders

**Answer:** use the operating system's scheduler priority, not a slot or a cap. Builders already run as unprivileged children of the worker, so the worker starts agent spawns at a lower priority (higher nice) and runs command spawns (land Jobs) at the worker's own priority.

- **Today.** No priority is set on any spawn (gap M6). The scripts get land priority from `nice -n -15` (`landbatch.sh:111-113`), which needs root to go below 0, and from `reserve-cpu.sh`, which pins lane users to CPUs 4-11 at +10 (`reserve-cpu.sh:2-8`). Pinning is a limit on agents; the owner rule rules it out.
- **Proposal.**
  - The app's `PlainLauncher.Start` (app `internal/kernel/worker/job.go:266-300`) raises the niceness of a spawn by the amount the brain's machines topic names (`agent nice: 10`), only for agent spawns. A command spawn keeps the worker's niceness.
  - On Linux this needs no privilege: a process may always raise its own child's nice. Under CFS, nice 0 weighs 1024 and nice 10 weighs 110 (kernel `sched_prio_to_weight`, UNVERIFIED on this kernel build). A running check therefore gets about 9 times the CPU share of each builder, and builders still use every idle cycle.
  - Nothing is paused, capped or refused. With no setting nothing changes, as the account pick does (app `account.go:62-67`).
- **Order inside a pass.** The tick assigns every ready Job in one pass (`tick.go:177-195`), so there is no queue for a kind to jump. No priority between kinds is needed beyond niceness.
- **Owner decision, not a lane.** A negative nice for land Jobs, as `landbatch.sh:112` uses today, needs root or CAP_SYS_NICE on the worker; the worker should not ask for that. With the +10 on agents the effect is the same.

## 5. Moving this build onto Grid

Beyond landing, the scripts do the following. For each, the existing plugin or app feature is named, or the gap:

| Script job | Script | Plugin or app feature today | Gap |
|---|---|---|---|
| Launch a builder lane in its own clone on a branch | `handlaunch.sh:2-5`, `planlaunch.sh:2-4` | The worker's worktree per Job on `grid-<task>-<job>-a<n>` (app `agentbuild.go:148-170`) | None |
| Start from another lane (stacked, M-, fix) | `handlaunch.sh:3` base-lane | The dependency merge into the worktree (app `agentbuild.go:154-170`); a rework continues the build (app `judge.go:1059-1062`) | None |
| Multi-account lanes, fewest running agents | `pickuser.sh:2-8` | Account selection, round robin or smart, per Job (app `internal/kernel/worker/account.go:57-72`) | None. The settings say which accounts. |
| Limit switching after a usage-limit stop | `mark-limits.sh:2-3`, `relaunch-limit.sh:2-7` | Park on `usage_limit` and reoffer when capacity is back (`tick.go:291-294`, `320-322`); "full at" thresholds in the account pick (app `account.go:62`) | None found |
| Review after build, one rework, then a person | `dispatch.sh:2-4` | The Judge (app `judge.go:19-31`, `defaultReviewRounds`) | None |
| Accepted list | `dispatch-accepted.txt`, `handaccept.sh` | Feed status `accepted` (`types.go:95`) | None |
| Landing | `land.sh` and the rest | Gaps M1-M6 (section 1) | Lanes L1-L5 (section 6) |
| Scope maps and backlog planners | `mapjobs.sh:2-3`, `collect.sh:12-35` | Planner role and the project's standing Planner (`internal/task/lane.go:50`); routines topic (`internal/routine/store.go:1`) for the cadence | Map-to-planner chaining is a routine plus a planner Job; the templates move into the brain. UNVERIFIED that a routine can post a Job on another task. |
| Folder reviews (R-fold) and end reviews | `collect.sh:41-60` | Reviewer with "end review" in the Job reviews a whole phase (agent list: grid-reviewer) | The end-review cadence becomes a routine |
| Throughput and run log | `throughput.sh`, `runlog-lanes.sh` (`collect.sh:9-10`, `96-98`) | Usage records per task (`internal/usage/usage.go:1`); the app's runs pages | UNVERIFIED whether the app shows per-Job cost as the run log does |
| Prune clones | `prune-clones.sh` (`collect.sh:11`) | The worker prunes worktrees of closed tasks (app `internal/kernel/worker/prune.go:227`) | None |
| Plugin repin and dev-UI rebuild after a land | `landpair.sh:17-36` | None | A `run` step or routine after land (L5) |

**Order of steps to move, smallest first**
1. Write the brain notes for the plugin and app projects:
   - environments: the check command (the content of `land.sh:75-108`), the generated-file list and regenerate command, and the union attributes;
   - conventions: no attribution lines, walk tests;
   - machines: 12 cores and the agent nice value.
   No code is needed for this.
2. Land L1-L3 (plugin): land Job kind, `grid land change`, flake and fix proposals. Run `grid land change` by hand from the scripts in place of `land.sh` for one repo (plugin first: smaller check). This proves per-change landing while the rest of the pipeline stays.
3. Land L4 (workflow `land` step) and L5 (app: Judge posts land Jobs, command spawns, agent nice). New lanes for the plugin repo go through `grid task new` and the worker instead of `handlaunch.sh`. The scripts keep landing the app.
4. Move the app repo's landing to land Jobs, which needs the r18 cross step expressed as the plugin check command. Retire `landbatch.sh`, `landpair.sh` and `collect.sh` step 3.
5. Move maps, planners and end reviews to routines and planner Jobs (section 5 table). Retire `collect.sh` and `mapjobs.sh`.

**Before the cutover on 23 Nov**
- Steps 1-3 must have landed: L1-L5, and the brain notes.
- Step 4 is the cutover itself. Step 5 can follow after it, because the scripts that start planners do not depend on landing.
- The risk is time: L2 and L5 are the large lanes, and L5 is in the app and needs the plugin pin moved.

## 6. Lane plan

**Naming.** `internal/land` and `grid land` already mean deploy-after-ship (`internal/land/land.go:1`). Keep that package as it is, and put the new code in `internal/landjob` with the verb `grid land change`. `grid land` with no subverb keeps running the deploy, so no existing caller changes. This is UNVERIFIED against `grid land`'s current argument parsing: L2 checks it in its first reads and picks `grid job land` if `change` collides.

| Lane | Repo | Model and effort | Files | Acceptance rows | Depends on |
|---|---|---|---|---|---|
| L1-land-kind | plugin | sonnet, high | `internal/job/job.go`, `internal/job/lint/lint.go`, `internal/contract/types.go` (doc only), tests | 1. `kind: land` with role runner passes `grid job lint`; it needs `target`, `branch` and `sha` and refuses each missing one with its own code. 2. A land Job's record round-trips through `grid feed post`/`grid feed state`. 3. The existing lint tests stay green; walk tests green. | none |
| L2-land-change | plugin | opus, high | new `internal/landjob/`, `internal/cli/land_change.go`, the `inprocess.go` walk table, tests with `internal/testgit` | 1. In a test repo, green check: the target moves to the merged sha by `update-ref` CAS; exit 0. 2. Red check: the target is unchanged and the exit is not 0. 3. Conflict: the target is unchanged and the result names the conflicted files. 4. Target moved between merge and swap: it re-merges and checks again, and the landed sha's tree equals the tested tree. 5. Tree unchanged: "nothing to land", accepted. 6. Generated-only conflict with a brain regenerate command: regenerates and lands. 7. The check command and generated list are read from the brain environments note, never hard-coded. | L1 |
| L3-land-red | plugin | sonnet, high | `internal/landjob/` (red path), tests | 1. A test failing once then passing alone: lands, and the result proposes one flake Job naming the test. 2. A test red alone on the merged tree and green on the base: fails, and proposes a fix Job that continues the build Job. 3. A test red on the base too: fails with "target red" and proposes a fix Job against the target. 4. Conflict: proposes a merge Job naming the files. | L2 |
| L4-land-step | plugin | opus, medium | `internal/workflow/parse.go` (step kind `land`), `internal/orchestrator/stream.go` (spawn line `command` for `kind: land`), workflow docs, tests | 1. A workflow table with a `land` step after `check` parses; a table without one is unchanged. 2. The spawn line of a `kind: land` Job carries `command` (the `grid land change` argv) and the other spawn lines are byte-for-byte unchanged. 3. The landing task: a land Job whose branch contains another pending land Job's sha gets an `after` edge on it. 4. Walk tests and `go test ./internal/orchestrator/... ./internal/workflow/...` green. | L1, L2 |
| L5-app-land | app | opus, high | `internal/kernel/worker/judge.go` (post the land Job on ACCEPTED when the workflow's next step is `land`), `job.go` PlainLauncher (agent nice from the machines note), PLUGIN_SHA repin, tests, `make realplugin` | 1. ACCEPTED with a `land` step: one land Job is posted on the landing task with the accepted branch and sha; without one, nothing changes. 2. A command spawn starts with no worktree build and no agent; its ack and exit are posted. 3. An agent spawn runs at the nice value of the machines note, and a command spawn at the worker's own. 4. Real-plugin test: build accepted, land Job lands it on a test repo's main. | L4 |
| L6-feed-land-ui | app | sonnet, high | Feed and Job views (webui) | 1. The Feed shows land Jobs with target, sha, check exit and the flake, fix or merge Jobs they proposed, from the served feed state (no assumed field). 2. A failed land shows its reason. 3. `make webui-test` green; screens checked by a checker after landing. | L5 |
| Brain notes | (no repo) | orchestrator, by hand | Project brain: environments (check command, generated files, regenerate command), conventions, machines (agent nice) | Each note readable with `grid brain show`, and `grid land change` in L2's test reads the same shape. | none; before L2's real use |

Owner decisions to ask, not lanes:
- (a) Agent nice +10 as the default when the machines note names none, or no change by default.
- (b) Whether the cross-repo pass floor (`land.sh:105-108`) stays part of the plugin check command.

DONE
