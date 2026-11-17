# P-brain-adopt

Person edits in an Obsidian vault used as the Grid brain are adopted into `.grid/` change records and
content-addressed versions. An agent write is version-checked and redone on top, and becomes a proposal
only when both edits cannot hold. Follows docs/research/brain-obsidian.md 4.3, 5.1-5.4, 6.1, 6.3, 7.1, 7.2.

Branch `lane-P-brain-adopt`, base `origin/main`. The head sha is in the report that carries this file; it
cannot name its own commit.

## What changed

- `internal/brain/adopt.go` (new): change records `<brain>/.grid/changes/<yyyy-mm>/*.change`, versions
  `<brain>/.grid/versions/<level>/<id>/<hash16>.version`, machine-folder caches and the local lock
  (`<MachineDir>/brain/<sha256(real BrainDir)[:8]>/`), adoption (created, edited, renamed, deleted, origin
  person, Proposals/ walked), `merge3` by front-matter key and body section, `vaultWrite` with expect-rev,
  `vaultPropose`, `vaultCheck` (grid integrity and `secret-shaped`), `decorate` (origin, author, updated,
  conflict) and person-first ranking.
- `internal/brain/check.go`: `CheckResult.Adopted`, vault check call. `store.go`: `MachineDir`, `MachineID`.
  `ke_write.go`: vault dispatch. `query.go`: `Hit.Conflict` and `personFirst` before the limit.
- `internal/hook/brain.go` and `hook.go`: `brainAdopt` runs in PreToolUse before an agent Edit, MultiEdit or
  Write on a brain file (after the guards, only when the decision is not deny); `brainRecord` runs in
  PostToolUse. Neither waits on the lock, neither prints a decision, both log and return on failure.
- `internal/braincmd/ke_write.go` (allowed only if the write path needs a flag). It needed no new flag. It
  needed two things: the `<topic>/<name>` check accepts a deeper name when the store is a vault, and the
  Grid-note parse and validate step is skipped for a vault, whose notes are any Markdown. A vault write
  prints `<status>: <path> rev <rev>` with status `created`, `edited` or `unchanged`, or `proposed: <path>`.
- Tests, all new files, helpers prefixed `pbAd`: `internal/brain/pbad_{adopt,write,rank}_test.go`,
  `internal/braincmd/pbad_{write,cmd}_test.go`, `internal/hook/pbad_direct_edit_test.go`.

## Proof table

All commands run from the repo root on the merged tree. "Pkg" = `go test -count=1 <package> -run <name>`.

