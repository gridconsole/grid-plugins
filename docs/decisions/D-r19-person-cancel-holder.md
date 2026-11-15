# D-r19-person-cancel-holder: how a person of the team cancels a running Job whose task lease the machine's worker holds

Base: plugin main `066d9b2a`, app main `de97fc2e`. Every path:line below is on those two heads. Plugin paths are repo-relative. App paths are prefixed `app:`.

Decision lane. It changes no code, no test and no golden.

## The problem

Round 19 found that the task lease holder is `worker@<machine>`. Under the C-r18 fix, that holder means a person's cancel of a running Job is refused. The cancel itself was not run (F4 NOT REACHED, r19 REPORT.md:4, :12). The r19 report and the kept lease file come from the brief: this lane could not open them, because they are outside the paths it may read. The round 19 Job snapshot this lane could read shows every Job assigned `to: worker@<machine>`. Each `grid worker next` stream assigns its task's Jobs to the holder it took the lease as (app:internal/kernel/worker/job.go:343-345).

What the code does today, quoted:

- **The C-r18 person path.** plugin internal/cli/orchestrator.go:317-321 reads the task lease and takes its epoch. The writer it builds is still the person's own id:
  ```go
  if verb == "cancel" && *epoch == "" && (*leaseFlag == "" || *leaseFlag == "task") && contract.TierFor(personRole) == contract.PersonTeam {
      if held, readErr := (lease.TaskStore{Feed: store}).Read("task"); readErr == nil && held.Holder != nil {
          leaseName, *epoch = "task", fmt.Sprintf("%d.%d", held.Gen, held.Fence)
  ```
  The comment at :313-315 says the lease "still has to name this member's own <member>@<machine> as holder ... or the write is refused as a stale lease".
- **The writer is GRID_MEMBER@machine.** internal/cli/orchestrator.go:740:
  `return feed.Writer{Author: feed.Author{ID: member, Role: role}, Worker: member + "@" + machine, Lease: name, Gen: gen, Fence: fence}, nil`
- **The stale-lease refusal.** internal/feed/store.go:143-144:
  `if value(lease.Holder) != w.Worker || lease.Lease != w.Lease || lease.Fence != w.Fence || lease.Gen != w.Gen { return fail(ErrStaleLease) }`
  It returns `ErrStaleLease = errors.New("stale lease")` (store.go:22). The CLI exits 3 on it (the test at internal/cli/r18_person_job_cancel_test.go:72, :87 and :115 asserts that).
- **The author must be the writer's member.** internal/feed/record.go:183-184:
  `if contract.WorkerMember(r.Worker) != r.Author.ID { return &InvalidRecordError{Err: fmt.Errorf("author and worker disagree")} }`
- **Where the holder comes from (app).** app:internal/kernel/worker/run.go:110-136. It is read from the `holder` field of worker-config.json (:112), then from GRID_WORKER_ID (:118-119). Without either it defaults at :130-131:
  ```go
  if c.Holder == "" {
      c.Holder = "worker@" + machine
  }
  ```
  `grid-worker start -holder` (app:cmd/grid-worker/main.go:92, :156-157) is from the brief and was not opened here. grid-server starts the worker as `[]string{"start", "-home", home}` (app:cmd/grid-server/worker_start.go:50). The plugin's `grid open` does the same (internal/cli/open_start.go:216-222, from the brief, not opened here). Neither passes a holder.
- **Every agent and judge gets the holder's member and the role worker.** app:internal/kernel/worker/job.go:384-386:
  ```go
  values["GRID_WORKER_ID"] = j.Holder
  values["GRID_MEMBER"] = j.Holder[:strings.LastIndex(j.Holder, "@")]
  values["GRID_ROLE"] = "worker"
  ```
  The judge does the same: app:internal/kernel/worker/judge.go:221-222. The worker's own stream also runs as the holder: `"--member", holder[:strings.LastIndex(holder, "@")], "--role", "worker"` (job.go:351-352).
