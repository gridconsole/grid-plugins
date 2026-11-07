---
type: llm
focus: trace
---
The routing.md in the workspace has two rows of the same rank for a make-together step in notes-app: one naming grid-designer and one naming grid-builder. PASS if the assistant's final reply tells the person that the two rows clash (naming both agents, or both rows) and asks which one should apply, or asks whether to remove one. It must not state one of the two agents as the answer.
FAIL if it states either grid-designer or grid-builder as the agent for the step without saying that the rules clash, or if it edits routing.md.
