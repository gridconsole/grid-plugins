# What each field of a found and a result body means

This page pins the inline body of two Feed post types to their schemas:
[found.json](../schemas/found.json) and [result.json](../schemas/result.json).
Both schemas allow one of two bodies (`body.oneOf`): the inline body described
here, or a `ref` body that points at it. The tables list every property of the
inline body. A nested property is written as a path from the body: `acceptance.state`
is the `state` of each item in the `acceptance` array. The schemas are the source;
a field this page lacks is a page bug, and a row here that a schema lacks is too.
The base for this page is plugin `dc301fce`.

## found

A `found` post is a finding a reviewer, the lead or the CLI writes about work
that is already done (author roles `reviewer`, `lead`, `cli`; type class `chatter`).
All six fields are required and no other field is allowed.

| Field | Meaning | Who reads it |
| --- | --- | --- |
| `job` | The Job the finding is about, as `J-1` or `J-1.2`. | The fold: it looks the Job up in its Job map (`internal/contract/fold.go:264`, `:314`). A Job id the fold does not know gives no lead flag and no wake. |
| `re` | The id (a 26-character ULID) of the post the finding is about, for example the result that was reviewed. It is a field of the body, not the envelope's `re`. | A person reading the Feed. The fold does not read it on this base. |
| `at` | Where the finding is, as free text such as `main.go:10`. | A person or the lead reading the finding. The fold does not read it. |
| `text` | The finding itself, in one or two plain sentences. | A person or the lead. The fold does not read it. |
| `kind` | What sort of finding it is. One of `bug` (the work does the wrong thing), `security` (the work is open to misuse or leaks something), `evolvability` (the work is hard to change later) or `test` (a test is missing, wrong or weak). A value outside these four fails validation and the post stays a bad record (`contract/README.md:350-352`). | The fold. Only `security` has an effect: on a known Job it sets the Job's `NeedsLead` flag (`fold.go:261-265`) and adds the post to `wakes` (`fold.go:311-315`), which wakes the lead. `bug`, `evolvability` and `test` change no Job state. |
| `class` | How the finding is classed, or `null` when it is not. One of `cut`, `irreversible`, `visible` or `detail`, or `null`. The schema fixes these values; no text on this base says what each one decides. | No Go code in `internal` or `cmd` reads it on this base, and the fold ignores it. It is data for a person or the lead. |

A `security` finding is the one `kind` that moves the fold, and only when its `job`
names a Job that exists. Its `class` does not change that.

### The ref form

When a post is too large for one Feed line, the Feed writes the body to a file and
the line carries only `{"ref": "bodies/<ulid>.json"}`, where the ulid is the post's
own id (`internal/feed/store.go:384-393`). The reader loads that file, checks the
name matches the post id, and puts the loaded body in place of the `ref`
(`internal/feed/read.go:67-88`). Until the file is there the post is held as
waiting for its body. The fold never reads a `ref` body: a post whose body is
only `ref` is held (`internal/contract/fold.go:117-120`), so the fields above
reach the fold only after the Feed has resolved them.

```json
{
  "job": "J-1",
  "re": "01K9J000000000000000000001",
  "at": "internal/contract/fold.go:264",
  "text": "The security check reads the job id from an unvalidated field.",
  "kind": "security",
  "class": "irreversible"
}
```

## result

A `result` post is what a worker or agent posts when a Job ends (type version 2,
class `state`, author roles `worker`, `agent`, `cli`). All ten fields are required,
with `[]` for a list that has nothing in it, and no other field is allowed. The
envelope carries `lease`, `fence`, `job` and `attempt`; they are not part of the body.
The fold records a result in the Job's `Results` list (`internal/contract/fold.go:351`).
A result does not accept or reject a Job: control posts do that
(`contract/README.md:340-342`).

