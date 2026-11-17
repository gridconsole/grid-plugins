# Brain board: tasks, Jobs and the board as brain nodes (decision)

Base: plugin main `885dc9a6`, app main `e60fab587`. Plugin paths are repo-relative. App paths are prefixed `app:`. Sources: the owner's brain-obsidian brief (Addenda 3 to 6), the run-jobs design brief (Correction and Addenda 2 to 5), the ruling of 17 Nov 02:12 UTC on this lane, and the sibling decision notes R-run-jobs-launcher and R-brain-obsidian. This note uses their names for post types, paths and lanes wherever they already decided them. This is a decision lane: it changes no product code. The one code file it adds is the measurement program for Q2, `docs/research/testdata/boardbench/main.go`, which `./...` never builds.

## Owned ledger rows

This note decides four rows of the decision ledger of 17 Nov (column "Decided in"). V9 is shared with P-run-jobs-decide: the part where the orchestrator assigns and starts Jobs is decided there, and the board side is decided here. The last column is the ledger's track or check column; the letters are the test plan's tracks in order (B plugin alone, E failures and recovery, F the brain, J scale and endurance).

| Row | Ledger decision | Where this note decides it | Lanes that carry it (Q9) | Ledger check |
| --- | --- | --- | --- | --- |
| V9 | No task lease; a reassigned Job gets a new attempt | Decision point 4 (task home and attempt), decision point 6 (local locks are not leases), Q1 (fold edits, F8 to F10), Q8 header key `attempt` | P-loop-board, B1, B3, P-loop-home, P-loop-retire, the sync requirement | E |
| V15 | Tasks, Jobs and the board are in the brain; board per task, one markdown file per post | Decision points 1 to 3, Q3 (two roots), Q7 (migration), Q8 (post file, header, and "Post body and links") | B1, B3, B5, A1, the P-shared-home requirement, the sync requirement | F, J |
| V16 | task.md opens with a status block, written only by the home launcher | Decision point 7, Q4, and decision point 4 for which machine is the home | P-loop-home | unit + B |
| V17 | Screenshots are task attachments, kept compressed by the curator | Decision point 8, Q5 (path, link, format, tool, size target, readability check) | B5 (path and link), L5b P-brain-curator-shots (compression; the lane is in R-brain-obsidian.md:818-835 and its section 7.3 at :605-621, and its V17 rows are set in Q5 and Q9 here) | unit |

## Decision

**Decided.**

1. **One immutable file per post.** A post is `projects/<p>/tasks/<T>/board/<seq>-<job>-<type>.md` in the brain. `<job>` is the Job id, or `task` for a post that names no Job. `<seq>` is zero-padded to 6 digits. The file is published once, by writing a dot temp file and then `link`ing it to its final name. The link fails if the name exists, which gives O_EXCL semantics; `saveBody` already publishes this way at internal/feed/store.go:584-606. A published post is never edited. A correction is a new post.
2. **Identity is the header `id` (a ULID), not the file name and not `seq`.** `re`, `after`, `assign_post`, the fold, and every reader use the id. `seq` gives display order and the order of one writer's posts. It does not confer authority, and two posts may share it.
3. **The header is strict YAML.** It holds a fixed key set in a fixed order. The typed body is one line of JSON under `body:`, which is valid YAML flow. The markdown under the header is a rendering of the header and typed body for people and for Obsidian's links, and the fold never reads it. It carries path-qualified `[[links]]` to the task, the Job node and the posts it answers (Q8, "Post body and links").
4. **No lease and no fence.** Authority comes from two sources:
   - **The task home.** A control counts only when its writer is the home machine of the control's `gen`. `gen` is the home generation; it starts at 0 and only a `task-move` post raises it (the person post that `grid task move` writes, R-run-jobs-launcher Q4). The fold derives each gen's home from the posts alone:
     - gen 0's home is the `home` field in the body of the task's first Job record, which `grid task new` writes;
     - each `task-move` body carries `from` and `to`, and gen n+1's home is its `to`.

     task.md `home:` is a display copy that the move rewrites, and the fold never reads it. So a refold after a move still counts the old home's controls for the old gen.

     **One source for every home check: the posts.** The home of the current gen, as the fold derives it, decides three things: which machine runs the orchestrator pass (internal/orchestrator/stream.go today), which launcher wakes the orchestrator, and which launcher writes the status block and its inbox section. Nothing reads task.md `home:` to decide anything. The evidence:
     - **The move is a post.** The owner: "Moving a task to another machine is a deliberate act by a person: `grid task move T-n --to <machine>`, recorded as a board post. Nothing moves a task by itself" (run-jobs Addendum 4.2). The sibling note follows it: "The old home's launchers stop at that post and the new home's start" (R-run-jobs-launcher Q4, "How", the Task move item).
     - **task.md can disagree with the move.** "task.md names the task home machine" (run-jobs Addendum 2.3) stays true for people: `grid task new` writes `home:` with the first Job record, and `grid task move` rewrites it after the post. But task.md is an edited, synced file. It can arrive before or after the `task-move` post, and a person can change `home:` in Obsidian, which is not a move. If machines read task.md, two machines could both act, or neither, until the file caught up. Every machine with the same posts derives the same home.
     - **Authority already comes from the posts.** A pass on a machine that task.md names but the posts do not would write controls that the fold does not count (decision point 4).

     **When they differ, the posts win.** The home launcher logs a finding and rewrites `home:` with the version-checked write of Q4. That launcher is already a writer of task.md, so this adds no writer.

     **Deviation from R-run-jobs-launcher.** That note says the home is "named in task.md and moved only by `grid task move`" (its Decision note, point 6) and that "only that machine's launchers act" on the task (Q4, "How", the Home item). This note keeps task.md naming the home for people, but the launchers and the pass act on the posts' home. Reason: the move itself is a post (Addendum 4.2), the same note already stops the old home "at that post", and the two sources can differ for a while after a move.
   - **The attempt.** A Job-side post counts only for the current attempt of its Job, and only when the writer is that attempt's `to`. Posts from an older attempt fold as `stale` and change nothing, as R-run-jobs-launcher Q1 also says. This replaces `lease`/`fence`/`takeovers.jsonl`. The ruling sets the attempt rule; the home rule is how it turns into a fold check.
