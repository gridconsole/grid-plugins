# C-cg-planguard-job-named-path: plan-guard does not warn a reviewer or checker about a file its own Job names

Reason: plugin check 2026-11-12T1247Z issue 1. The 12:15 end-review reviewer was warned five times for writing `end-reviews/2026-11-12T1215Z.md`, the one verdict file its Job allowed. The reviewer and checker fence (`internal/guard/plan.go`) fired for any write outside the projects root and never read the Job.

Base: main fe91d7f2 (`git rev-parse origin/main`; `git fetch origin && git merge origin/main` printed "Already up to date" before the last proof run). Test commit 714b6ea1, fix commit 9326858e; this report is committed on top.

## Proof table

| Row | Result |
| --- | --- |
| 1. A grid-reviewer whose transcript's first user message names the verdict file writes it: no warning, no deny, at default and at block | HOLDS. `TestPlanGuardJobNamedPath/{grid-reviewer,grid-checker}/named/{default,block}` (`internal/guard/planguard_job_path_test.go`): `Behavior ""`, no warnings, no reason. Passes on the lane; fails on main (below). `go test ./internal/guard/ -count=1 -v -run 'TestPlanGuardJobNamedPath\|TestPlanGuardFencesWarnByDefaultAndDenyAtBlock'`: every test `--- PASS`, `ok`. |
| 2. The same reviewer writes a file the Job does not name: the warning by default and the deny at block, unchanged | HOLDS. `TestPlanGuardJobNamedPath/*/not named/{default,block}` assert the exact text of `TestPlanGuardFencesWarnByDefaultAndDenyAtBlock` (`plan-guard: ` + the fence text by default, the same text as the deny at block, no warning there). `TestPlanGuardFencesWarnByDefaultAndDenyAtBlock` itself is not edited and passes. These rows pass on main too (nothing changes for an unnamed file); they pin the unchanged answer. |
| 3. Transcript missing or unreadable: behaviour as on main | HOLDS. Subtests `no transcript path`, `transcript missing`, `transcript is a folder`, `transcript is not JSON`: the warning by default, the deny at block. Also: the Job named after the 64 KiB limit, the Job named only in a later message, and a subagent whose own transcript is missing while the session's first message names the file. Pass on main as well (that is "as on main"). |
| 4. `go test ./internal/guard/... ./internal/hook/... ./internal/golden/... -count=1` ok; `GOOS=windows go vet ./internal/guard/` exit 0 | HOLDS. Tests: `ok internal/guard`, `ok internal/hook`, `ok internal/golden`. `GOOS=windows go vet ./internal/guard/`: exit 0. |
| 5. `make lint` exit 0; `make cross` exit 0 | HOLDS. `make lint` (`go vet ./...`): exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile of every package): exit 0. |
| Packages importing the changed one | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... \| grep internal/guard`: cmd/grid, internal/cli, internal/guard, internal/hook, internal/packaging, internal/packaging/cmd, lib. `go test -count=1 ./internal/guard/... ./internal/hook/... ./internal/golden/... ./cmd/grid ./internal/cli ./internal/packaging/... ./lib`: all `ok` (cmd/grid has no test files). The whole-module run is the orchestrator's. |
| Walk tests (plugin) | HOLDS, run on the merged head (already up to date with origin/main fe91d7f2). `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v`: both `--- PASS`, `ok`. `go test -count=1 ./internal/job/lint/`: `ok`. |
| Guest walk tables (`make realplugin ...`) | Not applicable: this is the plugin repo; that table is an app-repo check. |
| Existing tests not edited; nothing deleted | HOLDS. `git diff fe91d7f2 --stat` shows `internal/guard/plan.go` changed and three files added (`job_path.go`, `planguard_job_path_test.go`, this report). No test file other than the new one is touched; no test is deleted. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. The change removes a warning/deny in one case; nothing new warns or blocks. The warn-by-default level (C-need-planguard-warn) and the off hint (C-kr3b) are untouched. |
| New tests fail on base, pass on lane | HOLDS. Pasted below. |
| Public repo: no personal paths, hosts or emails | HOLDS. Files changed hold only repo-relative paths and `t.TempDir()` paths. |
| Review: yes (security boundary) | See "Security notes". |

## New tests on main (test commit 714b6ea1 on top of fe91d7f2, code unchanged)

`go test ./internal/guard/ -count=1 -run TestPlanGuardJobNamedPath` fails; first lines:

```
--- FAIL: TestPlanGuardJobNamedPath/grid-reviewer/named/default
    planguard_job_path_test.go:60: named at default: want no warning and no deny, got {... Warnings:[plan-guard: A grid-reviewer writes only its Verdict inside a task folder; <tmp>/repo/verdict.md is outside it. To turn this guard off, run grid hook level plan-guard off (or start Claude Code with GRID_PLAN_GUARD=off).] ...}
--- FAIL: TestPlanGuardJobNamedPath/grid-reviewer/named/block
    planguard_job_path_test.go:60: named at block: want no warning and no deny, got {... Behavior:deny Reason:A grid-reviewer writes only its Verdict inside a task folder; <tmp>/repo/verdict.md is outside it. ...}
