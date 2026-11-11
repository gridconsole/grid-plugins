# Lane C-ke-brain-rename: `grid brain rename`

Repo: plugin. Branch `lane-C-ke-brain-rename`. Base: `dc301fce757d6374de26d8d1b68de26575341b32` (the brief names
its ancestor `35af7d3c`; no brain rename exists on either: `git grep -hE '^(func|type|var) keRn' HEAD` was empty and
`rename` is not in internal/braincmd/command.go's verb list).

Commits on the base: `e6f0c9bf` (internal/brain), `099418ff` (internal/braincmd), then this report.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 Applied | HOLDS, with one difference in the backlinks key (below) | `go test -count=1 -v -run 'TestKeRnStore' ./internal/brain/` exit 0, `--- PASS: TestKeRnStore (3.74s)`, internal/brain/ke_rename_test.go:`TestKeRnStore` |
| 2 Refusals write nothing | HOLDS | `go test -count=1 -v -run 'TestKeRn' ./internal/brain/` exit 0, `--- PASS: TestKeRnRefuse`, 13 sub-tests (stale rev, missing note, `machines/x`, no stored version, same topic, other topic, `TOPIC`, same name, bad name, dropped, superseded, workflow, symlinked file), each with a before/after walk of the whole Grid home (Lstat-aware, lock file left out) |
| 3 Half way | HOLDS | same run, `--- PASS: TestKeRnHalfWay`: hook fails `knowledge/decisions/c.md`; c and its versions byte-identical, no `linked` line for c, `kept == ["decisions/c"]`, `linked == ["decisions/d"]`, `skipped == ["decisions/f"]`, error nil |
| 4 Race | HOLDS | `go test -count=1 -race -v -run 'TestKeRnRace' ./internal/brain/` exit 0, `--- PASS: TestKeRnRace (0.02s)`, no race report. One closed channel as start barrier, two roots, `RenameWith` against `DropWith` at one rev; the test accepts exactly the two outcomes of the row |
| 5 Command | HOLDS | `go test -count=1 -v -run 'TestKeRnCommand' ./internal/braincmd/` exit 0, `--- PASS: TestKeRnCommand` and 6 sub-tests: usage (8 bad forms, incl. `--by ana` and a three-part path) exit 2; guest and worker exit 3 with stderr `grid brain rename: caller role is not allowed to rename brain entries`; stale rev 6 with `{"error":"conflict","rev":"<16 hex>"}`; `machines/x` 7; missing note 8 `{"error":"no_such_note"}`; dropped 9 `{"error":"refused","reason":"note is dropped"...`; owner, member and empty role exit 0; `--json` result; Member `bo` gives `person:bo` on the `renamed` and `linked` lines, empty Member gives `session:` |
| 6a Only four files | HOLDS | `git diff main --stat` before this report: 4 files changed, 1085 insertions: internal/brain/ke_rename.go, internal/brain/ke_rename_test.go, internal/braincmd/ke_rename.go, internal/braincmd/ke_rename_test.go. This report (docs/lanes/) is added after, as the runbook asks |
| 6b `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`, no output after it) |
| 6c `make cross` | see the line added at the end | |
| 6d brain and braincmd tests | HOLDS | `go test -count=1 ./internal/brain/ ./internal/braincmd/` exit 0 (`ok ... brain 12.435s`, `ok ... braincmd 10.129s`) |
| New tests fail on base | HOLDS | on a worktree of `dc301fce` with the two test files copied in: `internal/brain/ke_rename_test.go:156:16: s.Rename undefined (type Store has no field or method Rename)`, `:191:10: undefined: RenameResult`, `:282:94: undefined: ErrRenameNotVersioned`, `:337:2: undefined: keRnWriteHook`, `:399:18: undefined: RenameWith`; `internal/braincmd/ke_rename_test.go:71:9: undefined: runRename`; both `FAIL ... [build failed]`. Worktree removed |
| No existing test edited or deleted | HOLDS | the diff holds only new files |
| Importers' tests | HOLDS | `go list` of packages that import internal/brain or internal/braincmd: contract/formats, internal/brain, internal/braincmd, internal/cli, internal/hook, internal/job, internal/job/jobcmd, internal/job/jobtest (no test files), internal/people, internal/routine, internal/task, internal/workflow. `go test -count=1` on all of them exit 0, all `ok` (cli 140s, workflow 56s, task 56s, hook 55s) |
| Walk tests, plugin | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` `ok ... 0.403s`; `go test -count=1 ./internal/job/lint/` `ok ... 0.505s`. App-only walk tests (webui, grid-server, slices) do not apply to this repo |
| Real plugin tests | N/A | no row names a *RealPlugin* test |
| No guard, gate, cap or hold on agents | HOLDS | the verb is a store operation; no hook, no limit; role check is the one `brain.RevertAllowed` that revert, drop and restore already use |
| Names | HOLDS | every new helper, type and var in test files starts `keRn`; tests `TestKeRn*`; shares nothing with `keWr` |
| Commits | HOLDS | one-sentence messages, no attribution lines |
| UI rows | N/A | none |

### The one difference from row 1: the backlinks key

The brief expects `backlinks.json` to hold the key `brain:<level>/b` and not `brain:<level>/a`. Input: the fixture of
row 1 (c links `brain:project/a#why`, f is dropped and links `brain:project/a`). Output as seen after the rename:

