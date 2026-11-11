# D-ie: who may write a knowledge note through `grid brain write`

Status: decision, for the re-filed build briefs that replace C-hr-plugin-brain-write and C-hr-app-knowledge-write.
This document changes no code. It settles the six points of the hr review once.

Base shas:
- plugin `91fadeff` (the cited files). Lane base is plugin main `ee8a96c1`. Of the cited files only contract/README.md, internal/cli/cli.go, internal/cli/task.go, contract_inventory_test.go and guest_parity_test.go differ between them. A build brief re-reads those line numbers on its own base.
- app `34f89245`. It is an ancestor of app main `a82ad6eb`. Of the cited app files, app main changes only cmd/grid-server/wire.go (one line) and internal/slices/brain/read.go.
- app lane `lane-M-app-route-train-2` at tip `1d9f3f99`. It is not on app main, so vault_write.go is cited from that tip.

Paths below are plugin paths unless they start with `app`.

## The six points, settled

1. **`--by` and the audit identity.** Decision: no flag names a person. `--by` is not a `brain write` flag and exits 2 as an unknown flag. The actor comes from the environment the host sets: GRID_WHO, else GRID_ACTOR (the order of internal/cli/task.go:113-117 at 91fadeff). GRID_MEMBER is not read for the actor. The app sets all three to the same person (app 34f89245 internal/kernel/plugincli/person_check.go:118), so they differ only in a person's own terminal and in a Job, which sets GRID_MEMBER and GRID_ROLE=worker but not GRID_WHO (app 34f89245 internal/kernel/worker/job.go:368-369) and is refused with exit 3 anyway. braincmd.Env gains one field, `Actor`, filled beside `Role` at internal/cli/brain.go:14; write, drop and restore share it (the same name as D-ie-notes-drop-upload-decision, app 637111b7 docs/decisions/D-ie-notes-drop-upload-decision.md:177, :212). The audit line names `person:<actor>` when brain.ValidName(actor) (internal/brain/note.go:83), else the session origin from brain.SessionOrigin() (internal/brain/store.go:109-112), as revert does (internal/braincmd/changes.go:244). The note's front matter keeps its kinds: session, job, server, import (internal/brain/note.go:118-122). `person` is written only into log.md, whose parser takes any `kind:id` (internal/brain/changes.go:75-79).
2. **Exit codes.** Decision: 6 is a stale rev, 7 is not versioned. Never 3 and never 4 for either. 4 is disk full for every command (internal/cli/exitcode.go:10, :19-20) and the app worker retries it (app 34f89245 internal/kernel/worker/supervisor.go:104-106). 3 stays the role refusal (internal/braincmd/changes.go:236-238). 5 is taken and the app skips it (app internal/kernel/plugincli/client.go:26, :105-106). exitStatusHelp (exitcode.go:10) gains `6 stale rev (read again), 7 not versioned (nothing written)`. The same 6 and 7 are used by `brain drop` and `brain restore`, as this lane's brief states and as D-ie-notes-drop-upload-decision does (app 637111b7 docs/decisions/D-ie-notes-drop-upload-decision.md:53 P9). Main gives no reason against it: no plugin command returns 6 or 7 today. One meaning for 7 in all three verbs: the note or its folder is not versioned. A note is versioned when knowledge/versions holds a copy of it, which every publish writes (recordVersion, internal/brain/versions.go:80); a hand-placed file has neither that copy nor an accepted line in log.md, so the two tests agree on it. A folder is versioned when it is a knowledge topic: only a knowledge topic has versions (internal/brain/revert.go:96). So a topic that is not knowledge (machines, skills) is 7 for write, drop and restore alike.
3. **Check order.** Decision: the app runs its own role check, then calls the plugin, and the plugin does every on-disk check under the brain lock. The app never writes the file. The on-disk type check of the route-train-2 vault write (app 1d9f3f99 internal/slices/brain/vault_write.go:193-198) moves into the plugin under the lock. Since owner answer 3 (11 Nov) it refuses no role the verb lets in: owner and member write a `type: workflow` note too. See (d).
4. **No fall-through.** Decision: exit 7 is a refusal that writes nothing. The app answers 409 `refused` ("the note has no accepted version") and never writes the file itself. App code words: the knowledge write is an op of train 2's vault_write, so it speaks that route's words, the same as drop and restore: 400 `bad_request`, 403 `forbidden`, 409 `conflict` with the current rev, 409 `refused` (app 1d9f3f99 internal/slices/brain/vault_write.go:80, :82, :88, :208). The workflow route's 403 `role_refused` (app 34f89245 internal/slices/workflows/write.go:50) is a different route and is not copied. The app reads only the exit code: the server's runner has no KeepFailedJSON (app 34f89245 cmd/grid-server/wire.go:775-776) and failed stdout stays private (app internal/kernel/plugincli/client.go:137-140). So every outcome the app must tell apart has its own exit code.
5. **Path rows.** Decision: an absolute path and a three-part path fail `<topic>/<name>` with brain.ValidName (internal/brain/note.go:83) before the lock and exit 2. A symlinked topic folder and a symlinked file fail safefile.NoSymlinks (internal/safefile/file.go:17; used at internal/brain/store.go:115) under the lock and exit 2, as `grid workflow write` refuses a linked target with 2 (contract/README.md:478 at 91fadeff). None writes a file, a version or an audit line.
6. **`valid:false`.** Decision: `brain write` never writes an invalid note; it validates with Note.Validate (internal/brain/note.go:93) and exits 2. A note that is invalid on disk (a hand edit) is listed by `brain index` and `brain show` with `"valid":false`, and the app answer keeps the `valid` field. `brain search` leaves it out, as on main. Main today: index counts it only in the `invalid` column (internal/brain/index.go:83-89) and scan drops a note that fails Validate (internal/brain/topics.go:241), so search never returns it; whether show (internal/brain/query.go:35) errors or drops it is a build-time check; the app brain slice has no `valid` field (`git grep '"valid"' 34f89245 -- internal/slices/brain` is empty). The build adds the field; see (f) question 2.

