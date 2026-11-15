# C-s6-brain-search-misses-note

Brief: `brain search` did not find a note that `brain read` found (end review 0212Z S6).
Base: main 842493ba. No product code changed.

## Proof table

| Row | State | Evidence |
|---|---|---|
| Acceptance 1: repro command and output before and after | NOT DONE | The round-8 grid home is not readable from this lane's session. Every read of `/home/grid2/evalprog/r8/grid-home/...` was refused: the Read tool on `history.md`, `ls .../sessions/`, `cat` and `head` of `history.md`, and `cp -r /home/grid2/evalprog/r8/grid-home tmp/r8-home/`. Without history.md:3 and the J-1 session, the search words, cwd and flags are unknown, so there is nothing to replay. |
| Acceptance 2: new test fails on main, passes on the lane; `make lint` and brain tests pass | NOT DONE | No test was written. The brief asks for a test with "the same note shape and words", and the words are in the unreadable session. A fix for a cause inferred from code alone was not made. |
| Acceptance 3: this lane doc | HOLDS | docs/lanes/C-s6-brain-search-misses-note.md |
| Rule: walk tests, merge origin/main, lint, Windows/macOS builds | not run, not asked | No code changed. This commit adds only this doc. |
| Rule: no guard, gate, cap, hold or hook added | HOLDS | No code changed. |

## What was found without the session

- The round's grid-home copy is not under `/srv/gridwork/tmp/evalprog/r8`, which holds only `bin`, `src` and `shas.txt`. The round's `setup-r8.log` shows the only seeded note. Its id is `brain:project/command-blocks`, in topic `conventions`, project `handbook`. Its description is "How shell commands are written in the handbook." Its summary and body are "Shell commands in the handbook are written in `sh` code fences, one command per line."
- The round ran the same search code as main. `diff` of `internal/brain/query.go` and `internal/hook/brain_search.go` between `/srv/gridwork/tmp/evalprog/r8/src/plugin` and this lane printed nothing.
- There is no `grid brain read` verb (internal/braincmd/command.go:170). "brain read" is the history event the PostToolUse hook writes for a `grid brain show` and for each hit of a `grid brain search` (internal/hook/brain_search.go, `brainSearch`).
- **Candidate 1, scope.** If history.md:3 ends in `why search: <words>`, then the hook's own re-run of that search hit the note. The two runs read different stores:
  - The hook takes the project from the task folder (`handbook`).
  - The CLI takes the project from the agent's cwd (internal/braincmd/command.go, `resolver.Resolve(where.Query{Cwd: env.Cwd, ...})`).
  - `hdSSameProject` returns true when neither `--project` nor `--task` is given, so the hook does not check that the cwd's project is the task's project.

  If the J-1 agent searched from a cwd in another project (for example the `tally` repo), the CLI misses the handbook note and the hook still records it as read. This is the leading hypothesis. It is not confirmed.
- **Candidate 2, tokens.** `words()` in internal/brain/query.go strips "es" to "e". So "boxes" becomes "boxe" and does not match "box", and "patches" does not match "patch". None of the note's words are hit by this, but query words could be.
- **Candidate 3, wrong words.** If line 3 has no `why search`, it came from a `show`, and the search words were probably wrong (brief step 3).

## Open items

- Give this lane read access to `/home/grid2/evalprog/r8/grid-home/projects/handbook/tasks/T-01-*/history.md` and that task's `sessions/`, or copy them under `/srv/gridwork/tmp`. Then rerun: read line 3 and the J-1 search command with its cwd and flags, replay it against a copy, and fix the scope check or the stemming as the evidence shows.

## NOT VERIFIED on a live screen

Nothing; no UI.

CLOSED BY C-s6c-brain-search-misses-note: accepted 03:31 UTC
