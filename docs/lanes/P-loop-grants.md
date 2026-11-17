# P-loop-grants: an agent posts as a person only under that person's grant (run-jobs lane 10, V22)

Source: run-jobs-launcher.md (plugin commit 048b380d) :652 and Q8 (:435-440). Owner direction V16 option C: the grant is read-side; a post without a grant is kept and counts as the agent's own, never refused.

**Ask for the orchestrator (read first).** The brief cannot be done inside its Touches list. Two files outside it are forced; see "Files outside Touches". Either add `contract/schemas/answer.json` and `internal/orchestrator/store.go` to Touches, or rule that `for`/`by`/`grant` become header keys, which also opens `internal/feed`.

Proofs ran on code head 9178793b (after `git fetch origin && git merge origin/main`). The lane doc commit that follows changes no code. Base-failure lines come from 81ba523f, the lane's start, which holds main and P-loop-board-fix1.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| (V22) (1) a granted answer folds as the person's | HOLDS | `go test -count=1 ./internal/orchestrator -run TestPlGr -v` exit 0. **`TestPlGrGrantedAnswerFoldsAsThePersons`:** the worker of J-3, with its Job lease and ack, posts an answer to dana's person Job J-1 with `for: dana`, `by: worker`, `grant: [[grid/people/dana/grants/answers]]` at a fixed time (2030-01-01T10:00Z, until 2030-01-02). After one pass J-1 is accepted with `reason: answered by dana`, and the message reads `dana answered J-1 … (posted by worker under grid/people/dana/grants/answers)`. The record keeps the agent as its author. **`go test -count=1 ./internal/contract -run TestPlGr -v` exit 0:** `TestPlGrGrantedAnswerIsThePersons` covers PostedAs and PostAs for three link spellings. It also checks that a post made before `until` is still the person's when folded later, because the expiry reads the post's own `at`. |
| (V22) (2) expired, wrong-agent or no grant folds as the agent's own | HOLDS | **`TestPlGrUngrantedAnswerFoldsAsTheAgents`** has four subtests: expired (until 09:59:59), expired at the post (until = at), wrong agent (grant names agent-9), and no grant. Each one passes: the answer is kept (not refused), J-1 stays posted, and no control closes it. Every time is fixed; the test never reads the wall clock. **`TestPlGrUngrantedAnswerIsTheAgents`** (contract) adds the cases where the author stays the agent's: by is another agent, no grant note, no grant/by key, another person's grant, a guest's grant, kind not named (approve), task not covered, and a person posting for another. |
| (V22) (3) mcp ignores an author argument | HOLDS | `go test -count=1 ./internal/mcp -run TestPlGr -v` exit 0. **`TestPlGrMcpIgnoresAnAuthorArgument`:** a session agent (agent-7, worker) posts an answer for dana whose body carries `by: dana`. The record's author is agent-7, and `body.by` is replaced with `agent-7` from the session (`internal/mcp/tools.go:222-230`). A top-level `author` argument is refused as `unknown argument` and nothing is written. That refusal predates this lane (`internal/mcp/server_test.go:278` pins it, unchanged); the lane adds no new refusal. |
| `go test -count=1 ./internal/contract/... ./internal/orchestrator/... ./internal/mcp/...` exits 0 | HOLDS | On 9178793b: `ok internal/contract`, `ok internal/orchestrator`, `ok internal/mcp`, exit 0. |
| `make lint` exits 0 | HOLDS | `make -C . lint` (`go vet ./...`), no findings, exit 0, on 9178793b. |
| `make cross` exits 0 | HOLDS | `make -C . cross`: darwin/arm64 and windows/amd64 vet and test compile, all `ok`, exit 0, on 9178793b. |
| Row 2 uses a fixed time for "expired", never the wall clock | HOLDS | The answer's `at` comes from `feed.Store.Now` set to `plGrPostedAt` (`internal/orchestrator/pl_gr_person_grant_test.go:17`), and the contract tests use the constant `plGrAt`. `contract.PostedAs` compares the post's `at` with the grant's `until` and never reads the clock (`internal/contract/fold.go:595-599`). |
| New tests fail on base | HOLDS | Run on 81ba523f with the three new test files checked out from the lane: `pl_gr_posted_as_test.go:12:21: undefined: Grant` … `:48:15: undefined: PostedAs` … `:57:17: undefined: PostAs` (`internal/contract [build failed]`); `pl_gr_person_grant_test.go:30:6: f.s.Grants undefined (type Store has no field or method Grants)` … `:128:14: undefined: LoadGrants` (`internal/orchestrator [build failed]`); `pl_gr_author_test.go:31: the answer for dana was refused: … "$.body: oneOf matched 0 branches: $.body: unknown property by"` (`--- FAIL: TestPlGrMcpIgnoresAnAuthorArgument`). The four orchestrator row-2 subtests would also pass on base if they compiled, because on base an agent's answer never closed a person Job. They pin that the lane keeps that behaviour. |
| Tests of changed packages and every package that imports them | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E 'internal/(contract\|orchestrator\|mcp)( \|$)'` lists 33 packages: contract, contract/formats, internal/{artifact, board, brain, braincmd, cli, contract, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, ports, question, runner, session, settings, signal, task, updates, usage, where, workflow}, lib. `go test -count=1` on all 33: every line `ok` (runner and updates have no test files), exit 0, on 9178793b. internal/contract, internal/orchestrator and internal/mcp ran in the standing command, and internal/job/lint in its walk test. |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok internal/cli`, exit 0, on 9178793b (after the merge). |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | `ok internal/job/lint`, exit 0, on 9178793b (after the merge). |
| Merge main before the last proof run | HOLDS | `git fetch origin && git merge origin/main`: clean merge (job-record kind land from P-land-kind). Every proof above ran after it. |
| Touches: fold.go, person.go, person_write.go, mcp/tools.go (envelope.json if header keys) | NOT DONE | Four files the lane changes are not named there. `contract/schemas/answer.json` and `internal/orchestrator/store.go` are forced and are taken up under "Files outside Touches". `internal/orchestrator/person_grants.go` is covered by the Builds line `internal/orchestrator/person*.go`. `testdata/golden/mcp-session.golden` is a golden the answer.json change forces. `person_write.go` is unchanged. |
| No guard, gate, cap, hold or blocking hook on agents; remove nothing | HOLDS | No write path gained a refusal. `PostedAs` only picks the author a reader counts. The mcp change stamps `by` on an answer and refuses nothing: the stamp is limited to `answer`, because `control.json:155` and `host.json:36` have their own body `grant`, and stamping those would have turned valid posts into schema refusals. No line was removed except the doc paragraph this lane itself had added. |
| No existing test edited | HOLDS | The new tests are only `internal/contract/pl_gr_posted_as_test.go`, `internal/orchestrator/pl_gr_person_grant_test.go` and `internal/mcp/pl_gr_author_test.go`, with helpers prefixed `plGr`. The golden is data, rewritten for the schema change. |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints `0` (grep exit 1, no match, which is the pass). |
| No personal paths, hosts or emails in committed files | HOLDS | The committed files and this doc use repo-relative paths only. |
| App rigs that read a person answer's author (gy_personjob_member_realplugin_test.go:185, d19_relay_realplugin_unix_test.go:262, hn_guest_job_answer_realplugin_test.go:46) | not run, not asked | Those are app-repo tests. Their authors are persons, not agents, so PostedAs leaves them as they are (it changes nothing for a non-agent author). |
| App walk tests | not run, not asked | Plugin lane. |

