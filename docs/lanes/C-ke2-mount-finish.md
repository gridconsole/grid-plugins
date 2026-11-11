# C-ke2-mount-finish: finish the brain write and rename mount left NOT DONE by C-ke-mount

Head: a3cc67b7 (code), plus this report's commit. Base: lane-C-ke-mount aa8ca4b1, which carries lane-C-ke-brain-write and lane-C-jz-inprocess-reasons. Rename was already on main (see docs/lanes/C-ke-mount.md:9-15).

Status: NOT DONE. Decision 1 cannot hold together with an existing test on main: no summary text that `TestExHelpSummariesNameEveryVerb` accepts fits under 120 columns. The working is under "Decision 1" below. Everything else is done.

## Decision 1: arithmetic

- `grid help` prints `"  %-*s  %s"`, where the width is the longest command name, 10 (internal/cli/cli.go:331-336). The prefix `  brain       ` is therefore 14 columns.
- `TestExHelpSummariesNameEveryVerb` (ex_help_summary_verbs_test.go:44-53, on main) needs every brain subcommand in contract/commands.json as a whole word of the summary. Only a plural `s` is allowed, and `-` does not count as a separator.
- The brain verbs are show, search, topics, index, propose, accept, keep, decide, supersede, check, changes, revert, drop, restore, write and rename. Together they are 16 verbs and 93 letters.
- With 15 one-character separators, the shortest summary is 108 columns, so the line is at least 14 + 108 = 122. `TestExHelpLinesFitOneLine` (ex_help_summary_verbs_test.go:99) needs 119 or fewer.
- Shortening the summary drops a verb, which turns `TestExHelpSummariesNameEveryVerb` red. Raising the limit is ruled out by the decision.
- So cli.go:148 and testdata/golden/help.golden:24 are left as C-ke-mount wrote them.

