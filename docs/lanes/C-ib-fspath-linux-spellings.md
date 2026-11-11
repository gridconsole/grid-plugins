# C-ib-fspath-linux-spellings

Base sha: ee8a96c126781462cbe0067771db40d01911cd76 (main in this clone; the brief names 91fadeff, and the fspath.go lines it quotes, :33-39, :53-69 and :62, are the same on this base). No test on the base creates two spellings of one name on a case-sensitive volume and calls OnDiskCase: fspath_test.go:38-40, projects_test.go:331-332 and guard/case_test.go (`skipUnlessFolding`) all skip first. So the brief applies.

This lane pins today's behaviour and changes no production file. The new tests therefore pass on the base too; their failing lines come from the mutations in rows 1 to 3, applied in the clone and reverted.

## Proof table

| Row | Result |
|---|---|
| 1. TestIbFspathTwoCaseSpellings | HOLDS. `go test -count=1 -v -run TestIbFspath ./internal/fspath/ ./internal/projects/` exit 0: `--- PASS: TestIbFspathTwoCaseSpellings (0.00s)`. Mutation `os.SameFile(want, info)` to `!os.SameFile(want, info)` at fspath.go:62 (exit 1): `ib_linux_spellings_test.go:69: OnDiskCase("<root>/Alpha") = "<root>/aLPHA", want it unchanged` and the same for `<root>/Alpha/x`. In this run the directory listed aLPHA first, so Alpha was the one rewritten; with the other listing order aLPHA is rewritten instead. `Gamma` is not affected (its swapped name `gAMMA` is missing, fspath.go:37-38). Reverted with `git checkout -- internal/fspath/fspath.go` |
| 2. TestIbFspathTwoUnicodeForms | HOLDS. `--- PASS: TestIbFspathTwoUnicodeForms (0.00s)`. Same mutation (exit 1): `OnDiskCase("<root>/café") = "<root>/café", want it byte for byte as given` and the same with `/x` appended. The names are Go escapes in the file (`"café"`, `"café"`) |
| 3. TestIbFspathProjectsKeepTwoFolders (package projects) | HOLDS. `--- PASS: TestIbFspathProjectsKeepTwoFolders (0.00s)`. It asserts sameFolder(Alpha, aLPHA) false (both orders), sameFolder(Alpha, Alpha) true, within(Alpha, aLPHA/sub) false, within(aLPHA, Alpha/sub) false, within(Alpha, Alpha/sub) true. Mutation `sameFolder` body to `strings.EqualFold(a, b)` at projects.go:359 (exit 1): `ib_linux_spellings_test.go:66: sameFolder Alpha, aLPHA = true, want false` and `sameFolder aLPHA, Alpha = true, want false`. The SameFile mutation of row 1 also fails it: the two sameFolder lines plus `within Alpha, aLPHA/sub = true, want false` and `within aLPHA, Alpha/sub = true, want false`. Reverted with `git checkout -- internal/projects/projects.go` |
| 4a. `go test -count=1 -v -run 'TestIbFspath' ./internal/fspath/ ./internal/projects/` | HOLDS, exit 0. Three PASS lines (pasted in rows 1 to 3), no SKIP line; `ok .../internal/fspath 0.004s`, `ok .../internal/projects 0.194s` |
| 4b. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output after it); `gofmt -l internal/fspath internal/projects` prints nothing |
| 4c. `make cross` | HOLDS, exit 0. `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`; every package `ok`, including internal/fspath and internal/projects. The files have no build tag and use only os, errors, io/fs, path/filepath, testing |
| 4d. `git diff main --stat` | HOLDS. Names only `internal/fspath/ib_linux_spellings_test.go` (90 +) and `internal/projects/ib_linux_spellings_test.go` (69 +); `git status --short` was clean after each revert |
| Tests of changed and importing packages | HOLDS, exit 0. Importers by `go list -f` over `.Deps` of fspath and projects: cmd/grid, approvefix, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, ports, question, signal, task, usage, workflow, lib. `go test -count=1` over those plus fspath and projects: all `ok` (cmd/grid, approvefix, job/jobtest have no test files) |
| Windows and macOS builds | HOLDS, via `make cross` (exit 0), which vets and compiles the tests of every package for darwin/arm64 and windows/amd64 |
| New tests fail on base, pass on lane | Pin-only, so they pass on base. The failing lines are the mutation runs above, one per row |
| Skips | HOLDS. Each test skips (never fails) when `foldsCase` says the volume folds, when creating the second spelling returns an already-exists error (for example APFS), or when `os.ReadDir` does not list two entries (`ibFsTwoFolders`, both files). On this Linux sandbox none skipped |
| No existing test edited or deleted | HOLDS. The diff only adds two files |
| No production file changed | HOLDS. Both mutations were reverted with `git checkout`; the diff stat above |
| Names | HOLDS. Tests start with `TestIbFspath`, helpers with `ibFs` (`ibFsTwoFolders` in each package, `ibFsRoot` in fspath). `foldsCase` of each package is reused, nothing redeclared |
| No guard, gate, cap, hold or blocking hook | HOLDS. None added |
| RealPlugin tests | Not applicable: no row names one |
| Commits | One commit, one sentence, no attribution lines. The files hold no personal path, host or email |

## What changed

- New internal/fspath/ib_linux_spellings_test.go: `TestIbFspathTwoCaseSpellings` (Alpha and aLPHA, plus Gamma and each with a missing tail `x`) and `TestIbFspathTwoUnicodeForms` (NFC and NFD café, each with and without a tail).
- New internal/projects/ib_linux_spellings_test.go: `TestIbFspathProjectsKeepTwoFolders`, six sameFolder and within assertions on the same two folders.
- Each test comment says it pins what the code does today and that whether two such folders are two projects on Linux is an open owner question with default yes (backlog-11nov-ib/INDEX.md, Owner questions).

## Notes

- Row 1 also asserts the folders themselves (`<root>/Alpha`, `<root>/aLPHA`) and a second tail, one more than the brief asked; they hold on the base.
- Row 3 also asserts the reverse pairs (`sameFolder(aLPHA, Alpha)`, `within(aLPHA, Alpha/sub)`), so a one-sided mutation cannot pass.

## NOT VERIFIED on a live screen

Nothing here is a screen row. Not run: a case-folding volume (macOS default, Windows) and a normalising one (APFS), where all three tests skip; the folding branch is covered by the existing `TestOnDiskCase` and `TestCaseSpellingsOfOneFolderAreOneProject`, which skip on Linux. A SKIP proves no row, so the three rows hold only on a case-sensitive volume, as on this Linux sandbox.

DONE
