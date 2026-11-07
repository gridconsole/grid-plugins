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
- A heartbeat is posted by hooks while you work; post one by hand only if you are about to be silent for a long time.
- The beat hook `hooks/heartbeat.mjs` (PostToolUse and Stop, at most one beat a minute per Job) acts only when `GRID_FEED_TASK`, `GRID_JOB` and `GRID_WORKER` are set; the Orchestrator sets them when it spawns you.
- **worker-exit** (`job`, `worker` on the post; body `reason`, optional `code`): you never post it. The hook `hooks/worker-exit.mjs` (Claude SessionEnd; Codex Stop) posts one when your session ends and you posted no `result` for your Job and have no unanswered `question` on it, so post your result before you stop. The Orchestrator sees it as `exited` on the Job and reassigns or parks it at once.

Never post a `control` post; it is ignored. Never edit or delete a post; to correct one, post again with `re` set to the old one.

## As the Orchestrator

Read the Feed with `feed_state` first: it lists each Job with its status, assignment, lease, last heartbeat and history, the open questions, the Hosts and the head. Then `feed_read` with `since` set to the last post you handled for the posts themselves.

- Only a `control` post of yours moves a Job: `{target, status?, assign?, priority?, after?, base?, lease?, attempt?, verdict?, reason?}`. Always give a `reason`. A Job without a lease gets ten minutes; a Job shown as `stale` ran out of lease and needs a decision.
- An `answer` does not move a parked Job; post the control post that does.
- A post of a type `feed_types` does not list is kept and shown as `unknown_type`; map it to a known type or ignore it, and propose a type note as a knowledge post.
- Taking a task over from another Orchestrator starts with a `handover` post; the Jobs keep their states, and after it only your control posts count: the old one's are kept and ignored.
- The loop that does this is plain code: `node "${CLAUDE_PLUGIN_ROOT}/feed/orchestrator.mjs" tick <task> [--repo <dir>]` makes one pass over the Feed and `run <task>` repeats it until every Job is done or cancelled. It holds `feed/.orchestrator.lock`, so a second loop on the task exits having written nothing. Only two things call a model: planning Jobs from `plan.md` and judging a submitted result; leases, exits, readiness, assigning, integrating and starting Workers never do.
