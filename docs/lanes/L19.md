# Lane L19: projects and inbox signals

Implemented `internal/projects`, `internal/signal`, shared local storage helpers,
and command registration in the existing CLI. The module and the resolver and
project packages are unchanged. This checkout exposes `project.Load` and
`project.Parse`, not `project.Read`; the implementation uses those readers and
stages every project record through `project.Write` before an atomic rename.
Existing prose after project front matter is preserved.

## Commands

Build with `make build`, then use `dist/grid`:

```sh
grid project add [path] [--name example] [--json]
grid project remove example [--json]
grid project list [--json]
grid project scan <discovery-folder> [--json] [--apply]
grid project relink example <moved-folder> [--json]
grid signal add --project example --source mail --source-id message-1 --title Example --body - --json
grid signal act <signal-id> [--project example] [--json]
grid signal dismiss <signal-id> [--project example] [--json]
```

Flags work before or after positional arguments. `add` defaults to the current
folder. Project lookup for remove, relink and signal selection accepts a name or
stable project id. Relative project paths are relative to the current folder.
`GRID_HOME` overrides the default `~/.grid`.

Discovery proposes visible direct child folders, preserving each child's repos
at depths one and two as one project. It skips hidden directories, directory
symlinks, Personal folders and candidates belonging to an enclosing project
outside the discovery folder. It includes ordinary non-git folders. Without
`--apply`, even a missing Grid home remains absent. Apply registers candidates
in order; a later failure does not roll back earlier registrations.

Signal project selection defaults to the current folder's registered project.
Outside projects, adding a signal creates or uses Personal, without binding the
home or temporary folder. Other projects must be registered first. `--source`
defaults to `typed`; `--source-id` is optional, but should be supplied for imported
items. `--body -` reads stdin; other body values are literal text. A title or
nonblank body is required. `act` marks the note `acted`; `dismiss` marks it
`dismissed`. Repeating the same transition is harmless; changing one terminal
state to the other is refused. Neither action creates a task or evaluates text.

## Validation

Run from the checkout, leaving `TMPDIR` at its default:

```sh
make test
make lint
make golden
go test -race ./internal/localdata ./internal/projects ./internal/signal ./internal/cli
GOOS=darwin go build ./...
GOOS=windows go build ./...
```

The tests cover temp homes, two remotes, two repos including a grandchild repo,
repeat adds, clones, changed remotes, ownership conflicts, naming collisions,
read-only discovery, apply, a moved folder, retained project prose and notes,
unlinking only the current machine, and Personal. Signal tests cover concurrent
capture, project/source/id deduplication, preserved bodies containing instruction
text and fake front matter, both terminal states, input limits, and unsafe paths.
JSON goldens cover every delivered command, including scan/apply and duplicate
signals; a note golden covers the external-data markers. Paths, generated ULIDs
and timestamps are normalized only in test output.

`make test`, `make lint`, the focused race check, and both cross-builds passed.
Native tests ran on Linux. A native macOS runner was unavailable; paths are
symlink-resolved in fixtures and no platform-specific syscalls were introduced.

## Intended differences and source references

References below are relative to the read-only reference bundle: `docs/` is the
new specification and `old-plugin/` is the old implementation.

1. One Grid home replaces desktop workspace selection and `GRID_PROJECTS`.
   Project storage has stable ULIDs, machine-specific folder bindings, normalized
   remotes and match keys. Names may contain spaces or uppercase letters and get
   numeric collision suffixes; Personal is reserved. Compare
   `old-plugin-map.md:73`, `old-plugin-map.md:92`,
   `old-plugin/hooks/lib.mjs:159`, and
   `old-plugin/skills/signals/scripts/add-signal.mjs:20` with
   `docs/plugin-design.md:130` and `docs/go-architecture.md:251`.
2. Explicit add and scan/apply create records; merely discovering or listing does
   not. Scan is a batch JSON array, with no background process or desktop setting
   persisted. The direct-child proposal policy and opt-in apply make the lane's
   discovery behavior concrete. Sources: `docs/plugin-design.md:136`,
   `docs/plugin-design.md:149`, `docs/go-architecture.md:163`,
   `docs/go-architecture.md:341`, and `docs/go-architecture.md:358`.
3. Every remote is retained. Because the supplied `project.Repo` has a singular
   `remote` field, a repo with multiple remotes has multiple entries with the same
   relative path, plus all normalized remote keys. Changed remotes add identity
   keys. Relink retains the id and shared keys, replaces this machine's folders
   and local keys, and refreshes repos. Remove retains data, repos, shared keys
   and other machines. Overlapping projects and partial multi-repo clone adds
   are refused instead of silently merging or rewriting the parent. Sources:
   `old-plugin-map.md:73`, `docs/plugin-design.md:133`,
   `docs/plugin-design.md:135`, and `docs/go-architecture.md:262`.
