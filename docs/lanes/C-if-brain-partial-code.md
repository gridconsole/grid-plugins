# C-if-brain-partial-code: brain revert and supersede partial success carry `index_rebuild_failed`

Repo: plugin. Branch lane-C-if-brain-partial-code. Base: origin/main ee8a96c1, which already contains
lane-C-hf-cli-failure-shapes 161396e7 (`git merge-base --is-ancestor 161396e7 origin/main` exit 0).

## Proof table

| # | Row | Status | Proof |
|---|-----|--------|-------|
| 1 | TestIfBeRevertPartialCode: exit 1, stdout version 4 + code, stderr exactly {error, code} | HOLDS | `go test ./internal/braincmd -run TestIfBe -count=1 -v` exit 0: `--- PASS: TestIfBeRevertPartialCode`. On base (test file only, before the change): `if_partial_code_test.go:58: stdout code "", want "index_rebuild_failed": {"version":4,"noop":false,"error":"reverted to v1 as v4, but rebuilding the index failed: renameat knowledge/.brain-….tmp knowledge/index.md: file exists"}` / `--- FAIL: TestIfBeRevertPartialCode` |
| 2 | TestIfBeSupersedePartialCode: exit 1, stdout is the SupersedeResult (wrote true, no extra keys), stderr has the code and the unchanged message | HOLDS | Same run: `--- PASS: TestIfBeSupersedePartialCode`. On base: `if_partial_code_test.go:101: stderr keys [error], want [code error]: {"error":"brain:home/c1 is superseded by brain:home/c2, but rebuilding the index failed: renameat … file exists"}` / `--- FAIL: TestIfBeSupersedePartialCode` |
| 3 | Scratch: sentinel `%w`→`%v` in revert.go:180 makes TestIfBeRevertPartialCode fail | HOLDS | Scratch edit `"reverted to v%d as v%d, but %v: %w"`: `if_partial_code_test.go:58: stdout code "", want "index_rebuild_failed": {"version":4,…,"error":"reverted to v1 as v4, but rebuilding the index failed: …"}` / `--- FAIL: TestIfBeRevertPartialCode` / `--- PASS: TestIfBeSupersedePartialCode`. Message identical, only the sentinel identity is lost. Edit reverted; `git status --short` empty before the commits |
| 4 | `go test ./internal/cli -run 'TestHfFs\|TestContractCommandsParserInventory\|TestDtGateWordsHelpGolden' -count=1` exit 0; scratch delete of the README row fails TestHfFsCodeTable | HOLDS | `--- PASS: TestContractCommandsParserInventory`, `TestDtGateWordsHelpGolden`, `TestHfFsCodeTable`, `TestHfFsShapes`, `TestHfFsSilent`, `TestHfFsKnownOpen`, `ok internal/cli`. Scratch (row deleted at 579904a9): `hf_failure_shapes_test.go:203: code "index_rebuild_failed" (changes.go:codeIndexRebuildFailed) is missing from the README code table` / `--- FAIL: TestHfFsCodeTable`; `git checkout -- contract/README.md`, `git status --short` empty, rerun `ok` |
| 5 | `go test ./internal/braincmd ./internal/brain -count=1` exit 0, existing revert index tests unchanged | HOLDS | `ok internal/braincmd`, `ok internal/brain`. changes_test.go and verbs_level_test.go untouched (`git diff origin/main --stat` lists neither) |
| 6a | `make generate` leaves `git status --porcelain` empty | HOLDS | `make generate` ok, then `git status --porcelain` printed nothing. contract/commands.json, help goldens unchanged |
| 6b | `make lint` exit 0 | HOLDS | `make -s lint cross && echo LINT_CROSS_EXIT_0` (see Checks) |
| 6c | `git diff <base> --stat` names only the Touches files | HOLDS, with two files the brief requires outside the Touches line | `git diff origin/main --stat`: contract/README.md, internal/brain/revert.go, internal/brain/supersede_verb.go, internal/braincmd/changes.go, internal/braincmd/if_partial_code_test.go, internal/braincmd/supersede.go, internal/cli/hf_failure_shapes_test.go (7 files). The two internal/brain files are not on the brief's Touches line, but "What to build" requires them ("wrap the rebuild failure in internal/brain with a sentinel … via %w at revert.go:180 and supersede_verb.go:145") and row 3 edits revert.go. `git diff origin/lane-C-hf-cli-failure-shapes --stat` also lists other lanes' files, because origin/main has merged other lanes on top of 161396e7; none of those are from this lane |
| R | New tests fail on base, pass on lane | HOLDS | Rows 1 and 2 |
| R | No existing test edited to pass, none deleted | HOLDS | Only edit to an existing test file is the scan widening in hf_failure_shapes_test.go:99-118 (`pkg == "braincmd"`), which the brief allows "only if the scan must widen". It makes the check stricter, not looser |
| R | No guard, gate, cap, hold or hook added | HOLDS | Diff adds a sentinel, a code constant and an output field only |
| R | Tests of changed packages and their importers | HOLDS | Importers (`go list -f '{{.ImportPath}}{{range .Imports}} {{.}}{{end}}' ./... \| grep -E ' …/internal/(brain\|braincmd\|cli)( \|$)'`): cmd/grid, internal/braincmd, internal/cli, internal/hook, internal/job, internal/job/jobcmd, internal/job/jobtest, internal/people, internal/routine, internal/task, internal/workflow, lib. `go test -count=1` over those plus internal/brain: all `ok` (cmd/grid and job/jobtest have no test files) |
| R | Windows and macOS builds | HOLDS | `make cross` (darwin/arm64, windows/amd64 vet + test compile); see Checks |
| R | Small one-sentence commits, no attribution, no personal paths | HOLDS | `git log origin/main..HEAD` |
| R | No UI change | n/a | |

