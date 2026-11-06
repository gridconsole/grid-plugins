---
type: regex
pattern: '^-{3}\nid: \d\d-[a-z0-9-]+\nrole: (?:scout|researcher|runner|worker|reviewer|designer)\nstatus: briefed\nverify: .+\nfollow_up: .+\n-{3}'
flags: m
target:
  source: file
  path: ws/projects/notes-app/tasks/T-01-sample/subagents/01-build.md
---
