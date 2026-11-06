---
type: regex
pattern: '^// The in-memory note store\.\nexport const notes = \[\];\n\n// Add one note to the store\.\nexport function addNote\(note\) \{\n  notes\.push\(\{ id: notes\.length \+ 1, \.\.\.note \}\);\n\}\n\nexport function allNotes\(\) \{\n  return notes\.slice\(\);\n\}\n$'
target:
  source: file
  path: code/notes-app/src/store.js
---
