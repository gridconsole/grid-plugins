# Lane C-r20-cancel-request-cli

This lane builds the CLI half of decision D-r19, option C (docs/decisions/D-r19-person-cancel-holder.md). `grid job cancel` by a person of the team (role owner, member or lead) with no lease epoch now writes the cancel at once under the task lease when the person is the holder (`GRID_MEMBER@<this machine>`), and saves an unleased cancel request when anyone else holds the lease, or nobody does. The holder's next pass carries it out (C-r20-cancel-request-core, already on this branch: head 82048785). It also answers end review 2026-11-15T1612Z M1: the help and comment no longer describe a path no product person can take. A guest and the `cli` role of `grid worker next` stay refused (R2, R5).

Base: the branch holds d807a936 (the decision) and 82048785 (the core lane); `git fetch origin && git merge origin/main` reported "Already up to date" before the last proof run. This lane's own commits are the three after 82048785: cd370e49 (CLI), a40aa06f (tests), and the gofmt fix.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| Do 1: keep the leased write only when the task lease holder is `GRID_MEMBER@<this machine>`; otherwise (another holder, or none) set `request` and write through the unleased writer | HOLDS | internal/cli/orchestrator.go:316-324 (`request`, then `unleased = true` at :322 so the existing `feedWriter(env, "", "")` at :327 builds the writer). The machine comes from `localMachine(env)` (`env.Machine`, else `gridhome.Machine`), the helper that `orchestratorWriter` now uses too, so both read it one way |
| Do 2: the request builds the same body as the leased cancel, keeps the `--reason` check (R1) before the write | HOLDS | internal/cli/orchestrator.go:419-425 (`cancel requires --reason` at :421, body `{target, status, reason, close_reason}` at :424); `unleased` selects `orchestrator.PostPerson` at :426-429. The check was already before the write; TestR20ClR1RequestNeedsAReason pins it |
| Do 3: print `Saved. The cancel will be carried out on the next pass of the task's orchestrator.` and the `pending: true` JSON for a request | HOLDS | internal/cli/orchestrator.go:452-463; TestR20ClN1PersonOfAnotherMemberSavesARequest (stderr), TestR20ClRequestJSONIsPending |
| Do 4: help (doc:157-181) and the comment replaced verbatim | HOLDS | help at internal/cli/orchestrator.go:32-46; comment at :307-315. The only difference from the doc's help text is the backticks around `grid job show <job>`, which sit inside a Go raw string. TestR20ClJobCancelHelpIsTheDecisionsText compares the five lines of doc:160-164 and the two lines of old :42-43 byte for byte |
| Do 5: N1 test replaces TestR18PersonOfAnotherMemberStaysStale (removal directed by the decision, said in the test comment); R10, R8, R9, R1 tests; guest refusal pinned; "reads the" dropped from the help test with a comment; other TestR18* unchanged; R7a added | HOLDS | internal/cli/r18_person_job_cancel_test.go:82-112 (N1; its comment names doc:126 and doc:152) and :114-131 (the help test; its comment names doc:157-165). `git diff 82048785..HEAD -- internal/cli/r18_person_job_cancel_test.go` touches only those two tests. The guest sees exit 1 `GRID_LEASE_EPOCH or --lease-epoch must be gen.fence` from orchestratorWriter: cli.go:241's gate does not take `job` and `workflowRoleRefusal` does not refuse `job cancel` (TestR20ClR2GuestIsRefusedWithoutAnEpoch) |
| Acc 1: N1, N10, R1, R2, R8, R9, R10 each have a named passing test | HOLDS | `go test -count=1 -v -run 'TestR20Cl\|TestR18' ./internal/cli/` exit 0: the 16 `--- PASS` lines are listed below |
| Acc 2: the CLI request, then the holder's pass: the Job is Cancelled by a leased control by the holder whose `re` is the request id | HOLDS | TestR20ClRequestThenHolderPassCancelsTheJob PASS. The pass runs as `grid feed tick --task <path> --json` under GRID_MEMBER=worker, GRID_ROLE=worker, GRID_TASK_EPOCH and GRID_LEASE empty, GRID_LEASE_EPOCH = the task epoch. The brief calls it `grid job tick`; no such command exists (`runOrchestratorJob` at orchestrator.go:49-50 has no `tick`; `runOrchestratorFeed` at :57-58 does), so the test uses `grid feed tick` |
| Acc 3: `go test ./internal/cli/ ./internal/orchestrator/` passes; TestHsIpWalksEveryCommand and TestIlGuestSiteList pass | HOLDS | tail below, exit 0 for both. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 |
| Acc 4: `grid job cancel --help` output holds doc:160-164, with :42-43 kept | HOLDS | output below (`go run ./cmd/grid job cancel --help`) |
| Acc 5: this file | HOLDS | docs/lanes/C-r20-cancel-request-cli.md |
| Rule: new tests fail on base and pass on the lane | HOLDS, see below | the failing lines on base are listed below. Four tests pass on base, as the decision marks their rows "today": N10, R1, R2 and R7a. They pin that the change leaves those rows as they are |
| Rule: tests of the changed package and every importer | HOLDS | `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep " .../internal/cli( \|$)"` gives cmd/grid and lib. `go test -count=1 ./internal/cli/ ./internal/orchestrator/ ./cmd/grid/ ./lib/` exit 0 (cmd/grid has no test files) |
| Rule: lint | HOLDS | `make lint` (go vet, gofmt check) exit 0 after the gofmt fix. The first run failed on gofmt in orchestrator.go; fixed in the head commit |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Rule: plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0 |
| Rule: merge origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main`: "Already up to date" |
| Rule: no change to store.go or record.go | HOLDS | `git diff 82048785..HEAD --stat` lists no internal/feed file |
| Rule: no guard, cap, gate or hold on agents beyond the decision's rule | HOLDS | the only new branch is for PersonTeam with no typed epoch, which turns a refusal into a saved request. Nothing new refuses an agent or a guest; R2 and R5 behave as before |
| Rule: no existing test edited to pass | HOLDS | one test removed by the decision (TestR18PersonOfAnotherMemberStaysStale, replaced by N1, both named above) and one wanted token dropped ("reads the") with the reason in the test comment |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log 82048785..HEAD` |
| Rule: no sudo, push, curl or cp | HOLDS | none run |
| Rule: public repo, no personal paths, hosts or emails | HOLDS | the diff and this file hold none |
| Rule: the app's guest walk tables, `make webui-test` | not run, not asked | plugin repo |
| Rule: *RealPlugin* tests | not run, not asked | the brief names none |

