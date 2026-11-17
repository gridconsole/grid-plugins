# Judge and land in the plugin

Lane P-judge-land-decide. Research only; nothing is built here. Evidence names a
file and line in this repo, in the app repo on `main` (prefix `app:`), or in the
landing script `land.sh` the grid uses today (prefix `land.sh:`). "Design" is the
owner's run-jobs design brief and its Addenda 3 to 5; "Addendum 6" is the
brain-obsidian brief's Addendum 6 (no task lease, `attempt` in the post header).

## Decision note

**Decided.**

1. Judgement leaves the app. No long-running judge process takes its place, in
   Go or as an LLM. The launcher (`grid loop`) already moves a Job that exited
   `done` to `submitted` with "awaiting judgement" (`internal/orchestrator/tick.go:330`)
   and prints the wake (`internal/orchestrator/stream.go:353`). From there the
   home orchestrator decides. It posts a reviewer Job. On the verdict it posts
   exactly one of three things:
   - a land Job;
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
   fields from P-land-kind-2. It runs `grid land change`, which:
   - rebases the change onto main;
   - resolves trivial conflicts only;
   - runs the checks;
   - bisects a red test within the change's own commits;
   - lands a flaky test with a flake note;
   - pushes with `git push --atomic --force-with-lease`;
   - posts `landed` with the new main SHA.

   Order within a repo uses the start condition "after J-7 lands". The new code
   goes in `internal/land` (Addendum 3's word), in new files beside the deploy
   executor (`internal/land/land.go:1`), which stays as it is. Deploy stays the
   existing `grid land` step (`internal/land/land.go:208`).
5. The launcher retries a lost push race mechanically, as a new attempt of the
   same land Job. Posts carry `attempt`, not a lease epoch (Addendum 6).
   - The app's `no_epoch` failure (`app: judge.go`, `epoch`) goes away.
   - So does the `--lease-epoch` flag on its posts (`app: judge.go:962-979`).

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
- *A hard-coded STUCK path in Go.* STUCK is a verdict word the verdict verb
  reports. Posting the person Job stays the orchestrator's choice.

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
- **`judge_stall_test.go`**, rows 1–5:
  - `TestJudgeStallFreshHome`
  - `TestJudgeReviewsWhenThePostedReviewerWaitsOnTheBuilder`
  - `TestJudgeAcknowledgesEveryWake`. Every path gives exactly one record that
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
  - `TestJudgeOrchestratorJobIsJudgedLikeABuilder`
  - `TestJudgeDefaultFlowNeverBeatsAWorkflow`
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
  none   -> control J accepted "workflow carries on" (+ land Job if J changed a repo)
reviewer J.1 exits done
  launcher: control J.1 submitted; wake result J.1/1
orchestrator: grid judge verdict J.1 --json
  ACCEPTED                      -> control J accepted; control J.1 accepted "verdict ACCEPTED";
                                   post land Job L (Q3)
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
| land Job | orchestrator | job | Q3 |
| rework Job | orchestrator via `grid judge rework` | job | new id, `continues J`, `discovered_from J.1`, copied body, row "Must-fix from review round N" (keys id, text, traces, state), `review_rounds N`, `attempt 1` |
| reject | `grid judge rework` | control | `target J, status rejected, close_reason "reworked as J'"` |
| person Job | orchestrator via `grid judge person` | job, `--person owner` | as in the app's `personBody` (Q1), with the reason: second fail with the reviewer's text, STUCK, or a review cancelled before it ran |

## Q3. The land Job

**One per accepted change.** The orchestrator posts it in the same turn that
accepts J, and only when J changed a repo (J has a branch and a SHA). The Job
record uses the fields of P-land-kind-2 (commit aa0cf14a on `lane-P-land-kind`):
`kind: land`, plus `repo`, `target`, `branch` and `sha`. Lint enforces them
through the rules `job.land-target`, `job.land-branch` and `job.land-sha`.
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
2. Rebase `branch` onto `origin/<target>`. This is the owner's word. The script
   merges (`land.sh:41-55`), and land-job-type section 3 merges too.
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
   deploy step runs as it does today (`internal/land/land.go:117` rule,
   `:208` Run, `:288` history note). Nothing in it changes.

## Q4. Tests

**The runner condition.** The land Job carries a second start condition: "no
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
| ATTR (union attributes from the note) | `:27`, `:79` | `conflict.go` | temp repo: union file conflict resolved, check command run |
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
| dropped: flock, MAXB, TRAIN-DROP, cross-floor, HOLD, STATE-UI | `:29-31`, `:59-70`, `:83`, `:88-98`, `:471` | — | — |

## Q5. Reconciling R-land-job-type L1–L6 and P-land-kind-2

| Lane (land-job-type section 6) | Verdict | What changes |
|---|---|---|
| L1-land-kind | **stays** | This is P-land-kind-2: `target`, `branch` and `sha` on `kind: land`, three lint rules, and a round-trip test (aa0cf14a). The rebase flow needs no other field. |
| L2-land-change | **stays, changed** | The package is `internal/land`, not `internal/landjob`. The flow is rebase, not merge. The push uses a lease (no local update-ref CAS). Stacking (option B) is dropped. It adds the result codes `landed`, `conflict`, `red`, `preexisting` and `push_lease_lost`. |
| L3-land-red | **stays, changed** | Red is bisected within the change's commits, not across lanes. The flake rule has the owner's second-time delete and the incident exception. |
| L4-land-step (workflow step `land`, spawn `command`) | **split** | The workflow step kind is dropped: the orchestrator posts the land Job after ACCEPTED, and no workflow step is needed. The `command` spawn (a runner Job that runs a plugin verb with no agent) moves to the launcher lanes of P-run-jobs-decide. |
| L5-app-land | **dropped from the app** | It moves to the plugin as `grid judge` and the orchestrator's text (lane J2 and J3 below), as dogfood-gaps G8 already found. |
| L6-feed-land-ui (app) | **dropped** | The task.md status block shows "landed <sha>" (Addendum 4). The app is not on the path. |
| agent nice +10 (section 4) | **not in this plan** | It is a capacity setting, which is the launcher's measuring side. Left to P-run-jobs-decide. |

**P-land-kind-2 continues as is.** Its rows 1–3 hold unchanged: the lint rules,
the round-trip test, and the existing tests staying green. It is the first lane
below, and nothing else depends on its internals. Its brief names sonnet high.
This plan lists every builder at xhigh, so the owner chooses whether to relaunch it
at that level.

## Q6. Lane plan

All lanes are in the plugin repo, and every builder runs on opus at xhigh. In
order:

| # | Lane | Files | Acceptance rows | Depends on |
|---|---|---|---|---|
| 1 | **P-land-kind-2** (as briefed) | `schemas/job-record.json`, `internal/job/job.go`, `internal/job/lint/`, tests from aa0cf14a | 1. lint refuses a land Job without target, branch or sha; 2. round-trip through the Feed keeps them; 3. existing tests green | — |
| 2 | **P-judge-verbs** | new `internal/judge/` (`verdict.go`, `review.go`, `rework.go`, `person.go`, `id.go`), `internal/cli/judge.go`, `commands.json`, `docs/job-contract.md:123-128` | 1. verdict parse ports the app's verdict cases and adds STUCK; 2. rework body ports `TestReworkPostsANewJobIDWithOnlyJobRecordKeys`; 3. request id ports both `judge_post_request_id` tests (no epoch); 4. a second call with the same wake succeeds with nothing posted (`ErrDuplicateWake`); 5. a cancelled review gives one person Job (ports both `judge_cancelled_review` tests); 6. `verdict` reports rounds over the `continues` chain; 7. a verdict from an older attempt is reported stale | 1 (only for `kind`), launcher wake (exists) |
| 3 | **P-judge-orchestrator** | `agents/orchestrator.md` (replace the `never_judged` text at line 28 with the flow in Q2), the brain default workflow note (`review rounds: 2`), `docs/job-contract.md` | 1. a scripted task with a fake reviewer: finished, reviewed REJECTED, one rework, reviewed ACCEPTED, accepted, with no hand step and the app stopped; 2. a second REJECTED gives one person Job; 3. STUCK gives one person Job and no rework; 4. every wake is acked once (ports `TestJudgeAcknowledgesEveryWake` cases as fold checks) | 2 |
| 4 | **P-land-change** | `internal/land/change.go`, `conflict.go`, `push.go`, `internal/cli/land.go` (`grid land change`) | Q4 table rows for branch, ATTR, generated, conflict, nothing to land, push with lease; plus: two accepted changes in one repo land in order and the second rebases onto the first | 1 |
| 5 | **P-land-tests** | `internal/land/check.go`, `bisect.go`, `flake.go` | Q4 table rows for checks, rerun, not waivable, preexisting, bisect, flake, one fix Job | 4 |
| 6 | **P-land-launch** | launcher files named by P-run-jobs-decide (start conditions "after L lands" and "no other test run on this machine"; a new attempt on `push_lease_lost`; the status block "landed <sha>"), `agents/orchestrator.md` (post a land Job after ACCEPTED; fix Jobs for flakes) | 1. a lost lease reruns as attempt 2 and lands; 2. two land Jobs on one machine never run checks at the same time; 3. the status block shows the main SHA | 3, 5, the launcher lanes of P-run-jobs-decide |
| 7 | **P-judge-land-proof** | a real-plugin test under `internal/cli` or `tests/`, plus the lane doc | the owner's acceptance below, end to end with the app not running | 1–6 |

**The owner's acceptance, mapped.**

| Owner row | Lane, row |
|---|---|
| finished, reviewed, rework once, accepted, landed; no hand step, app not running | 3.1 + 6.3 + 7 |
| a second failed review raises a person question | 3.2 (+ 2.5 for the cancelled review) |
| two accepted changes in one repo land in order, the second rebases | 4 (order row) + 6.2 + 7 |
| one conflict | 4 (conflict row): `conflict`, then a rework Job |
| one red test bisected | 5 (bisect row): the result names the commit and the test |
| one flaky test landed with a fix Job | 5 (flake row) + 6 (orchestrator posts the fix Job) |

**Open items.**
- The sibling decision doc (`docs/research/run-jobs-launcher.md`,
  P-run-jobs-decide) had not been handed in when this doc was written. Lane 6
  uses three launcher interfaces this doc assumes:
  - the wake on exit;
  - start conditions, including cross-task "after L lands";
  - the new attempt on `push_lease_lost`.
- How a test "pins a past incident" is decided by its doc comment or a brain
  decisions note. Lane 5 writes the check the fix Job runs. The owner may want a
  fixed marker word instead.

DONE
