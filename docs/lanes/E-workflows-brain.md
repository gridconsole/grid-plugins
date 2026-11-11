# E-workflows-brain: a workflow can be a brain note

Head: see the report. Repo: plugin, branch lane-E-workflows-brain.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. Brain reader and note fields | HOLDS, with a stated deviation | The lane reads brain files with `safefile.Read` and `mdfile.Load`, the calls `brain.scan` uses (internal/brain/topics.go:227, 231), in `workflowNotes` (internal/workflow/brain.go:154-240). Fields read: front matter `type: workflow` (brain.go:28, 219), `name:` (must equal the file name, brain.go:225), and `status:` (only accepted, or absent, counts; brain.go:222). Layout is `knowledge/<topic>/<name>.md` of the Grid home or `projects/<project>/`, as `brain.Store` lays it out; names are checked with `brain.ValidName`. Deviation: `brain.Store` and `brain.Parse` cannot be the reader. `brain.Parse` is strict (internal/brain/note.go:27 via mdfile.Load strict) and refuses `gate_post_roles`, and topic `workflows` is refused by `topicAt` (internal/brain/topics.go:63). See open items. |
| 2. Order task, project note, home note, default; Answer says which | HOLDS | `ReadFor` (internal/workflow/brain.go:36) tries `workflow.md`, project brain, home brain, then the default, and sets `Answer.Source` and `Answer.Note` (internal/workflow/read.go). Test `TestReadForOrder` (brain_test.go:64): home note answers `home-brain`, a project note then beats it (`project-brain`), a task `workflow.md` beats both (`task`). `go test ./internal/workflow/...` exit 0. |
| 3. One parser for both places | HOLDS | `take` (brain.go:74) is the only reader of front matter and body for task text and note text; it calls `gatePostRoles` (gates.go, the same code `GatePostRoles` uses) and `Parse` (parse.go). `TestWorkflowNoteAndWorkflowMdShareOneParser` (brain_test.go:152) reads the same text from `workflow.md` and from a project note and gets equal gate roles, templates (2 steps, approvers sam and robin) and errors; only Source, Note and the File label differ and the test blanks those. |
| 4. Go tests: no note, project beats home, broken note | HOLDS | `TestReadForNoNoteGivesTheDefault` (brain_test.go:46), `TestReadForOrder` (:64), `TestReadForBrokenNoteGivesTheDefaultAndAProblem` (:97: bad gate kind gives the default plus a `brain:home/bad` problem line and no error; front matter that is not valid YAML but says `type: workflow` gives the default plus a `brain:home/bad` problem line, and skips only that file so a good note beside it still answers (fix round 1: `claimsWorkflow`, brain.go:257; test lines 112-130; the same test failed on the old brain.go with `invalid yaml: source = "default" errors = []`); a non-workflow note with broken YAML stays silent; a broken project note falls to a good home note with its line kept; a name that is not the file name, two workflow notes and a broken task workflow.md each give a problem line and fall through), `TestReadForSkipsNotesThatAreNotAcceptedWorkflows` (:138). ReadFor returns no error, so a broken file cannot stop a task. |
| 5. go test ./internal/workflow/... and make lint | HOLDS | `go test ./internal/workflow/...` exit 0 (ok, 15.5s). `make lint` exit 0 (go vet and gofmt clean). |
| New tests fail on base | HOLDS | On origin/main sources with the new test file: `internal/workflow/brain_test.go:48:9: undefined: ReadFor`, `:49:19: undefined: SourceDefault`, `:52:43: undefined: defaultGatePostRoles`, `:70:19: undefined: SourceHomeBrain` (compile failure, all five tests). |
| Importers' tests | HOLDS | Packages that depend on internal/workflow (`go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...`, filtered): cmd/grid, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, orchestrator, packaging, packaging/cmd, ports, question, signal, task, usage. `go test` over all of them plus ./internal/workflow/...: every package ok or no test files, exit 0. Fix round 1 changed only unexported functions, so the exported API is unchanged; re-ran the direct importers `go test ./internal/workflow/... ./internal/feed/... ./internal/task/... ./internal/usage/... ./internal/cli/...` exit 0; `make lint` exit 0; Windows and macOS builds exit 0; `gofmt -l internal/workflow` empty. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Edited only the named packages | HOLDS | Changed: internal/workflow/{brain.go, brain_test.go, read.go, gates.go} and this file. Not internal/brain, not the app. No existing test edited or deleted. |
| No guard, gate, cap, hold or blocking hook | HOLDS | Nothing added that refuses or holds an agent. A bad file gives the default and a problem line. |
| Public repo: no personal paths, hosts, emails | HOLDS | Tests use temp dirs and the names `notes-app`, `sam`, `robin`. |
| Commit rules | HOLDS | One commit, one-sentence message, no attribution lines. |

## What changed

- `internal/workflow/brain.go` (new): `ReadFor(r, project, taskDir) Answer`, the source constants, and the brain note finder.
- `internal/workflow/read.go`: `Answer` gains `Source`, `Note` and `GatePostRoles`, all `omitempty`, so existing output is unchanged.
- `internal/workflow/gates.go`: `GatePostRoles` is split into `defaultGatePostRoles` and `gatePostRoles(raw)` so one parser serves `workflow.md` and a note. Behaviour and error text are unchanged (existing gate tests pass).
- A workflow note is a brain note (`type: workflow`, `name:` equal to its file) with the same front matter as `workflow.md` and, if it has a `# Workflows` body, the same template sections (steps, approvers). A guest does not read the home brain; a project the caller cannot see is skipped.

## Open items

1. **Deviation from the brief, `Read`.** `Read(r, project)` lists `workflows.md` templates for the Settings page and has no task folder, and `cli/workflow.go` takes it as a function value, so its signature stays. The new entry point is `ReadFor`. Nothing calls it yet: `feed` still calls `GatePostRoles(taskDir)`, so gates do not yet come from a brain note until a caller switches to `ReadFor(...).GatePostRoles`. That change is outside the packages this brief allows.
2. **Deviation from the brief, the brain reader.** The brief says read notes through the brain's existing reader. That reader cannot carry the policy keys, so `workflowNotes` reads the files itself. A brain-side change (a `workflows` topic in `BuiltinTopics`, or a policy-aware note schema) would let it move to `brain.Store`. This needs the owner's or the brain lane's decision.
3. **Several workflow notes at one level** is a problem line and the level is skipped; the brief names one note per level. If a project should hold several, say how the task picks one.
4. The "shipped default" is the code default (`defaultGatePostRoles`, no templates). No default workflow text ships in the repo.

## NOT VERIFIED on a live screen

Nothing here is UI. Not exercised: a real Grid home with a note written by the brain keeper, and the dev UI.
