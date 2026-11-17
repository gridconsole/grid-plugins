# Lane P-loop-schemas

Base origin/main 63d58a47. `git fetch origin` then `git merge origin/main`: "Already up to date." Proved commit 99052c24 (the commit after it only adds this doc). All commands ran in the lane clone with Go from the module.

## Proof table

| # | Row | Verdict | Proof |
|---|-----|---------|-------|
| A1 | Schemas validate the RJ examples (A, B, Q2); a TestFixtures sub-test per new schema | HOLDS | `go test -count=1 -v -run 'TestFixtures' ./internal/contract/` exit 0. PASS for TestFixtures/approval, blocked, cancel, exit, integrated, landed, started, task-move, waited, waiting and wake, and for control, submit and job-record. RJ B: contract/fixtures/exit/valid/basic.json is the B cost line. RJ Q2: contract/fixtures/control/valid/assign-loop-fields.json is the Q2 assign. RJ A: contract/fixtures/approval/valid/basic.json and blocked/valid/basic.json. |
| A2 | V9: a task-move without `to` fails | HOLDS | `go test -count=1 -v -run 'TestFixtures/task-move\|TestPlSc' ./internal/contract/` exit 0: TestFixtures/task-move/invalid/missing-to.json PASS and TestPlScTaskMoveNeedsTo PASS. `to` is required at contract/schemas/task-move.json:53. |
| A3 | V9: the first Job record carries `home` | HOLDS | `go test -count=1 -v -run 'TestPlScFirstJobHome' ./internal/cli/` exit 0. The writer is `internal/cli/task_new_first_job.go:84` (`rec["home"] = machine`, the machine of the writer id), called by `grid task new`. |
| A4 | An assign with the new fields validates; an unknown key is refused | HOLDS | TestPlScAssignFields PASS (same run as A2). Fixtures: control/valid/assign-loop-fields.json, control/valid/assign-start-now.json, control/invalid/assign-unknown-key.json. The assign keeps `additionalProperties: false` and the required job, attempt, to, fence and until. |
| A5 | All fixtures and feed-posts examples still validate | HOLDS | `go test -count=1 -v -run 'TestFixtures\|TestTypesRegistered\|TestSemanticRules' ./internal/contract/` exit 0. `go test -count=1 -v -run 'TestT12Posts' ./contract/` exit 0 (EveryType, Preface, ExamplesValidate, Negative PASS). `./internal/feed/...` and `./internal/orchestrator/...` were not run on their own; they show `ok` lines in the whole-module run below. Tests edited: only the fixed lists in row R6. |
| S1 | `go test -count=1 ./contract/... ./internal/contract/... ./internal/feed/...` | HOLDS | exit 0 (run with ./internal/mcp/ and ./internal/cli/ added: all ok). |
| S2 | Whole module | HOLDS | `go test -count=1 ./...` filtered with `grep -E '^(--- FAIL\|FAIL\|panic)'`: no lines. |
| S3 | Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 and `go test -count=1 ./internal/job/lint/` exit 0, both run after the merge. |
| S4 | `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| S5 | `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile). |
| R1 | control.json assign gains on, account, start, expect, test_run, base {ref, sha}, allow, grant, carry {job, attempt}; no retry_of; fence/until and additionalProperties false kept | HOLDS | contract/schemas/control.json assign properties: fence :41, grant :155, carry :160; the unchanged required list (job, attempt, to, fence, until) at :180. `grep -rn retry_of contract/schemas` finds nothing. model and effort were also added, because the Q2 example sets them. |
| R2 | New post schemas with x-class and x-roles | HOLDS | contract/schemas/{approval,blocked,cancel,exit,integrated,landed,started,task-move,waited,waiting,wake}.json, each with `"x-class": "state"` and `x-roles`; TestPlScNewTypes PASS checks registration, version, class and roles. |
| R3 | submit.json drops its own lease/fence; body gains optional repo, base_sha, commits | HOLDS | contract/schemas/submit.json has no lease or fence property (grep); TestPlScSubmitFields PASS; fixture submit/valid/repo-base-commits.json. |
| R4 | envelope.json accepts `by` and `legacy`; lease/fence kept and required | HOLDS | contract/schemas/envelope.json:151 by, :156 legacy; lease :62 and fence :78 are unchanged and stay in `required`. TestPlScEnvelopeByLegacy PASS (a post without fence fails). |
| R5 | job-record.json: optional home, after kind landed, cross-task after (`task`) | HOLDS | contract/schemas/job-record.json:304 home, :336 landed, `task` beside it. TestPlScJobRecordFields PASS; fixture job-record/valid/home-after-landed.json. |
| R6 | Fixed type lists go to 30; `git log -S` shows no incident pin | HOLDS | internal/feed/contract_test.go:20, internal/contract/schema_test.go:246, contract/formats/formats_test.go:281, internal/cli/contract_commands_test.go:389, testdata/golden/mcp-session.golden:15 (regenerated with `go test -count=1 -exec 'env UPDATE_GOLDEN=1' -run TestSessionGolden ./internal/mcp/`, then passing without it). `git log -S` gives b5a7dc72, d0dd052f (formats baseline), 41dd45aa, f5ae1614 (both counts) and, with `-S 'usage-notice'`, b2bb8586, df9ece55 (the cli `want` and the golden): feature commits, no incident. The "byte-for-byte baseline" comment at contract_commands_test.go:386-387 is left as it was. |
| R7 | feed-posts.md: one section per new type, sorted, first json block validates | HOLDS | contract/docs/feed-posts.md; TestT12PostsEveryType and TestT12PostsExamplesValidate PASS. The stale cites control.json:356 and submit.json:89 now read :484 and :94 (same content). formats.md is unchanged. |
| R8 | Fixtures: at least 2 valid and 2 invalid per new schema; new valid control, submit and job-record fixtures; an invalid assign with an unknown key | HOLDS | contract/fixtures/<type>/{valid,invalid}/ hold 2 and 2 for each of the 11 types; TestPlScNewTypes asserts at least 2 of each. Old-type fixtures as in A4, R3 and R5. |
| R9 | New tests only in new files, helpers prefixed plSc; shared table at schema_test.go:58 not edited | HOLDS | internal/contract/pl_sc_loop_schemas_test.go, internal/cli/pl_sc_first_job_home_test.go. schema_test.go changes only at :246. |
| R10 | New tests fail on base and pass on the lane | HOLDS | Failing lines below; passing in A2/A3. |
| R11 | Schemas only: no fold rule, no case in fold.go | HOLDS | `git diff origin/main..HEAD -- internal/contract/fold.go` is empty. The new types fold as counted posts with no effect. |
| R12 | No guard, gate, cap, hold or limit on agents | HOLDS | No code path reads the new fields; the only code change is the `home` key at task_new_first_job.go:84. |
| R13 | Nothing removed beyond submit's lease/fence; grid lease, --lease-epoch, the assign fence and the envelope lease/fence kept | HOLDS | Diff deletions: submit.json lease/fence, the five fixed lists' old values, the one Marshal line at task_new_first_job.go:81, which now marshals the same record with `home` added, the two stale cite lines in feed-posts.md (R7), and the after-kind enum line in job-record.json, reflowed to add `landed`. |
| R14 | No pushing; V16 = option C | HOLDS | No push was run. |
| R15 | Proof is go test and make only; no claude, curl or server | HOLDS | Only go test, go run (generator, below), go list, make and git were run. |
| R16 | Public repo: no personal paths, hosts or emails | HOLDS | The machine names `vm:grid2` and `grid2:grid3` in fixtures and tests follow the names base already uses (`git grep -c -E 'grid[0-9]' origin/main`: docs/research/run-jobs-launcher.md 6, brain-board.md 3, internal/machinefolder/launcher_test.go 1). |
| R17 | The doc commit breaks no test | HOLDS | Tests that read docs/lanes (`git grep -l 'docs/lanes' -- '*_test.go'`): `go test -count=1 ./internal/feed/ ./internal/golden/ ./internal/land/` exit 0 at the doc commit. |

