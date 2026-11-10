# Public audit

Owner decisions: everything in plugins/ is public (no personal paths, client data or secrets) and evals stay internal. This audit covers `evals`, `corpus`, `recordings`, `fixtures`, `golden`, `docs` and `agent-templates` at the lane head. No secret was printed; a secret would be listed here by file:line only.

## Patterns used

Run from the repo root, with `DIRS="evals corpus recordings fixtures golden docs agent-templates"`.

In the table, `\|` stands for a plain `|` (the table needs it escaped).

| Kind | Command |
|---|---|
| Home and user paths | `grep -rInE "/(home\|Users)/[A-Za-z0-9_.-]+\|C:\\\\Users\|~/\.(claude\|ssh\|aws)" $DIRS` |
| Machine temp paths | `grep -rInoE "(/private/var/f[o]lders\|/c/Users\|C:/Users)[^ ]{0,20}" $DIRS` |
| Email addresses | `grep -rIlE "[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+\.[A-Za-z]{2,}" $DIRS` |
| Tokens and keys | `grep -rIlE "(sk-[A-Za-z0-9_-]{16,}\|ghp_[A-Za-z0-9]{20,}\|github_pat_\|xox[bp]-\|AKIA[0-9A-Z]{16}\|BEGIN [A-Z ]*PRIVATE KEY\|Bearer [A-Za-z0-9._-]{20,}\|(token\|secret\|password\|api_?key)[\"']?[ ]*[:=][ ]*[\"'][A-Za-z0-9_+/=-]{12,})" $DIRS` |
| Personal names, hosts, org | `grep -rInoiE "$OWNER_FIRST_NAME\|$VM_USER\|\bgmf\b" $DIRS` (the owner's first name and the VM user name, set in the shell, not written here), `grep -rInoE "/srv/[a-z]+[^ ]{0,30}" $DIRS`, `grep -rInoiE "\b(gmail\|hotmail\|outlook\|yahoo\|icloud)\b\|gridconsole" $DIRS` |
| IP addresses | `grep -rInoE "\b[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\b" $DIRS` |
| Client names | `grep -rIniE "client[ _-]?name\|acme\|customer" evals corpus fixtures golden recordings agent-templates` |

## Hits and fixes

| # | file:line | Hit | Verdict | Fix |
|---|---|---|---|---|
| 1 | docs/lanes/L160.md:6, :23 | a personal first name inside a quoted test line (`by <name>`) | personal name | FIXED: now `by person` |
| 2 | docs/lanes/L135R.md:73,75,81,85,100-103 | a macOS per-user temp folder path with a hash in it | machine path | FIXED: now `<tmp>/` |
| 3 | docs/lanes/S6b.md:17, docs/lanes/PLUG-shell.md:26 | a path to a brief file on the build VM | host path | FIXED: now "the simplify brief" |
| 4 | docs/lanes/FP1.md:405 | token pattern matched `task-new-refuses-links` (the `sk-` inside "task-") | false positive | none needed; no secret |
| 5 | docs/lanes/L37.md:45 | `/c/Users/...` in prose about redaction forms | generic example, no user name | none needed |
| 6 | docs/lanes/D-agents-sync.md:10, D-accounts-plugin.md:15-17, L62.md:15,79, agent-templates/grid-builder.md:40, grid-runner.md:24, grid-planner.md:53 | `~/.claude`, `~/.codex`, `~/.ssh` | generic home-relative paths, no user name | none needed |
| 7 | docs/lanes/*.md (`<tmp>/home/.grid`, `.../home/.grid/projects/...`) | the test harness's fixture home | placeholder already | none needed |
| 8 | docs/lanes/L30.md:27,153,156 | `127.0.0.1` | loopback | none needed |
| 9 | golden/**/env.txt (`GRID_HOST=example.com`) | reserved example domain | placeholder already | none needed |
| 10 | recordings/cases.py:43, golden/grid-env/discovered/setup.json:11, docs/old-plugin-map.md:1, many docs/lanes/*.md | `gridconsole` (the app id `app.gridconsole.desktop`, the module path, the old repo name) | product and org name, the module path in go.mod; public by design | none needed |

Email addresses: no hit in any of the seven folders. Tokens and keys: no true hit. Client names: no hit. Home paths in `evals`, `corpus`, `recordings`, `fixtures`, `golden` and `agent-templates`: no hit, so there was no test data to replace with placeholders (row 2 of the brief).

## NOT DONE

| Item | Why |
|---|---|
| Physically moving `evals/`, `corpus/`, `recordings/`, `fixtures/` and `golden/` out of the source repo | Go tests in `internal/eval`, `internal/evalcmd` and `internal/golden` read `../../evals/tasks`, `golden/` and `fixtures/` by relative path, and `make eval-check` and `make golden` run them. A move touches those packages, which this lane may not edit, and a repo split is the owner's call. The publish set is closed instead: `dist/plugin` copies only `skills`, `agents`, `agent-templates` (`shippedDirs` in `internal/packaging/dist.go`), and `internal/packaging/publish_set_test.go` checks the staged tree against a deny list of folders. |
| `docs/lanes/` is still in the source repo | These are build reports. They are not in `dist/plugin`, and after the fixes above hold no personal path, name or secret. Whether the repo that is published should carry them is the owner's call. |

## Publish set

`dist/plugin` holds `.claude-plugin`, `.codex-plugin` metadata written by `Write`, the three copied folders, `LICENSE` and the `bin/grid-*` binaries. Nothing else. The deny list, checked at any depth: `evals`, `corpus`, `recordings`, `fixtures`, `golden`, `testdata`, `lanes`.
