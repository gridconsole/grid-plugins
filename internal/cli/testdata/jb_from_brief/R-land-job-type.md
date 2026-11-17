# R-land-job-type: landing as a first-class Grid Job, so the build can run on Grid itself

Repo: plugin. Clone <lanes>/R-land-job-type on lane-R-land-job-type from main. Model: opus, medium. About 80 calls (an estimate, not a cap).
Review: none
Skills: grid:research
Why: owner question 17 Nov (<briefs>/landing-batches-2026-11-17.md): why does this build land in serial all-or-nothing batches through shell scripts (collect.sh, landbatch.sh, land.sh in <hand>) instead of on Grid's own feed, Jobs, planners and reviewers? The owner wants landing as a Job type the orchestrator posts on the feed, one per accepted change, with dependencies, so parallel lanes land independently and this build can move onto Grid (dogfood). Owner rule [[no Grid-made limits]]: use built-ins (git merge-tree, worktrees, Claude Code/Codex) before building any guard; approvals come from the brain workflow, not hard-coded. Research and plan only, no code change.

## Questions (evidence: file:line in the plugin, or the script line in <hand>)
1. What the plugin has today for the life of a change after review: Job kinds (internal/job), the feed (internal/orchestrator, grid worker next), how a Job ends after a reviewer verdict, whether anything merges onto a branch. Name what is missing for landing.
2. What the scripts do that the plugin would need, read from land.sh, landbatch.sh, landrun.sh, collect.sh, eject2fix.sh, skip2merge.sh: list the rules (holds, stacks, M- merge lanes, ejects, flake reruns, cross-repo check, generated-file conflicts, attribution check, NOT DONE closing) and say for each whether it belongs in a land Job, in the project's workflow note (brain), or disappears with per-change landing.
3. Design of a land Job: posted by the orchestrator when a change's review passes; depends on the change's Job and on the land Jobs of changes it stacks on; runs as a runner (no agent needed for the green path): merge onto the current target in a worktree, run the project's check command (from the brain, e.g. make check), fast-forward or merge on green. Parallel: speculative stacking (test N on main+pending N-1, like a merge queue) or test each on main and rebase-check at merge; pick one with numbers from this box (12 cores, full check time from the land logs). A red check: rerun the failing tests alone; flake -> lands and posts a flake Job; real -> the land Job fails and posts a fix Job for the builder. Conflict -> a merge Job for a builder.
4. Capacity: how a land Job gets CPU before builders on one machine with the worker's existing launch path (priority between Job kinds, a slot for check Jobs), without adding a cap on agents.
5. Moving this build onto Grid: what else the scripts do (multi-account lanes, limit switching, planners, maps, end reviews) and which plugin features already cover it; an order of steps to move, smallest first, and what has to land before the cutover (23 Nov).
6. Lane plan: plugin lanes with repo, model/effort, files, acceptance rows and dependencies; then the app pieces (Feed shows land Jobs).

Write docs/research/land-job-type.md in the clone (committed on the lane), one section per question, ending with the lane plan; last line exactly DONE or NOT DONE. Copy it to <hand>/R-land-job-type.md.

Never: push, touch <gmf>, <root-home>, other users' homes, pkill -f; never edit the scripts in <hand> (read only). One command per Bash call. A refused Bash call is not the end: rerun it as one plain command and go on. No attribution lines in commits or files.
