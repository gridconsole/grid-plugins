# M-C-kb-found-worker-role-onto-main: lane C-kb-found-worker-role onto plugin main 21f02736

Repo: plugin. Branch lane-M-C-kb-found-worker-role-onto-main, lane-C-kb-found-worker-role with origin/main 21f02736 merged in.

## Proof table

| Row / rule | State | Proof |
|---|---|---|
| 1a. origin/main is an ancestor of HEAD | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0 |
| 1b. `git diff origin/main --stat` names only lane files | HOLDS | 6 files, all from lane C-kb-found-worker-role: contract/formats/formats_test.go, contract/schemas/found.json, docs/lanes/C-kb-found-worker-role.md, internal/cli/contract_commands_test.go, internal/cli/kb_found_role_test.go, testdata/golden/mcp-session.golden (this lane's own doc is added after the merge) |
| Merge conflicts | HOLDS | `git merge origin/main` completed with no conflict; nothing dropped from either side. Main's own additions (kb_reviewer_agent_found_test.go, review-posts-found eval, jz probe test) came in untouched. |
| 2a. C-kb row 1: TestKbRoleWorkerPostsSecurityFound | HOLDS | `go test -count=1 -v -run 'TestKbRole' ./internal/cli/` exit 0: `--- PASS: TestKbRoleWorkerPostsSecurityFound (0.98s)` |
| 2b. C-kb row 2: TestKbRoleFoundNamesAnotherJob | HOLDS | same command: `--- PASS: TestKbRoleFoundNamesAnotherJob (0.18s)` |
| 2c. C-kb row 3: TestKbRoleGuestStillRefused | HOLDS | same command: `--- PASS: TestKbRoleGuestStillRefused (1.31s)` |
| 2d. C-kb row 4: feed types pins | HOLDS | `go test -count=1 -v -run 'TestContractCommandsFeedTypes\|TestFormatFeedTypesUnchanged' ./internal/cli/ ./contract/formats/` exit 0: `--- PASS: TestContractCommandsFeedTypes`, `--- PASS: TestFormatFeedTypesUnchanged`. `go test -count=1 ./internal/contract/ ./contract/...` exit 0. contract/schemas/found.json:23-24 and :110-111 hold `worker`, `agent`. |
| 2e. C-kb row 4c: golden | HOLDS | `go test -count=1 ./internal/mcp/` exit 0 (TestSessionGolden pins the three role lists in testdata/golden/mcp-session.golden) |
| 2f. C-kb app check | HOLDS, output differs | `git --git-dir=/srv/gridwork/ref/app.git grep -n -E '"found"\|feed", "types"' main -- '*realplugin*'` prints 8 hits, all the `"found":[]` field of a result body in internal/kernel/worker/*_realplugin_unix_test.go. None posts a `found` record or pins a role list. The role list only got wider. (As recorded in docs/lanes/C-kb-found-worker-role.md.) |
| 2g. C-kb base check (new test fails on the role enum on base) | HOLDS (earlier proof) | Recorded in docs/lanes/C-kb-found-worker-role.md row "Base check": `$.author.role: value outside enum`. Unchanged by this merge: the lane's test and schema are the same bytes. |
| 3. Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v` exit 0: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`. `go test -count=1 ./internal/job/lint/` exit 0 (`ok .../internal/job/lint 0.379s`). |
| 4a. `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`) |
| 4b. `make cross` | see Open items | see below |
| Rule: tests of changed packages and importers | HOLDS | Changed: internal/cli (test), contract/formats (test), schema embedded in package contract, golden used by internal/mcp. Importers by `go list`: cmd/grid, lib. `go test -count=1 ./internal/cli/ ./internal/mcp/ ./contract/formats/ ./cmd/grid/ ./lib/...` exit 0: ok internal/cli 94.4s, ok internal/mcp 2.3s, ok contract/formats 0.6s, cmd/grid no test files, ok lib 32.7s. `./internal/contract/` ok 1.5s. The whole-module test is the orchestrator's. |
| Rule: no guard, gate, cap, hold or hook | HOLDS | The lane only widens a role enum; the merge adds no code. |
| Rule: no attribution lines, no personal paths or hosts | HOLDS | Merge commit message is one sentence, no trailer. The paths in this doc are `/srv/gridwork/...` grid paths, as in the existing lane docs. |
| Rule: remove nothing | HOLDS | merge only; `git diff origin/main --stat` shows 6 files, 202 insertions, 3 deletions (the three deletions are the three role-list lines the lane replaces). |

## What changed

Merge of origin/main (21f02736) into lane-C-kb-found-worker-role, no conflicts, no edits of either side. The lane's content is as in docs/lanes/C-kb-found-worker-role.md: found.json roles, the two feed types pins and the mcp-session golden, and the new kb_found_role_test.go.

## NOT VERIFIED on a live screen

Nothing in this lane is UI.

## Open items

- contract/README.md:261 still says "reviewers may post found" (WAIT-C-kb-readme-found-roles, another lane).
