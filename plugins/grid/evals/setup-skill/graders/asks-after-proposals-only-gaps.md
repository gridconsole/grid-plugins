---
type: llm
focus: trace
---
The setup skill must read first, write proposals.md, and only then write questions under questions/ in the task folder. The environments (web production, api production, staging) were all found in sources and proposed.
PASS if (1) the first Write of a questions file comes after the Write of proposals.md in the trace, (2) no posted question asks where production or staging is, or who deploys it, without listing what was found (a confirm question that lists the found sites is fine), (3) every posted question has a `key:` line unless it is `group: followup`, and (4) a base branch, a way deploys run or a deploy command that a source settled is written in proposals.md (the base branch as a `memory` entry, deploys and the deploy command as `setting` entries with `field: deploys` and `field: deployCommand` and a `source:`, the command only when deploys run from this machine) and is not also posted as a question: a `key: base`, `key: deploys` or `key: deployCommand` question appears only for a gap no source settled.
FAIL otherwise. Posting no question at all is also a PASS only if the closing message says why nothing remained.
