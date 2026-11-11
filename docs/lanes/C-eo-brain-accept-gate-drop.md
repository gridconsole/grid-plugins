# C-eo-brain-accept-gate-drop

`grid brain accept` and `grid brain decide` no longer refuse on a proposal-check finding. The keeper gate `gate()` and the `grid brain check` read stay.

Base: `bb3c1340` (lane branch from main). Read on that base before editing: `internal/brain/accept_gate.go` (present, 43 lines), `store.go:405-450`, `task_decide.go:120-140`, `braincmd/command.go:400-415` and `:550-575`, `braincmd/decide.go:55-70`. Every cited line matched.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. New test fails on base | HOLDS | Only the new file written, `go test -count=1 -v ./internal/brain/ -run TestDxNoGate` on `bb3c1340` code, exit 1: `dx_accept_no_gate_test.go:67: Accept: proposal check found 1 problem(s); first: test_finding in brain:project/brain-proposals/nogate.md (*brain.CheckRefusal)` (the `home` subtest fails the same way). Committed first as e99ac342. |
| 2. Accept publishes after the change | HOLDS | `TestDxNoGateAcceptPublishesDespiteAFinding` (project and home): nil error, entry at `knowledge/<topic>/<name>.md`, proposal status accepted, log holds one `\| accepted \| nogate \|` line. `go test -count=1 ./internal/brain/...` exit 0. |
| 3. DecideTask accept publishes | HOLDS | `TestDxNoGateDecideTaskAcceptPublishesDespiteAFinding`: one version via `wantChain`, receipt `Done`, proposal accepted. Extra, same file: `TestDxNoGateDecideTaskMergeIntoPublishesDespiteAFinding` (merge-into took the same check). On base both fail with `DecideTask: proposal check found 1 problem(s) ... (*brain.CheckRefusal)`. |
| 4. Config topic refused by `gate()` | HOLDS, passes on base by design | `TestDxNoGateConfigTopicStillRefusedByTheKeeperGate`, with the always-finding stand-in in place: Accept error holds `proposal rejected: Config topics must be changed through their own commands.`, proposal status rejected, no entry; DecideTask error holds `proposal refused: Config topics must be changed through their own commands.`, no receipt, no entry. This row pins what stays, so it passes on base too (shown in the base run: PASS). |
| 5. Grep and diff | HOLDS | `git grep -n -E "CheckRefusal\|AcceptCheckRequired\|brain_accept_check_levels\|checkAccept\|refuseAccept\|check_failed\|afterProposalRead" <rev> -- . ':!docs/lanes' \| wc -l`: **43 on `bb3c1340`, 0 on the lane**. `git diff main --stat`: 11 files, all named in Touches (README.md, accept_gate.go, accept_gate_test.go, dx_accept_no_gate_test.go, store.go, task_decide.go, check_test.go, command.go, decide.go, decide_test.go, SKILL.md); this report adds `docs/lanes/C-eo-brain-accept-gate-drop.md`. |
| 6a. Tests | HOLDS | `go test -count=1 ./internal/brain/... ./internal/braincmd/... ./internal/cli/... ./internal/packaging/...`: all `ok`, exit 0. `TestCheckExitCodes`, `TestCheckProposalByID`, `TestCheckFencesQuotedTextAndUsage`, `TestAcceptCleanProposalStillExitsZero`, `TestCheckReportsEntryEditedOutsideTheKeeper` are unedited and pass (`grid brain check <id>` still prints findings and exits 3). |
| 6b. Importers | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...` filtered to packages depending on `internal/brain` or `internal/braincmd`: cmd/grid, approvefix, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, people, ports, question, routine, signal, task, usage, workflow, lib. `go test -count=1` on all of them: `ok`, exit 0. `go build ./...`: exit 0. |
| 6c. Golden | HOLDS | `make golden` exit 0 (`TestGoldenCorpus` PASS); `git status --short` after it prints nothing, so no golden file changed. |
| 6d. Lint and cross builds | HOLDS | `make lint` (runs `go vet ./...`) exit 0; `GOOS=windows go vet ./...` exit 0; `GOOS=darwin go vet ./...` exit 0 (one `&&` chain, every step passed). |
| Rule: small commits, no attribution, public repo | HOLDS | 3 commits (test, removal, this report), one sentence each, no `Co-Authored-By` or session line; this file holds no personal path, host or email. |
| Rule: new tests fail on base, pass on lane | HOLDS | Rows 1 and 3 above; row 4 is the stay-behaviour and is marked. |
| Rule: no existing test edited to pass | HOLDS | Only deletions, listed below; `check_test.go` and `decide_test.go` shared helpers (`failMark`, `decideMark`, `snapshotFiles`) are untouched. |
| Rule: no guard, gate, cap, hold or blocking hook added | HOLDS | Nothing added outside the test file. `gate()`, `check.go`, `audit_check.go` and the `grid brain check` verb untouched. |

## Deleted tests and what covers what stays

The behaviour they pin was removed by the owner's direction: the removal list (needed.md:43, "Drop the check gate and trust tiers") and simplify.md:58.

| Deleted test | Covered by |
|---|---|
| `TestAcceptRefusedByCheckWritesNothing` | Removed with the gate (owner direction above). The opposite is `TestDxNoGateAcceptPublishesDespiteAFinding`. |
| `TestAcceptCleanProposalStillSucceeds` | Row 2 test and the existing accept tests (`TestAcceptKeepsVersionAndAuditHash`, `TestProposalLifecycle`). |
| `TestConfigTopicStillRefusedByGateNotCheck` | `TestDxNoGateConfigTopicStillRefusedByTheKeeperGate/Accept` (row 4; uses `agents`, not `deploy`). |
| `TestAcceptChecksBytesReadNotAnEarlierCheck` | Behaviour removed with the gate (owner direction above). |
| `TestAcceptIgnoresAnEarlierFailingCheck` | `TestDxNoGateAcceptPublishesDespiteAFinding` (a finding does not stop accept). |
| `TestAcceptRaceNeverPublishesFailingBytes` | Behaviour removed with the gate. |
| `TestAcceptUsesOneReadAcrossASwapBetweenReadAndCheck` | Behaviour removed with the gate; it used the `afterProposalRead` seam, which went with it. |
| `TestAcceptCheckRequiredSeamIsPerLevel` | Behaviour removed: `AcceptCheckRequired` and `brain_accept_check_levels` are gone. |
| `TestAcceptRefusedByCheckExitsThree` (braincmd) | Refusal removed. What stays is covered by `TestAcceptCleanProposalStillExitsZero` and `TestCheckExitCodes`. |
| `TestDecideRefusedByCheckWritesNothing` (braincmd) | Refusal removed. `TestDecidePassingCheckStillPublishes` and the DecideTask rows above cover what stays. |

## What changed

- Deleted `internal/brain/accept_gate.go` (`CheckRefusal`, `AcceptCheckRequired`, `afterProposalRead`, `checkAccept`) and `accept_gate_test.go`.
- `store.go`: Accept no longer calls `checkAccept` or the seam. Consequences: `readProposalBytes`'s second result is now ignored (`n, _, err :=`), and the two comments that described the check were removed.
- `task_decide.go`: `DecideTask` no longer calls `checkAccept`; its two comment lines went with it.
- `braincmd/command.go`, `decide.go`: the `CheckRefusal` branches and `refuseAccept` are gone; any error is `fail(err)` (exit 1). `plainCheck` stays for the verb.
- `contract/README.md`: the accept paragraph says accept publishes what passes the keeper gate and that `grid brain check` is a read the keeper may run first.
- `skills/brain-keeper/SKILL.md` step 3: "Run `grid brain check <id>` and judge the findings"; "the check is the gate that exists" is gone.
- New `internal/brain/dx_accept_no_gate_test.go` (helpers `dxNoGate*`, tests `TestDxNoGate*`), also covering the home level and merge-into.

## Open items and notes

- NOT VERIFIED on a live screen: none; no UI is touched. App main has no reader of `check_failed` (brief), so no app change.
- Note: `readProposalBytes` now has no caller that uses its bytes in Accept; it could shrink to a plain read. Left alone, outside the brief.
- Budget: the work-in-progress bundle was written after call 50, later than the call-35 rule, because the removal needed several edit retries.
