---
type: llm
focus:
  source: file
  path: ws/projects/notes-app/tasks/T-01-setup/proposals.md
---
This is the proposals file a setup task wrote for the notes-app project. The repo's README says tests run with make test and releases are cut on Fridays; docs/deploy.md says staging is staging.notes.example, deployed by a person from their machine; one listed Codex session says production is notes.example and only the person deploys it, from their laptop.
PASS if the file holds `## <pid>` blocks, each with a kind, a state of open and a source that names a file or a session and its date; at least one entry rests on the README or the docs; and at least one entry comes from a listed session (its source names claude or codex and a date) and states a fact that only the sessions hold, such as who deploys production.
FAIL if an entry has no source, if an entry is an invite, a person to add or a message to send, if an entry of any kind other than `environment` (a `setting`, a `workflow` or a `note`, for example) sets how work lands or when or how to deploy, or if the file is missing. An environment's `how`, `who` and `runs` fields describe the environment and are allowed.
