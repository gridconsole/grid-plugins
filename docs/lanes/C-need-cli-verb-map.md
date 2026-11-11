# C-need-cli-verb-map: who calls the 44 grid verbs

Map only. No code changed. Plugin main 78b08bc9, app main 4df0905b (read with `git -C /srv/gridwork/ref/app.git grep ... main`; the path was readable).

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Section A, one row per verb (44) and per subcommand of session, job, feed, task, brain, agent, account, usage, intent, with the grep | HOLDS | Section A below; the commands are in its header. Gaps are listed in "NOT VERIFIED" (app tests, `version`). |
| 2. Section B, loop verbs with call sites | HOLDS | Section B. |
| 3. Section C, verbs kept by decision, line quoted | HOLDS for needed.md:41 (quoted in the brief); the text of needed.md:43 and simplify.md:65, 66 is not in my sandbox (the lists are in neither repo), so I cite the line and what the brief says it covers | Section C. |
| 4. Section E, candidates grouped by at most three verbs, "list line" column, "covered by", "none: ask", docs-only under ask | HOLDS | Section E. |
| 5a. File under 260 lines | HOLDS | `wc -l docs/lanes/C-need-cli-verb-map.md` |
| 5b. `make lint` exit 0 | HOLDS | pasted in the report |
| 5c. `git diff --stat main` shows only this file | HOLDS | pasted in the report |
| Rule: no code change, no guard, gate, cap or hold on agents | HOLDS | only this file is new |
| Rule: no attribution lines, no personal paths, hosts or emails | HOLDS | `git log -1 --format=%B` |
| Rule: new tests fail on base | n/a: no test added (map only) | |
| Rule: package tests, go list importers, Windows and macOS builds | n/a: no package changed | `git diff --stat main` |

## Section A: callers

Columns: **app** = `plugincli.Request`/`Run(ctx, "<verb>"...)` call sites in non-test Go of the app, plus the `Plugin:` column of `internal/kernel/ops/table_gen.go` ("table" = only declared there; `notBuilt` = the table itself marks the plugin side `pluginNotBuilt`). **skills** = agents/, skills/, agent-templates/, topics/, hooks/hooks.json, views/, evals/, corpus/, `.mcp.json`. **Go** = plugin Go that execs `grid`. **docs**: R:n = README.md line, W:n = app docs/website/cli.md line. R:30-41 and W:11-65 are reference tables that list every verb; R:43-128 are examples a person types (marked "typed"). Paths without a prefix are under the app's `internal/slices/` or `internal/kernel/`.

Commands (every row comes from these; rerun to reproduce):

```
A=/srv/gridwork/ref/app.git
git -C $A grep -nE "plugincli.Request|Run(Text|Request)?\(ctx, \"[a-z-]+\"|\[\]string\{\"(<verbs>)\"" main -- '*.go' ':!*_test.go' ':!internal/kernel/ops'
git -C $A show main:internal/kernel/ops/table_gen.go | grep -no 'Name: "[^"]*"\|Plugin: "grid[^"]*",\( PluginBuilt: &[a-zA-Z]*\)\?'
git grep -noE "\bgrid (<verbs>)\b( [a-z-]+)?" -- agents skills agent-templates topics hooks views evals corpus README.md .mcp.json
git grep -nE "exec\.Command(Context)?\(|\"grid\"|GridBin" -- 'internal/*.go' 'cmd/*.go' ':!*_test.go'
git -C $A grep -nE "grid (<verbs>)\b" main -- docs/website/cli.md
git grep -nE "^\s+case \"[a-z-]+\"" -- internal/cli/<verb>.go    # subcommand lists; contract/commands.json for the rest
```

