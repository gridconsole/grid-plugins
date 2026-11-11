# C-small-plugin-account-path-fold-tilde

The account registry matches a folder whatever its letter case on macOS and Windows, and expands `~\` on Windows. Base: 5925b841.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Tests fail on base | HOLDS | Commit 0d0e139b has the `Env.GOOS` field and the tests, with compares and `expand` unchanged. `go test ./internal/account/ -count=1 -run Fold` failed: `signedIn on darwin: want true`, `signedIn on windows: want true`; `MakeFolder did not run: stat .../home/.claude-bob` (darwin, windows); `windows expand("~\\.claude-ann") = "~\\.claude-ann", want "/home/ada/.claude-ann"` (also `~\a\b`, `~\a/b`); `darwin entries: map[Claude-ann:{...} claude-ann:{...}]` (windows too). Full output: `tmp/hand/C-small-plugin-account-path-fold-tilde-base-fail.txt`. |
| 2. describe folds | HOLDS | `TestFoldDescribeMatchesTheFolderWhateverItsCase` (path_fold_tilde_test.go): darwin and windows SignedIn true, linux false, `Account.Dir` keeps the record's spelling. `go test ./internal/account/ -count=1` exit 0. |
| 3. registerNew folds | HOLDS | `TestFoldRegisterNewGivesANewEmailItsOwnFolderWhenTheCaseOnlyDiffers`: darwin and windows give a new `.claude-bob` folder and two different Dirs; linux records `~/.claude-ann`; registry bytes unchanged on a second call. |
| 4. expand | HOLDS | `TestFoldExpandTakesTildeBackslashOnWindowsOnly`: windows `~\.claude-ann`, `~\a\b`, `~\a/b`, `~`, `~/x`; linux and darwin keep `~\x` as `filepath.Clean` of the input; `tilde` writes `~/` for every GOOS. |
| 5. Existing tests unchanged | HOLDS | `git diff 5925b841 --stat` shows no existing test file touched; `go test ./internal/account/ -count=1` exit 0. |
| 6. Windows() | HOLDS | `TestFoldWindowsReadsTheScannedSpellingUnderTheAccount`: darwin and windows have the account entry, no `Claude-ann` entry, and the noted status line under the account (Source statusline). Linux has both entries; the numbers sit on `Claude-ann` and the account is "none". Code: registry.go `Windows` (`spelled`, `covered`, `dirs`). windows.go is unchanged. |
| 7. grep | HOLDS | `git grep -n -i "case-sensitive" -- internal/account/registry.go` prints registry.go:77. `git grep -n "runtime.GOOS" -- internal/account/account.go` prints account.go:63 (the `goos` method) and account.go:339 (MakeFolder, unchanged). |
| 8. Checks | HOLDS | `go test ./internal/account/ -count=1` ok. `GOOS=windows go vet ./internal/account/` exit 0. `make lint` exit 0 (after gofmt of the new test file). |
| Importers' tests | HOLDS | `go list` gives importers of internal/account: internal/cli, then cmd/grid and lib through cli. `go test -count=1 ./internal/account/ ./internal/cli/ ./cmd/grid/ ./lib/` is ok for account, cli and lib; cmd/grid has no test files. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...` exit 0. |
| New tests in a new file | HOLDS | internal/account/path_fold_tilde_test.go only. |
| No guard, gate, cap or hook | HOLDS | None added. No refusal and nothing removed. |
| Commits | HOLDS | One-sentence messages, no attribution lines. |

## What changed

- account.go: `Env.GOOS` (empty means the host's OS) and the small method `goos()`. Nothing else in that file.
- registry.go:
  - `expand` also takes `~\` plus rest on Windows. The rest is split on `\` and `/` and joined under Home.
  - New helper `pathKey` (the cleaned path, lower-cased on darwin and windows), with the case-sensitive APFS trade-off in its doc comment.
  - `registerNew`'s used-compare, the `describe` `byDir` key and lookup, and the `Windows` `covered` keys all use `pathKey`.
  - In `Windows`, an account's `dirs` gets the scanned spelling of its folder after `a.Dir` when the spelling differs, so numbers noted under that spelling are still read.
- New internal/account/path_fold_tilde_test.go.
- Left as they are, per the brief: account.go MakeFolder's `runtime.GOOS` (the real host's symlink ability), SignIn's `r.dir == dir`, folderList's `seen[dir]`, windows.go's snapshot hash (a hash of the path as spelled), and the `.claude-` name prefix. A change to the INDEX.md note on these was not made, because the brief names no INDEX.md path in this repo; the orchestrator can carry it.

## NOT VERIFIED on a live screen

- A real macOS or Windows host. All OS branches are driven through `Env.GOOS` on a Linux host. Path joins use the host's `filepath`, so a Windows drive-letter path is not exercised.
