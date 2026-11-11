set -eu
: "${GRID_HOME:?}"
mkdir -p "$GRID_HOME/projects/notes-app/tasks/T-01-sample"
printf "1\n" > "$GRID_HOME/VERSION"
printf 'map the notes app\n' > "$GRID_HOME/projects/notes-app/tasks/T-01-sample/intent.md"
mkdir -p work/notes-app/src
cat > work/notes-app/src/store.js <<'T'
// The in-memory note store.
export const notes = [];

// Add one note to the store.
export function addNote(note) {
  notes.push({ id: notes.length + 1, ...note });
}

export function allNotes() {
  return notes.slice();
}
T
cat > work/notes-app/src/list.js <<'T'
import { allNotes } from './store.js';

// Render the note list as plain text, one note per line.
export function renderList() {
  return allNotes().map((n) => `${n.id}. ${n.title}`).join('\n');
}
T
printf '{ "name": "notes-app", "type": "module" }\n' > work/notes-app/package.json
