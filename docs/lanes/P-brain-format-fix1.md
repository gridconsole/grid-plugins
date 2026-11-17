# P-brain-format-fix1

Fix round for P-brain-format, ejected from landing batch pint233 on `TestCaPnRealPluginResave`
(`internal/slices/brain/ca_page_note_realplugin_test.go`, app repo). Base: lane-P-brain-format `6b895ab1`, with
`origin/main` merged (`git fetch origin && git merge origin/main`; a second merge before the last run said "Already up to date").

## Cause (the failure, quoted)

The test needs the app repo and a plugin built from this lane. Built with `make build` here, app cloned from `ref/app.git`, run with
`make realplugin GRID_PLUGIN_BIN=<lane dist/grid> RUN=TestCaPnRealPluginResave PKGS=./internal/slices/brain`:

```
realplugin: pass 0 fail 1 skip 0
fail .../internal/slices/brain TestCaPnRealPluginResave
    ca_page_note_realplugin_test.go:202: a re-save with a bare origin: 200 map[created:false ... state:applied], want 400 the note is not a valid brain note
    ca_page_note_realplugin_test.go:205: the refused re-save changed the note on disk: "---\nname: cafe-plan ... status: accepted\norigin:\n---\n# Café plan ..."
    ca_page_note_realplugin_test.go:211: write knowledge/conventions/cafe-plan.md on "86dd9977ea2912b6": 409 conflict
```

The test pins that a note with a bare `origin:` is refused (400). P-brain-format changed that on purpose: a filed (accepted) note
carries no origin (4.5), so an empty `origin:` decodes to the zero origin, which `Note.Validate` now accepts (`internal/brain/note.go:155-166`,
lane doc "A filed note with no origin reads as knowledge"). An empty `origin:` is also what Obsidian Properties leaves when a person
adds the property, so refusing it would break the lane's aim (a person's note is never refused for lacking Grid fields). So the
lane is right and the test pin is stale. The lane was not changed. The test is not weakened: the refusal case it guards is kept with a
bytes that are still invalid (`origin` kind `bogus` still 400), and the bare-origin bytes are asserted to be accepted.

## What changed

- Plugin (this repo): new `internal/brain/pb_format_fix1_test.go`: `TestPbFmBareOriginIsAPersonNote`, `TestPbFmBadOriginStillRefused`. No production code.
- App repo (not this repo, see "App change"): `internal/slices/brain/ca_page_note_realplugin_test.go`: the re-save step now sends `kind: bogus` and expects 400 with the disk unchanged, then saves the bare-origin bytes and expects 200 `applied`; the final version count is 5 (add plus four accepted re-saves) instead of 4. The fixture `ml-page-note.md` and every jstest are unchanged: the page bytes are the same.

## App change: needs the app landing

The app test is in the app repo and the clone here is the plugin, so the app edit cannot ride this bundle. It is committed in an app
clone as `7bb3ddd2` on `ref/app.git` main and bundled to `/srv/gridwork/tmp/hand/P-brain-format-fix1-app.bundle`
(`git bundle create ... origin/main..HEAD`, one commit, one file, +9 -5). It is needed for the pint233 re-land: with the plugin lane
merged and the app test as it stands, the test fails. The edited app test expects the bare-origin save to be accepted, so it fails
against a plugin without P-brain-format: land the app commit in the same batch as this lane, not before it.

## Proof table

| # | Row | Result | Proof |
|---|---|---|---|
| 1 | `TestCaPnRealPluginResave` exit 0 (acceptance 1) | HOLDS (with the app commit) | App clone `.pbf-app` at `7bb3ddd2`, plugin = this lane's `make build`: `make realplugin ... RUN=TestCaPn PKGS=./internal/slices/brain` prints `realplugin: pass 3 fail 0 skip 0`, exit 0 (TestCaPnRealPluginAdd, AddDecisions, Resave). Before the app edit, the same run: `pass 0 fail 1` (quoted above). Plain `go test ./internal/slices/brain/` does not exist in the plugin repo. |
| 2 | Every acceptance row of /srv/gridwork/briefs/P-brain-format.md on the merged head | HOLDS | `go test -count=1 -v ./internal/brain/ -run TestPbFm` exit 0: row 1 `TestPbFmRenameKeepsWhatAPersonWrote`, `TestPbFmSupersedeChangesOnlyItsKey`; row 4 `TestPbFmFiledNoteIsFlat`; row 5 `TestPbFmRenameLeavesANoteWithoutFrontMatter`; rows 2 and 3 are asserted inside `TestPbFmRenameKeepsWhatAPersonWrote` (F5 links at `pb_format_test.go:73-88`, F2 lists at :21 and :57). Standing row: `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./contract/formats/ ./internal/cli/` exit 0 (4 ok). |
| 3 | Importers of internal/brain (go list: people, projectenv, job, job/jobcmd, hook, routine, task, workflow, cli, braincmd), plus contract and feed | HOLDS | `go test -count=1 ./internal/people/ ./internal/projectenv/ ./internal/job/... ./internal/hook/ ./internal/routine/ ./internal/task/ ./internal/workflow/ ./internal/contract/ ./internal/feed/` exit 0. |
| 4 | Walk tests: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0 (`ok internal/cli 0.041s`). |
| 5 | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0. |
| 6 | `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 7 | `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64 vet and test compile). `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin ...` exit 0. |
| 8 | New tests fail on base, pass on the lane | HOLDS for one, one is a guard | On `origin/main` with the new file: `TestPbFmBareOriginIsAPersonNote` FAIL `a filed note with a bare origin is refused: metadata must contain single-line values and an origin id`. `TestPbFmBadOriginStillRefused` passes on base and on the lane by design: it pins that the refusal the app test now relies on still exists, so it cannot fail on base. On the lane both PASS. |
| 9 | Existing test edited | HOLDS | none in this repo. The one edited test is the app's `TestCaPnRealPluginResave`, an assertion pinning a bare `origin:` refused, which the lane changed by design (above); nothing deleted, skipped or weakened. |
| 10 | Merge origin/main before the last run | HOLDS | merged at the start; `git fetch origin && git merge origin/main` before the last run: "Already up to date". |
| 11 | App guest walk tables (`make realplugin ... TestLcHostRealPlugin\|TestKdLeakRealPlugin\|TestKdProjRealPlugin`) | not run, not asked | this is the plugin repo; the runbook line is for the app repo. The app test that did fail was run (row 1). |
| 12 | No guard, gate, cap or limit added; nothing removed; no push | HOLDS | only a new test file here. |

## Notes

- An empty `origin:` on an accepted note is now accepted, as is a note with no origin: this is the owner question already carried in the lane doc ("a hand-placed note without one used to be refused"). If the owner wants a bare `origin:` refused again, that is a change to `Validate` and to the app test, not a bug of this fix.
- `.pbf-app/` in this clone is an untracked app checkout used only to run the app test; it is not committed and not in the bundle.

## NOT VERIFIED on a live screen

Nothing here touches a screen.