## Files outside the brief, forced by the change

- contract/README.md: TestIySchemaRows needs a row per schema (11 rows added).
- docs/job-contract.md: TestHfJrFields needs a row per job-record.json property (the `home` row).

## Importers of the changed packages

`go list -f '{{range .Deps}}{{if or (eq . "github.com/gridconsole/grid-plugins/contract") (eq . "github.com/gridconsole/grid-plugins/internal/cli")}}{{$.ImportPath}} <- {{.}}{{"\n"}}{{end}}{{end}}' ./...` names, for `contract`: cmd/grid, internal/{account, approvefix, artifact, brain, braincmd, cli, contract, doctor, env, feed, guard, hook, intent, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, people, ports, projectenv, projectmd, projects, question, routine, runner, session, settings, signal, task, updates, usage, where, workflow}, lib. For `internal/cli`: cmd/grid, lib. All of them are in the module run (S2).

## New tests fail on base (63d58a47)

`go test -count=1 -run 'TestPlSc' ./internal/contract/ ./internal/cli/` on the base tree with only the two new test files added, exit 1:

```
--- FAIL: TestPlScNewTypes (0.01s)
    --- FAIL: TestPlScNewTypes/approval (0.00s)
        pl_sc_loop_schemas_test.go:87: approval is not a registered post type: {Name: Version:0 Class: Roles:[]} (ok false)
    (the same line for blocked, cancel, exit, integrated, landed, started, task-move, waited, waiting, wake)
--- FAIL: TestPlScTaskMoveNeedsTo (0.00s)
    pl_sc_loop_schemas_test.go:117: task-move with from and to: unknown schema "task-move"
--- FAIL: TestPlScAssignFields (0.00s)
    pl_sc_loop_schemas_test.go:162: assign with the launcher fields: $.body: oneOf matched 0 branches: $.body: oneOf matched 0 branches: $.body.assign: unknown property account
--- FAIL: TestPlScSubmitFields (0.00s)
    pl_sc_loop_schemas_test.go:182: submit with repo, base_sha and commits: $.body: oneOf matched 0 branches: $.body: unknown property base_sha
--- FAIL: TestPlScJobRecordFields (0.00s)
    pl_sc_loop_schemas_test.go:207: job-record with home and landed edges: $.after[0]: anyOf matched 0 branches: $.after[0].kind: value outside enum
--- FAIL: TestPlScEnvelopeByLegacy (0.00s)
    pl_sc_loop_schemas_test.go:221: post with by and legacy: unknown schema "landed"
FAIL	github.com/gridconsole/grid-plugins/internal/contract
--- FAIL: TestPlScFirstJobHome (0.02s)
    pl_sc_first_job_home_test.go:53: J-1 home = <nil>, want the writer's machine "01ARZ3NDEKTSV4RRFFQ69G5FAW"
FAIL	github.com/gridconsole/grid-plugins/internal/cli
```