5. **Fold order is `(gen, seq, id)`.** It replaces `(gen, fence, worker, seq)` at internal/contract/fold.go:63-75. Cross-writer causality stays as `re`/`after`, and a post is held until its references arrive (fold.go:86-99, unchanged).
6. **Caches live in the machine folder and are never synced.** These are state.json, needs-you.json, the reader cache, liveness and the seq counter. Locks live there too: the board lock that allocates seq, and the task lock that serializes one machine's orchestrator pass. They are local and order honest writers on one machine. Neither one is a lease or a gate.
7. **task.md gets a status block between markers.** Only the home launcher writes it, after a hash or version check (Q4). The "Needs you" inbox is one brain note with one marked section per home machine.
8. **Screenshots go in `projects/<p>/tasks/<T>/proof/`.** They are linked with standard relative markdown image links (Q5). They are WebP or optimised PNG, never JPEG, and the curator compresses them in place with `grid shot compress` (Q5). Videos stay in the machine folder.
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
| fold.go:192-198 unleased control means a person's request | A control counts only when its author's role is an orchestrator, cli or launcher role, and its writer `by` names the home machine of `gen`, derived from the first Job record's `home` and the `task-move` posts' `from`/`to` (decision point 4), never from task.md. A person's control is always a request, written with the person as author. The home's pass writes the counted control with `re` set to the request. This keeps D-r19 option C as it is. |
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
| F3 | A partial file delivered by sync | Sync must write atomically (temp plus rename), a requirement on the sync lane. A reader that sees an unterminated or invalid header counts the file `bad` for this pass and does not cache it, so the next pass reads it again as an unseen name. |
| F4 | The same name on two machines (same seq, Job and type) | The sync server accepts a create only if the path is absent (Addendum 5A). The losing client renumbers its unaccepted posts to the next free seq: the same id and body, a new name and `seq`. This is allowed because no other machine has read them. The client pushes in seq order and stops at the first rejection, so every renumbered post is still unaccepted. Grid-written links inside those posts, and inside any unaccepted post that links to them, are rewritten in the same step by re-rendering the body (Q8, "Post body and links"). An accepted post cannot link to an unaccepted one, because it was pushed first. After acceptance a post never changes. |
| F5 | The same seq with different names | Harmless: the fold breaks ties by id, and display sorts by `(seq, id)`. |
| F6 | A post edited or deleted by hand (Obsidian or a shell) | The reader cache keeps the sha256 of each post's bytes when it is first read. `grid brain check` and the launcher's slow pass compare them and report an "edited post" finding in state.json errors and in the status block. The fold reads the file as it is now. A deleted post is a finding and drops out of the fold. The sync server refuses edits to `board/` from agents. A person's edit is kept locally and flagged, never synced over the original. |
| F7 | A post whose `re`/`after` has not synced yet | Held (fold.go:97-99) until it arrives, as today. |
| F8 | A late post from an older attempt (a slow remote machine) | Folded `stale`, reason "older attempt". |
| F9 | An exit after a cancel (F4a step 5) | Written by the launcher, or refused locally by the stale-attempt check. Either way it is folded `ignored` ("attempt already exited"). If the orchestrator has already posted a newer attempt, it is folded `stale`. The launcher logs `exit_superseded`. The status block shows Cancelled. |
| F10 | Two orchestrators: a task moved while the old home is still running | `grid task move` writes a `task-move` person post with `from` and `to` that raises `gen`, and it rewrites the display copy `home:` in task.md. It is the one person post the fold counts as authority, because the owner moves a task only by this command (run-jobs Addendum 4.2). Controls the old home writes before it sees the move have the lower `gen`, so they sort first, and the new home's controls override them. A conflicting assign loses on the attempt rule. The old home's launcher stops waking its orchestrator for the task once its fold shows another home (run-jobs Addendum 4.2). |
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
5. **Obsidian cost.** Obsidian indexes every `.md` file in the vault. Thousands of post files per task is NOT VERIFIED there. Grid does not add `board/` to Obsidian's "Excluded files". Excluded files are hidden in Search, Graph and Unlinked mentions, and less noticeable in the quick switcher and link suggestions (R-brain-obsidian section 1, from Obsidian's Settings.md). That would hide the board from the person, against the owner's "a board that is markdown, readable, linked like the brain". If indexing proves slow, that is a question for the owner, not a setting Grid picks.

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
| internal/orchestrator/stream.go:236-361, :399-432, :471-500 | `grid worker next`: AcquireTask, epoch, spawn line `fence`/`task_epoch` (:130, :308-322), ReleaseTask, handover | Without a lease: the home check decides whether this machine runs the pass. The home is the fold's home of the current gen, derived from the posts, not task.md `home:` (decision point 4). Spawn lines carry job, attempt and `to`. |
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
> - 1 question for you: [[projects/demo/tasks/T-7/board/000042-J-5-question|J-5 question]]
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

