# G-tests-plugin-files: unit tests for mdfile and safefile

Repo plugin, branch lane-G-tests-plugin-files. New files only: `internal/mdfile/markdown_test.go`, `internal/safefile/file_test.go`. No change to the code under test. No testdata was needed (inputs are inline strings and `t.TempDir` files).

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. `Split`: front matter and body, no front matter, unterminated block, CRLF, empty file | HOLDS | `go test -count=1 ./internal/mdfile/...` exit 0. `TestSplit` (markdown_test.go:9) has 10 cases: the five asked for plus empty front matter, closing line without newline, a later `---` kept in the body, an opening line only, and `--- x` not a fence. CRLF case checks front is normalised to `\n` and the body keeps `\r\n`. |
| 1. `Load`: strict refuses an unknown field, non-strict keeps it out | HOLDS | `TestLoadStrictRefusesUnknownField`, `TestLoadNonStrictDropsUnknownField` (known fields kept, extra left out), `TestLoadKnownFieldsStrict`. Exit 0. Also `TestLoadRefusesDuplicateKey` and `TestLoadSyntaxError`. |
| 1. `plainNodes` refuses anchor, alias, tag (rule quoted) | HOLDS | Rule, markdown.go:33: `n.Kind == yaml.AliasNode \|\| n.Anchor != "" \|\| n.Style&yaml.TaggedStyle != 0` gives `anchors, aliases and tags are not supported (line N)`. `TestLoadRefusesAnchorAliasAndTag` covers anchor on a scalar, anchor on a map, alias, `!!str`, a custom tag and a nested tag, and checks the message and a line number. `TestLoadAllowsPlainNesting` shows a plain document still loads. |
| 2. safefile: plain file reads | HOLDS | `TestReadPlainFile` (`sub/a.md`, `./sub/a.md`), `TestReadMissingFile` (NoSymlinks lets a missing path through, Read returns not-exist). `go test ./internal/safefile/...` exit 0. |
| 2. symlink to a file inside, symlink to a file outside, symlinked folder in the path, `..` path | HOLDS | Symlink rows refused with `brain paths cannot be symbolic links` (file.go:32) by both `NoSymlinks` and `Read`: `TestReadRefusesSymlinkToFileInside`, `...ToFileOutside`, `...SymlinkedFolderInPath`, `...SymlinkedFolderOutside`, `...DanglingSymlink`. `TestReadRefusesPathsOutsideLevel` covers `../x`, `sub/../../x`, `..`, empty, an absolute path: refused with `brain path must stay within its level` (file.go:19). Extra rows: directory refused, exactly 1 MiB reads, 1 MiB + 1 refused (file.go:46). |
| 2. symlink rows skipped on Windows | HOLDS | `symlink()` helper in file_test.go calls `t.Skip` when `runtime.GOOS == "windows"`: creating a symlink there needs elevated rights, and the rows only prove how Lstat reports links. Not run on Windows here (vet only). |
| 3. `GOOS=windows go vet ./internal/safefile/... ./internal/mdfile/...` | HOLDS | exit 0, no output. |
| 4. `go test ./internal/mdfile/... ./internal/safefile/...` | HOLDS | exit 0: `ok .../internal/mdfile`, `ok .../internal/safefile`. |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| Importers' tests | HOLDS | Importers by grep for the import path: brain, where, settings, projectmd, land, workflow, routine. `go test -count=1` on those plus the two packages: exit 0, all `ok` (brain 4.4s, where 8.6s, settings 1.3s, projectmd 0.02s, land 3.9s, workflow 3.6s, routine 0.9s). |
| macOS build | HOLDS | `GOOS=darwin go vet ./internal/safefile/... ./internal/mdfile/...` exit 0. |
| Windows build | HOLDS | Same Windows vet as row 3, exit 0. |
| New tests fail on base | NOT APPLICABLE | The code under test is unchanged and the brief forbids changing it, so these tests pass on base as well as on the lane. They add coverage; they do not fix a bug. I did not mutate the code to show failure. |
| No existing test edited or deleted | HOLDS | `git diff origin/main --stat` shows two added files and this report. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Only `_test.go` files and this report were added. The 1 MiB limit tested is existing code (file.go:46). |
| Public repo hygiene | HOLDS | Tests use `t.TempDir` only. No personal paths, hosts or emails. Commit message has no attribution lines. |

## Findings in the code under test

- No bug found. Observed, not changed: `mdfile.Load` returns nil for a YAML input with a second `---` document (`WithSingleDocument` does not refuse it). I tried a test for this, it failed, and I removed it because it asserted a refusal the code does not give. `Split` ends the front matter at the first `---`, so a second document cannot reach `Load` through `Split`. A caller that passes raw bytes to `Load` would silently drop the second document. This is not a row of the brief and I could not read the yaml library source in this sandbox to see whether it is intended.

## What changed

- `internal/mdfile/markdown_test.go`: Split table, Load strict and non-strict, duplicate key, syntax error, anchor/alias/tag refusals.
- `internal/safefile/file_test.go`: plain read, missing file, five symlink rows, escaping paths, directory, size limit.

## NOT VERIFIED on a live screen

Nothing here touches a UI. The Windows symlink skip was checked by vet only; the tests were not run on a Windows machine.
