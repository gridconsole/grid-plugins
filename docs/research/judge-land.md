# Judge and land in the plugin

Lane P-judge-land-decide. Research only; nothing is built here. Evidence names a
file and line in this repo, in the app repo on `main` (prefix `app:`), or in the
landing script `land.sh` the grid uses today (prefix `land.sh:`). "Design" is the
owner's run-jobs design brief and its Addenda 3 to 5 and 7; "Addendum 6" is the
brain-obsidian brief's Addendum 6 (no task lease, `attempt` in the post header).
Lane P-judge-land-a7 folds in Addendum 7 (17 Nov: production actions, and Job
code on hidden refs) and the ruling of 17 Nov 02:50 (section "Addendum 7").

**Ledger rows this doc owns (decision gate).**

The "Ledger" column quotes the decision ledger of 17 Nov. "Proved by" names its
test-plan tracks.

| Row | Ledger | Proved by | Decided in |
|---|---|---|---|
| V11 | Judgement without the app: review, one rework, then a person | D | Decision note 1-3, Q1, Q2, Q6 lanes 2, 3, 6 and 10 |
| V12 | Landing in the plugin: conflicts, red and flaky tests | D | Decision note 4-6, Q3, Q4, Q5, Q6 lanes 1, 4, 5, 6 and 10 |
| V26 | Production actions: workflow approval, command Job, exact allow rule, blocked post | D, G | Decision note 7, Addendum 7 A, Q6 lanes 9 and 10 (lane 8 merged into the sibling's lane 15) |
| V27 | Job code on hidden git refs, verified before it merges, readable from other machines | D, E, M | Decision note 8, Addendum 7 B, Q3 (redone), Q6 lanes 1, 2, 3, 4, 5 and 10 (lane 7 merged into the sibling's lane 14) |

V26 and V27 are shared with P-run-jobs-decide. One decision, with one owner lane
and one set of names per shared point, is in docs/research/run-jobs-launcher.md,
section "C. Shared with judge-land". This doc owns the integrate, land and
cleanup verbs (lane 4) and the deploy approval, deploy command Job and `wait`
verb (lane 9). It points to the sibling for `internal/gitref`, the secrets
check, submit and the permissions (its lanes 14 and 15, which took this doc's
lanes 7 and 8).

## Decision note

**Decided.**

1. Judgement leaves the app. No long-running judge process takes its place, in
   Go or as an LLM. The launcher (`grid loop`) already moves a Job that exited
   `done` to `submitted` with "awaiting judgement" (`internal/orchestrator/tick.go:330`)
   and prints the wake (`internal/orchestrator/stream.go:353`). From there the
   home orchestrator decides. It posts a reviewer Job. On the verdict it posts
   exactly one of three things:
   - an integrate Job and the land Job that waits on it (Addendum 7 B.7);
   - one rework Job that carries the must-fix items;
   - a person Job, after a second failed review or a STUCK verdict.
2. The mechanical parts of the app Judge become plain Go verbs. The orchestrator
   calls them, so it never writes these bodies by hand:
   - `grid judge verdict` reads a review;
   - `grid judge review`, `grid judge rework` and `grid judge person` post the
     review, rework and person Jobs in the exact shape the app posts today.

   They are one-shot commands. They decide nothing, hold no loop, and add no cap
   (the "Correction" in the design brief).
3. The number of rework rounds comes from the workflow's required `review rounds`
   (`internal/workflow/parse.go:248-271`). When a project names no workflow, it
   comes from a default workflow note in the home brain (`review rounds: 2`, which
   means one review, one rework and one more review). It does not come from a Go
   constant like `app: internal/kernel/worker/judge.go:35` (`defaultReviewRounds = 2`).
4. Landing is one land Job per accepted change. The Job is `kind: land` with the
   fields from P-land-kind-2. Under Addendum 7 B the change is first squashed
   onto the task head `refs/grid/<T>/head` by an integrate Job (decision 8). The
   land Job runs `grid land change`, which:
   - rebases the task head onto main;
   - resolves trivial conflicts only;
   - runs the checks;
   - bisects a red test within the change's own commits;
   - lands a flaky test with a flake note;
   - pushes main and the task head together with `git push --atomic
     --force-with-lease`;
   - posts `landed` with the new main SHA and the head SHA.

   Order within a repo uses the start condition "after J-7 lands". The new code
   goes in `internal/land` (Addendum 3's word), in new files beside the deploy
   executor (`internal/land/land.go:1`), which stays as it is. Deploy stays the
   existing `grid land` step (`internal/land/land.go:208`).
5. **Open for Abdu (V12.c).** The plan below is written for option A: the test
   run is the land Job itself. The land Job carries the start condition "no
   other test run on this machine" and runs the checks on the rebased SHA that
   it then pushes (Q4). Option B, the owner's wording (a separate runner Job that
   the land Job waits on), and what it changes are in section "Open for Abdu:
   V12.c". This is not settled here.
6. The launcher retries a lost push race mechanically, as a new attempt of the
   same land Job. Posts carry `attempt`, not a lease epoch (Addendum 6).
   - The app's `no_epoch` failure (`app: judge.go`, `epoch`) goes away.
   - So does the `--lease-epoch` flag on its posts (`app: judge.go:962-979`).
7. **Production actions (V26, Addendum 7 A).**
   - **Mode.** The launcher starts every Claude Job with `--permission-mode
     dontAsk`, never auto or bypassPermissions. The role tools plus the
     attempt's exact `allow` rules go in `--allowedTools`.
   - **Grants.** A rule is used only when the attempt's `grant` (an approval
     record: an `approval` post or an approve Job's accepted control, by a
     person or by an agent under a grant note) covers it.
   - **Deploys.** A deploy is a workflow `approve` step (a person Job) followed
     by a `kind: command` Job, which runs `grid land T-n` with no agent, and then
     a `kind: wait` Job.
   - **Denied mid-Job.** An agent that is denied mid-Job posts
     `blocked {need: permission}` and stops. An approval gives one new attempt
     with the rule.
8. **Job code on hidden refs (V27, Addendum 7 B).**
   - **Refs.** An attempt pushes its commits to `refs/grid/<T>/jobs/<J>/a<n>`
     after a secrets check of every commit. The attempt runs in a detached
     worktree, with no branch.
   - **Integrate.** An integrate command Job squashes each accepted Job onto
     `refs/grid/<T>/head`.
   - **Land.** The land Job moves main and the head together.
   - **Cleanup.** Land and task close delete the refs, and every machine prunes
     with the refspec `+refs/grid/*:refs/grid-remotes/origin/*`.
   - **What it replaces.** The runner's and app's per-Job branches, their
     dependency merges, and land.sh's lane branches.

**Why.** The design gives the orchestrator every decision and the launcher every
mechanical act (design brief, Addendum 3: "judgement and landing move to the
plugin as Jobs ... no new long-running LLM"). The app Judge mixes both kinds of
work:
- a few real decisions: carry on, review, rework or ask a person;
- many fixed rules: body shape, a request id, one acknowledgement per wake, verdict
  parsing.

Moving the fixed rules into Go verbs keeps them testable, and the ported app tests
stay meaningful. The decisions then sit where the design puts them. The owner's
acceptance runs with the app stopped, so nothing on the path may live in the app.

**Rejected.**
- *A Go `internal/judge` stream pass inside the launcher that posts reviews and
  reworks on its own* (dogfood-gaps G3 / lane 7, `docs/research/dogfood-gaps.md`).
  That puts decisions in the launcher, against the Correction.
- *App L5 and L6: the app Judge posts the land Job and the app shows it*
  (`docs/research/land-job-type.md`, section 6). The acceptance requires the app
  not running.
- *A new package `internal/landjob`* (land-job-type section 6). Addendum 3 names
  `internal/land`.
- *Merge plus speculative stacking (option B) and update-ref CAS on a local
  clone* (land-job-type section 3). The owner chose rebase onto main, order by
  "after J lands", and a push with lease.
- *Batches, MAXB, TRAIN-DROP, flock* (`land.sh:29-31`, `land.sh:83`,
  `land.sh:88-98`). One land Job per change makes them unneeded. Order becomes an
  after condition, and the lock becomes the lease on the push.
- *(Open for Abdu, V12.c: not rejected until he answers.)* *A separate
  test-runner Job beside the land Job.* Addendum 3 asks for a test
  runner Job with "no other test run on this machine". A second Job would test
  one SHA, and the land Job would then push a SHA rebased again after main moved.
  That SHA would be untested, unless the land Job waited on the runner and passed
  the SHA on, which is two Jobs doing the work of one. One runner-role land Job
  carrying that start condition meets the Addendum's condition and keeps "tested
  SHA = pushed SHA".
- *A hard-coded STUCK path in Go.* STUCK is a verdict word the verdict verb
  reports. Posting the person Job stays the orchestrator's choice.
- *Auto mode, bypassPermissions, wildcard allow rules, or a Grid hook that blocks
  production commands* (Addendum 7 A). The CLI check of 17 Nov shows that auto
  mode ran the production command both with the rule and without it, so it is
  no gate.
- *Pushed `grid/...` branches, a `refs/remotes/origin/grid/*` mapping, landing
  attempt refs with no task head, and a secrets check of the tip tree only*
  (Addendum 7 B; reasons in that section).

## Q1. The app Judge today, and where each part goes

Source: `app: internal/kernel/worker/judge.go` and
`app: internal/kernel/verdict/verdict.go` on app `main`.

### What it posts

Every post goes through `grid job post --task --type --job --id --lease-epoch
--body --json [--wake] [--person]` (`judge.go:962-979`). The `--id` is a ULID made
from the head of a SHA-256 over the length-prefixed task, type, job, body, wake
and person (`judge.go:949-960`, `requestID`). "wake write already exists" counts
as success (`judge.go:962-979`). The plugin's error for it is
`internal/orchestrator/write.go:19`, `ErrDuplicateWake`.

| Post | When | Body | Lines |
|---|---|---|---|
| control, no status | a wake that needs no action (open review, rounds used, nothing to judge) | `{target, reason: why+" (wake W)"}` | `judge.go:394-408` |
| control `accepted` on the builder | ACCEPTED verdict; workflow carries on; curator | reason "review accepted by R", "workflow carries on: ...", "curator asks no person" | `judge.go:490-511`, `658-735`, `1003-1083` |
| control `accepted` on the review | after every verdict | reason "verdict X", with the wake | `judge.go:1003-1083` |
| job post: review child | the next step is a check | id `J-n.k` (`childID`, `judge.go:920`), role reviewer (checker for grid-checker), `discovered_from` builder, `verify {kind: reviewer-check, job}`, A1 "the result of J meets its acceptance rows: ..." traces item, `attempt 1`, `review_rounds 0`, review level dropped | `judge.go:658-735` |
| job post: rework | REJECTED with rounds left | new top id (`nextTopID`), body copied without status/created_at/order, plus row "Must-fix from review round N:\n- ..." traces owner, `continues` builder, `discovered_from` review, `attempt 1`, `review_rounds` = reviews so far | `judge.go:1115-1142` |
| control `rejected` on the builder | with the rework | `close_reason` "reworked as J-n" | `judge.go:1115-1142` |
| job post: person (`--person owner`) | person step next; second REJECTED; cancelled review | role Person, kind ask, item and intent from the reviewed Job, read `intent.md#request`, verify file-exists `intent.md`, A1 "<owner> decides how J continues" traces owner, `on_blocked ask-and-continue`, `posts_back [result]` | `judge.go:762-787`, `839-862`, `898-917` |

### What it logs

These events are logged once per wake, keyed "task event/n" (`judge.go:243-315`):

- `judgement_failed`, with one of these reasons:
  - `post_failed`
  - `no_epoch`
  - `workflow_unreadable`
  - `verdict_unreadable`
  - `fold_late`
  - `failed`
- `judgement_wake_unsupported`
- `judgement_nothing_to_judge`
- `judgement_person_job_waiting`
- `judgement_carry_on`
- `judgement_default_flow`, once per task
- `judgement_review_running`
- `judgement_review_cancelled`
- `judgement_rounds_used`
- `judgement_person_point`
- `judgement_review_started`
- `judgement_verdict_late`
- `judgement_accepted`
- `judgement_rework`
- `judgement_person_job`
- `judgement_verdict_posted`

### Its rules

| # | Rule | Lines |
|---|---|---|
| R1 | Each wake is handled once and answered by exactly one record that carries it. | `judge.go:243-315` |
| R2 | Every submitted Job of the task is judged. The wake's own Job is always answered. | `judge.go:331-388` |
| R3 | A Job still assigned or running gets no answer and is retried (`errNotSubmittedYet`, 15 s retry). Any other status that is not submitted gets a status-less answer. | `judge.go:331-388` |
| R4 | A submitted person Job waits. A reviewer or checker Job goes to the verdict. A curator Job is accepted. Anything else goes to the hand. | `judge.go:490-511` |
| R5 | The workflow is read from the task's project. With no workflow, the default flow is Run, then Check by grid-reviewer with 2 rounds. A missing named workflow or a broken file fails as `workflow_unreadable`. Rounds below 1 mean 2. | `judge.go:529-618` |
| R6 | The step after the builder's run step decides what happens. A step of a kind other than run, schedule or check is a person step. | `judge.go:621-638` |
| R7 | The hand checks four things in order:<br>1. A review is open: answer and wait.<br>2. A review was cancelled before it ran: one person Job (`reviewLost`).<br>3. The rounds are used: answer.<br>4. Otherwise, by the next step: none, run or schedule means accept; a person step means a person point; check means a review child. | `judge.go:658-735` |
| R8 | A person point and a cancelled-review person Job are each raised once. | `judge.go:762-787`, `839-862` |
| R9 | The verdict is the first line of `outcome`, else `verdict`, `text`, `result` or `summary`, trimmed of ".:*# ", and must be ACCEPTED or REJECTED. Must-fix comes from strings or `{row, at}` objects, else from the lines under a "Must-fix:" heading. | `app: verdict.go` (Parse) |
| R10 | A verdict on a builder that is no longer submitted is accepted as late. | `judge.go:1003-1083` |
| R11 | ACCEPTED with a person step after check gives the approve point. Otherwise the builder is accepted. | `judge.go:1003-1083` |
| R12 | REJECTED with rounds left gives the rework. REJECTED with none left gives a person Job ("rejected again after its rework. The reviewer's recommendation: ..."). | `judge.go:1003-1083` |
| R13 | Rounds are counted over the rework chain (`continues` links). | `judge.go` `chainOf`, `rootOf`, `roundsOf` |
| R14 | There is no STUCK verdict. STUCK exists only in the dispatcher (`dispatch.sh:17`, `:29`). The plugin runner's "ended done with no result: stuck for the lead" comes from `no_result` on lane-P-run-jobs (`internal/runner/runner.go:613`, commit 43d26ad5). | — |

### The named tests

- **`judge_rework_newjob_test.go`**, `TestReworkPostsANewJobIDWithOnlyJobRecordKeys`:
  - the rework id is new (not J-1 or J-2);
  - the body holds only job-record keys;
  - `continues` is J-1;
  - the must-fix row has the keys id, text, traces and state, with id `A<n>`.
- **`judge_stall_test.go`**, rows 1–5. Each is ported by a row of lane 3 (Q6):
  - `TestJudgeStallFreshHome`: lane 3 row 8 (ported with a change: the default
    is the note, not Go).
  - `TestJudgeReviewsWhenThePostedReviewerWaitsOnTheBuilder`: lane 3 row 9.
  - `TestJudgeOrchestratorJobIsJudgedLikeABuilder`: lane 3 row 10.
  - `TestJudgeDefaultFlowNeverBeatsAWorkflow`: lane 3 row 11.
  - `TestJudgeAcknowledgesEveryWake`: lane 3 row 4. Every path gives exactly one record that
    carries the wake and one log line. The cases are:
    - review started;
    - started review still open;
    - rounds used;
    - person point, on the first and the second wake;
    - late verdict;
    - submitted person Job;
    - Job not submitted, or missing;
    - a second submitted Job gets no record;
    - duplicate wake.
- **`judge_post_request_id_test.go`**:
  - `TestJudgePostRequestIDFollowsThePost`
  - `TestJudgeControlCarriesARequestID`
- **The cancelled review**, `judge_cancelled_review_test.go`:
  - `TestJudgeReviewCancelledAtSpawnRaisesOnePersonJob`: the reason is the
    worker-exit code, and the choice is "accept J without a review, or ask for
    rework".
  - `TestJudgeReviewCancelledAfterAVerdictRaisesNoPersonJob`

### Where each part goes

| App behaviour | Plugin home | Kind |
|---|---|---|
| exit `done` gives submitted "awaiting judgement" | launcher, already: `internal/orchestrator/tick.go:330` | launcher post (exists) |
| wake on the result | launcher, already: `internal/orchestrator/stream.go:353`, candidates `internal/orchestrator/events.go:193-334` | launcher wake (exists) |
| R1 one record per wake | orchestrator acks every wake. The `grid judge` verbs take `--wake` and treat `ErrDuplicateWake` (`write.go:19`) as done. | Go verb rule |
| request id (`judge.go:949-960`) | the `grid judge` verbs derive `--id` the same way, minus the epoch | Go verb rule |
| R2–R4, R6, R7 decision of what comes next | orchestrator agent (`agents/orchestrator.md`, the `never_judged` text at `agents/orchestrator.md:28` is replaced by the judgement flow) | orchestrator decision |
| R5 workflow and default flow | `grid workflow show` (exists). The default comes from a home brain workflow note, not code. | brain note |
| R9 verdict parse (+ STUCK) | `grid judge verdict <review>`, port of `app: verdict.go` | Go verb |
| review, rework and person bodies | `grid judge review`, `grid judge rework`, `grid judge person` | Go verbs, posted by the orchestrator |
| R8 raise once | the verb's request id plus `job_exists` (`CodeJobExists`, `internal/cli/error_codes.go:46`) | Go verb rule |
| R10 late verdict, R11 approve point, R12 second fail | orchestrator decision, documented in the agent text | orchestrator decision |
| R13 rounds over the chain | `grid judge verdict` reports `rounds_used` and `rounds` | Go verb |
| curator carry-on | orchestrator accepts a curator result (agent text) | orchestrator decision |
| lease epoch, `no_epoch` | dropped; `attempt` (Addendum 6, `internal/contract/types.go:81`) | dropped |
| log events `judgement_*` | dropped as log lines. Each decision is a post in the Feed, which is the record. | dropped |
| `judgement_failed` reasons | become verb exit codes the orchestrator reads | Go verb |

## Q2. The judgement flow

```
builder J exits done (worker-exit, attempt a)
  launcher: control J submitted "awaiting judgement"            tick.go:330
  launcher: wake result J/a to the home orchestrator             stream.go:353
orchestrator: grid workflow show -> next step after run
  check  -> grid judge review J --wake result/J/a   posts J.1 (reviewer)
  person -> grid judge person J --point S --wake ...
  none   -> control J accepted "workflow carries on" (+ integrate Job I and land Job L if J changed a repo)
reviewer J.1 exits done
  launcher: control J.1 submitted; wake result J.1/1
orchestrator: grid judge verdict J.1 --json
  ACCEPTED                      -> control J accepted; control J.1 accepted "verdict ACCEPTED";
                                   post integrate Job I and land Job L after I (Q3, Addendum 7 B.7)
  REJECTED, rounds_used < rounds -> grid judge rework J --from J.1   posts J' (continues J),
                                   control J rejected "reworked as J'"
  REJECTED, rounds_used = rounds -> grid judge person J --from J.1 "rejected again after its rework"
  STUCK or no verdict line       -> grid judge person J --from J.1 "stuck: <reviewer's reason>"
rework J' exits done -> back to the top (the review of J' counts round 2)
```

**"One rework" is configuration.** It comes from the brain workflow, not from
code:
- the workflow's required `review rounds` (`internal/workflow/parse.go:248-271`,
  values 1, 2, 3 or 5);
- or the home brain's default workflow note.

`review rounds: 2` means one review, one rework, then a review whose REJECTED goes
to a person. That is the dispatcher's rule (`dispatch.sh:16-29`: a second REJECTED
and STUCK go to the orchestrator, and a fix round appends the must-fix) and the
app's default (`judge.go:35`). This plan moves it into a brain note. Code holds no
default; a project with no workflow and no note gets a person Job asking for one.