- **Path.** `projects/<p>/tasks/<T>/proof/<job>-<row>.<ext>`, for example `proof/J-4-row2.png`, with `-2`, `-3` for more shots of one row. Today's layout is `proof/<id>/<ulid>.<ext>` plus `proof.json` (internal/proof/proof.go:144-168).
- **Format: WebP or optimised PNG, no JPEG.** The owner's words: "a format Obsidian shows (WebP or optimised PNG), text still readable … Compression must not alter what the proof shows" (brain-obsidian Addendum 5.2). A screenshot in the brain is a `.png` or a `.webp`, and Grid never writes JPEG. Today a capture can be png, jpg, webp or gif (proof.go:138), so `grid proof add` stores it like this:
  - png and webp are stored as captured;
  - jpg and single-frame gif are decoded with Go's standard `image/jpeg` and `image/gif` and stored as `.png` with the same pixels, so nothing in the proof changes;
  - an animated gif is a video and stays in the machine folder.

  The extension is therefore fixed when the post that links the image is written, and compression never changes it. That matters because a post is never edited: a compressed file under a new name would leave the post's image link dead for good.
- **proof.json.** It is an edited file and goes away. Each proof record (kind, sha256, file, plan_hash, row) goes into the body of the result or checkpoint post that adds it. A machine-folder cache can rebuild the list.
- **Link form.** A standard relative markdown image link: `![J-4 row 2](../proof/J-4-row2.png)` from a board post, `![J-4 row 2](proof/J-4-row2.png)` from outcome.md. A bare `![[J-4-row2.png]]` embed is ambiguous, because the same name repeats in every task, and its resolution depends on the vault's link setting. A relative path does not. It works when the brain is a folder inside a larger vault, and it renders in the app and on GitHub. Rendering in Obsidian is NOT VERIFIED.
- **Videos.** They stay in the machine folder, `<machine>/tasks/<T>/videos/`. The post names the machine and the relative path as text. Obsidian cannot open them; the app can.
- **Compression is the curator's** (V17; the curator step itself is R-brain-obsidian section 7.3, built in its lane L5b). It runs when the curator files a task's proposals or delivers. This note sets the format, the tool, the target and the check, and overrides section 7.3's PNG-or-JPEG rule and its `--max 1568` scaling for proof screenshots:
  - **Tool.** `grid shot compress <file>`, in internal/shot and internal/cli/shot.go. It is part of the grid binary that the curator agent and the brain-keeper skill call, not a separate script (Addendum 5.2). It needs no new dependency: Go's standard `image/png` and `image/draw` are enough.
  - **What it does to a png.** It keeps the pixel size; it never scales or crops. Scaling a 3840-wide screenshot to 1568 would shrink 14 px text to under 6 px.
    1. **Lossless.** It re-encodes with `png.BestCompression`. An image with at most 256 colours is written as a paletted PNG with exactly the same pixels.
    2. **Palette, only when still over the target.** It reduces the image to a 256-colour palette (median cut, no dithering, so glyph edges stay solid). It keeps that result only when it passes the readability check below; otherwise it keeps the lossless file.
    3. **Publish.** It writes to a dot temp file and renames it over the same path, only when the result is smaller.
  - **What it does to a webp.** It leaves it as captured. WebP is already a compressed format Obsidian shows. The Go standard library has no WebP encoder (R-brain-obsidian section 7.3), and go.mod has no image dependency, so Grid writes no WebP itself.
  - **Size target: 300 KB per image** (the figure from R-brain-obsidian section 7.3). It is the curator's aim, not a limit the code enforces. An image still over 300 KB after step 2 is kept at its lossless size and reported as `over_target`; it is never scaled down or turned into JPEG to meet the figure. Whether UI screenshots at 1440 and 3840 wide reach 300 KB without scaling is not measured; L5b records the size of each fixture in its proof table.
  - **How "text still readable" is checked.** Three checks, none of which blocks an agent:
    - **In the tool.** The palette result is kept only when, compared with the source pixels, at least 99.5% of pixels differ by at most 16 in every channel and none by more than 48. Anti-aliased text keeps its edge contrast within that bound. L5b may tune the two numbers on its fixtures, and states the values it pins.
    - **In L5b's tests.** On real UI screenshots at 390, 1440 and 3840 wide, the output has the input's pixel size and passes the bound. A png with at most 256 colours decodes to exactly the input. A fixture with more than 256 colours (small anti-aliased text over a gradient), where the palette fails the bound, keeps the lossless file.
    - **By a person.** In the Mac round trip (R-brain-obsidian section 9.4 step 7) the compressed screenshot is opened in Obsidian at 100% and its text read. It is NOT VERIFIED until that run.
  - **The record.** The tool prints JSON with `file`, `old_sha256`, `new_sha256`, `old_bytes`, `new_bytes`, `mode` (`lossless`, `palette` or `kept`) and `over_target`. The curator writes a new `post` with those fields. The post that added the image still names the first sha256, and the curator's post explains the change. The file is replaced at the same path through the sync server's version-checked edit (the sync requirement in Q9).

