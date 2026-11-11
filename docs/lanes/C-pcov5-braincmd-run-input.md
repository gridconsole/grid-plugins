# C-pcov5-braincmd-run-input

Test-only lane: one new file, `internal/braincmd/run_input_test.go` (14 tests, helpers `riWant` and `riProposeArgs`, test names `TestRI...`). No code under test changed. This proof table is the only other file, as the runbook asks.

## Proof table

| Row | Result |
|---|---|
| 1 Coverage before | HOLDS. `go test -count=1 -coverprofile=<file> ./internal/braincmd/` exit 0, `coverage: 88.6% of statements`. Profile kept at tmp/hand/ri-cov-before.out; every block of rows 2-12 has count 0 there except command.go:77 and :483, which were already 1. |
| 2 linkFlags.Set :75-76 | HOLDS. `TestRILinkFlagRejectsNonJSON` run_input_test.go:30: exit 2, stderr contains `invalid value "nope" for flag -link: link must be a JSON object`, Grid home has no entries. |
| 3 linkFlags.Set :78-79 | HOLDS. `TestRILinkFlagRejectsUnknownField`: exit 2, stderr contains `json: unknown field "bogus"`. |
| 4 Help :145-146 | HOLDS. `TestRIProposeHelpPrintsUsageOnStdout`: `propose -h` exit 0, stderr empty, stdout equals `usage`. |
| 5 Level and task :212-213 | HOLDS. `TestRILevelHomeWithTaskIsAUsageError`: exit 2, stderr exactly `brain show: --level home cannot be combined with --task. Run: grid brain help\n`. |
| 6 No Grid home :250-251 | HOLDS. `TestRINoGridHomeFails`: exit 1, stderr exactly `brain: cannot find the Grid home; set GRID_HOME\n`. |
| 7 Project level, no project :283-284 | HOLDS. `TestRIProjectLevelWithoutProjectFails`: exit 1, stderr exactly `brain: project level needs a project; select --project or use --level home\n`. |
| 8 proposalID :481-482, :484-485 | HOLDS. `TestRIInvalidBrainID` (`show brain:bad/x`, exit 1, `brain: invalid brain id\n`) and `TestRIBrainIDLevelDisagreesWithLevel` (`show brain:project/x --level home`, exit 1, `brain: id level disagrees with --level\n`). |
| 9 propose from file :340-345, :356 | HOLDS. `TestRIProposeFileWithoutNameAndTopic`: exit 1, stderr `brain: name and topic must be plain names\n`; with `--json`, `{"error":"name and topic must be plain names"}\n`. `TestRIProposeFileMissing`: exit 1, stderr starts `brain: open <path>`. |
| 10 readInput :468-469, :472-473 | HOLDS. `TestRIStdinOverOneMiBFails`: 1 MiB + 2 bytes on stdin, no `--body`, exit 1, `{"error":"input exceeds 1 MiB"}\n` with `--json`, `brain: input exceeds 1 MiB\n` without. `TestRINilStdinProposesAnEmptyBody`: Stdin nil, exit 0, stdout contains `"body":""`. |
| 11 interspersed :498-499 | HOLDS. `TestRIDoubleDashKeepsLeadingDashWords`: `search --level home -- -words` exit 0, stdout exactly `hits: []\nquery: -words\n`. |
| 12 Closed stdout :423-424, :432-433, :538-539 | HOLDS. `TestRIClosedStdoutFailsSearchAndIndex` with the package's `failingWriter`: `search x --level home --json` exit 1, `{"error":"stdout closed"}\n`; plain search and plain `index --level home` exit 1, `brain: stdout closed\n`. |
| 13 After and gates | HOLDS. Coverage after 92.8% (before 88.6%). Count-1 lines in the new profile (`grep -E "command.go:(75\|14[56]\|212\|250\|283\|310\|34[0-5]\|356\|423\|432\|468\|472\|481\|484\|498\|538)\."`): 75.3,76.1 1 1; 78.3,79.1 1 1; 145.4,146.10 2 1; 212.3,213.1 1 1; 250.3,251.1 1 1; 283.3,284.1 1 1; 310.6,310.11 1 1; 340.5,342.19 3 1; 343.6,345.1 2 1; 356.4,356.9 1 1; 423.4,424.1 1 1; 432.3,433.1 1 1; 468.3,469.1 1 1; 472.3,473.1 1 1; 481.4,482.1 1 1; 484.4,485.1 1 1; 498.4,499.9 2 1; 538.6,539.1 1 1. `go test -count=1 ./internal/braincmd/` exit 0; `make lint` (go vet ./...) exit 0; `GOOS=windows go vet ./...` exit 0. |

## Rules

| Rule | Result |
|---|---|
| New tests fail on base, pass on lane | NOT APPLICABLE to the letter. The lane changes no code, so the tests pass on base code too; they are new coverage. On base the blocks have count 0 (before profile), which is the "fail" the brief measures. |
| No existing test edited or deleted | HOLDS. `git diff --stat origin/main..HEAD` shows the new test file and this doc only. |
| No guard, gate, cap or hold added | HOLDS. Test file only. |
| Packages changed and importers | HOLDS. Changed: `internal/braincmd`. Importers from `go list`: `internal/cli` (and through it `cmd/grid`, `lib`). `go test -count=1 ./internal/braincmd/ ./internal/cli/ ./cmd/grid/ ./lib/` all exit 0 (cmd/grid has no test files). |
| Windows and macOS builds | HOLDS. `GOOS=windows go vet ./...` exit 0; `GOOS=darwin go vet ./...` exit 0. |
| Nothing Unix-only, OS error words not asserted | HOLDS. No chmod, Mkfifo or syscall; the missing-file row asserts the prefix `brain: open <path>` only. |
| Helper prefix `ri`, failingWriter reused | HOLDS. `riWant`, `riProposeArgs`, tests `TestRI*`; no second failing writer. |
| No approval.json, no Approve/TryApprove | HOLDS. Not touched. |
| gofmt | HOLDS. `gofmt -l internal/braincmd` prints nothing. |

## What changed

- `internal/braincmd/run_input_test.go` (new): 14 tests, one per row part.
- `docs/lanes/C-pcov5-braincmd-run-input.md` (this file).

## NOT VERIFIED on a live screen

Nothing here is a UI row. The tests drive `Run` in process with the package harness; no real terminal or closed pipe was used.

## Open items

None.

DONE
