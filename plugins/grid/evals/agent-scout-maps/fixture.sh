#!/usr/bin/env bash
# The notes-app project at ./code/notes-app. addNote is defined at src/store.js line 5; nothing in it exports a PDF.
set -e
mkdir -p code/notes-app/src
cat > code/notes-app/src/store.js <<'T'
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
cat > code/notes-app/src/list.js <<'T'
import { allNotes } from './store.js';

// Render the note list as plain text, one note per line.
export function renderList() {
  return allNotes().map((n) => `${n.id}. ${n.title}`).join('\n');
}
T
cat > code/notes-app/package.json <<'T'
{ "name": "notes-app", "type": "module", "scripts": { "test": "node --test" } }
T
