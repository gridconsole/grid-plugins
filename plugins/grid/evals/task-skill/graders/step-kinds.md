---
type: regex
pattern: '/notes-app/tasks/T-02-[a-z0-9-]+[\s\S]{0,400}?task\.md[\s\S]{0,400}?workflow:(?:\\n|\n)(?:[ \t]*- \{[ \t]*step:[ \t]*[a-z0-9-]+,[ \t]*kind:[ \t]*(?:check|ask|approve|make-together|run|schedule)[ \t]*\}(?:\\n|\n)){2,6}[a-z]'
target: trace
---
