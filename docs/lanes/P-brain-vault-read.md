# Lane P-brain-vault-read

The brain reads an Obsidian vault: path ids, body links, defaults, `grid brain list --json`, and a Read hook that logs a vault note by its path id. Reading only; the write verbs keep their old layout.

Base: `4ca3d777de965de3956eaeffc3ceba3f73bbf0f8` (the base-check `git grep` for `Root|SplitOptional|ParseVault|ValidID|LinkableID` printed nothing). Then `git fetch origin && git merge origin/main` once, before the last proof run.

## Proof table

| # | Row | Result | Command / place |
|---|---|---|---|
| 1 | V13: F1-F6 read and list correctly | HOLDS | `go test -count=1 -run TestPbVr ./internal/brain/` exit 0. `TestPbVrFixturesList` (F1 no front matter, F2 tags/aliases/cssclasses, F3 nested, F4 spaces and a decomposed `Café`, F6 `.obsidian`/`.trash`/`Proposals`/other project skipped), `TestPbVrLinks` (F5 `[[x\|y]]` in a table, `[[x#h]]`, `![[x]]`, `[t](x%20y.md)`, code and fence skipped, `## Links` rel and why), `TestPbVrOldFrontMatterLinks`, `TestPbVrLookupOrder`, `TestPbVrListJSONShape`. The same through the command: `TestPbVrListJSONOfAVault`, `TestPbVrShowVaultIDWithSpaces` in `internal/braincmd`. |
| 2 | V13: an old `knowledge/` brain lists with the key absent | HOLDS | `TestPbVrLegacyBrainStillLists` (brain), `TestPbVrListOfAnOldBrain` (braincmd), and through the binary's own environment `TestPbVrBinaryListsAnOldBrainWithTheKeyAbsent` (cli); every existing brain, braincmd, hook and cli test is unchanged and green (rows 4 and 8). |
| 2b | The app rigs `ca_page_note_realplugin_test.go` and `fw_vault_realplugin_test.go` stay green | not run, not asked of this repo | Neither file is in this repo and the cross-repo set is the orchestrator's. A skip would prove nothing, so none is claimed. |
| 3b | The key reaches the binary: `grid brain list --json` with the config's `brain` key lists the vault | HOLDS | `TestPbVrBinaryListsTheVaultOfTheConfig` in `internal/cli` (`go test -count=1 -run TestPbVr ./internal/cli/` exit 0) writes a config with a `brain` key, runs `Run` with `OSEnv()`, and checks root, rows and `show` by path id. The CLI passes the data folder as `GridHome` and `Env.Home`; `WithConfig` finds the workspace by that data folder. |
| 3 | V13: a `Read` of `Ideas/Café plan.md` logs a brain read | HOLDS | `go test -count=1 -run TestPbVr -v ./internal/hook/` exit 0: `TestPbVrReadOfVaultNoteLogsBrainRead` expects `· brain read · brain:home/Ideas/Café plan · by claude · via session session-demo`; `TestPbVrReadOutsideTheVaultNotesLogsNothing` for `.obsidian/`, another project, a `.txt`, a missing file, a file outside the vault. Code: `internal/hook/brain.go:53`, `:80`. |
| 4 | `go test -count=1 ./internal/brain/ ./internal/braincmd/ ./internal/hook/ ./internal/mdfile/` exits 0, existing brain tests unchanged | HOLDS | exit 0, all four `ok`, at the final head. No existing test body was edited; two table rows were added to existing tests and one golden line changed (see "Existing files touched in tests"), no assertion changed. |
| 5 | `go test -count=1 -run 'TestContractCommandsInventory\|TestContractCommandsParserInventory\|TestHsIpWalksEveryCommand' ./internal/cli/` after the merge | HOLDS | `--- PASS: TestContractCommandsInventory`, `--- PASS: TestContractCommandsParserInventory`, `--- PASS: TestHsIpWalksEveryCommand`. |
| 6 | `make lint` | HOLDS | `make lint && echo ok` printed `go vet ./...` then ok, exit 0. |
| 7 | `make cross` | HOLDS | exit 0. |
| 8 | Importers of the changed packages | HOLDS | `go list` imports of brain, braincmd, hook, mdfile: cli, evalcmd, job, job/jobcmd, job/jobtest, land, packaging, people, projectenv, projectmd, routine, settings, task, workflow. `go test -count=1` of all of them, plus `./internal/cli/` whole (47 s), exit 0, every line `ok`. |
| 9 | Walk tests: `TestHsIpWalksEveryCommand`, `TestIlGuestSiteList`, `./internal/job/lint/` | HOLDS | `./internal/cli/` whole package ok (covers both walks and `TestGuestProjectParity`); `internal/job/lint` ok. |
| 10 | New tests fail on base | HOLDS | A detached worktree of `origin/main` with the three new test files copied in: `go test` of `internal/brain` and `internal/braincmd`: `unknown field BrainDir in struct literal of type Store`, `undefined: ListRow`, `s.List undefined`, `undefined: brain.Listing` (build failed). `internal/hook`: `--- FAIL: TestPbVrReadOfVaultNoteLogsBrainRead`, `pbvr_brain_read_test.go:72: history after the Read: ""`. Worktree removed. |
| 11 | Rule: no guard, gate, cap, hold or blocking hook | HOLDS | Nothing added that refuses an agent. The Read hook only records. The one refusal in the read path, `brain paths cannot be symbolic links`, is the existing one, kept. |
| 12 | Rule: Touches only | HOLDS with the three forced files below | `git diff --stat origin/main` lists the Touches files, the new `pbvr_` test files, this doc, and the three forced files `internal/cli/contract_inventory_test.go`, `internal/cli/guest_parity_test.go`, `testdata/golden/brain-help.golden`: walk tables and a golden that the new `list` verb forces, named here under the runbook's line 13. |
| 13 | Rule: no attribution lines, no personal data, nothing pushed | HOLDS | commits are one sentence with no trailer; no push; the README and tests use only generic names. |
| 14 | Rule: no sudo, no work outside the lane clone, no claude/curl, no server | HOLDS | None used. |