## (a) Who may

Verb: `grid brain write <topic>/<name> --rev <16-hex|none> --file <path|-> [--level home|project] [--project <p>] [--json]`. `--rev none` means the note must not exist. The rev is the first 8 bytes of SHA-256 as 16 lower hex, the audit hash without its `sha256:` prefix (internal/brain/versions.go:36-38).

Role rule for the verb: brain.RevertAllowed (internal/brain/revert.go:22, :25-31): the empty role, `owner`, `member`. Host key `brain_revert_roles` (revert.go:17).
Role rule for a `type: workflow` note (new text or on disk): the verb's rule, the empty role, `owner` or `member`. Reason: owner answer 3 of 11 Nov (owner-answers/2026-11-11T2107Z.md): "anyone can store things in the brain, author is written ... No owner-only gate on brain writes." The author is written in the audit line (point 1). Guests and workers stay refused by the verb's rule ("guest per role rules already in the product").

| Row | owner | member | guest of the project | guest of another project only | signed out | hosted session | agent Job (GRID_ROLE=worker) | empty role (own terminal) |
|---|---|---|---|---|---|---|---|---|
| `grid brain write` new note (`--rev none`) | 0 | 0 | exit 3 | exit 3 | no CLI call; the host answers 401 | the person's role (owner 0, member 0, guest exit 3) | exit 3 | 0 |
| `grid brain write` replace at a rev | 0 | 0 | exit 3 | exit 3 | no CLI call; 401 | the person's role | exit 3 | 0 |
| replace or create a `type: workflow` note | 0 | 0 | exit 3 | exit 3 | no CLI call; 401 | the person's role (owner 0, member 0, guest exit 3) | exit 3 | 0 |
| app knowledge write route | 200 | 200, a workflow note too (owner answer 3) | 404 `not_found` | 404 `not_found` | 401 | the person's role | no route; a worker has no app session | no route; the app always sets a role |

Notes on the table:
- The plugin cannot tell a guest of this project from a guest of another; both carry GRID_ROLE `guest` and both exit 3.
- The app route copies the brain slice: a guest, with or without grants, gets 404 `not_found` (app 34f89245 internal/slices/brain/http.go:95-99). A request with no person gets 401 (app cmd/grid-server/wire.go:573).
- A hosted session runs the plugin with the person's GRID_ROLE (app person_check.go:117-118), so its cell is that person's cell.
- Agents lose nothing. A worker keeps `propose` and `accept` as on main: `accept` has no role check (internal/braincmd/command.go:50, :134) and `brain write` is a new verb (command.go:38 has no write). A worker proposes, the curator (an agent) accepts, and the brain keeper never asks a person. This document adds no person approval and no role check to `propose` or `accept`.

