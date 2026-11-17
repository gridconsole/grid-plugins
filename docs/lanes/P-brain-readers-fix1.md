# P-brain-readers-fix1

Finishes P-brain-readers (review STUCK 06:31: no verdict, rows not shown). Branch `lane-P-brain-readers-fix1`, from `lane-P-brain-readers` (ad42cf00), merged with origin/main (f4a471f3, clean). V-number V13.

## What changed

- The tests the last run wrote and never committed are in: workflow, settings, guard, land, orchestrator and braincmd (`pb_rd_*_test.go`), plus job, routine, people and projectenv from the earlier run. Each drives the reader through the Grid config's `brain` key and checks `grid/...` with the key set and `knowledge/...` with the key absent.
- Row 1: `internal/braincmd/pb_rd_walk_test.go` parses the 2.2 table of `docs/research/brain-obsidian.md` and walks every file outside `internal/brain`.
- Row 2: `internal/settings/pb_rd_readers_test.go` checks the permission glob covers `<Root>/**`.
- The five `brainroot.go` files (land, people, projectenv, routine, settings) are gone. Their code sits in the Touches files: `land/land.go`, `people/store.go`, `projectenv/projectenv.go`, `routine/store.go`, `settings/claude.go`.
- People and routines now read the config's brain folder in production, not only through a `BrainDir` field a test sets.

## Rows

| Row | Result | Proof |
|---|---|---|
| Row 1 (V13): one test walks the 2.2 table | HOLDS | `go test -count=1 -run TestPbRdReaderTableWalk ./internal/braincmd/` exit 0; per-package `TestPbRd*` pass on both layouts |
| Row 2 (V13): settings glob covers `<Root>/**` | HOLDS | `TestPbRdSettingsGlobCoversRoot` (`Read(~/Documents/Vault/**)`, `Edit(...)`, `//abs/**` outside home, nil with key absent) |
| Item 1: the uncommitted tests are committed | HOLDS | 12 new test files in `git diff --stat origin/main..HEAD` |
| Item 4: brainroot.go files folded in | HOLDS | `git ls-files '*brainroot.go'` is empty; code in the Touches files above |
| Standing go test line | HOLDS | exit 0 on the merged head; `go test -count=1 ./...` was green except the root package, fixed below, then `-run TestHh .` ok |
| `make lint` | HOLDS | exit 0 |
| `make cross` | HOLDS | exit 0 (darwin/arm64, windows/amd64) |
| Merge of origin/main | HOLDS | merge 46d71e11, no conflicts; `git merge-base HEAD origin/main` = origin/main |
| Tests fail on base | NOT DONE | see open items |
| `make webui-test`, guest walk tables | not run, not asked | not part of the plugin repo |
| RealPlugin tests | not applicable | |

## Outside Touches

- `agent-templates/grid-library.md` was edited. `TestHhHelpersShipAsPluginAgents` requires it to match `agents/library.md`, which is in Touches. It is a one-sentence change.
- No shared helper. The five files were folded into Touches files instead, because `internal/brain` is outside Touches and one cross-package helper would add imports between packages that have none.

## Open items

- The fail-on-base run is not done. I copied the new tests into a clean worktree of origin/main, but the sandbox refused `go test` there, so there are no failing lines to show. The settings, workflow and routine tests use new symbols, so they cannot compile on base.
- Reads prefer `grid/`; writes still go to `knowledge/` (writers are P-brain-format's).
- The people and routine resolver sits in each store because `cli/routine.go` and `cli/person.go` are outside Touches.
- `braincmd/cb_author_origin.go` keeps `ValidName` for generated ids. The proposal and task ids at `command.go:567` and `:593` keep it pending P-brain-board-decide.
- The people vault read has no config-key test of its own; the routine one covers the same code.

## NOT VERIFIED

Nothing needs a live screen.