## What changed

- `internal/mdfile/markdown.go`: `SplitOptional`, Split for a file where front matter is optional (a lone `---` rule is not front matter; a BOM is tolerated).
- `internal/brain/note.go`: `ValidID` (path id: not empty, no `..`, no `\`, no control characters, valid UTF-8, 255 bytes a segment) and `LinkableID` (also none of `# | ^ [ ]`).
- `internal/brain/store.go`: `Store.BrainDir`, `Store.Author`, `Store.Root()` (BrainDir, else GridHome), `Store.Vault()`.
- new `internal/brain/vault.go`: `ParseVault` (lenient), the body and front-matter link readers, the 4.2 defaults, `NFC`, the walk and its skips, the 2.1 lookup, `ListRow`/`Listing`/`Edge`, `Store.List`, vault `Show`/entries/topics/rows, `Store.WithConfig` (reads the workspace `brain` and member through `gridhome.Resolve`, so no edit to `internal/cli`), `Store.VaultID`.
- `internal/brain/query.go`, `index.go`, `topic_text.go`: vault branches. `Search` weighs aliases and tags like a description. `RebuildIndex` in a vault writes nothing. A top-level folder's `TOPIC.md` is read leniently for the topic brief.
- `internal/braincmd/command.go`: the `list` verb with `--json`, `--project`, `--level`; the store is built with `WithConfig(env.Home, nil)`; `show brain:<level>/<id>` takes a path id in a vault.
- `internal/hook/brain.go`: `brainStore` uses the config helper; a Read inside a vault derives the id from the path and must resolve to the very file read.
- `contract/commands.json`: `brain list`. README: "Use your Obsidian vault as the Grid brain".
- New tests, helpers prefixed `pbVr`: `internal/brain/pbvr_vault_test.go`, `internal/hook/pbvr_brain_read_test.go`, `internal/braincmd/pbvr_list_test.go`, `internal/cli/pbvr_brain_list_test.go`.

### Existing files touched in tests (forced by the new `list` verb, named here)

- `internal/cli/contract_inventory_test.go`: one annotation row, `"brain list": {}`; `TestContractCommandsParserInventory` fails without it (`new parser route "brain list" needs operand/output annotations`). The brief names this test as the pin on `contract/commands.json`.
- `internal/cli/guest_parity_test.go`: one row, `"brain list": {}`; `TestGuestProjectParity` fails without it (`missing guest parity command: brain list`).
- `testdata/golden/brain-help.golden`: the usage line now lists `list` and has its row; `TestRemainingTaskOperandHelp/brain` compares to it.

No existing test was deleted or loosened.

## Deviations and open items

- **NFC is partial.** The module has no `golang.org/x/text`, so `NFC` composes the Latin letters of Western and Central European names (grave, acute, circumflex, tilde, macron, breve, dot, diaeresis, ring, double acute, caron, cedilla, ogonek). Other scripts pass through unchanged. The README says so.
- **`updated`** is the front matter `updated`, else the file's modification time, until change records exist (L4). **`origin`** is `person` for every note until P-brain-adopt. **`author`** is the config member. `conflict` is always `[]`.
- **Write verbs stay legacy.** `propose`, `accept`, `write`, `decide`, drop and the rest still use the data folder's `knowledge/` layout even with the key set; reading a vault and writing a proposal do not meet yet (writer lanes).
- **Rename records** in the lookup (2.1, step 4) are L4, not built.
- **A symbolic link** to a note or to a folder in the vault aborts the whole listing with the existing error `brain paths cannot be symbolic links` (a link to another kind of file is skipped), instead of skipping that file.
- **`grid brain show` in a Bash command** with a quoted vault id (`"brain:home/Ideas/Café plan"`) logs a read: `TestPbVrBrainShowOfAVaultIDLogsBrainRead`. Quoting is needed for an id with a space.
- **Config built-ins** (`grid/agents` and so on) are listed with their kind but kept out of Show, Search and the index entries.
- Row 2 names the app rigs `ca_page_note_realplugin_test.go` and `fw_vault_realplugin_test.go`; they are not in this repo and the cross-repo set is the orchestrator's. `*RealPlugin*` skips in this repo; no skip is claimed as proof.
- **Check, not run:** `whole-module go test` | not run, not asked.

## NOT VERIFIED on a live screen

- A real Obsidian vault on macOS with decomposed names, and the Grid app showing the list. Only temp-folder fixtures were used.
- Claude Code's own Read tool event against a vault; the hook is tested with the same input shape as the existing Read tests.
- The two app rigs above against a built plugin.
