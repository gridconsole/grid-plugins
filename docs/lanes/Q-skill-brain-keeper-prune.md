# Q-skill-brain-keeper-prune

brain-keeper gets a `## Prune` section whose example commands supersede a stale note and revert with a reason, proven on a seeded brain.

Base: lane branch cc89b9f9 (carries Q-brain-revert-reason, Q-brain-supersede-verb, O-brain-supersede-on-accept). Local `main` is ecd80ef5.

## Proof table

| Row | Result |
|---|---|
| 1. Test fails on base | HOLDS. With `skills/brain-keeper/SKILL.md` stashed back to the base text: `go test -count=1 -run TestPrune ./internal/braincmd/` gives `skill_prune_test.go:85: skills/brain-keeper/SKILL.md has no `## Prune` heading` (three tests FAIL, `FAIL github.com/gridconsole/grid-plugins/internal/braincmd`). |
| 2. Examples run on a seeded brain, each exit 0 | HOLDS. `TestPruneExamplesRunOnASeededBrain` (internal/braincmd/skill_prune_test.go) seeds a project brain with `deploy-old` and `deploy-new` (project record via `harness.put`, notes via propose and accept so each has v1), substitutes the ids into each fenced line and runs it through `Run`; each exits 0. `go test -count=1 ./internal/braincmd/` exit 0. |
| 3. Supersede effects | HOLDS. Same test: `deploy-old.md` contains `superseded_by: brain:project/deploy-new`; `search` lists only `brain:project/deploy-new` afterwards (query.go:118 skips superseded); `show deploy-new` holds one `supersedes` link to `brain:project/deploy-old` with a why of 8 or more characters. |
| 4. Changes shows the revert reason | HOLDS. Same test: the example's `grid brain changes` output contains `revert to v1: ` plus the reason read from the example's own `--reason` argument. |
| 5. Verbs and flags exist; no rm, delete, file edit | HOLDS. `TestPruneVerbsAndFlagsExistInTheContract` checks every `grid brain <verb>` and `--flag` in the section against `contractdata.Commands` (search, supersede, revert, changes, accept all named). `TestPruneSectionHeadingAndShape` refuses `rm `, `grid brain delete`, `sed `, `> knowledge`, a `delete` inside the example, and any line outside the fence that starts with `grid ` or `$ `. The prose says "Never delete a note"; no command deletes. |
| 6. Package test and vet | HOLDS. `go test -count=1 ./internal/braincmd/` exit 0; `go vet ./internal/braincmd/` exit 0. |
| New tests fail on base, pass on lane | HOLDS (row 1 above, row 6). |
| Importers of braincmd | HOLDS. `go list` importers: `internal/braincmd`, `internal/cli`. `go test -count=1 ./internal/cli/ ./internal/packaging/` exit 0 (packaging reads skill front matter and the contract). |
| Lint | HOLDS. `make lint` (go vet ./...) exit 0. |
| Windows and macOS builds | HOLDS. `make cross` exit 0 (vet and test compile for darwin/arm64 and windows/amd64). |
| gofmt | HOLDS. `gofmt -l internal/braincmd` prints nothing. |
| No guard, gate, cap or hold added | HOLDS. The change is skill text and a test; no code under internal/ other than the test file. |
| No existing test edited or deleted | HOLDS. One new file only. |
| No attribution lines, no personal paths | HOLDS. Commit messages are one sentence. |
| Edits only the brief's files | HOLDS. `skills/brain-keeper/SKILL.md`, `internal/braincmd/skill_prune_test.go`, this report. |

## What changed

- `skills/brain-keeper/SKILL.md`: `## Prune` section after Keep and before Quality bar. Five numbered steps (search for the old note on each proposal; newer fact wins via a `supersedes` link and accept; `grid brain supersede <old> --by <new> --reason`; `grid brain revert <topic>/<name> --to <n> --reason`; never delete or hand-edit a note) and one fenced `text` block with the example commands (search, supersede, revert, changes), each with `--project <project>`.
- `internal/braincmd/skill_prune_test.go`: three tests described above.

Note for the reviewer: the revert example reverts the old note (`<topic>/<old>`), which is the way to undo a supersede, since the old note regains the version without `superseded_by`. Step 2 (accept with a `supersedes` link) is a prose command, not in the fenced block, because the notes already exist when the block runs; the supersedes link and `superseded_by` effect of accept is covered by the O-brain-supersede-on-accept lane's tests.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Whether a live keeper agent follows the section was not run (no claude in the sandbox).
