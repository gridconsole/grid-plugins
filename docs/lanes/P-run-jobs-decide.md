# P-run-jobs-decide: proof table

Decision and research lane for the launcher (`grid loop`). The deliverable is `docs/research/run-jobs-launcher.md`. No code changed. Base: lane-P-run-jobs at 43d26ad5, with origin/main merged before the proof run. `<hand>` below is the build's hand-off folder.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| Deliverable written and committed on the lane | HOLDS | `docs/research/run-jobs-launcher.md` (461 lines), commits 45398334 and 9f907a4e |
| Decision note at the top: what was decided, why, rejected options | HOLDS | `docs/research/run-jobs-launcher.md:7-76` |
| Q1: what of 43d26ad5 survives (stream.go, lease, fold, mcp); reads Jobs and posts started/exit; attempts instead of lease fencing, older-attempt posts ignored | HOLDS | `docs/research/run-jobs-launcher.md:78-139` |
| Q2: start conditions (now, cores free, no test run, after J-4 lands, combinations), timeout with workflow default, time conditions replace routine timers, no cap/priority/learning | HOLDS | `docs/research/run-jobs-launcher.md:141-189` |
| Q3: measurements and sources on Linux and macOS, cost line on exit, machine note single writer about every minute, account notes without secrets, only the brain syncs | HOLDS | `docs/research/run-jobs-launcher.md:191-264` |
| Q4: waking (when, how, only the home launcher), fixed task home, `grid task move` as a person post | HOLDS | `docs/research/run-jobs-launcher.md:266-288` |
| Q5: lifecycle (singleton lock, SessionStart detached start, login services, VM one per login, restart resumes, `grid sync` beside it) | HOLDS | `docs/research/run-jobs-launcher.md:290-320` |
| Q6: remote launch with `grid run J-n --on`, sync-server push, ssh fallback, starting a stopped launcher, grid-worker out of the run path | HOLDS | `docs/research/run-jobs-launcher.md:322-339` |
| Q7: interfaces with P-brain-board-decide and P-brain-obsidian-decide, small post read/write interface | HOLDS | `docs/research/run-jobs-launcher.md:341-382` |
| New Q: status block (single writer, hash check, no LLM), the inbox, posting as a person under a grant | HOLDS | `docs/research/run-jobs-launcher.md:384-403` |
| Interface given to P-judge-land-decide: wake on exit, start conditions, mechanical push retry; reconciled with that lane's output | HOLDS | `docs/research/run-jobs-launcher.md:405-419` |
| Lane plan: lanes in order with repo, model/effort (xhigh for launcher, conditions, remote), files, acceptance rows, dependencies | HOLDS | `docs/research/run-jobs-launcher.md:421-439` |
| Owner's acceptance mapped onto the lanes (no-app delivery, research "now", build waits on "no test run", kill orchestrator, kill launcher, Mac starts J-n as grid2 and grid3) | HOLDS | `docs/research/run-jobs-launcher.md:441-450` |
| Last line exactly DONE or NOT DONE | HOLDS | `tail -n 1 docs/research/run-jobs-launcher.md` printed `DONE` |
| Copy at `<hand>/R-run-jobs-launcher.md` | HOLDS | `cp`, recopied after the last edit of the doc, no error |
| Launcher not built in this lane | HOLDS | `git diff --stat 43d26ad5 45398334` lists only `docs/research/run-jobs-launcher.md`; the later lane commits touch only that file and this one |
| Code facts cite path:line; unread facts marked UNVERIFIED | HOLDS | citations throughout; UNVERIFIED in Q1 (lease, mcp), Q5 (linger) and the open items |
| Rule: no sudo, never root | HOLDS | no such command run in this lane |
| Rule: one command per Bash call, allow-listed tools, files written with Edit/Write | HOLDS, one slip | one call used `;` and `echo` for an exit code; it was denied and never ran, and the commands were rerun one per call |
| Rule: work only in this clone and the hand folder | HOLDS | writes went to this clone and to three files in `<hand>`: the research copy, the bundle and the report |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log origin/main..HEAD`: one-sentence messages, no trailers |
| Rule: committed files carry no private paths, hosts or emails | HOLDS | a grep over both docs for private path prefixes, the build folder name, the host name and email domains printed nothing |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | the design adds none. Grants are a read-side fold rule (Q8). The counts and defaults are workflow keys, not code: `start-wait` (Q2), `wake-unanswered` (Q4). The `expect` correction ends with the next sample (Q2) |
| Rule: never push; never touch other lanes' clones or the paths the runbook forbids; hand scripts read only | HOLDS | no push. The base lane's uncommitted test file and the sibling hand outputs were read, not changed or copied |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin`, then `git merge --no-edit origin/main` with no conflict (4 docs files added), commit 82d1cde9 |
| Walk test: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok internal/cli 0.382s`, exit 0, after the merge; later commits change only docs |
| Walk test: `go test -count=1 ./internal/job/lint/` | HOLDS | `ok internal/job/lint 0.536s`, exit 0, after the merge |
| Lint: `make lint` | HOLDS | `go vet ./...` printed nothing, exit 0 |
| Windows build: `GOOS=windows go build ./...` | HOLDS | exit 0 |
| macOS build: `GOOS=darwin go build ./...` | HOLDS | exit 0 |
| Full package tests `go test ./...` | not run, not asked | no code changed |
| Bundle at `<hand>/P-run-jobs-decide.bundle` and `git bundle verify` | see the report | created after this file is committed, so its result is in the hand-off report |
| Lane doc copied to `<hand>/P-run-jobs-decide-report.md` | see the report | the copy is made after the bundle |

## What changed
- `docs/research/run-jobs-launcher.md`:
  - the decision note and Q1-Q7;
  - the ruling's status block, inbox and grant question;
  - the interface for P-judge-land-decide, reconciled with its output: the land verb pushes with a lease, and the launcher's only written assignment is a new attempt on `push_lease_lost`;
  - the lane plan of 13 lanes, with the owner's acceptance mapped;
  - the open items.
- `docs/lanes/P-run-jobs-decide.md`: this proof table.
- Merge of origin/main (four docs files from other lanes).

## NOT VERIFIED
- Whether a VM login can enable its own systemd linger without a person (Q5).
- `internal/lease/activity.go` and the full MCP tool list were not read. The claim that no MCP code reads `GRID_FENCE` rests on one grep.
- P-judge-land-decide's output was read for its interface lines only (decided list, order per repo, push steps, lane table). R-brain-obsidian.md arrived late and was not read. P-brain-board-decide's output was not in the hand folder.
- The macOS and Windows measurement commands in Q3 were not run on those systems.

DONE
