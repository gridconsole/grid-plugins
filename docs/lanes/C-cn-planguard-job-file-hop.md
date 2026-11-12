# C-cn-planguard-job-file-hop: plan-guard follows the one brief file a Job names

Repo: plugin. Branch lane-C-cn-planguard-job-file-hop, from main 54c163f9 (`git fetch origin && git merge origin/main`: Already up to date).

## Proof table

| Row | Result |
|---|---|
| A1: a reviewer or checker whose Job names a `.md` file that names the target gets no plan-guard answer for an Edit of the target | HOLDS. `go test ./internal/guard/ -run 'Hop' -count=1 -v` exits 0: `--- PASS: TestHopNamedFileNamesTarget`, `--- PASS: TestHopKeepsOtherFences`, `--- PASS: TestHopNamedFileNamesFolder`, `--- PASS: TestHopOneHopAndUnreadableFiles`, then `PASS` / `ok`. The cases run for the reviewer and the checker at the default level (warning) and at block (deny). |
| A1 on main: the same case fails | HOLDS. With `internal/guard/job_path.go` put back to main (`git stash push -- internal/guard/job_path.go`, test file committed), the run fails 16 subtests of TestHopNamedFileNamesTarget, TestHopKeepsOtherFences, 16 of TestHopNamedFileNamesFolder and 16 of TestHopOneHopAndUnreadableFiles, for example: `planguard_job_path_test.go:60: grid-reviewer/brief names the target at default: want no warning and no deny, got {... Warnings:[plan-guard: A grid-reviewer writes only its Verdict inside a task folder; <tmp>/repo/docs/plan.md is outside it. To turn this guard off, run grid hook level plan-guard off (or start Claude Code with GRID_PLAN_GUARD=off).] ...}` and `planguard_job_hop_test.go:138: want the brain warning ... got ... Warnings:[plan-guard: A grid-reviewer writes only its Verdict inside a task folder; <tmp>/home/.grid/knowledge/note.md is outside it. ...]`. The cases that must still answer pass on main, as they should. The stash was popped; the lane commit holds the change. |
| A2: the file names the target's folder with a trailing slash, the target sits directly in it: no answer; a target in a subfolder still gets the answer | HOLDS. TestHopNamedFileNamesFolder: "folder with a slash and the target directly in it", "... at the end of a line", "... in a list", "a subfolder and the folder both named" are silent; "folder without the slash", "parent folder with a slash and the target in a subfolder of it", "only a subfolder named", "a folder with a longer name", "target in a subfolder" and "a lone slash names no folder" (a file in the root) still answer. |
| A3: one hop only; a missing file, a symlink, a non-regular file or one of 64 KiB or more adds nothing | HOLDS. TestHopOneHopAndUnreadableFiles: "second file names the target" (A names B, B names the target) still answers; each negative sits beside its readable twin: "file missing" / "file there", "file is a symlink" / "file is the file a symlink points at", "file is a folder" / "file there", "file one byte under 64 KiB" (silent) / "file of 64 KiB", "file one byte over 64 KiB", "file far over 64 KiB" (answer). |
| Rule 4: `go test ./internal/guard/ -count=1` passes, planguard_job_path_test.go and planguard_warn_test.go unchanged | HOLDS. `ok github.com/gridconsole/grid-plugins/internal/guard 0.658s`, exit 0. `git diff --stat origin/main..HEAD` lists only internal/guard/job_path.go and internal/guard/planguard_job_hop_test.go (plus this doc). The existing "Job names a folder" subtest in planguard_job_path_test.go still passes: the folder rule reads the brief file's text only. |
| Rule 5: `make lint` passes | HOLDS. `make lint && echo` printed `go vet ./...` then `lint exit 0` (gofmt -l clean). |
| Rule 6: `make cross` passes | HOLDS. `make cross && echo` printed the darwin/arm64 and windows/amd64 vet and test-compile lists, all `ok`, then `cross exit 0`. |
| Rule: tests of the changed package and of every package that imports it | HOLDS. Importers by `go list` (Imports, TestImports, XTestImports of internal/guard and internal/hook): internal/cli, internal/golden, internal/guard, internal/hook, internal/orchestrator, internal/packaging. `go test -count=1` on all six exit 0: guard 1.028s, hook 10.339s, cli 55.103s, golden 0.231s, orchestrator 168.741s, packaging 2.155s. |
| Rule: walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'`: `ok ... internal/cli 0.284s`. `go test -count=1 ./internal/job/lint/`: `ok ... internal/job/lint 0.360s`. Both after `git fetch origin && git merge origin/main` (Already up to date). |
| Rule: new tests fail on base and pass on the lane | HOLDS, see the A1-on-main row. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS. The change makes the warn-level reviewer and checker fence answer less often; it adds no answer, no deny and no cap on an agent. |
| Rule: RealPlugin tests, guest walk tables | not run, the brief names no app-repo check; this is the plugin repo. |
| Rule: personal paths, hosts, emails in the public repo | HOLDS. The tests use t.TempDir paths only. |