## Q6. Ranking and context

- **Search.** `brain.Store.Search` (internal/brain/query.go:107-152) scores knowledge notes only today. It gains a node kind: `note`, then `task`, `job`, `post` (history), and `proposal`. `post` is the kind that R-brain-obsidian section 3.4 left open as `board`: one board file is one post. Proposals are left out unless asked for. Results sort by kind class first (knowledge before history), then by score. `--kind` filters. Only posts of type `result`, `question`, `answer` and `found`, plus task.md, outcome.md and Job nodes, are indexed. Checkpoints and heartbeats are not, so a 6,000-post task adds at most a few hundred entries. The index stays a machine-folder cache.
- **Library agent.** agents/library.md:18 gets one rule: answer from knowledge first. Cite history (task, Job, post) only when no note answers, and label it history. A decision made in a task becomes knowledge only when the curator files a note linking back to the task (Addendum 3 rule 1).
- **Agents start from their Job node.** The hook injects the Job's lines from `jobs/<J-n>.md` (internal/hook/hook.go:405-427). Those lines are the brief with its described links. The agent follows links on demand.
- **SessionStart** still injects only the topic index (internal/hook/hook.go:147, pinned by internal/hook/sessionstart_index_only_test.go). Tasks and posts never enter it.
- **Guests.** The share unit is the folder `projects/<p>/tasks/<T>/`. A guest sees task.md, `jobs/`, `board/` and `proof/` of that task, and nothing in `grid/`, the topic folders or other tasks. Links from the task into notes show as unresolved for the guest. The app's scope is per project today (app:internal/kernel/projectaccess/access.go:84-100), and the sync server needs per-folder ACLs. Both are requirements on the shared-home lane.

## Q7. Migration of existing tasks

`grid migrate board` is forward only and bumps `gridhome.FormatVersion` (internal/cli/feed.go:197-203 already refuses a mismatched format). It runs per task:

