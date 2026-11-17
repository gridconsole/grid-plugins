# P-land-kind-3: command and wait Job kinds, a full-ref land branch and the tested post

V-numbers: V12, V26, V27. Owner direction V12.c=B: a separate runner Job tests, and the land Job pushes only the tested SHA; the `tested` post is what that runner Job writes.

Commits on the lane (on origin/main bb4df62f):

- c9b2ac46 failing tests only (three new files);
- 60d126c3 `command`, `every`, `timeout` record fields and the `job.command-argv` lint rule;
- 02b85f91 the `tested` post type with fixtures and docs;
- d339c70c the fixed Feed type lists and the MCP session golden raised to 31 types;
- e9c222de the README citations of waited, waiting and wake moved down one line;
- then this doc.

The proof runs below ran on e9c222de, the last code commit. The two doc commits add only this file, and `go test -count=1 ./...` was run again on the doc head with no FAIL line.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. (V12) lint refuses a land Job without target, branch or sha | HOLDS | Rules `job.land-target`, `job.land-branch`, `job.land-sha` (`internal/job/lint/lint.go:355`, unchanged by this lane). `go test -count=1 -v -run 'TestLandKind\|TestLandFields\|TestFeedLandJobRoundTrip\|TestHfJrFields' ./internal/job/... ./internal/cli/`: `--- PASS: TestLandKindRefusesEachMissingField`, `--- PASS: TestLandKindJobLintCommand`, `--- PASS: TestLandKindWithAllFieldsLintsClean`, `--- PASS: TestLandFieldsAreOnlyCheckedForKindLand`, every package `ok`. |
| 2. (V12) round-trip through the Feed keeps them | HOLDS | Same run: `--- PASS: TestFeedLandJobRoundTrip` (`internal/cli`). Also `--- PASS: TestPlKtJobKindsRoundTrip` subtest `land_full_ref` (`internal/feed/pl_kt_roundtrip_test.go:118`): it posts the record and reads it back, and target, branch and sha are each compared. |
| 3. (V12) existing tests green | HOLDS | `go test -count=1 ./...` piped into `grep -vE '^(ok\|\?)'` prints nothing: no FAIL line in any package. Also `go test -count=1 ./contract/... ./internal/mcp/ ./internal/cli/` gives `ok` for all four packages, which hold the pins and doc tests. |
| 4. (V27) a land Job whose `branch` is `refs/grid/T-1/head` passes lint and survives the round-trip | HOLDS | `--- PASS: TestPlKtLandFullRefBranchLintsClean` (`internal/job/lint/pl_kt_kinds_test.go:35`): no finding at all. `TestPlKtJobRecordKinds` subtest `land_full_ref` (`internal/contract/pl_kt_schemas_test.go:42`) validates it against `job-record.json`. `TestPlKtJobKindsRoundTrip` subtest `land_full_ref` reads it back with the branch unchanged. The doc comment `internal/job/job.go:24` and the branch row in `docs/job-contract.md` name the full ref. These tests already passed on base (see "New tests fail on base"); they pin the behaviour. |
| 5. (V26) schema and lint accept `kind: command` with a `command:` argv and `kind: wait` with `every` and `timeout`; both survive the round-trip; lint refuses a `command` Job with no argv | HOLDS | Schema `contract/schemas/job-record.json:40-57`: `command` (array of non-empty strings, minItems 1); `every` and `timeout` (`^[1-9][0-9]*(s\|m\|h\|d)$`). Fields `internal/job/job.go:33-35`. Rule `job.command-argv` at `internal/job/lint/lint.go:373-384`, called at `:188`. Schema tests: `--- PASS: TestPlKtJobRecordKinds`, 13 cases accepted or refused. Lint accepts: `--- PASS: TestPlKtCommandAndWaitLintClean`. Round-trip: `--- PASS: TestPlKtJobKindsRoundTrip` (subtests `command`, `runner_command`, `wait`), where body and folded kind and role come back. Refusal: `--- PASS: TestPlKtCommandWithoutArgvRefused` (`pl_kt_kinds_test.go:64`). It covers no `command`, `command: []` and an argv with a blank word; each gives exactly one `job.command-argv` error, and build, wait and land never get it. |
| 6. (V12.c) the `tested` post schema (`sha`, `ref`, `base_sha`, `head_sha`, `result`, `flakes[]`, `preexisting[]`) validates its example and survives the round-trip | HOLDS | New `contract/schemas/tested.json`. Body fields are at `:33-113`, the rule that green needs `ref` is at `:116-131`, and `x-class` is at `:151`. Example `contract/fixtures/tested/valid/basic.json`, also shown in `contract/docs/feed-posts.md` under `## tested`. `--- PASS: TestPlKtTestedSchema` (`internal/contract/pl_kt_schemas_test.go:114`): registered as version 1, class state; at least two valid and two invalid fixtures; each of the 7 keys missing is refused; bad hex, bad flake, bad preexisting and unknown keys are refused; a member author is refused. `--- PASS: TestPlKtTestedRoundTrip` (`internal/feed/pl_kt_roundtrip_test.go:153`): the fixture example and a `nothing` body post and read back unchanged. `TestFixtures/tested` (`internal/contract/schema_test.go:21`) is green. |
| Standing: `go test -count=1 ./internal/job/... ./internal/contract/... ./internal/feed/...` exits 0 with a `--- PASS:` line per new test | HOLDS | Exit 0: `ok` for job, job/frontmatter, job/jobcmd, job/lint, contract and feed; jobtest has no test files. With `-v`: `--- PASS: TestPlKtLandFullRefBranchLintsClean`, `--- PASS: TestPlKtCommandAndWaitLintClean`, `--- PASS: TestPlKtCommandWithoutArgvRefused`, `--- PASS: TestPlKtJobRecordKinds`, `--- PASS: TestPlKtTestedSchema`, `--- PASS: TestPlKtJobKindsRoundTrip`, `--- PASS: TestPlKtTestedRoundTrip`. |
| Standing: `go test -count=1 -run 'TestHsIpWalksEveryCommand' ./internal/cli/` after `git merge main` | HOLDS | `git merge main`: "Already up to date." Then `ok .../internal/cli`, exit 0. |
| Walk test: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok .../internal/cli`, exit 0. Run after the merge. |
| Walk test: `go test -count=1 ./internal/job/lint/` | HOLDS | `ok .../internal/job/lint`, exit 0. Run after the merge. |
| Standing: `make lint` | HOLDS | `make -C . lint` runs `go vet ./...`: no findings, exit 0. |
| Standing: `make cross` (Windows and macOS builds) | HOLDS | `make -C . cross`: darwin/arm64 and windows/amd64 each pass `go vet ./...` and `go test -run XXX_NONE ./...`, with every package `ok` or no test files; exit 0. |
| Merge main before the proof | HOLDS | `git fetch origin` then `git merge origin/main`: "Already up to date." origin/main is bb4df62f, and every run above was on that merge. Main did not move, so the pins stay at 31. `TestPostTypes` passes with 31. |
| Red-test check: `internal/feed/contract_test.go:20` pins the type count | HOLDS | Only that number changed: `30` became `31` (commit d339c70c). Four more fixed lists of the same registered types also had to change; see the open items. |
| New tests in new files, helpers prefixed `plKt` | HOLDS | Three new files. `internal/job/lint/pl_kt_kinds_test.go` has `plKtBrief` and `plKtCheck`. `internal/contract/pl_kt_schemas_test.go` has `plKtRecord`, `plKtValidate`, `plKtTested` and `plKtGreen`. `internal/feed/pl_kt_roundtrip_test.go` has `plKtStore`, `plKtWriter`, `plKtJSON`, `plKtRecord`, `plKtJSONBytes` and `plKtReadOne`. |
| New tests fail on base; failing lines pasted | HOLDS | Run at c9b2ac46, which has the tests only on origin/main code; the lines are under "Failing lines on base". Some new tests already pass on base: `TestPlKtLandFullRefBranchLintsClean`, `TestPlKtCommandAndWaitLintClean` and the `land_full_ref` subtests. Base lint only checks that branch is not empty and does not look at the new keys, and base schema already allows any branch string. They pin row 4 and the "lint accepts" half of row 5. The refused cases of `TestPlKtJobRecordKinds` also refuse on base, but for the wrong reason: base refuses every `command`, `every` and `timeout` key as unknown. |
| Do not edit an existing test to make it pass | HOLDS | No assertion or logic was changed. The only edits are the five fixed type lists that list every registered type, each raised by the one `tested` entry; see the open items. |
| Tests of changed packages and their importers (`go list`) | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...` was condensed to the direct importers. `contract`: internal/contract. `internal/contract`: artifact, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, runner, session, settings, signal, task, updates, usage, where, workflow. `internal/feed`: approvefix, board, cli, hook, job/jobcmd, lease, mcp, migrate, orchestrator, question, runner, task, usage. `internal/job`: cli, hook, job/jobcmd, job/lint, jobgraph, lease, migrate, orchestrator, ports. `internal/job/lint`: cli, job/jobcmd. That is most of the module, so the whole module ran (row 3). |
| No guard, gate, cap, hold or blocking hook on agents; no Grid-made limits | HOLDS | The only new check is the lint rule `job.command-argv` on a Job brief. Nothing touches spawn, run, hooks or agents. `every` and `timeout` have a form, a number and a unit, but no upper bound. |
| Remove nothing | HOLDS | `git diff --numstat origin/main..e9c222de` (the last code commit): 868 insertions, 12 deletions. Every deletion is a line rewritten in place: 4 pins, 1 golden line, the `job.go` branch comment, 2 `docs/job-contract.md` lines (count and branch row) and 3 README citations in `contract/docs/feed-posts.md`. No file, test, field or rule was removed. |
| No pushing | HOLDS | No push was run; the lane leaves as a bundle only. |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0. |
| No personal paths, hosts or emails in committed files | HOLDS | Committed files name only repo-relative paths. Fixtures use example ids. |
| App walk tests and `make realplugin` guest walk tables | not run, not asked | Plugin lane. |
| Docs at `docs/formats.md` | HOLDS | That file does not exist in the repo. The formats are documented where the repo keeps them: `contract/docs/feed-posts.md` (`## tested`), `contract/README.md:185` (schema row) and `docs/job-contract.md` (the `command`, `every` and `timeout` rows, the full-ref branch, 40 properties with 27 required). |

