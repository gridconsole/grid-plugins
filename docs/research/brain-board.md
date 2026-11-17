# Brain board: tasks, Jobs and the board as brain nodes (decision)

Base: plugin main `885dc9a6`, app main `e60fab587`. Plugin paths are repo-relative. App paths are prefixed `app:`. Sources: the owner's brain-obsidian brief (Addenda 3 to 6), the run-jobs design brief (Correction and Addenda 2 to 5), the ruling of 17 Nov 02:12 UTC on this lane, and the sibling decision notes R-run-jobs-launcher and R-brain-obsidian. This note uses their names for post types, paths and lanes wherever they already decided them. This is a decision lane: it changes no product code. The one code file it adds is the measurement program for Q2, `docs/research/testdata/boardbench/main.go`, which `./...` never builds.

## Decision

**Decided.**

1. **One immutable file per post.** A post is `projects/<p>/tasks/<T>/board/<seq>-<job>-<type>.md` in the brain. `<job>` is the Job id, or `task` for a post that names no Job. `<seq>` is zero-padded to 6 digits. The file is published once, by writing a dot temp file and then `link`ing it to its final name. The link fails if the name exists, which gives O_EXCL semantics; `saveBody` already publishes this way at internal/feed/store.go:584-606. A published post is never edited. A correction is a new post.
2. **Identity is the header `id` (a ULID), not the file name and not `seq`.** `re`, `after`, `assign_post`, the fold, and every reader use the id. `seq` gives display order and the order of one writer's posts. It does not confer authority, and two posts may share it.
3. **The header is strict YAML.** It holds a fixed key set in a fixed order. The typed body is one line of JSON under `body:`, which is valid YAML flow. The markdown under the header is a human rendering for display only, and the fold never reads it.
4. **No lease and no fence.** Authority comes from two sources:
   - **The task home.** task.md names it as `home: <machine>`. A control counts only when the writer in force at the control's `gen` is the home machine. `gen` is the home generation; it starts at 0 and only a `task-move` post raises it (the person post that `grid task move` writes, R-run-jobs-launcher Q4).
   - **The attempt.** A Job-side post counts only for the current attempt of its Job, and only when the writer is that attempt's `to`. Posts from an older attempt fold as `stale` and change nothing, as R-run-jobs-launcher Q1 also says. This replaces `lease`/`fence`/`takeovers.jsonl`. The ruling sets the attempt rule; the home rule is how it turns into a fold check.
5. **Fold order is `(gen, seq, id)`.** It replaces `(gen, fence, worker, seq)` at internal/contract/fold.go:63-75. Cross-writer causality stays as `re`/`after`, and a post is held until its references arrive (fold.go:86-99, unchanged).
6. **Caches live in the machine folder and are never synced.** These are state.json, needs-you.json, the reader cache, liveness and the seq counter. Locks live there too: the board lock that allocates seq, and the task lock that serializes one machine's orchestrator pass. They are local and order honest writers on one machine. Neither one is a lease or a gate.
7. **task.md gets a status block between markers.** Only the home launcher writes it, after a hash or version check (Q4). The "Needs you" inbox is one brain note with one marked section per home machine.
8. **Screenshots go in `projects/<p>/tasks/<T>/proof/`.** They are linked with standard relative markdown image links (Q5). Videos stay in the machine folder.
9. **Task proposals go to `projects/<p>/Proposals/`** with a `task: <T>` front-matter key. That is the one proposals folder that R-brain-obsidian section 6.2 sets up, so the curator has one place to look. Each proposal is an immutable file.
10. **Search ranks knowledge above history** (Q6). A guest shared one task sees that task's folder only. That is a requirement on the shared-home lane.
11. **Migration is forward only, with a fold-equality check per task** (Q7).

**Why.**

- Immutable files cannot conflict under the sync server's create-only rule (run-jobs Addendum 5A). The only writes that need a version check are the status block and the inbox note.
- The fold never needed a task-wide total order. Today it traverses `(gen, fence, worker, seq)` "only as an implementation traversal". Cross-writer authority comes from controls and granted tenures (fold.go:61-62). Home plus attempt keeps that property without a lock.
- Keeping the id as identity makes a renumbered or duplicate `seq` harmless.
- Reads stay cheap. Posts are immutable, so a reader caches parsed posts by name and reads only names it has not seen (Q2: 5.5 ms to list 6,000 names).

**Rejected.**

| Option | Why not |
| --- | --- |
| Keep `feed/<writer>.<fence>.jsonl` and sync it | An append-only file grows on several machines at once. The sync server cannot merge appends without a lock, and the ruling removes the lease. |
| One shared `board.md` | Every post is an edit, so every concurrent writer conflicts. |
| A task-wide seq allocated by the home machine | Every remote post would need a round trip to the home, and a dead home would stop every post. |
| Seq or file name as identity | Two machines can pick the same seq offline, and a sync collision forces a rename. |
| Writer machine in the file name (`<seq>-<job>-<type>-<machine>.md`) | It avoids name collisions, but it deviates from the owner's name. Collisions are rare, and the rename-before-acceptance rule (Q1, F4) handles them. |
| Typed body as markdown sections or nested YAML | Lossy: exit code `2.0`, key order and bodies over 32 KB must round-trip byte-exactly for the Q7 check. Untrusted text could also forge a section. |
| Keep a task lease through the sync server | The ruling says there is no task lease or lock. The home rule plus attempts covers every case the lease covered (Q1). |

## Q1. Fencing, ordering, stale leases and the fold with attempts

### What exists today

