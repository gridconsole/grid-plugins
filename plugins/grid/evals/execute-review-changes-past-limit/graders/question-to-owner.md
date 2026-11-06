---
type: llm
focus: trace
---
PASS if the assistant, finding the review rounds used up (round 2 of 2) and the Verdict still returning must-fix items, raised one blocking question to Alex (the task's owner) through the questions skill, named the open must-fix items (the case-sensitive tag match, the missing test, the missing README line) and gave a recommendation, and did not start another worker.
FAIL if the assistant sent the work back to build for a third round without asking, wrote no question, asked about something other than the open must-fix items, put an amount of money in the question, or approved the work.