| Row | Result | Proof |
|---|---|---|
| 1 F7 rename recorded, old id resolves | HOLDS | `TestPbAdRenameIsRecordedAndOldIDResolves`, pbad_adopt_test.go:55, exit 0 |
| 2 F8 delete recorded | HOLDS | `TestPbAdDeleteIsRecorded`, pbad_adopt_test.go:85, exit 0 |
| 3 F9 other lines: both land, no Proposals, exit 0 with path | HOLDS | `TestPbAdOtherSectionIsRedoneOnTop` (pbad_write_test.go:45) and as the command `TestPbAdWriteCommandRedoesOnTop` (braincmd/pbad_write_test.go:39), exit 0 |
| 4 F9 same key: Proposals/, note byte-equal to person's, `proposed:` | HOLDS | `TestPbAdSameKeyMakesAProposal` (:76) and `TestPbAdWriteCommandProposesOnSameKey` (:62), exit 0 |
| 5 write on current rev goes on top, person bytes in `.grid/versions/`, no Proposals | HOLDS | `TestPbAdWriteOnPersonNoteKeepsPersonBytes` (:107) and as the command `TestPbAdWriteCommandOnPersonNote` (braincmd/pbad_cmd_test.go:16), exit 0 |
| 6 direct edit (spaced name, history-log off, no task folder): Pre, agent bytes, Post, check; person version with `edited`, origin person; agent record; no Proposals; both hooks exit 0, no decision, also when `.grid/` adoption fails | HOLDS | `TestPbAdDirectEditAdoptsBeforeAndRecordsAfter` (hook/pbad_direct_edit_test.go:130) and `TestPbAdDirectEditAdoptionFailureIsLoggedOnly` (:171, skipped on Windows or as root), driven through `hook.Run`, exit 0 |
| 6 lock held by another process: pre-hook does not wait, prints no decision, logs the skip | HOLDS | `TestPbAdDirectEditBusyLockSkips` (:216). The lock is held by a re-exec of the test binary (`TestPbAdLockHolderHelper`); the test asserts the `brain adopt skipped` and `brain record skipped` log lines and then adopts after release. No sleep and no elapsed-time assertion; the proof is that the call returns while the lock is still held. |
| 7 `grid brain check` adopts a person edit, no drift finding | HOLDS | `TestPbAdCheckAdoptsWithoutDriftFinding`, pbad_adopt_test.go:110, exit 0 |
| 8 two adoptions of one edit = one logical change | HOLDS | `TestPbAdTwoAdoptionsAreOneChange`, pbad_adopt_test.go:135, exit 0 |
| 9 draft in Proposals/ adopted as `edited`, origin person, version under `.grid/versions/<level>/Proposals/<name>/`, `list --json` origin person | HOLDS | `TestPbAdProposalEditIsAdopted` (pbad_write_test.go:139) and as commands `TestPbAdCheckAdoptsDraftAndListShowsPersonOrigin` (braincmd/pbad_cmd_test.go:48), exit 0 |
| 10 secret-shaped line: one `secret-shaped` finding with file, line N, empty text; bytes unchanged; no proposal | HOLDS | `TestPbAdSecretShapedLineIsOneFindingWithoutText` (pbad_adopt_test.go:183) and as the command `TestPbAdCheckCommandSecretShapedFinding` (braincmd/pbad_cmd_test.go:98, exit code 3 as for any finding) |
| 11 check JSON does not contain the matched value | HOLDS | same two tests; the command test asserts the value, a slice of it and a prefix are absent from stdout and stderr |
| 6.3 person-first ranking, `conflict` in search and listing | HOLDS | `TestPbAdPersonNoteRanksBeforeTheAgentNoteThatSupersedesIt`, pbad_rank_test.go:10, exit 0 |
| `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/hook/` | HOLDS | exit 0, ok on all three, last run on the final tree |
| Importers of the changed packages | HOLDS | `go test -count=1` ok on cli, job, job/jobcmd, job/jobtest, land, people, projectenv, routine, task, workflow, packaging/... |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok, after `git merge origin/main` |
| `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings) |
| `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test-compile) |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints `0` |
| Small commits, one-sentence messages | HOLDS | three lane commits and one merge, see `git log origin/main..HEAD` |
| Public repo: no personal paths, hosts, emails | HOLDS | tests use `t.TempDir()`; no host or user path in the new files |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | the pre-hook adopts without waiting, prints no decision and skips on a busy lock; no new deny path |
| Hook never waits on the lock | HOLDS | `lockLocal(true)` uses `TryLockExclusive` and returns `ErrLockBusy` at once; `TestPbAdDirectEditBusyLockSkips` |
| Edit only Touches files | NOT DONE | `internal/brain/vault.go` (22 lines) is outside Touches: `List`, `readFiles`, `vaultEntries`, `vaultShow` and `WithConfig` are defined there and cannot be reached from elsewhere. `braincmd/ke_write.go` is the allowed exception, see above. |
| Version check on every write path | NOT DONE | wired for `Write`/`WriteWith` (`vaultWrite`) only. Rename, Drop, Supersede, Revert and `EditConfigBy` have no vault-mode path; `hy_drop.go`, `revert.go`, `supersede_verb.go` and `index.go` are untouched. |
| The audit drift change | NOT DONE | `audit_check.go` is untouched. In vault mode `vaultCheck` (adopt.go) replaces the log.md drift finding, so row 7 holds, but the change was not made in the named file. |
| Fail on base for the new tests | NOT DONE for brain and braincmd | see Fail on base; the three hook tests were run against the base behaviour and fail |
| No existing test edited; no sleep or timing assertion | HOLDS | only new `pbad_*_test.go` files; `git diff --name-status origin/main -- internal/brain internal/braincmd internal/hook` lists every test file as `A`; `git grep -E "time\.Sleep\|Since\("` over them prints nothing |
| Nothing pushed, no claude or curl, no background server | HOLDS | none was run |