## What changed

- internal/cli/orchestrator.go: help text; the holder test for a person's cancel (`request`); `localMachine` helper shared with `orchestratorWriter`; "Saved." line and `pending` JSON for a request.
- internal/cli/r18_person_job_cancel_test.go: N1 replaces the stale-lease test; "reads the" is dropped from the help test.
- internal/cli/r20_cancel_request_cli_test.go: N10, R1, R2, R7a, R8, R9, R10, the no-holder case, the JSON shape, the end-to-end pass, and the help text.

A cancel with no readable lease file (none ever taken, or a read error) now saves a request as well. The brief says "Otherwise (another holder, or none)". Before, that case stopped at "must be gen.fence". `TestR20ClNobodyHoldsTheRequestIsSaved` covers a lease file with no holder (after a release).

## Failing on base (orchestrator.go stashed, tests kept)

```
--- FAIL: TestR20ClN1PersonOfAnotherMemberSavesARequest
    r18_person_job_cancel_test.go:93: a cancel by a member who does not hold the task lease: exit 3 stale lease
--- FAIL: TestR20ClR8UnknownJob
    r20_cancel_request_cli_test.go:150: an unknown Job: exit 3 stale lease
--- FAIL: TestR20ClR9ClosedJob
    r20_cancel_request_cli_test.go:171: a closed Job: exit 3 stale lease
--- FAIL: TestR20ClR10PersonOnAnotherMachine
    r20_cancel_request_cli_test.go:190: a person on another machine: exit 3 stale lease
--- FAIL: TestR20ClNobodyHoldsTheRequestIsSaved
    r20_cancel_request_cli_test.go:218: a cancel with no holder: exit 1 GRID_LEASE_EPOCH or --lease-epoch must be gen.fence; ...
--- FAIL: TestR20ClRequestJSONIsPending
    r20_cancel_request_cli_test.go:233: exit 3 stale lease
--- FAIL: TestR20ClRequestThenHolderPassCancelsTheJob
    r20_cancel_request_cli_test.go:256: the request: exit 3 stale lease
--- FAIL: TestR20ClJobCancelHelpIsTheDecisionsText
    r20_cancel_request_cli_test.go:325: job cancel --help does not hold: ...
```

