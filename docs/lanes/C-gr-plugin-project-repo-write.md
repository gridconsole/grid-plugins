# C-gr-plugin-project-repo-write

`grid project repo add|edit|remove` writes `projects/<project>/repos.json`, the list the app's Settings tab reads.
Code head (before this file): d0627cb4 on lane-C-gr-plugin-project-repo-write, from main.

## Proof table

| # | Row / rule | Result | Command and exit, or file:line |
|---|---|---|---|
| 1 | add prints `{"state":"applied","item":{"id":"app",...}}`; repos.json is one object, `items` holds the item, EOF after it | HOLDS | `go test -count=1 -run TestGrProjectRepo ./internal/cli/` exit 0. TestGrProjectRepoAddWritesTheItem (gr_project_repo_test.go:74) decodes with json.Decoder and checks EOF (grRepoItems, :52) and the exact bytes `{"items":[{"id":"app","where":"/tmp/x/app","remote":"git@h:a.git","branch":"main"}]}\n`. Setup sets HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME with t.Setenv inside the temp dir (grRepoHarness, :17). |
| 2 | edit changes only the given field and keeps `note`; remove takes only that item; unknown id / taken id leave repos.json byte-equal | HOLDS | TestGrProjectRepoEditKeepsOtherKeys, TestGrProjectRepoRemoveTakesOnlyThatItem, TestGrProjectRepoRefusalsLeaveTheFileAlone (13 cases: taken id, default id taken, unknown id on edit and remove, nothing to change, empty where, no where, field flag on remove, unknown verb `from`, no project operand, three malformed files) all PASS in the same run, exit 0. Extra keys keep their values and order (`note` string with `<`, `&`, quotes; `note` object). |
| 3 | guest refused with no file written; project `absent` with `--json` gives `{"error":{"code":"no_such_project",...}}` and the same stderr as without | HOLDS | TestGrProjectRepoGuestIsRefused (roles guest and grid-guest, add/edit/remove: exit 1, stderr `grid: guests cannot change project settings`, no repos.json) and TestGrProjectRepoUnknownProjectCode (add/edit/remove: exit 1, stderr equal with and without --json, `assertErrorOutput(..., CodeNoSuchProject)`), same run, exit 0. Error comes from projects/repos.go:211 as `where.LookupError{Kind: ErrProjectUnregistered}` through `commandError` (project_repo.go:100). |
| 4 | existing project verbs unchanged | HOLDS | `go test -count=1 -run 'TestProject' ./internal/cli/` exit 0 |
| 5 | `TestGgUnknown\|Contract` PASS with commands.json holding the new entry; no gg_*_walk_test.go in the diff | HOLDS | `go test -count=1 -run 'TestGgUnknown\|Contract' ./internal/cli/` exit 0. commands.json regenerated with `make generate` (the allow list refuses a leading `UPDATE_CONTRACT_COMMANDS=1`; the target runs the same go test line, and the packaging and doctor outputs it also regenerates did not change). `git diff main --stat` names no gg_*_walk_test.go. |
| 6a | `go test ./internal/cli/ ./internal/projects/` exit 0 | HOLDS | `go test -count=1 ./internal/cli/ ./internal/projects/` exit 0 (cli 66s, projects 1.5s) |
| 6b | `make lint` exit 0 | HOLDS | `make lint` exit 0 (after gofmt of guest_parity_test.go) |
| 6c | Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| 6d | `git diff main --stat` names only the Touches files | NOT HELD as written | One file outside Touches: internal/cli/guest_parity_test.go (2 lines, own commit). Without it `TestGuestProjectParity` fails on the lane: `guest_parity_test.go:350: missing guest parity command: project repo`, because that test lists every command that takes a project operand. The lines add `"project repo"` to `guestParityCommands` (:85) and to the guest early-answer case with `project remove` (:415). No assertion was changed. If the orchestrator wants the diff to match Touches, drop that commit and the brief needs the file added to Touches. |
| R1 | Package set run | HOLDS | Importers of internal/projects (go list, incl. tests): internal/cli, internal/job/jobcmd, internal/projectmd, internal/signal, internal/workflow. Importers of internal/cli: cmd/grid, contract/formats, internal/account, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on all of them plus internal/projects: all ok. The one failure in that run was TestGuestProjectParity (see 6d), fixed and re-run in the row 6a run. |
| R2 | New tests fail on base, pass on the lane | HOLDS | With `git checkout main -- internal/cli/projects.go` (base), `go test -count=1 -run TestGrProjectRepo ./internal/cli/` fails, e.g. `gr_project_repo_test.go:80: add: exit 2: grid project: unknown command "repo"`; every test that runs a repo verb fails the same way. Restored with `git checkout HEAD -- internal/cli/projects.go`; `git status --short` clean. |
| R3 | No existing test edited to pass; none deleted | HOLDS | Only additions in contract_inventory_test.go (:555 annotation, which the brief names) and guest_parity_test.go (6d). No test deleted. |
| R4 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. The guest refusal is the one the brief names (same as project add/remove/relink). |
| R5 | Small commits, one-sentence messages, no attribution lines, no personal paths/hosts/emails | HOLDS | `git log main..HEAD` shows one-sentence subjects with no trailer. The system reminder asked for a Co-Authored-By line; the owner's rule wins and none was added. Test paths are `/tmp/x/app`, `/w/...`, `/parity/app`. |
| R6 | RealPlugin tests | NOT APPLICABLE | No test here is named *RealPlugin*; none was built or run. |