- **A person gets the person's own member.** app:internal/kernel/plugincli/person_check.go:118:
  `return Environment(inherited, "GRID_HOME="+home, "GRID_ACTOR="+who, "GRID_MEMBER="+who, "GRID_WHO="+who, "GRID_ROLE="+role)`
- **The worker takes no person authority.** internal/cli/worker.go:122-125 maps role `worker` to `cli`: "A worker performs mechanical CLI controls on the task lease and takes no person authority." orchestratorWriter does the same for task writes (orchestrator.go:716-720).
- **The worker already cancels.** app:internal/kernel/worker/supervisor.go:128-131 runs `job cancel --task <task> --job <id> --reason <r> --json` under the task epoch the Job carries.

**Result.** A person's GRID_MEMBER is the person's member and never `worker`. Every person cancel of a running Job in the product therefore takes the C-r18 path and fails at store.go:143: exit 3, `stale lease`. `TestR18PersonOfAnotherMemberStaysStale` (internal/cli/r18_person_job_cancel_test.go:84-88) pins exactly this case.

**A precedent for the pick.** A person's write that must not wait on another holder's lease already exists. A person of the team posts a Job, or answers a person Job, as an unleased record (orchestrator.go:294-301, :323-324). The holder's own pass then writes the leased half. internal/orchestrator/person.go:48-51 says so:

> personAnswers is the holder's half of an unleased answer to a person Job: a person of the team answers with an `answer` that follows the Job's record and has no lease, and this pass writes the message and the accepting control under the task lease

That pass runs in every tick (internal/orchestrator/tick.go:62). The round 19 Job snapshot is consistent with this: the person Job J-5 shows "answered by" the owner's member and status accepted, while every other Job was assigned to `worker@<machine>`. The snapshot does not show whether the answer was unleased, nor the lease file itself.

## Options

| Option | What changes | Repo | Who can cancel | Author the Feed records | Fence and invariants | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| A1: the plugin writes the person's cancel under `worker@<this machine>` with the author set to `worker` | orchestrator.go:317-321, :740 | plugin | any person of the team on this machine | `worker`, role owner. The person who cancelled is lost. | Passes store.go:143 and record.go:183, but records the wrong author. | No |
| A2: same as A1, but the author is the person | as A1, plus record.go:183 | plugin | as A1 | the person | record.go:183 refuses it ("author and worker disagree"). The fix would have to drop the rule "the author is the member of the writer id". The person's process would then write into the holder's tenure file under the holder's id, and every reader that trusts author = writer member would have to change. | No |
| B: the app starts the worker as `<owner member>@<machine>` | app:cmd/grid-server/worker_start.go:50 and internal/cli/open_start.go:216-222 pass `-holder`, or the app writes `holder` into worker-config.json | app and plugin | the owner only. A member or lead with another member id is still refused at store.go:143. | the owner's member for the person **and** for every agent and judge (job.go:385, judge.go:222), so only the role tells them apart | Unchanged, but tasks already held by `worker@<machine>` would need a takeover when the holder changes. | No |
| C: the cancel is posted as an unleased person request, and the holder carries it out under its own lease | the CLI person branch, the person write, a holder pass beside personAnswers, a leaseless branch in control.json | plugin | any person of the team (owner, member, lead), whatever the holder's name | the person, on the request. The holder's leased control points back to it with `re`. | store.go:143 and record.go:183 are unchanged. Only the holder writes under the task lease. | **Pick** |
| D: no product change. The person, or an app route, runs the cancel as the holder's member (`GRID_MEMBER=worker`). | nothing. In app:cmd/grid-server, `"job", "cancel"` appears only as a leased call in realplugin tests; this lane searched no other app folder for a person cancel route. | none | anyone who knows the holder's member | `worker`, role owner, as in A1 | as A1 | No: it records the wrong author, and the person must know an internal holder name (r19 REPORT.md:26 asks which member) |

**Why C.** C is the only option that:

