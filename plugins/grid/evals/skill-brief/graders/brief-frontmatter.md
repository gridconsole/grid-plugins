---
type: regex
pattern: '^-{3}\nid: 01-find-note-storage\nrole: scout\nstatus: (?:briefed|running|done)\nverify: .+\nfollow_up: .+\n-{3}'
flags: m
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/subagents/01-find-note-storage.md
---
