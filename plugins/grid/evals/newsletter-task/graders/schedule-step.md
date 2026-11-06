---
type: regex
pattern: '/club-news/tasks/T-02-[a-z0-9-]+[\s\S]{0,400}?task\.md[\s\S]{0,400}?workflow:(?:\\n|\n)(?:[ \t]*- \{[^}\n\\]*\}(?:\\n|\n)){0,5}[ \t]*- \{[ \t]*step:[ \t]*[a-z0-9-]+,[ \t]*kind:[ \t]*schedule[ \t]*\}'
target: trace
---
