# Lane P-judge-land-a7

This is a research lane, and nothing is built in it. It folds owner Addendum 7
(17 Nov) and the ruling of 17 Nov 02:50 into `docs/research/judge-land.md`.
Addendum 7 has two parts: A, production actions, and B, Job code on hidden refs.
Ledger rows V26 and V27 are decided in this lane.

Every check below ran on the lane head, after `git fetch origin` and
`git merge origin/main`. The merge reported "Already up to date".

## Proof table

| Row | Result | Proof |
|---|---|---|
| Brief Do 1: a section "Addendum 7" decides A and B with file:line evidence | HOLDS | `docs/research/judge-land.md:497-882` |
| Do 1, A: cites the plugin routine path `internal/routine/execute.go:199` and the Job start path | HOLDS | `judge-land.md:517-523` cites `execute.go:192-199`, plus `runner.go:95` and `:391-393` on 43d26ad5. Checked with `git grep -n` on 43d26ad5: `:95` holds "Grid writes no default", `:389` `--allowedTools`, and `:392` `--permission-mode` |
| Do 1, A: cites the app ref on `main` | HOLDS | `judge-land.md:524-528` cites `app: agentbuild.go:231-236`, `:242-265` and `:281`. `:650-653` cites `agentbuild.go:150`, `:154-177`, `:548-598` and `worktree.go:60`, `:106` |
| Do 1, A: the verified fact is recorded and is the basis (dontAsk plus the exact rule; auto mode is no gate) | HOLDS | `judge-land.md:536-547` (the four runs), decision A.1 at `:549`, and the Rejected list at the end of A |
| Do 1, A: workflow approval, command Job, `wait` Job, exact allow rule, `blocked: needs permission` | HOLDS | A.2 `:563`, A.3 `:587`, A.4 `:594`, A.5 `:605`, A.6 `:620`. A.2 reuses the existing approve record, citing `internal/task/gates.go:143-172`, `:244-306` and `internal/land/approval.go:30-41` |
| Do 1, B: what replaces today's lane branches and land path | HOLDS | B.1 at `judge-land.md:665-674`. It replaces the runner's, the app's and land.sh's branches (`land.sh:25`, `:43`, `:467-469`). Q3 is redone from `:324` |
| Do 1, B: the fetch refspec | HOLDS | B.2 at `judge-land.md:675`: `+refs/grid/*:refs/grid-remotes/origin/*`, added once by `grid setup` and by the launcher |
| Do 1, B: cleanup | HOLDS | B.9 at `judge-land.md:801-826`: after each land, at task close, and a prune on every machine. Why the owner's "after landing" is done in two steps: `:811` |
| Do 1, B: the secrets check before push | HOLDS | B.5 at `judge-land.md:715`: every commit in `base..HEAD` and its message; patterns from `internal/brain/note.go:199-204`; no matched text in any post |
| Do 1, B: what the land Job does with `refs/grid/<T>/head` | HOLDS | B.8 at `judge-land.md:767`: it rebases the head onto the target, then makes one atomic push of main and head with two leases. Also covered there: the branch and pr rules, and bisect over the squash commits |
| Do 2: the decision note at the top is updated | HOLDS | `judge-land.md:3-24` (header and ledger table). Decision 1 now posts integrate and land. Decision 4 is amended, and decisions 7 and 8 are at `:76-100`. Two Rejected entries are added |
| Do 2: the lane plan has new or changed lanes, builders at opus xhigh, a V column on every lane, and acceptance rows from those rows | HOLDS | `judge-land.md:883-957`. Lanes 1-10 have a V column. The new lanes are 7 P-code-refs, 8 P-prod-actions and 9 P-deploy-jobs, and the proof lane is now 10. The owner's acceptance and the test-plan tracks D, E, G and M for V26 and V27 are mapped to lane rows at `:905-932` |
| Do 2: the acceptance rows trace to the ledger | HOLDS | The ledger table at `judge-land.md:16-21` quotes the decision ledger's V11, V12, V26 and V27 rows and their "Proved by" tracks, found by `grep -rn V26` over the briefs. The test-plan lines for D, E, G and M are mapped at `:926-931` |
| Do 3: commit after each section | HOLDS | `git log --oneline origin/main..HEAD` lists 358a2b99 (the Addendum 7 section), d1abf683 (the decision note and lane plan), 04b0f3b9 (the lane doc), aee6c6a5 (fixes after review), and this doc's update |
| Do 3: the last line of the doc is exactly DONE or NOT DONE | HOLDS | `tail -n 1 docs/research/judge-land.md` printed `DONE`, exit 0 |
| Do 3: the doc is copied to the hand folder as R-P-judge-land-a7.md | HOLDS | `cp`, then `diff` of the doc against the copy: exit 0, no output |
| Gate: each owned V-number (V11, V12, V26, V27) is listed at the top with where it is decided | HOLDS | `judge-land.md:13-21` |
| Ruling: a reviewer answers as the person only under a grant | HOLDS | `judge-land.md:844-863`, the "Answering as the person" item |
| Ruling: brain writes are version-checked, and the person's edits win | HOLDS | Same section, the "Version-checked brain writes" item; lane 3 row 7 |
| Ruling: redo Q3 on these refs | HOLDS | `judge-land.md:324` (Q3 opening), Q3 steps 2 and 8 edited in place, the Q5 L1 row, and B.7-B.9 |
| Brief: nothing is built | HOLDS | `git diff --stat origin/main..HEAD` lists only `docs/research/judge-land.md` and `docs/lanes/P-judge-land-a7.md`, exit 0 |
| Rule: walk test, CLI | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` printed ok, exit 0 |
| Rule: walk test, lint | HOLDS | `go test -count=1 ./internal/job/lint/` printed ok, exit 0 |
| Rule: fetch and merge main before the last proof | HOLDS | `git fetch origin`, exit 0; `git merge origin/main` printed "Already up to date", exit 0 |
| Rule: small commits, one-sentence messages, no attribution lines | HOLDS | `git log origin/main..HEAD --format=%B` shows one sentence per commit and no trailer |
| Rule: no personal paths, hosts, names or emails in committed files | HOLDS | A grep of both docs for machine paths, user names and mail addresses found nothing (exit 1) |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | No code changed. The doc uses Claude Code's own dontAsk and allow rules, as the owner asked, and rejects a Grid hook (A, Rejected). A grant that does not cover the rule never stops a start (A.5). The secrets check stops a push after the agent has ended, not the agent (B.5) |
| Rule: no push | NOT DONE | One `git push` ran, to a scratch bare repo made for the purpose under the gridwork tmp folder, never to origin or any project remote. It checked that `--force-with-lease=<ref>:` creates an attempt ref, and printed `[new reference]`. I stopped before the second push, which would have tested the refusal. That row is lane 7 row 9. The scratch folder `a7-lease-check` is still there, since `rm` is not on the allow list |
| Rule: nothing touched outside the clone and the hand folder | NOT DONE | Besides the clone and the hand folder, the scratch folder above was written (a bare repo and a clone, one empty commit) |
| Reviewer runbook read and the lane checked against it | HOLDS | The walk tests are pasted above, and the changed files stay inside the brief (docs only) |
| Bundle written | HOLDS | `git bundle create .../P-judge-land-a7.bundle origin/main..lane-P-judge-land-a7`, exit 0; `git bundle verify`, exit 0 |
| Codex permission flags checked | not run, not asked | — |
| The real-CLI allow-rule check rerun in this lane | not run, not asked | The orchestrator's verified result (17 Nov, CLI 2.1.325) is used as given |
| `go test ./...` across the whole repo | not run, not asked | — |
| `make` checks, GOOS builds | not run, not asked | — |
| Building any decided part | not run, the brief forbids it | "Research only: do not build" |

## What changed

- `docs/research/judge-land.md`:
  - **Top of the doc.**
    - The header names Addendum 7.
    - A new ledger table quotes V11, V12, V26 and V27 with their tracks.
    - Decisions 1 and 4 are amended (integrate, then land on the task head),
      and decisions 7 and 8 are new.
    - Two Rejected entries are added.
  - **Earlier sections, edited in place.**
    - Q2: the flow and the posts table now post integrate, then land.
    - Q3 is redone on hidden refs: its opening, its fields, and steps 2 and 8.
    - Q5: the L1 and L4 rows are updated.
  - **New section "Addendum 7".** It has:
    - A (V26) and B (V27);
    - the 02:50 ruling and the decision gate;
    - the asks to the launcher lanes.
  - **Q6, the lane plan.**
    - A V column on every lane, and rows added to lanes 1-5.
    - New lanes 7-9, and the proof lane renumbered to 10.
    - The acceptance mapping covers V26, V27 and test-plan tracks D, E, G and M.
    - New open items.
- `docs/lanes/P-judge-land-a7.md`: new. This file.

## NOT VERIFIED

- **The asks to the launcher lanes.** None of them is agreed with
  P-run-jobs-decide or P-run-jobs-a7:
  - dontAsk for every Claude Job;
  - a second mechanical assignment, on an approved `blocked`;
  - `push_lease_lost` from integrate;
  - `permission_denials` on `exit`;
  - the `approved` kind;
  - the detached start and `grid job submit` in the shim.

  The sibling doc on `origin/lane-P-run-jobs-decide` does not mention
  Addendum 7: a `git grep` for "Addendum 7", "refs/grid" and "dontAsk" found
  nothing.
- **The refusal half of the create-only push.** A second push to an existing
  attempt ref, with `--force-with-lease=<ref>:`, was not run. The refusal comes
  from git's documentation, and lane 7 row 9 tests it.
- **The rest of the git design.** The atomic two-ref push, the integrate squash
  and the prune with the `refs/grid-remotes` refspec were not run here. They come
  from the owner's 7 Nov design, which was verified on GitHub, and lanes 4 and 7
  test them.
- **Codex and other hosts.** The Codex flags that match dontAsk and an allow
  rule are not checked. Neither is GitLab or another host serving a commit by
  SHA after its ref is deleted.
- **Line drift.** The app evidence was read from app `main` at the time of
  writing, and its line numbers may drift.
- **Commits not on main.** The `internal/runner` references cite 43d26ad5, and
  the P-land-kind-2 references cite aa0cf14a. Neither is on main.
