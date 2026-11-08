# Old plugin map (gridconsole-os/plugins/grid, read-only)

Paths are relative to the old plugin root. Not opened: bodies of feed/start, hold, kill, retry, wait, host, hosts, git, mcp, refs-ssh, sync, deploy, run-settings, host-grants, lock, worker-start; bodies of brain.mjs; skill script bodies. Nothing was executed, so the node invocations in section 7 are unverified.

## 1. Hooks (hooks/hooks.json lines 1-130)

Event / matcher -> script (hooks.json line)
- SessionStart: session-check (6), grid-env (7), agent-routing (8), usage-log (9), session-attach (10)
- UserPromptSubmit: session-attach (17), brain-recall (18)
- SubagentStart matcher `grid-planner|grid:grid-planner`: brain-recall (25)
- PreToolUse Write|Edit|MultiEdit|NotebookEdit|Bash: plan-guard (31), host-guard (35)
- PreToolUse Agent|Task: brief-lint (40), brain-recall (41)
- PreToolUse Bash: history-log (46)
- PreToolUse Skill|Agent|Task: session-attach (50)
- PreToolUse Read|Write|Edit|MultiEdit|Glob|Grep|NotebookEdit|Bash: grid-home-allow (54)
- PostToolUse Write|Edit|MultiEdit|NotebookEdit|Bash: history-log (60)
- PostToolUse Read: brain-read (64); Skill: brain-recall (68); Agent|Task: usage-log (72); no matcher: heartbeat (75)
- PostToolUseFailure Bash: history-log (81)
- SubagentStop: history-log (87), stop-check (88), usage-log (89), session-record (90)
- Stop: stop-check (97), session-record (98), heartbeat (102)
- SessionEnd: session-record (108), worker-exit (112)
- Notification: needs-you (117)
- PermissionRequest: permission-forward (122), grid-home-allow (126)

Traps:
- terminal-task.mjs is not in hooks.json; session-attach.mjs:3 and terminal-task.mjs:72 call it.
- hooks.json wires 19 hook scripts (plus lib.mjs), not the 21 plugin-design.md states.

