# P-brain-adopt-fix1

The version check on every brain write path, the audit drift change, and the fail-on-base proof for P-brain-adopt (source: `docs/research/brain-obsidian.md` 5.3, 5.4, 6.1). Lane `lane-P-brain-adopt-fix1`, carrying lane-P-brain-adopt (ee37816a). Code head before this report: `a6236d84`. Merge of `origin/main` (a6c5cb02) before the last runs: "Already up to date".

## Proof table

| Row or rule | Result | Proof |
| --- | --- | --- |
| 1. Rename, Drop, Supersede, Revert and EditConfigBy each run the version check in vault mode; one test per verb in a new file | HOLDS | `go -C <clone> test -count=1 -run TestPbAf -v ./internal/brain/` exit 0. The tests are in `internal/brain/pbaf_verbs_test.go`: `TestPbAfEditConfigByRedoneOnTop` (other key: redone on top, no proposal; same key: proposal, entry keeps the person's bytes), `TestPbAfSupersedeRedoneOnTop` (the person's line and the link both stay), `TestPbAfRevertRedoneOnTop` (the person's section edit stays, the other section reverts), `TestPbAfDropWithPersonEditIsAProposal` (proposal, note untouched; then drop, bytes kept under `.grid/versions`, restore), `TestPbAfRenameCarriesPersonEdit` (person's edit moves with the file, old id resolves; new name taken: proposal). Code: `vaultEditConfig`, `vaultSupersede`, `vaultRevert`, `vaultDrop`, `vaultRename` in `adopt.go`, each reached from a `if s.Vault()` dispatch at the top of `EditConfigBy` (config.go), `Supersede` (supersede_verb.go), `RevertWith` (revert.go), `Drop` (hy_drop.go) and `Rename` (ke_rename.go). They share `vaultApply`, which reads under the lock, releases it, makes the new bytes, and writes through `vaultWriteOpt`, the version check of `vaultWrite` (merge by front-matter key and body section; a proposal only when both cannot hold). |
| 2. Audit drift change in audit_check.go | HOLDS | `vaultCheck` MOVED to `audit_check.go` (with `CheckGridIntegrity` and `CheckSecretShaped`); `check.go` still calls `s.vaultCheck`. It adopts, reports the count, gives no finding for an adopted person edit, and now reports the three `.grid/` findings of doc 5.3: a record that does not parse or was edited (hash not the one in its name), a version whose bytes do not hash to its name, a record naming a missing version. `TestPbAfCheckAdoptsPersonEditWithoutDrift`: Adopted 1, no findings. `TestPbAfCheckReportsUnadoptedGridChange`: a changed version file and an edited record each still give a `grid_integrity` finding. Same `go test -run TestPbAf` run, exit 0. |
| 3. Fail on base: every new test fails on plugin main before lane-P-brain-adopt | HOLDS, as a build failure | Worktree at `origin/main` (a6c5cb02), new test files checked out from the lane. `go -C <base worktree> test -count=1 -run TestPbAf ./internal/brain/` exit 1: `[build failed]`, first errors `pbaf_audit_test.go:20:7: undefined: pbAdVault`, `undefined: pbAdEdit`, `undefined: pbAdBase`, `undefined: pbAdOf`, `undefined: pbAdRecords`, `undefined: versionRel`, `undefined: CheckGridIntegrity`, `undefined: recFile` (go stops at 10). No per-test names exist from that run: base has no vault code, so the file does not compile. On the lane the same command passes (7 tests, exit 0). I did not run a second base at lane-P-brain-adopt's tip: the tests use new fields (`Proposed`) and `vaultBeforeWrite`, so they would not compile there either. |
| 4. P-brain-adopt rows 1-11 still hold | HOLDS | `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/hook/` exit 0: `ok internal/brain 2.353s`, `ok internal/braincmd 2.235s`, `ok internal/hook 5.165s` (run again after the report was written). No existing test was edited. |
| 5. Report ends with DONE or NOT DONE | HOLDS | Last line of this file. |
| `make lint` exits 0 | HOLDS | `make -C <clone> lint` exit 0 (the target runs `go vet ./...` and fails on any file `gofmt -l .` lists). |
| `make cross` exits 0 | HOLDS | `make -C <clone> cross` exit 0: vet and test-compile for darwin/arm64 and windows/amd64. Not run on those systems. |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` prints 0. |
| Walk tests after merging origin/main | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| Importers of the changed package | HOLDS | `go list` importers of `internal/brain`: braincmd, cli, hook, job, job/jobcmd, job/jobtest, land, people, projectenv, routine, task, workflow. `go test -count=1` on all of them was run before the report and passed (not re-run after the report text was written; the report changed no code). |
| Owner direction V16 C: no lock, refusal, guard, gate, cap or limit that stops an agent on a person's edit | HOLDS | A person's edit makes the agent redo on top or write a proposal; the only refusals are the old ones (an unrecorded rev is the stale `ConflictError`, as `vaultWrite` already did; a rename to a taken name is a proposal). The lock is taken per read and per write and never held across them. |
| No sleep and no elapsed-time assertion in a test | HOLDS | By reading the two new files: the person's edit is placed by `vaultBeforeWrite` (nil in production), not by timing. |
| New tests in new files, helpers `pbAf`, nothing removed | HOLDS | `pbaf_verbs_test.go`, `pbaf_audit_test.go`. The only code removed from a file is the `vaultCheck` block and its two constants, moved to `audit_check.go` as the brief allows. |
| No pushing | HOLDS | No `git push` was run. |
| Touches | DEVIATION, declared | `internal/brain/ke_rename.go` is not in the Touches list. `Store.Rename` needed a vault dispatch and `RenameResult` a `Proposed` field; it is 6 lines, kept in its own place, and the verb has no other home. Nothing else outside the list changed (`git diff --stat ccc67063 HEAD`: adopt.go, audit_check.go, config.go, hy_drop.go, ke_rename.go, revert.go, supersede_verb.go and the two new tests). |

## What changed

- `adopt.go`: `vaultWrite` now calls `vaultWriteOpt` (the config path leaves out the secret-line check, as the old config write did); `vaultPropose` writes through the new `vaultPutProposal`; new `vaultBeforeWrite` test seam, `vaultRead`, `vaultApply`, `vaultDelete`, `vaultProposeChange`, `vaultRecords`, the five verb functions and the text helpers for a link and a front-matter key.
- `audit_check.go`: `vaultCheck`, its constants, `recordNameHolds`, and the version-hash and edited-record findings.
- `config.go`, `supersede_verb.go`, `revert.go`, `hy_drop.go`, `ke_rename.go`: the vault dispatch; `Proposed` on `SupersedeResult` (list), `RevertResult`, `DropResult` and `RenameResult`.
- A config entry of a vault is the note `grid/<topic>/<name>.md`; a vault Drop deletes the note and keeps its bytes under `.grid/versions`; a vault Revert's version `to` is the note's to-th recorded state, oldest first; a vault Supersede adds `- [[old]] - supersedes: reason` to the new note's Links section and `superseded_by` to the old note's front matter.

## Open items and NOT VERIFIED

- Row 3 per-test failing names: not available, the base run is a build failure (see the row).
- A vault Rename does not rewrite links in other notes: `Linked`, `Kept` and `Skipped` stay empty. The old id still resolves to the note it became.
- `EditConfigBy` returns only an error, so a config proposal is visible in `Proposals/` and not in the return value.
- Not tested: a same-section proposal for Revert and Supersede, the secret-shaped refusal on those two verbs, a vault Revert after a rename (history before a rename belongs to the old id), and a Drop whose note a person deleted first.
- Whether adoption sees an edit to a `grid/` file: the write checks the rev from disk, so the version check holds either way; adoption of that folder was not checked here.
- NOT VERIFIED on a live screen: how Obsidian shows `Proposals/`, the new Links line, `superseded_by`, or a renamed note; nothing was opened in Obsidian, on Windows or on macOS (cross compiles and vets only).

DONE