1. **Stop old writers.** Take the old task lease the way `AcquireTask` does today (internal/lease/task.go:237). No old writer can append during the copy, and a task with a live holder is skipped and reported, not forced. `internal/migrate/legacyfeed` carries its own copy of everything it needs: this lease take, the takeover reader, the JSONL reader and the old fold. People will run `grid migrate board` after P-loop-retire removes internal/lease, so migration must not import it.
2. **Fold the old Feed** with the old reader and fold, kept as `internal/migrate/legacyfeed`. This gives S_old.
3. **Write one post file per admitted record**, that is, every record except `bad` lines. Each post keeps the old record's `id`, `type`, `type_v`, `author`, `at`, `re`, `after`, `job`, `attempt` and body bytes. A spilled body is inlined. `by` is the old `worker`, and `gen` is the old `gen`. The new `seq` follows the old canonical order `(gen, fence, worker, seq)`. Each post gets `legacy: <old verdict>`. Its markdown body comes from the same renderer as a new post's (Q8, "Post body and links"), so a migrated post links to its task, Job node and `re` post like any other.
4. **The new fold honours `legacy`** for posts that carry it, and only for the checks it cannot recompute: lease and fence authority, takeover stale, and `sameRetry` grants. `counted` grants authority and `stale` stays ignored. Every other rule is recomputed. This covers:
   - records stale only by takeover offset;
   - grants that are `sameRetry` (same attempt, higher fence);
   - controls written by an earlier holder on another machine.
5. **Set the home.** Append one `task-move` post with `from: null` and `to:` the machine of the last task-lease holder. Its `gen` is the highest old gen plus 1, so every post written after migration has a home derived from the posts. Migrated posts keep their old authority through `legacy`. Write the same machine as the display copy `home:` in task.md. The fold-equality check in step 6 ignores this one post. Move `jobs/<id>/brief.md` plus `job.yaml` into `jobs/<id>.md` with front matter, and move the brain part of the folder into the brain (the vault layout comes from P-brain-obsidian-decide).
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

The markdown below the header is display and links only; the fold never reads it.

### Post body and links

The owner asked for "a board that is markdown, readable, linked like the brain … a markdown body with [[links]]" (brain-obsidian Addendum 3). The body under the header is how a post is readable and linked.

**Rules.**

1. **One renderer.** Grid writes the body when it publishes the post. Its inputs are the header, the typed body, the post's own path (which gives the task folder and the Job node path `jobs/<job>`), and the file names of the `re` and `after` posts, taken from the names cache. It reads no file contents, and the same inputs always give the same bytes. That makes three things safe: the F4 renumber re-renders a post when a name it links changes, B3 gives migrated posts the same body (Q7 step 3), and a reader can check a body against its inputs. When a referenced post's file name is not known locally, the line shows its id as text (`re: 01JD…`) with no link.
2. **Layout.** A title line, a short facts list, the type's text, then a `## Links` section last:
   - **Title:** `# <job> <type>: <summary>`, or `# task <type>: <summary>` for a post that names no Job. The summary is the type's summary field from the table below, cut at 80 characters.
   - **Facts:** Job and attempt, author and `by`, the time in UTC, and the post it answers.
   - **Text:** the type's long text fields in full, as written. A line in that text that reads `## Links` is written as `### Links`, so the post keeps one Grid `## Links` section.
3. **Links point back, never forward.** A post is never edited, so it links only to what exists when it is written: task.md, the Job node, the `re` post, the `after` posts, and the proof images this post adds. There are no "answered by" links. Obsidian's backlinks pane shows those from the other side.
4. **Link form.**
   - **Notes and posts.** `[[<path from the brain root, without .md>|<label>]]`, the target form of R-brain-obsidian section 3.1. A bare `[[J-5]]` or `[[000042-J-5-question]]` is not used, for the reason Q5 gives for images: the same name repeats in every task.
   - **Images.** The relative markdown image link of Q5.
   - **`## Links`.** It uses R-brain-obsidian's line format `- [[<target>|<label>]] - <rel>: <why>`, with rels from the existing list (internal/brain/note.go:87):
     - task.md: `related`;
     - the Job node: `continues`;
     - the `re` post: `decides` from an answer to its question, `continues` otherwise;
     - each `after` post: `depends-on`.
5. **The vault reader.** It reads these links as edges, as it reads every body link (R-brain-obsidian section 3.2). So posts appear in Obsidian's graph and backlinks and in the app's link lists. A post is still never a knowledge note (Q8, "The vault reader"), and search lists it as kind `post` (Q6).

**What each type shows.** The field names are today's schemas (contract/schemas/question.json, answer.json, result.json, checkpoint.json, post.json, found.json) and the launcher types of R-run-jobs-launcher Q7, whose body fields P-loop-board fixes. Any other type, and any body key a row does not name, renders as one `- key: value` line per top-level body key after the type's text.

