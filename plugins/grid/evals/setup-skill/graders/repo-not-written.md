---
type: regex
pattern: '"name"\s*:\s*"(?:Write|Edit|MultiEdit|NotebookEdit)"\s*,\s*"input"\s*:\s*\{\s*"(?:file_path|notebook_path)"\s*:\s*"(?:[^"\\]|\\.)*repo/notes-app|"name"\s*:\s*"Bash"\s*,\s*"input"\s*:\s*\{\s*"command"\s*:\s*"(?=(?:[^"\\]|\\.)*repo/notes-app)(?:[^"\\]|\\.)*(?:(?<!>)>>?(?![>&])(?!\s*/dev/null)|\btee\b|\bcp\b|\bmv\b|\brm\b|\btouch\b|\bmkdir\b|\bsed\s+-i|\bgit\s+(?:-C\s+\S+\s+)?(?:add|commit|push|checkout|reset|merge|stash|clean|restore|apply|init|rebase|pull|fetch))'
match: not_contains
target: trace
---