- **The write side.** `Post` checks the lease file under LOCK_SH (store.go:126-157: holder, gen, fence, attempt). It refuses a writer file that a takeover ended (store.go:149-155). Then it allocates seq per writer file under an exclusive file lock (store.go:290-344).
- **Write-time checks that need the whole Feed:**
  - gate ids and Job ids under gate.lock and job.lock (store.go:251-289);
  - record id reuse (store.go:347-355);
  - an ack must name its counted assignment (store.go:371-380);
  - a guest answer must name its question (store.go:382-397);
  - the agent-side Job check in orchestrator/write.go:77-80.
- **The read side.** The reader marks a record `stale` if it lies past a takeover's end offset (read.go:107-117).
- **The fold:**
  - sorts by `(gen, fence, worker, seq)` (fold.go:63-75);
  - treats an unleased control as a person's request that never moves a Job (fold.go:192-198);
  - requires an assign to advance the attempt, or to be a `sameRetry`: the same attempt with a higher fence after `usage_limit` (fold.go:199-209);
  - matches side posts by attempt, `to`, fence and gen (fold.go:217);
  - consumes side posts in order, ack first (fold.go:215-238);
  - takes the newest task-lease `head` by tenure (fold.go:317-322, `newer` at :472-474).
- **The stale-lease rule.** F4a in the r20 check (docs/decisions/D-r19-person-cancel-holder.md:207-221) rests on store.go:143-144 refusing a post under a cancelled Job's lease. Step 5 says no worker-exit for the cancelled Job reaches the Feed, and the app logs `exit_superseded`.

### Confirm or refute

| Property | Verdict | With attempts and per-post files |
| --- | --- | --- |
| Fencing (lease, fence, takeovers) | **Refuted as a mechanism, kept as a property.** A per-post file cannot carry a fence, because nothing grants one across machines. | Attempt replaces the Job fence, and `gen` (home generation) replaces the task tenure. "Only the current tenure writes" becomes "the fold counts only the current attempt's writer, and only the home's controls". |
| Ordering of seq under concurrent writers | **Confirmed on one machine, refuted across machines.** The benchmark: 30 writers under one machine-folder lock produced 0 duplicate seq. Two machines without a shared lock can both pick seq N. | The fold does not need a total order (decision point 5). Controls come from one writer per `gen`, the home under its local lock, so their seq order is the home's order. Side posts of one `(job, attempt)` come from one machine: the launcher and its agent share that machine's lock. So their seq order is that machine's order. Ties break by id. |
| Stale-lease rules (F4a step 5, lease expiry) | **Replaced.** | A post from an attempt that a newer assign superseded is written but folded `stale` ("older attempt"). A post after its attempt's exit is folded `ignored` ("attempt already exited"; fold.go:221-223 already has this case). The launcher still logs `exit_superseded` in its machine log. Lease expiry (`until`, `lease_expires`) becomes the orchestrator's reassignment on a missed start or a stale machine note (run-jobs Addendum, "If an assigned Job is not started in time"). |
| The fold | **Survives with seven edits** (the next table). | It stays an in-memory function of the admitted posts, and no timestamp decides anything (fold.go:31). |

### Fold and write-side edits, one per current use of lease, fence or gen

| Today | Replacement |
| --- | --- |
| fold.go:63-75 sort `(gen, fence, worker, seq)` | `(gen, seq, id)` |
| fold.go:192-198 unleased control means a person's request | A control counts only when its author's role is an orchestrator, cli or launcher role, and its writer `by` names the home machine of `gen` (from task.md `home` and the `task-move` posts). A person's control is always a request, written with the person as author. The home's pass writes the counted control with `re` set to the request. This keeps D-r19 option C as it is. |
| fold.go:199-209 assign must advance the attempt, or be a `sameRetry` with a higher fence | The attempt must strictly advance. A usage-limit retry becomes a new attempt with `retry_of: usage_limit` in the assign body, as the ruling says ("a reassigned Job gets a new attempt number"). Readers that use the attempt as a rework count must skip `retry_of` attempts. The rework count lives in the Job graph (a new rework Job, run-jobs Addendum 3), so none is known to. |
| fold.go:217 side match on attempt, `to`, fence, gen | Match on `job`, attempt, and `by == assign.to`. |
| fold.go:273, store.go:392 guest addressee reads `Lease != nil \|\| Fence != 0` as "leased" | "Not a person-request writer": `by` is a launcher or agent writer. Same meaning, without the lease. |
| fold.go:317-322, :472-474 newest task-lease head by `(gen, fence, seq)` | The newest home-written head by `(gen, seq, id)`. `host` (fold.go:323-326) takes the same rule. Machine notes replace host posts (run-jobs Addendum 2). |
| read.go:107-117 and store.go:149-155 takeover end offsets | Removed. Closing an attempt does the job (row 3 of the previous table). |
| store.go:126-157 lease check; orchestrator/write.go:36-80 Job tenure check | A local check against the local fold: refuse a post whose attempt the local fold already shows closed, with exit 3 and the message "stale attempt". This is not a gate. It refuses only what the fold would ignore anyway, and it saves the agent a wasted post. Across machines the fold is the authority. |
| store.go:251-289 gate.lock and job.lock | Kept as machine-folder locks for one machine. Across machines, the fold keeps the first Job record (fold.go:145-148, already) and the first gate by `(gen, seq, id)`. Job records come only from the home orchestrator, so a Job id collision across machines cannot happen. |
| store.go:347-355 record id reuse; :357-369 wake dedup | Local checks, kept. Across machines the fold drops a duplicate id (fold.go:46). A duplicate wake is harmless, because only the home acts on wakes. |

### Failure cases and how each is handled

