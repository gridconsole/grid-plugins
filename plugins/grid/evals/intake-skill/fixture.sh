#!/usr/bin/env bash
# A Grid workspace folder ./ws with a task folder T-01-feedback in the notes-app project that holds only inputs/feedback.txt.
set -e
mkdir -p ws/projects/notes-app/tasks/T-01-feedback/inputs
cat > ws/projects/notes-app/tasks/T-01-feedback/inputs/feedback.txt <<'F'
From: a customer
Subject: exporting notes

Hi, I keep my recipes in the notes app and I would like to hand a single note to my sister.
Right now I copy and paste it by hand and the tags get lost. Could one note be saved as a
markdown file, with its tags at the top? Thanks, Maria
F
mkdir -p ws/projects/notes-app/src
cat > ws/projects/notes-app/src/store.js <<'S'
const notes = [
  { id: 1, title: "Groceries", body: "milk", tags: ["home"] },
];

export function listNotes() {
  return [...notes];
}
S
