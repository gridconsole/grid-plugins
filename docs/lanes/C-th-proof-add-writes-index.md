# C-th-proof-add-writes-index: `grid proof add` keeps proof/index.json fresh under its lock, and a GIF is proof

Head: see the report (the commit that adds this file follows the commit "Add tests for the index written by proof add and for gif proof.").

Base: the clone started at 792b8d6e (lane-Pint142 merge; local `main` is the same commit), not 7a4bc0c2 as the brief names. The base check was run on it:

```
$ git grep -n "index.json" -- internal/proof/proof.go
internal/proof/proof.go:312:	path, err := localdata.Safe(dir, "proof", "index.json")
```

Only Rebuild's line (:312, inside `Rebuild`, lock at :303). Add did not write the index, so the lane went on.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1 Red on base, green on lane: `TestThIdxAddWritesIndex` | HOLDS | Base (tests added, production untouched), `go test -count=1 -v -run TestThIdx ./internal/proof/`, exit 1: `th_add_index_test.go:65: read index: open /tmp/TestThIdxAddWritesIndex1497812101/001/proof/index.json: no such file or directory` then `--- FAIL: TestThIdxAddWritesIndex`. Lane, same command, exit 0: `--- PASS: TestThIdxAddWritesIndex`. The test makes the package fixture, adds item 1 once, reads `proof/index.json`, and requires it to DeepEqual `Build()` and byte-equal `MarshalIndent(Build())`+newline; i-1 lists exactly the returned record; i-2 is listed with none. A second Add (item 2) is shown in the file too. Code: proof.go:179-187 (the call in Add), proof.go:320 (Rebuild's call), proof.go:323-337 (`thIdxWrite`). |
| 2 `TestThIdxConcurrentAddsListAll` | HOLDS | Base: `th_add_index_test.go:122: read index: ... proof/index.json: no such file or directory`, `--- FAIL: TestThIdxConcurrentAddsListAll`. Lane, exit 0: `--- PASS: TestThIdxConcurrentAddsListAll (0.03s)`. 16 goroutines, items "1" and "2" (i%2), released together on a channel, waited on by a WaitGroup; no sleep, no retry. After they return, `index.json` equals `json.MarshalIndent(Build())` + `\n` byte for byte, lists 16 distinct records, and each returned record equals the listed one. Also `go test -count=1 -race -run 'TestThIdx\|TestGgProof' ./internal/proof/` exit 0. |
| 3 `TestThIdxGif` | HOLDS | Base: `screenshot file must be png, jpg or webp` and `video file must be mp4 or webm` on a `.gif`, `--- FAIL: TestThIdxGif`. Lane, exit 0: `--- PASS: TestThIdxGif` with subtests screenshot, video, refusals_name_gif, refused_add_leaves_no_index all PASS. A `rec.GIF` is kept as `.gif` for both kinds (stored bytes equal, index record's file ends `.gif`, kind kept); the two refusal messages contain "gif"; an empty `x.log` gives exactly "proof must contain 1 byte to 20 MiB" and `proof/index.json` does not exist afterwards. Code: proof.go:129 (extension kept), :138-143 (kinds and messages). |
| 4 `go test -count=1 ./internal/proof/ ./internal/cli/ ./internal/task/` | HOLDS | Exit 0: `ok .../internal/proof 0.492s`, `ok .../internal/cli 101.735s`, `ok .../internal/task 7.423s`. Existing tests unedited except `proof_add_build_test.go:121-122` (the two message literals). `TestGgProofConcurrent*`, `TestProofIndexLists` (internal/cli/proof_test.go) and the deliver tests passed as they are. |
| 5 Diff names only the three files; `make lint`; `make cross` | HOLDS | `git diff main --stat` before the report commit: `internal/proof/proof.go \| 24`, `internal/proof/proof_add_build_test.go \| 4`, `internal/proof/th_add_index_test.go \| 212`, 3 files changed. (The report file `docs/lanes/C-th-proof-add-writes-index.md` is a fourth path once committed, as every earlier lane's report is.) `make lint` exit 0 (`go vet ./...`). `make cross` exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package ok. `gofmt -l internal/proof` empty. |
| Importers tested | HOLDS | Direct importers of internal/proof: `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...` filtered gives `internal/cli` and `internal/task` (and `internal/proof` itself); all three ran under row 4. Every other package reaches proof only through those two (23 packages, from `go list -f '{{.ImportPath}} {{join .Deps " "}} END'`: cmd/grid, braincmd, cli, env, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, ports, signal, task, updates, lib); the module-wide test is the orchestrator's. `make cross` compiled their tests for both other platforms. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok ... 1.028s`). `go test -count=1 ./internal/job/lint/` exit 0 (`ok ... 0.722s`). The app-side walk tests (`make webui-test`, `TestFvArgvCollects`, grid-server walks, slices Imports) are for the app repo; this is the plugin repo and no app file changed. |
| `*RealPlugin*` tests | HOLDS (none named) | No row names one; none was run, none is claimed. |
| New tests fail on base, pass on lane | HOLDS | Red lines under rows 1-3, taken before the production edit. `TestThIdxBrokenSiblingKeepsAdd` (extra, see below) passes on base as well: it pins a behaviour the lane chose to keep, not a new one. |
| No existing test edited or deleted beyond :121-122 | HOLDS | `git diff main --stat` above; no test deleted. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | No new refusal on an agent. The only added path is an index refresh that never fails an add. The `.gif` change widens what is accepted. The 20 MiB `MaxBytes` is unchanged. |
| No CLI flag, help text, contract/commands.json, golden/ change | HOLDS | Diff touches only the three files above. |
| No personal paths, hosts or emails | HOLDS | The new file uses `t.TempDir()` only. |
| Cross-repo reading of the planner | HOLDS (not rerun) | Not rerun: the brief says it was done by the planner and the ref app repo is outside this clone. |
| Budget (about 50 calls, one command per Bash call) | HOLDS | About 32 tool calls. |

## What changed

`internal/proof/proof.go`:

- The index write moved out of `Rebuild` into `thIdxWrite(dir, idx)`, a package-level function that takes no lock (it resolves the path with `localdata.Safe`, makes `proof/`, indents, and writes through `atomicfile`). `Rebuild` still takes `.proof.lock`, builds, and calls it; its returns are unchanged.
- `Add` calls `s.Build()` then `thIdxWrite` after the `proof.json` write and before it returns, inside the same `.proof.lock` hold (there is one lock hold and no call to `Rebuild`, so no second lock on a second descriptor).
- `.gif` is kept as `.gif` and accepted for kind screenshot and video. The messages are now "screenshot file must be png, jpg, webp or gif" and "video file must be mp4, webm or gif".

`internal/proof/proof_add_build_test.go:121-122`: the two message literals.

`internal/proof/th_add_index_test.go` (new): `TestThIdxAddWritesIndex`, `TestThIdxConcurrentAddsListAll`, `TestThIdxGif`, and one extra, `TestThIdxBrokenSiblingKeepsAdd`.

## Open items and notes

- **A failed index refresh does not fail Add.** `Add` checks the level of its own item and reads only its own `proof.json`; `Build` checks every item and re-hashes every evidence file. A plan with a broken record on another item, or an unknown proof level there, adds fine on base and would otherwise fail after the new record was already on disk, and a retry would store the proof twice. So when `Build` or the write fails, `Add` still returns the record with no error and leaves `index.json` as it was; `grid proof index` and `grid deliver` report the cause as before. `TestThIdxBrokenSiblingKeepsAdd` pins it. The brief did not say which way to go; if the owner wants Add to return the index error instead, it is a few lines at proof.go:179-187 plus that test.
- **Cost per add.** The refresh runs `Build`, which reads and hashes every stored evidence file of the task (up to 20 MiB each) while the lock is held. A task with many large recordings makes each `grid proof add` slower, and a second writer waits longer on `.proof.lock`. Nothing was timed; the brief's shape (Add calls the helper inside its own hold) was followed.
- The brief's base sha 7a4bc0c2 is not the clone's base (792b8d6e); the line numbers in the brief held as quoted.

## NOT VERIFIED on a live screen

Whether the app's Result tab shows a checker's saved screenshot or `.gif` while the task runs. This lane writes the file the app reads (folderproof.go:151, per the brief) and proves its content byte for byte against `Build()`; the app was not started and no screen was looked at. Windows and macOS are checked by `go vet` and test compile through `make cross`; the new tests have not run on a Windows or macOS host.
