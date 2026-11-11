# T-hook-worker-job-tooluse

Plugin repo, branch lane-T-hook-worker-job-tooluse. Test-only lane: one new file, internal/hook/worker_job_tooluse_test.go. No guard, hook or format code changed.

## Result in one line

With the app's Job env (GRID_ROLE=worker, GRID_TASK=the task.md id, GRID_MEMBER, GRID_WORKER_ID, GRID_JOB, GRID_ATTEMPT) and a cwd outside the Grid home, none of the 14 calls is denied, on either event. One finding: if GRID_TASK is the task's folder name and GRID_PROJECT is unset, main denies every Read, Grep and Glob in the Grid home (finding 1).

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1 Fixture sets the app's Job env, a task with task.md, plan.md, history.md, `feed/`, one project brain note, cwd a repo outside the Grid home | HOLDS | `jobSetup`, worker_job_tooluse_test.go:23-62 (built on `eventSetup`/`f.run`, events_test.go:26-64). Env keys set at :55-60 |
| 2 PreToolUse: Read task.md/plan.md/history.md, Read brain note, Grep and Glob with `path` the task folder; decision only asserted | HOLDS | `go test -count=1 -race -run TestJobAgent -v ./internal/hook/` exit 0; the `calls` table, :118-144 |
| 3 Same six calls as PermissionRequest; decision only | HOLDS | same command; the loop covers both events, :146-166 |
| 4 Bash: `grid job show`, `grid job result --body '{}'`, `grid brain search deploy`, `git status` | HOLDS | same command; table rows "Bash ..." |
| 5 Write and Edit in the cwd repo; Write of brain-proposals/x.md and plan.md | HOLDS | same command; table rows "Write/Edit ..." |
| 6 Report table of rows 2-5 with event, call, decision, reason, deciding guard; every deny a finding | HOLDS | tables below; finding 1 is the only deny, in an extra case (not one of the 14 calls) |
| 7 `go test -count=1 -race ./internal/hook/` 0 fail; `go vet ./internal/hook/` exit 0 | HOLDS | `ok  .../internal/hook 5.582s` exit 0; vet exit 0 |
| New tests fail on base, pass on the lane | HOLDS (trivially) | The file does not exist on main, so its tests do not exist there. The lane pins what main answers; no guard was changed to make a row pass. My first fixture used the folder name as GRID_TASK and every Read came back deny; that is finding 1, pinned by `TestJobAgentTaskNamedByFolderNeedsProject`. |
| Packages that import hook (`go list`): internal/cli, internal/packaging (and hook itself) | HOLDS | `go test -count=1 ./internal/cli/ ./internal/packaging/` ok, exit 0 |
| Lint | HOLDS | `make lint` (go vet ./... and gofmt -l .) exit 0 |
| Windows and macOS builds | HOLDS | `make cross` (vet and test compile for darwin/arm64 and windows/amd64) exit 0 |
| Only the allowed file touched | HOLDS | `git diff --stat main...lane-T-hook-worker-job-tooluse` lists the test file and this report |
| No guard, gate, cap or hold added | HOLDS | no non-test code changed |
| Commits: one sentence, no attribution lines | HOLDS | `git log main..HEAD` |
| No personal paths, hosts or emails | HOLDS | fixture uses member@example.com and temp dirs |

## What each call answers

Env and cwd as in row 1. "Guard" is read from the reason text. The deciding guard for each allow is the home guard's fallback: `EvaluateHome`, internal/guard/home.go:295 ("<Tool> stays inside the Grid home"). The host guard and the plan guard stayed silent on all 14 calls.

PermissionRequest answers carry no reason text on an allow: the grant has none. That is how main writes it, not an omission here.

