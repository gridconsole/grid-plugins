---
name: feed
description: Post to and read a task's Feed, the append-only board where the Orchestrator posts Jobs and Workers post results, questions and knowledge. Use when you run a Job and must report back, when you are the Orchestrator reading the Feed or moving Jobs, or when the person asks what is on a task's Feed.
---

# Feed

Every task has a Feed: posts in `<projects folder>/<project>/tasks/<task>/feed/`, one file per writer, never in a repo. Posts are only ever added, never edited. The same tools work in a Claude session (MCP tools) and in a shell, a hook or Codex (`grid-feed`).

| Need | MCP tool | Shell |
|---|---|---|
| post | `feed_post {task, type, body, re?, job?}` | `grid-feed post --task T-12 --type result --body '{...}'` |
| read | `feed_read {task, since?, type?, job?}` | `grid-feed read --task T-12 --since <post id>` |
| state of the Jobs | `feed_state {task}` | `grid-feed state --task T-12` |
| the types and their fields | `feed_types {task?}` | `grid-feed types` |
| sync with another of your Hosts | none | `grid-feed sync --task T-12 --host <ssh host> [--remote-dir <task folder there>]` (rsync over ssh, both ways, files only grow) |
| sync a task's Job refs with another of your Hosts | none | `node "${CLAUDE_PLUGIN_ROOT}/feed/refs-ssh.mjs" sync T-12 --repo <clone> --host <ssh host> [--remote-repo <clone there>]` (git over ssh, never forced; the Host's refs arrive under `refs/grid-remotes/host-<name>/`) |
| sweep the refs of closed tasks on your Hosts | none | `node "${CLAUDE_PLUGIN_ROOT}/feed/refs-ssh.mjs" sweep --repo <clone> --tasks <tasks folder> --host <ssh host> [--host ...] [--task T-12]` (an unreachable Host is skipped with a line and found again by the next sweep) |

`task` is a task id such as `T-12` or the path of the task folder. Your name, machine and role come from `GRID_MEMBER`, `GRID_HOST` and `GRID_ROLE` (default `worker`), your worker id from `GRID_WORKER`; do not set them yourself. An error comes back as a result, never as a crash: read it and post again.

## As a Worker

You run one Job, then close. Post what the Job needs and nothing else; always set `job` to your Job's id.

- **result**: when the Job is done, once: `outcome`, what changed, the evidence (the output that proves it), what you think comes next, and any knowledge. Then stop.
- **question**: when you cannot go on without a decision and the brain does not answer it. Set `blocking: true` if you must wait. A question names its Job; the answer arrives as an `answer` post with `re` set to your question.
- **knowledge**: something the brain should keep (a fact, a decision, a procedure). You never write the brain; the Orchestrator hands the post to the Brain agent.
- **submit**: when the Job made a commit: the sha and the ref you pushed. Post it with the result.
- A Host announces itself with a `host` post (online, grant for the task, repos with head shas, slots total/free, usage_left as a fraction), refreshed within 3 minutes or it counts offline; the Orchestrator picks Hosts from these.
- **claim** (optional; body `session`, `model`): says you took the Job. The beat hook already puts `session`, `model` and a `tail` (your last 8 session lines) on each beat.
- A teammate's Host runs Workers only under a grant file, `<host-home>/host/grants/<member>.json` (host-home is `--home` or `GRID_HOME`; one file per member). Keys: `version` (1), `id`, `grantee`, `scope` (`"all"` or task ids), `repos`, `profile` (`read`, `edit` or `run`), `slots`, `cap` (`{"hours": n}`, a whole number of hours from `granted_at`; a `tokens` cap is refused), `expires` (ISO-8601 or null), `revoked`, `granted_at`. Settings makes and revokes it; nobody edits it by hand and the daemon (`feed/host.mjs`) never writes it. The daemon kills a Worker within one tick (10 s) of expiry, `revoked: true`, a deleted or unreadable file, or a scope that no longer covers it, then posts `worker-exit`. Its `host` post says `grant` true only for tasks an active grant covers. Once the host has seen a grant file for a member it keeps `<host-home>/host/seen/<member>.json`; while that exists a member with no valid grant file is refused (`no_grant`), never started as an ungranted one, and only the host owner removes it. A granted member's Orchestrator Workers run only in their own worktree (`<host-home>/worktrees/<project>/<task>/<worker>`) or their task folder, and a Job with no repo only in its task folder.
- A heartbeat is posted by hooks while you work; post one by hand only if you are about to be silent for a long time.
- The beat hook `hooks/heartbeat.mjs` (PostToolUse and Stop, at most one beat a minute per Job) acts only when `GRID_FEED_TASK`, `GRID_JOB` and `GRID_WORKER` are set; the Orchestrator sets them when it spawns you.
- **worker-exit** (`job`, `worker` on the post; body `reason`, optional `code`): you never post it. The hook `hooks/worker-exit.mjs` (Claude SessionEnd; Codex Stop) posts one when your session ends and you posted no `result` for your Job and have no unanswered `question` on it, so post your result before you stop. The Orchestrator sees it as `exited` on the Job and the loop marks it stale at once.