## What changed

- internal/projects/repos.go: `Store.RepoAdd`, `RepoEdit`, `RepoRemove`. An item keeps its keys in file order (`RepoItem`), so keys other than id, where, remote and branch stay as they are. The list is read the way the app reads it (one object, `items` an array, nothing after); a missing file is an empty list; a file that is anything else is refused and left alone. Written with atomicfile as `{"items":[...]}` plus newline, under `.projects.lock`. An unknown project returns `where.LookupError{ErrProjectUnregistered}`; a project is found by name or id.
- internal/cli/project_repo.go: `projectRepo` (not named `run*`, so the contract inventory sees one route, `project repo`). One flag set: `--id --where --remote --branch --json`; flags may come before or after the operands. add needs `--where`, id defaults to `filepath.Base(where)`; edit needs `--id` and at least one of where/remote/branch (an empty given value clears it; empty `--where` is refused); remove needs `--id` and takes no field flag. Guests are refused after argument checks, as project remove is.
- internal/cli/projects.go: `case "repo"` before the flag set, and the usage line. cli.go:134 and help.golden: the summary names `repo`. contract/commands.json: one `repo` entry (regenerated). contract_inventory_test.go: one annotation.
- internal/cli/gr_project_repo_test.go: the new tests.

## Answers

- add / edit with `--json`: `{"state":"applied","item":{...}}`. The item is the stored object: the four keys, plus any extra keys on an edit. remove: `{"state":"applied"}`. Plain output: `repo: <id>` and `where: <where>`, or `removed: <id>`.
- Exit 2 is a usage error (no verb, unknown verb such as `from`, wrong operand count, missing `--where` or `--id`, edit with nothing to change). Exit 1 is a refusal (guest, unknown project, taken or unknown repo id, malformed repos.json).

## Open items for the review and the app route lane

- A taken id and an unknown id exit 1 with plain stderr and no error code (`errorCode` has no code for them and error_codes.go is not in Touches), so with `--json` stdout is empty. The route can only tell them apart from stderr text. A code such as invalid_argument would need error_codes.go.
- Top-level keys other than `items` in an existing repos.json are not kept: the file is rewritten as `{"items":[...]}`, as the brief says. Items that are not objects are kept as they are.
- `edit` with no field flag is a usage error (exit 2), not a no-op.
- `where` is stored as given: no existence check, no absolute-path conversion. The app marks a missing absolute path itself.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: the app's Settings tab reading a file this verb wrote, and the app route C-gr-app-project-repos-write-route calling the verb. Both belong to the app lane and need the built plugin and the app together. The strict decoder in the tests mirrors projectsettings jsonItems (app main, read from the reference repo), not the app's own code.
