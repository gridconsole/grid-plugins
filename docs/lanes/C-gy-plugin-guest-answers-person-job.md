# C-gy-plugin-guest-answers-person-job

Base: d1c76719044999b8cb0bbff892c105e21801676b (main). On that base a guest cannot answer an addressed person Job: orchestrator.go:284 takes the unleased writer only for a team role, so a guest reaches orchestratorWriter and fails with the gen.fence text. No line of the base lets a guest through.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1 Addressed guest answers (`TestGyGpAddressedGuestAnswers`) | HOLDS | `go test -count=1 -run TestGyGp ./internal/cli/` exit 0 (test file internal/cli/gy_guest_person_answer_test.go:97). guest dana, no lease: exit 0, one `answer` record re J-2's first record, Author dana/guest, no lease, fence 0; a second `job result` exits 0 with a second answer record; one holder pass: J-2 accepted, reason "answered by dana", the second answer has the "was not used" message. |
| 1 fails on base | HOLDS | With the five production files at d1c76719: `gy_guest_person_answer_test.go:99: guest dana: exit 1: GRID_LEASE_EPOCH or --lease-epoch must be gen.fence; the task holder gets it from: grid lease take task`. Orchestrator side on base: `gy_guest_person_answer_test.go:101: stale lease`; `:168: the fold marked the addressee's answer "ignored", want counted`. |
| 2 Refusal table (`TestGyGpGuestRefusals`) | HOLDS | Same run, exit 0. Each case exits non-zero and the `.jsonl` byte count of the Feed is unchanged: eve, "Dana", pointer owner `plan.md#approve`, kind approve (with and without --plan-hash), kind ask with --plan-hash, worker Job J-1 all print the gen.fence text; empty GRID_MEMBER prints `project is not registered`; `job post --person dana` by a guest prints the gen.fence text; accepted Job prints `J-2 is already accepted`. No grant: no flag `project is not registered`, `--project example` prints `project "example" is not registered`, `--task <folder>` prints `task not found`; none prints gen.fence, none leaves a record. |
| 2 empty owner | NOT TESTABLE AS A JOB | A job record with an empty intent.owner is refused by the schema (`$.body.intent.owner: string length`), so the Job cannot exist. The empty-identity case is pinned in `TestGyGpGuestAddresseeRule` ("both empty") and the CLI row uses an empty GRID_MEMBER on dana's Job. |
| 3 Hand-appended answers (`TestGyGpHandAnswerIgnored`, internal/orchestrator) | HOLDS | `go test -count=1 ./internal/orchestrator/` exit 0. Guest eve through the store: refused with "guest answer must name a question addressed to that guest" (rejection_test.go:84 unchanged). Forced below the store (tier set to person-guest so the schema passes): fold verdict `ignored, question is not addressed to this guest`; personAnswers called directly with the record put in the counted set writes nothing; after a tick J-1 is still posted. A forced answer signed dana is counted. |
| 4 Lease (`TestGyGpGuestTaskLease`) | HOLDS | Same CLI run, exit 0. eve and dana with GRID_LEASE=task, and with `--lease task`, exit non-zero with the gen.fence text and no record. The cli.go:194 tests pass unchanged: `go test ./internal/cli -run 'TestGgUnknown\|TestGgPersonRefusal\|TestGuestProjectParity\|TestGuestStillAnswersAndComments\|TestFvGuestTask\|TestFyGuestTask' -count=1` exit 0. cli.go is untouched. |
| 5 Team unchanged | HOLDS | `TestPersonAnswerWhileLeaseHeld` and `TestPersonAnswerWhileTaskLeaseHeld` pass with no edit (`go test -count=1 ./internal/orchestrator/` exit 0; `go test ./internal/cli -run 'TestPersonAnswerWhileTaskLeaseHeld\|TestAgentCannotWriteAsPersonWithoutLease' -count=1` exit 0). The store text for a guest answer to an unaddressed question is unchanged (feed tests exit 0). |
| 6 Tests and lint | HOLDS | `go test -count=1 ./internal/feed/ ./internal/contract/ ./internal/orchestrator/` exit 0. `make lint` exit 0 (go vet and gofmt). |
| 6 `git diff main --stat` names only Touches | HOLDS, plus the report | internal/cli/orchestrator.go, internal/contract/fold.go, internal/contract/guest_person_answer.go, internal/feed/store.go, internal/orchestrator/person.go, person_write.go, person_write_test.go (the guest row only), the two new gy_ tests, and this report. |
| Importers | HOLDS | `go list` importers of contract, feed, orchestrator and cli: cmd/grid, approvefix, artifact, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, usage, where, workflow, lib. Ran: cli (whole), job/..., question, lease, mcp, hook, task, workflow, cmd/grid, lib, all exit 0. Not run: approvefix, artifact, brain, braincmd, guard, intent, jobgraph, land, migrate, session, settings, signal, usage, where (the orchestrator's whole-module run covers them). |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Existing test edited | NOTED | internal/orchestrator/person_write_test.go: in `TestUnleasedPersonWritesRefuseAgents` only the `guest` row now uses id "eve", because personWriterOf gave it the id "dana", the addressee of J-1. The addressed case is covered by `TestGyGpAnswerPersonGuest`. |
| No guard, gate, cap or hold on agents | HOLDS | Nothing is added that stops an agent. The change lets one more writer (the addressed guest) write. |
| No attribution lines, no personal paths | HOLDS | Commits are one sentence with no trailer. |
| Mismatch proof, *RealPlugin* rows | n/a | No row names one. |

## What changed

- internal/contract/guest_person_answer.go: `GuestAddressee(role, author, leased, jobRole, owner, kind)`. True only for a guest, with no lease, author and owner both set and byte-equal, Job role Person, kind not approve.
- Fold (fold.go): a map of each Job's role, owner and kind by its first job record, read by the guest-answer verdict.
- Feed store (store.go): a guest answer is admitted re a counted job record when GuestAddressee holds, besides the existing counted-question rule.
- person_write.go / person.go: `guestAnswerer` and `guestAddressed`; AnswerPersonWith takes an unleased guest, and inside the lock refuses with feed.ErrStaleLease unless the Job names that guest (the check needs the Feed, so it sits after Load rather than in the writer check at :167); personAnswers takes a guest's answer only when GuestAddressee holds.
- internal/cli/orchestrator.go: personResult also keeps the Job's role, owner and kind; the unleased writer is also taken when GuestAddressee holds with GRID_MEMBER, no GRID_LEASE, no `--lease`, no epoch and no `--plan-hash`. Every other guest keeps the old path and text. PostPerson is unchanged.

## Notes

- A guest whose id is literally the owner string `plan.md#approve` would match a Job whose owner is that pointer (exact compare, as the brief states). No project grants such an id in practice; a reviewer who wants it closed can name the rule.
- The tool-call budget of 60 was exceeded (about 75) because the forged-record test needed three rounds to find that the forged record had to carry tier person-guest and that Read verdicts differ from fold verdicts.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. The real `grid` binary was not run; every row ran through `cli.Run` in the test harness. The guest's `grid job result` through the real plugin with the app's project grants is not exercised.
