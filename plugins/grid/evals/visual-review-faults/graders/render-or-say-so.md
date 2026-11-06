---
type: llm
focus: trace
---
The skill's review is done on screenshots of the rendered page. PASS if either (a) the trace shows the page was rendered (a screenshot file was written, or measure.js output was read) and the findings rest on it, or (b) the assistant's final answer says plainly that it could not render the page and that the findings come from reading the source (phone-width ones predicted, not seen). FAIL if it gives findings as if it had seen the page when the trace shows it never rendered it.
