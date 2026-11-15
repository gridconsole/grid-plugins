# Lane C-r20-cancel-request-core

This lane builds the contract, fold and orchestrator half of decision D-r19, option C (docs/decisions/D-r19-person-cancel-holder.md). A person of the team whose task lease another holder holds posts an unleased cancel request. The holder's next pass then writes the leased control. The spec is the rule at doc:73-107, the rows at doc:109-135 (N rows doc:126-134) and the Files table at doc:137-153. The CLI half (`grid job cancel` posting the request) is lane C-r20-cancel-request-cli. No internal/cli file is changed here.

Base: origin/main d807a936. Merging origin/main before the last proof run reported "Already up to date".

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| Do 1: control.json has a third leaseless branch (lease null, gen 0, fence 0, role owner/member/lead, status `cancelled` only, Job-id target), with fixtures and a README entry | HOLDS | contract/schemas/control.json, third outer `oneOf` branch (status const at :406); fixtures in contract/fixtures/control/valid/person-cancel-request.json and invalid/person-cancel-{guest,not-cancelled,fence,gen}.json; contract/README.md:331 |
| Do 2: an unleased control does not move a Job (N6); fold_test.go:461-468 "control-without-task-lease" still passes | HOLDS | internal/contract/fold.go:186-192 (`if p.Lease == nil { continue }` after the unknown-target check, so the verdict stays counted). `go test -count=1 ./internal/contract/` exit 0. TestR20CrN6FoldLeavesJobForRequest PASS |
| Do 3: PostPersonUnless takes a cancel `control` from a person writer, checks R8/R9 under the lock and returns them as plain errors; no new export | HOLDS | internal/orchestrator/person_write.go:60-84; TestR20CrR8R9Refusals checks the exact text and that the error is not InvalidRecordError. personCancels and personCancelID are unexported |
| Do 4: personCancels runs after personAnswers; it writes a leased control with re = request id and a derived id; a request already answered is done; the earliest wins; the others and late requests get a not-used message | HOLDS | internal/orchestrator/person.go:139 (personCancels), :210 (personCancelID); internal/orchestrator/tick.go:66-73 (reload after personAnswers, then personCancels). Tests N2, N7, N8, N9 |
| Do 5 / N2: the holder carries out the request | HOLDS | TestR20CrN2HolderCarriesOutRequest PASS |
| Do 5 / no holder waits: with no holder, the request waits and nothing moves the Job | HOLDS | TestR20CrNoHolderWaits PASS |
| Do 5 / N3, N4: a guest or agent role is refused by the store, and the pass ignores a snapshot holding such a record | HOLDS | TestR20CrN3N4StoreRefusesGuestAndAgent, TestR20CrN3N4PassIgnoresGuestAndAgent PASS (the pass test hands personCancels a snapshot holding the forged record) |
| Do 5 / N5: any other status is refused; error kind pinned | HOLDS | TestR20CrN5OtherStatusRefused: `*feed.InvalidRecordError` from Store.Post (schema) and from PostPerson (errPersonOnly), which the CLI maps to exit 3. Nothing is written. TestR20CrN5SchemaShape PASS |
| Do 5 / N6 | HOLDS | TestR20CrN6FoldLeavesJobForRequest PASS |
| Do 5 / N7: the earliest wins | HOLDS | TestR20CrN7EarliestWins PASS (winner by (At, ID); one control and one not-used post; a second tick writes nothing) |
| Do 5 / N8: the target closed first | HOLDS | TestR20CrN8ClosedFirst PASS (one not-used post, no control; a second tick writes nothing) |
| Do 5 / N9: the pass runs twice with a new task fence between, and again in the same tenure | HOLDS | TestR20CrN9NewTenure, TestR20CrN9RepeatedPassSameTenure PASS |
| Acc 1: named tests pass | HOLDS | `go test -count=1 -v -run 'TestR20Cr' ./internal/orchestrator/ ./internal/contract/` exit 0; all 13 `--- PASS` (listed below) |
| Acc 2: the schema rejects a guest-authored leaseless cancel and any status but cancelled | HOLDS | `go test -count=1 -v ./internal/contract/ -run Fixture` exit 0: `--- PASS: TestFixtures/control/invalid/person-cancel-guest.json`, `.../person-cancel-not-cancelled.json`, `.../person-cancel-fence.json`, `.../person-cancel-gen.json`, `.../valid/person-cancel-request.json`; also TestR20CrSchemaRoles and TestR20CrN5SchemaShape |
| Acc 3: existing tests pass; TestR18* unchanged | HOLDS | `go test -count=1 ./internal/job/lint/ ./internal/mcp/ ./internal/orchestrator/ ./internal/contract/ ./contract/...` exit 0 (tail below). `git diff --stat origin/main..HEAD` lists no TestR18 file |
| Acc 4: no internal/cli file changed | HOLDS | `git diff --stat origin/main..HEAD`: 15 files, none under internal/cli |
| Acc 5: lane doc with proof table | HOLDS | this file |
| Rule: tests of changed packages and every importer | HOLDS | importers (`go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep -E " .../internal/(orchestrator\|contract)( \|$)"`): artifact, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, updates, usage, where, workflow. All ok on c2e885b3, except mcp, which failed on its golden before the golden commit (updates has no test files). On the head, orchestrator, contract, contract/..., mcp, job/lint and the cli walk tests were rerun, exit 0. The head commit changes only the golden |
| Rule: lint | HOLDS | `make lint` (go vet ./...) exit 0 |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Rule: plugin walk tests, after the merge of origin/main | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0 |
| Rule: no change to store.go:143/:146/:153 or record.go:183 | HOLDS | the diff stat has no internal/feed file |
| Rule: no guard, cap, limit, gate or hold beyond the decision's rule | HOLDS | the only refusals are R8/R9 (doc rows) and the schema's role and status pins (doc rule point 1); nothing blocks an agent |
| Rule: no existing test edited to pass | HOLDS | the diff adds three new test files. testdata/golden/mcp-session.golden was regenerated (`make test UPDATE_GOLDEN=1 GOFLAGS=-run=^TestSessionGolden$`) because it embeds control.json; the one changed line is the schema text. No test code changed |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log origin/main..HEAD` |
| Rule: no sudo, push, curl or cp; work only in this clone | HOLDS | none run |
| Rule: public repo, no personal paths, hosts or emails | HOLDS | the diff and this doc hold none |
| Rule: live screen | not run, not asked | see NOT VERIFIED |

## Named tests (Acc 1)

```
--- PASS: TestR20CrN3N4PassIgnoresGuestAndAgent
--- PASS: TestR20CrN9RepeatedPassSameTenure
--- PASS: TestR20CrN2HolderCarriesOutRequest
--- PASS: TestR20CrNoHolderWaits
--- PASS: TestR20CrN3N4StoreRefusesGuestAndAgent
--- PASS: TestR20CrN5OtherStatusRefused
--- PASS: TestR20CrR8R9Refusals
--- PASS: TestR20CrN7EarliestWins
--- PASS: TestR20CrN8ClosedFirst
--- PASS: TestR20CrN9NewTenure
ok  	github.com/gridconsole/grid-plugins/internal/orchestrator	1.859s
--- PASS: TestR20CrN6FoldLeavesJobForRequest
--- PASS: TestR20CrSchemaRoles
--- PASS: TestR20CrN5SchemaShape
ok  	github.com/gridconsole/grid-plugins/internal/contract	0.025s
```

## Existing tests (Acc 3, tail)

```
ok  	github.com/gridconsole/grid-plugins/internal/job/lint	0.147s
ok  	github.com/gridconsole/grid-plugins/internal/mcp	0.195s
ok  	github.com/gridconsole/grid-plugins/internal/orchestrator	91.079s
ok  	github.com/gridconsole/grid-plugins/internal/contract	0.529s
ok  	github.com/gridconsole/grid-plugins/contract	0.053s
ok  	github.com/gridconsole/grid-plugins/contract/formats	0.060s
```

## On base (the new tests fail without the change)

- `TestR20CrN6FoldLeavesJobForRequest r20_cancel_request_test.go:38: the request is "bad" ($: oneOf matched 0 branches: $.body: missing required question_event), want counted`
- `TestR20CrSchemaRoles :56: owner: the leaseless cancel was refused: ...`; `TestR20CrN5SchemaShape :70: the leaseless cancel was refused: ...`
- `TestR20CrN2HolderCarriesOutRequest :90: only a person of the team posts a Job without the task lease`; NoHolderWaits :128, N7 :232, N8 :263 and N9NewTenure :285 fail with the same text
- `TestR20CrN3N4StoreRefusesGuestAndAgent :175: owner: $: oneOf matched 0 branches ...`; `TestR20CrN5OtherStatusRefused :201`; `TestR20CrR8R9Refusals :213: R8: got only a person of the team posts a Job ...`
- r20_cancel_pass_test.go does not compile on base (undefined personCancels).
- Review note 1, with the schema change alone and no fold change: `TestR20CrN6FoldLeavesJobForRequest :42: the request moved J-1: got ... Status:cancelled ... want ... Status:running`. The fold change at fold.go:190 is what fixes it.

## What changed

- contract/schemas/control.json: a leaseless cancel branch. contract/README.md:331 documents it. There are five fixtures.
- internal/contract/fold.go:186-192: an unleased control is counted, but it moves no Job.
- internal/orchestrator/person_write.go:60-84: a person may post the cancel request, with R8/R9 checked under the lock.
- internal/orchestrator/person.go:139-220: personCancels and personCancelID. The handled set holds the leased records whose re is a request. Ids are derived as the request's time prefix plus a sha256 tail, so a repeated pass in the same tenure returns the stored record. The not-used notices are written before the control.
- internal/orchestrator/tick.go:66-73: after personAnswers, the pass reloads the snapshot and then runs personCancels.
- testdata/golden/mcp-session.golden: regenerated for the schema text.

## NOT VERIFIED on a live screen

- A real worker holding the task lease and writing the control on its next tick on a machine. The tests drive Tick in process only.
- F4a and whether the app supervisor stops the agent running a Job once the leased cancel lands.
- The CLI path (`grid job cancel` without an epoch posting the request), which is lane C-r20-cancel-request-cli.