## Failing lines on base

Run at c9b2ac46, which has the tests only on origin/main code. The three test files are the same at the lane head: `git diff --stat c9b2ac46 HEAD` on them prints nothing. The lines are condensed: subtests that fail the same way share one line.

```
--- FAIL: TestPlKtCommandWithoutArgvRefused ... no_command / empty_command / empty_word: pl_kt_kinds_test.go:81: errors = [], want one job.command-argv
--- FAIL: TestPlKtJobRecordKinds/command, runner_command: pl_kt_schemas_test.go:66: Validate = $: unknown property command, want ok true
    /wait, wait_hours_and_seconds: unknown property every
--- FAIL: TestPlKtTestedSchema: pl_kt_schemas_test.go:117: tested is not a registered post type: {Name: Version:0 Class: Roles:[]} (ok false)
--- FAIL: TestPlKtJobKindsRoundTrip/command: pl_kt_roundtrip_test.go:133: job.Job record command = <nil>, want []interface {}{"land", "T-1"}
    /runner_command: same with {"land","test","T-1","J-3"}
    /wait: pl_kt_roundtrip_test.go:133: job.Job record every = <nil>, want "1m"
--- FAIL: TestPlKtTestedRoundTrip: pl_kt_roundtrip_test.go:156: open ../../contract/fixtures/tested/valid/basic.json: no such file or directory
```