## Checks

- `make -s lint cross && echo LINT_CROSS_EXIT_0`, unpiped: exit code 0, printed `LINT_CROSS_EXIT_0`. This is go vet,
  gofmt, and vet plus test compile for darwin/arm64 and windows/amd64.
- `go test -count=1` over brain, braincmd, cli and their direct importers: all `ok` (row R above).

## Choice: where the code constant lives

`codeIndexRebuildFailed` is a const in internal/braincmd/changes.go, and TestHfFsCodeTable's scan is widened
to package braincmd (one clause). Why: the constant the scan reads is the one the code prints, so there is
only one copy. A const in cli/error_codes.go would be a duplicate literal used by no cli code, and the test
tying the two copies would need its own file. The README sentence on where codes live now names
internal/braincmd/changes.go.

## What changed

- internal/brain/revert.go: `var ErrIndexRebuild = errors.New("rebuilding the index failed")`. The revert
  rebuild error is now `"reverted to v%d as v%d, but %w: %w"` with the sentinel and the cause. The text is
  byte-identical.
- internal/brain/supersede_verb.go:145: the same wrap. The text is byte-identical.
- internal/braincmd/changes.go: `codeIndexRebuildFailed`, `partialCode` (`errors.Is(err, brain.ErrIndexRebuild)`)
  and `partialFailure` (writes the stderr object `{error, code}`). Revert's --json stdout object gains
  `code,omitempty`. Its stderr uses partialFailure.
- internal/braincmd/supersede.go: stderr uses partialFailure. The stdout SupersedeResult is unchanged.
- Exit stays 1. Human output is unchanged (asserted in both new tests). failWith's uncoded fallback (changes.go:31)
  is untouched.
- contract/README.md: a sentence on partial success (result on stdout, `{"error","code"}` on stderr, exit 1),
  the `index_rebuild_failed` table row, and the location sentence.
- internal/cli/hf_failure_shapes_test.go: the scan includes package braincmd.
- New internal/braincmd/if_partial_code_test.go (helpers `ifBeObject`, `ifBeCode`; reuses `vlBreakIndex`, `svNotes`,
  `threeVersions`).

## Open items

- failWith's uncoded `{"error": …}` fallback (changes.go:31) is unchanged. This includes the --json path where
  the stdout write fails after a partial success, which TestRevertKeepsTheRebuildErrorWhenStdoutFailsJSON and
  TestVLClosedStdoutOnSupersede pin. That is the owner question in the INDEX.

## NOT VERIFIED on a live screen

Nothing; no UI.

DONE
