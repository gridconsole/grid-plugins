# Q-brain-revert-reason: `grid brain revert --reason <why>`

Stacked on lane-Q-brain-supersede-verb (merged into this branch, base 78a3d3fc). Local main is 3ccec5cb.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. Test fails first | HOLDS | Before any code change, on base 78a3d3fc: `go test -count=1 ./internal/braincmd/ -run 'RevertReason\|RevertWithoutReason'` gave `--- FAIL: TestRevertReasonIsLoggedAndListed` / `revert_reason_test.go:16: revert --reason: exit 2, stderr "brain: unknown flag --reason\n"`; also JSONNamesIt, WithAwkwardCharacters, OnNoopWritesNoLine failed with `exit 2 brain: unknown flag --reason`. `go test ./internal/brain/ -run Revert` failed to build: `s.RevertWith undefined`, `undefined: RevertOptions`. The flag-error text is the repo's own (`unknown flag`), exit 2 as the brief says. |
| 2. Exit 0, newest change `reverted`, reason `revert to v1: v2 claim was wrong` | HOLDS | `TestRevertReasonIsLoggedAndListed` (internal/braincmd/revert_reason_test.go:11), passes; `go test -count=1 ./internal/braincmd/` exit 0. Brain level: `TestRevertWithReasonLogsIt`. |
| 3. No `--reason` gives exactly `revert to v1` | HOLDS | `TestRevertWithoutReasonKeepsTheFixedText` (braincmd) and `TestRevertWithoutReasonKeepsTheText` (brain, also a blank reason). `revert_test.go` and `changes_test.go` unchanged and pass. |
| 4. `\|`, newline, leading `-` stored as one line, read back by `parseAudit` as one Change | HOLDS | `TestRevertReasonIsOneCleanAuditLine` (internal/brain/revert_reason_test.go:44): reason `-x \| y\r\n\tz  w` gives `revert to v1: -x / y z w`, the log line has 7 ` \| ` separators, `parseAudit(last) == Change`. braincmd: `TestRevertReasonWithAwkwardCharacters`. `auditLine` and the format are untouched (git diff shows no change in brain/changes.go). |
| 5. No-op writes no line | HOLDS | `TestRevertWithReasonOnNoopWritesNothing` (snapshot of the whole Grid home equal before and after); `TestRevertReasonOnNoopWritesNoLine` (still 3 change lines). The no-op branch in revert.go is untouched. |
| 6. `--json` gains `reason`; text line names it | HOLDS | `TestRevertReasonJSONNamesIt`: `{"version":4,"noop":false,"reason":"stale"}`, key absent without `--reason`. Text: `v4 (reason: v2 claim was wrong)`. On a no-op the text stays `already at v3` (nothing was written, so no reason is shown); JSON still echoes the reason given. |
| 7. Checks | HOLDS | `go test -count=1 ./internal/brain/ ./internal/braincmd/` ok; `go test -count=1 ./internal/cli -run '^TestContractCommandsParserInventory$'` ok; `go vet ./internal/brain/ ./internal/braincmd/` exit 0; `gofmt -l internal` empty. |
| Importers tested | HOLDS | Importers of brain/braincmd (go list): cli, hook, job, job/jobcmd, job/jobtest, people, routine, task, workflow. `go test -count=1` on cli, hook, job/..., people, routine, task, workflow: all ok. |
| Lint, Windows and macOS builds | HOLDS | `make lint` (go vet ./... and gofmt) exit 0; `make cross` (darwin/arm64 and windows/amd64 vet and test compile) no FAIL, ran to the end. |
| New tests fail on base, pass on lane | HOLDS | See row 1; they pass now. |
| No existing test edited or deleted | HOLDS | git diff touches no `_test.go` file already on base. |
| No guard, gate, cap, hold or hook | HOLDS | Only an optional free-text field; no limit on its length or content. |
| Contract and golden regenerated | HOLDS | `make generate` added `--reason` (value, not required) to contract/commands.json; the help golden `testdata/golden/brain-help.golden:14` was edited by hand because UPDATE_GOLDEN=1 before `go test` is outside the allow list; `go test ./internal/cli` passes. |
| No personal paths, hosts or emails | HOLDS | None added. |

## What changed

- `internal/brain/revert.go`: `RevertOptions{Reason}` and `Store.RevertWith`; `RevertTo` (6 args) and `Revert` delegate with empty options, so every existing caller is source-compatible. The logged reason is `revert to vN: <why>` (whitespace folded) or `revert to vN` when blank.
- `internal/braincmd/changes.go`: `--reason` flag, usage line, JSON `reason` (omitempty), text suffix ` (reason: ...)` when written.
- `internal/braincmd/command.go`, `testdata/golden/brain-help.golden`, `contract/commands.json`, `contract/README.md`: help text and contract.
- New: `internal/brain/revert_reason_test.go`, `internal/braincmd/revert_reason_test.go`.
- Not touched: `skills/brain-keeper/SKILL.md` mentions revert; the stacked lane Q-skill-brain-keeper-prune owns it.

## NOT VERIFIED on a live screen

Nothing here is UI; no live check needed. The reason is a trusted-enough free string stored only after `auditLine` cleaning; the text output prints the whitespace-folded reason raw (a control character in `--reason` could reach the terminal in the revert's own output, as for any argument echoed back; `grid brain changes` still maps controls to `?`).

DONE