## Files outside the Touches list, forced by the change

- `contract/fixtures/tested/{valid,invalid}/*.json`: `TestFixtures` (`internal/contract/schema_test.go:21`) needs at least two valid and two invalid fixtures per schema. `TestWrittenRecordsMatchContract` (`internal/feed/wire_test.go:15`) and `TestRecordFixtureGolden` (`internal/feed/record_test.go:120`) read a valid fixture for every type. The brief's "plus an example" is `valid/basic.json`.
- `contract/README.md:185`: `TestIySchemaRows` (`internal/contract/iy_schema_rows_test.go:36`) needs every schema linked as `](schemas/<name>)`. The waited, waiting and wake citations in `contract/docs/feed-posts.md` moved down one line to follow it.
- `contract/docs/feed-posts.md`: `TestT12PostsEveryType` (`contract/t12_feed_posts_test.go:147`) needs a section per registered type, and `TestT12PostsExamplesValidate` validates its example.
- `docs/job-contract.md`: `TestHfJrFields` (`internal/job/hf_record_doc_test.go:49`) needs one row per `job-record.json` property.
- Fixed type lists that fail once `tested` is registered. Each gets the one `tested` entry or 30→31. The precedent is commit 99052c24, which did the same when the count went to 30.
  - `internal/feed/contract_test.go:20` (`TestTypesRegistered`, named by the brief)
  - `internal/contract/schema_test.go:246` (`TestPostTypes`)
  - `internal/cli/contract_commands_test.go:389` (`TestContractCommandsFeedTypes`)
  - `contract/formats/formats_test.go:286` (`TestFormatFeedTypesUnchanged`)
  - `testdata/golden/mcp-session.golden` (`TestSessionGolden`, `internal/mcp/server_test.go:452`). It was edited by hand because the `UPDATE_GOLDEN=1` prefix is not on the shell allow list.

## Open items for the owner

1. The brief names one pin; five exist. All five were raised; they are listed above.
2. The `tested` shape choices bind lane P-land-change; the owner may want to change them:
   - `result` is `green` or `nothing`.
   - `ref` is required only for `green`.
   - `sha`, `base_sha`, `head_sha`, `flakes` and `preexisting` are required even for `nothing`.
   - A flake's `green` is an `"n/m"` string.
   - `preexisting` items are `{test, package}`.
   - The roles are cli, host and worker, as for `landed`.
   - The post is not folded into state; `internal/contract/fold.go` has no `tested` case.
3. `kind` stays a free string, and `job-record.json` does not tie `command` to `kind: command` with if/then. The schema accepts a command Job without an argv, and lint alone refuses it. Lint is stricter than the schema on argv words that are only whitespace.
4. The `wait` lint rule (`every` and `timeout` present for `kind: wait`) is left to P-deploy-jobs, as the brief says.
5. `job.Job` decodes the job-record fixture's string `worktree` as a map that the schema refuses. `internal/feed/pl_kt_roundtrip_test.go` drops `worktree` before decoding, as the existing cli land test does with `j.Worktree = nil`. This is an existing quirk; the lane did not change it.
6. The call budget of about 120-150 was exceeded.

## NOT VERIFIED on a live screen

Nothing: this lane has no UI.

DONE
