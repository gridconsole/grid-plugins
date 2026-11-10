# PLUG-shell: where agentShell still changes behaviour

Base: main (9dc5d049). Lane head: see the report. Changed: one new test file, `internal/cli/role_policy_setup_test.go`. No production code changed.

The brief says agentShell is still passed to the trash, question, feed and task packages. That was true before S6b8 and is not true on main: S6b8 removed those parameters (docs/lanes/S6b.md:169-172) and S6tidy recorded that only task attach is left (docs/lanes/S6tidy.md:8). The grep below confirms it.

## Row 1: every use of agentShell on main

`grep -rn agentShell . --include=*.go` finds two lines; the consumer in the task package is added because that is where the behaviour is.

| file:line | what it is | changes behaviour? |
|---|---|---|
| internal/cli/task_write.go:202-204 | doc comment of the function | no |
| internal/cli/task_write.go:205-213 | `func agentShell(env Env) bool`: true when GRID_AGENT, GRID_WORKER_ID or GRID_WORKER is non-empty, or GRID_ROLE (normalised) is not empty, owner, member or cli | no, it only returns a bool |
| internal/cli/task.go:211 | `Agent: agentShell(env)` in the `task.AttachOptions` of `grid task attach`; the only caller | yes, through the next line |
| internal/task/attach.go:22,23,48-71 | `AttachOptions.Agent` and `BoundTask`: an agent must resolve GRID_TASK to a task folder and the target must be that same folder; otherwise `agent cannot attach <id> without its assigned task folder` or `... assigned to another task's folder` | yes, the one place |

No use in trash (`runTaskTrash`, `task/trash.go`), question (`internal/question`, `cli/question.go`) or feed (`cli/feed.go`, `mcp.go`, `orchestrator.go`).

Env checks that are not agentShell, listed because the brief names the first:
- internal/cli/feed.go:311-313 (the brief says about 328; it moved): refuses a GRID_WORKER that is not `<member>@<machine>`. Identity check. Stays.
- internal/hook/events.go:266: GRID_WORKER set marks a worker session for the Job-result Stop check. Refuses nothing.

## Row 2: task attach is not removed

I did not remove it. `docs/simplify.md` (read at /srv/gridwork/briefs/simplify.md; the clone has no docs/simplify.md or docs/needed.md) does not list it:
- row "The shell text guard ... Remove it" is the code that read shell command lines (S6a deleted it). agentShell reads environment variables, not shell text.
- row "Hard enforcement around those gates" covers approve and refusal decide (S6b removed their agent refusals). The attach rule is a task-scope rule, not a gate.
- docs/needed.md has no entry for it.

If the owner wants it gone, the work is: delete `agentShell` (task_write.go:202-213), the `Agent:` and `BoundTask:` arguments (task.go:211), `AttachOptions.Agent` and `BoundTask` and attach.go:48-71, and the tests `TestAttachAgentBoundary` (internal/task/attach_test.go:339) and `TestTaskAttachAgentCannotSwitchTasks` (internal/cli/task_test.go:463). That needs a line on simplify.md first.

## Row 3: parameters to remove

None. `grep -rnE "(agent|shell)[A-Za-z]* +bool|bool +(agent|shell)|AgentShell|PersonalAllowed" internal cmd --include=*.go` (non-test) finds only `where.Resolver.PersonalAllowed` (a replaceable resolver caller policy that S6b8 kept on purpose) and `DisableSkillShellExecution` (internal/job/render.go:28, an unrelated Job field, simplify.md row 7). `AttachOptions.Agent` still changes behaviour (row 2). Nothing else in a signature depends on agentShell.

## Row 4: GRID_WORKER identity check

Stays at feed.go:311-313. A test already pins it, so none was added:
- `TestCodexFeedCLI` (internal/cli/feed_test.go:42-67): `member@<machine>` posts; `another@<machine>` gives exit 1 and names GRID_WORKER.
- internal/cli/record_exit_test.go:65 row "writer assertion": exit 1 with the exact text `GRID_WORKER must match GRID_MEMBER and the local machine id`.
- internal/question/withdraw_cli_test.go:28 row "worker mismatch": same text, exit 1.

## Row 5: setup preview under a role list

New `TestSetupPreviewHonoursWorkflowRoleList` (internal/cli/role_policy_setup_test.go), 14 sub-tests: scope project (`workflows.md`, caller's folder is the project's registered folder) and scope task (`workflow.md`), each with `person_verbs: {setup: [owner]}` and GRID_ROLE member, empty (default member), lead, member with `--json`, then owner listed, member listed, and no policy. A refused row: exit 3, stderr `grid setup: ... (reason: role_not_allowed)`, no preview on stdout, `--json` gives `"error":"person_required"`, no settings file written. An allowed row: exit 0 and the preview (`+ Bash(grid *)`, `Nothing written.`).

New tests fail on base: this one pins behaviour that is already on main (S6b8 did not restore the old preview exemption), so it passes on base. To show it can fail, I put the old exemption back by hand, `if verb == "setup" { return 0 }` at the top of `workflowRoleRefusal`, and ran it (then `git checkout` of role_policy.go; `git status` showed only the new test file):

```
--- FAIL: TestSetupPreviewHonoursWorkflowRoleList (0.02s)
    --- FAIL: .../project/member_not_listed   role_policy_setup_test.go:58: setup exit 0, want 3: + Bash(grid *)
    --- FAIL: .../project/default_role_is_member   (same)
    --- FAIL: .../project/lead_not_listed   (same)
    --- FAIL: .../project/not_listed_json   setup exit 0, want 3: {"path":".../custom-claude/settings.json","added":[...],"written":false}
    --- FAIL: .../task/member_not_listed, default_role_is_member, lead_not_listed, not_listed_json   (same)
```
All 8 refusal rows failed; the 6 allowed rows passed. Without the change the test is `ok`.

## Row 6 and checks (lane head)

- `go test -count=1 ./...`: every package `ok`, no FAIL line (output went through `grep -v "no test files" | tail`, so the exit status printed was tail's, not go's). internal/cli ok in 22.6s.
- Importers of internal/cli (go list, imports plus test imports): cmd/grid, contract/formats, internal/golden, internal/mcp, internal/packaging, internal/question, internal/settings. All inside the run above. I changed no production package, so no other package imports a changed one.
- `go build ./...`: exit 0. `go vet ./...`: exit 0. `gofmt -l internal cmd`: no output.
- `GOOS=windows go build ./...`: exit 0. `GOOS=darwin go build ./...`: exit 0.
- `make lint`: not run (not allowed in this session).

## NOT VERIFIED

- UI: none changed.
- The exit status of the whole `go test ./...` run (see above); the orchestrator's landing run is the check.
- `make lint`.
- The lead/guest point below comes from reading the code; I did not run it.

## Open items for the orchestrator

1. Decide whether the task attach scope rule (row 2) stays. The brief's removal path needs a line on simplify.md.
2. `agentShell` returns true for GRID_ROLE `lead` and `guest` (they are not owner, member or cli), so by the code a lead who runs `grid task attach` from outside the task folder, with no GRID_TASK, is refused with `agent cannot attach ... without its assigned task folder`. `contract.TierFor` treats lead and guest as people. Not changed; say if it should be.
3. Generated files: none touched; no goldens or ops rows needed.