```
"brain:project/a":   [ from brain:project/f ]      (f is dropped, not rewritten, by the brief's own rule)
"brain:project/b#why": [ from brain:project/c ]    (the index keys on the whole target, anchor included)
"brain:project/d#old": [ from brain:project/b ]    (b's own supersedes link, not touched)
```

So the key for c's link is `brain:project/b#why`, and a key for the old name remains only because the dropped note f
still links it: RebuildIndex (internal/brain/index.go) lists a dropped note's links. I did not touch index.go (not in
Touches). The test asserts exactly this: `b#why` has the one backlink from c, no key `a#...` exists, the key `a` has
f as its only source. If the owner wants no `a` key at all, the index must skip dropped notes' links: a change to
index.go, outside this lane.

## What changed

- `internal/brain/ke_rename.go`: `RenameResult`, `ErrRenameNotVersioned`, `keRnWriteHook`, `RenameWith`, `Store.Rename`.
  Under `locked` it checks, writing nothing: settings topic (not versioned, at either level), `TOPIC`, file present,
  rev, newest version holds the exact bytes, parse and validate, accepted, no `superseded_by`, not dropped, not a
  workflow, new name valid, not `TOPIC`, not the old name, free in the topic and in every other topic. Then: version
  under the new name, new file, remove old file, one `renamed` line (reason `from <topic>/<name>[: <reason>]`). Then
  the relink of every other accepted note of the level whose link or `superseded_by` names the old id (anchor kept),
  each as a version, a file and a `linked` line; a dropped one goes to `skipped`; a failed one (hook, encode, version,
  write or log) is restored and goes to `kept`; the call returns a nil error. The old name's versions folder stays.
- `internal/braincmd/ke_rename.go`: `runRename`, `renameUsage`, in the order of `hyDrRun`. Plain output: one
  `renamed a -> b vN rev R` line, then a `linked`, `kept` or `skipped` line per note; `--json` prints the result.
  Exit 7 prints `{"error":"not_versioned","reason":...}`.
- Two test files, new names only.

Choices the brief left open:
- The hook gets the path `knowledge/<topic>/<name>.md` and runs before the note's version, file and log line.
- A failed relink also removes the version copy it had stored, so the newest copy of a kept note still holds its file
  (otherwise a later rename of that note would be refused as not versioned).
- If the `renamed` line cannot be written, the old file is put back and the new file and its version copy are removed
  (error returned, nothing applied). If removing the old file fails, the new file and copy are removed. These two
  paths have no test: they need a failing disk.
- If `scan` fails after the rename is applied, `RenameWith` returns the result with an error; the command prints the
  result, then the error, and exits 1, as drop does for a failed index rebuild.

## Open items

- `scan` drops invalid, secret-shaped and ambiguous-name notes, so such a note that links the old name is neither
  relinked nor named in `kept` or `skipped`. The brief names `scan` as the source; I kept it.
- A note's link to itself, proposals in `brain-proposals/`, and notes of the other level are not rewritten (same level
  only, accepted notes only, "other" notes, as the brief says).
- Not wired into `Run` or the verb list: WAIT-C-ke-mount.
- `RenameWith` stamps the lines with `time.Now()` as `DropWith` does, so `Store.Now` does not reach it.

## NOT VERIFIED on a live screen

Nothing here has a screen. The app's knowledge rename stays at 501 until WAIT-C-ke-mount wires the verb.
