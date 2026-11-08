---
type: llm
focus: trace
---
The setup skill must read first, write proposals.md, and only then write questions under questions/ in the task folder. The environments (web production, api production, staging) were all found in sources and proposed.
PASS if (1) the first Write of a questions file comes after the Write of proposals.md in the trace, (2) no posted question asks where production or staging is, or who deploys it, without listing what was found (a confirm question that lists the found sites is fine), and (3) every posted question has a `key:` line unless it is `group: followup`; base branch, deploys and the deploy command, when asked, use `key: base`, `key: deploys` and `key: deployCommand` and list what was found in Recommendation.
FAIL otherwise. Posting no question at all is also a PASS only if the closing message says why nothing remained.
