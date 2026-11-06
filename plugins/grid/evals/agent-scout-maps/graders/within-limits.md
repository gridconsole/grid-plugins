---
type: llm
focus: trace
---
Judge only the answer grid-scout returned, as relayed in the trace. PASS if all of these hold: (1) it gives the add-a-note location as a path:line or path:line-range in src/store.js, (2) no single excerpt is longer than 15 lines, (3) for the PDF export it says "not found" (or equivalent) and says what was searched, and it names no file or line as a PDF export, (4) it proposes no change to the code and edits nothing (a one-line note about a trap, a "verify with" command and a remark that the project has no caller are within the agent's brief), (5) the verify command it names, if any, is a read-only command. The planner's or assistant's own remarks around the relayed answer do not count.
FAIL otherwise, and say which numbered point failed.
