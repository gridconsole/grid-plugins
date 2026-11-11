# C-pcov2b-signal-refusals: proof table

Head: see the report. One new file: `internal/signal/refusals_test.go`. No non-test file changed.

| Row | Result | Proof |
|---|---|---|
| 1 coverage before/after | HOLDS | Before (untouched clone, Linux): `go test -count=1 -coverprofile=.cov/signal.out ./internal/signal/ && go tool cover -func=.cov/signal.out \| tail -1` -> `total: (statements) 83.3%`, exit 0. After: same command -> `total: (statements) 91.7%`, exit 0. 91.7 > 83.3. |
| 2 ValidateMove and Move refusals | HOLDS | `TestValidateMoveRefusals` (refusals_test.go:38) and `TestMoveRefusals` (:48): empty `--to`, bad project name (from and to), two bad ids as contrast; directory source (error, not a RefusedError), dismissed note, from == to, id already in target (both files unchanged). Run in `go test -count=1 ./internal/signal/`, exit 0. |
| 3 relocateStatus direct calls | HOLDS | `TestRelocateStatusRefusals` (:106) has all nine cases with exact bytes, plus a pointer check that old == status returns the same slice. Every case behaved as the brief says; nothing differed. exit 0. |
| 4 Add and read refusals | HOLDS | `TestValidateRefusals` (:156), `TestAddAndReadRefusals` (:186): invalid UTF-8 source, 129-byte source, 1025-byte title, MaxBytes+1 and non-UTF-8 body; Add on `../x`; `s.path("bad")`; no front matter; `id: [` header; id conflict with another source (file unchanged); directory at the id path (no file written). The `ValidateMove("S-abc", ...)` case is in `TestValidateMoveRefusals`. exit 0. |
| 5 Apply refusals | HOLDS | `TestApplyStoreRefusals` (:275): project "nowhere" (error holds `project "nowhere" is not registered`); note replaced by a directory (not a RefusedError); act turn with `title: Ti<tab>tle` and `title: "Ti\ttle"` -> "title must be one line", bytes unchanged, no task under projects/example/tasks. exit 0. |
| 6 test, vet, lint | HOLDS | `go test -count=1 ./internal/signal/` exit 0. `GOOS=windows go vet ./internal/signal/` exit 0. `GOOS=darwin go vet ./internal/signal/` exit 0. `make lint` (runs `go vet ./...`) exit 0. No row skips on Windows. |
| Importers of the changed package | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...` filtered for internal/signal gives: `cmd/grid`, `internal/cli`. `go test -count=1 ./internal/signal/ ./internal/cli/ ./cmd/grid/` -> ok, ok, no test files; exit 0. |
| Tests fail on base | NOT APPLICABLE | The lane is tests only, for behaviour the code already has; the tests pass on base code by design. Proof they exercise the blocks: coverage rose 83.3% -> 91.7%, and every block in the brief's list now has count > 0 (checked against the coverprofile; the zero-count blocks left are the brief's "not picked" ones plus move.go:255-256, signal.go:159-160 and :179-180). |
| No existing test edited or deleted | HOLDS | `git status` shows one new file; no test deleted. |
| No guard, gate, cap, hold | HOLDS | Tests only. |
| Commit | HOLDS | One commit, one sentence, no attribution lines. |
| No personal paths, hosts, emails | HOLDS | Test file uses temp dirs and fixed names only. |

## What changed

New `internal/signal/refusals_test.go`: six tests (about 40 cases) for the refusals on ValidateMove, Move, relocateStatus, Validate, Add, read and Apply. Helpers reused: `fixture`, `moveFixture`, `validNoteID`, `actNote`, `readBytes`, `wantRefused`.

## Outcome notes

- Every expected result in the brief matched what the code does; no assertion was changed to follow the code.
- The `id: [` case asserts a YAML error by exclusion (not one of the three known header refusals), not by the library's exact text.
- The coverage output went to `.cov/` inside the clone (mktemp and mkdir under /srv/gridwork/tmp were refused). It is git-ignored, not committed.

## NOT VERIFIED on a live screen

Nothing here is UI. Windows and macOS were only vetted (`go vet`), not run: compiled, needs Windows run `GOOS=windows go test -c ./internal/signal/` on a Windows host, and the same on macOS.
