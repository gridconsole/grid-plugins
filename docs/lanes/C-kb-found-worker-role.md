# C-kb-found-worker-role: a Job agent (role worker or agent) can post a `found` record under its own Job lease

Repo: plugin. Branch lane-C-kb-found-worker-role from main dc301fce. Head: see the last commit of the bundle.

## Proof table

| Row / rule | State | Proof |
|---|---|---|
| Base check: the new test fails on the role enum, not on a lease or flag | HOLDS | Before the schema change, `go test -count=1 -v -run 'TestKbRole' ./internal/cli/` exit 1: `kb_found_role_test.go:69: found post as worker under the J-1 lease: exit 3 $.author.role: value outside enum`, the same for `role=agent`, and `kb_found_role_test.go:100: security found on J-2 under the J-1 lease: exit 3 $.author.role: value outside enum` |
| App check (no rig pins the found roles) | HOLDS, output differs | App main is now a025dc1c, not 0bcf2535. `git -C /srv/gridwork/ref/app.git grep -n -E '"found"\|feed", "types"' main -- '*realplugin*'` is not empty: 8 hits, all of them the `"found":[]` field of a result body (hq_journey, hq_replay_approve_land, jl_judge_person_point, jl_judge_rework, job_env_inherited, replay_first_task). None is a `found` post or a role list. `git -C /srv/gridwork/ref/app.git grep -n -E '"--type", *"found"\|--type found\|"reviewer", *"lead", *"cli"' main` prints nothing (exit 1). The list only gets wider, so nothing that was accepted is now refused. |
| 1. TestKbRoleWorkerPostsSecurityFound | HOLDS | `go test -count=1 -v -run 'TestKbRole' ./internal/cli/` exit 0: `--- PASS: TestKbRoleWorkerPostsSecurityFound` (subtests role=worker and role=agent). Runs `job ack J-1` and then `job post --task <path> --job J-1 --attempt <J-1 attempt> --type found` as GRID_ROLE=worker with GRID_LEASE=J-1. Checks exit 0, the record has type `found`, author role worker (or agent) and lease `J-1`, `Jobs["J-1"].NeedsLead` is true and `state.Wakes` holds the record id. The test is in internal/cli/kb_found_role_test.go:63-92. |
| 2. TestKbRoleFoundNamesAnotherJob | HOLDS | Same command, exit 0: `--- PASS: TestKbRoleFoundNamesAnotherJob`. J-2 is posted with a.postJob. A security found with body.job J-2, posted under the J-1 lease, sets `Jobs["J-2"].NeedsLead` true and leaves `Jobs["J-1"].NeedsLead` false. A `bug` found is then stored (2 found records), J-1 still has no NeedsLead, and `len(Wakes)` does not change (kb_found_role_test.go:94-124). |
| 3. TestKbRoleGuestStillRefused | HOLDS | Same command, exit 0: `--- PASS: TestKbRoleGuestStillRefused` with subtests guest, member, owner, curator, host, external and lease=J-2. Every refused role gets a non-zero exit and the store holds no `found` record. member, owner, curator and host are refused with `outside enum`. guest and external are refused earlier with `task not found`, as on base. Posting with `--job J-2` while GRID_LEASE=J-1 gets exit 3 `stale lease` and stores no found record (kb_found_role_test.go:126-158). This test only checks that these roles are still refused, so it also passes on base. Rows 1 and 2 are the ones that fail on base. |
| 4a. Feed types pins | HOLDS | `go test -count=1 -v -run 'TestContractCommandsFeedTypes\|TestFormatFeedTypesUnchanged' ./internal/cli/ ./contract/formats/` exit 0: `--- PASS: TestContractCommandsFeedTypes`, `--- PASS: TestFormatFeedTypesUnchanged` |
| 4b. `go test -count=1 ./internal/contract/` | HOLDS | Part of `go test -count=1 ./...`, which printed `ok` for internal/contract (see the importers row) |
| 4c. `git diff main --stat` names only the four Touches files | DIFFERS | It names 5 files: the 4 from Touches plus `testdata/golden/mcp-session.golden` (1 line). That golden is a third copy of the `feed types` output that the brief did not list. `TestSessionGolden` (internal/mcp/server_test.go:452) calls `feed_types` and pins the found schema, so the golden holds the found role list three times: the feed types entry, the schema `author.role` enum and the schema `x-roles`. Without the update, `go test -count=1 -v -run TestSessionGolden ./internal/mcp/` exits 1 with `server_test.go:466: mcp-session differs from the golden file`. Only those three strings changed, each to `["reviewer","lead","worker","agent","cli"]`, in their own commit, so the orchestrator can drop that commit and move it to a follow-up brief. With it the same command exits 0 with `--- PASS: TestSessionGolden`. |
| 5. `make lint` / `make cross` | HOLDS | `make lint` exit 0 (`go vet ./...`). `make cross` exit 0: the Windows and macOS builds, and vet passed. |
| Rule: tests of changed packages and importers | HOLDS | The schema is embedded in package `contract`. `go list -test -f '{{.ImportPath}} {{join .Deps " "}}' ./...` filtered on `grid-plugins/contract` lists cmd/grid, contract/formats, internal/{account, approvefix, artifact, brain, braincmd, cli, contract, doctor, env, eval, feed, golden, guard, hook, intent, job, jobgraph, job/jobcmd, job/jobtest, job/lint, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, people, plan, ports, projectmd, projects, question, routine, session, settings, signal, task, updates, usage, where, workflow} and lib, which is nearly the whole module. I ran `go test -count=1 ./...` as the superset: 60 `ok`. Three packages were red in that run. internal/mcp was red on TestSessionGolden, now fixed (row 4c). internal/cli and internal/orchestrator hit `panic: test timed out after 10m0s`, Go's default per-package timeout, while `make cross` was running at the same time. |
| Rule: internal/cli, internal/orchestrator, internal/mcp green on the lane head | HOLDS | `go test -count=1 -timeout 40m ./internal/cli/ ./internal/orchestrator/ ./internal/mcp/` exit 0: `ok internal/cli 121.532s`, `ok internal/orchestrator 169.405s`, `ok internal/mcp 1.251s` |
| Rule: walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`). `go test -count=1 ./internal/job/lint/` exit 0. |
| Rule: new tests in the one new file, `kbRole`/`TestKbRole` names | HOLDS | internal/cli/kb_found_role_test.go only. Its helpers are kbRoleAgent, kbRoleJobRecord, kbRolePost and kbRoleFound. No existing test was edited apart from the two feed types pins and the golden, which carry the contract change. |
| Rule: no guard, gate, cap, hold or hook | HOLDS | The diff only widens a role enum. No code changed. |
| Rule: no attribution lines, no personal paths or hosts | HOLDS | `git log origin/main..HEAD` messages are one sentence each, with no trailers |
| Not touched: contract/README.md, internal/cli/feed.go | HOLDS | `git diff main --stat` |

## What changed

- contract/schemas/found.json: `author.role` enum and `x-roles` now read `reviewer, lead, worker, agent, cli`. Nothing else in the file changed.
- internal/cli/contract_commands_test.go:389 and contract/formats/formats_test.go:281: the found entry of the `feed types` pin only.
- testdata/golden/mcp-session.golden: the same role list, three times, for the found entry and schema that `feed_types` prints (see row 4c).
- internal/cli/kb_found_role_test.go: new, rows 1-3.

## NOT VERIFIED on a live screen

Nothing in this lane is UI.

## Open items

- WAIT-C-kb-readme-found-roles: contract/README.md:261 still says "reviewers may post found". That brief belongs to another lane.
- Row 4c differs from the brief: the diff names 5 files, because the golden is a pin of the found roles that the brief did not list.
