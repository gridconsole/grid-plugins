# C-jd-ic-job-post-exists-fix

Finishes C-ic-job-post-exists. The lane merges lane-C-ic-job-post-exists (333c8b07) onto plugin main 31824184. It then deletes the second `J-1` post from two test setups. That post repeated the J-1 the orchestrator harness already writes. With C-ic's `job_exists` refusal in place it exits 3.

## Proof table

| Row | Status | Proof |
| --- | --- | --- |
| 1. TestApproveWaitsForTheAfterJob passes | HOLDS | `go test -count=1 -run '^TestApproveWaitsForTheAfterJob$' -v ./internal/cli/` exit 0: `--- PASS: TestApproveWaitsForTheAfterJob (0.22s)`, `ok internal/cli 0.247s`, no SKIP. |
| 2. Fu2 waiting and GlJobWaitingCode pass | HOLDS | `go test -count=1 -run '^(TestFu2ApproveJobWaiting\|TestGlJobWaitingCode)' -v ./internal/cli/` exit 0: PASS for TestFu2ApproveJobWaitingJSON, TestFu2ApproveJobWaitingWithoutJSON, TestFu2ApproveJobWaitingRoles (/owner, /member, /agent) and TestGlJobWaitingCode, no SKIP. |
| 3a. Package run (C-ic row 5b) | HOLDS | `go test -count=1 ./internal/feed/ ./internal/orchestrator/ ./internal/approvefix/ ./internal/mcp/ ./internal/contract/ ./internal/cli/` exit 0: `ok internal/feed 4.345s`, `ok internal/orchestrator 165.954s`, `? internal/approvefix [no test files]`, `ok internal/mcp 0.846s`, `ok internal/contract 1.611s`, `ok internal/cli 122.840s`. internal/approvefix has no test files (`ls internal/approvefix/` lists only approvefix.go). The unpiped rerun is in the importers row. |
| 3b. TestIcj* | HOLDS | `go test -count=1 -run '^TestIcj' -v ./internal/feed/ ./internal/cli/` exit 0: PASS for TestIcjJobExistsRefused, TestIcjOneWinner (feed), TestIcjJobPostExistsCLI, TestIcjUnleasedKeepsFirstJob, TestIcjErrorCode (cli). |
| 3c. No other test posts a second record for an existing Job id | HOLDS | The 3a run and the importers run below fail nothing, so no other test hits `job_exists`. The same fix was not needed anywhere else. |
| Diff of the two tests is only the six removed lines | HOLDS | `git diff 333c8b07 -- internal/cli/approve_job_test.go internal/cli/fu2_approve_job_waiting_test.go` shows two hunks, each `-` three lines (approve_job_test.go:143-145, fu2_approve_job_waiting_test.go:33-35) and no `+` line. |
| Only Touches files plus the merge's files | HOLDS | `git diff --name-only origin/main...origin/lane-C-ic-job-post-exists` lists C-ic's 7 files: contract/README.md, docs/lanes/C-ic-job-post-exists.md, internal/cli/error_codes.go, internal/cli/icj_job_post_exists_test.go, internal/feed/icj_job_exists_test.go, internal/feed/jobid.go, internal/feed/store.go. `git diff --name-only origin/main HEAD` lists those 7 plus internal/cli/approve_job_test.go, internal/cli/fu2_approve_job_waiting_test.go and docs/lanes/C-jd-ic-job-post-exists-fix.md, and nothing else. `git diff --name-only 333c8b07` also lists main's moves from ee8a96c1 to 31824184, which are not this lane's. |
| Merge note: error_codes.go | HOLDS | internal/cli/error_codes.go:31-35 has `CodeWorkflowFileChanged` and then `CodeJobExists`. :12 and :16 import `internal/feed` and `internal/workflow`. :55-58 has `workflow.ErrChanged` and then `feed.ErrJobExists`. Merge commit ba83be44. |
| Merge note: contract/README.md count | HOLDS | `grep -c "^	Code[A-Za-z]* *= " internal/cli/error_codes.go` gives 7. contract/README.md:85 reads "The first seven constants". The table keeps rows `workflow_file_changed` (:76) and `job_exists` (:83). |
| Merge note: merges cleanly with main | HOLDS | `git fetch origin main` gives origin/main 31824184. `git merge-base --is-ancestor 6feacb3a origin/main` exit 0. `git merge-tree --write-tree HEAD origin/main` exit 0 (tree only, no CONFLICT). |
| Fails on base, passes on lane | HOLDS | On merge commit ba83be44, before the fix, the row 1 and row 2 runs exit 1. `approve_job_test.go:144: 3 Job J-1 already exists (post 01M757ZQ3FMPD68TRSHBH2J611); revisions need a new Job`. `fu2_approve_job_waiting_test.go:70`, `:100`, `:135` (owner, member), `:147` (agent) and `gl_job_waiting_code_test.go:42`: the same `3 Job J-1 already exists ...` line. On 9e5960da they pass (rows 1, 2). No new test file. |
| No existing assertion edited, no test deleted | HOLDS | The diff above removes only setup lines. Every assertion is unchanged. The harness J-1 (orchestrator_test.go:43, contract/fixtures/job-record/valid/basic.json) equals `personBody(t, "J-1", nil)` (person_job_test.go:12-27 sets only `id`, which is already `J-1`). The asserted `posted` status of J-1 (approve_job_test.go:160, fu2_approve_job_waiting_test.go:55) comes from the harness's record. |
| No production file changed by this lane | HOLDS | Outside the merge, the lane commits are 9e5960da (two `_test.go` files) and this report. error_codes.go and contract/README.md change only in the merge resolution the Merge note names. |
| Tests of changed packages and their importers | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep -E " github.com/gridconsole/grid-plugins/internal/(feed\|cli)( \|$)" \| cut -d' ' -f1` gives cmd/grid, internal/approvefix, internal/cli, internal/hook, internal/job/jobcmd, internal/lease, internal/mcp, internal/migrate, internal/orchestrator, internal/question, internal/task, internal/usage, lib. Changed packages: internal/feed and internal/cli (from C-ic and the merge); internal/contract reads contract/README.md. The unpiped run `go test -count=1 ./internal/feed/ ./internal/orchestrator/ ./internal/approvefix/ ./internal/mcp/ ./internal/contract/ ./internal/cli/ ./internal/hook/ ./internal/job/jobcmd/ ./internal/lease/ ./internal/migrate/ ./internal/question/ ./internal/task/ ./internal/usage/ ./cmd/grid/ ./lib/` exit 0: ok feed 12.045s, orchestrator 164.260s, mcp 1.514s, contract 7.248s, cli 132.821s, hook 26.791s, job/jobcmd 14.984s, lease 0.383s, migrate 35.860s, question 12.517s, task 26.837s, usage 2.312s, lib 154.802s. approvefix and cmd/grid have no test files. |
| make lint | HOLDS | `make lint && git status --short` printed only `go vet ./...`, so there were no vet findings and no "gofmt needed" line. `git status --short` ran after the `&&`, which means make succeeded, and it printed nothing. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./... && GOOS=darwin go build ./... && GOOS=windows go test -run XXX_NONE -count=1 -exec /bin/true ./internal/cli/ ./internal/feed/` exit 0. `make cross` was not run, as the brief says; the orchestrator runs it on the merge. |
| No guard, gate, cap or hold on agents | HOLDS | The lane adds none. It deletes six test setup lines. C-ic's `job_exists` refuses a malformed write, a second record for one Job id. It limits nothing an agent may do. |
| No clock in a test, no expected-fail test | HOLDS | No test was added. The removed lines hold no sleep or timeout. |
| RealPlugin rows | N/A | The brief names no *RealPlugin* test. |
| Commits: one sentence, no attribution | HOLDS | `git log --first-parent --oneline origin/main..HEAD` gives this lane's three commits: ba83be44 (merge), 9e5960da and the report commit. Each message is one sentence. `git log --first-parent --format=%B origin/main..HEAD \| grep -ci "co-authored\|claude-session"` prints 0. C-ic's own commits come in through the merge because C-ic is not on main yet. |
| Public repo: no personal paths, hosts or emails | HOLDS | The diff adds only test-line removals, the merge resolution and this report, and none of them holds a path outside the repo or an email. |

## What changed

- internal/cli/approve_job_test.go: three lines removed (the old :143-145, the `job post --type job --job J-1` setup in TestApproveWaitsForTheAfterJob).
- internal/cli/fu2_approve_job_waiting_test.go: three lines removed (the old :33-35, the same setup in waitingApproveHarness).
- Merge of lane-C-ic-job-post-exists onto main 31824184. internal/cli/error_codes.go keeps both constants and both cases, ia's first. contract/README.md:85 now reads "first seven constants".

## NOT VERIFIED

Nothing needs a live screen. There is no UI change.

## Open items

None.

DONE