| # | Case | Handling |
| --- | --- | --- |
| F1 | Two local writers post at the same moment | The machine-folder board lock serializes seq allocation and publishing. Logins on one machine share the machine folder (run-jobs Addendum 2), so they share the lock. |
| F2 | A crash between the temp write and the link | The temp file lives in `board/.tmp/`, a dot folder: Obsidian hides it and grid sync must skip dot names in the brain (a requirement on the sync lane). The next poster under the lock removes temps older than one hour. Nothing half-written is ever visible. |
| F3 | A partial file delivered by sync | Sync must write atomically (temp plus rename), a requirement on the sync lane. A reader that sees an unterminated or invalid header counts the file `bad` for this pass and re-reads it next pass, because the cache keys on name plus size. |
| F4 | The same name on two machines (same seq, Job and type) | The sync server accepts a create only if the path is absent (Addendum 5A). The losing client renumbers its unaccepted posts to the next free seq: the same id and body, a new name and `seq`. This is allowed because no other machine has read them. The client pushes in seq order and stops at the first rejection, so every renumbered post is still unaccepted. Grid-written links inside those posts are rewritten in the same step. After acceptance a post never changes. |
| F5 | The same seq with different names | Harmless: the fold breaks ties by id, and display sorts by `(seq, id)`. |
| F6 | A post edited or deleted by hand (Obsidian or a shell) | The reader cache keeps the sha256 of each post's bytes when it is first read. `grid brain check` and the launcher's slow pass compare them and report an "edited post" finding in state.json errors and in the status block. The fold reads the file as it is now. A deleted post is a finding and drops out of the fold. The sync server refuses edits to `board/` from agents. A person's edit is kept locally and flagged, never synced over the original. |
| F7 | A post whose `re`/`after` has not synced yet | Held (fold.go:97-99) until it arrives, as today. |
| F8 | A late post from an older attempt (a slow remote machine) | Folded `stale`, reason "older attempt". |
| F9 | An exit after a cancel (F4a step 5) | Written by the launcher, or refused locally by the stale-attempt check. Either way it is folded `ignored` ("attempt already exited"). If the orchestrator has already posted a newer attempt, it is folded `stale`. The launcher logs `exit_superseded`. The status block shows Cancelled. |
| F10 | Two orchestrators: a task moved while the old home is still running | `grid task move` writes a `task-move` person post that raises `gen` and rewrites `home:` in task.md. It is the one person post the fold counts as authority, because the owner moves a task only by this command (run-jobs Addendum 4.2). Controls the old home writes before it sees the move have the lower `gen`, so they sort first, and the new home's controls override them. A conflicting assign loses on the attempt rule. The old home's launcher stops waking its orchestrator for the task once its fold shows another home (run-jobs Addendum 4.2). |
| F11 | Clock skew between machines | `at` is display only (fold.go:31). |
| F12 | No hard links (FAT, exFAT, some network drives) | Fallback: create the final name with O_EXCL, write it, fsync it. A reader skips a header without its closing `---` until the next pass, as in F3. Windows NTFS supports `os.Link`. |

## Q2. Read performance with 30 agents on one task

**Measured.** `go run docs/research/testdata/boardbench/main.go` ran twice in this VM's default temp directory (filesystem type not checked). Each run took about 2 s. Each read row is the mean of 5 repetitions.

| Posts (30 writers) | Write: lock + counter + temp + link | Today: parse 30 jsonl (cold) | List board, names only | List + lstat every name | List + parse every header (cold) | List + parse 30 files |
| --- | --- | --- | --- | --- | --- | --- |
| 1,200 | 574 µs/post (first run, cold cache) | 11.3 ms | 0.60 ms | 7.97 ms | 57.1 ms | 0.90 ms |
| 6,000 | 140 µs/post | 184.6 ms | 5.55 ms | 77.7 ms | 303.4 ms | 6.71 ms |
| 1,200 (second run) | 133 µs/post | 32.0 ms | 0.43 ms | 4.23 ms | 25.8 ms | 1.47 ms |
| 6,000 (second run) | 132 µs/post | 80.7 ms | 5.35 ms | 62.7 ms | 256.2 ms | 32.1 ms |

The two runs differ by up to 5 times on the small rows, which is noise from a shared VM. Two things hold in both runs. Listing names stays near 5 ms at 6,000 posts. The warm read (list plus a few new files) stays an order of magnitude below a cold parse of either format.

**Caveats on the numbers:**

- The "today" row is a cold full parse. The real reader is incremental: it seeks to the cached offset of each file (internal/feed/cache.go:87-115), so a warm read of today's Feed is cheaper than 184 ms.
- The write row has no fsync. `Post` syncs state-class records and new directory entries (store.go:309, :436), so the row is a lower bound.
- The "30 files" row parses the 30 last names. Synced posts arrive out of seq order, so the right warm model is: list the names, diff them against the cached set, and parse each unseen name. That costs the same as the row.
- Fold cost at 6,000 posts was not measured. contract.Fold's side-post loop is O(assigns × active posts) (fold.go:216). The build lane B1 adds a benchmark.

**What replaces the per-writer append:**

1. **A names cache in the machine folder.** `<machine>/tasks/<T>/board.cache.json` maps file name to the parsed post plus its sha256. Posts are immutable, so a warm read is one `Readdirnames` plus a parse of the new names: 1 to 32 ms at these sizes across the two runs. It does not lstat known names; that 78 ms check moves to the slow pass in F6.
2. **state.json and needs-you.json are machine-folder caches**, rebuilt under the machine-folder fold lock when the names digest changes. `ohSqFeedKey` (internal/orchestrator/oh_quiet_task.go:46-76) becomes a digest of the sorted board names alone. No size or mtime is needed, because posts never change.
3. **No sharding and no index file in the brain.** 6,000 names list in 5.5 ms, roughly linear. Revisit above about 20,000 posts per task. An index in the brain would be one more synced edited file, which the ruling rules out.
4. **Posting cost with 30 agents.** Each post takes the machine lock for under 1 ms, plus a refold (`RefoldPublished`, internal/mcp/tools.go:237): one listing plus one new parse plus the fold. At 3 posts/s (30 agents, one post per 10 s each) the lock is idle more than 99% of the time.
5. **Obsidian cost.** Obsidian indexes every `.md` file in the vault. Thousands of post files per task is NOT VERIFIED there. If it is slow, `board/` folders can be added to Obsidian's "Excluded files" setting without losing anything Grid needs.

