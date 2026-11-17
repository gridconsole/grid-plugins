# Lane P-brain-board-decide

This is the decision lane for tasks, Jobs and the board as brain nodes. The decision note is `docs/research/brain-board.md`. The lane adds no product code. The one code file it adds is the measurement program for Q2, `docs/research/testdata/boardbench/main.go`. It sits under `testdata/`, so `go build ./...`, `go vet ./...` and `go test ./...` never build it.

The proof was taken after `git merge origin/main` ("Already up to date", plugin main `885dc9a6`). App evidence was read at app main `e60fab587`, using `git grep` and `git show` on the reference repository.

## Proof

| Row | Status | Proof |
|---|---|---|
| Decision note at the top: decided, why, rejected options | HOLDS | `docs/research/brain-board.md`, section "Decision": 11 decided points, a "Why" list and a "Rejected" table of 7 options. |
| Q1 fencing, ordering, stale-lease rules and the fold, redone on attempts, with failure cases | HOLDS | Section Q1 has three tables. The first gives the verdict per property. The second gives one replacement for each current use of lease, fence or gen (fold.go:63-75, :192-209, :217, :273, :317-326, :472; store.go:126-157, :251-289, :347-369; read.go:107-117; orchestrator/write.go:36-80). The third lists failure cases F1 to F12, each with its handling. |
| Q2 read performance at 30 agents, with numbers from a local measurement | HOLDS | `go run docs/research/testdata/boardbench/main.go`, exit 0, run twice. Both runs are in the Q2 table, with caveats: the "today" baseline is cold, the write has no fsync, and the filesystem type was not checked. |
| Q3 every reader and writer that changes, plugin and app, with file:line | HOLDS | Section Q3 has a plugin table, 24 named `feed.Store` call sites, and app tables split into CLI-mediated readers, direct file readers, the web UI and guest scope. |
| Q4 status block: format, single writer, when rewritten, Obsidian edits kept | HOLDS | Section Q4, plus the inbox note. Its rendering in Obsidian is NOT VERIFIED (the VM has no Obsidian). |
| Q5 proof screenshots: path and link form; compression left to the curator | HOLDS | Section Q5. Its rendering in Obsidian is NOT VERIFIED. |
| Q6 ranking and context: knowledge above history, start from the Job node, SessionStart index only, guest sees one task folder | HOLDS | Section Q6. The guest scope is written as a requirement on P-shared-home. |
| Q7 forward-only migration with a fold-equality check | HOLDS | Section Q7, steps 1 to 7. "Equal" is defined as a list of fields dropped and a list compared. Records stale only by takeover offset, and `sameRetry` grants, are handled through `legacy`. |
| Q8 post read/write interface for the launcher and the vault reader | HOLDS | Section Q8: the header key table, `board.Board` methods for per-post files (including `Claim`), the launcher post types and the vault reader rules. It is aligned with the sibling notes R-run-jobs-launcher and R-brain-obsidian, which landed during this lane. The one deviation, the claim file name, is stated with its reason. |
| Q9 lane plan: order, repo, model/effort, files, acceptance rows, dependencies; format and migration at xhigh | HOLDS | Section Q9. B1 (format) and B3 (migration) are opus xhigh. Rows are added to run-jobs lanes P-loop-board, P-loop-home and P-loop-retire, so no two lanes edit fold.go, internal/lease or the status writer. |
| Ruling of 17 Nov 02:12 followed (attempts, no lease, version-checked writes, caches never sync, home launcher writes the status block, inbox note) | HOLDS | Decision points 2 to 7, Q1, Q4. No lease or lock is proposed across machines. The locks named are local to one machine and order honest writers there. |
| Last line is DONE or NOT DONE | HOLDS | The last line of `docs/research/brain-board.md` is `DONE`. |
| Copy to the hand folder as R-brain-board.md | HOLDS | `cp`, exit 0 (after the last commit). |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | This is a decision note only. The local stale-attempt refusal in Q1 refuses only a post the fold would already ignore. It is stated as not a gate, and it is a proposal for a build lane. |
| No personal paths, hosts or emails in committed files | HOLDS | `grep -nE` for private path prefixes, home folders and email domains on the note, the bench and this doc: no match, exit 1. |
| No attribution lines; small commits with one-sentence messages | HOLDS | `git log origin/main..HEAD` |
| Merge origin/main before the proof | HOLDS | `git fetch origin`, exit 0. `git merge origin/main`: "Already up to date", exit 0. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'`: ok, exit 0. `-v` shows both tests ran (40 commands walked). `go test -count=1 ./internal/job/lint/`: ok, exit 0. |
| Lint | HOLDS | `make lint` (`go vet ./...`), exit 0. |
| Windows and macOS builds | HOLDS | `env GOOS=windows go build ./...`, exit 0. `env GOOS=darwin go build ./...`, exit 0. |
| Bench file formatted | HOLDS | `gofmt -l docs/research/testdata/boardbench/main.go`: no output, exit 0. |
| Skill grid:research | HOLDS, with a deviation | The brief's layout (decision note first, Q1 to Q9, last line DONE) wins over the skill's research.md shape. This lane is not a Grid Job, so no `grid job result` was posted. |
| Skill go-conventions | HOLDS | It applies only to the bench program: stdlib only, gofmt clean, kept out of `./...`. |
| make test, full go test | not run, not asked | — |

## What changed

- `docs/research/brain-board.md`: the decision note.
- `docs/research/testdata/boardbench/main.go`: the Q2 measurement (stdlib, Linux flock, about 2 s).
- `docs/lanes/P-brain-board-decide.md`: this doc.

## NOT VERIFIED

- How Obsidian renders the status callout and its markers, relative image links, and a one-line JSON `body:` in the Properties view.
- Obsidian's indexing speed with thousands of post files per task.
- Hard links on Windows network drives and on exFAT (the F12 fallback).
- Fold cost at 6,000 posts. It was not measured and is left to lane B1.
- The filesystem type of the temp directory the benchmark ran in.
- Most file:line citations come from reads made during this lane, before the note was written. Only internal/proof/proof.go, internal/orchestrator/oh_quiet_task.go and internal/mcp/tools.go were read again at the end.

## Open items

- The claim file name differs from the R-run-jobs-launcher suggestion (seq-prefixed name plus a lock-checked `Claim`, instead of `board/<job>-a<n>-started.md`). The owner or that lane may prefer the keyed name.
- Merging this note's rows into run-jobs lanes 1, 5 and 11 needs those lanes' briefs to carry them.
- Requirements on other lanes: P-shared-home (per-task guest scope, per-folder ACLs) and the sync lane (create-only paths, version-checked edits, skip dot names, atomic delivery).

DONE