Never post a `control` post or a `post` (the task page's thread, for people and the Orchestrator); both are refused or ignored. Never edit or delete a post; to correct one, post again with `re` set to the old one.

## As the Orchestrator

Read the Feed with `feed_state` first: it lists each Job with its status, assignment, lease, last heartbeat and history, the open questions, the Hosts and the head. Then `feed_read` with `since` set to the last post you handled for the posts themselves.

- Only a `control` post of yours moves a Job: `{target, status?, assign?, priority?, after?, base?, lease?, attempt?, verdict?, reason?}`. Always give a `reason`. `priority` is `high`, `normal` or `low`. A `control` post with no status and a `reason` of dependency, worker_cap, needs_person, question, host_offline, conflict_hold, starting or orchestrator_stopped says why an unclaimed Job waits; post it only when it changed. A `job` or `handover` counts only from the Orchestrator (role `orchestrator`, and a Worker id that is missing or is not a Job attempt's Worker id); from anyone else it is kept and ignored. A `job` may carry `item`, the plan item it serves. A Job without a lease gets ten minutes; a Job shown as `stale` ran out of lease and needs a decision.
- Stale and retry: a Job whose Worker posts no beat before its lease ends, whose Worker exits with no result, or whose result has no submit is marked stale once. The loop then retries it once as attempt 2 with a new Worker (the old attempt's posts do not count). If attempt 2 also goes stale, the loop posts one blocking question to the Owner (who: owner, the job id, why) and parks the Job.
- An `answer` to the loop's own question to the Owner, from a person with the owner or member role, moves a parked Job: "Retry it" makes it ready again with a fresh attempt count, "Drop it" cancels it, "Change the brief" leaves it parked (no status of its own; post the control post that does). Any other answer does not move a parked Job. An answer is used once: a later park (a failed judge, say) stays parked until a new answer. A Worker that stopped with a blocking question still open is not retried at once: the Job parks, and when that question is answered the loop makes it ready again as a counted attempt.
- **post** (the task page's thread; body `ref` task, `item:i-<n>` or `question:Q-<n>`, `kind` message, pickup, ask-pickup, ship, close or reopen, `text` up to 4000 characters, `attachments` under `attachments/posts/<post id>/`): written by people and the Orchestrator, not a Worker: the role comes from the session, never the body, and is owner, member, guest, planner or agent, so a Worker's role (builder, scout, ...) is refused. A guest may not post pickup or ship.
- Only an Orchestrator's `job`, `handover` and `control` posts count: role `orchestrator`, and a Worker id that is missing, or is not a Job attempt's Worker id (your session id, say). One from another role, or with a Job's Worker id, is kept, shown in `ignored` with the reason and does nothing.
- A post of a type `feed_types` does not list is kept and shown as `unknown_type`; map it to a known type or ignore it, and propose a type note as a knowledge post.
- Taking a task over from another Orchestrator starts with a `handover` post; the Jobs keep their states, and after it only your control posts count: the old one's are kept and ignored.
- The loop that does this is plain code: `node "${CLAUDE_PLUGIN_ROOT}/feed/orchestrator.mjs" tick <task> [--repo <dir>]` makes one pass over the Feed and `run <task>` repeats it until every Job is done or cancelled. It holds `feed/.orchestrator.lock`, so a second loop on the task exits having written nothing. Only two things call a model: planning Jobs from `plan.md` and judging a submitted result; leases, exits, readiness, assigning, integrating and starting Workers never do.
- **Wait Jobs**: a Job post with `kind: wait` and a `check` (`{command: [argv...]}` or `{url}`, `interval_s`, `timeout_s`) is run by the loop itself through `feed/wait.mjs`, not by a Worker and not by a model. The command is spawned from its argv array with no shell. Exit 0 (or a URL that answers 2xx or a passed status) is green: a result is posted and the Job goes done. Exit 1 (or a failed status) is red: a result with the failure and the Job goes rejected with the reason. Exit 3 (or a waiting-for-approval status) posts one question to the Owner (`who: owner`) and the Job keeps watching; the loop never approves. At `timeout_s` the lease runs out and the Job is stale through the normal path, so the retry hook decides.
