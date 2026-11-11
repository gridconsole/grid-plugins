# C-hn-job-refusal-code: proof table

Base: 01bd2ee4 (lane-M-ho-feed-types-scope-onto-main stacked). `git merge-base --is-ancestor 50f05e55 HEAD` exit 0; `grep -n 'json:"code' internal/job/jobcmd/*.go` printed nothing on the base.

| Row | Status | Proof |
| --- | --- | --- |
| 1 TestHnCodeUsage | HOLDS | `go test -count=1 -run TestHnCode -v ./internal/job/jobcmd/` exit 0, `--- PASS: TestHnCodeUsage`. Base (production files stashed): `hn_refusal_code_test.go:91: [render T-01 --json]: code null, want "invalid_argument"; stdout {"ok":false,"findings":[],"error":"render requires <task> <item>"}` |
| 2 TestHnCodeLookup | HOLDS | same run, `--- PASS: TestHnCodeLookup`. Covers show/lint T-99 (no_such_task, "task T-99 not found in project example"), show --project absent as owner (no_such_project) and as guest (no_such_task), render/verify --project absent (no_such_project), trace --project absent (exit traceExitNotFound, no_such_project, rows/problems kept). Base: `hn_refusal_code_test.go:109: [show T-99 J-1 --json]: code null, want "no_such_task"` |
| 3 TestHnCodeUnclassified | HOLDS | same run, `--- PASS: TestHnCodeUnclassified`. verify J-9 --task T-01 → exit 1, no `code` key. Trace() error that is not errTraceProjectNotFound: empty GridHome → "cannot find the Grid home; set GRID_HOME" (trace.go:157-158), exit traceExitFailed, no `code` key. It also passes on base (it pins unchanged behaviour). |
| 4 TestHnCodeWalk | HOLDS | same run, `--- PASS: TestHnCodeWalk`. Base: `lint/render/show/verify/trace: ... stdout lacks a top-level code` (5 lines, hn_refusal_code_test.go:152). Successes checked: render, lint of the rendered brief, show, trace. verify has no success on the fixture without running the Job's command. |
| 5 `go test -count=1 ./internal/job/... ./internal/cli/` | NOT DONE | job/... all ok; the three TestGg* walks pass. `internal/cli` FAILS in `TestGuestProjectParity` on exactly 3 rows: `job_lint/project-task/json/invited`, `project-task-project-id/json/invited` and `project-task-task-path/json/invited` (`guest_parity_test.go:393: invited guest lint --task read a file outside the Grid home`). Cause: `internal/cli/guest_parity_test.go:437` pins the guest by-path lint JSON as `{"ok":false,"findings":[],"error":"task not found"}` with no code and requires invited == unknown. The unknown guest's refusal is `where.ErrTaskNotFound`, which now gives `no_such_task`. The invited guest's refusal is `internal/where/where.go:219` `fmt.Errorf("task not found")`, with no sentinel and so no code. Both files are outside Touches. |
| 6 `make lint`, diff stat | HOLDS | `make lint && GOOS=windows go build ./... && GOOS=darwin go build ./...` exit 0. `git diff 01bd2ee4 --stat`: contract/README.md, the three gg_unknown_*_walk_test.go, jobcmd command.go, trace.go, verify.go, hn_refusal_code_test.go (all in Touches), plus this report docs/lanes/C-hn-job-refusal-code.md (runbook-required). |
| Rule: new tests fail on base | HOLDS | Rows 1, 2 and 4 failing lines above. Row 3 pins unchanged behaviour by design. |
| Rule: no existing test edited to pass | HOLDS | Only the three walk files' job entries changed, as the brief directs. guest_parity_test.go is NOT edited (see row 5). |
| Rule: importers tested | PARTIAL | `go list` importers of jobcmd: `internal/cli` (and jobcmd itself). Both run; cli fails as in row 5. |
| Rule: no guard/gate/cap | HOLDS | No new guard; the change only adds an output field. |
| Rule: no attribution, no personal paths | HOLDS | commit messages are one sentence; no paths or emails added. |

## Walk entry diff (row 5)
- gg_unknown_flag_walk_test.go: all five job entries removed (`ggFlagKnownOpen` is now empty). Each verb now answers exit 2 with one object and `code: invalid_argument`.
- gg_unknown_task_walk_test.go: job lint/render/show/verify keep their entry. They are still open because stderr is empty with --json. The seen text gains `,"code":"no_such_task"`.
- gg_unknown_project_walk_test.go: job render/show/verify/trace keep their entry, still open on stderr parity. The text gains `; stdout code no_such_project`. The job lint entry is unchanged: lint without --task makes no project lookup.

## What changed
- `refusalCode(err, exit)` in internal/job/jobcmd/command.go. ErrProjectUnregistered or errTraceProjectNotFound gives no_such_project; ErrTaskNotFound gives no_such_task; else exit 2 gives invalid_argument; else no code.
- `report.Code` and `TraceResult.Code` (`code,omitempty`) are set by the memberContext (both forms), parse, fail, verify fail and trace fail refusals. Successes, verify results and lint findings are unchanged.
- contract/README.md: one sentence after the signal example names the five verbs.

## Open items
- Split needed for row 5. Either (a) Touches gains `internal/where/where.go:219`, returning `ErrTaskNotFound` (same text), plus `internal/cli/guest_parity_test.go:437`, whose literal gains `,"code":"no_such_task"`; or (b) the owner rules that the guest by-path gate stays unclassified, and `guestFile` drops the sentinel. This is the same class of gap as the three walk files the brief did list.
- Unclassified guest refusals in jobcmd (`errors.New("task not found")` at command.go lint same-task check and guestFile) carry no code; a follow-up may move them to the sentinel together with (a).

## Fix round 1
- Reproduced on head d2be3cc3: `go test -count=1 -run TestGuestProjectParity ./internal/cli/` exit 1. The same 3 rows fail at `guest_parity_test.go:393`, and invited prints `{"ok":false,"findings":[],"error":"task not found"}` with no code.
- No code was changed in this round. Both fixes are outside Touches (where/where.go, cli/guest_parity_test.go) or need an owner ruling (option b), so the brief must be amended first.
- Recommended amendment, option (a): Touches gains `internal/where/where.go` and `internal/cli/guest_parity_test.go`, the :437 JSON literal only.
  - where.go:219 returns `ErrTaskNotFound`. The text stays the same: `internal/where/errors.go:6` is `errors.New("task not found")`.
  - command.go:199 and :495 return `where.ErrTaskNotFound` too (already in Touches).
  - The :437 literal gains `,"code":"no_such_task"`.
  - Then every guest by-path refusal is one classified not-found, as contract/README.md asks.
- Option (b) would fit inside command.go but needs an owner ruling. All guest by-path refusals would get an unclassified error, so both outputs would match the code-free :437 literal. It hides a classified not-found and goes against refusalCode as the brief defines it.

## NOT VERIFIED
Nothing needs a live screen. RealPlugin tests were not part of this brief.

NOT DONE
