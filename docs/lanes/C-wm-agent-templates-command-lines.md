# C-wm-agent-templates-command-lines

Base: 219ca2bb (clone of main). On it, `grep -n 'grid job post --task <task> --body' agent-templates/grid-planner.md` hit line 44 with no `--type job`, and `grep -n 'grid brain decide' agent-templates/grid-curator.md` hit line 24 as the only filing line. Both were unfixed, so the lane went ahead. (The brief names main 316ce7f2; the clone's main is 219ca2bb and carries the same stale lines.)

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 Contract walk | HOLDS | `go test -count=1 -v -run TestWmTpl ./internal/contract/` exit 0: `--- PASS: TestWmTplCommandsExist` globs `agent-templates/grid-*.md` (at least 10 files) and checks every span; subtest "a flag the command lacks is one error" runs the check on "`grid job post --task <task> --no-such-flag`" and expects one error (internal/contract/wm_tpl_commands_test.go). `go test -count=1 ./internal/contract/` exit 0. The span rule found nothing wrong on base: the drift was in meaning, not in the contract. The same file adds `TestWmTplProofAddNamesTheTask`, which was red on base for grid-checker.md. |
| 2(a) Planner post line runs | HOLDS | `go test -count=1 -v -run TestWmTpl ./internal/cli/` exit 0: `--- PASS: TestWmTplPlannerPostsJobs`; the span `grid job post --task <task> --type job --body <json>` exits 0 and `job tree` lists J-2 (internal/cli/wm_tpl_lines_test.go). |
| 2(b) Result-read spans run | HOLDS | `--- PASS: TestWmTplResultReadLines/grid-planner.md:...` and `/grid-reviewer.md:...`: `grid feed read --task <task> --job <job> --type result` on `assignedExitHarness` after `job result` prints the posted text. The test fails when a file has no such span. |
| 3 Curator walk | HOLDS | `go test -count=1 -v -run TestWmTpl ./internal/braincmd/` exit 0: `--- PASS: TestWmTplCuratorWalk` (task rows read with `--proposal --task`, filed with accept, merge-into and reject; project and home rows read, checked, accepted with `grid brain accept --project <project> brain:<level>/<id>` or rejected with `--reject`; `keep --task T-01 --json` prints `[]` after). `go test -count=1 ./internal/braincmd/` exit 0. search, topics, index and bare propose are not run. |
| 4 Proof lines run | HOLDS | `--- PASS: TestWmTplProofAddLines` for builder (1), checker (once per kind: screenshot, video, log, check), operator (4 kinds + the `--text` line) and runner (1), all from `h.home`; each exits 0 and `proof index --task` shows `i-N: <kind> proof saved`. The test fails when builder, checker, operator or runner has no proof add span. |
| 5 Red on base, then green | HOLDS | Tests were committed first (10d8e129) and run on the unfixed text. 2(a): `"grid job post --task <task> --body <json>" exits 1, stderr: Job id is required` (run with only grid-planner.md reverted). 2(b): `grid-planner.md has no \`grid feed read ... --type result\` line` and `grid-reviewer.md has no ...` (the reviewer line was present in that run only because the reviewer fix was kept; the first base run listed both missing). 3: `grid-curator.md has no level accept (\`grid brain accept --project <project> brain:<level>/<id>\`) line`. 4: `"grid proof add <item> <file> --kind screenshot --by grid-checker --note \"<what it shows>\"": exit 1: proof: give a task id or folder, or run inside a task folder` (also for video, log, check). After the fixes (f30714c7) all TestWmTpl tests print `--- PASS:`. |
| 6 Touches and checks | HOLDS | `git diff main --stat`: the three new test files, agent-templates/grid-{checker,curator,planner,reviewer}.md, agents/reviewer.md, and this doc. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestQd\|TestQi\|TestWmTpl'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0; `go test -count=1 ./internal/hook/ ./internal/contract/ ./internal/braincmd/` exit 0; `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| Packages that import the changed ones | HOLDS | Only test files and Markdown changed in Go packages. `go list` importers of internal/cli, internal/braincmd and internal/contract include cmd/grid, lib, internal/{hook,mcp,job,job/jobcmd,job/lint,orchestrator,session,workflow,task,brain,question,...}. `go test -count=1 ./cmd/grid ./lib ./internal/hook ./internal/mcp ./internal/job/... ./internal/orchestrator ./internal/session ./internal/workflow ./internal/task ./internal/brain ./internal/question` exit 0. |
| Walk tests | HOLDS | `TestHsIpWalksEveryCommand`, `TestIlGuestSiteList` and `./internal/job/lint/` pass (rows above). The app's walk tests are not this repo's. |
| No production Go file, no commands.json change, no guard | HOLDS | `git diff main --stat` lists no non-test Go file and no contract file. TestDtGateWordsTemplates passes inside `go test ./internal/contract/`; the new planner and curator text has no "gate". |
| Text of the existing tests | HOLDS | No existing test was edited or deleted. |

## What changed

- grid-planner.md: the post line is `grid job post --task <task> --type job --body <json>`; a Job's result is read with `grid feed read --task <task> --job <job> --type result`; `grid job show` reads the Job itself.
- grid-curator.md: list with `keep --task <task> --json`; a `task:` row is read with `show <id> --proposal --task <task>` and filed with `decide <task> <id> accept|merge-into <note>|reject --note "<reason>"`; a `brain:` row is read, checked and filed with `accept --project <project> brain:<level>/<id>` (or `--reject "<why>"`), as agents/curator.md says; a refused decide is run again as reject.
- grid-checker.md: the proof line carries `--task <task>`.
- grid-reviewer.md and agents/reviewer.md: the builder's result is read with `grid feed read --task <task> --job <job> --type result`, and `grid feed read` is named as a read-only command.
- Three new walk tests, named `TestWmTpl*`.

Other templates (builder, designer, library, operator, researcher, runner) needed no change: the walk shows no stale span.

## NOT VERIFIED

A real planner, curator, reviewer or checker Job started from the synced template. The lane lands NOT VERIFIED until a real planner Job posts a Job from the synced template.