## Q3. Every reader and writer that changes

### Plugin

The biggest change is that one task folder becomes two roots:

- **The brain:** task.md, intent, plan, research, outcome, `jobs/`, `board/` and `proof/` screenshots.
- **The machine folder:** state.json, needs-you.json, caches, `.live`, usage, sessions and videos.

Every `TaskPath` user that reads a cache must take the machine root. 24 call sites build `feed.Store{TaskDir: ...}`, including internal/approvefix/approvefix.go:39, internal/migrate/receipt.go:108 and :137, internal/task/gates.go:265, internal/cli/ship_land.go:56, internal/cli/deliver_keeper_job.go:111 and :162, internal/cli/deliver_person_job.go:112 and :157, internal/cli/approve_record.go:96, internal/cli/task_new_first_job.go:92, internal/cli/reopen_continue.go:127, internal/cli/reopen_job.go:28, internal/question/feed.go:29 and internal/job/jobcmd/feedjob.go:45. They stay unchanged if the `feed.Store` API is kept as a facade that takes both roots (lane B1).

| File:line | Today | Change |
| --- | --- | --- |
| internal/feed/store.go:57 | `Dir()` = `<task>/feed` | Board dir in the brain, plus a cache dir in the machine folder |
| internal/feed/store.go:111-179 | Lease view, `leaseLock`, `readEnds` | Removed; local stale-attempt check |
| internal/feed/store.go:191-442 | `Post`: tenure file append, per-file seq, 32 KB line, `bodies/` spill | File-per-post publish, task seq under the board lock, no line limit, no spill |
| internal/feed/store.go:570-623 | `saveBody`, `wakeLock` | `saveBody` removed; `wakeLock` moves to the machine folder |
| internal/feed/cache.go:32-132 | Globs `*.jsonl`, offset cache in `feed/.cache/reader.json` | Lists `board/*.md`, names cache in the machine folder |
| internal/feed/read.go:38-136 | Sort by file/offset, takeover stale, `displayOrder` by writer stream | Sort by `(seq, id)`; no takeover |
| internal/feed/read.go:146-158 | `wireValid` checks JSON envelope keys incl. `lease`, `fence` | Strict header decoder |
| internal/feed/fold.go:84-184 | state.json, needs-you.json and fold.lock in `feed/` | Machine folder |
| internal/feed/record.go:149 | `writerFile(worker, fence)` | Removed; post file name builder |
| internal/feed/record.go:183 | Author must be the writer's member | Kept, with `by` as the writer |
| internal/contract/fold.go:63-75, :192-209, :217, :273, :317-326, :472 | Lease, fence and tenure rules | Q1 edits table |
| internal/contract/schema.go:434 | Takeover file name check | Removed with takeover.json |
| contract/schemas/envelope.json, control.json, worker-exit.json, takeover.json, lease.json, handover.json, transfer.json | Lease, fence, takeover | Envelope gets `by`, `seq`, `gen`, `attempt` and `legacy`, and loses `lease` and `fence`. Assign loses `fence` and `until` and gets `retry_of`. The launcher types of R-run-jobs-launcher Q7 are added (`started`, `exit` with `cost`, `waiting`, `wake`, `cancel`, `task-move`). Takeover, lease, handover and transfer are retired; old records read as `unknown` (fold.go:101-104). |
| internal/lease/task.go (all), internal/lease/job.go:46-111 | Task lease, Job grants, takeovers, expiry | Removed. Liveness is read from the launcher's machine-folder records (run-jobs lane). |
| internal/orchestrator/stream.go:236-361, :399-432, :471-500 | `grid worker next`: AcquireTask, epoch, spawn line `fence`/`task_epoch` (:130, :308-322), ReleaseTask, handover | Without a lease: the home check (task.md `home`) decides whether this machine runs the pass. Spawn lines carry job, attempt and `to`. |
| internal/orchestrator/tick.go:24, :207, :241-254, :396-416, :447-468 | Requires the task lease, GrantJob fence, observed fence, `.live/<to>.<fence>.json` | Assign with attempt only; liveness keyed by `(job, attempt)` |
| internal/orchestrator/write.go:36-80 | Lease and Job tenure checks | Stale-attempt check |
| internal/orchestrator/store.go:119 | `WithLease` around the pass | Machine-folder task lock |
| internal/orchestrator/oh_quiet_task.go:46-76 | Digest of `*.jsonl` and `lease/` | Digest of board names |
| internal/mcp/tools.go:40-44, :199, :237 | `feed_post`, `feed_read`, `feed_state`, `feed_handover` ("Identity and lease come from the session") | Same tools; `feed_handover` retired; identity and attempt come from the session |
| internal/mcp/stale.go | Stale lease env | Stale attempt env |
| internal/cli/feed.go:119, :178, :193-223 | Store resolution and refold | Two roots |
| internal/cli/orchestrator.go:195, :307-321, :450-491, :617, :709-740 | Writer with lease and epoch; the C-r18 person epoch path | Writer `by`; the person path always writes a request (D-r19 C) |
| internal/cli/cli.go, internal/cli/global.go, internal/cli/mcp.go, internal/question/command.go, internal/hook/events.go | `GRID_LEASE_EPOCH`, `GRID_TASK_EPOCH`, `--lease-epoch` | Removed. GRID_ATTEMPT stays. About 60 test files under internal/cli name these variables; lane P-loop-retire rewrites them. |
| internal/hook/events.go:385-393, internal/hook/pe_job_message.go:38-46 | Stat `feed/`, read posts of one Job | Stat `board/` |
| internal/hook/hook.go:405-427 (:417) | Job context from `jobs/<id>/brief.md` | `jobs/<J-n>.md` (the Job node) |
| internal/hook/brain_why.go:32 | `jobs/<id>/job.yaml` | Front matter of `jobs/<J-n>.md` |
| internal/hook/history.go:235 | Skips `feed` | Skips `board` |
| internal/proof/proof.go:144-168 | `proof/<id>/<ulid>.<ext>` + edited `proof.json` | Q5 |
| internal/brain/query.go:107-152 | Searches knowledge notes only | Q6 kinds |
| internal/hook/hook.go:147 | SessionStart topic index | Unchanged (Q6) |