## (b) Refusals

| Exit | stderr (plain) | When | App status and `code` | Rule it copies |
|---|---|---|---|---|
| 2 | `grid brain write: flag provided but not defined: -by` and the usage line | `--by`, any unknown flag, missing `--rev` or `--file`, a `--rev` that is neither `none` nor 16 hex | 400 `bad_request` | flag parse as runRevert, internal/braincmd/changes.go:216-218 |
| 2 | `brain write: expected <topic>/<name>. Run: grid brain help` | absolute path, three-part path, a part that fails ValidName | 400 `bad_request` | changes.go:226-232; internal/brain/note.go:83 |
| 2 | `grid brain write: <reason from Note.Validate>` | the new text fails Note.Validate | 400 `bad_request` | internal/brain/note.go:93-122 |
| 2 | `grid brain write: linked or non-regular target` | symlinked topic folder or symlinked note file, seen under the lock | 400 `bad_request` | contract/README.md:478; internal/safefile/file.go:17 |
| 3 | `grid brain write: caller role is not allowed to write brain entries` | RevertAllowed(GRID_ROLE) is false (guest, worker) | 403 `forbidden` (the app checks first, so this is a backstop) | internal/braincmd/changes.go:236-238; internal/brain/revert.go:22 |
| 6 | `grid brain write: note changed since read` | `--rev` does not match the file under the lock, `--rev none` and the file exists, or a rev and no file | 409 `conflict` with the current `rev` | contract/README.md:474 with 6 in place of 3; app 1d9f3f99 vault_write.go:208 |
| 7 | `grid brain write: note is not versioned; nothing written` | the target folder is not a knowledge topic (machines, skills; revert.go:96), or the note exists but knowledge/versions holds no copy of it, so replacing it would lose its bytes | 409 `refused` "the note has no accepted version"; the app writes nothing | point 4; versions folder internal/brain/versions.go:19-22, :80 |
| 1 | `brain: <error>` | store error (a full disk included), unregistered `--project` | 502 `read_or_plugin_failed` | failWith, internal/braincmd/changes.go:25-36; app workflows/write.go:131-132 |

Rules for the mapping:
- No refusal uses exit 4 or 5.
- Under `--json` the failure object goes to stderr, as failWith does (changes.go:30-31). Exit 6 adds `{"error":"conflict","rev":"<current>"}`. The app does not see it (point 4) and reads the rev again.
- The app reads exit 3 the way workflows write does: the plugin client returns a FinalError for 3 (app internal/kernel/plugincli/client.go:103-104), so the route checks both ExitError and the process exit code (app workflows/write.go:118-125). Exit 6 and 7 arrive as ExitError (client.go:107-108).

## (c) Negative rows a build brief must carry

