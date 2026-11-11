# Agents: which scheme a session uses

Two folders hold agents. They are not the same thing.

- `agent-templates/` (12 files, `grid-*` and `Explore`). SessionStart copies them to `~/.claude/agents/` with the person's model and effort (`internal/hook/hook.go`, `docs/lanes/D-agents-sync.md`). The session's context names no agent (`TestSessionStartNamesNoAgents` in `internal/hook/agentsync_test.go` pins this). `assembleContext` in `internal/hook/hook.go` builds it from the session lines (`sessionContext`), the Job lines (`jobContext`, only for a worker-started Job) and the brain index.
- `agents/` (`builder`, `orchestrator`, `reviewer`). Claude loads these from the plugin folder as plugin agents. They carry the Job skills `run-job`, `plan-jobs` and `review-job`.

`builder` and `reviewer` do the same work as `grid-builder` and `grid-reviewer`, and `orchestrator` the same as `grid-planner`'s Job posting. Nothing in `internal/hook` names the `agents/` files, so the session context never points at them. Which of the two sets goes is the owner's decision; this lane removed neither, and did not check what else reads the `agents/` files.

Rules every agent here follows (from the owner, 10 Nov): no Grid-made guards, caps or limits; approvals and gates come from the workflow in the brain, with a default; everything is a Job and the Job is the brief; tasks carry on by themselves until a person point; no plan mode; the brain needs no attention from the person; every link carries a why. The four templates in this lane (`grid-planner`, `grid-scout`, `grid-researcher`, `grid-runner`) carry no `maxTurns` and no reply-length cap for that reason.
