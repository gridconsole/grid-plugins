# L-brain-task-decide-tests: unit tests for DecideTask

Repo: plugin, lane branch lane-L-brain-task-decide-tests, from main. One new file: `internal/brain/task_decide_test.go` (19 test functions; the pre-existing `TestDecideTaskAcceptNamesThePublishedEntry` in versions_test.go is untouched). No change to `task_decide.go`.

## Proof table

All commands run from the repo root on this lane's head. Exit 0 unless noted.

| # | Row or rule | Result | Proof |
|---|---|---|---|
| 1 | Refusals: "a/b", "approve", 301-character note, note with a newline each error with their message, no receipt, no note | HOLDS | `go test -count=1 -race -run TestDecideTaskRefusesBadArguments ./internal/brain/` exit 0. Ten subtests (also empty proposal and action, 301 multibyte characters, carriage return, unknown target level, target not a plain name). Each asserts the message, that the tree under GridHome and the task folder is byte for byte unchanged (so no receipt, no note, no lock file), and that history was not called. `TestDecideTaskTakesA300CharacterNote` pins the boundary: 300 multibyte characters pass, so the limit counts characters, not bytes. |
| 2 | Accept: note lands under knowledge/<topic>/ at the target level; receipt has action accept and Done true; history ran once with a line naming the proposal; a repeat returns the same decision and does not call history | HOLDS | `TestDecideTaskAcceptFilesTheNoteAndRepeatsAsTheSameDecision` (decision struct, filed note parsed, receipt Action accept, Done and Noted true, history lines `["brain decided: draft accept brain:project/final; Worth keeping."]`, repeat equal and history count still 1, note and receipt bytes unchanged, version chain still 1). Also `TestDecideTaskReceiptKeepsItsFormat` (the eleven receipt keys), `TestDecideTaskRejectPublishesNothing`, `TestDecideTaskDefaultsTheTargetLevel` (project by default, home when the Store has no project, and the disagreement when a project proposal meets a Store with no project). |
| 3 | Conflicts: reject after accept errors "different decision"; accept onto an existing name errors "use merge-into"; merge-into a missing target errors "does not exist"; target at the other level errors "disagrees with destination" | HOLDS | `TestDecideTaskRefusesADifferentSecondDecision` (another action, target, note, no note and another task id; the first receipt and the note stay byte for byte), `TestDecideTaskRefusesAcceptOntoAnExistingNote`, `TestDecideTaskRefusesMergeIntoAMissingTarget`, `TestDecideTaskRefusesATargetAtTheOtherLevel` (accept, merge-into, reject). The three refusals before a receipt each assert no receipt, the proposal still `proposed` and history not called. All pass in the `-race` run below. |
| 4 | Merge-into updates the target; an edit between the receipt write and publication errors "destination changed since decision" | HOLDS | `TestDecideTaskMergeIntoUpdatesTheTarget` (target body first then proposal body, Source joined with "; ", links appended, receipt `Before` equals the old bytes, version chain 2). `TestDecideTaskRefusesToOverwriteADestinationEditedAfterTheDecision` rewrites the real receipt with Done false and edits the file; the error is returned, the edit is kept, the receipt stays not done, history is not called. `TestDecideTaskFinishesADecisionStoppedBeforePublication` covers the other state (file still the receipt's `Before`): the retry publishes and finishes. |
| 5 | History retry: a history func that fails once leaves Noted false; the next call runs history again, sets Noted and Done, note written once | HOLDS | `TestDecideTaskRetriesAFailedHistoryWithoutMergingAgain` (merge-into: receipt Noted false and Done false after the failure; retry returns the receipt's decision, history count 2 with the same line, Noted and Done true, note bytes equal before and after, the proposal text appears once, chain stays 2, one log line, a third call adds no history). `TestDecideTaskWritesNotedBeforeDone` blocks the index rebuild: receipt is Noted true and Done false, the retry sets Done without a second history line. `TestDecideTaskRunsHistoryAfterTheLevelLock`: inside the callback the note is published, the receipt is written and not noted, and another writer can take the level lock. |
| 6 | Two goroutines deciding the same proposal at once; one decision lands; no error but "different decision" when actions differ; -race | HOLDS | `TestDecideTaskSameDecisionAtOnce` (8 goroutines, same action: all succeed with the same decision, history once, one source line, chain 1). `TestDecideTaskDifferentActionsAtOnceLandOneDecision` (12 rounds of 6 goroutines mixing accept and reject: every success has the receipt's action, every failure contains "different decision", history once, accept leaves the note and chain 1, reject leaves no note). `go test -count=20 -race -run 'TestDecideTask(SameDecisionAtOnce\|DifferentActionsAtOnce\|RunsHistoryAfterTheLevelLock)' ./internal/brain/` exit 0 (ok 8.1s). |
| 7 | `go test -count=1 -race ./internal/brain/`: 0 fail | HOLDS | exit 0, `ok internal/brain 40.7s` in the combined importer run, and again `ok 11.6s` alone after the temporary mutations were reverted. |
| 8 | Packages that import brain, with -race | HOLDS | `go list -f '{{.ImportPath}}\|{{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` lists: contract/formats, internal/braincmd, internal/cli, internal/hook, internal/job, internal/job/jobcmd, internal/job/jobtest, internal/people, internal/routine, internal/task, internal/workflow. `go test -count=1 -race` on those plus brain: all ok, exit 0 (job/frontmatter and job/lint come with `./internal/job/...`; jobtest has no tests). |
| 9 | Lint | HOLDS | `make lint` exit 0 (go vet ./... and gofmt -l . clean). |
| 10 | Windows and macOS builds | HOLDS | `make cross` exit 0: vet and test compile of every package for darwin/arm64 and windows/amd64. |
| 11 | New tests fail on base and pass on the lane | HOLDS, see note | This lane changes no code, so the tests pass on base code by design; on base there is no test file, so nothing can fail there. To show they bite, `task_decide.go` was changed five ways without committing, then restored with `git checkout` (`git status --short` empty after). Failing lines under "Mutation evidence". |
| 12 | Edit only the named file; no change to task_decide.go; no existing test edited or deleted | HOLDS | Commit 11aff5ec adds `internal/brain/task_decide_test.go` only; this report is the second file in the lane. `git status --short` empty after the mutation run. braincmd/changes_test.go and cli/brain_test.go were read, not edited. |
| 13 | A bug found is NOT DONE with file:line | HOLDS (none found) | Every assertion written passed against `task_decide.go` as it reads today, so none was left out. |
| 14 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | Only a test file was added. |
| 15 | Small commits, one-sentence messages, no attribution lines, public repo clean | HOLDS | One test commit and one report commit, one sentence each, no Co-Authored-By or session line. Tests use `t.TempDir()`; no personal path, host or email in the file. |

## Mutation evidence (temporary, not committed)

`go test -count=1 -run DecideTask ./internal/brain/` with these changes to `task_decide.go`:

- Line 98: drop `w.Action != action ||` (a different action no longer conflicts):
  `task_decide_test.go:437: another action: error <nil>` and `TestDecideTaskDifferentActionsAtOnceLandOneDecision: round 0: reject call returned <nil>, the decision is accept`.
- Line 215: `if false {` (the edit check removed):
  `TestDecideTaskRefusesToOverwriteADestinationEditedAfterTheDecision: task_decide_test.go:564: error <nil>, want one containing "destination changed since decision; refusing to overwrite it"`.
- Line 244: drop `!pending.Noted` (history called even when Noted):
  `TestDecideTaskWritesNotedBeforeDone: task_decide_test.go:733: history lines ["brain decided: draft accept brain:project/draft" "brain decided: draft accept brain:project/draft"]`.
- Line 52: `size(note) > 301`:
  `TestDecideTaskRefusesBadArguments/note_is_301_characters` and `/note_is_301_multibyte_characters` fail with `error note id must be a plain name ..., want one containing "note must be at most 300 characters on one line"`.
- Line 113: `if false {` (level check removed):
  `TestDecideTaskRefusesATargetAtTheOtherLevel/accept: error <nil>, want one containing "proposal level disagrees with destination"` (also /merge-into and /reject), and `TestDecideTaskDefaultsTheTargetLevel/project_proposal_on_a_store_without_a_project`.

## What changed

`internal/brain/task_decide_test.go`: a `tdHistory` callback recorder (counts lines, can fail), a fixture that builds the task folder with `brain-proposals/<name>.md` through the existing `taskWithProposal` helper, receipt read and write helpers, and a directory-tree snapshot helper. 19 test functions, listed in the rows above.

## Notes for the backlog (not bugs)

- `splitID("")` refuses an empty target ("note id must be a plain name"), so the default level at `task_decide.go:59-64` is reached only with a bare name such as `final`, never with an empty target. The tests use bare names.
- When history fails, `DecideTask` returns the non-empty decision together with the error (`:270`); the tests do not pin that.
- `conventions` is a project-only topic; the home-level test uses `decisions`.

## NOT VERIFIED on a live screen

Nothing here touches a UI or a served read; no live check applies.
