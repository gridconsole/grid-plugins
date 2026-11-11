# The person Job that `grid question ask` posts

`grid question ask` turns a question into a Job for a person. The Job is a job-record body built by `jobBody` (`internal/question/job.go:33`). The question itself, with its options, is a separate question post that names this Job in its `job` field. `contract/t12_person_job_test.go` captures the body that `Ask` hands to the Job port and checks this page against it.

## The record, field by field

Values are for a question `Q-<n>` asked of person `<who>`.

| Field | Value on a person Job | Why |
|---|---|---|
| `item` | `1` unless the ask gives an item number of 1 or more | The plan item the question belongs to; an item below 1 counts as 1. |
| `title` | `Q-<n>: <the question on one line>`, cut to 120 characters with a final `…` | A person scanning the Job list sees which question it is. |
| `role` | `Person` | A person works this Job, no agent (`job.RolePerson`, `internal/job/job.go:171`). |
| `kind` | `ask` | Marks the Job as the one a question posts (`question.JobKind`, `internal/question/job.go:14`). |
| `skills` | `[]` | A person needs no skill. |
| `review` | `L0` | Nobody reviews an answer. |
| `risk` | `[]` | None named. |
| `max_rounds` | `1` | One answer ends it; there is no review loop. |
| `intent` | object: `owner` is `<who>`, `item` is `questions/Q-<n>.md`, `goal` is the question text | Says who is asked and what, and points at the question file. |
| `links` | `[]` | None. |
| `keep` | `[]` | None. |
| `decisions` | `[]` | None. |
| `read` | `[]` | None. |
| `write` | `[]` | The Job changes no file. |
| `out_of_scope` | `[]` | None. |
| `acceptance` | one row: `id` `A1`, `text` `<who> answers Q-<n>`, `traces` `item` | The Job is done when the person answers. |
| `verify` | object: `kind` `file-exists`, `path` `questions/Q-<n>.md` | The question file is the read-only view of this Job; its existence is the check. |
| `budget` | object: `turns_estimate` 1, `reads_before_first_edit` 0 | Nothing for an agent to spend. |
| `on_blocked` | `ask-and-continue` | Other Jobs carry on while the person has not answered. |
| `after` | `[]` | The Job waits on no other Job. |
| `attempt` | `1` | First and only attempt. |
| `continues` | `null` | Continues no earlier Job. |
| `discovered_from` | `null` | Found by no other Job. |
| `close_reason` | `null` | Set when the Job closes. |
| `checkpoint` | `null` | None. |
| `review_rounds` | `0` | No review happens. |
| `level_raised` | `null` | The review level is not raised. |
| `worktree` | `none` | The Job touches no worktree. |
| `posts_back` | `[]` | None. |

## Who may ask

`postsJob` (`internal/question/job.go:59`) decides by the role of the writer that asks.

- `owner`, `member` and `lead` post the person Job, and then the question post that names it.
- `agent` and `cli` post no Job. The port is not called.
  - A `cli` writer's question is posted as a plain question record with no Job behind it.
  - For an `agent` writer the Feed also refuses the question post itself, with `role agent may not post person gates`, so the ask returns that error and posts nothing. The package does not post a Job the question would then leave without its question (`internal/question/job.go:63`). `TestAskWithoutJobBehindIt` (`internal/question/ask_job_test.go:80`) covers both.
- With no Job port wired the question is a plain Feed record too.

## How another Job waits on it

The person Job holds no Job in its own `after`. Only a Job that lists it carries on after the answer: put the Job id, or `gate:Q-<n>`, in that Job's `after` list (`internal/question/question.go:140`). A Job that does not list it does not wait. The Job's own `on_blocked` is `ask-and-continue`; the code that sets it is `internal/question/job.go:51`.

## How an answer closes it

When a question is answered, defaulted or withdrawn, `settleJobs` (`internal/question/job.go:80`) closes the Job of each record it decided. `settleJob` reads the last `answered`, `defaulted` or `withdrawn` event of the question and passes its text as the Job's result: the answer text, the default text, or `withdrawn: <reason>`. Only then do the Jobs that list this Job in their `after` carry on. A repeated command closes again, so a crash between the two posts is repaired by the retry. A question with no Job (an agent's, a `cli` writer's, or one added with `Add`) makes no close call.

## The choices stay on the question post

A Job record has no field for choices. The options, the recommended option, the default and the blocking flag stay on the question post, which names this Job in its `job` field. The Job holds who is asked and what; read the question post for the options.
