# C-small-version-cache-atomic-rename

The version check cache write uses `atomicfile.Rename`, so on Windows a rename over a cache file another Grid process holds open is retried instead of failing at once.

Base sha: 5925b841098c704e445a7bd6da9d342a05983f68 (`git merge-base HEAD origin/main`; the brief names plugin main 55c46cbd, check.go line numbers are the same).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. grep on base: `git grep -n "os.Rename(" -- internal/version/check.go` | HOLDS | Base prints `internal/version/check.go:168:		_ = os.Rename(f.Name(), path)`. After the change it prints nothing. |
| 1. grep after: `git grep -n "atomicfile.Rename" -- internal/version/check.go` | HOLDS | Base printed nothing. After the change: `internal/version/check.go:170:` (one line; the import is the other `atomicfile` mention and does not match the `.Rename` pattern). |
| 1. No behaviour test can tell main from the change on Linux | HOLDS | `internal/atomicfile/rename_unix.go:9` is `os.Rename`; no seam was added. The grep above is the fails-on-main proof. |
| 2. Second Check replaces the cache, no `.version-*` left | HOLDS | `TestCacheReplacedOnSecondCheck` in `internal/version/cache_rename_test.go`; `go test ./internal/version/ -count=1` exit 0. Checks one cache file after each call, the first reads 1.1.0, after +25 h and a server serving 1.2.0 the file reads 1.2.0 with `Checked` equal to the new time. |
| 3. Cache path a non-empty folder | HOLDS | `TestCacheWriteOverFolderKeepsResult`: Check returns "up to date" with release 1.0.0, the folder and `keep.txt` are unchanged, the cache folder holds only that folder and no `.version-*` file. Exit 0. |
| 4. `go test ./internal/version/ ./internal/atomicfile/ -count=1` | HOLDS | exit 0, both `ok`. |
| 4. `GOOS=windows go vet ./internal/version/` | HOLDS | exit 0, no output. |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| Importers of internal/version tested | HOLDS | Packages that import it, direct or by dependency (from `go list -test=false -f '{{.ImportPath}}\|{{join .Deps " "}}' ./...`): cmd/grid, internal/cli, internal/doctor, internal/migrate, internal/packaging (test import), internal/packaging/cmd, lib. `go test` on all of them with version and atomicfile, `-count=1`: exit 0; cmd/grid has no test files. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base, pass on lane | NOT DONE (as the brief says) | Both tests exercise behaviour that is identical on Linux before and after; the brief says the grep is the fails-on-main proof and no seam is to be added. They pass on the lane, and they pin rows 2 and 3 against later change. |
| No existing test edited or deleted | HOLDS | `git diff --stat origin/main..HEAD` touches only `check.go`, the new test file and this record. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The diff is one call and one import. |
| Small commits, one-sentence messages, no attribution lines | HOLDS | 6fc7b0de for code and test; one more for this record. |
| No personal paths, hosts or emails in the diff | HOLDS | The diff holds none. |
| saveCache still swallows errors | HOLDS | `check.go:170` keeps `_ =`. |

## What changed

- `internal/version/check.go`: the import of `internal/atomicfile` and `_ = atomicfile.Rename(f.Name(), path)` at :170 in place of `os.Rename`. `os` is still used elsewhere in the file.
- `internal/version/cache_rename_test.go` (new): the two tests for rows 2 and 3.
- This record.

## NOT VERIFIED on a live screen

- The Windows retry itself: only Windows builds and vets here; the retry loop is tested inside `internal/atomicfile`. Two Grid processes checking at once on a real Windows machine were not run.
- Windows behaviour of rename over a non-empty folder (row 3's accepted retry up to 750 ms) was not run; Linux gives the same result as before.

DONE
