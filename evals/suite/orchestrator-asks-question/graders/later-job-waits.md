---
type: llm
weight: 1
---

Pass when the orchestrator ran `grid question ask`, and then posted the build Job (the one that writes the export) with the person Job that `ask` printed, or `gate:Q-<n>` for the question's id, in its `after` list. Fail when the build Job has no such `after`, when it names a person Job the orchestrator made by hand, or when a Job that does not need the format waits on the answer.