4. The machine file contains an exclusively created ULID, never a hostname.
   This lane initializes it and `VERSION` on mutation, preserving the required
   write-free scan and list behavior. It does not change unrelated read commands
   to create a machine on their first call. Sources:
   `docs/go-architecture.md:256`, `docs/go-architecture.md:265`, and
   `docs/go-architecture.md:266`.
5. Go flags replace the Node JSON argument: `source`, `source-id` and `body`
   replace `via`, `ext` and `text`. Plain output is the default; `--json` adds
   `id`, `project` and `status` to the old result/note information. The source
   name is open-ended for locally named MCP sources; it defaults to `typed`.
   Project selection can use the existing resolver and Personal instead of
   requiring a routine-provided slug. Sources:
   `old-plugin/skills/signals/scripts/add-signal.mjs:7`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:17`,
   `old-plugin/skills/signals/SKILL.md:12`, `docs/plugin-design.md:134`,
   `docs/plugin-design.md:262`, and `docs/go-architecture.md:346`.
6. Signal filenames are `S-<sha256>.md` for a source/id pair, or `S-<ULID>.md`
   without an external id. The project directory supplies the deduplication
   scope. This replaces date/title filenames and `.grid/seen.jsonl`, avoiding a
   note/index dual write. Duplicate capture returns the original note even after
   act/dismiss, without replacing its title or body. Deleting the note also
   deletes its deduplication record. Sources:
   `old-plugin/skills/signals/scripts/add-signal.mjs:2`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:191`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:216`, and
   `old-plugin/skills/signals/scripts/add-signal.mjs:240`.
7. YAML serialization replaces manual front matter rewriting. Notes explicitly
   carry code-assigned `tier: external` and `body_kind: data`; the body is preserved
   byte for byte after validation. Unsupported controls and oversized notes are
   refused instead of silently stripping or truncating content. Source names
   allow 128 UTF-8 bytes, source ids 512, and titles 1024; ids may contain spaces.
   The existing 64 KiB whole-note and 4000-file limits remain; duplicates are
   checked before the file-count limit. Sources:
   `old-plugin/skills/signals/scripts/add-signal.mjs:22`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:175`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:188`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:203`,
   `old-plugin/skills/signals/SKILL.md:40`, and `docs/brain-feed.md:15`.
8. Capture and update timestamps are UTC RFC3339 with subsecond precision,
   replacing local minute timestamps and the `GRID_NOW` override. Files written
   by the atomic helper have mode 0600 rather than the old note's usual 0644.
   Local O_EXCL writer locks plus staged rename replace the old note hard-link
   and seen-index sequence. A leftover lock fails closed after a bounded wait;
   no process guesses that it may steal it. Sources:
   `old-plugin/skills/signals/scripts/add-signal.mjs:11`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:173`,
   `old-plugin/skills/signals/scripts/add-signal.mjs:219`, and
   `old-plugin/skills/signals/scripts/add-signal.mjs:246`.
9. Act/dismiss operate on stored metadata only, retain the note and deduplication
   identity, and enforce the terminal-state policy described above. They do not
   start work from a suggestion or message. Sources:
   `old-plugin/skills/signals/SKILL.md:39`,
   `old-plugin/skills/signals/SKILL.md:41`, and
   `docs/go-architecture.md:346`.

## Not implemented and limitations

- Old `from`, `origin`, `due` and `sug` fields and the old JSON invocation are not
  ported. They are beyond the requested minimal signal interface; callers can
  retain contextual text in the body. This is an intentional compatibility gap
  relative to `old-plugin/skills/signals/scripts/add-signal.mjs:8` and
  `old-plugin/skills/signals/SKILL.md:28`. Old note files and seen indexes need a
  separate migration before these commands can manage them.
- Project merge, task-triggered automatic registration/refresh, missing-repo
  notifications, server sync, routing, routines and task creation are separate
  lanes. This implementation neither changes the protected resolver nor adds
  desktop operations. See `docs/plugin-design.md:135`,
  `docs/plugin-design.md:136`, `docs/go-architecture.md:264`, and
  `docs/go-architecture.md:346`.
- The supplied old-plugin tree contains the registry reader and signal capture
  script, but not the desktop's project add/scan/relink and inbox mutation
  implementations. Exact old-app parity could not be checked; the new spec and
  requested behavior define those commands here. The ownership transfer is
  listed at `docs/go-architecture.md:341` and `docs/go-architecture.md:346`.
- The local writer locks coordinate these CLI processes, not server sync or
  unrelated programs. Symlinks in data paths are refused, but the filesystem
  checks are not a sandbox against another process swapping parent directories
  concurrently. The old script describes the same parent-swap limitation at
  `old-plugin/skills/signals/scripts/add-signal.mjs:242`.