## Passing on the lane

```
--- PASS: TestR18PersonCancelsRunningJobWithoutAnEpoch
--- PASS: TestR18PersonTypedGuessedEpochStaysStale
--- PASS: TestR20ClN1PersonOfAnotherMemberSavesARequest
--- PASS: TestR18JobCancelHelpNamesHowAPersonCancels
--- PASS: TestR18AgentCancelIsNotGivenTheLease
--- PASS: TestR20ClN10HolderPersonCancelsAtOnce
--- PASS: TestR20ClR1RequestNeedsAReason
--- PASS: TestR20ClR2GuestIsRefusedWithoutAnEpoch
--- PASS: TestR20ClR7aPersonJobLeaseNoEpoch
--- PASS: TestR20ClR8UnknownJob
--- PASS: TestR20ClR9ClosedJob
--- PASS: TestR20ClR10PersonOnAnotherMachine
--- PASS: TestR20ClNobodyHoldsTheRequestIsSaved
--- PASS: TestR20ClRequestJSONIsPending
--- PASS: TestR20ClRequestThenHolderPassCancelsTheJob
--- PASS: TestR20ClJobCancelHelpIsTheDecisionsText
ok  	github.com/gridconsole/grid-plugins/internal/cli	0.989s
```

```
ok  	github.com/gridconsole/grid-plugins/internal/cli	37.635s
ok  	github.com/gridconsole/grid-plugins/internal/orchestrator	110.030s
```

## `grid job cancel --help` (Acc 4)

```
Usage: grid job cancel <job> --reason <text> [--task <task>] [--json]

Cancels a Job, a Running one too.
A person (role owner, member or lead) types no lease and no epoch. When the task lease is
held by your own <member>@<machine> (GRID_MEMBER on this machine), the cancel is written
at once under it. When another holder holds it (the machine's worker, worker@<machine>, by
default), the cancel is saved as your request and the holder writes it under its lease on
its next pass; `grid job show <job>` then shows it Cancelled. Both exit 0.
An agent cancels with the GRID_TASK_EPOCH its worker gave it. A --lease-epoch typed on the
command line is used as typed, so a guessed or old one is refused as a stale lease (exit 3).
```

## Notes for the reviewer

- R9 is pinned at the CLI for a Cancelled Job only. Accepted and Rejected come from the same check in `PostPersonUnless` (person_write.go), which TestR20CrR8R9Refusals in the core lane covers.
- The holder test reads `GRID_MEMBER`, as `orchestratorWriter` does for the leased write. The request's writer comes from `feedWriter`, which takes `GRID_ACTOR` first, as the unleased Job post does. The app sets both to the same value for a person.
- An empty GRID_ROLE counts as `member`, so a shell with neither GRID_ROLE nor a task epoch takes the request path: the same exposure the unleased Job post has (review of the decision, note 3).

## NOT VERIFIED on a live screen

- A real worker holding the task lease, writing the leased control on its next tick (F4a, decision doc:207-221). The tests drive `grid feed tick` in process.
- Whether the app's supervisor stops an agent whose Job another process cancelled, and that no `judgement_review_cancelled` is logged for the rework's builder (F4a steps 5 and 6).
- The text of `grid job cancel --help` in a terminal, beyond the output above.
