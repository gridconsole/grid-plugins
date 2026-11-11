# C-jd-hn-job-refusal-code-fix

Lane branch lane-C-jd-hn-job-refusal-code-fix, from main 31824184 with lane-C-hn-job-refusal-code (a34dbf80, which carries M-ho 01bd2ee4 and gs 50f05e55) merged in. Code head bd70b3a4; this report is the commit on top.

Commits:
- 833e986c Merge lane-C-hn-job-refusal-code onto main 31824184 and resolve the walk, README and usage conflicts.
- 54ca1890 Return the ErrTaskNotFound sentinel for the guest's outside-the-Grid-home and guest-file task refusals.
- bd70b3a4 Pin the job lint guest refusal code in the parity test and add the guest outside-path sentinel test.

## Proof table

| Row | Verdict | Proof |
| --- | --- | --- |
| 1 merge onto main | HOLDS | `git merge-base --is-ancestor 31824184 HEAD` exit 0. `go test -count=1 -run '^(TestGuestProjectParity\|TestGg.*)$' -v ./internal/cli/` exit 0: `--- PASS: TestGgUnknownFlagWalk (0.59s)`, `--- PASS: TestGgUnknownTaskWalk (0.20s)`, `--- PASS: TestGgUnknownProjectWalk (0.00s)` (and the three `...CoversTheContract`). Resolved map below. On a34dbf80 the ancestor check exits 1 (not re-run here; the merge of 31824184 conflicted in three files, so a34dbf80 does not contain it). |
| 2 guest outside sentinel | HOLDS | `go test -count=1 -run '^TestJdHnGuestOutsideIsTaskNotFound$' -v ./internal/where/` exit 0: `--- PASS: TestJdHnGuestOutsideIsTaskNotFound (0.00s)`. Before 54ca1890 (on 833e986c, where.go:219 still `fmt.Errorf("task not found")`) exit 1: `jdhn_guest_outside_sentinel_test.go:25: guest outside path: errors.Is(task not found, ErrTaskNotFound) = false, want true` / `--- FAIL: TestJdHnGuestOutsideIsTaskNotFound (0.00s)`. Test: internal/where/jdhn_guest_outside_sentinel_test.go (fixture of guest_test.go:245-264; guest errors.Is + text; owner keeps "outside the Grid home"). |
| 3 guest parity | HOLDS | Same run as row 1, exit 0: `--- PASS: TestGuestProjectParity (4.69s)`, `--- PASS: TestGuestProjectParity/job_lint/project-task/json/invited`, `--- PASS: TestGuestProjectParity/job_lint/project-task-project-id/json/invited`, `--- PASS: TestGuestProjectParity/job_lint/project-task-task-path/json/invited`. No "still gets unknown answer" row for another verb; guestParityEarlyAnswer not widened (only the `job lint` json literal, internal/cli/guest_parity_test.go:444). Failure on a34dbf80 at :393 taken from the review (review-C-hn-job-refusal-code.md, Must-fix 1); not re-run here. |
| 4 package tests | HOLDS | `go test -count=1 ./internal/job/... ./internal/cli/ ./internal/where/ ./internal/braincmd/` exit 0 (cli 68.5 s). `go test -count=1 -run '^TestHnCode' -v ./internal/job/jobcmd/` exit 0: PASS TestHnCodeUsage, TestHnCodeLookup, TestHnCodeUnclassified, TestHnCodeWalk. |
| Importers tested | HOLDS | `go list -f ... ./...` importers of internal/where and internal/job/jobcmd: braincmd, cli, guard, hook, job/jobcmd, job/jobtest, land, projectmd, projects, question, settings, signal, task, workflow. `go test -count=1 ./internal/guard/ ./internal/hook/ ./internal/land/ ./internal/projectmd/ ./internal/projects/ ./internal/question/ ./internal/settings/ ./internal/signal/ ./internal/task/ ./internal/workflow/` exit 0 (the rest are in row 4; jobtest has no tests). |
| make lint | HOLDS | `make lint` exit 0 (`go vet ./...`). |
| Windows / macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. `make cross` not run, per the brief. |
| diff scope | HOLDS | `git diff --stat origin/main`: contract/README.md, internal/cli/gg_unknown_{flag,project,task}_walk_test.go, internal/job/jobcmd/{command,trace,verify}.go, internal/job/jobcmd/hn_refusal_code_test.go, docs/lanes/C-hn-job-refusal-code.md (hn); internal/cli/feed.go, internal/cli/gs_feed_types_scope_test.go, docs/lanes/{C-gs-feed-types-scope-check,M-ho-feed-types-scope-onto-main}.md (gs/M-ho); internal/where/where.go, internal/cli/guest_parity_test.go, internal/where/jdhn_guest_outside_sentinel_test.go, this report (this brief). |
| merges onto main | HOLDS | `git merge-tree --write-tree HEAD origin/main` exit 0 (ed39e701). |
| only the named lines changed | HOLDS | internal/where/where.go:219 `return "", "", ErrTaskNotFound`; internal/job/jobcmd/command.go:199 `fail(where.ErrTaskNotFound, 1)` and :511 (was :495 on a34dbf80) `return "", where.ErrTaskNotFound`; command.go:81 main's `const usage` (with `\n` before Exit status) after the lane's refusalCode at :66-79. |
| README | HOLDS | contract/README.md:63 is the brief's two sentences; main's side of the conflict kept (lane's sentence and "Person-gate refusals" paragraph dropped); `grep -c -e 'job render' -e 'job verify' contract/README.md` = 1. |
| New tests fail on base, pass on lane | HOLDS | Row 2 above. The parity literal is an edit of an existing pin for the behaviour this brief changes (same assertion, plus `code`); plain-mode literal unchanged. |
| No test deleted, no guard/gate/cap | HOLDS | No test removed; the walk map loses `signal act` and `feed types` entries because main and M-ho fixed those members (the walk fails for a listed member that now answers well). No guard, gate, cap or hook added. |
| Helper prefix | HOLDS | New file defines only TestJdHnGuestOutsideIsTaskNotFound; no package-level helpers. |
| Commits | HOLDS | One sentence each, no attribution lines. |
| No clock in tests | HOLDS | New test has no sleep, timeout or elapsed-time check. |

Resolved ggTaskKnownOpen (internal/cli/gg_unknown_task_walk_test.go:58-64):

```go
// its own on stdout with an "error" string, and stderr stays empty.
var ggTaskKnownOpen = map[string]ggTaskOpen{
	"job lint":   {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example","code":"no_such_task"}`},
	"job render": {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example","code":"no_such_task"}`},
	"job show":   {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example","code":"no_such_task"}`},
	"job verify": {1, `stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example","code":"no_such_task"}`},
}
```

## What changed

- Merge: hn's stack onto main 31824184; conflicts in the task walk map, contract/README.md and the jobcmd `const usage` line resolved as the brief says.
- where.go:219 and jobcmd command.go:199, :511 return the `ErrTaskNotFound` sentinel (same text), so the invited guest's `job lint --json` refusal now carries `"code":"no_such_task"` like the uninvited guest's.
- The parity pin for `job lint` json carries the code; new where test pins the sentinel.

## NOT VERIFIED

No UI change; nothing needs a live screen. Not re-run on a34dbf80: row 1's ancestor exit 1 and row 3's :393 failure (taken from the brief and the review).

## Open items

None.

DONE