Options for the orchestrator (each needs an owner decision):
- (a) Raise the limit to 123 columns, or exempt brain from it.
- (b) Exempt brain from the every-verb test and shorten the summary to a pointer, for example `show/search/propose/accept/... (grid brain help lists all)`.
- (c) Change the help layout for long summaries, for example a wrapped continuation line.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 Leaf tests pass | HOLDS | `go test -count=1 -v -run 'TestKeWr\|TestKeRn' ./internal/brain/ ./internal/braincmd/` exits 0. PASS: TestKeRnStore, TestKeRnRefuse, TestKeRnHalfWay, TestKeRnRace, TestKeWrStore, TestKeWrRefuse, TestKeWrRace, TestKeRnCommand, TestKeWrCommand. |
| 2 TestKeMt through cli.Run | HOLDS | `go test -count=1 -v -run 'TestKeMt\|TestContractCommandsParserInventory\|TestGuestProjectParity\|TestRemainingTaskOperandHelp\|TestGbDocsVerbsAreImplemented\|TestHsIpWalksEveryCommand\|TestGg' ./internal/cli/` exits 0 (`ok internal/cli 6.736s`). PASS: TestKeMtWriteThroughRun, TestKeMtRenameThroughRun, TestKeMtBrainHelp, TestKeMtGridHelpExits. The fails-on-base lines are in docs/lanes/C-ke-mount.md:28. ke_mount_test.go is unchanged in this lane. |
| 3 The five named walks | HOLDS | Same run. `--- PASS:` for TestContractCommandsParserInventory, TestGuestProjectParity, TestRemainingTaskOperandHelp, TestGbDocsVerbsAreImplemented and TestHsIpWalksEveryCommand. The -v output lists TestGuestProjectParity/brain_write/{project,project-project-id}/{plain,json}/{invited,parity} and the same eight brain_rename sub-tests, all PASS. |
| 4 TestGg walks | HOLDS | Same run. PASS: TestGgUnknownFlagWalk (with brain_write and brain_rename sub-tests), TestGgUnknownProjectWalk, TestGgUnknownTaskWalk and their CoversTheContract tests. No brain row was added to ggProjectKnownOpen. |
| 5a Second `make generate` leaves the tree clean | HOLDS | Run at 9977ad0d and again at a3cc67b7: `make generate` exits 0, then `git status --porcelain` prints nothing. |
| 5b `make golden` | HOLDS | At 9977ad0d: exit 0, `PASS`, `ok internal/golden 0.454s`. After the jobcmd edit, `./internal/golden` was `ok` again in the importers' run. |
| 5c `make lint` | HOLDS | Exit 0 (`go vet ./...`, no findings) at 9977ad0d and at a3cc67b7. |
| 5d `make cross` (macOS and Windows) | HOLDS | At 9977ad0d, unpiped: exit 0, every package `ok` for darwin/arm64 and windows/amd64. At a3cc67b7: vet and test compile ran for both targets with no FAIL or error line (`make cross 2>&1 \| grep -E '^(cross:\|FAIL\|---)\|cannot\|error'`). |
| 5e `go test -count=1 ./internal/cli/ ./internal/braincmd/ ./internal/brain/` | NOT DONE | braincmd and brain are `ok`. internal/cli FAIL, with one test left red: `ex_help_summary_verbs_test.go:100: help line for brain is 122 columns`. This is decision 1, above. |
| Step 6 README write and rename sections, seven refusals, :737 | HOLDS | contract/README.md: the refusal paragraph now lists seven refusals (changes, revert, supersede, drop, restore, write, rename) with hints `write brain entries\|rename brain entries`. ":737" now reads "Exit codes of drop and restore", so "Never 4, 5 or 7" covers only those two. New `### grid brain write ...` and `### grid brain rename ...` sections follow restore, with exit lists from the code: write 0,1,2,3,6,7,9 (internal/braincmd/ke_write.go:53-55); rename 0,1,2,3,6,7,8,9 (internal/braincmd/ke_rename.go:16-19). The rename section has the rename rules (same level relinked, dropped notes skipped, `kept` on a failed relink, old versions stay). The tests that read the README pass: TestIaBrReadme, TestGbDocsVerbsAreImplemented, TestGbDocsSchemaLinksExist, TestHfFs*, TestIl*. |
| Decision 1: brain summary under 120 columns | NOT DONE | Not possible without turning TestExHelpSummariesNameEveryVerb red (arithmetic above). On lane-C-ke-mount and on this head: `ex_help_summary_verbs_test.go:100: help line for brain is 122 columns`. |
| Decision 2: TestExitRefusalHelpGolden pins the new line | HOLDS | c8049113 sets exit_refusal_test.go:184 to the help.golden:52 text, as the decision directs. Red on lane-C-ke-mount: `exit_refusal_test.go:197: help exit status golden: got ["Exit status: ... 5 taken (intent already claimed or done; skip), 6 stale rev (read again), 7 not versioned (nothing written)."], want "Exit status: ... 5 taken (intent already claimed or done; skip)."`. After: `--- PASS: TestExitRefusalHelpGolden`. |
| 3b Job help exit line (found by the full cli run) | HOLDS | `grid job --help` printed its own copy of the old exit line (internal/job/jobcmd/command.go:84). Red on lane-C-ke-mount (run at 9977ad0d): `ik_job_usage_test.go:29: lines starting with Exit status: [... 5 taken (intent already claimed or done; skip).], want exactly "... 6 stale rev (read again), 7 not versioned (nothing written)."` and `record_exit_test.go:284: exit 0: Usage: grid job render ...`. a3cc67b7 adds 6 and 7 to that line; both tests then pass (absent from the FAIL list of the importers' run). No test was edited for this. |
| Rule: importers' tests | HOLDS except internal/cli | `go list -test` importers of internal/brain, internal/braincmd, internal/cli, internal/job/jobcmd, contract and lib: cmd/grid, contract, contract/formats, internal/account, internal/brain, internal/braincmd, internal/cli, internal/contract, internal/golden, internal/hook, internal/job, internal/jobgraph, internal/job/jobcmd, internal/job/jobtest, internal/job/lint, internal/mcp, internal/packaging, internal/people, internal/question, internal/routine, internal/settings, internal/task, internal/workflow, lib. `go test -count=1` over all of them gives `ok` for every package except internal/cli, which fails on TestExHelpLinesFitOneLine only (decision 1). |
| Rule: repo-wide walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: both `--- PASS`, `ok`. `go test -count=1 ./internal/job/lint/`: `ok`. |
| Rule: edit only the brief's packages | DEPARTURE, stated | internal/job/jobcmd/command.go:84 is outside the files the brief names. It was changed because two existing tests on main, TestIkJuJobHelpExitLine and TestRecordExitHelp, compare the job help exit line with `exitStatusHelp`, and step 3 and decision 2 change that constant. The only alternative was to edit those tests, which the rules forbid. The change is one help string; no logic. |
| Rule: no guard, gate, cap or hold added | HOLDS | This lane changes one inline test golden, README text and one help string. propose and accept are untouched. |
| Rule: one-sentence commits, no attribution | HOLDS | `git log aa8ca4b1..HEAD`: c8049113, 9977ad0d, a3cc67b7 and the report commit, each one sentence with no trailer. |
| Rule: existing tests not edited to pass | HOLDS, with the decision-2 exception | The only test edit is exit_refusal_test.go:184, which decision 2 orders. No test was deleted. |
| Rule: RealPlugin and UI rows | N/A | The brief names no RealPlugin test, and this lane has no UI. |
| Runbook-reviewer read | HOLDS | Read and checked against it. Every row above names its command and exit code, or its file:line. The verdict is NOT DONE, so a reviewer will mark the lane STUCK for a re-brief on decision 1. |

## What changed in this lane

- internal/cli/exit_refusal_test.go:184 now holds the new exit-status line (decision 2).
- contract/README.md:
  - the refusal paragraph lists seven refusals;
  - the drop exit table covers drop and restore only;
  - there are new sections for `grid brain write` and `grid brain rename`.
- internal/job/jobcmd/command.go:84: the job help exit line now matches `exitStatusHelp`.
- contract/commands.json: regenerated with no diff.

## Open items

1. Decision 1 needs a new owner decision: (a), (b) or (c) above. Until then `go test ./internal/cli/` stays red on TestExHelpLinesFitOneLine.
2. lib/lib.go:56-58 (package doc) and :126-127 (Run doc) still list exits 0 to 5. No test reads 6 or 7 there, and lib is outside this lane's steps. This is a note for the backlog.
3. README nit: write's exit 2 also covers a `--file` path that cannot be opened (internal/braincmd/ke_write.go:105-113). The write section says "input that is not such a note" and does not name the unopenable path.
4. The README's rename link id `brain:<level>/<name>` was checked against internal/brain/ke_rename.go:241.

NOT VERIFIED on a live screen: nothing; this lane has no UI.

NOT DONE
