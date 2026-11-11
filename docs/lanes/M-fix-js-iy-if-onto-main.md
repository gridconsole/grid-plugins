# M-fix-js-iy-if-onto-main

Carries M-js-iy-signal-codes-onto-il, C-iy-signal-refusal-codes, C-il-guest-refusal-code,
M-js-if-brain-partial-onto-main and C-if-brain-partial-code onto origin/main (fc3b76aa and later).

## What changed

- Merged origin/main into the lane. `TestIlGuestSiteList` then failed:

  ```
  il_guest_refusal_code_test.go:240: update.go: 1 errors.New(guestSettingsRefusal) site(s) not in the known-open list; return the guest sentinel
  il_guest_refusal_code_test.go:252: the walk finds 5 sites, want the 4 known open: map[budget.go:1 project_repo.go:1 settings.go:1 signal.go:1 update.go:1]
  --- FAIL: TestIlGuestSiteList (0.01s)
  ```

- Cause: commit be3d0074 on main ("Add grid update draft|edit|discard|skip|send", C-gr-plugin-update-act)
  added a new guest refusal at `internal/cli/update.go:87` as `errors.New(guestSettingsRefusal)`. That
  site prints no `code` with `--json`. Main did not change a refusal code or message on purpose; it just
  added a site the lane's walk has not seen. So the lane's code intent wins and the test stays as it is:
  `update.go:87` now returns the shared sentinel `ilGuestRefused` (same text, so the plain refusal and
  `guest_parity_test.go` are unchanged; `errorCode` maps it to `guest_refused`). One line, commit 3af0cb1a.
  The known-open list in the test is not edited.
- Merged origin/lane-M-js-if-brain-partial-onto-main. One conflict, contract/README.md, two hunks:
  - the paragraph after the four shapes: kept main's sentence on the `job` commands and the
    partial-success paragraph (`index_rebuild_failed`); dropped only the lane's old line
    "Some commands keep their own older JSON objects on failure; those carry no code.", which main's
    sentence ("Other commands that keep their own older JSON objects on failure carry no code.") replaces;
  - the table end: `guest_refused` and `index_rebuild_failed` rows, each once, in that order; the sentence
    now reads "The first ten constants live in `internal/cli/error_codes.go`, the settings codes in
    `internal/settings/claude_user.go` and `index_rebuild_failed` in `internal/braincmd/changes.go`".
    error_codes.go holds ten constants (grep of `Code... = "..."`, lines 20-39).
- No test deleted, skipped or loosened. origin/main did not move between the first and the last fetch.

## Proof table

| Row / rule | Result | Proof |
| --- | --- | --- |
| 1 failure after merging main | HOLDS | lines pasted above; `go test -count=1 -run TestIlGuestSiteList -v ./internal/cli/` exit 1 |
| 2 `go test -count=1 -run 'TestIl\|TestIy\|TestIf\|TestHfFsCodeTable' -v ./internal/...` | HOLDS, exit 0 | `--- PASS: TestIlGuestSiteList (0.04s)` (internal/cli), `--- PASS: TestHfFsCodeTable (0.30s)` (internal/cli, hf_failure_shapes_test.go); also PASS TestIfBeRevertPartialCode, TestIfBeSupersedePartialCode (braincmd), TestIySigCLI/Walk/Constants, TestIySchemaRows |
| 3 `go test -count=1 ./internal/cli/ ./internal/feed/ ./internal/brain/...` | HOLDS, exit 0 | `ok .../internal/cli 85.358s`, `ok .../internal/feed 4.972s`, `ok .../internal/brain 5.427s` |
| 4 ancestors | HOLDS, exit 0 each | `git merge-base --is-ancestor` of origin/lane-M-js-iy-signal-codes-onto-il, origin/lane-M-js-if-brain-partial-onto-main and origin/main (after the second `git fetch origin`) all exit 0 |
| 5 one README row per code | HOLDS | `grep -n` on the table: no_such_project 1 (l.77), no_such_task 1 (78), no_such_note 1 (79), note_state 1 (80), note_exists 1 (81), notes_full 1 (82), invalid_argument 1 (84), invalid_policy 1 (85), workflow_file_changed 1 (86), guest_refused 1 (93); plus index_rebuild_failed 1 (94) from changes.go. `TestHfFsCodeTable` passes |
| 6 `make lint` | HOLDS, exit 0 | `go vet ./...` printed nothing but its command line |
| 6 `make cross` (darwin/arm64, windows/amd64) | HOLDS, exit 0 | vet and test compile for both targets; last lines `cross: windows/amd64 test compile` and `ok .../grid-plugins` |
| Importers of the changed package | HOLDS | `go list` importers of internal/cli: cmd/grid (no test files) and lib; `go test -count=1 ./cmd/grid/ ./lib/` exit 0: `ok .../lib 241.544s` (cmd/grid has no test files) |
| Product code only where step 1 needs | HOLDS | one line, `internal/cli/update.go:87` |
| No guard, gate, cap or hook added | HOLDS | none added; the change reuses an existing refusal sentinel |
| No attribution lines, no personal paths | HOLDS | commits are one sentence |
| `*RealPlugin*` tests | not needed | no row names one |

## NOT VERIFIED on a live screen

Nothing here is UI. The whole-module test and the cross-repo set are for the orchestrator at landing.