## Files outside Touches

- **`contract/schemas/answer.json`** gains optional `for`, `by` and `grant` on the first body branch.
  - The body branch has `"additionalProperties": false`, so an answer that carries them is refused without this change.
  - The other route is header keys, which needs `contract/schemas/envelope.json` (named in Touches). The header route also needs `internal/feed`, which is not named: `feed.Record`, `feed.Input` and the feed fold's `posts()` copy a fixed field list, so a new header key never reaches a reader. The envelope's `by` already means `<machine>:<login>` (P-loop-board).
- **`internal/orchestrator/store.go`** gains one field, `Grants []contract.Grant`, through which `personAnswers` reads the grants.
  - The Store has no brain folder (`store.go:59-84`).
  - The in-Touches alternatives are package-level state, or a pass that reads the environment for the brain. Both are worse than one declared field.
- **`testdata/golden/mcp-session.golden`** is forced by answer.json. `feed_types` embeds the answer schema, so `TestSessionGolden` went red until the three properties were added (by hand: `UPDATE_GOLDEN=1` as a command prefix is outside the allow list). The diff is one line, adding `by`, `for` and `grant` in sorted order.

## What changed

- `internal/contract/fold.go`: `Grant`, `PostedAs`, `PostAs`, `GrantTarget`.
  - An agent's post counts as the person's only with `for`, `by` (equal to its stamped author id) and a `grant` link to a note that:
    - is that person's;
    - belongs to a person of the team;
    - names the agent by id or role;
    - covers the post type, the person-Job kind (`approve` only when named) and the task;
    - had not expired at the post's `at`.
  - Otherwise the stamped author stands, and a reason says why. `Fold` is unchanged.
- `internal/orchestrator/person_grants.go`: `LoadGrants(brainDir)` reads `grid/people/<person>/grants/*.md`.
  - Front matter: `agents`, `posts`, `kinds`, `tasks` (one name or a list) and `until` (RFC 3339).
  - The person's role comes from `grid/people/<person>.md`.
  - A note that does not read is left out.
- `internal/orchestrator/person.go`: `personAnswers` takes a granted agent answer as the person's, whatever its Job lease. The message names the agent and the grant. The answer record is not rewritten.
- `internal/orchestrator/store.go`: `Store.Grants`.
- `internal/mcp/tools.go`: on an answer with `for` or `grant`, `by` is set from the session's author.
- `contract/schemas/answer.json` and the golden: as above.

## Open items

- **Production wiring.** No production caller sets `Store.Grants` yet: internal/cli would set it to `orchestrator.LoadGrants(<brain>)` where it builds the Store. Until then row 1 holds in tests only, and in production every agent answer stays the agent's own.
- **Name conflict.** brain-board.md:346 puts `for` (a grant link) in the per-post header, and the envelope's `by` is `<machine>:<login>`. RJ's `for: <person>, by: <agent>, grant: [[link]]` is used here as answer body keys. The adapter lane should settle one shape.
- **Contract doc.** A paragraph for `contract/docs/feed-posts.md` on `for`, `by` and `grant` was taken back out because that file is outside Touches.
- **Not done in this lane:**
  - `approval.json` (A6) carrying the same keys;
  - a status-block note for an answer whose `for` is not granted (PostedAs returns the reason);
  - project-level people grants (only the brain folder's `grid/people` is read).

## NOT VERIFIED

- That a real launched agent's `grid feed post` answer for a person lands as the person's: the wiring is missing (see Open items), so no end-to-end run was possible.
- Live screens: none, this lane has no UI.

NOT DONE
