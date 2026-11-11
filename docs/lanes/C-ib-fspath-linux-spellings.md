# C-ib-fspath-linux-spellings

Base sha: ee8a96c126781462cbe0067771db40d01911cd76 (main in this clone; the brief names 91fadeff, and the fspath.go lines it quotes, :33-39, :53-69 and :62, are the same on this base). No test on the base creates two spellings of one name on a case-sensitive volume and calls OnDiskCase: fspath_test.go:38-40, projects_test.go:331-332 and guard/case_test.go (`skipUnlessFolding`) all skip first. So the brief applies.

This lane pins today's behaviour and changes no production file. The new tests therefore pass on the base too; their failing lines come from the mutations in rows 1 to 3, applied in the clone and reverted.

Fix round 1 (row 2): the NFC and NFD names in `TestIbFspathTwoUnicodeForms` were typed letters, not Go escapes. They are now `"caf\u00e9"` and `"cafe\u0301"` in the Go source, and the file holds no non-ASCII byte at all (the comment above the test names the escapes too).

## Proof table

| Row | Result |
|---|---|
| 1. TestIbFspathTwoCaseSpellings | HOLDS. `go test -count=1 -v -run TestIbFspath ./internal/fspath/ ./internal/projects/` exit 0: `--- PASS: TestIbFspathTwoCaseSpellings (0.00s)`. Mutation `os.SameFile(want, info)` to `!os.SameFile(want, info)` at fspath.go:62 (exit 1): `ib_linux_spellings_test.go:69: OnDiskCase("<root>/Alpha") = "<root>/aLPHA", want it unchanged` and the same for `<root>/Alpha/x`. In this run the directory listed aLPHA first, so Alpha was the one rewritten; with the other listing order aLPHA is rewritten instead. `Gamma` is not affected (its swapped name `gAMMA` is missing, fspath.go:37-38). Reverted with `git checkout -- internal/fspath/fspath.go` |
| 2. TestIbFspathTwoUnicodeForms | HOLDS. Rerun after fix round 1, exit 0: `--- PASS: TestIbFspathTwoUnicodeForms (0.00s)`. Same mutation at fspath.go:62 (exit 1): `ib_linux_spellings_test.go:87: OnDiskCase("<root>/caf\u00e9") = "<root>/cafe\u0301", want it byte for byte as given` and the same with `/x` appended (`%+q` prints the two names as escapes). The names are Go escapes in the file: line 82 is `nfc, nfd := "caf\u00e9", "cafe\u0301"` (NFC first, NFD second). `grep -nP '[^\x00-\x7F]' internal/fspath/ib_linux_spellings_test.go` prints nothing (exit 1, no match), so no typed letter is left, not even in a comment; `grep -n 'u00e9\|u0301'` matches lines 76 and 82 only. Reverted with `git checkout -- internal/fspath/fspath.go`; `git status --short` then showed only the test file |
| 3. TestIbFspathProjectsKeepTwoFolders (package projects) | HOLDS. `--- PASS: TestIbFspathProjectsKeepTwoFolders (0.00s)`. It asserts sameFolder(Alpha, aLPHA) false (both orders), sameFolder(Alpha, Alpha) true, within(Alpha, aLPHA/sub) false, within(aLPHA, Alpha/sub) false, within(Alpha, Alpha/sub) true. Mutation `sameFolder` body to `strings.EqualFold(a, b)` at projects.go:359 (exit 1): `ib_linux_spellings_test.go:66: sameFolder Alpha, aLPHA = true, want false` and `sameFolder aLPHA, Alpha = true, want false`. The SameFile mutation of row 1 also fails it: the two sameFolder lines plus `within Alpha, aLPHA/sub = true, want false` and `within aLPHA, Alpha/sub = true, want false`. Reverted with `git checkout -- internal/projects/projects.go`. Unchanged by fix round 1 (the projects file has no non-ASCII byte: `grep -nP '[^\x00-\x7F]'` prints nothing) |
| 4a. `go test -count=1 -v -run 'TestIbFspath' ./internal/fspath/ ./internal/projects/` | HOLDS, exit 0, rerun after fix round 1. Three PASS lines (`TestIbFspathTwoCaseSpellings`, `TestIbFspathTwoUnicodeForms`, `TestIbFspathProjectsKeepTwoFolders`), no SKIP line; `ok .../internal/fspath`, `ok .../internal/projects` |
| 4b. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output after it); `gofmt -l internal/fspath internal/projects` prints nothing (rerun after fix round 1) |
| 4c. `make cross` | HOLDS, exit 0. `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`; every package `ok`, including internal/fspath and internal/projects. The files have no build tag and use only os, errors, io/fs, path/filepath, testing |
| 4d. `git diff main --stat` | HOLDS. Names the two new test files, `internal/fspath/ib_linux_spellings_test.go` (91 +) and `internal/projects/ib_linux_spellings_test.go` (69 +), and this report `docs/lanes/C-ib-fspath-linux-spellings.md`, which the runbook requires; no production file. `git status --short` was clean after each revert |
| Tests of changed and importing packages | HOLDS, exit 0. Importers by `go list -f` over `.Deps` of fspath and projects: cmd/grid, approvefix, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, ports, question, signal, task, usage, workflow, lib. `go test -count=1` over those plus fspath and projects: all `ok` (cmd/grid, approvefix, job/jobtest have no test files). Fix round 1 changed only a string literal and a comment in a test file of fspath; fspath and projects were rerun (4a) |
| Windows and macOS builds | HOLDS, via `make cross` (exit 0), which vets and compiles the tests of every package for darwin/arm64 and windows/amd64 |
| New tests fail on base, pass on lane | Pin-only, so they pass on base. The failing lines are the mutation runs above, one per row |
| Skips | HOLDS. Each test skips (never fails) when `foldsCase` says the volume folds, when creating the second spelling returns an already-exists error (for example APFS), or when `os.ReadDir` does not list two entries (`ibFsTwoFolders`, both files). On this Linux sandbox none skipped |
| No existing test edited or deleted | HOLDS. The diff only adds files |
| No production file changed | HOLDS. Both mutations were reverted with `git checkout`; the diff stat above |
| Names | HOLDS. Tests start with `TestIbFspath`, helpers with `ibFs` (`ibFsTwoFolders` in each package, `ibFsRoot` in fspath). `foldsCase` of each package is reused, nothing redeclared |
| No guard, gate, cap, hold or blocking hook | HOLDS. None added |
| RealPlugin tests | Not applicable: no row names one |
| Commits | One-sentence messages, no attribution lines. The files hold no personal path, host or email |

