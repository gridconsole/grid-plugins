# C-r12-builder-proposes-asked-note

A builder whose Job asks for a brain note proposes it with `grid brain propose` and lists it in the result's `knowledge`. Eval round 12, E3. Text change plus one text test; no code, no app change.

Verdict: DONE. `make test` and `make lint` ran on the lane head (the commit after 8b8959ca); `git fetch origin` showed nothing new on main (`git log HEAD..origin/main` empty), so there was nothing to merge.

## Cause (inputs in /srv/gridwork/tmp/C-r12-builder-proposes-asked-note-inputs/)

Task T-03 in `tally-e3/tasks/T-03-.../`.

How the ask reached J-2:
- The owner's request (`task.md:4`, `intent.md#request`, repeated in the J-2 brief at `jobs/J-2/brief.md:39`): "please leave a note about how money is handled in tally for whoever works on it next".
- `plan.md:3-4` turns it into "leave a note on how money is handled in tally". `plan.md:11` is the step "Add the export subcommand and the money note". Neither names the brain.
- J-2 (`jobs/J-2/job.yaml`, role builder) has `keep: []` and `decisions: []` (J-2.json:10-11), `write` lists `README.md` (J-2.json:21), and the only row about the note is A6 (J-2.json:32): "`README.md` has a section on money handling that says: ...". So the planner turned the ask into a file row. No row says "propose it to the brain" or "list it in `knowledge`".
- The brief's Instructions (`jobs/J-2/brief.md:123`) are `agents/builder.md:28` verbatim: "how to read brain notes with `grid brain show` and propose a fact or a replacement with `grid brain propose`". That is a capability, not an instruction tied to an ask.