| Type | Summary in the title | Text | Links besides task.md and the Job node |
| --- | --- | --- | --- |
| `assign` | `assigned to <to>, attempt <n>` | start condition; `retry_of` when set | none |
| `started` | `started, attempt <n>` | none | its assign post (`re`) |
| `exit` | `exited: <reason>, code <code>` | cost | its assign post |
| `result` | `outcome` | `outcome`; `changed`, `acceptance`, `must_fix` and `decisions_needed` as lists; one image link per proof record | its assign post |
| `checkpoint` | `step` | `step`; `commit`; `acceptance` as a list; image links | its assign post |
| `question` | `question` | `why`; `options` as a list; `who`; `blocking` | the post it asks about (`re`), if any |
| `answer` | `answer` | `note`; `reason`; the `for` grant link when set | the question (`re`, rel `decides`) |
| `waiting` | `waiting on <unmet atoms>` | the condition and measured values | its assign post |
| `cancel`, `wake` | `cancel` or `wake` | reason; the event post ids as links | `re` and `after` posts |
| `task-move` | `moved from <from> to <to>, gen <n>` | none | none besides task.md |
| `post`, `found` | the first line of `text` | `text` | `re`, if any |

**Example.** The post `projects/demo/tasks/T-7/board/000042-J-5-question.md`:

```markdown
---
id: 01JD7Q4ZK0V8M2T6Y1N3B5C9XE
task: 01JD7P0A1B2C3D4E5F6G7H8J9K
type: question
type_v: 1
job: J-5
attempt: 1
gen: 0
seq: 42
at: 2026-11-17T01:52:10.123Z
author: {id: J-5, role: agent}
by: grid2:grid3
for: null
re: 01JD7Q1W3X5Y7Z9A2B4C6D8E0F
after: []
body: {"question":"Should the inbox keep one section per machine?","options":["yes","no"],"why":"Two homes share one inbox note.","blocking":true}
---
# J-5 question: Should the inbox keep one section per machine?

- Job: [[projects/demo/tasks/T-7/jobs/J-5|J-5]], attempt 1
- From: agent J-5, by grid2:grid3, 2026-11-17 01:52 UTC
- About: [[projects/demo/tasks/T-7/board/000041-J-5-result|J-5 result]]

Two homes share one inbox note.

- yes
- no

Blocking: yes

## Links

- [[projects/demo/tasks/T-7/task|T-7]] - related: the task this board belongs to
- [[projects/demo/tasks/T-7/jobs/J-5|J-5]] - continues: the Job this question comes from
- [[projects/demo/tasks/T-7/board/000041-J-5-result|J-5 result]] - continues: the result the question is about
```

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

- It classifies by path. `projects/<p>/tasks/<T>/board/*.md` is a post: never a knowledge note, never "adopted" as a person edit (F6 flags it instead), and never in the topic index. Its body links are edges like any note's. `tasks/<T>/jobs/*.md` is a Job node, and `tasks/<T>/task.md` is a task node.
- It skips dot folders (`board/.tmp/`).
- Its "person edits are adopted" rule excludes the status block and the inbox sections.

## Q9. Lane plan

Three of the lanes this note needs are already in R-run-jobs-launcher's lane plan, because they edit the same files. Two lanes editing fold.go, internal/lease or the status writer would collide, so this note adds its acceptance rows to those lanes instead of opening new ones:

- run-jobs lane 1, P-loop-board;
- run-jobs lane 5, P-loop-home;
- run-jobs lane 11, P-loop-retire.

Order: P-loop-board, then B1, then B3 and P-loop-home, then P-loop-retire, B5 and A1. Each build lane runs the whole-module tests and the walk tests as usual.

- **The new fold rules wait for migrated data.** P-loop-board runs on today's JSONL feeds, which have per-writer seq and fence-ordered tenures. So the `(gen, seq, id)` sort, home authority, `task-move` gens, `legacy` and duplicate claims land in B1, behind the home's format version (`gridhome.FormatVersion`, which internal/cli/feed.go:197-203 already checks). A home before B3's version bump folds with today's order and authority. A migrated home folds with the new rules. The attempt-stale rule applies to both and stays in P-loop-board.
- **B3 does not depend on internal/lease.** `legacyfeed` carries its own copy of the lease take and the takeover reader (Q7 step 1), so B3 before P-loop-retire is a convenient build order, not a correctness dependency.

Every lane names the ledger rows it carries in the V column, and each of its acceptance rows starts with the V-number it traces to. Each row is a test that fails on base, unless it says it is a person check.