### App (`app:` at `e60fab587`)

**Through the CLI.** These readers are safe if the JSON of `grid feed read/state --json` keeps its shape. Lane B1 fixes the shape: records keep `id`, `type`, `seq`, `attempt`, `author`, `verdict`, `reason`, `waiting` and `body`, gain `by` and `file` (the board file name), and lose `lease`, `fence`, `end` and `gen`. `worker` stays as an alias of `by` for one release.

| File:line | Reads |
| --- | --- |
| app:internal/slices/feed/args.go:12, :16; app:internal/slices/feed/fold.go:29, :51, :69, :119, :239-246 | `lease_expires`, `epoch` and `takeover_open`; computes "stale" from lease expiry. It must read start deadlines or liveness instead. |
| app:internal/slices/feed/post.go:101 | `feed post --type post` |
| app:internal/slices/gates/args.go:14, :16 | feed state and read |
| app:internal/slices/projectsettings/read.go:406 | feed state |
| app:internal/slices/taskcode/build.go:179, :186 | feed state and read |
| app:internal/slices/taskview/feed.go:23, app:internal/slices/taskview/view.go:480 | feed state |
| app:internal/slices/taskview/view_feed.go:20, :38, :379 | `worker`, via `memberOf(r.Worker)` |
| app:internal/slices/usage/report.go:322, :501, :505 | `lease` field; feed state and read |
| app:internal/kernel/worker/agentbuild.go:438, supervisor.go:119, judge.go:414, :423 | feed state and read (the run path, which leaves the app: run-jobs Addendum 5B) |
| app:internal/kernel/worker/epoch.go:61, judge.go:932 | `lease status task` (removed with the lease) |
| app:internal/kernel/worker/relaypull.go:197 | feed post |

**Direct file readers.** Each one changes.

| File:line | Reads | Change |
| --- | --- | --- |
| app:internal/slices/tasks/store.go:619-630 (and the ops note at app:internal/kernel/ops/table_gen.go:56) | Activity = newest mtime of `feed/*.jsonl` | Newest board name, or its header `at` |
| app:internal/slices/feed/http.go:82, app:internal/slices/feed/post.go:246 | `feed/` exists | `board/` |
| app:internal/slices/gates/model.go:430 | `feed/needs-you.json` | Machine folder, or the CLI |
| app:internal/slices/taskcode/build.go:167, :594 | `feed/` exists; `feed/<ref>` spilled body | `board/`; no spill |
| app:internal/slices/taskview/posts.go:74 | `feed/.orchestrator.json` | Machine folder |
| app:internal/slices/taskview/store.go:51, view.go:752, view_warm.go:46 | Inventory and existence of `feed/` | `board/` |
| app:internal/slices/usage/report.go:519, :929, :1104, :1144 | Lists `feed/`, skips the state.json and needs-you.json projections | `board/` plus the machine folder |
| app:internal/kernel/worker/job.go:106-150 | `feed/.live/<holder>.<fence>.json`, `feed/lease/<id>.json`, `takeovers.jsonl` | Removed with the app run path |

**Web UI and guest scope.**

| File:line | Reads | Change |
| --- | --- | --- |
| app:internal/kernel/webui/app/client.mjs:534-542 | The `task_feed` op shape (`epoch`, `lease_expires`, `head`) | Drop `epoch`; `lease_expires` becomes `start_by` |
| app:internal/kernel/webui/app/pages/task/feed-tab.mjs:2, :139 | Comment on `feed/*.jsonl`; status ranks | Comment and `stale` wording |
| app:internal/kernel/webui/jstest/task-feed-r25-3.test.mjs:23 | Asserts the UI writes no `.jsonl`, `state.json` or `feed/lease` | Extend to `board/` |
| app:internal/kernel/projectaccess/access.go:12, :84-100 | `GuestProjects`: a project-level guest scope | Per-task folder scope (Q6) |

## Q4. The status block in task.md

**Format.** It goes directly after the front matter. It is an Obsidian callout between HTML comment markers, and every time in it is absolute:

```markdown
<!-- grid:status v1 sha256=3f2a… -->
> [!grid] building
> - J-4 running on grid2 since 2026-11-17 01:40 UTC
> - J-5 waiting on review
> - 1 question for you: [[0042-J-5-question]]
<!-- /grid:status -->
```

- The `sha256` in the opening marker is the hash of the lines between the markers, so Grid can tell its own bytes from a hand edit inside the block.
- There are no relative times ("5 min ago"). The rendered bytes change only when the state changes, so syncing the block causes no churn.
- The block uses no `## ` heading. The app's section parsers, which end a section at the next `## ` (app:internal/slices/taskview/outcome.go:44-56, app:internal/slices/taskview/store.go:243-249), and the plugin's task.md header reader (internal/task/hold.go:89) are unaffected.
- The brain-obsidian rule "description from the first paragraph" must skip the status block. The vault reader classifies task.md as a task node, not a note, and takes its description from the title and intent.
- The rendering of the callout and markers in Obsidian is NOT VERIFIED (the VM has no Obsidian).

