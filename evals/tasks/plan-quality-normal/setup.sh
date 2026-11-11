# A Grid home with task T-03 (a change across several files) that has a scout map and research.md but no plan yet, and a tiny notes-app repo beside it.
set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects"
printf "1\n" > "$GRID_HOME/VERSION"
t="$GRID_HOME"/projects/notes-app/tasks/T-03-tag-filter
mkdir -p work/notes-app/src work/notes-app/tests "$t"
cat > work/notes-app/src/store.js <<'S'
const notes = [
  { id: 1, title: "Groceries", body: "milk", tags: ["home"] },
  { id: 2, title: "Standup", body: "blockers", tags: ["work"] },
];

export function listNotes() {
  return [...notes];
}

export function searchNotes(query) {
  return notes.filter((n) => n.title.includes(query));
}
S
cat > work/notes-app/src/list.js <<'S'
import { listNotes, searchNotes } from "./store.js";

export function renderList(query) {
  const rows = query ? searchNotes(query) : listNotes();
  return rows.map((n) => "- " + n.title).join("\n");
}
S
cat > work/notes-app/src/cli.js <<'S'
import { renderList } from "./list.js";

console.log(renderList(process.argv[2]));
S
cat > work/notes-app/tests/store.test.js <<'S'
import { searchNotes } from "../src/store.js";

if (searchNotes("Stand").length !== 1) throw new Error("search by title failed");
S
cat > "$t/intent.md" <<'I'
let people filter the note list by tag, from the command line and the list view
I
cat > "$t/task.md" <<'T'
---
id: T-03
title: Filter notes by tag
project: notes-app
owner: alex
source: idea
size: normal
workflow:
  - {step: plan, kind: ask}
  - {step: build, kind: run}
  - {step: verify, kind: check}
step: plan
status: running
updated: 2026-03-04T10:00
summary: Let people filter the note list by tag.
next: Write the plan from research.md.
---

- 4 Mar · plan · scout and research done
T
cat > "$t/scout.md" <<'M'
# Scout map

- work/notes-app/src/store.js:1 the notes array; each note has a tags list
- work/notes-app/src/store.js:10 searchNotes matches the title only
- work/notes-app/src/list.js:3 renderList picks searchNotes or listNotes
- work/notes-app/src/cli.js:3 the command line passes its first argument to renderList
- work/notes-app/tests/store.test.js:3 the only test, for searchNotes
M
cat > "$t/research.md" <<'M'
# Research: tag filtering

## Seen
- Tags are stored per note as an array of lowercase strings (work/notes-app/src/store.js:2).
- Search looks at titles only, so a tag word never matches (work/notes-app/src/store.js:11).
- The list view and the command line share renderList (work/notes-app/src/list.js:3, work/notes-app/src/cli.js:3), so one filter in the store reaches both.
- Array.prototype.includes is case-sensitive; people will type Home and home (https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/includes).

## Inferred
- A separate filterByTag function is smaller than widening searchNotes, but a combined query such as "tag:home standup" would need one parser.

## Could not find
- No test for the list view or the command line exists.

## Options
- A: add filterByTag(tag) to the store and a tag argument to renderList.
- B: teach searchNotes to read a tag: prefix in the query.
M
printf -- '- 2026-03-04T10:00 · created · by alex\n' > "$t/history.md"