**Attempts, not leases (Addendum 6).**
- Every post the orchestrator makes through `grid judge` names the Job and the
  attempt it judges (`Post.Attempt`, `internal/contract/types.go:81`).
- A verdict from an older attempt of the reviewer is ignored.
- A reassigned reviewer gets a new attempt, and its own exit wakes again.
- No `--lease-epoch`.

**Posts and fields.**

| Post | By | Type | Fields |
|---|---|---|---|
| awaiting judgement | launcher | control | `target J, status submitted, reason "awaiting judgement"`, header `attempt a` |
| wake | launcher | stream wake line (`stream.go:37`) | `result J/a` |
| review Job | orchestrator via `grid judge review` | job | as in the app's review child (Q1), plus header `wake result/J/a`; `attempt 1` |
| verdict read | `grid judge verdict` (no post) | — | JSON: `verdict` (ACCEPTED, REJECTED or STUCK), `must_fix[]`, `review`, `review_attempt`, `rounds_used`, `rounds` |
| accept | orchestrator | control | `target J, status accepted, reason "review accepted by J.1"`, wake |
| integrate Job | orchestrator | job, `kind: command`, runs `grid task integrate T J` | Addendum 7 B.7 |
| land Job | orchestrator | job, after the integrate Job | Q3 |
| rework Job | orchestrator via `grid judge rework` | job | new id, `continues J`, `discovered_from J.1`, copied body, row "Must-fix from review round N" (keys id, text, traces, state), `review_rounds N`, `attempt 1` |
| reject | `grid judge rework` | control | `target J, status rejected, close_reason "reworked as J'"` |
| person Job | orchestrator via `grid judge person` | job, `--person owner` | as in the app's `personBody` (Q1), with the reason: second fail with the reviewer's text, STUCK, or a review cancelled before it ran |

