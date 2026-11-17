One directory per brain topic: topics/<topic>/TOPIC.md holds the topic's description and its pointers, read by the CLI and the hooks.

The eight built-in brain topics are listed in internal/brain/topics.go:25; they need no file. A custom knowledge topic is one TOPIC.md, in the plugin folder (the topics the plugin ships), under a project's Grid folder or under the Grid home. The older knowledge/<topic>/TOPIC.md is still read when topics/<topic>/TOPIC.md is absent, so an existing definition keeps working; nothing writes it. A topic's notes stay in knowledge/<topic>/; with the Grid config's `brain` key set they are read from grid/<topic>/ in the brain folder (projects/<project>/grid/<topic>/ at project level).

TOPIC.md front matter: name, kind: knowledge, level, description. The description is the topic's summary. Each body line that starts with "- " is a pointer: what to open, then why.

No hook puts a topic in front of an agent. A topic reaches an agent in these ways:

- the SessionStart index (internal/hook/hook.go) lists each topic with its description and entry count;
- `grid brain search --topic <t> <words>` finds notes in a topic (it needs search words), and `grid brain show <id>` reads one;
- a skill's front-matter description says when to read a note;
- the brain links of a Job name the notes that Job uses, and the Job's inputs.json `not_rendered` lists the ones its brief cut.

A skill's `topics: <topic>` front matter names the topic the skill belongs to; it does not start a hook.

Shipped topics: delivering (named by skills/run-job) and reviewing (named by skills/review-job).
