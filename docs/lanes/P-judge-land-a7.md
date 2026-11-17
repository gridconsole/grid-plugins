# Lane P-judge-land-a7

This is a research lane, and nothing is built in it. It folds owner Addendum 7
(17 Nov) and the ruling of 17 Nov 02:50 into `docs/research/judge-land.md`.

Addendum 7 has two parts:
- **A, production actions:** the permission mode, approval and the allow rule;
- **B, Job code on hidden refs.**

Ledger rows V26 and V27 are decided here. Every check below ran on the lane head
after `git fetch origin` and `git merge origin/main`. The merge reported
"Already up to date".

## Proof table

| Row | Result | Proof |
|---|---|---|
| Brief Do 1: a section "Addendum 7" decides A and B with file:line evidence | HOLDS | `docs/research/judge-land.md:493-865` |
| Do 1, A: cites the plugin routine path `internal/routine/execute.go:199` and the Job start path | HOLDS | `judge-land.md:513-519` cites `execute.go:192-199` and the Job path `runner.go:95`, `:391-393` on 43d26ad5. The Job path lines were checked with `git grep -n` on 43d26ad5: `:95` holds "Grid writes no default", `:389` holds `--allowedTools` and `:392` holds `--permission-mode` |
| Do 1, A: cites the app ref on `main` | HOLDS | `judge-land.md:520-524` (`app: agentbuild.go:231-236`, `:242-265`, `:281`), `:639-642` (`agentbuild.go:150`, `:154-177`, `:548-598`, `worktree.go:60`, `:106`) |
| Do 1, A: the verified fact (dontAsk plus the exact rule; auto mode is no gate) is recorded and is the basis | HOLDS | `judge-land.md:532-543` (table of the four runs), decision A.1 at `:545-558`, rejected list at the end of A |
| Do 1, A: workflow approval, command Job, `wait` Job, exact allow rule, `blocked: needs permission` | HOLDS | A.2 `:559`, A.3 `:576`, A.4 `:583`, A.5 `:594`, A.6 `:609` |
| Do 1, B: what replaces today's lane branches and land path | HOLDS | B.1 at `judge-land.md:654-663`, which replaces the runner's, the app's and land.sh's branches (`land.sh:25`, `:43`, `:467-469`). Q3 is redone at `:320` |
| Do 1, B: the fetch refspec | HOLDS | B.2 at `judge-land.md:664`: `+refs/grid/*:refs/grid-remotes/origin/*`, added once by `grid setup` and by the launcher |
| Do 1, B: cleanup | HOLDS | B.9 at `judge-land.md:785-810`: after each land, at task close, and a prune on every machine. It also says why the owner's "delete `refs/grid/<T>/*` after landing" is done in two steps (`:795`) |
| Do 1, B: the secrets check before push | HOLDS | B.5 at `judge-land.md:699`: every commit in `base..HEAD` and the messages, patterns from `internal/brain/note.go:199-204`, and no matched text in posts |
| Do 1, B: what the land Job does with `refs/grid/<T>/head` | HOLDS | B.8 at `judge-land.md:751`: rebase the head onto the target, then one atomic push of main and head with two leases; branch and pr rules; bisect over the squash commits |
| Do 2: the decision note at the top is updated | HOLDS | `judge-land.md:3-21` (header and ledger table), decision 4 amended, decisions 7 and 8 at `:73-97`, two Rejected entries added |
| Do 2: the lane plan has new or changed lanes, builders at opus xhigh, each lane names its V-numbers, and acceptance rows come from those rows | HOLDS | `judge-land.md:866-934`: V column on lanes 1-10; new lanes 7 P-code-refs, 8 P-prod-actions and 9 P-deploy-jobs; the proof lane is now 10; the owner's acceptance is mapped for V26 and V27 at `:888-909` |
| Do 3: commit after each section | HOLDS | `git log --oneline origin/main..HEAD` shows 358a2b99 (the Addendum 7 section with the in-place Q3 and Q5 edits), d1abf683 (the decision note and lane plan), and this lane doc |
| Do 3: the last line of the doc is exactly DONE or NOT DONE | HOLDS | `tail -n 1 docs/research/judge-land.md` printed `DONE`, exit 0 |
| Do 3: the doc is copied to the hand folder as R-P-judge-land-a7.md | HOLDS | `cp`, then `diff` of the doc against the copy: exit 0, no output |
| Gate: each owned V-number (V11, V12, V26, V27) is listed at the top with where it is decided | HOLDS | `judge-land.md:11-21` |
| Ruling: a reviewer may answer as the person only under a grant | HOLDS | `judge-land.md:828-847`, the "Answering as the person" item |
| Ruling: brain writes are version-checked, and the person's edits win | HOLDS | the same section, the "Version-checked brain writes" item; lane 3 row 7 |
| Ruling: redo Q3 on these refs | HOLDS | `judge-land.md:320` (the opening of Q3), Q3 steps 2 and 8 edited in place, the Q5 L1 row, and B.7-B.9 |
| Rule: walk test, CLI | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed ok, exit 0 |
| Rule: walk test, lint | HOLDS | `go test -count=1 ./internal/job/lint/` printed ok, exit 0 |
| Rule: fetch and merge main before the last proof | HOLDS | `git fetch origin` exit 0; `git merge origin/main` printed "Already up to date", exit 0 |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log origin/main..HEAD --format=%B` shows one sentence per commit and no trailer |
| Rule: no personal paths, hosts, names or emails in committed files | HOLDS | a grep of both docs for machine paths, user names and mail addresses found nothing, exit 1 |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | No code changed. The doc uses only Claude Code's own dontAsk and allow rules, as the owner asked, and rejects a Grid hook (A, Rejected). A grant that does not cover the rule never stops a start (A.5). The secrets check stops a push after the agent has ended, not the agent (B.5) |
| Rule: no push, and nothing touched outside the clone and the hand folder | HOLDS | No push was run. Files were written only in the clone and as copies in the hand folder |
| Reviewer runbook read and the lane checked against it | HOLDS | The walk tests are pasted above, and the changed files are inside the brief (docs only) |
| Bundle written | HOLDS | `git bundle create .../P-judge-land-a7.bundle origin/main..lane-P-judge-land-a7` exit 0 |
| Codex permission flags checked | not run, not asked | — |
| The real-CLI allow-rule check run again in this lane | not run, not asked | The orchestrator's verified result (17 Nov, CLI 2.1.325) is used as given |
| `go test ./...` across the whole repo | not run, not asked | — |
| `make` checks, GOOS builds | not run, not asked | — |
| Building any of the decided parts | not run, the brief forbids it | "Research only: do not build" |

## What changed

- `docs/research/judge-land.md`:
  - **Top.** The header names Addendum 7, and a new ledger table lists V11, V12,
    V26 and V27. Decision 4 now rebases the task head. Decisions 7 and 8 are new,
    and so are two Rejected entries.
  - **Q3 and Q5.** Q3 is redone on hidden refs: its opening, the fields, and
    steps 2 and 8. Q5's L1 row is updated.
  - **New section "Addendum 7".** It has:
    - A (V26) and B (V27);
    - the 02:50 ruling and the decision gate;
    - the asks to the launcher lanes.
  - **Q6.** It has a V column, rows added to lanes 1-5, new lanes 7-9, the
    proof lane renumbered to 10, the owner's acceptance extended to V26 and V27,
    and new open items.
- `docs/lanes/P-judge-land-a7.md`: new. This file.

## NOT VERIFIED

- **The asks to the launcher lanes.** These are:
  - dontAsk for every Claude Job;
  - a second mechanical assignment, on an approved `blocked`;
  - `permission_denials` on `exit`;
  - the `approved` after kind;
  - detached start and `grid job submit` in the shim.

  None of them is agreed with P-run-jobs-decide or P-run-jobs-a7. The sibling
  doc on `origin/lane-P-run-jobs-decide` does not mention Addendum 7: a
  `git grep` for "Addendum 7", "refs/grid" and "dontAsk" found nothing.
- **Other tools and hosts.** The Codex flags that match dontAsk and an allow rule
  are not checked. Neither is SHA access after ref deletion on GitLab or other
  hosts.
- **The atomic push.** The decided commands were not run in this lane: the
  atomic two-ref push with two `--force-with-lease`, the integrate squash, and
  prune with the `refs/grid-remotes` refspec. They come from the owner's 7 Nov
  design, which was verified on GitHub, and lanes 4 and 7 test them.
- **Line drift.** The app evidence was read from app `main` at the time of
  writing, and its line numbers may drift.
- **Commits not on main.** The `internal/runner` references cite 43d26ad5, and
  the P-land-kind-2 references cite aa0cf14a. Neither commit is on main.