## Q3. The land Job

**Redone on hidden refs (ruling of 17 Nov 02:50, Addendum 7 B).** Code no longer
moves on branches. Accepted work is first squashed onto the task head
`refs/grid/<T>/head` by an integrate Job, and the land Job takes that head to
the target. Steps 1, 2, 6 and 7 below therefore work on the task head, as set out
in "Addendum 7", B.7 to B.9. Step 8 (deploy) now runs as a command Job after the
workflow's approve step (Addendum 7, A.2 to A.4). The rest of this section stands.

**One per accepted change.** The orchestrator posts it in the same turn that
accepts J, and only when J changed a repo (J's `submit` names a ref and a SHA,
Addendum 7 B.4). The land Job waits on J's integrate Job. The Job record uses
the fields of P-land-kind-2 (commit aa0cf14a on `lane-P-land-kind`): `kind: land`,
plus `repo`, `target`, `branch` and `sha`. Under Addendum 7, `branch` holds the
full ref `refs/grid/<T>/head` and `sha` holds the head SHA that the integrate
posted. Lint enforces the fields through the rules `job.land-target`,
`job.land-branch` and `job.land-sha`. They check only that each field is
non-empty (`internal/job/lint/lint.go:360-361` on aa0cf14a), so a ref name
passes.
- Role: runner.
- Spawn: a plugin verb, `grid land change`, with no agent (land-job-type section
  3).

**Order per repo.** Each land Job carries the start condition "after L-7 lands".
L-7 is the land Job of the change accepted before it in the same repo. The
launcher evaluates it as an `after` edge of kind `done` on L-7
(`internal/job/lint/lint.go:252`: done, failed, children-done). A land Job is
accepted only when it posted `landed`, so "done" means landed.

Edges today stay inside one Feed. Ordering land Jobs from two tasks on one repo
needs the launcher's cross-task start condition. That condition is the sibling
lane's interface (P-run-jobs-decide). Until it exists, the push with lease below
still keeps main correct; only the order between tasks is not fixed.

**Steps of `grid land change`.**

1. Fetch. Create a worktree at `origin/<target>` under the runner's worktree
   layout (Home/worktrees/<task>/<job>-a<n>, lane-P-run-jobs `internal/runner/runner.go`).
2. Rebase the task head (`branch`, which is `refs/grid/<T>/head`) onto
   `origin/<target>`. Rebase is the owner's word. The script merges
   (`land.sh:41-55`), and land-job-type section 3 merges too.
3. **Trivial conflicts are resolved, and only these:**
   - **A generated file.** The file matches the brain note's generated pattern
     (`land.sh:36`). Take main's side, rerun the note's regenerate command (the
     script's `make ops`, `land.sh:41-55`), and continue.
   - **A union file.** The file is on the brain note's union list (the script's
     ui-seed, `land.sh:79`). Use a union merge, then run the note's check (the
     script's `node --check`).

   Anything else: abort the rebase. Post the result `conflict` with the files,
   and the Job fails. The orchestrator then posts a rework Job, "rebase J onto
   main; conflicts in ...", with `continues J`.
4. **No change.** If the rebased tree equals the target's tree, accept with
   "nothing to land" (land-job-type section 3; `land.sh:84`, "already").
5. Run the checks (Q4). A red result posts `red` with the commit and the test;
   the orchestrator posts a rework with that must-fix item. A flaky result goes
   on to step 6 with a flake note.
6. Push with `git push --atomic --force-with-lease=refs/heads/<target>:<base sha>
   origin <rebased sha>:refs/heads/<target>`. This replaces the update-ref CAS of
   `land.sh:467-469` ("main moved").
   - A lost lease is not a failure of the change. The verb exits with the code
     `push_lease_lost`.
   - The launcher then starts a new attempt of the same land Job, which fetches
     and rebases again. This is mechanical, as Addendum 3 says. The launcher
     makes no decision and has no cap.
   - Every attempt shows in `grid job tree`.
7. Post the result `landed` with `main_sha` (the pushed SHA), `base_sha`, `commits`
   and any `flakes[]`. The home launcher, the only writer of the task.md status
   block (Addendum 4), rewrites the block from the fold: "J landed <sha>".
8. **Deploy.** If a deploy note names this repo and target, the existing `grid land`
   deploy step runs (`internal/land/land.go:117` rule, `:208` Run, `:288` history
   note). Its code does not change. Addendum 7 A changes how it is started:
   - after the workflow's approve step;
   - as a command Job with no agent;
   - followed by a `wait` Job.

   The push in step 6 also moves the task head, and the land deletes the attempt
   refs of the Jobs it landed (Addendum 7 B.8, B.9).

## Q4. Tests

**The runner condition** (option A of "Open for Abdu: V12.c"; option B moves it
to a separate runner Job). The land Job carries a second start condition: "no
other test run on this machine". The launcher evaluates it from the machine note
(grid/machines/<m>.md, design brief), and the land Job holds it while its checks
run. The checks run on the rebased SHA that the push sends, so a land Job is both
the land step and the test run. The script's `reserve-cpu` and flock
(`land.sh:29-31`) go away.

**Checks.** The commands come from the brain environments note (land-job-type
section 3), not from code:
- for the plugin: `go test ./...`, the Windows and Darwin builds, and lint;
- for the app: ops-check, webui-test and test-lib (`land.sh:102-109`, `:123`,
  `:125-127`).

**Red.**
1. Rerun the failing packages alone, as the script does (`land.sh:110-119`). If
   they pass, go to flake.
