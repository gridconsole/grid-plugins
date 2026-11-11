# C-ke-mount: wire `grid brain write` (and `rename`) into the plugin's command list, help, contract and walks

Head: 14129b03 (code), plus this report's commit. Base: origin/main a7809a58, with lane-C-jz-inprocess-reasons 32caf89d merged (stacked start) and lane-C-ke-brain-write 66f9c41a merged.

Status: NOT DONE. The build stopped at the 60-call budget, and two existing tests conflict with the brief's step 3 (see open items).

## Base facts that differ from the brief

- `brain rename` is already on main a7809a58, outside the README. The lane-C-ke-brain-rename leaf (e6f63da5) is an ancestor of main, so `git diff origin/main...origin/lane-C-ke-brain-rename` is empty. Main already has:
  - the rename usage and dispatch: internal/braincmd/command.go:65-66, :144-146;
  - rename in the cli.go:148 summary;
  - the inventory row: contract_inventory_test.go:522;
  - the guest argv `{"parity/parity", "parity-new", ...}` and the want case: guest_parity_test.go:37, :487-491.

  Main's rename rows are left as they are (`parity-new`, not the brief's `parity2`). Rename-only assertions therefore pass on base.
- Write's exit list follows runWrite (internal/braincmd/ke_write.go:53-55): 0, 1, 2, 3, 6, 7, 9 and never 8. The brief listed 0, 1, 2, 3, 6, 7.
- The stacked merge conflicted in lib/lib.go ("What a call reads"). The resolution keeps:
  - main's user-name bullet, still true at internal/task/create.go:207-210;
  - jz's provider-child bullet, still true at internal/cli/account.go:30 and internal/account/account.go:68.

  It drops jz's "migrate reads CLAUDE_PLUGIN_DATA from the host" clause, because on the merged base migrate reads it from the call (internal/cli/migrate.go:69) and lib.go:19-21 already says so.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 Leaf tests pass after the merge | HOLDS | `go test -count=1 -v -run 'TestKeWr\|TestKeRn' ./internal/brain/ ./internal/braincmd/` exits 0. PASS: TestKeRnStore, TestKeRnRefuse, TestKeRnHalfWay, TestKeRnRace, TestKeWrStore, TestKeWrRefuse, TestKeWrRace, TestKeRnCommand, TestKeWrCommand. |
| 2 TestKeMt through cli.Run | HOLDS | `go test -count=1 -v -run 'TestKeMt\|TestGg' ./internal/cli/` exits 0. PASS: TestKeMtWriteThroughRun (write new exits 0, log.md has person:bo; replace at its rev exits 0; a stale rev exits 6), TestKeMtRenameThroughRun (rename exits 0 and z.md links brain:home/y, not x), TestKeMtBrainHelp, TestKeMtGridHelpExits. Fails on base (run before the production edits): `ke_mount_test.go:76: brain write new: 2 "" "brain: unknown command \"write\". Run: grid brain help\n"`; `ke_mount_test.go:97: brain write x: 2 ... unknown command "write"`; `ke_mount_test.go:127: brain help lacks "\|drop\|restore\|write\|rename\|"` plus the write usage and exit lines; `ke_mount_test.go:140: grid help lacks ".../restore/write/rename"`, `"6 stale rev (read again)"`, `"7 not versioned (nothing written)"`. The rename usage assertion alone passes on base (main mounts rename). |
| 3 The five named walks | HOLDS (five), see the two reds below | `go test -count=1 -v -run '...\|TestGolden\|Help' ./internal/cli/` printed PASS for TestContractCommandsParserInventory, TestGuestProjectParity (with the TestGuestProjectParity/brain_write/... and brain_rename/... sub-tests), TestGbDocsVerbsAreImplemented and TestHelpListsEveryCommand. TestRemainingTaskOperandHelp and TestHsIpWalksEveryCommand fell outside the truncated output and were NOT VERIFIED separately. |
| 3b Other internal/cli tests | NOT DONE | Two existing tests are red on the lane. `TestExHelpLinesFitOneLine`: `ex_help_summary_verbs_test.go:100: help line for brain is 122 columns` (the limit is under 120, :99), so adding `write/` to cli.go:148 does not fit. `TestExitRefusalHelpGolden`: `exit_refusal_test.go:197`, whose inline `want` at :184 pins the old Exit status line, so step 3's change to exitcode.go:10 breaks it. Either fix edits an existing test or departs from step 3; owner decision needed. |
| 4 TestGg walks | HOLDS | Same run as row 2, exit 0. PASS: TestGgUnknownFlagWalk, TestGgUnknownProjectWalk, TestGgUnknownTaskWalk and their CoversTheContract tests. No brain row was added to ggProjectKnownOpen. |
| 5 generate, golden, lint, cross, package tests | NOT DONE | `make generate` ran once and its commands.json is committed; the second-run porcelain check, `make golden`, `make lint` and `make cross` were not run (budget). The `./internal/cli/` package test is red per row 3b. |
| Step 6 README write and rename sections, seven refusals | NOT DONE | Not written (budget). The README also needs :737 ("the table of every brain write verb … Never 4, 5 or 7") scoped to drop and restore. |
| Rule: importers' tests (go list) | NOT DONE | Not run (budget). |
| Rule: repo-wide walk tests | PARTIAL | TestHsIpWalksEveryCommand was not seen separately. `go test ./internal/job/lint/` was not run. |
| Rule: no guard, gate, cap or hold added | HOLDS | Only dispatch, help text, exit text, contract rows and goldens were changed. propose and accept are untouched. |
| Rule: no attribution, one-sentence commits | HOLDS | `git log origin/main..HEAD`. |
| Runbook-reviewer read | NOT DONE | /srv/gridwork/tmp/hand/runbook-reviewer.txt was not read (budget). |

## What changed

- internal/braincmd/command.go:
  - `write` in the verb list, plus a write usage line and exit line;
  - dispatch to runWrite beside restore and rename;
  - Run's doc comment names 6, 7, 8 and 9 per verb.
- internal/cli/cli.go:148 adds `write/`.
- internal/cli/exitcode.go:10 adds `6 stale rev (read again), 7 not versioned (nothing written)`.
- testdata/golden/brain-help.golden and testdata/golden/help.golden were edited by hand.
- contract/commands.json was regenerated by `make generate`.
- The `brain write` row was added to contract_inventory_test.go, and the guest argv and want case (exit 3, `grid brain write: caller role is not allowed to write brain entries`, person_required JSON) to guest_parity_test.go.
- internal/cli/ke_mount_test.go is new.
- lib/lib.go has the merge resolution described above.

## Open items

1. Owner decision on the two red tests (row 3b). For the 122-column summary line, either shorten the brain summary or raise the limit. For exit status, either update the inline golden at exit_refusal_test.go:184, or print 6/7 on a separate help line and leave exitStatusHelp as it was.
2. README sections for write and rename, the seven-refusal list, and scoping :737.
3. Remaining proofs: the second `make generate`, `make golden`, `make lint`, `make cross`, the importers' tests, and the plugin walk tests.

NOT VERIFIED on a live screen: nothing; this lane has no UI.

NOT DONE