What the builder text said before this lane:
- `agents/builder.md:28`: the sentence above; nothing about a note the Job asks for.
- `skills/run-job/SKILL.md:70`: "To leave a durable fact for later tasks, propose it" (optional, the builder's own judgement). `:43` shows `"knowledge": []` in the example body and `:47` says nothing about when to fill it.
- `skills/plan-jobs/SKILL.md:13` says only that a subagent curates proposals; checks 1-7 (`:16-24`) have no check for an asked note.
- The `knowledge` field: `contract/schemas/result.json:244-262` requires the key, an array of non-empty strings, so `[]` is valid. `contract/docs/found-and-result.md:88`: "A person and the brain keeper. The fold does not read it." Nothing in code makes a builder fill it, so a builder that wrote README.md and `knowledge: []` passes the schema.

Why round 11 proposed and round 12 did not: the same ask, the same builder text. With no row naming the proposal, whether a builder proposes depends on its own judgement (r12-REPORT.md:14, :30: J-2 `attempt-1.log` holds `grid brain propose` only in the brief text; r12-REPORT.md:30 says round 11 proposed). The ask reaches the builder only sometimes because only the person's words carry it, and the planner turned them into a file row.

## The change

`skills/plan-jobs/SKILL.md`:
- new check 8 (after `:24`) "Note to keep": when the request or the plan step asks for a note, a decision or knowledge to keep, the Job has an acceptance row of its own naming what it says and its topic; the row says the builder proposes it with `grid brain propose --task <task>` and lists it in `knowledge`; a file the request also names is a second row; a file never stands in for the proposal.
- the closing line now reads "passed the eight checks" (was seven).

`skills/run-job/SKILL.md` Brain section:
- when the Job asks for a note, decision or knowledge to keep (intent, the owner's request in the Intent block, or a row), propose it before the result, one proposal per thing asked; topic `conventions` or `decisions`; a README does not stand in for it.
- list each proposal in `knowledge`, one string each: `<name> (<topic>): <what it holds>`. A builder whose Job asks for a note must fill `knowledge`; a row that asks for the note is `not met` while `knowledge` is `[]`. `[]` is right when the Job asks for nothing to keep.

`agents/builder.md:28`:
- before: `... and propose a fact or a replacement with `grid brain propose`. Small commits, one sentence each.`
- after: `... and propose a fact or a replacement with `grid brain propose`. When the Job's intent or an acceptance row asks for a note, a decision or knowledge to keep, propose it with `grid brain propose --task <task>` before you post the result and list it in the result's `knowledge`; a file in the repo does not replace the proposal. Small commits, one sentence each.`

No code changed. `knowledge` stays optional in the schema (`[]` valid) because a Job that asks for nothing has nothing to list; the rule is text, and the reviewer's proof table shows an unmet row. No guard, gate or cap was added.

Test: `internal/job/lint/r12_proposes_asked_note_text_test.go` (`TestR12AskedNotePathIsNamed`) pins the sentences in the three files and that "seven checks" is gone.

## Proof table

| Row | State | Proof |
|---|---|---|
| Brief 1: how the ask reached J-2 and what the builder text says, with file:line | HOLDS | "Cause" above |
| Brief 2: the Job names the asked note (plan-jobs), the builder proposes it and lists it in its result (run-job); whether `knowledge` must be filled | HOLDS | `skills/plan-jobs/SKILL.md` check 8, `skills/run-job/SKILL.md` Brain section, `agents/builder.md:28`; `knowledge` answer under "The change" |
| Brief 3: test where code changes; text-only: before and after quoted | HOLDS | no code changed; before/after for `agents/builder.md:28` quoted above; a text test added |
| Brief 4: lane doc | HOLDS | this file |
| Acceptance: the cause with file:line | HOLDS | "Cause" above |
| Acceptance: the diff | HOLDS | `git diff origin/main` : `agents/builder.md`, `skills/plan-jobs/SKILL.md`, `skills/run-job/SKILL.md`, the new test, this doc |
| Acceptance: `make test` | HOLDS | exit 0; every package `ok`, last line `temp leak check: no new matching entries` |
| Acceptance: `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| New test fails on base | HOLDS | failing lines below |
| No existing test edited | HOLDS | the diff only adds a test file |
| No guard, gate, cap or hold on agents | HOLDS | text only |
| App not changed | HOLDS | plugin-only diff |
| Tests of changed packages and importers | HOLDS | the changed package `internal/job/lint` is test-only (no importers); `make test` ran all packages, exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok internal/cli 0.078s` |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok internal/job/lint 0.123s` |
| Merge origin/main before the last proof | HOLDS | `git fetch -q origin`, `git log HEAD..origin/main` printed nothing |
| Cross-repo set | not run, not asked | the orchestrator runs it at landing |
| *RealPlugin* tests | not run, not asked | the brief names none |

### New test on base (detached worktree of origin/main with only the new test copied in)

```
--- FAIL: TestR12AskedNotePathIsNamed (0.00s)
    r12_proposes_asked_note_text_test.go:50: ../../../skills/plan-jobs/SKILL.md lacks "8. Note to keep:"
    r12_proposes_asked_note_text_test.go:50: ../../../skills/plan-jobs/SKILL.md lacks "passed the eight checks"
    r12_proposes_asked_note_text_test.go:50: ../../../skills/run-job/SKILL.md lacks "When the Job asks for a note, a decision or knowledge to keep"
    r12_proposes_asked_note_text_test.go:50: ../../../skills/run-job/SKILL.md lacks "`not met` while `knowledge` is `[]`"
    r12_proposes_asked_note_text_test.go:50: ../../../agents/builder.md lacks "list it in the result's `knowledge`"
    ... (15 lines in all)
FAIL	github.com/gridconsole/grid-plugins/internal/job/lint
```

On the lane: `go test -count=1 -run TestR12 -v ./internal/job/lint/` exit 0, `--- PASS: TestR12AskedNotePathIsNamed`.

## Open items

- `agent-templates/grid-builder.md:34` (the app's builder agent) says "When you learn something ... propose it ... Name the proposal in your result". It is triggered by learning, not by an ask. I left it alone: the brief names `agents/builder.md` and `skills/run-job/`, and the J-2 Instructions come from `agents/builder.md`. A follow-up could add the same sentence there.
- `agents/orchestrator.md:26` lists the checks by name ("the 10 Nov rules: ..."); check 8 is not in that list, only in the skill. A follow-up could add it.
- A lint that finds "note" words in a request would be a guess and was not built.

## NOT VERIFIED on a live run

That an orchestrator reading plan-jobs check 8 writes a proposal row for the money note, and that a builder then runs `grid brain propose` and fills `knowledge`. Needs the next eval round (E3 again): a curator Job and a brain note should follow.

DONE