--- FAIL: TestPlanGuardJobNamedPath/grid-checker/named/default   (same, grid-checker)
--- FAIL: TestPlanGuardJobNamedPath/grid-checker/named/block
--- FAIL: TestPlanGuardJobNamedPathReadsFirstMessageOnly/named_in_content_blocks/{default,block}
--- FAIL: TestPlanGuardJobNamedPathReadsFirstMessageOnly/named_after_a_row_that_is_no_message/{default,block}
--- FAIL: TestPlanGuardJobNamedPathMatchesWholePaths/{absolute,_sentence_end | absolute,_in_parentheses | relative_call,_absolute_Job | relative_as_given}
--- FAIL: TestPlanGuardJobNamedPathSubagent/{subagent_transcript_beside_the_session_names_it | hook_names_the_agent_transcript | transcript_path_is_the_subagent's_own}/{default,block}
--- FAIL: TestPlanGuardJobNamedPathKeepsOtherFences
    planguard_job_path_test.go:291: want the brain warning "plan-guard: Only a grid-curator ... writes the brain; a grid-reviewer ...", got {... Warnings:[plan-guard: A grid-reviewer writes only its Verdict inside a task folder; <tmp>/home/.grid/knowledge/note.md is outside it. ...]}
FAIL	github.com/gridconsole/grid-plugins/internal/guard
```

On the lane the same command prints `ok` (all five `TestPlanGuardJobNamedPath*` tests `--- PASS`).

## What changed

- `internal/guard/job_path.go` (new): `context.jobNames(given, target)` reads the caller's Job and reports whether it spells the target; `jobTranscript` picks the file; `jobText` reads it; `namesPath` matches a whole path.
- `internal/guard/plan.go`: the reviewer/checker condition gets `&& !c.jobNames(given, target)` as its last term, so the transcript is read only where the fence would fire. There is no early return: a write the Job names falls through to the later fences (a brain file is still a brain file).
- `internal/guard/planguard_job_path_test.go` (new): the tests above.

How the Job is found and read:
- Which transcript: `agent_transcript_path` if the hook gives it; else, with an `agent_id`, the subagent's own file (`transcript_path` itself when its base is already `agent-<id>.jsonl`, otherwise `<transcript_path minus .jsonl>/subagents/agent-<id>.jsonl`; an id with a separator or NUL gives no file); with no `agent_id`, `transcript_path`. A subagent never falls back to the session's file, whose first message is a person's prompt, not its Job.
- How: `Lstat` must show a regular file (a link or folder fails open), the path must be absolute, under 1000 bytes, with no NUL/CR/LF. At most 64 KiB are read from the start; a row cut at the limit is dropped. Rows are JSON-decoded; the first row whose role (`message.role`, else `type`) is `user` and that is not `isMeta` is the Job; its `content` is a string or the joined `text` blocks. Any error, a non-JSON row before it, or no such row gives "" and the fence answers as on main.
- Match: the target as the call gave it, as the absolute path it names (cwd + `~` expanded, not symlink-resolved), and as the real path (`realNative`), each as written and slash-converted. It is named when the text holds it as a whole path: the byte before is not a path character (`A-Za-z0-9_-./\~`) and the byte after is not one (a `.` ends it unless a file-name character follows). So `v.md` is not named by `/a/b/v.md`, `/x/v.md` is not named by `zz/x/v.md`, `v.md.bak` or `v.mdx`; `v.md.` at a sentence end, `(v.md)`, backticks and `v.md:12` are named.

## Security notes (for the reviewer)

- This only silences a warning/deny; it grants nothing. The Job text comes from whoever wrote the Job (the orchestrator, or a parent agent's prompt); a path that Job names is the authority, as the brief asks. The later fences (brain, curator, planner) still run for the same write.
- Match tightening beyond "appears literally": a plain substring would let a relative `notes.md` be named by `/other/notes.md`, or `/x/v.md` by `/x/v.md.bak`. The whole-path rule above prevents that; it matches the literal spelling the Job uses in every case the brief names (absolute path, path as given).
- A bare relative name as given (`verdict.md`) counts when the Job spells `verdict.md` on its own; it is then resolved against the call's cwd, so the Job and the call mean the same word. A Job that names only the absolute path also covers the relative call (the resolved absolute form is checked).
- The transcript is read only for a reviewer or checker write outside the projects root, so no other call pays for the read.
- The reviewer could edit its own transcript only by a write the fence itself covers (the transcript lives outside the projects root and the Job does not name it).

## NOT VERIFIED

- Which transcript Claude Code names in `transcript_path` for a subagent's PreToolUse, and that it stores subagent transcripts as `<session>/subagents/agent-<agent_id>.jsonl`. The brief cites `subagents/agent-a035cbf7a13c2dce6.jsonl` and I could not read it (the sandbox allows only /srv/gridwork). The code is safe either way: it takes `agent_transcript_path`, then the sibling `subagents/agent-<id>.jsonl`, or `transcript_path` when that already is `agent-<id>.jsonl`; when none reads, the fence warns as on main and never reads the session's first message. If a live hook names the subagent's file some other way, a reviewer will keep being warned until the picker learns it.
- The line shape of a live subagent transcript (`type`/`message.role`/`message.content` string or text blocks, as in the main transcripts the repo's tests use); the reader accepts `message.role` or `type` equal to `user`, string or text-block content.
- That Claude Code stops showing the warning to a live reviewer after landing.

## Open items

- None blocking. Verdict: DONE.
