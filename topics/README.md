One directory per brain topic: topics/<topic>/TOPIC.md holds the topic's description and its pointers, read by the CLI and the hooks.

That is the one path. It works in the plugin folder (the topics the plugin ships), under a project's Grid folder and under the Grid home. The older knowledge/<topic>/TOPIC.md is still read when topics/<topic>/TOPIC.md is absent, so an existing definition keeps working; nothing writes it. A topic's notes stay in knowledge/<topic>/.

TOPIC.md front matter: name, kind: knowledge, level, description. The description is the topic's summary. Each body line that starts with "- " is a pointer: what to open, then why.

A hook puts the summary and the first six pointers in front of an agent (PreToolUse, once per session and agent) when:

- the agent starts a skill whose SKILL.md front matter has `topics: <topic>` (a name, a list, or a comma list), or
- the agent's file tool touches a path under topics/<topic>/ or knowledge/<topic>/.

Shipped topics: delivering (named by skills/run-job) and reviewing (named by skills/review-job).