| # | Verb or route | Caller | Input | Expected | Must not happen |
|---|---|---|---|---|---|
| 1 | `grid brain write` | owner | `/abs/x --rev none` | exit 2, usage line | no file, no version, no audit line, no index change |
| 2 | `grid brain write` | owner | `a/b/c --rev none` | exit 2, usage line | same |
| 3 | `grid brain write` | owner | `notes/x` where knowledge/notes is a symlink to a folder | exit 2, `linked or non-regular target` | nothing written through the link or beside it |
| 4 | `grid brain write` | owner | `notes/x` where notes/x.md is a symlink | exit 2, `linked or non-regular target` | the link target is unchanged |
| 5 | `grid brain write` | member | `notes/x --rev <old>` after another write | exit 6, `note changed since read` | note bytes, versions and log.md unchanged |
| 6 | `grid brain write` | owner | `machines/x --rev none`, a folder that is not a versioned knowledge topic (the unversioned folder) | exit 7, `not versioned; nothing written` | same; no folder made |
| 6b | `grid brain write` | owner | `notes/x --rev <current>` where notes/x.md has no stored version | exit 7, same text | same |
| 7 | app knowledge write | owner | plugin exits 7 | 409 `refused` | no raw write by the app, no file |
| 8 | `grid brain write` | GRID_ROLE=member | replace a note whose file says `type: workflow` | exit 0, the note written, an `edited` audit line naming the member (owner answer 3, 11 Nov) | a refusal; an audit line without the author |
| 9 | `grid brain write` | GRID_ROLE=member | new note whose text says `type: workflow` | exit 0, the note written, a `created` audit line naming the member (owner answer 3) | a refusal; no file |
| 10 | app knowledge write | member | replace a `type: workflow` note | 200, the note written (owner answer 3) | a 403 |
| 11 | `grid brain index --json` | owner | a hand-made note that fails Validate | listed with `"valid":false`, counted in `invalid` | not silently dropped |
| 12 | `grid brain search` | owner | words that match that note | not in hits | not returned as usable |
| 13 | `grid brain show --json` | owner | that note's id | `"valid":false` | no error that hides the note |
| 14 | app brain read | member | that note | answer keeps `"valid":false` | the field is not dropped |
| 15 | `grid brain write` | GRID_ROLE=guest | new note | exit 3, `caller role is not allowed to write brain entries` | no file, no audit line |
| 16 | `grid brain write` | GRID_ROLE=guest, grants on another project only | new note | exit 3, same | same |
| 17 | `grid brain write` | GRID_ROLE=worker | new note | exit 3, same | same; the Job is not retried (3 is final, app supervisor.go:104-108) |
| 18 | `grid brain write` | guest, other-project guest, worker | replace at a rev | exit 3, same | same |
| 19 | `grid brain write` | guest, other-project guest, worker | `type: workflow` note | exit 3, the role line of row 15 (role is checked first) | same |
| 20 | app knowledge write | guest of the project | any note | 404 `not_found` | no plugin call |
| 21 | app knowledge write | guest of another project only | any note | 404 `not_found` | no plugin call |
| 22 | app knowledge write | signed out | any note | 401 | no plugin call |
| 23 | `grid brain write` | owner | `--by ana` | exit 2, unknown flag | no audit line names `person:ana` |
| 24 | `grid brain write` | GRID_WHO=bo, GRID_MEMBER=ana; and GRID_ACTOR=cy, GRID_MEMBER=ana with no GRID_WHO | a good write | exit 0, audit `person:bo`; then `person:cy` (GRID_MEMBER never names the actor) | front matter origin kind stays `session` |
| 25 | `grid brain write` | owner | unregistered `--project` | exit 1 through failWith | no file |

Rows 1 to 4 are the four path rows of point 5. Row 5 is the stale rev. Rows 6, 6b and 7 are the unversioned folder and note. Rows 8 to 10 are a member's workflow-note write. Rows 11 to 14 are the `valid:false` rows. Rows 15 to 22 are the refusal cells of (a).

## (d) Order of checks

1. Flags: parse, `--rev` form, `--file`, operand form and ValidName. Exit 2. (Before the role, a usage error leaks nothing.)
2. Role: RevertAllowed(GRID_ROLE). Exit 3. Before reading input, as `grid workflow write` refuses before reading input (contract/README.md:478).
3. Input: read the file and Note.Validate. Exit 2. A `type: workflow` text is written by owner or member too (owner answer 3).
4. Store: levelStore(..., writes=true) (internal/braincmd/changes.go:49). Exit 1 through failWith.
5. The lock: the level lock, `locked` (internal/brain/store.go:149).
6. Under the lock, on disk: no symlink on the path (exit 2), the rev (exit 6), the versions folder holds the current bytes (exit 7). A `type: workflow` note on disk is replaced by owner or member too (owner answer 3). Then store the version, publish the note atomically, append the audit line.
7. After the lock: RebuildIndex, as revert does (internal/brain/revert.go:174-181).

Why the app calls the plugin only after its own role check: the app answers 404 to a guest without telling it the project exists (app internal/slices/brain/http.go:95-99) and 403 `forbidden` before any plugin call, in the order the workflow write uses for its 403 `role_refused` (app workflows/write.go:49-51, :85-87). Why the app never writes the file itself: the checks of step 6 hold only under the plugin's lock; a check before the lock and a write after it is the hole of point 3. A raw write after a refusal is the hole of point 4. The app reads only the exit code (point 4), so the plugin owns the decision and the app maps it.