**The single writer.** The launcher on the task's home machine renders the block from the fold, with no LLM (run-jobs Addendum 4.4). No other process writes between the markers. The other Grid writers of task.md are internal/task/rename.go:92-116 and internal/task/deliver.go:141-272 (`commitWrite` under `.task.lock`). They keep the block's bytes as they are. They share the machine-folder task lock with the status writer, and they use the same version-checked write.

**When it is rewritten.** After any fold whose rendered block differs from the bytes on disk. That means a new post changed a Job status, a question opened or closed, or the home moved. Most posts, such as checkpoints and heartbeats, do not change it.

**How Obsidian edits are kept:**

1. Read task.md and note its version: the sync version, or the sha256 of the bytes on one machine.
2. Replace only the bytes between the markers. If the markers are missing, insert the block after the front matter. Every other byte is copied.
3. Write with the version it was based on. The sync server rejects a stale write. On one machine the writer re-hashes under the machine-folder lock just before the rename.
4. On a rejection, or a hash mismatch, re-read and repeat from step 1. A person's save always wins, and the block is applied again on top of it.
5. A hand edit inside the markers (the hash in the marker differs) is replaced on the next write and logged as a finding. The block is Grid's alone (brain-obsidian Addendum 5.1).

**The inbox ("Needs you").** One brain note, `Needs you.md`, at the brain root. Each home machine owns one section between `<!-- grid:inbox machine=<m> -->` markers. It writes that section the same way as the status block, so every section has a single writer and the file itself is version-checked (Addendum 4.6).

## Q5. Proof screenshots in the brain

- **Path.** `projects/<p>/tasks/<T>/proof/<job>-<row>.<ext>`, for example `proof/J-4-row2.png`, with `-2`, `-3` for more shots of one row. The extension is the one captured: png, jpg, webp or gif (proof.go:138). After compression it is png or jpg, because R-brain-obsidian section 7.3 compresses to PNG or JPEG. Today's layout is `proof/<id>/<ulid>.<ext>` plus `proof.json` (internal/proof/proof.go:144-168).
- **proof.json.** It is an edited file and goes away. Each proof record (kind, sha256, file, plan_hash, row) goes into the body of the result or checkpoint post that adds it. A machine-folder cache can rebuild the list.
- **Link form.** A standard relative markdown image link: `![J-4 row 2](../proof/J-4-row2.png)` from a board post, `![J-4 row 2](proof/J-4-row2.png)` from outcome.md. A bare `![[J-4-row2.png]]` embed is ambiguous, because the same name repeats in every task, and its resolution depends on the vault's link setting. A relative path does not. It works when the brain is a folder inside a larger vault, and it renders in the app and on GitHub. Rendering in Obsidian is NOT VERIFIED.
- **Videos.** They stay in the machine folder, `<machine>/tasks/<T>/videos/`. The post names the machine and the relative path as text. Obsidian cannot open them; the app can.
- **Compression** is the curator's (P-brain-obsidian-decide). Posts are immutable, so the curator's compressed file replaces the image (or sits beside it under its new extension, with the original removed) and the curator writes a new `post` recording the file, the old and new sha256, and the sizes. The original post's sha256 then names the first image, and the new post explains the change.

## Q6. Ranking and context

- **Search.** `brain.Store.Search` (internal/brain/query.go:107-152) scores knowledge notes only today. It gains a node kind: `note`, then `task`, `job`, `post` (history), and `proposal`. `post` is the kind that R-brain-obsidian section 3.4 left open as `board`: one board file is one post. Proposals are left out unless asked for. Results sort by kind class first (knowledge before history), then by score. `--kind` filters. Only posts of type `result`, `question`, `answer` and `found`, plus task.md, outcome.md and Job nodes, are indexed. Checkpoints and heartbeats are not, so a 6,000-post task adds at most a few hundred entries. The index stays a machine-folder cache.
- **Library agent.** agents/library.md:18 gets one rule: answer from knowledge first. Cite history (task, Job, post) only when no note answers, and label it history. A decision made in a task becomes knowledge only when the curator files a note linking back to the task (Addendum 3 rule 1).
- **Agents start from their Job node.** The hook injects the Job's lines from `jobs/<J-n>.md` (internal/hook/hook.go:405-427). Those lines are the brief with its described links. The agent follows links on demand.
- **SessionStart** still injects only the topic index (internal/hook/hook.go:147, pinned by internal/hook/sessionstart_index_only_test.go). Tasks and posts never enter it.
- **Guests.** The share unit is the folder `projects/<p>/tasks/<T>/`. A guest sees task.md, `jobs/`, `board/` and `proof/` of that task, and nothing in `grid/`, the topic folders or other tasks. Links from the task into notes show as unresolved for the guest. The app's scope is per project today (app:internal/kernel/projectaccess/access.go:84-100), and the sync server needs per-folder ACLs. Both are requirements on the shared-home lane.

## Q7. Migration of existing tasks

`grid migrate board` is forward only and bumps `gridhome.FormatVersion` (internal/cli/feed.go:197-203 already refuses a mismatched format). It runs per task:

