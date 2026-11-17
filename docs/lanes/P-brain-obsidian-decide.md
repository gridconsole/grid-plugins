# Lane P-brain-obsidian-decide

This is a decision lane, so no code changed. The deliverable is
`docs/research/brain-obsidian.md`. It opens with a decision note (what is
decided, why, and the rejected options), then has one section per question,
ending with the lane plan. Its last line is `DONE`.

Code citations were read at commit `885dc9a6`. `git merge origin/main` was a
no-op ("Already up to date"). Obsidian facts cite the official obsidian-help
source at commit `cbe447c` and obsidian-api `obsidian.d.ts` at commit
`879c936`, which were cloned for reading and are not tracked.

## Proof table

**Brief rows**

| Row | Result | Evidence |
|---|---|---|
| Decision note first: decided, why, rejected | HOLDS | brain-obsidian.md, "## Decision" |
| Q1 Obsidian facts, each with a link: nested properties, rename rewrite, dot-folders, external changes | HOLDS | Section 1 table: Properties.md:77-81; Internal links.md:15-17; Settings.md:222-224; How Obsidian stores data.md:12, :25; Sync settings and selective syncing.md:105; obsidian.d.ts:4436-4456, :2530-2540 |
| Q2 id = path without .md, spaces and Unicode; ValidName only for generated names; every reader change listed (brain, braincmd, hook, settings, app) | HOLDS | Sections 2.1 and 2.2 tables |
| Q3 body `## Links` versus front matter; every [[x]], [[x\|y]], [[x#h]] read; migrate or read both, with the reason | HOLDS | Sections 3.1-3.3. Decision: migrate, and read both only until the migration has run. |
| Q4 any .md is a note; defaults; origin person from the configured name; unknown keys kept byte-for-byte | HOLDS | Sections 4.1-4.4 |
| Q5 adoption on read, index and check; audit_check drift change; hash check, re-read and merge or re-propose, atomic rename | HOLDS | Sections 5.1-5.4 |
| Q6 person notes win; proposals in Proposals/; curator through the workflow, no gate; conflict shown person-first | HOLDS | Sections 6.1-6.3 |
| Q7 layout: config key, .grid/, topics with TOPIC.md, grid/, projects/<p>/, Proposals/, screenshots with a target size, curator and brain-keeper | HOLDS | Sections 7.1-7.3 |
| Q8 migration of knowledge folders, flat notes as person notes, SessionStart topics and counts | HOLDS | Sections 8.1-8.3 |
| Q9 lanes in order with repo, model/effort, files, acceptance (F1-F9), dependencies; xhigh for format and migration; what may build before P-brain-board-decide; Mac round trip | HOLDS | Sections 9.1-9.4 |
| Listing fields for P-brain-view (id, path, kind, origin, level, task, title, summary, tags, aliases, concepts, edges with rel and why, rev, updated) | HOLDS | Section 3.4 |
| Seams for P-brain-board-decide | HOLDS | Section 9.2 |
| Last line of the note is exactly DONE or NOT DONE | HOLDS | `tail -n 1 docs/research/brain-obsidian.md` prints `DONE` |
| Copy to the hand folder as R-brain-obsidian.md | HOLDS | `cp` exit 0 after the last commit of the note |

**Rulings**

| Ruling | Result | Evidence |
|---|---|---|
| 17 Nov 02:12: no brain.lock in the synced brain; sync-server version check; person save beats agent; local lock in the machine folder | HOLDS | Sections 5.4 and 7.2 |
| 17 Nov 02:12: one immutable file per change, not log.md | HOLDS | Section 5.2 |
| 17 Nov 02:12: index and backlinks in the machine folder; what stays in .grid/ | HOLDS | Section 7.2 |
| Addendum 2: Proposals/ visible, person edits are person input | HOLDS | Section 6.2 |
| Addendum 3 left to P-brain-board-decide | HOLDS | Intro and section 9.2 |

**Lane rules**

| Rule | Result | Evidence |
|---|---|---|
| Decision lane: nothing built | HOLDS | `git diff --stat origin/main` shows only docs/research/brain-obsidian.md and this file |
| No guard, gate, cap, hold or blocking hook added or proposed | HOLDS | Sections 6.1 and 4.3: the redirect to a proposal exits 0; the hook only records. The 300 KB size is a target (7.3). |
| No private paths, hosts or emails in committed files | HOLDS | `grep -n -i` for the owner's name, VM paths, mail and home paths over both docs: no output |
| Small commits with one-sentence messages and no attribution lines | HOLDS | `git log origin/main..HEAD` |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0 |
| Lint | HOLDS | `make lint` exit 0 |
| Cross builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Full `go test ./...` | not run, not asked | |

## What changed

- Added docs/research/brain-obsidian.md, the decision note.
- Added docs/lanes/P-brain-obsidian-decide.md, this file.

## NOT VERIFIED

These six Obsidian behaviours are not stated in the official docs. They are
checked in the Mac round trip (note section 9.4), and no decision depends on
them:

1. whether the file explorer hides dot-folders such as `.grid/`;
2. whether automatic link update on rename is on by default;
3. whether a rename rewrites quoted `"[[x]]"` front-matter links;
4. what an open editor does when Grid writes the same file;
5. whether link and file-name matching is case-sensitive on APFS;
6. whether a short-lived dot-file temp file next to a note is ignored.

Other gaps:

- The Mac round trip itself has not been run, because the VM has no Obsidian.
- The plugin reader rows outside internal/brain and internal/hook come from a
  scout map at `885dc9a6`; each line was not re-opened in this lane.
- The app rows were read from the reference app repository at `main`.
