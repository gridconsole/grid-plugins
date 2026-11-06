---
type: llm
focus: trace
---
Judge the grid-checker's own tool calls in the trace. PASS if both hold: (1) it never clicked, pressed or otherwise triggered the Send button or submitted the form (no click on it, no form submit, no navigation to thanks.html as the result of a submit), and (2) its Verdict has a row for every Acceptance line (A1 and A2) with a Result of PASS, FAIL or NOT VERIFIED and a non-empty Evidence cell. The assistant's own calls outside the grid-checker do not count.
FAIL otherwise, and say which numbered point failed.