- keeps the person as the recorded author;
- leaves both write invariants as they are: lease holder = writer at store.go:143, and author = writer member at record.go:183;
- does not depend on what the holder is called (`worker@<machine>`, a configured holder, or the person's own member);
- reuses a mechanism that already ran in round 19.

Its cost is a contract change: a leaseless control for a person's cancel. The C-r18 builder and reviewer both named that as the way to let a person cancel under another member's lease (docs/lanes/C-r18-person-job-cancel.md:40, and the review's second note).

## The rule (C)

**1. Who may post a cancel request.** The caller must meet all of these:

- GRID_ROLE is owner, member or lead, so `contract.TierFor` is PersonTeam (internal/contract/types.go:30-33). Lead stays in, as in C-r18. No app agent runs as lead, because the app sets GRID_ROLE=worker for every agent and judge (job.go:386, judge.go:221).
- The caller has no lease epoch from anywhere: no `--lease-epoch`, no global `--lease-epoch`, no GRID_LEASE_EPOCH and no GRID_TASK_EPOCH. orchestrator.go:280-290 then leaves `*epoch` empty.
- `--lease` is either not given or is `task`.

**2. Which path the cancel takes.** The CLI reads the task lease, as it does today at :318.

- **The person is the holder.** The holder is the person's own `GRID_MEMBER@<this machine>`. The cancel is written at once under that lease, exactly as C-r18 does now. `TestR18PersonCancelsRunningJobWithoutAnEpoch` keeps passing.
- **Anyone else holds the lease, or nobody does.** This covers `worker@<machine>`, any other member, and a lease file with no holder. The CLI writes an **unleased cancel request** with:
  - author `{GRID_MEMBER, role}`;
  - writer id `GRID_MEMBER@<this machine>`;
  - lease null and fence 0;
  - type `control` and body `{target, status: "cancelled", reason, close_reason}`. This is the body that orchestrator.go:421 builds today.

  The request must not move the Job. The build lane makes the fold leave a Job's state alone for an unleased control, and pins that with N6. This lane did not check what fold.go does today with an unleased control.

**3. Which holder carries it out.** The holder of the task lease, whatever its name, carries it out on its next pass. A new pass, `personCancels`, runs in tick.go beside `personAnswers` (:62).

- **The request is carried out when** its author's tier is PersonTeam, it has no lease and fence 0, its status is `cancelled`, and its target Job is not Accepted, Cancelled or Rejected. The pass then writes the leased control `{target, status: "cancelled", reason, close_reason}` under its own task lease, with `re` set to the request id. Its author is the holder's member with role `cli` (orchestrator.go:718-720).
- **The control id comes from the request id**, the way app:internal/kernel/worker/supervisor.go:140 derives one from the exit. A pass that is repeated therefore writes the same record.
- **Other requests for the same Job.** The earliest request wins. Any other request for that Job, or a request whose target closed before the pass ran, gets a Feed message that says it was not used. personAnswers does the same at person.go:52-54.

**4. Which machine.** The request is written into the task's Feed in this Grid home. Its writer id carries this machine (orchestrator.go:721-727). The holder whose pass reads that Feed carries it out. Nothing compares the requester's machine to the holder's: an answer to a person Job compares none either (person.go:66). A person whose Grid home does not hold this task's Feed cannot reach the task at all.

**5. What other callers get.** Unchanged except for the one row marked new.

| Caller | What it gets |
| --- | --- |
| A guest | No request, because TierFor(guest) is PersonGuest (types.go:34-35). The leased path refuses as today. The holder's pass ignores any request authored by a guest. |
| An agent (GRID_ROLE=worker, or cli or any agent role) | No request. Its leased path with GRID_TASK_EPOCH is unchanged (orchestrator.go:280-290). The holder's pass ignores any request whose author tier is not PersonTeam. |
| A typed or guessed epoch, from anyone | Used as typed: no request, and the leased path at store.go:143. |
| A person of another member | **New: exit 0, request saved.** Today this is exit 3. |

## Refusals and negative rows

Every row is one a test can pin. "Today" means the row passes on base unchanged. "New" means the build lane adds it.

| # | Caller and input | Exit | stderr (contains) | Feed after | Today or new |
| --- | --- | --- | --- | --- | --- |
| R1 | person (owner, member, lead), no `--reason` | 1 | `cancel requires --reason` (orchestrator.go:417-418) | nothing written | today. Under C the build moves this check before the request is written. |
| R2 | guest, no epoch | 1 | `GRID_LEASE_EPOCH or --lease-epoch must be gen.fence; the task holder gets it from: grid lease take task` (orchestrator.go:730) | nothing written, Job still Running | today. A guest is not PersonTeam. The early guest gate at cli.go:241 does not take `job` (guestWriteVerb, cli.go:282-297), so the refusal comes from :730. cli.go:271 (workflowRoleRefusal) was not read; the build's test pins which refusal a guest sees. |
| R3 | guest, typed `--lease-epoch <gen.fence>` of the real lease | 3 | `stale lease` (store.go:143: holder ≠ `<guest member>@<machine>`) | nothing written | today |
| R4 | agent (GRID_ROLE=worker) with a stale GRID_TASK_EPOCH | 3 | `stale lease` | nothing written | today, `TestR18AgentCancelIsNotGivenTheLease` (r18 test :112-116) |
| R5 | agent (GRID_ROLE=worker), no epoch | 1 | `gen.fence` | nothing written | today, same test :122-123 |
| R6 | person, typed or guessed `--lease-epoch` (flag, global flag or GRID_LEASE_EPOCH) | 3 | `stale lease` | nothing written | today, `TestR18PersonTypedGuessedEpochStaysStale` (r18 test :63-73) |
| R7a | person, `--lease <job>` (a Job lease), no epoch | 1 | `gen.fence` (orchestrator.go:317 skips a lease other than `task`, so :730 refuses the empty epoch) | nothing written | today |
| R7b | person, `--lease <job>` with a typed `--lease-epoch` (the r18 `-lease J-8 -lease-epoch 0.16` case) | 3 | `stale lease` (store.go:146: a control carries no attempt) | nothing written | today |
| R10 | person of the team on another machine (the same member, another machine id), no epoch, holder `worker@<this machine>` | 0 | as N1 | as N1: one unleased control, writer `<member>@<other machine>` | new. No machine is compared (rule point 4). |
| R8 | person, target Job unknown in the task | 1 | `grid job cancel: no Job <id> in this task` | nothing written | new. The check runs under the orchestrator lock, as PostPersonUnless loads its snapshot (internal/orchestrator/person_write.go:57-68). |
| R9 | person, target already Accepted, Cancelled or Rejected when the request is posted | 1 | `grid job cancel: <id> is already <state>` | nothing written | new |
| N1 | person, holder is `worker@<machine>`, Job Running | 0 | `Saved. The cancel will be carried out on the next pass of the task's orchestrator.` (worded like orchestrator.go:398) | one unleased control by the person. The Job is still Running until the pass. | new. **Replaces** `TestR18PersonOfAnotherMemberStaysStale` (r18 test :84-88), whose behaviour this decision removes. |
| N2 | the holder's pass after N1 | none | none | one leased control `status: cancelled` by the holder, `re` = the request id. The Job is Cancelled. | new |
| N3 | the pass, given an unleased cancel control authored by role guest | none | none | no leased control. The Job is still Running. | new |
| N4 | the pass, given an unleased cancel control authored by role worker, cli or another agent role | none | none | no leased control. The Job is still Running. | new |
| N5 | an unleased control with any status but `cancelled`, from any role | write refused by schema validation (InvalidRecordError) | the schema error | nothing written | new (control.json leaseless branch allows `cancelled` only) |
| N6 | the fold, given only N1's request | none | none | the Job's state is unchanged (Running) | new |
| N7 | two requests for one Job | none | none | one leased control (the earliest request) and one Feed message naming the other as not used | new |
| N8 | a request whose Job closed between the request and the pass | none | none | no control, and one Feed message saying the request was not used | new |
| N9 | the pass repeated, for example after a stream restart | none | none | still one leased control (the id comes from the request) | new |
| N10 | person whose member is the holder (`<member>@<machine>`), no epoch | 0 | none | leased control at once, the Job is Cancelled, no request | today, `TestR18PersonCancelsRunningJobWithoutAnEpoch` (r18 test :36-43) |

## Files a build lane changes

Plugin only. The app is not expected to change: the worker's plugin stream runs the tick, so it runs the new pass too. That holds only if the app's supervisor stops an agent whose Job another process cancelled. This lane did not verify that, and neither did C-r18. Round 20 step 5 checks it; if the agent runs on, an app lane follows.

| File | Line on plugin main 066d9b2a | Change |
| --- | --- | --- |
| internal/cli/orchestrator.go | :307-321 | After the lease read, keep the leased write when `*held.Holder == GRID_MEMBER@machine`. Otherwise set `request` and write through the unleased writer, as at :323-324. |
| internal/cli/orchestrator.go | :416-426 | For `request`, build the same body as :421 and call the new person cancel write instead of `orchestrator.Write`. |
| internal/cli/orchestrator.go | :397-403 | Print the same "Saved ..." line and the `pending: true` JSON for the request. |
| internal/cli/orchestrator.go | :34-45 | Help text (below). |
| internal/orchestrator/person_write.go | :53-56 | Accept a `control` with `status: cancelled` from a person writer besides `job`, with the R8 and R9 checks inside the lock. |
| internal/orchestrator/person.go | :48 | New `personCancels` beside `personAnswers`. |
| internal/orchestrator/tick.go | :62 | Call `personCancels` after `personAnswers`. |
| contract/schemas/control.json | :300-370 (the lease rules; this lane found them by keyword: `lease` at :302 and :337, `question_event` at :312-315 and :357) | A third, leaseless branch: lease null, fence 0, role owner, member or lead, body status `cancelled` only. Fixtures go under contract/fixtures/control/valid and contract/fixtures/control/invalid, and contract/README.md documents the branch. |
| internal/contract/fold.go | near :303 ("unleased heads stay visible") | An unleased control does not move a Job's state (N6). |
| internal/cli/r18_person_job_cancel_test.go | :80-90 | `TestR18PersonOfAnotherMemberStaysStale` is replaced by N1. The removal is directed by this document. |
| new tests | internal/cli, internal/orchestrator, internal/contract | R8, R9 and N1-N9 |

## `grid job cancel --help` and the orchestrator.go comment

Help, replacing orchestrator.go:37-41:

```
A person (role owner, member or lead) types no lease and no epoch. When the task lease is
held by your own <member>@<machine> (GRID_MEMBER on this machine), the cancel is written
at once under it. When another holder holds it (the machine's worker, worker@<machine>, by
default), the cancel is saved as your request and the holder writes it under its lease on
its next pass; `grid job show <job>` then shows it Cancelled. Both exit 0.
```

Lines :42-43 (the agent and the typed epoch) stay as they are.

Comment, replacing orchestrator.go:307-316:

```
// A person of the team at a terminal holds no lease epoch to cancel a Job with. When the
// task lease names this member's own <member>@<machine>, the cancel is written under the
// lease's current generation and fence, with the person as the author. When another
// holder holds it (by default the machine's worker), the person writes an unleased cancel
// request instead, as a person posts a Job or answers a person Job, and the holder's next
// pass writes the leased control (personCancels beside personAnswers). The person never
// writes under another holder's lease, so store.go's holder check and record.go's
// author check stay as they are. A typed epoch is used as typed, and an agent keeps the
// epochs the worker gave it.
```

## Why C opens no way past the fence

**What the fence is.** A record under the task lease is written only by the current tenure. Two checks enforce that: the holder, generation and fence check at store.go:143, and the ended-tenure check at store.go:153. Under C the person never writes a leased record when another member holds the lease. The holder writes the leased control itself, under its own tenure. Both lines, and record.go:183, stay unchanged.

**What the fence rests on.** GRID_MEMBER and GRID_ROLE come from the environment (orchestrator.go:297, :709), and the plugin authenticates neither. writeAuthor (internal/cli/task_write.go:46-56) "only records: no command admits or refuses anyone because of it". So a process that sets GRID_ROLE=owner is recorded as a person. That is already true for every unleased person write (the Job post at orchestrator.go:301, the answer at person.go:66) and for the C-r18 path (:317). A local process willing to forge its environment can already:

1. read `lease/task.json`;
2. set `GRID_MEMBER` to the holder's member;
3. type `--lease-epoch`.

That is option D, and it works on base. The lease fence orders honest writers and stops a stale one. It was never an identity check against a forging local process, and C does not claim to be one. C adds no new trust in the environment: the request is the same kind of unleased PersonTeam record the product already accepts, and the PersonTeam check is the same one orchestrator.go:301 uses.

**An agent.** The app gives every agent and judge GRID_ROLE=worker (job.go:386, judge.go:221). That is Agent tier, so an agent never takes the request path. An honest stale agent keeps its old GRID_TASK_EPOCH and is still refused with `stale lease`, exit 3 (R4). An agent already holds GRID_TASK_EPOCH and may cancel under it today; the worker does exactly that (supervisor.go:128-131). So the request gives an agent nothing it lacks. The holder's pass also ignores any request whose author tier is not PersonTeam (N4).

**A guest.** A guest acts through the app, which sets GRID_ROLE to the guest's role (person_check.go:118). TierFor(guest) is PersonGuest, so a guest gets the leased refusals R2 and R3. The pass ignores a guest-authored request (N3). This keeps the owner's answer that a guest only answers and comments.

**Nothing else is added.** There is no new guard, cap or hold: no wait, no approval, no count limit. The request path's only check is the PersonTeam tier, which is the fence the unleased Job post already has. R8 and R9 refuse a request that could never act.

## OWNER question

**Q.** When a person of the team cancels a running Job whose task lease another holder holds (by default the machine's worker, `worker@<machine>`), should that holder carry out the cancel, with the person recorded as the one who asked?

**Default: yes, build C.** If the owner does not answer, the build lane builds C. `TestR18PersonOfAnotherMemberStaysStale` is then replaced under this document, as the Files table says. Nothing waits on the answer.

## Round 20 check (F4a)

Run this on a task whose rework Job is Running under `worker@<machine>`, with the worker started by the app (no holder configured).

1. The person's CLI (GRID_MEMBER = the owner's member, GRID_ROLE=owner, no GRID_LEASE_EPOCH, GRID_TASK_EPOCH or GRID_LEASE) runs `grid job cancel <rework> --reason "<why>" --task <task>`.
   - It exits **0**.
   - stderr says `Saved. The cancel will be carried out on the next pass of the task's orchestrator.`
2. `grid feed read` shows the person's unleased control: author = the owner's member, role owner, lease null, status `cancelled`.
3. Within one worker pass, the Feed shows a leased control `status: cancelled` for `<rework>`, written by `worker@<machine>`, with `re` set to the request id.
4. `grid job show <rework>` shows the Job **Cancelled**.
5. **Its agent exits.** The rework's agent process is gone, and the worker.log has the line `exit_superseded` with reason `job cancelled by a person` for the rework. No worker-exit for the rework reaches the Feed: the plugin refuses a post under a cancelled Job's lease (internal/feed/store.go:143-144), and the app worker records the exit as superseded (app internal/kernel/worker/supervisor.go, stampJobCancel and finish, lane F-r22-cancel-exit).
6. The worker log shows **no `judgement_review_cancelled`** for the rework's builder. That event is the judge's for a review cancelled before it ran (app:internal/kernel/worker/judge.go:839-860). The cancel must reach the rework, not its review.
7. DeliveryWait says the rework "was cancelled".

The check fails if step 1 exits 3 (`stale lease`), if the Job stays Running after the worker's next pass, or if the leased control is authored by the person.
