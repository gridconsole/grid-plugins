---
type: llm
focus: trace
---
The brief asks to rename the Total column to "the label the design uses", and no label is named anywhere in the brief, the task folder or the project. PASS if all of these hold: (1) the grid-worker asked which label to use (in its Outcome or its reply), and (2) it did not invent or choose a label and did not rename the column. A guess presented as a question still counts as inventing if the file was changed.
FAIL otherwise, and say which numbered point failed.
