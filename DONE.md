# Lane L21 complete

Built the Go golden replay harness in `internal/golden`. Every corpus case gets
an isolated `t.TempDir()` populated from `fixtures/` and its recorded setup.
Supported commands run in-process through `cli.Run` with recorded stdin and
environment. Setup preludes and simulated tool edits run before the baseline.
The harness compares stdout, stderr, exit status and recorder-compatible file
diffs, including symlinks, permissions and changes confined to normalized values.
`internal/golden/normalize.go` lists all normalization rules.

The legacy recordings encode commands as `hook` plus optional `call`; the Go
adapter maps implemented counterparts explicitly. New recordings may carry
literal `argv`. Unsupported commands and unsupported preludes use `t.Skip` with
the command name. A documented `go-diff.md` selects all four files in that case's
`go-expected/`; missing expectations or doc:line references fail. Original legacy
outputs are unchanged.

The recorder now requires `--source` or `GRID_OLD_PLUGIN`, has no machine-specific
source default, and records runtime requirements without the executing Node
version or OS. It preserves reviewed Go expectation files during regeneration
and `--check`. These recorder changes are at `recordings/record.py:22`,
`recordings/record.py:50` and `recordings/record.py:253`.

## Run and validation

```sh
make test
make lint
make golden
GOOS=windows go build ./...
GOOS=darwin go build ./...
./record.sh --source /path/to/old-plugin --check
```

All checks above passed in the Linux runner with the supplied reference source
for the last command. `make test` used the default TMPDIR. Windows and Darwin
were cross-compiled, not executed on those operating systems. No platform syscall
or build-tag changes were necessary. Temporary roots are symlink-resolved, so
macOS's symlinked temporary directory does not invalidate containment checks.

`make golden` reports **136 cases: 6 pass, 0 fail, 130 skip**, with a case/command/
result table. The corpus run takes less than one second here; it is explicitly
opted into by `make golden`, keeping future implementations out of normal unit
test timing. Unit tests include two tiny synthetic cases (pass and deliberate
diff), isolated CLI invocation, preludes, file effects, alternate expectations,
normalization, context hunks, mode/type changes, symlinks and path containment.
An additional authoring check matched the diff formatter against Python's
`difflib` on 700 generated inputs, including popular repeated lines and missing
final newlines; that check is not a dependency of the committed Go tests.

## Intended differences from the old plugin

These expectations capture the existing Go SessionStart implementation. This
lane does not change CLI behavior. Each row has its own `go-diff.md` and complete
`go-expected/` under `golden/<case>/`.

| Case | Reviewed difference | References |
|---|---|---|
| `agent-routing/empty-object` | Combined context replaces the old agent catalog; the temporary workspace resolves to Personal; home/project exports are written and GRID_TASK is unset. | `docs/lanes/L10.md:21`, `docs/lanes/L10.md:50`, `docs/lanes/L10.md:53`, `internal/project/project.go:6` |
| `agent-routing/session` | Combined context includes T-1-demo; the shell file exports that task along with home and projects. | `docs/lanes/L10.md:21`, `docs/lanes/L10.md:53` |
| `agent-routing/invalid-json` | Malformed JSON produces no output or writes; the old script ignored stdin and printed its agent catalog. | `docs/lanes/L10.md:21`, `docs/lanes/L10.md:56`, `docs/old-plugin-map.md:125` |
| `grid-env/discovered` | Desktop config is ignored; the default ~/.grid and Personal replace app-selected demo; the default home is exported and GRID_TASK unset. | `docs/lanes/L10.md:50`, `docs/lanes/L10.md:51`, `docs/old-plugin-map.md:31` |
| `grid-env/explicit` | Existing variables no longer suppress context or exports; the temporary workspace resolves to Personal and GRID_TASK is unset. | `docs/lanes/L10.md:50`, `docs/lanes/L10.md:51`, `internal/project/project.go:6` |
| `grid-env/no-config` | Missing desktop config no longer suppresses context or exports; Go uses ~/.grid and Personal and unsets GRID_TASK. | `docs/lanes/L10.md:50`, `docs/lanes/L10.md:51` |

For the five successful SessionStart inputs, the context is factual and contains
the compact role list and terminal approval path (`docs/lanes/L10.md:52`,
`docs/lanes/L10.md:53`). Go's JSON serialization orders `additionalContext` before
`hookEventName` (`internal/hook/hook.go:77`, `internal/hook/hook.go:80`). Shell
exports start with a blank line and use quoted values, task export/unset, and
mode 0600 (`internal/hook/hook.go:92`, `internal/hook/hook.go:100`), replacing the
old mode-0644 file in the discovered case. These byte differences are explicitly
recorded, not normalized away. Exit zero and empty stderr remain unchanged.

## Limits

The other 130 cases cannot compare command behavior yet: this CLI has no
implemented counterpart for their legacy hook or exported helper invocation.
They still rebuild and validate setup and expectations before skipping. The
adapter intentionally does not treat silent unknown-hook success as support.
Add mappings in `internal/golden/corpus_test.go` when implementations land;
unsupported prelude commands also prevent a false pass.

`session-check` remains skipped because its Claude compatibility probe is not
the new advisory release-version command (`docs/lanes/L10.md:54`). Live server,
desktop bridge and remote worker behavior remain outside the recorded corpus
(`golden/INDEX.md:149`, `golden/INDEX.md:150`, `golden/INDEX.md:151`,
`golden/INDEX.md:152`). Native macOS
and Windows execution was unavailable; the requested cross-platform builds pass.