1. **Stop old writers.** Take the old task lease the usual way (`AcquireTask`, internal/lease/task.go:237). No old writer can append during the copy, and a task with a live holder is skipped and reported, not forced.
2. **Fold the old Feed** with the old reader and fold, kept as `internal/migrate/legacyfeed`. This gives S_old.
3. **Write one post file per admitted record**, that is, every record except `bad` lines. Each post keeps the old record's `id`, `type`, `type_v`, `author`, `at`, `re`, `after`, `job`, `attempt` and body bytes. A spilled body is inlined. `by` is the old `worker`, and `gen` is the old `gen`. The new `seq` follows the old canonical order `(gen, fence, worker, seq)`. Each post gets `legacy: <old verdict>`.
4. **The new fold honours `legacy`** for posts that carry it, and only for the checks it cannot recompute: lease and fence authority, takeover stale, and `sameRetry` grants. `counted` grants authority and `stale` stays ignored. Every other rule is recomputed. This covers:
   - records stale only by takeover offset;
   - grants that are `sameRetry` (same attempt, higher fence);
   - controls written by an earlier holder on another machine.
5. **Set `home:` in task.md** to the machine of the last task-lease holder. Move `jobs/<id>/brief.md` plus `job.yaml` into `jobs/<id>.md` with front matter, and move the brain part of the folder into the brain (the vault layout comes from P-brain-obsidian-decide).
6. **Fold the new board** to get S_new. Compare S_new with S_old after mapping both:
   - **Drop** the fields the new format removes: `assign.fence`, `assign.until`, `lease_expires`, `gen`, and verdicts of retired types.
   - **Compare** everything else: per Job `status`, `attempt`, `assign.{job,attempt,to}`, `assign_post`, `results`, `submits`, `history`, `open_questions`, `acceptance`, `checkpoint`, `exited`, `exit_reason`, `exit_code`; then `questions`, `needs_you`, `by_ref`, `head`, `wakes`, and the verdict kind of every id.
   - **If equal:** write the new state.json in the machine folder and move the old `feed/` to `<machine>/migrated/<T>/feed/`. It is kept, not deleted.
   - **If not equal:** remove the new board, leave the task untouched, print the first differing path, and go on to the next task. The command exits non-zero when any task stayed behind.
7. Release the old lease.

## Q8. The interface both sibling lanes rely on

**Post file.** `board/<seq>-<job>-<type>.md`. Its header keys, in this order:

| Key | Notes |
| --- | --- |
| `id` | ULID |
| `task` | Task ULID |
| `type` | A contract type |
| `type_v` | |
| `job` | Id or null |
| `attempt` | Int or null |
| `gen` | Home generation |
| `seq` | |
| `at` | RFC 3339 UTC, ms |
| `author` | `{id, role}` in YAML flow |
| `by` | `<machine>:<login>` |
| `for` | Grant link or null (run-jobs Addendum 4.5) |
| `re` | Id or null |
| `after` | List of ids |
| `legacy` | Migrated posts only |
| `body` | One-line JSON, validated against `contract/schemas/<type>.json` exactly as today |

The markdown below the header is display only.

**Go API.** R-run-jobs-launcher Q7 proposes the interface `board.Board`, with `Tasks`, `Read(since Cursor)` in write order, `Post` and `Claim(Key)`. Lane B1 implements it for per-post files and keeps `feed.Store` as a facade for today's callers:

| Method | Per-post files |
| --- | --- |
| `Tasks` | The task folders under `projects/*/tasks/` in the brain whose fold has an open Job (from the machine-folder state cache) |
| `Read(t, since)` | List `board/`, diff the names against the cursor (a digest plus the name set, kept in the machine folder), parse the new names, and return them in `(gen, seq, id)` order. Q2 measured 1 to 32 ms for this. |
| `Post(t, p)` | Under the machine-folder board lock: take the next seq, write `board/.tmp/<name>`, link it to `board/<name>`, refold |
| `Claim(t, k, p)` | Under the same lock: if the names cache holds a post with the key's `(job, attempt, type, firing)`, return `ErrClaimed`; otherwise `Post`. The file name stays `<seq>-<job>-<type>.md` and is not keyed. On one machine the lock makes the claim exactly-once. Across machines no race exists, because one attempt names one `machine:login` (R-run-jobs-launcher Q7 point 5). If two copies still meet after sync, the fold counts the first by `(gen, seq, id)` and folds the other `ignored` ("duplicate claim"). |

This deviates from the deterministic claim name that R-run-jobs-launcher suggests (`board/<job>-a<n>-started.md`). The reason: one name scheme keeps every board listing in seq order, and that note already places the cross-machine guarantee on the attempt, not on the name.

**CLI** (unchanged names, the stable interface for the app and for shell callers): `grid feed post|read|state --json`, with the record shape given in Q3.

**The launcher (P-run-jobs-decide):**

- **Post types** are the ones R-run-jobs-launcher Q7 lists: `assign`, `started`, `exit`, `waiting`, `wake`, `cancel`, `task-move`, `landed`, `push-request` and `push-lost`. `started` replaces today's `ack` and `exit` replaces `worker-exit`. The fold rules that `ack` has today (first side post, names its assign, fold.go:215-238) move to `started`.
- **Exit** carries `reason` (done, crash, usage_limit, cancelled, lost, start_lost, account_missing), `code` (the exit code, `no_result`, or the usage reset) and `cost` (R-run-jobs-launcher Q3). Today's enum (contract/schemas/worker-exit.json:34-40) has neither the new reasons nor `cost`. The contract lane P-loop-board owns the change.
- **Reading Jobs.** The launcher reads `jobs/<J-n>.md` front matter plus the fold's `assign`. The start condition is carried in the assign body (R-run-jobs-launcher Q2).
- **Writing.** It writes only through `board.Board` or the CLI, never by hand. The status block and the inbox section use the Q4 writer.

**The vault reader (P-brain-obsidian-decide):**

- It classifies by path. `projects/<p>/tasks/<T>/board/*.md` is a post: never a knowledge note, never "adopted" as a person edit (F6 flags it instead), and never in the topic index. `tasks/<T>/jobs/*.md` is a Job node, and `tasks/<T>/task.md` is a task node.
- It skips dot folders (`board/.tmp/`).
- Its "person edits are adopted" rule excludes the status block and the inbox sections.

