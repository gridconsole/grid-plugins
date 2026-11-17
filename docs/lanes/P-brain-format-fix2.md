# P-brain-format-fix2

Fix round for P-brain-format-fix1, ejected from landing batch pint234 on `TestCaPnRealPluginResave`
(`internal/slices/brain/ca_page_note_realplugin_test.go`, **app repo**). Base lane-P-brain-format-fix1 `9d879c21`, `origin/main`
merged (`git fetch origin && git merge origin/main` again before the last run: "Already up to date").

## Cause

`go test ./internal/slices/brain/` does not exist in the plugin repo; the test lives in the app repo and runs against the lane's
plugin build. On the lane it fails (app clone from `ref/app.git` main, plugin built from this clone, `make realplugin ... RUN=TestCaPn PKGS=./internal/slices/brain`):

```
realplugin: pass 2 fail 1 skip 0
fail .../internal/slices/brain TestCaPnRealPluginResave: path:knowledge/conventions/cafe-plan.md]], want 200 applied created false
```

The test pins that a note with a bare `origin:` is refused (400, "not a valid brain note"). P-brain-format made a bare `origin:` read
as a filed note. fix1 kept that and edited the app test instead (app commit `7bb3ddd2`, bundle `P-brain-format-fix1-app.bundle`), but
that commit never reached `ref/app.git` main, and the landing runs app main's test against the lane's plugin. So fix1 could not land.

## Decision

The plugin side changes, not the app test. The test pin is not stale in a way this repo can fix, and the parent brief needs only an
*absent* origin accepted (rows 4 and 5: Grid writes none, a note without front matter is untouched). A filed note may leave the
`origin` key out; it may not leave it present and empty (`origin:`, `origin: null`, `origin: {}`), which is what it was refused for
before the lane. A present origin that names a kind and an id is read as before. The app test is unchanged and must not be edited
now; **do not land `P-brain-format-fix1-app.bundle`**: it expects 200 on a bare origin and fails against this lane.

## What changed

- `internal/brain/markdown.go`: `Parse` probes the front matter for an `origin` key; present but empty is an error ("origin must name a kind and an id, or the key must be left out"). Absent key: unchanged.
- New `internal/brain/pb_format_fix2_test.go`: `TestPbFmBareOriginRefused`, `TestPbFmNoOriginKeyReads`.
- `internal/brain/pb_format_fix1_test.go`: `TestPbFmBareOriginIsAPersonNote` (fix1's own test, never landed) reversed in place and renamed `TestPbFmBareOriginIsNotAPersonNote`, because the behaviour it pinned is reversed by design (above). `TestPbFmBadOriginStillRefused` is unchanged.

## Proof table

| # | Row | Result | Proof |
|---|---|---|---|
| 1 | `TestCaPnRealPluginResave` exit 0 (acceptance 1) | HOLDS | unedited app main clone `.pbf-app`, plugin = this lane's `make build`: `make -C .pbf-app realplugin GRID_PLUGIN_BIN=<lane dist/grid> RUN=TestCaPn PKGS=./internal/slices/brain` prints `realplugin: pass 3 fail 0 skip 0`, exit 0. Before the fix, same run: `pass 2 fail 1 skip 0` (quoted above). The plain `go test ./internal/slices/brain/` of the brief does not exist in the plugin repo. |
| 2 | Every acceptance row of /srv/gridwork/briefs/P-brain-format.md on the merged head | HOLDS | `go test -count=1 -v ./internal/brain/ -run TestPbFm` exit 0, 8 PASS: row 1 `TestPbFmRenameKeepsWhatAPersonWrote`, `TestPbFmSupersedeChangesOnlyItsKey`; row 4 `TestPbFmFiledNoteIsFlat`, `TestPbFmNoOriginKeyReads`; row 5 `TestPbFmRenameLeavesANoteWithoutFrontMatter`; rows 2 and 3 inside `TestPbFmRenameKeepsWhatAPersonWrote`. Standing: `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./contract/formats/ ./internal/cli/` exit 0 (4 ok). |
| 3 | Importers of internal/brain and the walk tests | HOLDS | `go test -count=1 ./internal/braincmd/ ./internal/hook/ ./internal/job/... ./internal/land/ ./internal/people/ ./internal/projectenv/ ./internal/routine/ ./internal/task/ ./internal/workflow/ ./internal/contract/ ./internal/feed/ ./internal/mdfile/` all ok (go list of importers: braincmd, cli, hook, job, job/jobcmd, job/jobtest, land, people, projectenv, routine, task, workflow). `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `./internal/job/lint/` ok. |
| 4 | `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 5 | `make cross` | HOLDS | darwin/arm64 and windows/amd64 vet and test compile, no failure line. |
| 6 | New tests fail on base, pass on the lane | HOLDS | on the fix1 head with the new file: `--- FAIL: TestPbFmBareOriginRefused ... pb_format_fix2_test.go:23: a filed note with a bare origin key is accepted`. `TestPbFmNoOriginKeyReads` passes on base and lane by design (pins that an absent key still reads). Both PASS on the lane. |
| 7 | Existing test edited | EDITED, by design | `TestPbFmBareOriginIsAPersonNote` (fix1's, in `pb_format_fix1_test.go`, not landed anywhere) now asserts the refusal and is renamed; nothing deleted, skipped or weakened. No test of main touched. |
| 8 | App guest walk tables and app brain real-plugin tests | HOLDS | `make -C .pbf-app realplugin ... RUN='TestLcHostRealPlugin\|TestKdLeakRealPlugin\|TestKdProjRealPlugin' PKGS=./cmd/grid-server`: `pass 20 fail 0 skip 0`. `RUN=RealPlugin PKGS=./internal/slices/brain`: `pass 9 fail 0 skip 0`. |
| 9 | Merge origin/main before the last run | HOLDS | merged at the start; again before the last run: "Already up to date". |
| 10 | Other walk tests (`make webui-test`, `TestFvArgvCollects`, grid-server and slices walks) | not run, not asked | plugin repo: those are app-repo rows. |
| 11 | No guard, gate, cap or limit; nothing removed; no push | HOLDS | a parse check on a note's own front matter, no agent is stopped by it; no push. |

## Open items

- `P-brain-format-fix1-app.bundle` must not land: it asserts 200 on a bare origin.
- Whether a bare `origin:` should be accepted (so a person's Obsidian Properties edit never fails) is an owner question; it needs an app lane that changes `TestCaPnRealPluginResave` in the same batch, then this check is removed.
- The `.pbf-app` checkout and `dist/` were removed (`git clean`) before the final runs; `git status` shows only the lane files.

## NOT VERIFIED on a live screen

Nothing here touches a screen.

DONE
