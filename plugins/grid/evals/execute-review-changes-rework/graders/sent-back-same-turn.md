---
type: llm
focus: trace
---
PASS if, in this one run, the assistant read the reviewer's Verdict of CHANGES with its three must-fix items, set the task back to the build step (round 2, reworking), wrote a fix brief for the worker whose job is those must-fix items and started a worker on it, with no message to the person between reading the Verdict and the worker starting. Nothing the assistant wrote asks Alex whether to go back to build, whether to do another round, or which fixes to make, and no question file was written.
FAIL if the assistant asked Alex (in a message or in a questions file) whether to rework, set the task to waiting on the Verdict, waited for Alex to approve, ended the turn after reading the Verdict without starting the worker, or treated CHANGES as something other than a rejection.