| Lane | V | Repo | Model, effort | Files | Acceptance rows | Depends on |
| --- | --- | --- | --- | --- | --- | --- |
| **P-loop-board** (run-jobs lane 1), plus these rows | V9 | plugin | opus, xhigh (format) | Its files, plus contract/schemas/envelope.json (`by`, `seq`, `gen`, `legacy` accepted; `lease`, `fence` still read for JSONL homes), the `task-move` schema with `from` and `to`, `home` in the Job record body, and internal/contract/schema.go:434 (takeover name check kept for JSONL homes) | Added rows. V9: a usage-limit retry is a new attempt with `retry_of`, and an assign that does not advance the attempt is not counted. V9: a side post from an attempt that a newer assign superseded folds `stale` with reason "older attempt" (F8). V9: a `task-move` without `to` fails validation. V9: the first Job record carries `home`. | none |
| **B1 P-board-store** | V9, V15 | plugin | opus, xhigh (format) | the per-post adapter of `board.Board` (header codec, body rendering, publish by link with the F12 fallback, `Claim` under the board lock, names cache, machine-folder caches); internal/feed facade; internal/feed/fold.go:84-184 (state.json, needs-you.json and fold.lock to the machine folder); internal/contract/fold.go (the new rules behind the format version); internal/mcp/tools.go; internal/cli/feed.go; internal/hook/events.go:385-393, internal/hook/pe_job_message.go:38-46 and internal/hook/history.go:235 (`feed/` to `board/`); internal/orchestrator/oh_quiet_task.go (digest of names) | V9: a fold on a migrated home sorts `(gen, seq, id)`. V9: a control counts only from the home of its `gen`, and a non-home control is a request. V9: after a `task-move`, a refold still counts the old home's controls for the old gen (F10). V9: the fold reports the current home from the posts, and a task.md whose `home:` names another machine does not change it (decision point 4). V9: a duplicate claim folds `ignored`; `Claim` twice from two processes gives one post and one `ErrClaimed`. V9: `legacy` verdicts are honoured, and a JSONL home folds exactly as on base. V15: the header round-trips byte-exactly (exit code `2.0`, a 64 KB body). V15: 30 concurrent goroutines give unique names, published by link. V15: the body under the header is the rendering of Q8 "Post body and links": a golden per post type, with path-qualified `[[links]]` to task.md, the Job node and the `re` post; rendering twice from the same inputs (header, body, own path, linked file names) gives the same bytes, and an unknown `re` name renders as the id without a link. V15: deleting or editing the markdown body leaves the fold result unchanged. V15: an F4 renumber rewrites the links inside the renumbered posts and inside every unaccepted post that links to them. V15: F2, F3, F5 and F6 tests. V15: a fold benchmark at 6,000 posts. V15: the CLI JSON shape of Q3 is pinned by a golden. | P-loop-board; P-grid-config-read (machine folder and brain path) |
| **B3 P-board-migrate** | V15, V9 | plugin | opus, xhigh (migration) | internal/migrate (legacyfeed, board), internal/gridhome format bump, the task folder split into brain and machine folder, Job nodes, task proposals into `projects/<p>/Proposals/` | V15: on 3 fixture homes (one with a takeover, one with a `sameRetry`, one with a person cancel request) S_new equals S_old. V15: a forced mismatch leaves the task untouched and exits non-zero. V9: a task with a live holder is skipped. V15: every migrated post carries the same body rendering that B1 writes for a new post of its type (B1's goldens). V15: `jobs/<id>/brief.md` plus `job.yaml` become `jobs/<id>.md`. V9: after migration the fold's home and task.md `home:` name the same machine. | B1; the R-brain-obsidian layout lane (its section 7) |
| **P-loop-home** (run-jobs lane 5), plus these rows | V16, V9 | plugin | opus, xhigh | Its files; the status block in the Q4 format; the inbox note `Needs you.md`; the home check that decides which launcher wakes | Added rows. V16: a hand edit outside the markers survives a rewrite. V16: an edit inside the markers is replaced and logged. V16: a hash change between read and write causes a retry, not an overwrite. V16: the bytes do not change when the state does not. V16: two homes write two inbox sections without loss. V16: no process other than the home launcher writes between the markers (the status writer is the only caller of the block renderer). V9: only the launcher on the fold's home machine wakes the orchestrator; with task.md `home:` edited by hand to another machine, the wake stays on the fold's home, a finding is logged, and the home launcher rewrites `home:` with the version-checked write. | B1 |
| **P-loop-retire** (run-jobs lane 11), plus these rows | V9 | plugin | opus, high | Its files, plus internal/orchestrator/write.go (stale-attempt check), internal/orchestrator/store.go:119 (`WithLease` to the machine-folder task lock), internal/orchestrator/stream.go (the pass runs when the fold's home is this machine), internal/cli/orchestrator.go (writer `by`, the person path as a request), internal/mcp/stale.go, the `GRID_TASK_EPOCH`/`GRID_LEASE_EPOCH` readers in Q3, and the tests that pin epochs | Added rows. V9: no `GRID_TASK_EPOCH` or `GRID_LEASE_EPOCH` remains, and internal/lease is gone. V9: D-r19 rows N1 to N9 hold with home authority. V9: F4a step 5: the cancelled attempt's exit folds `ignored` and the Job shows Cancelled. V9: a post from a closed attempt is refused locally with exit 3 "stale attempt". V9: the pass on a machine that is not the fold's home does nothing. V9: `internal/migrate/legacyfeed` still builds without internal/lease. | P-loop-home, B3 |
| **B5 P-board-search-proof** | V15, V17 | plugin | sonnet, high | internal/brain/query.go and its index, agents/library.md, internal/proof (records in posts, new path, jpg and single-frame gif stored as png), internal/hook/hook.go and brain_why.go job context | V15: knowledge ranks above history for an equal score. V15: `--kind` works. V15: the SessionStart index-only test still passes. V17: a png screenshot lands at `proof/J-n-rowN.png` and a webp one at `proof/J-n-rowN.webp`, each linked relatively from its post. V17: a jpg or single-frame gif screenshot is stored as `proof/J-n-rowN.png` with the same decoded pixels, and its post links the png (Q5). | B1; the R-brain-obsidian vault reader lane |
| **L5b P-brain-curator-shots** (R-brain-obsidian section 9.1), with these rows in place of its JPEG item | V17 | plugin | sonnet, high (its own) | agents/curator.md, skills/brain-keeper/SKILL.md, internal/shot/shot.go, internal/cli/shot.go: `grid shot compress` (Q5) instead of `grid shot crop --jpeg` | V17: `grid shot compress` on a png writes an optimised png at the same path and the same pixel size, and never writes JPEG. V17: on a png with at most 256 colours the output decodes to exactly the input pixels. V17: the palette step is kept only when the readability check of Q5 passes on it; a fixture with more than 256 colours (small anti-aliased text over a gradient) where the palette fails the bound keeps the lossless png. V17: a webp is left byte-for-byte. V17: the command prints the old and new sha256 and sizes, and the curator writes them in a `post`. V17: on the fixture set (one screenshot each at 390, 1440 and 3840 wide) the lane records each output size against the 300 KB target in its proof table. V17: agents/curator.md and the brain-keeper skill carry the step, the target and the check. V17, person check (R-brain-obsidian 9.4 step 7): the compressed screenshot opens in Obsidian and its text reads at 100%. | L5a; this note |
| **A1 A-board-readers** | V15 | app | opus, high | the direct readers in Q3 (tasks/store.go, feed/http.go, feed/post.go, gates/model.go, taskcode/build.go, taskview/{posts,store,view,view_warm}.go, usage/report.go), slices/feed/fold.go (`epoch`, `lease_expires`), webui feed-tab and client.mjs, the jstest walk at task-feed-r25-3 | V15: every Q3 app row reads `board/` or the CLI shape. V15: the Feed tab renders a migrated fixture home. V15: no reader opens `feed/*.jsonl`. | B1's JSON golden |
| **Requirement on P-shared-home** | V15 | app and sync | (its own lane) | app:internal/kernel/projectaccess/access.go; sync server ACLs | V15: a guest shared T-n reads only `projects/<p>/tasks/<T-n>/` (task.md, `jobs/`, `board/`, `proof/`), and nothing in `grid/`, the topic folders or other tasks. | B3 layout |
| **Requirement on the sync lane** | V15, V9 | sync | (its own lane) | grid sync | V15: create-only for `board/`, `jobs/` and proposals; an edit to a `board/` file from an agent is refused. V15: version-checked edits for task.md, `Needs you.md` and `proof/` files (the curator's in-place compression). V15: dot names are skipped. V15: delivery is atomic (temp plus rename). V9: no task lease or lock is synced; the machine folder is never synced. | none |

V17 compression has its own lane in R-brain-obsidian's lane plan, L5b (R-brain-obsidian.md:818-835; its section 7.3 at :605-621), which edits the curator agent and the shot package. This note cannot edit that plan. For the format, the tool and the check, this note's Q5 overrides R-brain-obsidian section 7.3 and L5b's `grid shot crop --jpeg` item, and the L5b row above is the one that lane takes its V17 rows from.

The app worker's lease and takeover readers (app:internal/kernel/worker/job.go:106-150, epoch.go:61, judge.go:932) leave with the app run path in the run-jobs build lanes, not in A1.

## NOT VERIFIED

- Obsidian rendering of the status callout and its markers, of relative image links, and of a one-line JSON `body:` in the Properties view.
- Obsidian resolving the path-qualified post links when the brain is a folder inside a larger vault (the target form is R-brain-obsidian section 3.1's, and that note owns the prefix rule).
- Obsidian indexing speed with thousands of post files.
- Hard-link behaviour on Windows network drives and on exFAT (the F12 fallback).
- Fold cost at 6,000 posts (left to lane B1).

DONE
