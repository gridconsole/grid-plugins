---
type: regex
pattern: 'find +(?:/|~)|(?<!"name":"[^"]*",)\\?"(?:pattern|path)\\?":\s*\\?"(?:~|/(?:Users|home)?)(?=[/*"\\])'
match: not_contains
target: trace
---