| Field | Meaning | Who reads it |
| --- | --- | --- |
| `outcome` | What happened, in plain words. A reviewer writes the verdict here, with `ACCEPTED` or `REJECTED` on the first line (`skills/review-job/SKILL.md:26`). | A person and the next agent. The fold does not read it. |
| `changed` | The paths the Job changed, one string each. `[]` when nothing changed. | A reviewer, checking the change against the Job's write list. The fold does not read it. |
| `evidence` | What proves the work: a command with its exit code, or a `path:line`, one string each. | A reviewer. The fold does not read it. |
| `acceptance` | One item per acceptance row of the Job. | A reviewer and the lead. The fold does not read it. |
| `acceptance.id` | The row id as the Job numbers it: `A1`, `A2` and so on. | The reviewer matching the row. |
| `acceptance.state` | Where the row stands. One of `met` (the row holds), `not met` (it does not, or was not finished), `dropped` (it was set aside) or `waiting` (the work cannot hold until something outside the Job lands; `docs/decisions/D-job-hold.md`). A `dropped` row must carry a `decision` and a `waiting` row must carry `waits_on`; `met` and `not met` may leave both out. `waiting` is not `not met`: `not met` says the work is short, `waiting` says it waits on someone else. | The reviewer. |
| `acceptance.decision` | Why a row was dropped, in one sentence. Required when `state` is `dropped`. | The reviewer and the lead. |
| `acceptance.waits_on` | What a `waiting` row waits on: one object with `kind` and `ref`, both required, no other key. Required when `state` is `waiting`. A row that needs two things names the one that is not there yet. | The reviewer, and the orchestrator that releases the hold. |
| `acceptance.waits_on.kind` | One of `job` (another Job's change accepted), `landed` (another Job's change merged and installed), `question` (a person's answer or decision) or `permission` (a permission rule the Job lacks). | The reviewer and the orchestrator. |
| `acceptance.waits_on.ref` | The thing waited on: the Job id `J-n` for `job` and `landed`, the question's gate id `Q-n` for `question`, and the id of the `blocked` post that names the rule for `permission`. | The reviewer and the orchestrator. |
| `verified` | A check made by someone other than the author, or `null` when there was none. | The fold copies it onto the Job as `Verified` (`fold.go:360`). |
| `verified.level` | How strong the check was. One of `L1`, `L2` or `L3`. `L1` also needs `verified.job`. | The fold only copies it; the lead reads it. |
| `verified.pass` | The outcome of the check: a boolean, or one of `pass` or `unsure`. | The lead. |
| `verified.by` | Who checked. One of `sonnet` or `grid-reviewer`. | The lead. |
| `verified.job` | The Job that did the check, as `J-n`. Required when `level` is `L1`. | The lead. |
| `verified.evidence` | What the check showed, one string each. | The lead. |
| `must_fix` | What the reviewer says must change, as objects, not strings. `[]` when nothing must change. This is the default of owner question 2 of folder hf: the schema stands. | The worker who reworks the Job, and the lead. The fold does not read it. |
| `must_fix.row` | Which row the item breaks: an acceptance id such as `A2`, or one of `write` (a change outside the write list), `verify` (a check that failed or was skipped) or `classify` (the finding is in the wrong class). | The worker who reworks the Job. |
| `must_fix.at` | Where it breaks, as `path:line` or other free text. | The worker who reworks the Job. |
| `found` | The ids of `found` posts the Job wrote, as 26-character ULIDs. | A person reading the Feed. The fold does not read it. |
| `proposed_jobs` | Jobs the worker suggests. A proposal is not a Job. | The fold copies the list onto the Job (`fold.go:358`), and the orchestrator turns a proposal into a Job only when it fits (`internal/orchestrator/events.go:124-175`). |
| `proposed_jobs.title` | The title the new Job would have. | The orchestrator and the lead. |
| `proposed_jobs.role` | The role the new Job would have. | The orchestrator and the lead. |
| `proposed_jobs.why` | Why, in 8 to 160 characters. | The lead. |
| `proposed_jobs.gap` | The gap in the plan the proposal closes. | The lead. |
| `proposed_jobs.discovered_from` | The Job the worker found the need in, as `J-n`. | The orchestrator, as the new Job's `discovered_from` edge. |
| `decisions_needed` | Questions only a person or the lead can answer, one string each. | The fold: a non-empty list sets the Job's `NeedsLead` flag (`fold.go:359`, `:361`), and the orchestrator does not widen the plan while it is set (`events.go:148`). |
| `knowledge` | Facts worth keeping for the project, one string each. | A person and the brain keeper. The fold does not read it. |

Five enums sit in the body. `acceptance.state` takes `met`, `not met`, `dropped` or
`waiting`. `acceptance.waits_on.kind` takes `job`, `landed`, `question` or `permission`.
`verified.level` takes `L1`, `L2` or `L3`. `verified.by` takes `sonnet` or
`grid-reviewer`. `verified.pass` takes a boolean, or `pass` or `unsure`.

### The ref form

The `ref` form is the same as for `found`: a result too large for one Feed line is
stored as `bodies/<ulid>.json` with the ulid of the post, the line holds only the
`ref` (`internal/feed/store.go:384-393`), the reader loads and checks it
(`internal/feed/read.go:67-88`), and the fold holds a `ref` body until it is resolved
(`internal/contract/fold.go:117-120`). The fold reads `proposed_jobs`,
`decisions_needed` and `verified` from the resolved body (`fold.go:351-361`).

```json
{
  "outcome": "ACCEPTED\nRows: A1 met, A2 dropped",
  "changed": [
    "contract/docs/found-and-result.md"
  ],
  "evidence": [
    "go test -count=1 ./contract/ exit 0",
    "contract/docs/found-and-result.md:1"
  ],
  "acceptance": [
    {
      "id": "A1",
      "state": "met"
    },
    {
      "id": "A2",
      "state": "dropped",
      "decision": "the Job's out-of-scope list rules it out"
    }
  ],
  "verified": {
    "level": "L1",
    "pass": "pass",
    "by": "grid-reviewer",
    "job": "J-1.1",
    "evidence": [
      "go test -count=1 ./contract/ exit 0"
    ]
  },
  "must_fix": [
    {
      "row": "A2",
      "at": "contract/docs/found-and-result.md:40"
    }
  ],
  "found": [
    "01K9J000000000000000000002"
  ],
  "proposed_jobs": [
    {
      "title": "Document the ref form",
      "role": "builder",
      "why": "The ref form has no page of its own yet.",
      "gap": "No page names bodies/<ulid>.json.",
      "discovered_from": "J-1"
    }
  ],
  "decisions_needed": [
    "Should class values get a meaning?"
  ],
  "knowledge": [
    "A security found on a known Job wakes the lead."
  ]
}
```