| Event | Call | Decision | Reason text | Guard |
|---|---|---|---|---|
| PreToolUse | Read `<task>/task.md` | allow | Grid: Read stays inside the Grid home. | home (home.go:295) |
| PreToolUse | Read `<task>/plan.md` | allow | Grid: Read stays inside the Grid home. | home |
| PreToolUse | Read `<task>/history.md` | allow | Grid: Read stays inside the Grid home. | home |
| PreToolUse | Read `<project>/knowledge/ops/deploy.md` | allow | Grid: Read stays inside the Grid home. | home |
| PreToolUse | Grep, path = task folder | allow | Grid: Grep stays inside the Grid home. | home |
| PreToolUse | Glob, path = task folder | allow | Grid: Glob stays inside the Grid home. | home |
| PreToolUse | Bash `grid job show <task> J-1` | silence | | none |
| PreToolUse | Bash `grid job result --task <task> --job J-1 --body '{}'` | silence | | none |
| PreToolUse | Bash `grid brain search deploy` | silence | | none |
| PreToolUse | Bash `git status` | silence | | none |
| PreToolUse | Write `<repo>/main.go` | silence | | none |
| PreToolUse | Edit `<repo>/main.go` | silence | | none |
| PreToolUse | Write `<task>/brain-proposals/x.md` | allow | Grid: Write stays inside the Grid home. Also a systemMessage: "only-writer: Task folders and the brain are written only by grid. Use the corresponding grid command to make this change." | home allow; only-writer warns (guard/writer.go:36, level warn by default, so not a deny) |
| PreToolUse | Write `<task>/plan.md` | allow | same two texts | same |
| PermissionRequest | the six Read/Grep/Glob calls | allow | (none) | home |
| PermissionRequest | the four Bash calls | silence | | none |
| PermissionRequest | Write and Edit in repo | silence | | none |
| PermissionRequest | Write `<task>/brain-proposals/x.md` | allow | systemMessage: only-writer warning as above | home allow; only-writer warns |
| PermissionRequest | Write `<task>/plan.md` | allow | same | same |

Reading across the rows: no call is denied. A Job's agent writing into its task folder (plan.md, brain-proposals) gets an allow and a warning, not a deny, at the default only-writer level. Per verdicts-11nov v-4 the default is warn; at `block` the same two writes would be denied by guard/writer.go:36 (pinned by TestGuardDispatch, tooluse_test.go:20-24). I did not change that.

## Findings (denies)

1. **GRID_TASK given as the task folder name denies every home Read, Grep and Glob.** Extra case, `TestJobAgentTaskNamedByFolderNeedsProject`. With GRID_TASK=`T-01-cache-configuration`, GRID_PROJECT unset and cwd outside any project with a record, both events give deny: "Cannot resolve hook settings: task T-01-cache-configuration needs --project: this folder is in no project with a record". Source: the home guard's fallback allow becomes a deny in internal/guard/settings.go:32-41 ("An unreadable permission fallback must never grant access"), from the error at internal/where/where.go:269 via internal/settings/store.go:192. Bash, Write and Edit give no decision and log the same settings error to stderr (exit 0). A task id (the ULID in task.md) resolves with no project (where.go:247, `taskULID`), and that is what the 14-call test uses. Not fixed here. Check before the first live round: the app must set GRID_TASK to the task.md id (app internal/kernel/worker/job.go:316-337) or also set GRID_PROJECT. I could not read the app repo from this sandbox, so I did not check which one it sets.

No other denies.

## What changed

- internal/hook/worker_job_tooluse_test.go (new): `jobSetup`, `decide`, `TestJobAgentToolCallsThroughTheHook` (14 calls x 2 events = 28 subtests; each logs its table line with `TABLE |`), `TestJobAgentTaskNamedByFolderNeedsProject`.
- docs/lanes/T-hook-worker-job-tooluse.md (this report).

## NOT VERIFIED on a live screen

- What the app's Worker sets as GRID_TASK (id or folder name) and whether it sets GRID_PROJECT: read from the brief's env list only. The test covers both forms.
- Claude Code's own permission layer: the plugin hooks are only one half of a live Job's refusals. The orchestrator's live check covers the real `permission_denied` count.
- Hook behaviour with a real worktree path or a symlinked Grid home.

DONE
