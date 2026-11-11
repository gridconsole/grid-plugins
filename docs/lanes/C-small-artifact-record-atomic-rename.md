# C-small-artifact-record-atomic-rename

Base: plugin main 5925b841098c704e445a7bd6da9d342a05983f68 (the brief named 55c46cbd; the clone's main is 5925b841, line numbers matched except the import shift of +1).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. grep on main shows `os.Rename(` at the record write | HOLDS | On base: `git grep -n "os.Rename(" -- internal/artifact/artifact.go` printed `internal/artifact/artifact.go:382:	return os.Rename(f.Name(), filepath.Join(dir, r.ID+".json"))`. After the change: same command prints nothing (exit 1). `git grep -n "atomicfile.Rename" -- internal/artifact/artifact.go` prints `internal/artifact/artifact.go:383:	return atomicfile.Rename(f.Name(), filepath.Join(dir, r.ID+".json"))`. |
| 2. Import then Publish replaces the record | HOLDS | `TestPublishReplacesImportedRecord` (internal/artifact/record_rename_test.go): Import, Publish, Publish again, List shows Published true Kind file; `<id>.json` is a regular file; no `.record-*` left. `go test ./internal/artifact/ -count=1` exit 0. |
| 3. Link then Publish | HOLDS | `TestPublishReplacesLinkRecord`: List shows Kind "link", Published true, regular file, no `.record-*`. Same run, exit 0. |
| 4a. `go test ./internal/artifact/ ./internal/atomicfile/ -count=1` | HOLDS | both `ok`, exit 0. |
| 4b. `GOOS=windows go vet ./internal/artifact/` | HOLDS | exit 0, no output. |
| 4c. `make lint` | HOLDS | exit 0 (runs `go vet ./...`). |
| New tests fail on base | NOT APPLICABLE, as the brief states | On Linux `atomicfile.Rename` is `os.Rename`, and `save` has no seam, so the two new tests also pass on base. The fails-on-main proof is the grep in row 1. They guard the replace-an-existing-record path through the public callers. |
| Tests of packages importing artifact | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...` lists dependents: cmd/grid, braincmd, cli, env, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, orchestrator, packaging, packaging/cmd, ports, signal, task, lib. `go test -count=1` on all of them: every package `ok` or no test files, exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| No guard, gate, cap, hold or blocking hook | HOLDS | The diff is one call and one import; no refusal added. |
| No existing test edited or deleted | HOLDS | `git diff --stat` shows artifact.go and one new test file only. |
| Small commits, no attribution lines, no personal paths | HOLDS | One commit d1987903 with a one-sentence message and no trailers. |

## What changed

- `internal/artifact/artifact.go`: import `internal/atomicfile`; `save` ends with `atomicfile.Rename` in place of `os.Rename`. Same temp file, same Sync, same error returned.
- `internal/artifact/record_rename_test.go` (new): the two tests above.

## NOT VERIFIED on a live screen / on Windows

- The retry over a record another process holds open runs only on Windows (errors 5, 32, 33). I ran `GOOS=windows go vet` and a Windows build, not a Windows run. The retry loop is tested inside `internal/atomicfile`.

DONE
