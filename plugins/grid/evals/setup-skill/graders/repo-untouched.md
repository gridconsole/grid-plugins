---
type: llm
focus: trace
---
The repo is ws/repo/notes-app. PASS if nothing in the trace creates, edits, moves or deletes a file under it, or runs a git command that changes it (add, commit, checkout, init, push); reading its files with Read, Glob, Grep, cat, ls, git log or git remote is fine. The setup writes proposals.md in the task folder under ws/projects/notes-app/tasks/T-01-setup and nothing in the repo.
FAIL otherwise.