## (e) Files a build will touch

Plugin half (cite on the build's own base):
- internal/braincmd/command.go:38 (verb list and a usage line under revert), :92-94 (Run's exit doc), the dispatch beside :123-125; a new file internal/braincmd/write.go with runWrite.
- internal/brain: a new write function beside RevertWith that runs step 5 to 7; the `valid` field on the index row and on Show.
- internal/cli/brain.go:14: `Actor:` beside `Role:`, from GRID_WHO, else GRID_ACTOR. The same field drop and restore use; whichever lands second keeps the one `Actor` field.
- internal/cli/exitcode.go:10: add 6 and 7 to exitStatusHelp. testdata/golden/help.golden follows.
- internal/brain/note.go:118-122: unchanged. The audit names the actor in log.md only; front matter kinds stay session, job, server, import.
- internal/cli/contract_inventory_test.go: one row beside the brain rows (:521-523 at 91fadeff), `"brain write": {args: []string{"<topic>/<name>"}, required: map[string]bool{"--rev": true, "--file": true}}`.
- internal/cli/guest_parity_test.go: argv beside :34 and a want beside :441-442 with exit 3 and the role line of (b).
- internal/cli/gg_unknown_project_walk_test.go: no row. ggProjectKnownOpen has no brain rows since 91fadeff (hr review row 1 point 4); the walk takes members from contract/commands.json and fails a known-open member that answers no_such_project (:290-291, :334). The verb passes by sending an unregistered project through failWith.
- testdata/golden/brain-help.golden (:1 copies command.go:38).
- contract/commands.json, regenerated only with `UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1`, never by hand.
- internal/cli/cli.go:121, the brain summary line, adds `write`.
- contract/README.md: a `### grid brain write` section after `### grid brain revert` (:619 at 91fadeff) with the exit list of (b).

App half:
- api/ops.yaml vault_write (app 34f89245 api/ops.yaml:2218-2227): served, plugin `grid brain write`.
- The brain slice: the knowledge write route calls the plugin through the person's runner and maps exits as (b). It adds `valid` to its read answer.
- TestEveryWiredMount and the guest walks get the new route (guest 404, signed out 401).

Merge order: plugin half, then the plugin-dist rebuild, then the app half. `brain write` and `brain drop`/`restore` add the same exit codes to exitStatusHelp and the same Env field; whichever lands second rebases on the first and keeps one line and one field.

## (f) Owner questions

Each has a default, the more closed choice. A build follows the default until the owner answers.
1. May `--by` name another person? Default: no. The flag does not exist; the actor comes from GRID_WHO, then GRID_ACTOR; GRID_MEMBER is not read for it.
2. Is a `valid:false` note shown with a warning or left out? Default: shown in `brain index` and `brain show` with `"valid":false`, left out of `brain search`, kept in the app answer.
3. May a member write a `type: workflow` note through `brain write`? Answered 11 Nov (owner-answers/2026-11-11T2107Z.md, answer 3): yes, owner or member; "anyone can store things in the brain, author is written ... No owner-only gate on brain writes." Guests and workers stay refused.
4. Should `brain write` store an unversioned note's bytes as version 1 and then write? Default: no; exit 7 and nothing written.

Quoted, not asked: a worker does not write knowledge notes directly; it proposes and a curator accepts, and no person is asked (owner, 8 and 10 Nov). The hi review's owner answers stand and are not asked again.

## Agreement with the build briefs

- C-hr-plugin-brain-write (withdrawn): the brief's text is not readable in this lane's sandbox (it sits in a research folder that is not on this machine). The changes it needs, from the hr review points quoted in this lane's brief: before `--by <person>` sets the audit origin; after: no `--by`, actor from GRID_WHO, else GRID_ACTOR, in Env field `Actor` (point 1). Before exit 4 stale rev; after exit 6, and 7 for not versioned (point 2). Before exit 1 `not_versioned`; after exit 7, nothing written (point 4). Add the path rows 1 to 4 and the `valid:false` rows 11 to 14 of (c). Line numbers: NOT VERIFIED.
- C-hr-app-knowledge-write (withdrawn): not readable here either. Changes: before, the plugin is called before the lock and the on-disk check runs in the app; after, the app checks the role, calls the plugin, and maps 3/6/7 as (b): 403 `forbidden`, 409 `conflict`, 409 `refused` (point 3). Before, `not_versioned` falls through to a raw write; after, 409 `refused`, nothing written (point 4). Before, the answer drops `valid`; after, it keeps it (point 6). Line numbers: NOT VERIFIED.
- C-hy-plugin-drop-restore (the dispatched brief text, /srv/gridwork/tmp/hand/prompt-builder-C-hy-plugin-drop-restore.txt): agrees on exit 6 for a stale rev (:29, :40), exit 3 for the role with RevertAllowed (:27, :40), no `--by` (:28, :40), `person:` in the audit only, never 4 or 5. Changes:
  - :28 before "GRID_WHO, else GRID_ACTOR (the order of internal/cli/task.go:113-116), else GRID_MEMBER"; after "GRID_WHO, else GRID_ACTOR (the order of internal/cli/task.go:113-116); GRID_MEMBER is not read for the actor".
  - :29 before "7 not versioned (`brain write` only; drop and restore never return it)"; after "7 not versioned: the note has no copy in knowledge/versions, or its topic is not a knowledge topic (write, drop and restore alike)".
  - :35 before "Env field `Member`" and "`Member:` beside Role on :14, filled from GRID_WHO, else GRID_ACTOR, else GRID_MEMBER"; after "Env field `Actor`" and "`Actor:` beside Role on :14, filled from GRID_WHO, else GRID_ACTOR".
  - :38 before "a topic whose Kind is not knowledge, or name `TOPIC`, is ErrDropRefused"; after "a topic whose Kind is not knowledge, or a note with no copy in knowledge/versions, is ErrNotVersioned (exit 7); name `TOPIC` is ErrDropRefused".
  - :40 before "`Origin{Kind: "person", ID: env.Member}` when brain.ValidName(env.Member)"; after "`Origin{Kind: "person", ID: env.Actor}` when brain.ValidName(env.Actor)". Before "never 4, 5 or 7"; after "7 not versioned `{"error":"not_versioned"}`; never 4 or 5", and Run's doc and the two usage lines list 7 too.
  - The older text this lane's brief describes (exit 4 for a stale rev): before `4`, after `6`. The dispatched text already says 6.
- D-ie-notes-drop-upload-decision (app lane, 637111b7, docs/decisions/D-ie-notes-drop-upload-decision.md): agrees on 6 stale rev and 7 not versioned for drop and restore (:53 P9, :126 R-P9). It agrees on the actor from GRID_WHO, else GRID_ACTOR, with GRID_MEMBER not read (:177), and on the Env field `Actor` (:212). It agrees on the app words 400 `bad_request`, 403 `forbidden`, 409 `conflict` with `rev`, and 409 `refused` for exit 7 (:49, :53). Its test for 7 (no accepted line in log.md) and this one (no copy in knowledge/versions) both mark a hand-placed file; a build uses the versions copy. Changes:
  - :59 P15 before "topic not knowledge (machines, skills) or name `TOPIC` | 9 | `{"error":"refused","reason":"not a knowledge note"}`"; after "topic not knowledge (machines, skills): 7 `{"error":"not_versioned"}`, 409 `refused`; name `TOPIC`: 9 as before". Reason: only a knowledge topic has versions (revert.go:96), and `brain write` answers 7 for the same folder (row 6 of (c)).
  - :132 R-P15 before "`machines/m`, `skills/s`, `x/TOPIC` | exit 9"; after "`machines/m`, `skills/s` exit 7; `x/TOPIC` exit 9".
  - :177 before "the origin stays brain.SessionOrigin() as revert does (changes.go:244) and the actor goes in the audit line only"; after "the audit line's origin is `person:<actor>` when brain.ValidName(actor), else brain.SessionOrigin() as revert does (changes.go:244); no front matter is written".
  - :212, its change to C-hy :40 ("The origin is brain.SessionOrigin() and the audit line carries the actor") and to the test line ("actor `ana`" / "actor `bo`"); after: the audit line's origin is `person:ana` / `person:bo`, set through GRID_WHO or GRID_ACTOR, as point 1 and row 24 of (c).
