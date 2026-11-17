# P-land-kind (continued as P-land-kind-2): a land Job kind on the feed

Design: R-land-job-type sections 3 and 6, lane L1-land-kind. `internal/land` (deploy-after-ship) is unchanged.

Commits on the lane: aa0cf14a (land kind fields, lint rules, round-trip test; from P-land-kind), then in P-land-kind-2: the doc table rows, the `grid job lint` command test, the merge of origin/main, and the fix for the schema conflict that merge left.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. `kind: land` with role runner passes `grid job lint` | HOLDS | `go test -count=1 ./internal/job/jobcmd/ -run TestLandKindJobLintCommand -v`: `--- PASS: TestLandKindJobLintCommand`, exit 0. The test runs `lint <file> --task T-01 --json` (the command's own options: resolver, repos, known roles) on a rendered Job with role runner, kind land, target, branch and sha, and gets exit 0, `ok: true`, no error and no `job.role-agent` finding (`internal/job/jobcmd/land_lint_cmd_test.go`). Library level: `TestLandKindWithAllFieldsLintsClean` PASS (`internal/job/lint/land_kind_test.go`). |
| 1. needs target, branch and sha and refuses each missing one with its own code | HOLDS | Rules `job.land-target`, `job.land-branch`, `job.land-sha` (`internal/job/lint/lint.go`, func `land`). `TestLandKindRefusesEachMissingField` PASS (one error per missing field, three when all are missing); `TestLandKindJobLintCommand` PASS (each missing field: exit 1, `ok: false`, exactly one `job.land-<field>` error); `TestLandFieldsAreOnlyCheckedForKindLand` PASS (another kind is never asked for them). |
| 2. A land Job's record round-trips through `grid feed post` / `grid feed state` | HOLDS | `go test -count=1 ./internal/cli/ -run TestFeedLandJobRoundTrip -v`: `--- PASS: TestFeedLandJobRoundTrip`, exit 0. The record built by `job.Job.Record()` validates against `job-record.json`, posts with `grid feed post --type job`, reads back whole (kind, target, branch, sha) from `grid feed read`, and `grid feed state --json` validates against `cmd-feed-state` and folds the Job with kind land and role runner. Target, branch and sha are kept in the record body, not folded into the state: section 6 makes `internal/contract/types.go` a doc-only change. |
| 3. Existing lint tests stay green; `go test ./internal/job/... ./internal/contract/...` 0 fail | HOLDS | `go test -count=1 ./internal/job/lint/ ./internal/job/... ./internal/contract/...`: `ok` for job/lint, job, job/frontmatter, job/jobcmd and contract; jobtest has no test files. Exit 0. |
| 3. `make lint` 0 fail | HOLDS | `make -C . lint`: runs `go vet ./...`, no findings, exit 0. |
| 3. Walk test: `go test ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok .../internal/cli`, exit 0 (run after the merge of main). |
| 3. Walk test: `go test ./internal/job/lint/` | HOLDS | `ok .../internal/job/lint`, exit 0 (run after the merge of main). |
| 3. Merge main into the lane before the proof | HOLDS | `git fetch origin && git merge origin/main`: conflicts in `internal/job/job.go`, `docs/job-contract.md` and `contract/schemas/job-record.json`, each resolved by keeping both sides (main added base, model and effort beside the land fields). All proofs above ran after the merge. |
| New tests fail on base | HOLDS | With `internal/job/job.go`, `internal/job/lint/lint.go` and `contract/schemas/job-record.json` checked out from aa0cf14a^ (then restored; `git status --short` clean): `land_kind_test.go:68: findings = [], want one job.land-target error` (likewise branch, sha) and `land_kind_test.go:73: all missing: findings = [], want three`; `land_lint_cmd_test.go:91: no target: exit 0 ok true land findings []; want exit 1 and one job.land-target error`; `feed_land_roundtrip_test.go:30:4: j.Target undefined (type job.Job has no field or method Target)` (`internal/cli [build failed]`). `TestLandKindWithAllFieldsLintsClean` and `TestLandFieldsAreOnlyCheckedForKindLand` also pass on base: they guard that the lane adds no finding to a full land Job or to another kind. |
| Tests of changed packages and every package that imports them | HOLDS | `go list` finds 33 packages that import `internal/job`, `internal/job/lint` or `internal/contract`, so the whole module was run: `go test -count=1 ./...` with non-ok lines filtered prints nothing, exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| No existing test edited, no test deleted | HOLDS | The lane adds `internal/job/lint/land_kind_test.go`, `internal/job/jobcmd/land_lint_cmd_test.go` and `internal/cli/feed_land_roundtrip_test.go`; `TestHfJrFields` went red on aa0cf14a and was made green by adding doc rows, not by editing the test. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The only new check is three lint rules on the Job brief; nothing on spawn, run or agents. |
| No attribution lines; no private paths, hosts or emails in committed files | HOLDS | Commit messages are one sentence with no trailers; committed files name only repo-relative paths. |
| App walk tests and `make realplugin` guest walk tables | not run, not asked | Plugin lane; those are app-repo checks. |
| Whole-module and cross-repo set at landing | not run, not asked | Run by the orchestrator at landing (the whole module was also run here). |

## Files outside the L1 file list, forced by the change

- `contract/schemas/job-record.json`: the schema has `"additionalProperties": false` at the top level, so a record carrying target, branch and sha does not validate without the three properties (`TestFeedLandJobRoundTrip` validates it).
- `docs/job-contract.md`: `TestHfJrFields` (`internal/job/hf_record_doc_test.go`) needs one row in the Job record fields table per schema property; it failed on aa0cf14a with `property branch has no row in the Job record fields table` (likewise sha, target).

## What changed

- `internal/job/job.go`: `Target`, `Branch`, `SHA` on `job.Job`, omitted from the record when empty.
- `internal/job/lint/lint.go`: `land` check, one error rule per missing field, only for `kind: land`.
- `internal/contract/types.go`: doc comment on `Kind` only.
- `contract/schemas/job-record.json`, `docs/job-contract.md`: the three properties and their rows.
- Tests: `internal/job/lint/land_kind_test.go`, `internal/job/jobcmd/land_lint_cmd_test.go`, `internal/cli/feed_land_roundtrip_test.go`.

## NOT VERIFIED on a live screen

Nothing: this lane has no UI.

DONE