## Q9. Lane plan

Three of the lanes this note needs are already in R-run-jobs-launcher's lane plan, because they edit the same files. Two lanes editing fold.go, internal/lease or the status writer would collide, so this note adds its acceptance rows to those lanes instead of opening new ones:

- run-jobs lane 1, P-loop-board;
- run-jobs lane 5, P-loop-home;
- run-jobs lane 11, P-loop-retire.

Order: P-loop-board, then B1, then B3 and P-loop-home, then P-loop-retire, B5 and A1. B3 must land before P-loop-retire removes internal/lease, because migration step 1 takes the old lease. Each build lane runs the whole-module tests and the walk tests as usual.

| Lane | Repo | Model, effort | Files | Acceptance rows | Depends on |
| --- | --- | --- | --- | --- | --- |
| **P-loop-board** (run-jobs lane 1), plus these rows | plugin | opus, xhigh (format) | Its files, plus contract/schemas/envelope.json (`by`, `seq`, `gen`, `legacy`; no `lease`, `fence`) and the retired takeover, lease, handover and transfer schemas | Added rows: fold sorts `(gen, seq, id)`. A control counts only from the home of its `gen`, and a non-home control is a request. A `task-move` raises `gen` (F10). A usage-limit retry is a new attempt with `retry_of`. `legacy` verdicts are honoured. A duplicate claim folds `ignored`. Each row is a test that fails on base. | none |
| **B1 P-board-store** | plugin | opus, xhigh (format) | the per-post adapter of `board.Board` (header codec, publish by link with the F12 fallback, `Claim` under the board lock, names cache, machine-folder caches); internal/feed facade; internal/mcp/tools.go; internal/cli/feed.go; internal/orchestrator/oh_quiet_task.go (digest of names) | Header round-trips byte-exactly (exit code `2.0`, a 64 KB body). 30 concurrent goroutines give unique names. `Claim` twice from two processes gives one post and one `ErrClaimed`. F2, F3, F5 and F6 tests. A fold benchmark at 6,000 posts. The CLI JSON shape of Q3 is pinned by a golden. | P-loop-board; P-grid-config-read (machine folder and brain path) |
| **B3 P-board-migrate** | plugin | opus, xhigh (migration) | internal/migrate (legacyfeed, board), internal/gridhome format bump, the task folder split into brain and machine folder, Job nodes, task proposals into `projects/<p>/Proposals/` | On 3 fixture homes (one with a takeover, one with a `sameRetry`, one with a person cancel request) S_new equals S_old. A forced mismatch leaves the task untouched and exits non-zero. A task with a live holder is skipped. | B1; the R-brain-obsidian layout lane (its section 7) |
| **P-loop-home** (run-jobs lane 5), plus these rows | plugin | opus, xhigh | Its files; the status block in the Q4 format; the inbox note `Needs you.md` | Added rows: a hand edit outside the markers survives a rewrite. An edit inside the markers is replaced and logged. A hash change between read and write causes a retry, not an overwrite. The bytes do not change when the state does not. Two homes write two inbox sections without loss. | B1 |
| **P-loop-retire** (run-jobs lane 11), plus these rows | plugin | opus, high | Its files, plus internal/orchestrator/write.go (stale-attempt check), internal/mcp/stale.go, the `GRID_TASK_EPOCH`/`GRID_LEASE_EPOCH` readers in Q3, and the tests that pin epochs | Added rows: no `GRID_TASK_EPOCH` or `GRID_LEASE_EPOCH` remains. D-r19 rows N1 to N9 hold with home authority. F4a step 5: the cancelled attempt's exit folds `ignored` and the Job shows Cancelled. `internal/migrate/legacyfeed` still builds. | P-loop-home, B3 |
| **B5 P-board-search-proof** | plugin | sonnet, high | internal/brain/query.go and its index, agents/library.md, internal/proof (records in posts, new path), internal/hook/hook.go and brain_why.go job context | Knowledge ranks above history for an equal score. `--kind` works. The SessionStart index-only test still passes. A screenshot lands at `proof/J-n-rowN.png` and is linked relatively from its post. | B1; the R-brain-obsidian vault reader lane |
| **A1 A-board-readers** | app | opus, high | the direct readers in Q3 (tasks/store.go, feed/http.go, feed/post.go, gates/model.go, taskcode/build.go, taskview/{posts,store,view,view_warm}.go, usage/report.go), slices/feed/fold.go (`epoch`, `lease_expires`), webui feed-tab and client.mjs, the jstest walk at task-feed-r25-3 | Every Q3 app row reads `board/` or the CLI shape. The Feed tab renders a migrated fixture home. No reader opens `feed/*.jsonl`. | B1's JSON golden |
| **Requirement on P-shared-home** | app and sync | (its own lane) | app:internal/kernel/projectaccess/access.go; sync server ACLs | A guest shared T-n reads only `projects/<p>/tasks/<T-n>/` | B3 layout |
| **Requirement on the sync lane** | sync | (its own lane) | grid sync | Create-only for `board/`, `jobs/` and proposals. Version-checked edits. Skip dot names. Atomic delivery. | none |

The app worker's lease and takeover readers (app:internal/kernel/worker/job.go:106-150, epoch.go:61, judge.go:932) leave with the app run path in the run-jobs build lanes, not in A1.

## NOT VERIFIED

- Obsidian rendering of the status callout and its markers, of relative image links, and of a one-line JSON `body:` in the Properties view.
- Obsidian indexing speed with thousands of post files.
- Hard-link behaviour on Windows network drives and on exFAT (the F12 fallback).
- Fold cost at 6,000 posts (left to lane B1).

DONE