## What changed

- `internal/guard/job_path.go`: `jobNames` still matches the target (as given, as the absolute path, as the real path) against the first user message. If that does not match, `hopText` reads the brief files the Job names and `jobNames` matches the same spellings against that text. New helpers: `namesAny` (the old loop), `hopText`, `hopPaths`, `hopRead`.
- A named file is an absolute path (`filepath.IsAbs`) in the Job text that ends in `.md` (a full stop that ends a sentence is trimmed; `zz/a/b.md`, `a.md.bak` and `a.mdx` name nothing), is a regular file by `Lstat` (a symlink is not), and is under 64 KiB (`jobTextLimit`; a file of exactly 64 KiB is not read). Its text is read once through a `LimitReader`. Paths inside it are not followed. A file that cannot be read adds nothing, and the answer is then the one without the hop.
- A folder the brief file names with a trailing slash (`<dir>/`, matched as a whole path by `namesPath`) covers a target directly in `<dir>`. A target in a subfolder, a folder named without the slash, a longer folder name and a bare root `/` do not match. This folder rule reads the brief file's text only: the Job's own text naming a folder still answers as before (existing subtest "Job names a folder").
- `internal/guard/planguard_job_hop_test.go`: four tests, helpers prefixed `hop`.

Reading of the brief worth the reviewer's eye:

1. The brief says "append that file's text once to the text jobNames matches against". Appended into one string, the folder rule would also read the Job's own text and break the existing "Job names a folder" subtest (row 4 forbids changing that file). So the Job text and the file text are matched as two strings: the exact-path match reads both, the folder match reads the file text only.
2. "The Job text names an absolute path ending in `.md` that is a regular file": every distinct such path is read, in the order the Job names them, each once. A Job that lists a verdict file that does not exist yet before the brief still reaches the brief. There is no count cap; the Job text is at most 64 KiB.
3. "Under 64 KiB" is read as strictly under: a file of exactly 65536 bytes adds nothing (it matches `jobText`, where a read that reaches the limit is a cut row).
4. Paths are found as runs of the characters `namesPath` already treats as path characters (letters, digits, `_-./\~`); a path with a space or a comma in it is not found. A Windows `C:\dir\brief.md` is found; `~/brief.md` and relative paths are not (not absolute).

Residual risk, not guarded by this lane: a reviewer can write inside a task folder, so it can edit the brief the Job names before it edits a target. The fence is warn level and the brief says the file is the Job; no guard was added.

## NOT VERIFIED on a live screen

Nothing here is a screen. NOT VERIFIED: a live Fable folder review against the real hook (the check 2026-11-12T1449Z incident); the tests drive `EvaluatePlan` with a written transcript, the same way the existing job-path tests do.