Go column: no plugin Go execs `grid <verb>` except `internal/routine/execute.go:83,151,165` (a routine's command that starts with `grid` runs through `GridBinary`; any verb can be named there; `internal/cli/routine.go:145`). Other `exec` hits run git, sh, claude or a deploy/eval command, not `grid`. `internal/hook/brain.go:175` only reads a Bash command that starts with `grid`. So Go is "none" in every row below unless it says otherwise.

### A1. The 44 verbs (cli.go:103-146)

| verb | app | skills | docs |
|---|---|---|---|
| version | none | none | R:30 W:11 |
| where | worker/agentbuild.go:79 | none | R:30 W:13 |
| open | none | none | R:30 W:12 |
| doctor | none | none | R:30,54,56 typed; W:14 |
| hook | none | hooks/hooks.json:8,19,30,41,52,63,75,86,97,108,119 | R:118 W:69 |
| task | see A2 | skills/task/SKILL.md:21,29,30 | R:43,45,46,128 typed; W:23 |
| plan | gates/args.go:15 (hash) | agents/orchestrator.md:13,20; agent-templates/grid-planner.md:26 | R:32 W:24 |
| views | none (stub, cli.go:110) | views/README.md:1 (title) | R:41 |
| job | see A2 | agents/*.md, agent-templates/*.md, skills/*-job/SKILL.md | R:32 W:33 |
| lease | worker/epoch.go:61, worker/judge.go:410 (`lease status task`) | none | R:32 W:69 |
| feed | see A2 | agents/orchestrator.md:23 (says "you do not call it") | R:33,50 W:34 |
| mcp | none | .mcp.json:5 | R:33,50 typed; W:69 |
| question | gates/withdraw.go:115 (withdraw) | agent-templates/grid-planner.md:27,53,59 | R:34 W:35 |
| ask | none (stub, cli.go:116) | none | R:41 W:35 |
| answer | gates/args.go:17 | none | R:34 W:35 |
| approve | gates/args.go:16; plugincli/taskfolders.go:20 | none | R:34 W:36 |
| deliver | none | topics/delivering/TOPIC.md:5,11,12 | R:34 W:37 |
| proof | none | agent-templates/grid-builder.md:26; skills/browser/SKILL.md:19,22 | R:34 W:38 |
| history | none | agent-templates/grid-planner.md:28; grid-curator.md:14 | R:34 W:25 |
| land | none | none | R:34 W:39 |
| brain | see A2 | agents/curator.md; skills/brain-keeper/SKILL.md | R:35 W:27 |
| machine | none (stub) | none | R:41 |
| env | none | skills/browser/SKILL.md:11 | R:36 W:62 |
| ports | none | none | R:36 W:63 |
| deploy | none (stub) | none | R:41 |
| person | plugincli/person_check.go:70 (check) | agent-templates/grid-planner.md:34 (approvers) | R:37 W:40 |
| routine | routines/http.go:140,226 | none | R:37,60,78,88,98,106 typed; W:55; Go: execute.go:151 |
| worker | worker/job.go:288 (`worker next`, run.go:154) | none | R:37 W:53 |
| agent | agentreads/http.go:75 (budget show); table 22-27,29,154,163 (notBuilt) | skills/brain-keeper/SKILL.md:66 | R:38 W:47 |
| claude | table_gen.go:54,55 (notBuilt) | none | R:38 W:49 |
| workflow | taskview/view_steps.go:35; workflows/args.go:20,24; cmd/grid-server/write_policy.go:139 | agent-templates/grid-planner.md:14; skills/plan-jobs/SKILL.md:11 | R:38 W:51 |
| skill | table_gen.go:61,167,191,192 (notBuilt) | none | R:38 W:50 |
| session | see A2 | skills/setup/SKILL.md:28 (`list`) | R:37 W:52 |
| intent | worker/intents.go:247-293,351 | none | R:37 W:69 |
| usage | taskview/usage.go:40 (show) | none | R:37 W:54 |
| account | agentaccounts/http.go:151,200,253,323,339 | none | R:37 W:48 |
| project | setup/http.go:148,262 | skills/setup/SKILL.md:19; skills/signals/SKILL.md:22 | R:39 W:20 |
| project-md | table_gen.go:128,129,139,141 (notBuilt) | none | R:39 W:21 |
| setup | table_gen.go:166,180 (notBuilt) | skills/setup/SKILL.md:3,35,40,48; evals/tasks/setup-skill/task.json:1 | R:56 typed; W:22 |
| signal | inbox/act.go:131, inbox/move.go:85; table 76,77,79 | skills/signals/SKILL.md:18,22,28-30; skills/task/SKILL.md:24 | R:39 W:41 |
| artifact | table_gen.go:36,37,38 (built; no Go call found) | none | R:39 W:26 |
| eval | none | evals/README.md:6-10,31,32,67 | R:39 W:56 |
| migrate | none | none | R:39 W:65 |
| sync | worker/run.go:167 (`sync --watch`); a stub in cli.go:146 | none | R:41 W:64 |

### A2. Subcommands (names from `case` lines in internal/cli and contract/commands.json)

| verb sub | app | skills | docs (named in the verb's line) |
|---|---|---|---|
| session list | sessions/http.go:97; usage/report.go:370; taskview/view_sessions.go:38 | skills/setup/SKILL.md:28 | W:52 |
| session show | table_gen.go:169,170 | none | W:52 |
| session start, stop, send, pause, resume, handback, switch (one row each, same callers) | tasks/intent.go:35 (`session <action>`); table 70,171-177,213 | none | W:52 |
| job show | worker/agentbuild.go:195; taskcode/build.go:92 | most agents, skills/run-job:7, review-job:7 | W:33 |
| job result | none | agent-templates/grid-builder.md:38 and 9 more; agents/builder.md:16; skills/run-job:18 | W:33 |
| job post | worker/judge.go:425; worker/supervisor.go:74 | agents/orchestrator.md:21; skills/plan-jobs:11,31 | W:33 |
| job lint | none | agents/orchestrator.md:20; grid-planner.md:44 | W:33 |
| job tree | none | agents/orchestrator.md:21; skills/plan-jobs:33 | W:33 |
| job cycles | none | agents/orchestrator.md:21; skills/plan-jobs:29 | W:33 |
| job floor | none | skills/plan-jobs/SKILL.md:29 | W:33 |
| job verify | none | skills/run-job/SKILL.md:17 | W:33 |
| job render | worker/agentbuild.go:173 | none | W:33 |
| job blocked | taskview/queue.go:32 | none | W:33 |
| job next | none (`worker next` is what runs) | agents/orchestrator.md:23 ("runs in code") | W:33 |
| job ack | none (app posts `job post --type ack`, supervisor.go:256) | none | W:33 |
| job cancel | none | none | W:33 |
| job order, trace | none | none | not named |
| job submit, checkpoint, worker-exit, integrate (feed.go:275) | none (app posts `job post --type worker-exit`, supervisor.go:53,425) | none | not named |
| feed post | feed/post.go:101 | none | W:34 |
| feed read | feed/args.go:16; taskview/view_feed.go:37; taskcode/build.go:178 | none | W:34 |
| feed state | feed/args.go:12; taskview/feed.go:23; taskview/view.go:341; gates/args.go:14 | none | W:34 |
| feed tick | none | agents/orchestrator.md:23 | W:34 |
| feed types, handover, fold | none | none | W:34 |
| task new | tasks/newtask.go:141 | skills/task/SKILL.md:21; grid-planner.md:25 | W:23 |
| task show | none | agent-templates/*.md (9 files), skills/task:28,36 | W:23 |
| task list | none | skills/task/SKILL.md:29 | W:23 |
| task status | table 104 (move_lane) | grid-planner.md:25,61 | W:23 |
| task stop, close, reopen | tasks/lifecycle.go:130; table 56,157,205 | none | W:23 |
| task history | none | skills/task/SKILL.md:30 | W:23 |
| task attach | none | none | W:23 |
| task move | tasks/intent.go:52; table 106 | none | W:23 |
| task delete, restore, rename, decline | tasks/lifecycle.go:130, tasks/edit.go:131; table 60,62,155,158 | none | not named |
| task undo | tasks/undo.go:125; the plugin has no such `case` (task.go:58-68) | none | not named |
| brain show, search, index, topics | brain/vault_more.go:155 (search) | agents/curator.md:14; grid-library.md:12-14; grid-planner.md:34; skills/visual-design-review:11 | W:27 |
| brain propose | none | grid-builder.md:34; skills/brain-keeper:17; plan-jobs:13 | W:27 |
| brain accept, keep, check, decide | table 50 (decide) | skills/brain-keeper/SKILL.md:20,38-44; agents/curator.md:13-16 | W:27 |
| brain changes | brain/vault_more.go:218; brainchanges/http.go:98,125 | skills/brain-keeper/SKILL.md:45 | W:27 |
| brain revert | none | skills/brain-keeper/SKILL.md:45 | W:27 |
| agent check | table 26,29,154 (notBuilt) | skills/brain-keeper/SKILL.md:66 (`grid agent`) | W:47 |
| agent sync | none | none | not named |
| agent list, set | table 27,163 (notBuilt) | none | W:47 |
| agent budget show | agentreads/http.go:75; table 22 (notBuilt) | none | not named |
| agent budget apply, undo | table 23,24,25 (notBuilt) | none | not named |
| account scan | agentaccounts/http.go:151 | none | W:48 |
| account folder | agentaccounts/http.go:200,323 | none | W:48 |
| account windows | agentaccounts/http.go:253 | none | W:48 |
| account sign-in | agentaccounts/http.go:339 | none | W:48 |
| account list, status, note | none (table 16 names `account scan`) | none | W:48 |
| usage show | taskview/usage.go:40 | none | W:54 |
| usage post | none | none | W:54 |
| intent list, claim, done | worker/intents.go:266,293,351 | none | not named |
| intent renew, release | worker/intents.go:251 (same wrapper) | none | not named |

App calls the plugin has no `case` for (found while reading; for the owner): `task undo` (tasks/undo.go:125; task.go:58-68 has no undo), `refusal decide` (table_gen.go:150; no `refusal` verb), and `sync --watch` (worker/run.go:167; `sync` is a stub, cli.go:146). `account folder` and `agent budget show` do exist (account.go:44, agent.go:21).

## Section B: what the one-task loop calls

Needed.md:23-31 is not in my sandbox; the list below is by role, each with the call that proves it.

| Role | Verbs | Call site |
|---|---|---|
| Orchestrator posts Jobs | plan items, plan check, job lint, job post, job tree, job cycles; the tick and `job next` run in code | agents/orchestrator.md:13,20,21,23; app `job post` judge.go:425 |
| Worker runs one agent per Job | worker next, intent list/claim/done, lease status task, where, job render, job show, job post (ack, worker-exit) | worker/job.go:288, intents.go:266,293,351, epoch.go:61, agentbuild.go:79,173,195, supervisor.go:74 |
| The agent in the Job | job show, job result, task show, proof add, brain (search, show, propose, keep), history add | agent-templates/grid-builder.md:14,26,34,38; agents/builder.md:13,16 |
| Person Jobs | approve, answer, question withdraw, plan hash, signal act | gates/args.go:15-17; gates/withdraw.go:115; inbox/act.go:131 |
| Task page reads | feed state, feed read, job blocked, session list, usage show, workflow show | taskview/feed.go:23, view_feed.go:37, queue.go:32, view_sessions.go:38, usage.go:40, view_steps.go:35 |
| Task creation, setup | task new/move/stop/close/reopen, project add/scan, person check | tasks/newtask.go:141, lifecycle.go:130, setup/http.go:148,262, plugincli/person_check.go:70 |

That is about 15 verbs: where, plan, job, lease, feed, worker, intent, approve, answer, question, signal, session, usage, workflow, person; plus task and project for the app's own writes.

## Section C: kept whatever the callers

| Verbs | Owner decision | Source |
|---|---|---|
| signal, routine | "Corrected 11 Nov: signals and routines stay" | needed.md:41 (quoted in the brief) |
| question, answer (and approve, which carries the person's answer) | questions and answers stay | simplify.md:65 row 14 (named in the brief; line text not read) |
| brain: show, search, topics, index, propose, accept, keep, decide, check, changes, revert | brain notes, changes and revert stay | needed.md:43 (named in the brief; line text not read) |
| usage (post, show) | usage as records | simplify.md:66 row 15 (named in the brief; line text not read) |

## Section E: removal candidates (last)

Rule: a verb is a candidate only when no row in A has a caller (docs excepted) and C does not hold it. The owner's rule is that nothing goes unless a list line names it, so only rows with a line are briefs. `docs-only` verbs are listed under "ask" with the doc line, not as candidates. App test callers (see NOT VERIFIED) are not in A.

### E1. Candidate briefs (no caller outside docs and the table)

| Brief | Verb | List line | Callers left | Files that would go | Tests that go | Tests that keep covering the rest |
|---|---|---|---|---|---|---|
| covered by F-cli-drop-stubs | views, ask, machine, deploy, sync (stubs, run nil) | stubs: that brief | sync: app calls it, worker/run.go:167; views: views/README.md:1 | the four table rows cli.go:110,116,124,127,146 | none (no run) | `go test ./internal/cli` |
| covered by F-cli-drop-stubs | env | needed.md:41 ("the rest goes"; no word names it) | skills/browser/SKILL.md:11; docs R:36 W:62 | internal/env/*, internal/cli/environment.go | environment_test.go: TestEnvironmentCLI, TestEnvironmentUsage; internal/env tests | `internal/packaging` TestSkillCommandsImplemented reads `grid <verb>` in skills (D-plugin-skills-a.md:9), so the skill line must change in the same brief |
| covered by F-cli-drop-stubs | ports | needed.md:41 ("ports, budgets, refusals") | docs R:36 W:63 | internal/ports/*, runPorts in internal/cli/environment.go:80 | internal/ports tests; environment_test.go: TestPortsCommands | `go test ./internal/cli` |
| E-1 (one verb: agent budget) | `agent budget apply`, `agent budget undo` | needed.md:41 ("budgets") | table_gen.go:23-25 only, marked notBuilt | Only the apply and undo branches. In internal/cli/budget.go: the `apply` and `undo` names in the op check (:34), apply's flags (:41-45) and operand count (:51-53), the `undo` case and the apply/dry-run default (:68-79). In internal/settings/budget.go: BudgetApply (:729), BudgetPreviewFor (:648), BudgetUndo (:842) and helpers only they use. `runAgentBudget`, `show` (:66, `settings.BudgetShow` :417, which the app calls at agentreads/http.go:75) and `runClaude` (:89) stay. | Tests that call the removed functions: budget_test.go TestBudgetApplyThenUndoGivesTheSameBytes :358, TestBudgetUndoSkipsChangedFile :411, TestBudgetApplyLeavesPersonsOwnFile :431, TestBudgetApplyAllOrNothing :451, TestBudgetApplyDigestMismatch :476, TestBudgetApplySurvivesPluginUpdate :575, TestBudgetUndoAfterPluginUpdateGivesFreshDefaults :624; accounts_key_test.go:57 TestBudgetApplyUndoAndSetKeepTheAccountsKey. Tests that use them in part and need the call removed, not the test: budget_test.go :218 (in TestCodexKeysReadOnly), :323 and :351 (TestBudgetNoteStrict :314, TestBudgetPresetValuesComeFromTheNote :338). The names in internal/cli/settings_choke_test.go:110 lose their three entries. | budget_test.go TestClaudeSet* and TestClaudeShowReadsEnvAndLayers (`grid claude settings`), TestBudgetEnvOverrideRead :492 and the show reads; internal/settings claude_test.go, agentsync_test.go, store_test.go; internal/cli agent tests |

### E2. Named by a list line, but the app still calls them (app removal first)

| Verb | List line | App caller to remove first | Plugin files and tests |
|---|---|---|---|
| `agent budget show` | needed.md:41 | agentreads/http.go:75; table_gen.go:22 | runAgentBudget (budget.go:28-85) and settings.BudgetShow (:417) with the BudgetShow tests; goes after the app drops http.go:75. E-1 does not touch it. |
| `session start, stop, send, pause, resume, handback, switch` (one brief) | simplify.md:60 row 9 | tasks/intent.go:35; table_gen.go:70,171-177,213; W:52 | Only the seven session actions: the `case` at internal/cli/intent.go:20-22 (the `default` usage line at :24 loses the names) and their branches in runIntentWrite: `send` :79, `switch` :87, `start` :95, the send/switch/start flags (:64-70) and the positional-value test (:52) for send and switch; stop, pause, resume and handback have no branch of their own and pass through to ValidateRequest and Enqueue (:119, :134), which stay for `move`. Stays: runSessionCommand's `list`/`show` path (:17-19, loop reads, A2); runTaskIntent (:29-35, `grid task move`, which the app calls at tasks/intent.go:52); the `move` path in runIntentWrite (:52, :99); `grid intent release` and the other `grid intent` verbs. Tests that go or lose their session cases: intent_test.go TestSessionIntentCommands :17, TestSendWhoAndKeys :191; the `session start/send/pause` calls in TestIntentCommandsLifecycle :62, TestIntentRenewCommand :134 (:137), TestIntentJSONGolden :241 (:247) and TestIntentUsage :309 (:311-313, :327, :330) need a different verb or case removed. Tests that keep covering what remains: intent_test.go TestTaskMoveIntents :337, TestTaskMoveChecksEveryRepository :377, TestTaskUsageListsMove :234; intent_release_test.go (`grid intent release`, :29, :60, not part of this removal); internal/session tests. |
| `refusals` | needed.md:41 | table_gen.go:150 names `grid refusal decide`; the plugin has no such verb. Guest refusal text lives in internal/cli/cli.go:192-228, not in a verb | ask which of these the owner means |

### E3. none: ask (no list line names them)

| Verb or sub | Why it has no caller | Doc line |
|---|---|---|
| version, open, land, migrate | no caller anywhere | R:30,39 W:11,12,39,65 (reference table only) |
| doctor | a person types it | R:54,56 W:14 |
| eval | evals/README.md only (a person runs it) | W:56 |
| claude, skill, project-md | app table only, notBuilt | W:49,50,21 |
| artifact | app table only (built rows 36-38), no Go call found | W:26 |
| deliver | topics/delivering/TOPIC.md:5,11,12 text only | W:37 |
| lease (take, renew, release, merge slot) | app uses `status task` only; merge slot is C-need-merge-lease-remove; task lease is kept (F-leases-map.md:33) | W:69 |
| worker list | no caller (`worker next` is called) | W:53 |
| feed types, handover, fold, tick; job order, trace, cancel, ack, submit, checkpoint, worker-exit, integrate; task attach; agent sync; account list, status, note; usage post; person list, show; plan diff, migrate; intent renew, release | no caller outside docs or code paths of their own | named in W:33,34,23,47,48,54,40,24 |

Next step for the owner: say yes or no to E1 (E-1), E2 (session; refusals meaning), and each E3 row; each yes becomes one brief of at most three verbs.

## What changed

Only this file.

## NOT VERIFIED

- App test callers: I searched non-test Go only. Tests such as cmd/grid-server/realplugin_seed_test.go:39, task_questions_realplugin_test.go:93, feed_refusal_realplugin_test.go:65 run the real plugin with other verbs; a verb whose only caller is such a test is not marked here.
- `version`: the app's plugincli may probe `grid version`; my grep of `"version"` argv found none, but I did not read plugincli/*.go in full.
- Needed.md and simplify.md are in neither repo, so lines 23-31, 43, 65, 66 are cited from the brief, not read.
- Whether a person types the reference-table verbs (R:30-41, W:11-65): I treated those tables as docs, not as proof of use.
- No live screen is involved.
