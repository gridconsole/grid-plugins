# The Grid brain as an Obsidian vault

Decision lane P-brain-obsidian-decide, 17 Nov 2026. This note decides the vault
reader, ids, links, authority and folder layout. It builds nothing. The owner's
person notes become first-class brain notes, and Obsidian can be used as the
editor without Grid fighting it. Tasks, Jobs and the board as brain nodes
(Addendum 3) are decided by P-brain-board-decide; section 10 names the seams
that lane must use.

Evidence is given as plugin `file:line` at the lane head. Obsidian facts cite
the official help source
([obsidianmd/obsidian-help](https://github.com/obsidianmd/obsidian-help) at
commit `cbe447c`, 16 Nov 2026, published at https://obsidian.md/help) and the
official API typings
([obsidianmd/obsidian-api](https://github.com/obsidianmd/obsidian-api)
`obsidian.d.ts` at commit `879c936`, 10 Nov 2026). Help paths below are
relative to `en/` in that repo.

## Decision

**Decided.**

1. **Any `.md` file in the brain folder is a note.** Grid fields are optional.
   The reader takes a file with no front matter, unknown keys and any name.
   Every Grid field has a default taken from the path and the text.
2. **The id is the path.** It is the path inside the level root, without
   `.md`, NFC-normalised, and spaces and Unicode are allowed. The link target
   is `brain:<level>/<id>` with a multi-segment id. `ValidName` stays only for
   names Grid generates.
3. **Front matter stays flat; described links go in the body.** Grid writes
   only flat keys that Obsidian Properties can show and edit. The described
   links move to a body `## Links` section, one line each:
   `- [[target]] - rel: why`. Every `[[x]]`, `[[x|y]]`, `[[x#h]]` and
   `![[x]]` in the body is an edge. Old front-matter `links:` are read during
   the transition and rewritten into the body by the migration.
4. **Who wrote a note is a Grid record, not note text.** Origin and author come
   from immutable per-change records that Grid writes. Bytes that no Grid
   record explains are a person change, with the author taken from the
   configured Grid name. A front-matter `origin:` is never trusted.
5. **Grid adopts person changes.** A changed, new, renamed or deleted file is
   seen on read, on index rebuild and on `grid brain check`. It is recorded
   with origin person, versioned (content-addressed) and reindexed. Audit drift
   on a brain file becomes adoption, not a finding.
6. **Person notes win.** An agent never rewrites a person note. An agent write
   aimed at a person note, or one that loses a version check against a person
   save, becomes a proposal in the visible `<brain>/Proposals/` folder. This is
   a redirect with exit 0, not a refusal, gate or lock on the agent.
7. **Layout.** The brain is its own folder, read from the brain path key in
   `~/.config/grid/config.json`. It is separate from GRID_HOME, where tasks,
   accounts and usage stay.
   - Topics are top-level folders, each with an optional `TOPIC.md`.
   - `grid/` holds the config Grid acts on.
   - `projects/<p>/` has the same shape.
   - `Proposals/` is visible and not indexed.
   - `.grid/` holds only the synced, immutable bookkeeping: `changes/` and
     `versions/`.
   - Index, backlinks, the file-hash cache and the local lock are caches in
     the machine folder and are never synced.
8. **One reader.** `grid brain list --json` is the one listing the app and
   P-brain-view call. Its fields are fixed in section 3.4.

**Why.**

- Obsidian 1.x does not support nested properties (Properties.md:81). Today's
  front matter nests `origin:` (note.go:17-19) and `links:` as a list of maps
  (note.go:56-60, :72). Obsidian shows those as source YAML only.
- Obsidian rewrites `[[links]]` on rename, not Grid's `to: brain:home/x`
  strings (Internal links.md:15). So links in front matter rot the first time
  the owner renames a note in Obsidian.
- Today's reader is strict on both ends:
  - no front matter is an error (mdfile/markdown.go:50);
  - an unknown key is an error (brain/markdown.go:21);
  - the topic, level and name must equal the path (topics.go:241);
  - the name must match `ValidName` (note.go:80-83).

  So every person note is invalid today.
- Rewriting re-dumps the whole struct (config.go:187-192), which drops the
  keys a person added.
- The ruling of 17 Nov 02:12 forbids a lock in the synced brain and a shared
  `log.md`, and puts index and backlinks in the machine folder.

**Rejected.**

- *Keep links in front matter as a list of maps.* Not shown by Properties
  (Properties.md:81) and not rewritten on rename.
- *Front-matter links as quoted `"[[x]]"` lists.* Obsidian caches these
  (`frontmatterLinks`, obsidian.d.ts:2530-2540), but a list value cannot carry
  rel and why without nesting, and the docs do not say whether a rename
  rewrites them.
- *Read both forms forever.* Two sources of edges disagree after the first
  Obsidian rename. They are read only until the migration lane has run.
- *Keep `name`, `topic` and `level` keys as the id.* An Obsidian rename or move
  changes the path, not the keys, so the note turns invalid (topics.go:241).
- *Id = bare file name.* Two folders may hold `README.md`; the path is unique.
- *Strict `ValidName` for every file.* It rejects spaces and Unicode, which the
  owner's notes use.
- *Trust a front-matter `origin:`.* Anyone can type it, and the brief requires
  the id to come from the configured name, never from note text.
- *Refuse agent writes to person notes.* That is a gate; the lane rules forbid
  new gates. A redirect to a proposal gives the same authority without
  blocking.
- *`.grid/proposals/`* (first layout addendum). Replaced by Addendum 2: visible
  `Proposals/`.
- *`brain.lock`, `log.md`, `index.md` and `backlinks.json` in `<brain>/.grid/`*
  (first layout addendum). Replaced by the 17 Nov 02:12 ruling.
- *Numbered versions `<n>.md`* (versions.go:80-102). Two machines both pick
  last+1. Content-addressed names cannot collide.

## 1. Obsidian facts

| Fact | What Obsidian does | Source |
|---|---|---|
| Property types | text, list, number, checkbox, date, date & time, tags. A type is vault-wide per property name. | Editing and formatting/Properties.md:37-47 ([obsidian.md/help/properties](https://obsidian.md/help/properties)) |
| Nested maps and lists | "Not currently supported … **Nested properties**: To view nested properties, we recommend using the source mode." A nested map or a list of maps is not editable in the Properties UI. | Properties.md:77-81 |
| Lists | A list is one value per `- ` line. Values may be text, numbers or internal links. | Properties.md:164-180 |
| Links in properties | Wikilinks in text and list properties must be quoted: `link: "[[Episode IV]]"`. | Properties.md:154, :168 |
| Default properties | `tags`, `aliases`, `cssclasses`, all lists. Publish also reads `description`. `tag`, `alias` and `cssclass` were deprecated in 1.4 and dropped in 1.9. | Properties.md:278-282, :292, :298 |
| JSON front matter | Read and "saved as YAML". | Properties.md:272 |
| How a property edit is saved | `processFrontMatter(file, fn)`: "frontmatter is passed in as a JS object" and written back, so Obsidian re-serialises the whole block. That key order, comments and quoting survive is **not stated** (inference). | obsidian.d.ts:4436-4456 ([source](https://github.com/obsidianmd/obsidian-api/blob/879c936/obsidian.d.ts#L4436)) |
| Front-matter links | Cached as `frontmatterLinks` (since 1.4.0) with the property key. | obsidian.d.ts:2530-2540, :4893 |
| Rename | "Obsidian can automatically update internal links in your vault when you rename a file. If you want to be prompted instead, you can disable it." The setting is Files and links → Automatically update internal links. | Linking notes and files/Internal links.md:15-17 ([obsidian.md/help/links](https://obsidian.md/help/links)); User interface/Settings.md:222-224 |
| Link format written | New link format: shortest path when possible, relative path, or absolute path in vault. The API's `fileToLinktext` gives the shortest unique form. | Settings.md:214-220; obsidian.d.ts:6495 |
| Folder links | "Folder paths start at the vault root and use forward slashes (`/`), even on Windows." A link to a missing note creates it at that path. | Internal links.md:28-30 |
| Link forms | `[[x]]`, `[[x.md]]`, `[[x#heading]]`, `[[x#h1#h2]]`, `[[x#^block]]`, `[[x\|text]]`, Markdown `[t](x%20y.md)`, embed `![[x]]`. A non-Markdown target needs its extension. | Internal links.md:23-24, :57-59, :76-84, :100, :158-161 |
| Unsafe characters | "A string which contains the following characters may not work as a link: `# \| ^ : %% [[ ]]`". | Internal links.md:44-45 |
| Dot-folders | Operating systems hide them; Obsidian Sync treats files and folders starting with `.` as hidden and does not sync them (except `.obsidian`). | Files and folders/How Obsidian stores data.md:25; Obsidian Sync/Sync settings and selective syncing.md:105 |
| Excluded files | Hidden in Search, Graph and Unlinked mentions, and less noticeable in the quick switcher and link suggestions. | Settings.md:252; Internal links.md:61-62 |
| Deleted files | Moved to the system trash, to `.trash` in the vault, or deleted for good (a setting). | Settings.md:240-246 |
| Changes outside Obsidian | "Obsidian automatically refreshes your vault to keep up with any external changes." The API fires vault `modify`, `create`, `delete` and `rename` events. | How Obsidian stores data.md:12; obsidian.d.ts:10445, :10457 |
| Symlinks | Changes to symlinked files are not watched. | Files and folders/Symbolic links and junctions.md:15 |

**NOT VERIFIED from the docs.** These go to the Mac round trip (section 9.4):

- whether the file explorer hides dot-folders such as `.grid/` (only the OS
  and Sync rules are documented);
- whether automatic link update is on by default;
- whether a rename also rewrites quoted `"[[x]]"` front-matter links;
- what an open editor does when Grid writes the same file;
- whether file names are case-sensitive in links on APFS.

The decisions below do not depend on these facts:

- `.grid/` holds no notes;
- adoption catches renames whether or not links were rewritten;
- Grid writes no front-matter links;
- an agent never writes a person note.

## 2. Ids and names

### 2.1 The rule

- **Level root.** The home level is the brain root minus `projects/`. The
  project level is `projects/<p>/`.
- **Id.** The path of the file inside its level root, `/`-separated, with `.md`
  removed and the text NFC-normalised. Examples:
  - `decisions/Use Go for the CLI`
  - `Ideas/Café plan`
  - `grid/agents/curator`
- **Link target.** `brain:<level>/<id>`, for example
  `brain:home/decisions/Use Go for the CLI`. The id may have any number of
  segments.
- **Readable id (`ValidID`).** Not empty. No leading or trailing `/`, no
  empty, `.` or `..` segment, no `\`, no control characters, at most 255 bytes
  per segment.
- **Linkable id.** A readable id that also contains none of Obsidian's unsafe
  characters `# | ^ [ ]` (Internal links.md:44). A readable but not linkable
  person file is listed and searchable, and `grid brain check` gives a hint.
  It is never renamed by Grid.
- **Lookup.**
  1. An exact match on the NFC id.
  2. A match that ignores case. macOS's default file system ignores case, and
     a link typed in Obsidian may differ in case from the file name.
  3. For a bare target with no `/`, the unique note with that file name in the
     level. This is Obsidian's shortest-path rule (Settings.md:214-220). If two
     notes share the file name, the link is reported as ambiguous and not
     resolved.
  4. An old id found in a rename change record (section 5) resolves to the
     new path.
- **`ValidName`** (note.go:80-83) stays only for names Grid generates:
  - a proposal file name;
  - the base name of a note the curator files;
  - skill, agent, routine and workflow names;
  - `EditConfig` names;
  - topic folder names Grid creates.
- **A note at a level root** (no folder) has topic `-`.

### 2.2 Every reader that must change

Plugin, at the lane head:

| Where | Today | Change |
|---|---|---|
| internal/brain/note.go:83 `ValidName` | the rule for every note and topic | Keep it for generated names. Add `ValidID` and `LinkableID` next to it. |
| internal/brain/note.go:172 `Link.Validate` | a `brain` target needs exactly `level/ValidName` | `level/<ValidID>`, any number of segments. The 8-160 character why rule (:165-195) applies to `## Links` lines Grid writes, not to bare body links. |
| internal/brain/note.go:93-160 `Validate` | name, topic, type, description, summary, source and origin are required; at most 300 lines | Split it: `ValidateGenerated` keeps today's rules for notes Grid writes. A person note is never invalid for a missing Grid field. |
| internal/brain/note.go:118-122 | origin kind is session, job, server or import | Add `person`. Origin moves out of front matter (section 4). |
| internal/brain/markdown.go:21 `Parse` | strict `mdfile.Load(...,true)` | A new lenient `ParseVault`: no front matter is allowed, unknown keys are kept, known keys accept Obsidian's shapes (`tags` as a string or a list). |
| internal/mdfile/markdown.go:46-50 `Split` | an error when there is no `---` | `SplitOptional` returns empty front matter. |
| internal/brain/topics.go:204-269 `scan` | walks `knowledge/`; invalid unless topic, level and name equal the path and `len(parts)==3` (:241); a duplicate name is invalid (:254-266) | Walk the level root at any depth. Skip `.obsidian/`, `.trash/`, `.grid/`, every dot-folder, `Proposals/`, `projects/` (at home level) and non-`.md` files. Derive topic, id and title from the path. Duplicate base names are legal and only make a bare link ambiguous. Keep the symlink refusal (:217). |
| internal/brain/topics.go:47-63 `topicAt` | a custom topic needs `topics/<t>/TOPIC.md` with `kind: knowledge` | A top-level folder with a `TOPIC.md` is an indexed topic. A folder without one holds searchable notes but has no row in SessionStart (layout addendum). |
| internal/brain/topic_text.go:26 | `topics/<name>/TOPIC.md`, falling back to `knowledge/<name>/TOPIC.md` | `<level root>/<topic>/TOPIC.md` |
| internal/brain/store.go:25-33 `Store`, `Entry` | `GridHome`; `Entry.Path` is relative to GRID_HOME | Add `Root` (the brain folder from config; GRID_HOME when the key is absent). `Entry.Path` is relative to `Root`. |
| internal/brain/store.go:60-75 `prefix`, `scope` | `projects/<p>` and `os.OpenRoot(GridHome)` | The same, opened on `Root`. |
| internal/brain/store.go:208-215 `entry` | `knowledge/<topic>/<name>.md`, `brain:<level>/<name>` | Id from the path. Proposals go to `Proposals/<name>.md`. |
| internal/brain/query.go:58-70 `splitID` | needs `ValidName` | `ValidID` plus the lookup order in 2.1. |
| internal/brain/query.go:109 `Search` | scores name and description | Scores title (the file name), description, aliases, tags and body. Person-first rule from 6.3. |
| internal/brain/index.go:57-87, :219-240 | counts `knowledge/<topic>`; writes `knowledge/index.md` and `knowledge/backlinks.json`; backlink `From` is `brain:level/name` | Counts topic folders. Index and backlinks go to the machine folder (7.2). `From` uses the full id. |
| internal/brain/config.go:29 `configPath` | `knowledge/<topic>/<name>.md` | `grid/<topic>/<name>.md` |
| internal/brain/check.go:102-105, :136 | reads `brain-proposals/` and `knowledge/agents/` | `Proposals/` and `grid/agents/`. |
| internal/brain/sb_skill_for.go, sa_agent_for.go | `knowledge/skills/<n>/SKILL.md`, `knowledge/agents/<n>.md` | `grid/skills/<n>/SKILL.md`, `grid/agents/<n>.md` |
| internal/brain/waiting.go:131-150 | `brain-proposals/` | `Proposals/` |
| internal/brain/ke_rename.go | rewrites front-matter `links` | Rewrites body `[[...]]` and `## Links` targets: the same job Obsidian does on rename. |
| internal/brain/task_decide.go:195-197 | `GridHome/knowledge/<topic>` | `Root/<topic>`; a task part, see 9.2 |
| internal/hook/brain.go:51-52 | `ValidName(base name)`, otherwise ignored | Take the read path relative to `Root`. If it is inside, ends in `.md` and is not under a skipped folder, its id is the path. Spaces and Unicode are recorded. |
| internal/hook/brain.go:61-65 | `Show("brain:"+level+"/"+name)`, compared with `GridHome + e.Path` | `Show` by id, compared with `Root + e.Path`. |
| internal/hook/brain.go:175-228 `parseBrainShow` | id is one shell word | Unchanged: quoted ids with spaces already work (shellWords :154). |
| internal/hook/hook.go:147 | SessionStart `Index()` | Unchanged in shape (8.3). |
| internal/braincmd/command.go:369-376, :539-579 | `brain:` and `task:…/brain-proposals` ids need `ValidName` | `ValidID`; the task form follows P-brain-board-decide. |
| internal/braincmd/ke_rename.go:46-50, ke_write.go:83, hy_drop.go:53,92, cb_author_origin.go:19-24, changes.go:268 | `ValidName` on the target | `ValidID` for the target; `ValidName` only for new names Grid makes. |
| internal/job/source.go:75, :169 | brain links of a Job | `ValidID` (id parsing only; the Job shape is untouched). |
| internal/workflow/brain.go:164, :181-184, :193, :225 | `knowledge/workflows` | `grid/workflows` |
| internal/routine/schedule.go:126, workflow.go:29,97, execute.go:98,103, store.go:150,183,260,299 | `knowledge/routines` | `grid/routines` |
| internal/people/store.go:35 | `knowledge/people` | `grid/people` |
| internal/projectenv/projectenv.go:74, internal/cli/ln_project_env.go:325 | `knowledge/environments` | `grid/environments` |
| internal/land/land.go:124, approval.go:62 | `projects/<p>/knowledge/deploy`, `knowledge/workflows` | `projects/<p>/grid/deploy`, `grid/workflows` |
| internal/settings/claude.go:54-55 | permission glob `knowledge/**` under GRID_HOME | The absolute brain root `<Root>/**`. |
| internal/settings/budget.go:61 | `knowledge/budget/agent-budget.md` | `grid/budget/agent-budget.md` |
| internal/guard/context.go:250-257 | knowledge paths | `Root` paths |
| internal/orchestrator/progress.go:33 | skips `brain-proposals` | Skips `Proposals`. |
| internal/jobcmd/command.go:1082 | `Index()` | Unchanged. |
| internal/migrate/plan.go:213, :231 | old knowledge paths | Read by the migration lane (section 8). |
| contract/formats.md:175-177 | `brain-proposals` paths | `Proposals/`; task proposals per P-brain-board-decide. |
| skills/brain-keeper/SKILL.md:6,31,39-45,73; skills/plan-jobs/SKILL.md:41,89; agents/library.md:15; agents/curator.md:15-19; topics/README.md:3,9 | old paths and the front-matter link rule | New layout, `## Links` rule, person authority. |

App (`git -C <ref>/app.git show main:internal/slices/brain/vault.go`):

| Where | Today | Change |
|---|---|---|
| internal/slices/brain/vault.go:80-81 `vaultRoots` | hard-codes `<project>/knowledge` and `knowledge` under the Grid home | Delete it. Call `grid brain list --json` and read `path` relative to the brain root it returns. |
| vault.go:401, :445-447 | strips `workspace/` and `knowledge/` prefixes to match a link | Match on `id` and `aliases` from the listing. |
| internal/kernel/webui/app/pages/brain/browser.mjs | shows rows from vault.go | Shows the listing fields. P-brain-view owns it. |

## 3. Links

### 3.1 Decision: a body `## Links` section

A note Grid writes ends with:

```markdown
## Links

- [[decisions/Use Go for the CLI]] - decided-by: the CLI choice fixed the build language
- [[grid/agents/curator|curator]] - uses: the curator files these proposals
```

- **Line format.** `- [[<target>]] - <rel>: <why>`, where:
  - rel comes from the existing relations list (note.go:87);
  - why keeps the 8-160 character rule for lines Grid writes (note.go:165-195).
- **Target form.** The absolute path in the vault, without `.md`. It is
  unambiguous and is what Obsidian's "absolute path in vault" setting writes
  (Settings.md:214-220). A shortest-path target written by Obsidian is also
  read (2.1, rule 3).
- **The link summary is not stored.** Today's `Link.Summary` (note.go:59) is
  computed on read from the target's description, so it cannot go stale.
- **Rel without why.** A `## Links` line with an unknown rel or no why is still
  an edge, with rel `related` and an empty why.

### 3.2 Every link in the body is read

- **Link forms.** `[[x]]`, `[[x|y]]`, `[[x#h]]`, `[[x#^b]]`, `![[x]]` and
  Markdown `[t](x.md)` with `%20` decoded. Each gives an edge to `x`.
  - The heading or block is kept as an anchor in the edge.
  - The display text is ignored.
- **Code is skipped.** Links in fenced code blocks and inline code are not
  edges.
- **Rel.** An edge outside `## Links` has rel `related` and no why. Missing why
  is not a finding for person notes.
- **Unresolved targets.** A target that resolves to no file becomes a
  *concept*: Obsidian's "note that does not exist yet" (Internal links.md:30).
  It is listed under `concepts` (3.4).
- **Front-matter links.** Quoted `"[[x]]"` values in front matter (Obsidian's
  form, Properties.md:154) are also edges with rel `related`.

### 3.3 Existing front-matter links: migrate, and read both only until then

- **Migrate.** The migration lane rewrites every front-matter `links:` list of
  maps into the `## Links` section:
  - `to: brain:home/x` becomes `[[<topic>/x]]`;
  - rel and why are kept;
  - summary is dropped (computed).
- **Read both until then.** The vault reader reads old `links:` maps as edges
  until the migration has run, so existing notes and tests keep passing.
- **Why.** Front-matter link maps are invisible in Properties (Properties.md:81)
  and are not wikilinks, so an Obsidian rename leaves them pointing at the old
  name. After migration the body is the only place edges live. One source of
  edges means the app, search and Obsidian's backlinks pane agree.

### 3.4 Listing fields (`grid brain list --json`)

One JSON object per file, in path order. P-brain-view and the app read only
this.

| Field | Meaning |
|---|---|
| `id` | `brain:<level>/<id>` (section 2). |
| `path` | Path relative to the brain root, `/`-separated, with `.md`. |
| `root` | The brain root (once, in the envelope, not per row). |
| `kind` | `note`, `topic` (a TOPIC.md), `proposal`, `skill`, `agent`, `workflow`, `routine`, `person`, `machine`, `account`, `config`, or one of the kinds P-brain-board-decide adds (`task`, `job`, `board`). |
| `origin` | `person` or `agent`, from the change records (4.3). |
| `author` | For person changes, the configured Grid name. For agent changes, the session, Job or server id. |
| `level` | `home` or `project`. |
| `project` | The project name at project level. |
| `task` | The task a proposal or agent note came from (from its change record). Empty otherwise. |
| `title` | The file name without `.md`. |
| `summary` | Front-matter `summary`, else `description`, else the first paragraph. |
| `description` | Front-matter `description`, else the first paragraph (4.2). |
| `topic` | The first folder under the level root, or `-`. |
| `tags` | Front-matter `tags` plus inline `#tags` in the body, deduplicated. |
| `aliases` | Front-matter `aliases`. |
| `concepts` | Front-matter `concepts` if present, plus unresolved link targets. |
| `edges` | `[{to, rel, why, anchor, resolved}]`: every link out of the note (3.2). |
| `rev` | `NoteRev` of the bytes (hy_drop.go:62), the same value the app computes. |
| `updated` | Time of the newest change record for the path. Not mtime, because sync tools rewrite mtime. |
| `conflict` | Ids of waiting proposals that target this note (6.3). |

## 4. Reading person notes

### 4.1 Any `.md` is a note

- **Lenient parse.** The reader uses `SplitOptional` and `ParseVault` (2.2).
  - A file with no `---` block has empty front matter.
  - Front matter that is not valid YAML is listed with `kind: note` and a
    `grid brain check` finding "front matter is not YAML". It is still
    searchable by body.
- **Not notes.** Non-`.md` files are attachments, not notes. They are found
  through embeds (`![[x.png]]`).

### 4.2 Defaults

| Field | Default |
|---|---|
| topic | First folder under the level root; `-` at the root. |
| id | Path without `.md` (2.1). |
| title | File name without `.md`. The front matter has no `title` key in Obsidian's defaults (Properties.md:278-282). |
| description | Front-matter `description` (Obsidian Publish uses the same key, Properties.md:292), else the first paragraph of the body after an optional `# H1`. The paragraph has Markdown removed and is cut at 200 characters on a word. |
| type, status, source | Empty. These are required only for notes Grid generates (`ValidateGenerated`). |

### 4.3 Origin is person, from config

- **Where origin comes from.** The change record whose content hash equals the
  file's current bytes (section 5).
- **No record.** If none matches, the bytes were not written by Grid, so
  origin is `person` and author is the Grid name configured in
  `~/.config/grid/config.json` (the user name key P-grid-config-read reads).
  When no name is configured, the author is the OS user name.
- **Note text never counts.** A front-matter `origin:` or `author:` is kept as
  an unknown key and never read as origin. Grid stops writing `origin:` into
  notes; the migration moves existing `origin:` maps into change records.

### 4.4 Unknown keys kept byte-for-byte

- **Patch, not re-dump.** A rewrite never re-dumps the struct. Today
  `EncodeConfig` does that (config.go:187-192), and so does `Encode`
  (brain/markdown.go:12) through it.
- **The patch.** It works on lines:
  - find the top-level key's span: its line, up to the next line that starts
    at column 0;
  - replace only that span;
  - append a new key before the closing `---`;
  - leave every other byte (comments, order, quoting, unknown keys, CRLF) as
    it was.
- **Body rewrite.** Only the `## Links` section is replaced. The rest of the
  body is copied unchanged.
- **Callers.** Every caller of `Encode` or `EncodeConfig` that rewrites an
  existing file moves to the patch: `EditConfig` and `EditConfigBy`, decide's
  status rewrite (store.go:436, :497), `markSuperseded` (store.go:560) and
  `RenameWith`. `Encode` stays for a new file Grid creates.

## 5. Person edits adopted

### 5.1 When and how

- **When.** `adopt(root)` runs:
  - at the start of every `grid brain` read (show, search, list, index);
  - under the write lock before every Grid write;
  - in `grid brain check`.
- **How.** It walks the brain (with the skips in 2.2) and compares each file
  with the machine-folder cache `files.json` (path → size, mtime, hash).
  - Size and mtime unchanged means no hash is needed.
  - Otherwise the file is hashed. If the hash matches a change record Grid
    wrote for this path, the change is Grid's own; only the cache is updated.

| Seen | Recorded |
|---|---|
| new path, no record | `created`, origin person; version stored |
| known path, new hash, no record | `edited`, origin person; version stored |
| path gone and a new path with the same hash | `renamed` old → new, origin person; the old id resolves through this record (2.1, rule 4) |
| path gone, no new path with that hash | `deleted`, origin person. Bytes already versioned stay in `versions/`. Bytes that were never seen are not recoverable by Grid; Obsidian's `.trash` setting keeps them (Settings.md:240-246). |

After any change the index and backlinks caches are rebuilt for the touched
notes.

### 5.2 Change records and versions

- **One immutable file per change** (ruling 17 Nov 02:12):
  `<brain>/.grid/changes/<yyyy-mm>/<utc-time>-<machine>-<hash8>.md`.
  - It holds the fields of today's audit line (changes.go:55-58): time,
    status, id, origin kind and id, reason, topic, content hash.
  - A rename also holds the old id. A proposal record also holds the task.
  - It replaces the append-only `knowledge/log.md` (changes.go:38, :94).
- **Content-addressed versions**:
  `<brain>/.grid/versions/<level>/<id>/<hash16>.md`. Storing the same bytes
  twice is a no-op, and two machines never collide. This replaces last+1
  (versions.go:80-102). Order comes from the change records.
- **Duplicate adoptions.** Two machines adopting the same person edit write two
  records with the same (path, hash, status). Readers deduplicate them.
- **Same-second records.** Records sort by time, then machine. Readers never
  need a global counter.

### 5.3 audit_check.go drift

- **Today.** `auditDrift` (audit_check.go:35-91) compares the newest `log.md`
  hash with the file bytes and reports "entry changed outside the keeper"
  (:85-87).
- **New, for a note, proposal or `grid/` file.** A difference is a person
  change. `check` adopts it (5.1) and reports `adopted <n> person changes`, a
  count, not a finding.
- **The finding stays only for `.grid/`:**
  - a version whose bytes do not hash to its file name;
  - a change record that was edited (its hash is in its name);
  - a record that names a version that does not exist.
- **Write errors that go away.** `ErrNotVersioned` (ke_write.go:211-213) and
  the version mismatch in `markSuperseded` (store.go:583) no longer happen,
  because adoption runs under the lock just before the write.

### 5.4 Concurrent writes

1. **Read.** An agent write carries the `rev` it read (`--expect-rev`,
   `NoteRev`, hy_drop.go:62; checked today at ke_write.go:188-195).
2. **Check.** Under the write lock (7.2): adopt, then compare the current
   bytes' rev with the expected rev.
3. **Mismatch.**
   - If the current bytes are a person change (origin person), the agent
     change is written as a proposal to `Proposals/` and the command exits 0
     with `proposed: Proposals/<name>.md`.
   - If they are another agent's change, the command re-reads, re-applies the
     agent's patch (4.4) when it touches different keys or sections, and
     otherwise writes a proposal. It never overwrites silently.
4. **Write.** A temp file in the same folder (a dot-file, so Obsidian and the
   reader ignore it), then rename. Today's `atomicWrite` (store.go:117) is
   kept, with the temp file moved next to the target because ids have folders.
5. **With the grid sync server.** The write sends the version it was based on.
   The server rejects a stale write. The client re-reads, re-applies and
   retries by steps 2-4 (ruling 17 Nov 02:12; run-jobs Addendum 5A). A person
   save that reaches the server first therefore turns the agent write into a
   proposal.
6. **Person saves.** Obsidian writes without Grid's lock. A person save that
   lands after a Grid write is a later person change and is adopted, so the
   person wins. One that lands before is caught by step 2.

## 6. Authority

### 6.1 Person notes win

- **Which notes are person notes.** A note whose newest change record has
  origin person, or that has no record at all.
- **An agent never rewrites one.** Every write path (`WriteWith`
  ke_write.go:117, `EditConfigBy`, `RenameWith`, supersede, revert) checks the
  target's origin first. If it is person, the write becomes a proposal (6.2)
  and returns success with the proposal path.
- **No gate.** Nothing is refused, held or blocked; the work is kept as a
  proposal.
- **Deleting a person note** is likewise a proposal (`status: proposed`,
  `change: delete`).

### 6.2 Proposals in `<brain>/Proposals/`

- **Where.** `Propose` (store.go:219) writes `Proposals/<name>.md`, where name
  is a `ValidName` made by Grid. The front matter has the target id (for an
  edit), task and status. The body is the proposed note or edit.
- **Not indexed.** `Proposals/` is visible in Obsidian, not a topic, and not
  indexed as knowledge (Addendum 2). The listing gives its files
  `kind: proposal`.
- **Person edits to a draft.** The owner may edit a draft. Adoption records
  that edit with origin person, and the curator works from the edited bytes.
  The person's edits are person input (Addendum 2).
- **Decision state.** It is a change record (`accepted`, `rejected`), not an
  in-place rewrite of the proposal. Today's in-place status rewrite
  (store.go:436, :497) goes. On accept:
  1. the curator files the note into its topic folder (a Grid write with
     origin agent and the proposal id as reason);
  2. the proposal file is removed;
  3. its bytes stay in `versions/`.

### 6.3 The curator and conflicts

- **Two choices.** The curator Job (agents/curator.md) either files a proposal
  as a new note or turns it into an edit proposal against a person note. It
  never applies that edit itself.
- **Applying an edit.** The person applies it in Obsidian, or a person step in
  the brain workflow approves it. That step lives in the workflow file
  (`grid/workflows/brain.md`, read by workflow/brain.go), not in code.
- **Conflicts.** A note with a waiting proposal against it, or an agent note
  linked to a person note by `supersedes` or `duplicates`, is a conflict.
  - Search, the library agent and briefs list the person note first, with the
    proposal or agent note after it marked `conflict`.
  - The listing carries `conflict: [<proposal ids>]` (3.4).
  - Among notes that do not conflict, the score decides.

## 7. Layout

### 7.1 The brain folder

```
<brain>/                      brain path key in ~/.config/grid/config.json
  .obsidian/                  Obsidian's own; ignored by Grid
  .trash/                     Obsidian's trash; ignored
  .grid/                      synced Grid bookkeeping, immutable files only
    changes/<yyyy-mm>/<time>-<machine>-<hash8>.md
    versions/<level>/<id>/<hash16>.md
  Proposals/                  visible; not a topic; not indexed
  decisions/  TOPIC.md  *.md  built-in knowledge topics
  conventions/ TOPIC.md *.md
  Ideas/  Daily/  ...         the owner's folders; searchable; a topic once it has TOPIC.md
  grid/                       config Grid acts on; visible and editable
    agents/  skills/<name>/SKILL.md  workflows/  routines/
    people/  machines/  accounts/  environments/  deploy/  budget/
  projects/<p>/               same shape: topics, grid/, Proposals/
```

- **Brain path key.** The brain folder comes from the brain path key that
  P-grid-config-read adds (read-only). When the key is absent, `Root` is
  GRID_HOME and the reader also reads the old `knowledge/` layout, so nothing
  moves until the owner sets the key and runs the migration ("when absent, the
  brain stays where it is today").
- **Built-in config topics** move from `knowledge/` to `grid/`: environments,
  machines, deploy, people, agents, routines (topics.go:25-36). This ends the
  clash between a top-level `people/` topic and config.
- **`environments/`, `deploy/` and `budget/`** go under `grid/` too. They are
  config Grid reads (projectenv.go:74, land.go:124, budget.go:61), although
  the brief's list does not name them.
- **`grid/accounts/`** holds account notes (run-jobs Addendum 2). Secrets never
  go in the brain; the secret check (note.go:199-214) still runs on every
  Grid write.

### 7.2 What stays in `<brain>/.grid/`

- **In `.grid/`.** Only `changes/` and `versions/`: immutable, synced
  bookkeeping that cannot be rebuilt from the notes.
- **In the machine folder,** `<machine folder>/brain/<root-hash>/`, never
  synced:

  | File | Use |
  |---|---|
  | `index.json` | The cache SessionStart reads. |
  | `backlinks.json` | The backlinks cache. |
  | `files.json` | The adoption cache (5.1). |
  | `brain.lock` | The local lock, used when no sync server is configured. |

  The machine folder is `~/Library/Application Support/Grid` on macOS and
  `~/.local/state/grid` on Linux (run-jobs Addendum 2).
- **Deviation from the first layout addendum.** It placed index.md,
  backlinks.json, log.md, brain.lock and proposals/ in `.grid/`. Stated
  reason: the 17 Nov 02:12 ruling and Addendum 2 replace it.
- **Obsidian Sync.** It does not sync `.grid/` (Sync settings and selective
  syncing.md:105). Only the grid sync server carries it, which is what the
  ruling wants.

### 7.3 Screenshots

- **What they are.** Screenshots are task attachments; their place is P-brain-board-decide's
  task folder.
- **The curator compresses them** when it files a proposal or delivers. It runs
  `grid shot crop <png> --max 1568`. `shot.Crop` already scales the long side
  down to `MaxSide` = 1568 (shot.go:21, :80-118) and never up.
- **Target size: at most 300 KB per image,** with the text still readable at
  100%.
  - Today `shot.Encode` writes PNG only (shot.go:120-127). PNG stays for UI
    screenshots, where it is small and sharp.
  - A PNG over 300 KB after scaling is written as JPEG quality 80 through a new
    `--jpeg` flag. It uses Go's standard `image/jpeg`, with no new dependency.
    Go has no WebP encoder in the standard library.
- **Docs change in one lane.** The curator agent (agents/curator.md) and the
  brain-keeper skill (skills/brain-keeper/SKILL.md) gain the step and the
  target in the same lane.

## 8. Migration

### 8.1 `grid brain migrate`

`grid brain migrate --dry-run|--apply` runs once, with the brain path key set.
It copies from GRID_HOME into `<brain>` and leaves the old folder in place
until the owner removes it.

| From | To |
|---|---|
| `knowledge/<topic>/<name>.md` (decisions, conventions, custom) | `<topic>/<name>.md` |
| `knowledge/{environments,machines,deploy,people,agents,routines,workflows,budget}/…` | `grid/<topic>/…` |
| `knowledge/skills/<n>/SKILL.md` | `grid/skills/<n>/SKILL.md` |
| `topics/<t>/TOPIC.md`, `knowledge/<t>/TOPIC.md` | `<t>/TOPIC.md` |
| `brain-proposals/*.md` | `Proposals/*.md` |
| `knowledge/log.md`, one line each | one record each in `.grid/changes/`, with time and hash kept |
| `knowledge/versions/<topic>/<name>/<n>.md` | `.grid/versions/<level>/<id>/<hash16>.md` |
| `projects/<p>/knowledge/…` and the rest | `projects/<p>/…` with the same mapping |
| `knowledge/index.md`, `backlinks.json`, `.brain.lock` | not copied; rebuilt in the machine folder |

### 8.2 Changes to each note

The migration changes only Grid's own keys and leaves every other key as it is
(4.4):

- the `links:` maps become the `## Links` section (3.3);
- the `origin:` map becomes the note's first change record;
- the `name`, `topic` and `level` keys are removed (derived from the path);
- an `updated:` value that is newer than the last log line seeds the record
  time.

For each old id `brain:<level>/<name>`, the migration writes a `renamed`
record to its new id. Task files, Job sources and history lines that still
name the old id resolve through it (2.1, rule 4). No task file is rewritten.

### 8.3 Flat notes and SessionStart

- **Flat notes.** The owner's notes in `~/.grid/<name>/knowledge` that sit
  directly in `knowledge/`, or that fail today's validation, become person
  notes. They are copied byte-for-byte into the topic `--flat-topic <folder>`
  (default `Notes/`, with a `TOPIC.md`), with one `created` record each,
  origin person. No front matter is added.
- **SessionStart** keeps listing only topics and counts within the same
  1200-byte budget (index.go:175):
  - one row per built-in topic and per folder with a `TOPIC.md`;
  - the entries column counts person and agent notes;
  - a waiting-proposals count replaces nothing.

  It still says how to read a note: search, then show.
- **Dry run.** `--dry-run` prints every copy and the counts before and after.
  It must show the same number of notes, every old id resolving, and the same
  topics in the SessionStart text.

## 9. Lane plan

The plugin repo is gridconsole/grid-plugins; the app repo is gridconsole-app.
Builders run on sonnet as the brief says. The format change and the migration
run at xhigh. Every lane keeps the existing brain tests passing and adds the
fixtures named below.

The brief's fixture list (F1-F9):

- F1 no front matter;
- F2 tags, aliases and cssclasses;
- F3 nested folders;
- F4 spaces and Unicode in names;
- F5 `[[x|y]]` and `[[x#heading]]`;
- F6 an `.obsidian/` folder;
- F7 a rename;
- F8 a delete;
- F9 an edit between Grid's read and its write.

### 9.1 Lanes in order

**L1 P-brain-vault-read**

- **Repo and model.** plugin, sonnet, high.
- **Depends on.** P-grid-config-read (brain path key).
- **Builds.**
  - `Store.Root`;
  - `SplitOptional` and `ParseVault`;
  - `ValidID` and `LinkableID`;
  - path ids and the 2.1 lookup;
  - body link parsing (3.2) and old front-matter links (3.3);
  - the 4.2 defaults;
  - the walk with its skips;
  - `grid brain list --json` with the 3.4 fields (origin is `person` for
    everything until L4);
  - hook/brain.go.
- **Files.**
  - mdfile/markdown.go;
  - brain/markdown.go, note.go, topics.go, topic_text.go, store.go, query.go,
    index.go;
  - a new brain/vault.go;
  - braincmd/command.go;
  - hook/brain.go;
  - a README section "Use your Obsidian vault as the Grid brain".
- **Acceptance.**
  - F1, F2, F3, F4, F5 and F6 read and list correctly;
  - an old `knowledge/` brain still lists with the key absent;
  - a `Read` of `Ideas/Café plan.md` logs a brain read.

**L2 P-brain-readers**

- **Repo and model.** plugin, sonnet, high.
- **Depends on.** L1.
- **Builds.** Every row of 2.2 outside internal/brain moves to `Root` and the
  `grid/` paths: braincmd, job/source.go (id parsing only), workflow, routine,
  people, projectenv, land, settings, guard, orchestrator, contract, skills
  and agents docs.
- **Acceptance.**
  - each reader finds its file under `grid/…` in a new-layout fixture and
    under `knowledge/…` with the key absent;
  - the settings permission glob covers `<Root>/**`.

**L3 P-brain-format**

- **Repo and model.** plugin, sonnet, **xhigh**.
- **Depends on.** L1.
- **Builds.**
  - the `## Links` writer;
  - the line-level front-matter patch (4.4) for every rewrite caller;
  - `ValidateGenerated` versus person notes;
  - origin out of front matter;
  - `RenameWith` rewriting body links.
- **Acceptance.**
  - a rewrite of a note with unknown keys, comments and CRLF changes only the
    edited key (byte diff);
  - F5 links survive a Grid rename;
  - F2 lists stay lists.

**L4 P-brain-adopt**

- **Repo and model.** plugin, sonnet, high.
- **Depends on.** L1, L3.
- **Builds.**
  - per-change records and content-addressed versions in `.grid/` (5.2);
  - adoption (5.1);
  - the machine-folder caches and local lock (7.2);
  - the audit drift change (5.3);
  - expect-rev and proposal on mismatch (5.4);
  - person authority on every write path (6.1);
  - person-first ranking (6.3);
  - `origin`, `author`, `updated` and `conflict` in the listing.
- **Acceptance.**
  - F7: a rename is recorded and the old id resolves;
  - F8: a delete is recorded;
  - F9: an edit between read and write turns the agent write into a proposal
    and the person bytes are unchanged;
  - an agent write to a person note writes `Proposals/` and exits 0;
  - `grid brain check` adopts a person edit without a drift finding;
  - two adoptions of the same edit produce one logical change.
- **Seam.** The sync server's version-checked write is a client call into
  the run-jobs sync API. Until that lands, the local lock is the path.

**L5a P-brain-layout**

- **Repo and model.** plugin, sonnet, high.
- **Depends on.** L2, L4.
- **Builds.**
  - `Proposals/` and the decision state in records (6.2);
  - the `grid/` config topics;
  - TOPIC.md topics;
  - the SessionStart rows (8.3).
- **Acceptance.**
  - the SessionStart text is at most 1200 bytes, with topics and counts only;
  - a folder without TOPIC.md is searchable but has no row;
  - `Proposals/` is not in search results as knowledge.

**L5b P-brain-curator-shots**

- **Repo and model.** plugin, sonnet, high.
- **Depends on.** L5a and **P-brain-board-decide**.
- **Builds.**
  - agents/curator.md and skills/brain-keeper/SKILL.md (new layout,
    `## Links`, person authority, screenshots);
  - `grid shot crop --jpeg`;
  - task proposals and attachments per the board decision;
  - task_decide.go.
- **Acceptance.**
  - a 3840×2400 PNG fixture ends at most 300 KB with its long side at most
    1568;
  - the curator files a proposal into a topic with working `[[links]]`.

**L6 P-brain-migrate**

- **Repo and model.** plugin, sonnet, **xhigh**.
- **Depends on.** L3, L4, L5a.
- **Builds.** `grid brain migrate --dry-run|--apply` (section 8).
- **Acceptance.**
  - a fixture copy of a home and project brain with log, versions,
    proposals, front-matter links and origin, plus a flat-notes folder,
    migrates with equal note counts;
  - every old id resolves;
  - links appear in `## Links`;
  - flat notes are byte-identical in `Notes/`;
  - the SessionStart topics are unchanged;
  - a second `--apply` changes nothing.

**L7 P-brain-view**

- **Repo and model.** app, sonnet, high.
- **Depends on.** L1 (the field shape) and L4 (origin).
- **Builds.** It drops `vaultRoots` (vault.go:80-81) and the prefix matching
  (:401, :445-447) and reads `grid brain list --json`.
- **Acceptance.** The app lists a person note with spaces in its name, shows
  its edges and backlinks, and marks a conflict person-first.

### 9.2 Before P-brain-board-decide lands

- **May build now:** L1, L2, L3, L4, L5a and L6, the vault reader and the
  layout. They do not touch the task, Job or board shape. L2 changes only id
  parsing in job/source.go.
- **Must wait:**
  - L5b, which covers task proposals in task folders (store.go:280
    `ProposeTask`, task_decide.go), screenshots as task attachments and the
    curator's task steps;
  - any `task`, `job` or `board` kind in the listing.
- **Seams that lane must use:**
  - ids are paths under a level root (2.1);
  - edges are body `[[links]]` plus `## Links` (3);
  - origin comes from change records (4.3);
  - adoption and the person-first rule apply to every `.md` it places in the
    brain (5, 6);
  - new kinds are added to the listing's `kind` (3.4);
  - task files may link `[[decisions/x]]` and the reader resolves them.

### 9.3 Order

L1 → L2 ∥ L3 → L4 → L5a → L6; L7 after L4; L5b after L5a and
P-brain-board-decide. Then the Mac round trip.

### 9.4 Mac round trip

Run by the owner or a grid-checker on the Mac, after L6 (and L7 for the app
step). Nothing is called done before this.

1. **Set up.**
   - Set the brain path key in `~/.config/grid/config.json` to a new folder.
   - Run `grid brain migrate --dry-run`, read the counts, then run
     `--apply`.
2. **Open the vault.** Open the folder as a vault in Obsidian 1.x.
   - Properties shows `description`, `tags` and `aliases` as editable rows, and
     no source-mode block for Grid keys.
   - Each note's `## Links` lines are clickable, and the backlinks pane of a
     target shows them.
   - Look whether `.grid/` shows in the file explorer (NOT VERIFIED, section
     1).
3. **Edit an agent note** in Obsidian, then run `grid brain check`.
   - It reports one adopted person change.
   - `grid brain list --json` shows `origin: person` and the owner's name for
     it.
4. **Write a new note** in `Ideas/`, then run `grid brain search <a word in it>`.
   - The note is found.
   - Start a Claude session: the session-start index lists topics and counts
     only.
5. **Rename a note** that others link to.
   - Obsidian rewrote the `[[links]]`. Note whether the auto-update setting
     was on by default (NOT VERIFIED).
   - `grid brain show brain:home/<old id>` resolves to the new path.
   - Try a case-only rename `Foo` → `foo` (NOT VERIFIED).
6. **Person-first.** Ask an agent to edit that renamed note through
   `grid brain write`.
   - A file appears in `Proposals/`.
   - The note is unchanged.
   - Search lists the note first, with the proposal marked as a conflict.
7. **Curator.** Have the curator file a proposal (after L5b).
   - It shows in its topic folder with working links and backlinks.
   - A screenshot attachment is at most 300 KB.
8. **Open editor.** With a note open in Obsidian, let Grid write a different
   agent note it links to. Look whether the open editor refreshes cleanly
   (NOT VERIFIED).
9. **Delete a note** in Obsidian.
   - `grid brain check` records the delete.
   - The bytes are in `.grid/versions/`.

## Open items

- Five Obsidian behaviours are not stated in the official docs and are checked
  on the Mac (section 1, step lines marked NOT VERIFIED). No decision depends
  on them.
- The app reader was read from the reference app repository at `main`. The
  plugin reader rows outside internal/brain and internal/hook come from a
  scout map at the lane head and were not each re-opened in this lane.
- The sync server's version-checked write API is not built yet (run-jobs
  lane). L4 uses the local lock until it lands.

DONE