## What changed

- New internal/fspath/ib_linux_spellings_test.go: `TestIbFspathTwoCaseSpellings` (Alpha and aLPHA, plus Gamma and each with a missing tail `x`) and `TestIbFspathTwoUnicodeForms` (the NFC and NFD forms of cafe with an acute e, written as Go escapes, each with and without a tail).
- New internal/projects/ib_linux_spellings_test.go: `TestIbFspathProjectsKeepTwoFolders`, six sameFolder and within assertions on the same two folders.
- Each test comment says it pins what the code does today and that whether two such folders are two projects on Linux is an open owner question with default yes (backlog-11nov-ib/INDEX.md, Owner questions).

## Notes

- Row 1 also asserts the folders themselves (`<root>/Alpha`, `<root>/aLPHA`) and a second tail, one more than the brief asked; they hold on the base.
- Row 3 also asserts the reverse pairs (`sameFolder(aLPHA, Alpha)`, `within(aLPHA, Alpha/sub)`), so a one-sided mutation cannot pass.
- Why the letters got in: the edit tools turn a typed backslash-u escape into the letter before it reaches the file. The escapes now in the file were stored by typing the backslash as its own escape, and the result was checked with grep, not by eye.

## NOT VERIFIED on a live screen

Nothing here is a screen row. Not run: a case-folding volume (macOS default, Windows) and a normalising one (APFS), where all three tests skip; the folding branch is covered by the existing `TestOnDiskCase` and `TestCaseSpellingsOfOneFolderAreOneProject`, which skip on Linux. A SKIP proves no row, so the three rows hold only on a case-sensitive volume, as on this Linux sandbox.

DONE