Per script (stdin -> effect -> output):
- session-check.mjs:10-13: NEEDS_VERSION 2.1.78; if CLAUDE_PLUGIN_DATA is unset, prints one JSON warning; exit 0.
- grid-env.mjs:11-25: returns at once if GRID_HOME or GRID_PROJECTS is set (13). Else workspaceFor (15) reads the desktop app's config.json (lib.mjs:111-150). Appends `export GRID_HOME=` / `GRID_PROJECTS=` to CLAUDE_ENV_FILE (18-21). Prints additionalContext (25).
- agent-routing.mjs:12-50: reads the first description sentence of agents/*.md (21); prints additionalContext telling the main session to delegate to grid agents. Pure: stdout only.
- usage-log.mjs:17-28 record(), 59 response(): appends one JSON line to CLAUDE_PLUGIN_DATA/sessions/<session_id>.usage (lib.mjs:599-606 appendUsage); trims with trimUsage (612-620). Records transcript paths and Agent totals, never prompt text.
- session-attach.mjs:9-: attaches the session to a task (lib taskRun 513, touchSession 457). Writes the sessions/<id> pointer and the task's sessions/<id>.json. Never blocks.
- terminal-task.mjs:72-110: on UserPromptSubmit, a work prompt in a registered project creates the task folder. Writes intent.md (prompt text), a history.md line `- <min> · created · from a terminal session · by <owner>`, and task.md (fields in section 3). Staged in tasks/.hook-<pid>-<id>, then renamed. Skips when GRID_TASK is set, GRID_BRIDGE=off, or unattended (79, header 45-46).
- brain-recall.mjs:29-106: taskNamed 29, taskOf 42, firstTime 65 (sessions/<id>.recalled, keeps 100 lines, line 72). Recalls brain hits via brain.mjs recallFor (92), prints additionalContext (106), appends `brain recall · <n> hits` to history.md. Once per task, agent and plan revision.
- plan-guard.mjs:72-212: GRID_PLAN_GUARD=off returns at 74. Deny JSON at 212. Rules in header 1-49: a held task (no `plan accepted` line) blocks repo writes; agents write only in task folders; brain writes are limited to the curator; approval lines are never written by agents; a recall line is required before `plan ready`.
- host-guard.mjs:25-44: denies file tools and Bash that touch the `.host` folder beside the data folders. Runs even with GRID_PLAN_GUARD=off (header 10).
- brief-lint.mjs:15-30, lint 34: denies an Agent call whose brief lacks BRIEF_KEYS or BRIEF_SECTIONS (lib.mjs:34-35).
- history-log.mjs:22-85: fileLine 42, bashBefore 56, bashAfter 67, subagentStop 85. Appends history lines via lib appendHistory (387).
- grid-home-allow.mjs:34-166: judge 136. Allows Grid's own scripts and paths. Emits a PreToolUse allow (166) or a PermissionRequest decision (163). Fallback only.
- brain-read.mjs:15-43: appends `brain read · <note> · by <agent>` to history.md; records the seen key (43).
- heartbeat.mjs:96 beat, 149 workerExit, 172-207: acts only if GRID_FEED_TASK, GRID_JOB and GRID_WORKER are set; at most one beat a minute per Job, under a lock (83, stale after 10 s); posts a feed heartbeat; always exits 0.
- stop-check.mjs:39-127: exits 2 once with the reason (header 1-3); skips when stop_hook_active (44); stopProblems 56, subagentProblems 84; markBrief 108 writes brief status done or blocked (120) via a tmp file with flag wx (127).
- session-record.mjs:6-: touchSession with end on SessionEnd/Stop; writes sessions/<id>.json.
- worker-exit.mjs:7-30: reads stdin, posts a worker-exit feed record, exits 0.
- needs-you.mjs:5-: POST to $GRID_URL/hooks/notification or the bridge; silent when absent.
- permission-forward.mjs:9-24: POST /hooks/permission, waits GRID_PERMISSION_TIMEOUT_MS (default 85 s, line 13). Prints a decision only on allow or deny.
- lib.mjs: readInput 38 (3 s cap), main 50 (writes out.stdout/stderr, exits out.code), real 62 and realNative 78 (symlink-safe paths), appConfigFile 111, openWorkspace 125, workspaceFor 153, projectFor 160, projectsRoot 179, gridFolders 187, taskAt 198, frontmatter 208 and parseFrontmatter 222, historyLine 379, appendHistory 387, pointerFile 409, rememberTask 433, touchSession 457, findTask 492, taskRun 513, appendUsage 599, trimUsage 612, briefInTranscript 647, briefInPrompt 737, saveSnapshot 822, unreviewedApproval 1008, feedLoopLive 1062, readBridge 1164, bridgeTarget 1186, post 1198.

## 2. bin, feed, remote, brain scripts

- bin/grid-feed (1-55): `post --task --type --body <json|-> [--re] [--job]`, `read [--since] [--type] [--job]`, `state --task`, `types [--task]`, `sync --task --host [--remote-dir]` (rsync of feed/*.jsonl). Calls feed/lib.mjs callTool (51-52) with feed_post, feed_read, feed_state, feed_types; sync calls feed/sync.mjs syncTool (45). JSON on stdout, errors on stderr with exit 1, usage exit 2. Gap: feed_handover is in callTool (lib.mjs:743) but not in the CLI.
- bin/grid-brain (1-24): `concept --task [--sha ...]`, `knowledge --task`, `decisions --task`, `recall --task [--query]`. Calls brain/brain.mjs conceptFor, knowledgeFor, decisionsFor, recallFor (14-21). Writes notes. Header 4-5: needs GRID_ROLE=brain.
- feed/lib.mjs: ulid 53, readFeed 186, post 298 (append), fold 485, MCP tool defs 714-763 (feed_post, feed_read, feed_handover, feed_state, feed_types), callTool 772, LEASE_MS 23 (10 min), LIVE 29.
- feed/orchestrator.mjs: header 1-9 (loop of one task; the model is used only for planning and judging results); tick 880, run 894, main 943; lock at feed/.orchestrator.lock.
- feed/spawned.mjs: per-task spawn registry (header 1-8), kept in worktrees, never in feed/.
- feed/worker-start.mjs:21 startWorker: detached process, log file.
- Other feed/*.mjs (start, hold, kill, retry, wait, host, hosts, git, mcp, lock, run-settings, sync, deploy, refs-ssh, host-grants): not opened.
- brain/brain.mjs: writeBrainNote 44; conceptNote 132 (conceptFor 159); knowledgeNotes 186 writes knowledge/research/<slug>.md linked [[task-<id>]] (184-185; knowledgeFor 218); specDecisions 224 reads the `## Decisions` section of spec.md; decisionNotes 249 (decision-<slug>-<task>; decisionsFor 275); recall 285, recallFor 304.
- remote/sync.json: per-runtime launch config (claude, codex): command, newSessionFlag `--session-id`, firstTurn `/grid:resume {task}`, plugin grid@grid, trust trust-claude.mjs (lines 43-77).
- remote/trust-claude.mjs (header 1-8): marks one checkout trusted in <home>/.claude.json (hasTrustDialogAccepted), under a lock. SSH-driven; the design deletes SSH.
- Skill scripts: deliver/scripts (land-note, land-plan, project-md, proof-add, proof-index); execute/scripts (item-record: a history line per plan item testing|deployed|failed, header 2-9; ready-steps: `needs:` lines, header 2-9); plan/scripts (items-sync, plan-hash: revision line, --print writes nothing); signals/scripts/add-signal.mjs.

## 3. On-disk formats

- Task folder: <projects>/<project>/tasks/T-<n>-<slug>/ (lib.mjs:4, 196).
- task.md frontmatter (terminal-task.mjs:103-105): `id, title, project, owner, source: idea, size: normal, workflow: [{step: build, kind: run}, {step: review, kind: approve}], step, status: running, updated, summary, next`. Example: `id: T-12`, `status: running`.
- history.md line (lib.mjs:379): `- <time> · <event> · <detail> · by <who>[ · via <via>]`. Gate lines: `· plan accepted ·` (278), `· plan ready ·` (293).
- intent.md: the raw prompt (terminal-task.mjs:101).
- feed/<member>@<host>[~<worker>].jsonl, one post per line (feed/lib.mjs:2-3, 107). The post id is a ULID; fields type, body, re, job, epoch (lib.mjs:325-335). Job fold fields (lib.mjs:477-650): id, status (assigned, running; stale is derived at 648), assign {worker, host}, priority, after, attempt, lease_expires, last_beat, history, results, open_questions, exited.
- feed/types/*.md: 13 type specs (answer, claim, control, deploy-failed, handover, head, host, knowledge, post, question, result, transfer, worker-exit).
- project.md (<grid home>/projects/<name>/project.md, lib.mjs:159-168): frontmatter `id`, `folder` (else `repo`).
- sessions/<session_id>: pointer list, the latest 20 with held tasks kept (lib.mjs:409-421).
- sessions/<session_id>.json (lib.mjs:450-484, in the task folder): `session_id, where, cwd, transcript, started, last, ended?`.
- sessions/<session_id>.usage: JSON lines (usage-log.mjs:2; lib.mjs:599).
- sessions/<session_id>.recalled: recall marks (brain-recall.mjs:72).
- .tasks and .tasks-<agent_id> lists (lib.mjs:428-445).
- bridge.json (lib.mjs:1150-1186): `url, token, pid`, mode 0600, loopback only.
- Desktop app config.json (lib.mjs:111-150): `current`, `workspaces[{name, folder, data}]`; legacy `workspace`, `projectsFolder`.
- Brief files (subagents/NN-slug.md): keys id, role, status, verify, follow_up (lib.mjs:34); sections Intent, Job, Context, Out of scope, Open, Acceptance, Outcome, Verdict, Follow-up (35).
- Brain notes: knowledge/research/<slug>.md, decision-<slug>-<task>.md (brain.mjs 184-249).

## 4. Agents (11) and skills (18)

Agents: grid-builder (executes a brief; old name grid-worker, lib.mjs:400), grid-brief-writer, grid-checker (read-only browser check), grid-curator (curates brain proposals, asks no one), grid-designer (spec.md), grid-operator (state-changing browser work), grid-planner, grid-researcher (research.md), grid-reviewer (writes the Verdict), grid-runner (one command or check), grid-scout (read-only map).

Skills: brief, deliver, execute, feed, intake, memory, plan, plan-page, preview, questions, research, resume, routing, setup, signals, task, views, visual-design-review. The design says 12 skills; there are 18 here.

## 5. Env vars and paths

- GRID_HOME, GRID_PROJECTS (lib.mjs:187-193; when set, `set: true`; when unset, a cwd fallback to the desktop workspace).
- GRID_URL (lib.mjs:1187), GRID_BRIDGE=off (lib.mjs:1188; terminal-task.mjs:79).
- GRID_TASK (terminal-task.mjs:79), GRID_ROLE (feed/lib.mjs:319), GRID_WORKER, GRID_JOB, GRID_FEED_TASK (heartbeat.mjs:2).
- GRID_PLAN_GUARD=off (plan-guard.mjs:74), GRID_PERMISSION_TIMEOUT_MS (permission-forward.mjs:13).
- CLAUDE_PLUGIN_DATA (lib.mjs:409, 599, 814), CLAUDE_ENV_FILE (grid-env.mjs:18), CLAUDE_CONFIG_DIR (lib.mjs:1154), CLAUDE_CODE_ENTRYPOINT and CLAUDE_CODE_SESSION_ATTENDED (terminal-task.mjs:45-46).
- HOME, USER, APPDATA, XDG_CONFIG_HOME (lib.mjs:111-115, 143).

Resolution from cwd: gridFolders (lib.mjs:187) -> projectFor (159-173, the longest project.md folder that holds cwd) -> taskAt (198). terminal-task.mjs:81-84 does the same. plugin-design.md (around lines 125-137) defines the new order: explicit, task folder, walk up, multi-repo, git root, folder.

## 6. Mapping to plugin-design.md section 2 (command groups, lines 109-119)

- terminal-task -> task (new, attach). session-attach -> task attach and session records.
- plan-hash, items-sync, plan-page and views skills -> plan, views.
- stop-check, ready-steps, brief-lint, feed/lib.mjs, orchestrator.mjs, heartbeat, worker-exit -> job, lease, feed, mcp, hook.
- feed/* lifecycle (start, hold, kill, retry, wait, host, hosts, git, lock, spawned, run-settings, host-grants, refs-ssh, sync) -> job, lease, feed. Partly covered; not opened.
- permission-forward, needs-you, grid-home-allow -> hook, approve (host path), signal. Flag: no explicit home for the permission bridge.
- plan-guard, host-guard -> hook enforce (the design names plan-guard, host-guard, Job lint).
- question and approve equivalents -> ask, answer, approve.
- deliver scripts, land-*, proof-* -> deliver, proof, land.
- brain.mjs, brain-read, brain-recall, grid-brain -> brain (index, recall, propose, accept). Flag: propose, merge and decline have no old script.
- usage-log, session-record -> session, usage.
- grid-env, session-check, agent-routing -> setup, hook sessionstart, version.
- trust-claude.mjs, sync.json, feed/sync.mjs, refs-ssh.mjs -> machine topics, grid-worker. Flag: SSH is removed by design; no direct command.
- add-signal.mjs, signals skill -> signal.
- execute item-record -> plan items / job result.

New commands with no old counterpart: `grid lease take merge`, `grid task move` (intent only), `grid task cleanup`, `grid doctor --bundle`, `grid where`, `grid migrate`, `grid eval`, `grid topic`, `grid config`, `grid project merge`, `grid brain dedupe`, `grid brain revert`, `grid job blocked`, `grid feed tree`.

Old behaviour with no new home: the feed_handover CLI verb (the design lists handover only in MCP); the grid-home-allow fallback counting; the brain-recall once-per-plan-revision markers; remote/trust-claude.mjs.

## 7. Golden-test candidates

Pure: stdin JSON in, stdout or exit code out. Shape: `printf '%s' '<json>' | GRID_PROJECTS=<scratch>/projects CLAUDE_PLUGIN_ROOT=<plugin> node <plugin>/hooks/<script>.mjs`.
- agent-routing.mjs: no stdin needed; `node hooks/agent-routing.mjs </dev/null`; stdout JSON.
- brief-lint.mjs: stdin Agent input; stdout deny JSON or nothing.
- session-check.mjs: stdout only; `env -u CLAUDE_PLUGIN_DATA node hooks/session-check.mjs </dev/null`.
- grid-home-allow.mjs (judge, 136): stdin Bash/Read input; allow or PermissionRequest JSON.
- stop-check.mjs: exit 0 or 2 with stderr; needs a project tree in GRID_PROJECTS.
- Pure lib functions: parseFrontmatter (222), historyLine (379), agentName (404), gateLike (284), workflowSteps (301), planStage (320), real (62). Run with `node --input-type=module -e "import('<plugin>/hooks/lib.mjs').then(m=>...)"`.
- bin/grid-brain recall and bin/grid-feed read/state: files in, JSON out (need a fixture task folder).

## Findings

1. terminal-task.mjs is not wired in hooks.json; it runs only through session-attach.
2. 19 hook scripts, not the 21 in plugin-design.md; 18 skills, not 12.
3. feed_handover is an MCP tool with no CLI verb in bin/grid-feed.
4. The heartbeat beats at most once a minute and LEASE_MS is 10 minutes (feed/lib.mjs:23), not the 60-second .live file in the design.