## Open items

1. internal/job/lint/lint.go:252 accepts only after kinds done, failed and children-done, so a Job file with `kind: landed` is still reported by lint although the schema accepts it. Lint is outside this lane.
2. Two other J-1 writers do not set `home`: the worker's fallback (internal/cli/worker.go:206, `FirstJob: firstJobOf`) and reopen (internal/cli/reopen_continue.go:156). Both files are outside Touches.
3. feed.Record has no `by` or `legacy` field, so the Feed drops them on write (B1). The valid fixtures do not carry `by` because TestRecordFixtureGolden round-trips them through feed.Record; TestPlScEnvelopeByLegacy covers the schema.
4. Shapes chosen where the docs are silent: `commits` is an integer count; `after.task` is a task ULID; `legacy` is free text of 1-40 characters; `start` is `"now"` or one level of all/any atoms (not recursive); `exit.reason` also has `failed` (RJ C point 16); `started.firing` is a date-time.
5. The RJ examples need the kept required fields: the Q2 assign needs to, fence and until, and the RJ `after: {job, kind: landed}` needs `why`. The fixtures carry them.
6. The one-off generator for schemas and fixtures sits untracked in `.plsc-gen/` in the clone. `rm` was refused in this sandbox; it is not committed and not in the bundle, and can be deleted.

DONE