### Base check

`git ls-tree origin/main internal/brain/adopt.go` prints nothing: adopt.go does not exist on base. The named
lines on `origin/main`, printed with `git show origin/main:<file> | sed -n '<n>p'`:

- `internal/hook/hook.go:474`: `in.HookEventName = "PreToolUse"`
- `internal/hook/hook.go:497`: `if in.HookEventName == "PreToolUse" && decision.Behavior != "deny" {`
  (line 498, `history(o, *in)`, follows it; nothing there adopts)
- `hooks/hooks.json:47`: `"PreToolUse": [`
- `internal/brain/store.go:152`: `f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)` (in
  `atomicWrite`); `:170` is a blank line; `:196`: `c, cerr := root.OpenFile(lockName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)`
  (in `openLock`). The brief's `locked`, `openLock` and `checkLock` are at `:175`, `:193` and `:219` on this
  base, so those three numbers in the brief are off. `locked` takes `lockName` in a level's root and waits on
  `fslock.LockExclusive`; it is the old layout's per-level lock, not a per-vault machine-folder lock.

### Fail on base

CLOSED BY P-brain-adopt-fix1: accepted 17:22 UTC for the brain and braincmd tests. I tried to copy the new test files into a worktree at
`origin/main` and run them there; the copy into a path outside the repo was denied in this session, so I did
not run it. The new tests call symbols that exist only in this lane (`Adopt`, `AdoptPath`, `Hit.Conflict`,
`LockLocal`), so they should not compile on base, but that is a reading, not a run.

For the hook rows I ran a mismatch instead: with the `brainAdopt(o, *in)` call removed from
`internal/hook/hook.go` (the base behaviour), the three direct-edit tests fail, and I restored the file:

```
--- FAIL: TestPbAdDirectEditAdoptsBeforeAndRecordsAfter
    pbad_direct_edit_test.go:140: the person's bytes are not under .grid/versions/; log ""
--- FAIL: TestPbAdDirectEditAdoptionFailureIsLoggedOnly
    pbad_direct_edit_test.go:187: the failed adoption is not in the log: ""
--- FAIL: TestPbAdDirectEditBusyLockSkips
    pbad_direct_edit_test.go:251: the skip is not in the log: ""
```

## Limits

- "Both edits cannot hold" is judged per front-matter key or per body section (`##` or `#`). A note with no
  `##` headings is one section, and no row tests two edits in one section on different lines.
- NOT VERIFIED: whether Claude Code's stale-read refusal covers an Obsidian save in the gap after Grid saves
  the person's version; whether Obsidian File recovery keeps overwritten text. Neither is claimed.
- NOT VERIFIED on a live screen: nothing here ran in Claude Code, Codex or Obsidian. Every row is a Go test
  against a temp vault; the hook rows drive `hook.Run` with event JSON, not a live session.
- Seam: the sync server's version-checked write is not built. The local lock is the path; P-brain-sync-write
  carries the client side.
- Owner question: `source` names the first Grid writer, and no `source: <owner>` line is added on adoption.
  Should adoption add one?

## Open items

- `internal/brain/vault.go` is outside the Touches list. It changed by 22 lines: `s.adoptForRead()` in
  `readFiles`, `s.decorate` in `List`, `ix.apply` in `vaultEntries`, the `renamedTo` fallback in `vaultShow`,
  and the MachineDir lines in `WithConfig`. Each function is defined in vault.go, so the hooks into them
  cannot sit anywhere else.
- Version check is wired on `Write` / `WriteWith` only (`vaultWrite`). Rename, Drop, Supersede, Revert and
  `EditConfigBy` have no vault-mode path yet: `audit_check.go` (in vault mode `vaultCheck` replaces the
  log.md drift finding), `hy_drop.go`, `revert.go`, `supersede_verb.go` and `index.go` are untouched.
  The brief's "version check on every write path" is therefore met only for the write path.
- Hook caveats: adoption does not run when the event is over the input limit or a guard denies, and a
  PermissionRequest event does not adopt. Where `fslock` is unsupported, adoption logs an error and skips.
- Visible `Proposals/` with accept and reject records is P-brain-layout, not this lane.