2. If still red, run the failing test on the base (main) 3 times alone. Red 3 of 3
   means the test was already red on main (MAIN-FAULT, then PREEXISTING,
   `land.sh:211`, `land.sh:245-248`). This does not block the change, as in the
   script. The change lands, with a `preexisting` note, and the orchestrator posts
   a fix Job for main.
3. Otherwise bisect within the change's own commits. Run `git bisect run <the one
   failing test>` between the base and the rebased head. The first red commit and
   the test go into the `red` result. The script's lane and prefix bisect
   (`land.sh:197-220`, `:333-348`) goes away because batches are gone.

**Never waivable.** A build failure, a panic or a timeout is red at once
(`land.sh:121`, `testbad`). Only Go unit tests and real-plugin tests may be called
flaky (`land.sh:351-356`).

**Flaky.**
1. Rerun the whole package 3 times on the same SHA (`land.sh:308-311`).
   - 2 or 3 green: flaky (`land.sh:315`). The change lands with the flake note
     `{test, package, green: n/3}`.
   - 1 of 3 (FLAKE-UNSURE, `land.sh:316`) or 0 of 3: red, and it is bisected as
     above.
2. For each flake, the orchestrator posts a fix-the-cause Job, unless an open Job
   already names the test (`land.sh:272-295`, `waive_brief`). The fix Job's
   acceptance rows keep the script's words: "never delete the test", "no retry
   loop, no t.Skip" (`land.sh:291-292`).
3. If the same test is flaky or red again on a later land, the orchestrator posts a
   fix Job that deletes the test and names the test that covers the same
   behaviour. The exception is a test that pins a past incident: its doc comment
   or a brain decisions note names an incident. Such a test is fixed and never
   deleted, and the fix Job's acceptance row says so.

**land.sh rules that become Go in `internal/land`.** All are new files; the
deploy executor `land.go` is untouched.

| Rule | land.sh | Go file | Test |
|---|---|---|---|
| branch exists and differs from target | `:25` | `change.go` | temp repo: missing branch gives an error code |
| ATTR: an attribution line in a change commit is reported, not blocked (the script prints and logs, `:26-27`) | `:26-27` | `change.go` (the `landed` or `conflict` result lists `attribution` commits) | temp repo: a commit with a Co-Authored-By trailer is named in the result, and the land still goes ahead |
| union attributes for the note's union files, then the note's check | `:79` | `conflict.go` | temp repo: union file conflict resolved, check command run |
| generated file takes main's side, then regenerate | `:36`, `:41-55` | `conflict.go` | temp repo: generated conflict resolved, regenerate called once |
| other conflict gives `conflict` | `:41-55` (returns 3) | `conflict.go` | temp repo: real conflict gives result `conflict` with files |
| nothing to land | `:84` | `change.go` | temp repo: tree equal gives accepted "nothing to land" |
| checks from the note | `:102-109`, `:123-127` | `check.go` | fake commands: order and exit codes |
| rerun failing packages | `:110-119` | `check.go` | fake `go test` red then green |
| build, panic and timeout are not waivable | `:121`, `:351-356` | `check.go` | panic output gives red, no flake |
| red on main 3 of 3 gives preexisting | `:211`, `:245-248` | `bisect.go` | fake runner red on base gives `preexisting` |
| bisect within the change's commits | replaces `:197-220`, `:333-348` | `bisect.go` | temp repo with 3 commits, the 2nd breaks a test: the result names commit 2 and the test |
| flake: whole package 3 times, 2 of 3 green | `:300-316` | `flake.go` | fake runner 2 of 3 gives `landed` + flake; 1 of 3 gives red |
| one fix Job per test | `:272-295` | `flake.go` (finds open Jobs naming the test) | an open Job names it: no second proposal |
| push with lease (was update-ref CAS) | `:467-469` | `push.go` | bare remote moved between fetch and push gives `push_lease_lost`; untouched remote gives `landed <sha>` |
| **ported: drain** (design :70, V12.f). The word "drain" is not in land.sh, landpair.sh or the 16-17 Nov landing notes, so this is a reading, and Abdu confirms it (open items). Read as: the queue keeps draining past a failed change, so one red or conflicting change does not hold back the rest. That is the 17 Nov owner order in the land.sh header ("one red test no longer blocks the batch", landing-batches asks 2 and 3), done by the eject-and-rebuild of the reduced batch in the same run and by "landing with the rest green" for a waived test. With one land Job per change there is no batch to rebuild: the failed change's land Job ends `conflict` or `red`, and the orchestrator posts its rework. One rule is new: a `done` edge on a failed land Job never releases (`internal/jobgraph/graph.go:367-369`), so the orchestrator also re-posts each land Job of another task that waited on the failed one, with its edge moved to the last land Job still in flight (or none), and cancels the old one. The drain holds across tasks only: a land Job lands its task's whole `refs/grid/<T>/head` (B.8), so a later land of the same task still holds the red change and waits until the rework is integrated | `:11-16`, `:373`, `:444-463` | none (orchestrator) | lane 6 row 8 |
| not a land.sh rule: the output drain of the deploy executor (`internal/land/process.go:69-71`, `:101-108`, reads the deploy command's output to EOF; from 11 Nov) | — | `process.go`, kept as it is | its existing tests |
| dropped: flock and reserve-cpu (replaced by the push lease and the machine condition) | `:29-31` | — | — |
| dropped: cross-floor, MAXB, TRAIN-DROP (one change per Job; order by after edges) | `:59-70`, `:83`, `:88-98` | — | — |
| dropped: blame, msgblame, eject (lane blame inside a batch; a single change needs no blame) | `:133-164` | — | — |
| dropped: try_waive and the attempts 2–3 loop with FLAKE-RERUN (replaced by the flake rule above and a new attempt of the land Job) | `:360-374`, `:432-464` (FLAKE-RERUN at `:441`) | — | — |
| dropped: the STATE-UI row (a file outside the repo; the status block replaces it) | `:471` | — | — |

## Q5. Reconciling R-land-job-type L1–L6 and P-land-kind-2

The L1–L6 rows are at `docs/research/land-job-type.md:207-213`. The naming choice
is at `:203`, the CPU section at `:154`, and the plan that sends the app Judge the
land Job is at `:192`.

| Lane (land-job-type section 6) | Verdict | What changes |
|---|---|---|
| L1-land-kind (`land-job-type.md:207`) | **stays** | This is P-land-kind-2, commit aa0cf14a on `lane-P-land-kind`. It adds `target`, `branch` and `sha` on `kind: land` in `contract/schemas/job-record.json` and `internal/job/job.go`, the lint rules `job.land-target`, `job.land-branch` and `job.land-sha` in `internal/job/lint/lint.go`, and the tests `internal/job/lint/land_kind_test.go` and `internal/cli/feed_land_roundtrip_test.go`. The rebase flow needs no other field. Under Addendum 7 B, `branch` holds the full ref `refs/grid/<T>/head`, which the non-empty lint accepts (`lint.go:360-361` on aa0cf14a). |
| L2-land-change (`:208`, naming at `:203`) | **stays, changed** | The package is `internal/land`, not `internal/landjob`. The flow is rebase, not merge. The push uses a lease (no local update-ref CAS). Stacking (option B) is dropped. It adds the result codes `landed`, `conflict`, `red`, `preexisting` and `push_lease_lost`. |
| L3-land-red (`:209`) | **stays, changed** | Red is bisected within the change's commits, not across lanes. The flake rule has the owner's second-time delete and the incident exception. |
| L4-land-step (`:210`; workflow step `land`, spawn `command`) | **split** | The workflow step kind is dropped: the orchestrator posts the integrate Job and the land Job after ACCEPTED, and no workflow step is needed. The `command` spawn (a runner Job that runs a plugin verb with no agent) moves to the launcher lanes of P-run-jobs-decide. |
| L5-app-land (`:211`) | **dropped from the app** | It moves to the plugin as `grid judge` and the orchestrator's text (lanes 2 and 3 below), as dogfood-gaps G8 already found. |
| L6-feed-land-ui (`:212`, app) | **dropped** | The task.md status block shows "landed <sha>" (Addendum 4). The app is not on the path. |
| agent nice +10 (`:154`, section 4) | **not in this plan** | It is a capacity setting, which is the launcher's measuring side. Left to P-run-jobs-decide. |
| Brain notes (`:213`) | **stays, owner named** | Lane 4 below writes the note shape that `grid land change` reads (generated pattern, regenerate command, union list and check, the check commands), with a seed note for the plugin repo. Lane 3 writes the default workflow note. |

**P-land-kind-2 continues as is.** Its rows 1–3 hold unchanged: the lint rules,
the round-trip test, and the existing tests staying green. It is the first lane
below, and nothing else depends on its internals. Its brief names sonnet high.
This plan lists every builder at xhigh, so the owner chooses whether to relaunch it
at that level.

## Addendum 7. Production actions and Job code on hidden refs

**Sources.**
- Addendum 7 of the design brief.
- The ruling of 17 Nov 02:50 in this lane's brief.
- The 7 Nov refs design (its "Git" and "Landing and deploy" sections).
- The orchestrator's check of the allow rule on the real CLI (Claude Code
  2.1.325, headless `claude -p`, 17 Nov).

**Ownership.** Ledger rows V26 and V27 are shared with P-run-jobs-decide
(docs/research/run-jobs-launcher.md, "sibling" below), whose Addendum 7 now
covers them. Its section "C. Shared with judge-land" names one owner lane and
one set of names per shared point. Where a point below is the sibling's, the
text says so and points there; this doc owns integrate, land, cleanup and the
deploy Jobs (lanes 4 and 9).

### A. Production actions (V26)

**Today.**
- **Routines fix the mode.** A routine runs `claude -p <prompt> --allowedTools
  <list> ... --permission-mode dontAsk` (`internal/routine/execute.go:192-199`).
  It is the only headless path in the plugin that fixes the permission mode.
- **The Job path leaves the mode open.** `grid run` adds `--permission-mode`
  only when its caller sets one; "Grid writes no default"
  (`internal/runner/runner.go:95`, `:391-393` on 43d26ad5). The allowed tools are
  the role tools of the spawn line (`runner.go:388-390`).
- **The app passes the person's setting.** It passes the person's
  `permissionMode` from settings.json through as written
  (`app: internal/kernel/worker/agentbuild.go:231-236`). It pre-approves
  `Bash(grid:*)`, `Bash(git:*)`, the file tools and two feed tools
  (`agentbuild.go:242-265`, `:281`).
- **The deploy step checks an approval.** `grid land` checks an approval of
  the plan hash and the lands-rule hash before it runs (`internal/land/land.go:85-88`,
  `:243-251`). It then runs the deploy command with `sh -c` (or `cmd.exe` on
  Windows) under a 10-minute default timeout (`land.go:252-277`).
- **A failed-deploy post exists.** The contract has a `deploy-failed` post with
  `env`, `stage` and `reason` (`contract/fixtures/deploy-failed/valid/basic.json`).

**Verified on the real CLI (17 Nov).** The stand-in was `make deploy-prod`.

| Mode | Allow rule | Result |
|---|---|---|
| dontAsk | `--allowedTools "Bash(make deploy-prod)"` | ran the command |
| dontAsk | none | refused, with one permission denial |
| auto | with the rule | ran |
| auto | none | ran |

Auto mode is no gate for a production action that the prompt asks for.

**Decided.**

1. **Mode.** The launcher starts every Claude Job with `--permission-mode dontAsk`.
   - `--allowedTools` is the role's tools plus the attempt's `allow` rules.
   - It never uses `auto` or `bypassPermissions`.
   - It does not pass a person's settings.json `permissionMode` to a Job.

   This changes today's Job path, which leaves the mode to the caller
   (`runner.go:95`) or the person (`app: agentbuild.go:231-236`). It adds no Grid
   gate: dontAsk and allow rules are Claude Code's own, as Addendum 7 asks
   ("use the workflow and Claude Code built-ins"), and routines already run this
   way (`execute.go:199`).

   For Codex Jobs, the matching `codex exec` approval and sandbox flags are not
   checked here (UNVERIFIED). The launcher lane that adds `codex exec` decides
   them on the same basis.
2. **Approval comes from the workflow, before the Job.** The project's deploy
   workflow note has an `approve` step before `deploy`. In one turn the
   orchestrator posts two Jobs:
   - a person Job (role Person, `kind: approve`) asking "approve deploy of
     <sha> to <env>", which the person sees in the brain inbox and the status
     block. This is the plan Job shape the plugin already reads as an approval
     (`internal/task/gates.go:244-248`, `:285`);
   - the deploy command Job, with `after: {job: <person Job>, kind: done}`
     (run-jobs-launcher.md section C, point 14).

   **The approval record exists today.** `grid approve` answers that person Job
   (`internal/land/approval.go:30-41` adds the lands-rule hash). It posts an
   accepted control carrying `plan_hash`, `plan_digest` and `lands_hash`, which
   `planJobResult` reads (`gates.go:297-306`). `land.Run` checks the plan hash
   and the lands-rule hash against it (`land.go:243-251`, via
   `ApprovalForGate`, `gates.go:143-172`). So no new record and no new check are
   needed: the deploy rule's own check stays as it is. Answering from the brain
   inbox must post the same control. Lane 9 row 5 tests it.

   **No new dependency kind.** The `after` kinds stay `done`, `failed` and
   `children-done` (`internal/job/lint/lint.go:252-253`,
   `contract/schemas/job-record.json:292-318`). A `done` edge releases only on
   an accepted person Job (`internal/jobgraph/graph.go:230-231`), and `land.Run`'s
   plan-hash and lands-hash check is the content check, so an `approved` kind
   would add nothing. A declined answer wakes the orchestrator, which cancels the
   deploy Job or replans.
3. **The deploy is a command Job, not an agent.**
   - The Job is `kind: command` with role runner, and it runs `grid land T-n`,
     the existing deploy executor (`land.go:208`).
   - The launcher runs it under the same claim, shim and exit post as an agent
     (sibling lane P-loop-core, row 9).
   - The exit post is the proof. It holds the exit code, the log path, and the
     pipeline link when the deploy note says how to read it.
4. **`wait` Job, with no LLM.** It is `kind: wait`. The launcher runs
   `grid wait T-n --job W`. That command runs the deploy note's poll command
   (`gh run view`, or the GitLab API, as in the 7 Nov design) every `every`
   until the poll reports green or red, or `timeout` passes. `every` and
   `timeout` come from the Job, defaulting to the workflow keys `wait-every` and
   `wait-timeout`; the code holds no default, and lint refuses a `wait` Job with
   neither (run-jobs-launcher.md section C, point 16). The exit reason stays
   `done` or `failed` by exit code.
   - It posts the result `waited {status: green | red | timeout, link, log}`.
   - On red or timeout it also posts `deploy-failed {env, stage, reason}`, which
     is an existing type. The task then reopens, and the orchestrator follows the
     project's rule:
     - fix forward: a new Job;
     - revert: a revert Job, then a land again (7 Nov design).
5. **When an agent must act, the grant is for that Job.** This is for actions
   that need judgement. The grant names the exact rule, for example
   `Bash(make deploy-prod)`. It is either a person's approving answer or a grant
   note (Addendum 4.5).
   The sibling owns this point and the next (run-jobs-launcher.md section C,
   points 10-13; its lane 15, which took this doc's lane 8).
   - **The assignment.** It carries `allow: ["Bash(make deploy-prod)"]` and
     `grant: <approval record>`: an `approval` post, or the accepted control of
     an approve person Job (section C, point 11). The launcher appends exactly
     those rules to `--allowedTools`, verbatim.
   - **Exact.** An `allow` rule holds no `*` and no `:*`. Lint reports one
     (`job.allow-exact`). That is a rule on the shape of the post, like
     `job.land-target`.
   - **The grant must cover it.** The launcher uses `allow` only when `grant`
     is a person's approving answer for this Job, or a grant note that covers
     this Job and rule. This is a rule on reading, like the sibling's grant rule
     (sibling Q8). Otherwise the launcher starts the attempt without the extra
     rules, and `started` carries `allow_dropped: [<rule>]`. It never refuses to
     start a Job.
6. **An agent denied mid-Job.** The agent does not try another route. It posts
   `blocked {need: permission, rule: "Bash(make deploy-prod)", command, why}` and
   ends. These words go into the role agents and the run-job skill.
   - The launcher copies the `permission_denials` list from the stream-json
     result into the `exit` post, as a fact. So a denial the agent did not report
     still shows.
   - The fold puts the Job in `blocked`, and the status block lists it under
     "needs you".
   - The person answers on the `blocked` post:
     - **Approved** (an `approval` post answering the `blocked` post): the
       launcher posts a new attempt of the same Job, on the same target,
       account, model and effort, with `base: {ref: refs/grid/<T>/jobs/<J>/a<n>}`
       of the blocked attempt, `allow: [<that rule>]` and `grant: <the
       approval>`. This is mechanical, in the owner's words ("the launcher
       restarts it as a new attempt with that allow rule"). It is the second
       assignment a launcher writes, beside `push_lease_lost` (section C,
       point 18).
     - **Declined:** the home launcher wakes the orchestrator, which replans.

**Rejected.**
- *Auto mode, with or without an allow rule.* The check showed it is no gate.
- *bypassPermissions, or a wider rule such as `Bash(make:*)`.* Either is wider
  than the grant.
- *A Grid hook that blocks production commands.* Addendum 7: "No Grid-made gate".
- *An agent that tries a denied action another way.* Addendum 7 forbids it.

### B. Job code on hidden refs (V27)

**Today.**
- **Plugin runner.** It makes one local branch per attempt,
  `grid-<task>-<job>-a<n>`, with `git worktree add -b` from the base
  (`runner.go:161`, `:255-296` on 43d26ad5). Nothing is pushed.
- **App.** It makes one local branch `grid-<task>-<slug>` per Job
  (`app: agentbuild.go:150`). Its worktree merges the branches of the Jobs it
  waits on (`agentbuild.go:154-177`, `:548-598`;
  `app: internal/kernel/worktree/worktree.go:60`, `:106`).
- **Landing.** `land.sh` merges the `lane-<ID>` branches of a bare repo into main
  (`land.sh:25`, `:43`, `:84`) and moves main with `update-ref`
  (`land.sh:467-469`).
- **No code uses `refs/grid/` yet.** A grep of the non-test Go in both repos
  finds no read or write of it.
- **The post types exist.** The contract already has `submit {sha}` and
  `head {sha}` from the 7 Nov design (`contract/fixtures/submit/valid/basic.json`,
  `contract/fixtures/head/valid/basic.json`).

**Decided.**

1. **Refs.**
   - `refs/grid/<T>/jobs/<J>/a<n>` holds the work of attempt n of Job J. That
     attempt's submit writes it once, and it never moves.
   - `refs/grid/<T>/head` is the task head: the base plus one squash commit per
     integrated Job. Only integrate and land move it, each with a lease.

   No local branch is made for a Job. The refs replace:
   - the runner's `grid-<task>-<job>-a<n>` branches;
   - the app's `grid-<task>-<slug>` branches and its dependency merges;
   - land.sh's `lane-<ID>` branches.
2. **Fetch refspec.** Every clone Grid works in gets
   `git config --add remote.origin.fetch '+refs/grid/*:refs/grid-remotes/origin/*'`.
   It is never `refs/remotes/origin/grid/*`, so `git fetch --prune` never
   touches a real `grid/...` branch (7 Nov design, verified on GitHub). The
   sibling owns it (run-jobs-launcher.md B2 and section C, point 1; its lane 14).
3. **Job start, by the launcher.** The sibling owns it (run-jobs-launcher.md B2
   and section C, points 1 and 6): a narrow `git fetch --prune` of the task's
   refs and the target, then a detached worktree at the base sha in the machine
   folder. The assignment names the base:
   - for a builder, the task head's SHA, or the target's tip when the task has
     no head yet;
   - for a reviewer or runner, the SHA under review;
   - to continue an earlier attempt, `refs/grid/<T>/jobs/<J>/a<n-1>`.
4. **Job done: `grid job submit`.** The sibling owns it (run-jobs-launcher.md B3
   and section C, points 3 and 4; its lane 14). The shim runs it once per
   attempt: commit what is left, push nothing when HEAD equals the base, run the
   secrets check, push create-only with
   `--force-with-lease=refs/grid/<T>/jobs/<J>/a<n>:` (the same sha already there
   counts as done; another sha gives `ref_exists`), then post
   `submit {repo, ref, sha, base_sha, commits}`. The fold records it on the Job,
   and the reviewer and land Jobs read it from there.
5. **Secrets check before push.** The sibling owns it (run-jobs-launcher.md B3
   and section C, points 2 and 5; its lane 14): every commit in `base_sha..HEAD`,
   added lines, key and env file names and commit messages, with the patterns of
   a new `internal/secret` plus the environments note's. On a hit nothing is
   pushed and the exit reason is `secret_found`, which wakes the orchestrator.
   This doc's part is the answer: the orchestrator posts a rework Job, "remove
   the secret from the history" (`grid judge rework`, `continues` J), on the same
   `machine:login` with `carry: {job, attempt}` (lane 3 row 6).
   - **Why it is not a Grid-made gate.** It stops a push, not an agent, which
     has already ended, and the owner asked for it.
6. **Verify.** A reviewer or runner Job can run on any machine:
   - the launcher fetches with the refspec above;
   - it makes a detached worktree at the submitted SHA.

   A reviewer pushes nothing of J's. If it leaves commits, its own submit pushes
   them under its own attempt ref, and nothing integrates a reviewer's ref.
7. **Integrate.** On ACCEPTED, the orchestrator posts a command Job,
   `grid task integrate T J` (role runner, no agent, start now). It:
   - squashes `base..sha` of J's accepted attempt onto `refs/grid/<T>/head` as
     one commit. The message is the Job's title and id, with the trailer
     `Grid-Ref: refs/grid/<T>/jobs/<J>/a<n>`, which is not an attribution line;
   - pushes it with `--force-with-lease=refs/grid/<T>/head:<old sha>`, or creates
     the head from the target's tip when it is absent.

   Trivial conflicts are resolved with the note rules of Q3 step 3. It posts one
   of three results:
   - `integrated {head_sha}`, plus the existing `head {sha}` post;
   - `conflict {files}`, after which the orchestrator posts a rework Job based
     on the current head;
   - `push_lease_lost`, after which the launcher starts a new attempt
     mechanically, as for land.

   **Why a separate Job.**
   - The owner lists integrate and land as two steps.
   - Under a `branch` or `pr` landing rule, integrate happens without a land.
   - A later Job that needs J's code starts from the head once J is integrated
     (`after: {job: <integrate Job>, kind: done}`), before J lands.
8. **Land: Q3, redone.** The land Job takes the task head to the target according
   to the project's landing rule. The rule is a key in the deploy or environments
   note; the 7 Nov design names three: main, branch and PR.
   - **`main`** (the plugin and the app today). Q3 holds: one land Job per
     accepted change. It runs after its integrate Job and after the previous land
     Job in the repo. It:
     1. fetches;
     2. rebases `refs/grid/<T>/head` onto `origin/<target>`;
     3. runs the checks (Q4);
     4. pushes main and the head together in one atomic push:
        `git push --atomic --force-with-lease=refs/heads/<target>:<base>
        --force-with-lease=refs/grid/<T>/head:<old head> origin
        <sha>:refs/heads/<target> <sha>:refs/grid/<T>/head`.

     After a land the head equals the new main SHA, and the next integrate
     squashes onto it. If an earlier land already took this change (the head
     held two squashes), the tree check gives "nothing to land" (Q3 step 4).
     `landed` carries `main_sha` and `head_sha`.
   - **`branch`.** One land Job at the task's deliver step pushes the head to
     `refs/heads/<prefix><T>`, with the prefix from the note. It runs the same
     checks.
   - **`pr`.** The same as `branch`. It then opens a pull request (`gh pr create`
     or the host's API), and a `wait` Job (A.4) polls until the PR is merged or
     closed.

   **Bisect (Q4, "within the change's commits").** The change is the squash
   commits on the head above main. On red, the land Job:
   1. finds the red squash commit, whose `Grid-Ref` trailer names its Job and
      attempt ref;
   2. replays that attempt ref's own commits onto the same main base in a
      scratch worktree, and bisects them.

   The `red` result names the Job, the commit and the test. If the replay
   conflicts, the result names the Job and the test, with no commit.
9. **Cleanup.**
   - **After a land.** In the same verb, once the main push has succeeded, the land
     Job deletes the attempt refs of the Jobs it landed. It lists them with
     `git ls-remote origin 'refs/grid/<T>/jobs/<J>/*'` and runs
     `git push origin --delete` on each. A failed delete is reported in `landed`
     and tried again at task cleanup. It never fails the land.
   - **When the task closes** (delivered or cancelled). The orchestrator posts a
     command Job, `grid task cleanup T`. It deletes every `refs/grid/<T>/*` on the
     remote, the head included. Under a `pr` rule this Job is posted after the
     PR's wait ends, so the head stays while the PR is open.
   - **Why in two steps.** The owner's line is "delete `refs/grid/<T>/*` after
     landing". Under rule `main` a task lands once per accepted change, while
     other Jobs of the task may still be running from the head or be under
     review on their attempt refs. Deleting the whole namespace at the first land
     would remove that work. So each land removes what it landed, and the
     task's last landing step (its close) removes the rest. Under `branch` and
     `pr` there is one landing per task, and the two steps happen together.
   - **On every machine.** The deletion travels by git, not by the brain. That
     side is the sibling's (run-jobs-launcher.md section C, points 1 and 6; its
     lane 14): a narrow `git fetch --prune` at each Job start, a full
     `git fetch --prune origin` on the task's close post and once an hour per
     repo with a Grid worktree, worktree removal on the close post, and a
     worktree with uncommitted changes kept and named.
10. **The brain and the code.** Only the brain syncs, and code moves through git
    (Addendum 7 B, last line). The board holds SHAs and ref names, never code.

**Rejected.**
- *Keep a local branch per attempt (`runner.go:161`) and push it as a
  `grid/...` branch.* It would show as a branch and clash with real `grid/...`
  branches (7 Nov design).
- *Map the refs to `refs/remotes/origin/grid/*`.* A prune would delete real
  branches.
- *Land each attempt ref straight to main, with no task head* (Q3 as first
  written). The owner's design squashes onto the head first. The head also lets
  a later Job of the task start from accepted work before it lands.
- *A secrets check of the tip tree only.* A commit that added a secret and a
  later commit that removed it would still be served by SHA.
- *Integrate inside the orchestrator's session, as a `grid judge` verb.* The
  home machine may not have the repo, while a command Job runs where the repo is.

### The 02:50 ruling and the decision gate

- **Addenda 4 and 5.**
  - **No app.** Judgement and landing run without the app, and the launcher
    starts them (Q2, Q3, sibling Q9). The home launcher alone writes the status
    block (Q3 step 7).
  - **Answering as the person.** A reviewer may give a verdict, or answer a
    person Job, as the person only under a grant note (Addendum 4.5).
    `grid judge verdict` reads the verdict from the review's result as before.
    A person Job from `grid judge person` may be answered by a granted agent.
    The fold counts that answer as the person's only when the grant covers it
    (sibling lane P-loop-grants). The judge verbs add no rule of their own.
  - **Version-checked brain writes, and the person's edits win** (Addendum 5 A).
    The land, deploy and workflow notes are the person's notes:
    `grid land change`, `grid wait` and the orchestrator only read them.
    `grid setup` writes the default workflow note (lane 3) once, as a
    version-checked create, and a person's later edit wins.
- **Addendum 7.** A and B above.
- **The decision gate.** The owned rows are listed at the top of this doc.

### Shared with the launcher lanes (P-run-jobs-decide)

The former asks are settled. docs/research/run-jobs-launcher.md, section
"C. Shared with judge-land", is the one decision for both docs: one owner lane
and one set of names per point. This doc owns the integrate, land and cleanup
verbs (lane 4) and the deploy approval, deploy command Job and `wait` verb
(lane 9). The sibling owns `internal/gitref`, `internal/secret`, `grid job
submit`, the permissions and the running of command and `wait` Jobs (its lanes
14, 15 and 4). This doc's lanes 7 and 8 are merged into its lanes 14 and 15.

## Open for Abdu: V12.c

The design (:67) says: "the lander asks for a runner Job (test command from the
project's environments/deploy topic) with start condition 'no other test run on
this machine'". This doc's plan makes the land Job the test run. The gate may
not choose for you, so both options are below. The lane plan is written for A;
answering B means applying the B column in one edit.

| | **A. The land Job is the test run** (as planned) | **B. A separate runner Job; the land Job waits on it** (the design's words) |
|---|---|---|
| Jobs per accepted change | integrate, then land (role runner, assign `test_run: true`, start `no_test_run`) | integrate, then a runner Job (`kind: command`, `command: [land, test, T-n, J]`, assign `test_run: true`, start `no_test_run`), then land with `after: {job: <runner Job>, kind: done}` and no test condition |
| What is tested | the rebased SHA the land Job then pushes | the runner rebases head onto `origin/<target>` in a scratch worktree, runs the note's checks (and bisect and flake), and posts `tested {sha, base_sha, result}` |
| What is pushed | the tested SHA, always | the land Job pushes the tested sha only if `origin/<target>` still equals the tested `base_sha`. If main moved, it pushes nothing and ends `base_moved`, and the orchestrator posts a new runner Job and land Job |
| Cost | one Job; the test lock and the push are one step | two Jobs; a retest whenever main moves between the test and the land |
| Decision note 5 | stays (drop its "Open for Abdu" line) | becomes: "The test run is a runner Job the land Job waits on; the land Job pushes only the tested sha" |
| Rejected list | the "separate test-runner Job" bullet stays as written (drop its "Open" mark) | that bullet is deleted; a new bullet rejects "the land Job runs its own checks" with the reason "the owner's design names a runner Job" |
| Q4 "The runner condition" | stays (drop the option note) | moves to the runner Job; the Q4 table's check, rerun, not-waivable, preexisting, bisect and flake rows run in `grid land test` (same Go files) |
| Lane 1 (P-land-kind-2) | no change | adds the `tested` post schema |
| Lane 3 row 5 | no change | "one integrate Job, one runner Job, and one land Job that waits on it" |
| Lane 4 (P-land-change) | no change | adds `grid land test` to `internal/cli/land.go`; new row (m): `grid land change` pushes nothing without a `tested` green post for the same head and base, and with main moved ends `base_moved` |
| Lane 5 (P-land-tests) | no change | `check.go`, `bisect.go` and `flake.go` are called by `grid land test`; the rows stay |
| Lane 6 (P-land-launch) | no change | row 2 reads "two runner Jobs on one machine never run at once"; new row 9: `base_moved` leads to exactly one new runner Job and land Job |
| Sibling (run-jobs-launcher.md) | no change | lane 4 row 9 adds `grid land test` to the verbs run as command Jobs; Q9's `no_test_run` line is unchanged (the runner Job carries `test_run: true`) |

The optional note in the same question: should a test that is flaky again be
deleted like one that is red again (Q4, the flake rule)? The plan keeps it as
written until you answer.

## Q6. Lane plan

All lanes are in the plugin repo, and every builder runs on opus at xhigh. No
builder starts before the decision gate passes on this doc (the orchestrator's
reply to Addendum 7). Lanes are numbered for reference; the "Depends on" column
gives the order. Lanes 1-6 keep their numbers from P-judge-land-decide, with the
Addendum 7 rows added. Lane 9 is new; lanes 7 and 8 are merged into the
sibling's lanes 14 and 15 and keep their numbers as stubs; the proof lane is now 10. Each
lane's rows come from the ledger rows in its V column.

| # | Lane | V | Files | Acceptance rows | Depends on |
|---|---|---|---|---|---|
| 1 | **P-land-kind-2** (as briefed; also owns the Job kinds `command` and `wait`, RJ C point 17) | V12, V26, V27 | `contract/schemas/job-record.json`, `internal/job/job.go`, `internal/job/lint/`, tests from aa0cf14a | 1. lint refuses a land Job without target, branch or sha; 2. round-trip through the Feed keeps them; 3. existing tests green; 4. (V27) a land Job whose `branch` is `refs/grid/T-1/head` passes lint and survives the round-trip; 5. (V26) schema and lint accept `kind: command` with a `command:` argv and `kind: wait` with `every` and `timeout`, both survive the round-trip, and lint refuses a `command` Job with no argv | — |
| 2 | **P-judge-verbs** | V11, V27 | new `internal/judge/` (`verdict.go`, `review.go`, `rework.go`, `person.go`, `id.go`), `internal/cli/judge.go`, `commands.json`, `docs/job-contract.md:123-128` | 1. verdict parse ports the app's verdict cases and adds STUCK; 2. rework body ports `TestReworkPostsANewJobIDWithOnlyJobRecordKeys`; 3. request id ports both `judge_post_request_id` tests (no epoch); 4. a second call with the same wake succeeds with nothing posted (`ErrDuplicateWake`); 5. a cancelled review gives one person Job (ports both `judge_cancelled_review` tests); 6. `verdict` reports rounds over the `continues` chain; 7. a verdict from an older attempt is reported stale; 8. (V27) the review Job's base is the submitted SHA of the reviewed attempt, and a rework Job's base is the current `refs/grid/<T>/head` SHA, or the target's tip when the task has no head | 1 (only for `kind`), launcher wake (exists) |
| 3 | **P-judge-orchestrator** | V11, V27 | `agents/orchestrator.md` (replace the `never_judged` text at line 28 with the flow in Q2), the brain default workflow note (`review rounds: 2`), `docs/job-contract.md` | 1. a scripted task with a fake reviewer: finished, reviewed REJECTED, one rework, reviewed ACCEPTED, accepted, with no hand step and the app stopped; 2. a second REJECTED gives one person Job; 3. STUCK gives one person Job and no rework; 4. every wake is acked once (ports `TestJudgeAcknowledgesEveryWake` cases as fold checks); 5. (V27) after ACCEPTED the orchestrator posts one integrate Job and one land Job that waits on it; 6. (V27) `secret_found` leads to one rework Job, "remove the secret from the history", on the same `machine:login` with `carry`; 7. (ruling) `grid setup` creates the default workflow note once with a version check, and a second run after a person's edit leaves the edit; 8. (ports `TestJudgeStallFreshHome`, with a change) on a fresh home after `grid setup` (no project workflow, the default workflow note present), a submitted builder gets one review Job discovered from it; a second builder in the same task gets its own; `judgement_default_flow` is recorded once per task over both wakes; nothing reports `workflow_unreadable`. The change: the app's default lives in Go (`app: judge.go:35`), here it is the note (decision note 3), so a home with no project workflow and no note gets one person Job asking for a workflow (Q2), which the app test has no case for; 9. (ports `TestJudgeReviewsWhenThePostedReviewerWaitsOnTheBuilder`, three cases) (a) a reviewer the orchestrator posted with an `after` done edge on builder J is not the open review: the flow posts the review Job J.1 and posts nothing that targets that reviewer; (b) with the review J.1 REJECTED and a round left, the result is one rework Job (`continues` J) with J closed rejected, no person Job and nothing on the other reviewer; (c) a running reviewer discovered from J gives no new review, only the wake's acknowledgement; 10. (ports `TestJudgeOrchestratorJobIsJudgedLikeABuilder`, two cases) a submitted orchestrator-role Job goes the builder's way: under the stock workflow it gets a review Job; under a workflow Build then Approve it gets one person Job for the task's owner; 11. (ports `TestJudgeDefaultFlowNeverBeatsAWorkflow`, five cases) when the project has a workflow, the default note is never used: (a) a workflow with `review rounds: 1` and one accepted review gives "rounds used" and no default; (b) an approve step after run gives the person Job for the owner and no review Job; (c) a named workflow that is missing, (d) a `grid workflow show` that fails and (e) a broken workflow file each give `workflow_unreadable`, post nothing and do not fall back to the default | 2 |
| 4 | **P-land-change** | V12, V27 | `internal/land/change.go`, `conflict.go`, `push.go`, new `integrate.go` and `cleanup.go`, `internal/land/note.go` (reads the brain note: generated pattern, regenerate command, union list and check, check commands, landing rule), `internal/cli/land.go` (`grid land change`), `internal/cli/task.go` (`grid task integrate`, `grid task cleanup`), a seed environments note for the plugin repo | Q4 table rows for branch, ATTR, generated, union, conflict, nothing to land, push with lease; plus: two accepted changes in one repo land in order and the second rebases onto the first; plus: the generated list and commands come from the note, none hard-coded; plus (V27, bare remote in a temp dir): (a) integrate squashes `base..sha` onto the head as one commit with the `Grid-Ref` trailer; (b) an integrate conflict gives `conflict` with files; (c) a head moved between fetch and push gives `push_lease_lost` for integrate and for land; (d) land rebases the head and pushes main and head in one atomic push, and after it head equals main; (e) if either lease fails, neither ref moves; (f) the landed Jobs' attempt refs are gone from the remote, and a failed delete is reported in `landed` while the land still succeeds; (g) rule `branch` pushes the head to `refs/heads/<prefix>T-1`; (h) rule `pr` calls a fake `gh pr create`, and the land result carries the PR link; the verb posts no Job (the orchestrator posts the `wait` Job); (i) `grid task cleanup` removes every `refs/grid/T-1/*` on the remote; (j) (test plan E) integrate squashes only the attempt the accepted review names, and with `a1` and `a2` on the remote it never takes `a1`; (k) (test plan D) before ACCEPTED, no `refs/heads/*` on the remote changes and the head holds none of the Job's commits; (l) (was RJ 14 row 7) two integrates racing on one head: both changes end on head, one through a mechanical new attempt after `push_lease_lost`, and integrate posts `integrated {head_sha}` plus `head {sha, ref}` | 1, RJ 14 |
| 5 | **P-land-tests** | V12, V27 | `internal/land/check.go`, `bisect.go`, `flake.go` | Q4 table rows for checks, rerun, not waivable, preexisting, bisect, flake, one fix Job; plus (V27): a head with two squash commits where the second is red: the result names that Job, then bisects its attempt ref's commits replayed on the main base and names the commit and the test | 4 |
| 6 | **P-land-launch** | V11, V12 | launcher files named by P-run-jobs-decide (start conditions "after L lands" and "no other test run on this machine"; a new attempt on `push_lease_lost`; the status block "landed <sha>"), `agents/orchestrator.md` (post an integrate Job and a land Job after ACCEPTED; fix Jobs for flakes) | 1. a lost lease reruns as attempt 2 and lands; 2. two land Jobs on one machine never run checks at the same time; 3. the status block shows the main SHA; 4. a `conflict` result leads the orchestrator to post one rework Job (`grid judge rework`, `continues` J) that names the files; 5. a `red` result leads to one rework Job whose must-fix names the commit and the test; 6. a `landed` result with a flake leads to one fix-the-cause Job, and none when an open Job already names the test; 7. a `preexisting` note leads to one fix Job against main; 8. (drain, Q4 table) three land Jobs L1, L2, L3 of three different tasks in one repo, L2 after L1 and L3 after L2; L2 ends `red`: the orchestrator posts one rework for L2's change, re-posts L3 with its edge on L1 (or none once L1 has landed) and cancels the old L3, and L1 and L3 land | 3, 5, the launcher lanes of P-run-jobs-decide |
| 7 | **P-code-refs**: merged into P-run-jobs-decide lane 14 P-loop-refs | V27 | none here | its rows moved to RJ 14 (row map in run-jobs-launcher.md, section C) | — |
| 8 | **P-prod-actions**: merged into P-run-jobs-decide lane 15 P-loop-permissions | V26 | none here | its rows moved to RJ 15 (row map in run-jobs-launcher.md, section C) | — |
| 9 | **P-deploy-jobs** (new; owns C points 11 (deploy approval), 14, 15, 16) | V26 | the deploy workflow note's `approve` step (the default deploy workflow note), `internal/workflow` (`wait-every`, `wait-timeout` keys; the `waited` schema is RJ 1's), new `internal/land/wait.go` and `internal/cli` (`grid wait`), `internal/job/lint/` (a `wait` Job needs `every` and `timeout` or the workflow keys), `agents/orchestrator.md` (post the person Job, the deploy command Job and the wait Job; fix forward or revert), the inbox answer path for an approve Job (calls `internal/land/approval.go`) | 1. a deploy command Job with `after: {job: <approve Job>, kind: done}` does not start before the person approves, and starts once they do; a decline wakes the orchestrator and the Job never starts; 2. the deploy runs as a `kind: command` Job (`grid land T-n`) with no agent, its exit holds the code, the log path and the link, and `internal/land`'s approval tests stay green; 3. `grid wait` against a fake poll command, with `every` and `timeout` from the Job or the workflow keys, gives `waited green`, then `red` and `timeout` each give `deploy-failed` and reopen the task, and no LLM starts; lint refuses a `wait` Job with neither; 4. with the project rule "revert", red leads to one revert Job and then a land; with "fix forward", it leads to one fix Job; 5. an approval answered from the brain inbox posts the same accepted control as `grid approve` (`plan_hash`, `plan_digest`, `lands_hash`), and `land.Run` accepts it; an answer for another plan or lands rule is refused | 1, P-loop-core row 9 (RJ 4) |
| 10 | **P-judge-land-proof** (was 7) | V11, V12, V26, V27 | a real-plugin test under `internal/cli` or `tests/`, plus the lane doc | the owner's acceptance below, end to end with the app not running, with the code on hidden refs (RJ 14) and one approved deploy (lane 9) | 1–6, 9, RJ 14, RJ 15 |

**The owner's acceptance, mapped.**

| Owner row | Lane, row |
|---|---|
| finished, reviewed, rework once, accepted, landed; no hand step, app not running | 3.1 + 6.3 + 10 |
| a second failed review raises a person question | 3.2 (+ 2.5 for the cancelled review) |
| two accepted changes in one repo land in order, the second rebases | 4 (order row, 4d) + 6.2 + 10 |
| one conflict | 4 (conflict row, 4b): `conflict`; 6.4: the rework Job |
| one red test bisected | 5 (bisect row and the squash row): the result names the commit and the test; 6.5: the rework Job |
| one flaky test landed with a fix Job | 5 (flake row) + 6.6 (orchestrator posts the fix Job) |
| (V26, A) approval from the workflow before the Job, in the person's inbox | 9.1 |
| (V26, A) the deploy is a command Job plus a `wait` Job; log, exit code and link are the proof | 9.2, 9.3 |
| (V26, A) an agent that must act gets the exact allow rule, nothing wider, never bypassPermissions | RJ 15 (1), (2), (9), (10) |
| (V26, A) denied mid-Job: `blocked: needs permission`; approved gives a new attempt, declined gives a replan | RJ 15 (4), (5) |
| (V27, B) detached worktree at the base SHA, in the machine folder | RJ 14 (1), (5) |
| (V27, B) commit, secrets check, push the attempt ref, SHA on the board | RJ 14 (2), (3), (12) |
| (V27, B) verify from any machine at that SHA | RJ 14 (5), 2.8 |
| (V27, B) integrate: squash onto `refs/grid/<T>/head` | 3.5, 4a-4c, 4l |
| (V27, B) land: rebase head onto main, or a branch or PR, per the landing rule | 4d, 4e, 4g, 4h |
| (V27, B) cleanup on the remote and on every machine; prune never touches real `grid/...` | 4f, 4i, RJ 14 (8), (11) |
| (V27, B) the refspec on every clone | RJ 14 (9) |
| Test plan D: a Job's code arrives on `refs/grid/<T>/jobs/<J>/a<n>`; nothing reaches main or a branch before a reviewer accepts it; after landing the refs are gone on the remote and every machine | RJ 14 (2), 4k, 4f, 4i, RJ 14 (11) |
| Test plan D: no deploy before approval in the inbox; the command Job deploys, the wait Job reports, a failed deploy reopens the task | 9.1, 9.5, 9.2, 9.3 |
| Test plan D: an agent refused a permission posts `blocked: needs permission`, tries no other route, and approving restarts it with only that rule | RJ 15 (4), (5), (2) |
| Test plan E: a reassigned Job pushes a new attempt ref; the old attempt's ref is never integrated | RJ 14 (14), 4j |
| Test plan G: approval as a file-based gate, answered in the inbox with no app | 9.1, 9.5 |
| Test plan M: a reviewer on another machine checks out a Job ref with no branch and no copy by hand | RJ 14 (5), 10 |

"RJ n (m)" is row m of lane n in docs/research/run-jobs-launcher.md, which took
this doc's former lanes 7 and 8 (row map in its section C).

**Open items.**
- **Launcher interfaces.** Agreed with the sibling doc: every shared point has one
  owner lane and one name in docs/research/run-jobs-launcher.md, section
  "C. Shared with judge-land". Its Q9 gives lane 6 the wake on exit, the start
  conditions and the new attempt on `push_lease_lost`.
- **V12.c** waits for Abdu (section "Open for Abdu: V12.c").
- **Drain** (V12.f): the reading in the Q4 table is this lane's; Abdu confirms it.
- **Codex.** The `codex exec` flags that match dontAsk and an exact allow rule
  are not checked (Addendum 7 A.1).
- **Other hosts.** GitLab and other hosts may serve a commit by SHA after its ref
  is deleted, which the 7 Nov check did not cover. The sibling's lane 14 row 13 records the first
  use. The secrets check runs on every host either way.
- **Where the landing rule lives.** The key for main, branch or PR is placed in
  the deploy or environments note by lane 4 (`internal/land/note.go`). The owner
  may want it elsewhere, for example in project.md.
- How a test "pins a past incident" is decided by its doc comment or a brain
  decisions note. Lane 5 writes the check the fix Job runs. The owner may want a
  fixed marker word instead.

DONE
